package evacuator

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

type KubernetesHandlerConfig struct {
	InCluster            bool   // Indicates if running inside a Kubernetes cluster
	CustomKubeConfigPath string // Optional custom kubeconfig path
}

type KubernetesHandler struct {
	RestConfig *kubernetes.Clientset // Kubernetes clientset for interacting with the cluster
	logger     *slog.Logger          // Logger for structured logging
}

func NewKubernetesHandler(config *KubernetesHandlerConfig, logger *slog.Logger) (*KubernetesHandler, error) {

	var k8sRestConfig *rest.Config

	// Get Kubernetes config based on configuration
	if config.InCluster {
		var err error
		k8sRestConfig, err = rest.InClusterConfig()
		if err != nil {
			return nil, fmt.Errorf("failed to get in-cluster config: %s", err)
		}
	} else {

		var err error
		k8sRestConfig, err = clientcmd.BuildConfigFromFlags("", config.CustomKubeConfigPath)
		if err != nil {
			return nil, fmt.Errorf("failed to get kubeconfig: %s", err)
		}
	}

	clientset, err := kubernetes.NewForConfig(k8sRestConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes client: %s", err)
	}

	return &KubernetesHandler{
		RestConfig: clientset,
		logger:     logger,
	}, nil
}

func (h *KubernetesHandler) Name() string {
	return "kubernetes"
}

func (h *KubernetesHandler) HandleTermination(ctx context.Context, event TerminationEvent) error {
	h.logger.Info("handling kubernetes node termination", "node", event.Hostname, "handler", h.Name())

	// check if kubernetes node is exist
	_, err := h.RestConfig.CoreV1().Nodes().Get(ctx, event.Hostname, v1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get kubernetes node: %s", err)
	}

	h.logger.Info("kubernetes node found, proceeding with cordon", "node", event.Hostname, "handler", h.Name())

	// cordon the node
	_, err = h.RestConfig.CoreV1().Nodes().Patch(ctx, event.Hostname, types.MergePatchType, []byte(`{"spec":{"unschedulable":true}}`), v1.PatchOptions{})
	if err != nil {
		return fmt.Errorf("failed to cordon kubernetes node: %s", err)
	}

	h.logger.Info("kubernetes node successfully cordoned", "node", event.Hostname, "handler", h.Name())

	// drain the node
	// same like `kubectl drain nodes xxxx --ignore-daemonsets --delete-emptydir-data`
	err = h.drainNode(ctx, event.Hostname)
	if err != nil {
		return fmt.Errorf("failed to drain kubernetes node: %s", err)
	}

	h.logger.Info("kubernetes node termination handling completed successfully", "node", event.Hostname, "handler", h.Name())
	return nil
}

// drainNode drains a Kubernetes node by evicting all pods except DaemonSet pods
func (h *KubernetesHandler) drainNode(ctx context.Context, nodeName string) error {
	h.logger.Info("starting node drain", "node", nodeName, "handler", h.Name())

	// Get all pods on the node
	fieldSelector := fields.OneTermEqualSelector("spec.nodeName", nodeName).String()
	pods, err := h.RestConfig.CoreV1().Pods("").List(ctx, v1.ListOptions{
		FieldSelector: fieldSelector,
	})
	if err != nil {
		return fmt.Errorf("failed to list pods on node %s: %w", nodeName, err)
	}

	h.logger.Info("found pods on node", "node", nodeName, "total_pods", len(pods.Items), "handler", h.Name())

	var podsToEvict []corev1.Pod
	var skippedPods = make(map[string]int)

	// Filter and collect pods to evict
	for _, pod := range pods.Items {
		// Skip pods that are already terminating
		if pod.DeletionTimestamp != nil {
			skippedPods["terminating"]++
			continue
		}

		// Skip completed pods (Succeeded or Failed phase)
		if pod.Status.Phase == corev1.PodSucceeded || pod.Status.Phase == corev1.PodFailed {
			skippedPods["completed"]++
			continue
		}

		// Skip DaemonSet pods (--ignore-daemonsets behavior)
		if h.isDaemonSetPod(&pod) {
			skippedPods["daemonset"]++
			h.logger.Debug("skipping DaemonSet pod", "pod", pod.Name, "namespace", pod.Namespace, "handler", h.Name())
			continue
		}

		// Skip static pods (managed by kubelet directly)
		if h.isStaticPod(&pod) {
			skippedPods["static"]++
			h.logger.Debug("skipping static pod", "pod", pod.Name, "namespace", pod.Namespace, "handler", h.Name())
			continue
		}

		podsToEvict = append(podsToEvict, pod)
	}

	// Log summary of pods found
	h.logger.Info("pod eviction summary",
		"node", nodeName,
		"pods_to_evict", len(podsToEvict),
		"skipped_terminating", skippedPods["terminating"],
		"skipped_completed", skippedPods["completed"],
		"skipped_daemonset", skippedPods["daemonset"],
		"skipped_static", skippedPods["static"],
		"handler", h.Name())

	if len(podsToEvict) == 0 {
		h.logger.Info("no pods to evict", "node", nodeName, "handler", h.Name())
		return nil
	}

	// Evict all pods in parallel with shared context timeout
	return h.evictPodsInParallel(ctx, podsToEvict, nodeName)
}

// evictPodsInParallel evicts multiple pods in parallel and waits for all to complete
func (h *KubernetesHandler) evictPodsInParallel(ctx context.Context, podsToEvict []corev1.Pod, nodeName string) error {
	h.logger.Info("starting parallel pod eviction", "node", nodeName, "pod_count", len(podsToEvict), "handler", h.Name())

	// Use sync package for coordination
	var wg sync.WaitGroup
	var mu sync.Mutex
	var evictionErrors []error
	successCount := 0

	// Start eviction for each pod in parallel
	for _, pod := range podsToEvict {
		wg.Add(1)
		go func(p corev1.Pod) {
			defer wg.Done()

			err := h.evictPod(ctx, &p)

			mu.Lock()
			if err != nil {
				evictionErrors = append(evictionErrors, fmt.Errorf("pod %s/%s: %w", p.Namespace, p.Name, err))
			} else {
				successCount++
			}
			mu.Unlock()
		}(pod)
	}

	// Wait for all evictions to complete
	wg.Wait()

	h.logger.Info("parallel pod eviction completed",
		"node", nodeName,
		"total_pods", len(podsToEvict),
		"successful_evictions", successCount,
		"failed_evictions", len(evictionErrors),
		"handler", h.Name())

	// If we have any errors, log them but don't fail the entire operation
	// In emergency situations, partial success is better than total failure
	if len(evictionErrors) > 0 {
		for _, err := range evictionErrors {
			h.logger.Error("pod eviction error", "error", err, "handler", h.Name())
		}

		// Only fail if more than half the pods failed to evict
		if len(evictionErrors) > len(podsToEvict)/2 {
			return fmt.Errorf("failed to evict majority of pods (%d/%d failed)", len(evictionErrors), len(podsToEvict))
		}
	}

	h.logger.Info("node drain completed successfully", "node", nodeName, "evicted_pods", successCount, "handler", h.Name())
	return nil
}

// isDaemonSetPod checks if a pod is managed by a DaemonSet
func (h *KubernetesHandler) isDaemonSetPod(pod *corev1.Pod) bool {
	for _, ownerRef := range pod.OwnerReferences {
		if ownerRef.Kind == "DaemonSet" {
			return true
		}
	}
	return false
}

// isStaticPod checks if a pod is a static pod
func (h *KubernetesHandler) isStaticPod(pod *corev1.Pod) bool {
	for _, ownerRef := range pod.OwnerReferences {
		if ownerRef.Kind == "Node" {
			return true
		}
	}
	// Also check for static pod annotation
	if _, exists := pod.Annotations["kubernetes.io/config.source"]; exists {
		return true
	}
	return false
}

// evictPod evicts a pod using the eviction API with a short timeout for emergency situations
func (h *KubernetesHandler) evictPod(ctx context.Context, pod *corev1.Pod) error {
	h.logger.Info("evicting pod",
		"pod", pod.Name,
		"namespace", pod.Namespace,
		"node", pod.Spec.NodeName,
		"handler", h.Name())

	eviction := &policyv1.Eviction{
		ObjectMeta: v1.ObjectMeta{
			Name:      pod.Name,
			Namespace: pod.Namespace,
		},
	}

	// Try to evict the pod
	err := h.RestConfig.PolicyV1().Evictions(pod.Namespace).Evict(ctx, eviction)
	if err != nil {
		h.logger.Error("pod eviction failed",
			"pod", pod.Name,
			"namespace", pod.Namespace,
			"error", err,
			"handler", h.Name())
		return fmt.Errorf("eviction failed: %w", err)
	}

	h.logger.Debug("pod eviction request sent, waiting for deletion",
		"pod", pod.Name,
		"namespace", pod.Namespace,
		"handler", h.Name())

	// Use a much shorter timeout for emergency drain situations
	// 5 seconds should be enough for most pods to start terminating
	timeout := time.NewTimer(5 * time.Second)
	defer timeout.Stop()

	ticker := time.NewTicker(500 * time.Millisecond) // Check more frequently
	defer ticker.Stop()

	for {
		select {
		case <-timeout.C:
			h.logger.Warn("timeout waiting for pod deletion, pod may still be terminating",
				"pod", pod.Name,
				"namespace", pod.Namespace,
				"timeout", "5s",
				"handler", h.Name())
			// Don't return error - pod might still be terminating gracefully
			// This allows the drain to continue with other pods
			return nil
		case <-ticker.C:
			_, err := h.RestConfig.CoreV1().Pods(pod.Namespace).Get(ctx, pod.Name, v1.GetOptions{})
			if err != nil {
				// Pod is deleted (not found error is expected)
				h.logger.Info("pod successfully evicted and deleted",
					"pod", pod.Name,
					"namespace", pod.Namespace,
					"handler", h.Name())
				return nil
			}
		case <-ctx.Done():
			h.logger.Error("context cancelled while waiting for pod deletion",
				"pod", pod.Name,
				"namespace", pod.Namespace,
				"handler", h.Name())
			return ctx.Err()
		}
	}
}
