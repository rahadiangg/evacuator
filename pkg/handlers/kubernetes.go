package handlers

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/rahadiangg/evacuator/pkg/cloud"
)

// KubernetesHandler handles node evacuation in Kubernetes clusters
type KubernetesHandler struct {
	logger              *slog.Logger
	client              kubernetes.Interface
	drainTimeoutSeconds int
	forceEvictionAfter  time.Duration
	skipDaemonSets      bool
	deleteEmptyDirData  bool
	ignorePodDisruption bool
	gracePeriodSeconds  int
	nodeName            string
}

// KubernetesConfig contains configuration for the Kubernetes handler
type KubernetesConfig struct {
	// KubeConfig is the path to kubeconfig file (empty for in-cluster config)
	KubeConfig string

	// InCluster indicates if running inside a Kubernetes cluster
	InCluster bool

	// NodeName is the current node name (auto-detected from NODE_NAME env var if empty)
	NodeName string

	// DrainTimeoutSeconds is the timeout for draining nodes (default: 300)
	DrainTimeoutSeconds int

	// ForceEvictionAfter is the duration after which to force evict pods (default: 5 minutes)
	ForceEvictionAfter time.Duration

	// SkipDaemonSets ignores DaemonSet-managed pods during drain (default: true)
	SkipDaemonSets bool

	// DeleteEmptyDirData deletes local data in empty dir volumes (default: false)
	DeleteEmptyDirData bool

	// IgnorePodDisruption ignores pod disruption budgets (default: false)
	IgnorePodDisruption bool

	// GracePeriodSeconds is the grace period for pod termination (default: 30)
	GracePeriodSeconds int

	// Logger is the logger instance
	Logger *slog.Logger
}

// NewKubernetesHandler creates a new Kubernetes handler
func NewKubernetesHandler(config KubernetesConfig) (*KubernetesHandler, error) {
	if config.Logger == nil {
		config.Logger = slog.Default()
	}

	if config.DrainTimeoutSeconds == 0 {
		config.DrainTimeoutSeconds = 300
	}

	if config.ForceEvictionAfter == 0 {
		config.ForceEvictionAfter = 5 * time.Minute
	}

	if config.GracePeriodSeconds == 0 {
		config.GracePeriodSeconds = 30
	}

	// Auto-detect node name from environment if not provided
	nodeName := config.NodeName
	if nodeName == "" {
		nodeName = os.Getenv("NODE_NAME")
		if nodeName == "" {
			// Fallback to hostname if NODE_NAME is not set
			if hostname, err := os.Hostname(); err == nil {
				nodeName = hostname
			} else {
				return nil, fmt.Errorf("unable to determine node name: NODE_NAME env var not set and hostname lookup failed: %w", err)
			}
		}
	}

	// Create Kubernetes client configuration
	var kubeConfig *rest.Config
	var err error

	if config.InCluster {
		// Use in-cluster configuration (service account)
		kubeConfig, err = rest.InClusterConfig()
		if err != nil {
			return nil, fmt.Errorf("failed to create in-cluster config: %w", err)
		}
		config.Logger.Info("[handlers.kubernetes] using in-cluster kubernetes configuration")
	} else {
		// Use kubeconfig file
		kubeconfigPath := config.KubeConfig
		if kubeconfigPath == "" {
			// Try default locations
			kubeconfigPath = clientcmd.RecommendedHomeFile
		}

		kubeConfig, err = clientcmd.BuildConfigFromFlags("", kubeconfigPath)
		if err != nil {
			return nil, fmt.Errorf("failed to build kubeconfig from %s: %w", kubeconfigPath, err)
		}
		config.Logger.Info("[handlers.kubernetes] using kubeconfig file", "path", kubeconfigPath)
	}

	// Create Kubernetes client
	client, err := kubernetes.NewForConfig(kubeConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err = client.CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to verify node %s exists: %w", nodeName, err)
	}

	config.Logger.Info("[handlers.kubernetes] successfully connected to kubernetes cluster", "node_name", nodeName)

	return &KubernetesHandler{
		logger:              config.Logger,
		client:              client,
		drainTimeoutSeconds: config.DrainTimeoutSeconds,
		forceEvictionAfter:  config.ForceEvictionAfter,
		skipDaemonSets:      config.SkipDaemonSets,
		deleteEmptyDirData:  config.DeleteEmptyDirData,
		ignorePodDisruption: config.IgnorePodDisruption,
		gracePeriodSeconds:  config.GracePeriodSeconds,
		nodeName:            nodeName,
	}, nil
}

// Name returns the handler name
func (h *KubernetesHandler) Name() string {
	return "kubernetes-handler"
}

// HandleTerminationEvent handles node termination by draining the Kubernetes node
func (h *KubernetesHandler) HandleTerminationEvent(ctx context.Context, event cloud.TerminationEvent) error {
	h.logger.Info("[handlers.kubernetes] handling node termination event for kubernetes",
		"node_id", event.NodeID,
		"node_name", event.NodeName,
		"termination_time", event.TerminationTime,
		"grace_period", event.GracePeriod,
	)

	// Use the node name from the handler (which was auto-detected or configured)
	nodeName := h.nodeName
	if event.NodeName != "" {
		nodeName = event.NodeName
	}

	// Calculate available time for draining
	availableTime := event.GracePeriod
	if availableTime <= 0 {
		h.logger.Warn("[handlers.kubernetes] no grace period available, performing emergency drain")
		availableTime = 30 * time.Second
	}

	// Reserve some time for cleanup and buffer
	drainDeadline := time.Now().Add(availableTime - 30*time.Second)
	if drainDeadline.Before(time.Now()) {
		drainDeadline = time.Now().Add(30 * time.Second)
	}

	drainCtx, cancel := context.WithDeadline(ctx, drainDeadline)
	defer cancel()

	// Step 1: Cordon the node (mark as unschedulable)
	if err := h.cordonNode(drainCtx, nodeName); err != nil {
		h.logger.Error("[handlers.kubernetes] failed to cordon node", "node_name", nodeName, "error", err)
		return fmt.Errorf("failed to cordon node: %w", err)
	}

	h.logger.Info("[handlers.kubernetes] successfully cordoned node", "node_name", nodeName)

	// Step 2: Drain the node (evict all pods)
	if err := h.drainNode(drainCtx, nodeName); err != nil {
		h.logger.Error("[handlers.kubernetes] failed to drain node", "node_name", nodeName, "error", err)
		// Continue with force eviction if normal drain fails
		if err := h.forceEvictPods(drainCtx, nodeName); err != nil {
			h.logger.Error("[handlers.kubernetes] failed to force evict pods", "node_name", nodeName, "error", err)
			return fmt.Errorf("failed to evacuate node: %w", err)
		}
	}

	h.logger.Info("[handlers.kubernetes] successfully drained node", "node_name", nodeName)

	// Step 3: Mark node as terminated (add taint/label)
	if err := h.markNodeTerminated(drainCtx, event); err != nil {
		h.logger.Warn("[handlers.kubernetes] failed to mark node as terminated", "node_name", nodeName, "error", err)
		// This is not critical, so we don't fail the operation
	}

	h.logger.Info("[handlers.kubernetes] node evacuation completed successfully", "node_name", nodeName)

	return nil
}

// cordonNode marks the node as unschedulable
func (h *KubernetesHandler) cordonNode(ctx context.Context, nodeName string) error {
	h.logger.Debug("[handlers.kubernetes] cordoning node", "node_name", nodeName)

	// Get the current node
	node, err := h.client.CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get node %s: %w", nodeName, err)
	}

	// Check if already cordoned
	if node.Spec.Unschedulable {
		h.logger.Debug("[handlers.kubernetes] node is already cordoned", "node_name", nodeName)
		return nil
	}

	// Create a patch to set unschedulable
	patch := []byte(`{"spec":{"unschedulable":true}}`)

	// Apply the patch
	_, err = h.client.CoreV1().Nodes().Patch(ctx, nodeName, types.MergePatchType, patch, metav1.PatchOptions{})
	if err != nil {
		return fmt.Errorf("failed to cordon node %s: %w", nodeName, err)
	}

	h.logger.Info("[handlers.kubernetes] successfully cordoned node", "node_name", nodeName)
	return nil
}

// drainNode evicts all pods from the node
func (h *KubernetesHandler) drainNode(ctx context.Context, nodeName string) error {
	h.logger.Debug("[handlers.kubernetes] draining node", "node_name", nodeName)

	// List all pods on the node
	pods, err := h.getPodList(ctx, nodeName)
	if err != nil {
		return fmt.Errorf("failed to list pods on node %s: %w", nodeName, err)
	}

	if len(pods) == 0 {
		h.logger.Info("[handlers.kubernetes] no pods to evict on node", "node_name", nodeName)
		return nil
	}

	h.logger.Info("[handlers.kubernetes] found pods to evict", "node_name", nodeName, "pod_count", len(pods))

	// Filter pods that should be evicted
	podsToEvict := h.filterPodsForEviction(pods)

	if len(podsToEvict) == 0 {
		h.logger.Info("[handlers.kubernetes] no pods need eviction after filtering", "node_name", nodeName)
		return nil
	}

	h.logger.Info("[handlers.kubernetes] evicting pods", "node_name", nodeName, "pods_to_evict", len(podsToEvict))

	// Evict pods
	evictionErrors := make([]error, 0)
	for _, pod := range podsToEvict {
		if err := h.evictPod(ctx, pod); err != nil {
			h.logger.Warn("[handlers.kubernetes] failed to evict pod", "pod", fmt.Sprintf("%s/%s", pod.Namespace, pod.Name), "error", err)
			evictionErrors = append(evictionErrors, err)
		}
	}

	// Wait for pods to be evicted
	if err := h.waitForPodEviction(ctx, podsToEvict); err != nil {
		return fmt.Errorf("failed waiting for pod eviction: %w", err)
	}

	if len(evictionErrors) > 0 {
		h.logger.Warn("[handlers.kubernetes] some pods failed to evict gracefully", "error_count", len(evictionErrors))
		return fmt.Errorf("failed to evict %d pods", len(evictionErrors))
	}

	h.logger.Info("[handlers.kubernetes] successfully evicted all pods", "node_name", nodeName)
	return nil
}

// forceEvictPods performs forced eviction when normal drain fails
func (h *KubernetesHandler) forceEvictPods(ctx context.Context, nodeName string) error {
	h.logger.Warn("[handlers.kubernetes] performing force eviction", "node_name", nodeName)

	// List remaining pods on the node
	pods, err := h.getPodList(ctx, nodeName)
	if err != nil {
		return fmt.Errorf("failed to list remaining pods: %w", err)
	}

	// Filter pods that should be evicted
	podsToEvict := h.filterPodsForEviction(pods)

	if len(podsToEvict) == 0 {
		h.logger.Info("[handlers.kubernetes] no pods remaining to force evict", "node_name", nodeName)
		return nil
	}

	h.logger.Info("[handlers.kubernetes] force evicting pods", "node_name", nodeName, "pods_to_evict", len(podsToEvict))

	// Force delete pods with grace period 0
	gracePeriodSeconds := int64(0)
	deleteOptions := metav1.DeleteOptions{
		GracePeriodSeconds: &gracePeriodSeconds,
	}

	for _, pod := range podsToEvict {
		err := h.client.CoreV1().Pods(pod.Namespace).Delete(ctx, pod.Name, deleteOptions)
		if err != nil && !apierrors.IsNotFound(err) {
			h.logger.Warn("[handlers.kubernetes] failed to force delete pod", "pod", fmt.Sprintf("%s/%s", pod.Namespace, pod.Name), "error", err)
		} else {
			h.logger.Debug("[handlers.kubernetes] force deleted pod", "pod", fmt.Sprintf("%s/%s", pod.Namespace, pod.Name))
		}
	}

	h.logger.Info("[handlers.kubernetes] completed force eviction", "node_name", nodeName)
	return nil
}

// markNodeTerminated adds labels/taints to indicate node termination
func (h *KubernetesHandler) markNodeTerminated(ctx context.Context, event cloud.TerminationEvent) error {
	nodeName := h.nodeName
	if event.NodeName != "" {
		nodeName = event.NodeName
	}

	h.logger.Debug("[handlers.kubernetes] marking node as terminated", "node_name", nodeName)

	// Create labels and taints for termination
	labels := map[string]string{
		"evacuator.io/termination-reason":  string(event.Reason),
		"evacuator.io/termination-time":    event.TerminationTime.Format(time.RFC3339),
		"evacuator.io/cloud-provider":      event.CloudProvider,
		"evacuator.io/termination-node-id": event.NodeID,
	}

	taints := []corev1.Taint{
		{
			Key:    "evacuator.io/terminating",
			Value:  "true",
			Effect: corev1.TaintEffectNoSchedule,
		},
		{
			Key:    "evacuator.io/terminating",
			Value:  "true",
			Effect: corev1.TaintEffectNoExecute,
		},
	}

	// Get current node
	node, err := h.client.CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get node %s: %w", nodeName, err)
	}

	// Add labels
	if node.Labels == nil {
		node.Labels = make(map[string]string)
	}
	for k, v := range labels {
		node.Labels[k] = v
	}

	// Add taints
	for _, newTaint := range taints {
		// Check if taint already exists
		found := false
		for i, existingTaint := range node.Spec.Taints {
			if existingTaint.Key == newTaint.Key && existingTaint.Effect == newTaint.Effect {
				// Update existing taint
				node.Spec.Taints[i] = newTaint
				found = true
				break
			}
		}
		if !found {
			// Add new taint
			node.Spec.Taints = append(node.Spec.Taints, newTaint)
		}
	}

	// Update the node
	_, err = h.client.CoreV1().Nodes().Update(ctx, node, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to update node %s with termination labels/taints: %w", nodeName, err)
	}

	h.logger.Info("[handlers.kubernetes] successfully marked node as terminated", "node_name", nodeName)
	return nil
}

// getPodList lists all pods on a specific node from all namespaces
func (h *KubernetesHandler) getPodList(ctx context.Context, nodeName string) ([]corev1.Pod, error) {
	// Use field selector to get pods on specific node
	fieldSelector := fields.SelectorFromSet(fields.Set{"spec.nodeName": nodeName})

	h.logger.Debug("[handlers.kubernetes] listing pods from all namespaces", "node_name", nodeName)

	// Use empty namespace to list pods from all namespaces
	podList, err := h.client.CoreV1().Pods("").List(ctx, metav1.ListOptions{
		FieldSelector: fieldSelector.String(),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list pods: %w", err)
	}

	return podList.Items, nil
}

// filterPodsForEviction filters pods that should be evicted
func (h *KubernetesHandler) filterPodsForEviction(pods []corev1.Pod) []corev1.Pod {
	var podsToEvict []corev1.Pod

	for _, pod := range pods {
		// Skip pods that are already terminated or terminating
		if pod.DeletionTimestamp != nil {
			continue
		}

		// Skip completed pods
		if pod.Status.Phase == corev1.PodSucceeded || pod.Status.Phase == corev1.PodFailed {
			continue
		}

		// Skip mirror pods (static pods)
		if _, exists := pod.Annotations[corev1.MirrorPodAnnotationKey]; exists {
			h.logger.Debug("[handlers.kubernetes] skipping mirror pod", "pod", fmt.Sprintf("%s/%s", pod.Namespace, pod.Name))
			continue
		}

		// Skip DaemonSet pods if configured
		if h.skipDaemonSets && h.isDaemonSetPod(pod) {
			h.logger.Debug("[handlers.kubernetes] skipping daemonset pod", "pod", fmt.Sprintf("%s/%s", pod.Namespace, pod.Name))
			continue
		}

		// Skip kube-system critical pods unless forced
		if h.isCriticalSystemPod(pod) {
			h.logger.Debug("[handlers.kubernetes] skipping critical system pod", "pod", fmt.Sprintf("%s/%s", pod.Namespace, pod.Name))
			continue
		}

		podsToEvict = append(podsToEvict, pod)
	}

	return podsToEvict
}

// isDaemonSetPod checks if a pod is managed by a DaemonSet
func (h *KubernetesHandler) isDaemonSetPod(pod corev1.Pod) bool {
	for _, owner := range pod.OwnerReferences {
		if owner.Kind == "DaemonSet" {
			return true
		}
	}
	return false
}

// isCriticalSystemPod checks if a pod is a critical system pod
func (h *KubernetesHandler) isCriticalSystemPod(pod corev1.Pod) bool {
	// Check for critical system namespaces
	if pod.Namespace == "kube-system" || pod.Namespace == "kube-public" {
		// Check for critical pod annotations
		if priority, exists := pod.Annotations["scheduler.alpha.kubernetes.io/critical-pod"]; exists && priority == "" {
			return true
		}

		// Check for system critical priority class
		if pod.Spec.PriorityClassName == "system-cluster-critical" || pod.Spec.PriorityClassName == "system-node-critical" {
			return true
		}

		// Check for common critical system pods
		criticalPods := []string{"kube-proxy", "kube-dns", "coredns", "calico", "flannel", "weave"}
		for _, critical := range criticalPods {
			if strings.Contains(pod.Name, critical) {
				return true
			}
		}
	}

	return false
}

// evictPod evicts a specific pod using the eviction API
func (h *KubernetesHandler) evictPod(ctx context.Context, pod corev1.Pod) error {
	// Create eviction object
	eviction := &policyv1.Eviction{
		ObjectMeta: metav1.ObjectMeta{
			Name:      pod.Name,
			Namespace: pod.Namespace,
		},
	}

	// Attempt eviction using the eviction API
	err := h.client.CoreV1().Pods(pod.Namespace).EvictV1(ctx, eviction)
	if err != nil {
		// If eviction fails due to PDB and we're ignoring PDB, try direct deletion
		if apierrors.IsTooManyRequests(err) && h.ignorePodDisruption {
			h.logger.Debug("[handlers.kubernetes] eviction blocked by pdb, ignoring and deleting directly", "pod", fmt.Sprintf("%s/%s", pod.Namespace, pod.Name))
			gracePeriodSeconds := int64(h.gracePeriodSeconds)
			deleteOptions := metav1.DeleteOptions{
				GracePeriodSeconds: &gracePeriodSeconds,
			}
			return h.client.CoreV1().Pods(pod.Namespace).Delete(ctx, pod.Name, deleteOptions)
		}
		return fmt.Errorf("failed to evict pod %s/%s: %w", pod.Namespace, pod.Name, err)
	}

	h.logger.Debug("[handlers.kubernetes] successfully evicted pod", "pod", fmt.Sprintf("%s/%s", pod.Namespace, pod.Name))
	return nil
}

// waitForPodEviction waits for pods to be evicted
func (h *KubernetesHandler) waitForPodEviction(ctx context.Context, pods []corev1.Pod) error {
	if len(pods) == 0 {
		return nil
	}

	h.logger.Debug("[handlers.kubernetes] waiting for pod eviction", "pod_count", len(pods))

	// Create a map for quick lookup
	podMap := make(map[string]bool)
	for _, pod := range pods {
		podMap[fmt.Sprintf("%s/%s", pod.Namespace, pod.Name)] = true
	}

	// Wait for pods to be terminated
	return wait.PollUntilContextTimeout(ctx, 2*time.Second, time.Duration(h.drainTimeoutSeconds)*time.Second, true, func(ctx context.Context) (bool, error) {
		remainingPods := 0

		for podKey := range podMap {
			parts := strings.Split(podKey, "/")
			if len(parts) != 2 {
				continue
			}
			namespace, name := parts[0], parts[1]

			// Check if pod still exists
			_, err := h.client.CoreV1().Pods(namespace).Get(ctx, name, metav1.GetOptions{})
			if err != nil {
				if apierrors.IsNotFound(err) {
					// Pod is gone, remove from map
					delete(podMap, podKey)
					continue
				}
				// Some other error, count as remaining
				h.logger.Debug("[handlers.kubernetes] error checking pod status", "pod", podKey, "error", err)
			}

			remainingPods++
		}

		h.logger.Debug("[handlers.kubernetes] waiting for pod eviction", "remaining_pods", remainingPods)

		if remainingPods == 0 {
			h.logger.Info("[handlers.kubernetes] all pods evicted successfully")
			return true, nil
		}

		return false, nil
	})
}
