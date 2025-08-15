package cloud

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"time"
)

// NodeMonitoringService manages termination event monitoring for a single node (DaemonSet deployment)
type NodeMonitoringService struct {
	registry      CloudProviderRegistry
	eventHandlers []EventHandler
	logger        *slog.Logger

	// Provider configuration
	manualProvider string
	autoDetect     bool

	// Monitoring state
	mu              sync.RWMutex
	isRunning       bool
	cancelFunc      context.CancelFunc
	currentProvider CloudProvider
	eventChannel    <-chan TerminationEvent
	nodeName        string
}

// NodeMonitoringConfig contains configuration for single-node monitoring
type NodeMonitoringConfig struct {
	// NodeName is the Kubernetes node name (usually from NODE_NAME env var)
	NodeName string

	// EventBufferSize is the buffer size for event channels
	EventBufferSize int

	// Logger is the logger instance to use
	Logger *slog.Logger

	// Provider specifies which cloud provider to use (manual selection)
	// If empty, auto-detection will be used
	Provider string

	// AutoDetect enables automatic detection when Provider is empty
	AutoDetect bool
}

// NewNodeMonitoringService creates a new monitoring service for single-node operation
func NewNodeMonitoringService(registry CloudProviderRegistry, config NodeMonitoringConfig) *NodeMonitoringService {
	logger := config.Logger
	if logger == nil {
		logger = slog.Default()
	}

	nodeName := config.NodeName
	if nodeName == "" {
		// Try to get from NODE_NAME environment variable
		nodeName = os.Getenv("NODE_NAME")
	}

	return &NodeMonitoringService{
		registry:       registry,
		logger:         logger,
		nodeName:       nodeName,
		manualProvider: config.Provider,
		autoDetect:     config.AutoDetect,
	}
}

// AddEventHandler adds an event handler to process termination events
func (ms *NodeMonitoringService) AddEventHandler(handler EventHandler) {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	ms.eventHandlers = append(ms.eventHandlers, handler)
	ms.logger.Info("Added event handler", "handler", handler.Name())
}

// Start begins monitoring for termination events on the current node
func (ms *NodeMonitoringService) Start(ctx context.Context) error {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	if ms.isRunning {
		return fmt.Errorf("monitoring service is already running")
	}

	// Detect or select the cloud provider
	var provider CloudProvider
	var err error

	if ms.manualProvider != "" {
		// Manual provider selection
		ms.logger.Info("Using manually specified provider", "provider", ms.manualProvider)
		provider, err = ms.registry.GetProvider(ms.manualProvider)
		if err != nil {
			return fmt.Errorf("failed to get specified provider %s: %w", ms.manualProvider, err)
		}

		// Optionally validate that the provider is supported in current environment
		if !provider.IsSupported(ctx) {
			ms.logger.Warn("Manually specified provider is not supported in current environment",
				"provider", ms.manualProvider,
				"note", "Proceeding anyway as requested")
		}
	} else if ms.autoDetect {
		// Auto-detection
		ms.logger.Info("Auto-detecting cloud provider...")
		provider, err = ms.registry.DetectCurrentProvider(ctx)
		if err != nil {
			return fmt.Errorf("failed to detect cloud provider: %w", err)
		}
		ms.logger.Info("Detected cloud provider", "provider", provider.Name())
	} else {
		return fmt.Errorf("no provider specified and auto-detection is disabled")
	}

	// Get current node information
	nodeInfo, err := provider.GetNodeInfo(ctx)
	if err != nil {
		return fmt.Errorf("failed to get node info: %w", err)
	}

	// Use the node name from provider if not set
	if ms.nodeName == "" {
		ms.nodeName = nodeInfo.NodeName
	}

	ms.logger.Info("Monitoring node",
		"node_name", ms.nodeName,
		"node_id", nodeInfo.NodeID,
		"cloud_provider", nodeInfo.CloudProvider,
		"region", nodeInfo.Region,
		"zone", nodeInfo.Zone,
		"instance_type", nodeInfo.InstanceType,
		"is_spot_instance", nodeInfo.IsSpotInstance,
	)

	// Create a cancellable context
	monitorCtx, cancel := context.WithCancel(ctx)
	ms.cancelFunc = cancel

	// Start monitoring
	eventChan, err := provider.StartMonitoring(monitorCtx)
	if err != nil {
		cancel()
		return fmt.Errorf("failed to start monitoring: %w", err)
	}

	ms.currentProvider = provider
	ms.eventChannel = eventChan
	ms.isRunning = true

	// Start event processing goroutine
	go ms.processEvents(monitorCtx)

	ms.logger.Info("Node monitoring started successfully",
		"provider", provider.Name(),
		"node_name", ms.nodeName,
	)

	return nil
}

// Stop stops the monitoring service
func (ms *NodeMonitoringService) Stop() error {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	if !ms.isRunning {
		return fmt.Errorf("monitoring service is not running")
	}

	// Cancel the context to stop monitoring
	if ms.cancelFunc != nil {
		ms.cancelFunc()
	}

	// Stop the provider
	var lastError error
	if ms.currentProvider != nil {
		if err := ms.currentProvider.StopMonitoring(); err != nil {
			ms.logger.Error("Failed to stop monitoring for provider",
				"provider", ms.currentProvider.Name(), "error", err)
			lastError = err
		} else {
			ms.logger.Info("Stopped monitoring", "provider", ms.currentProvider.Name())
		}
	}

	// Clear state
	ms.currentProvider = nil
	ms.eventChannel = nil
	ms.isRunning = false
	ms.cancelFunc = nil

	ms.logger.Info("Node monitoring stopped")

	return lastError
}

// IsRunning returns true if the monitoring service is running
func (ms *NodeMonitoringService) IsRunning() bool {
	ms.mu.RLock()
	defer ms.mu.RUnlock()

	return ms.isRunning
}

// GetCurrentProvider returns the currently active cloud provider
func (ms *NodeMonitoringService) GetCurrentProvider() string {
	ms.mu.RLock()
	defer ms.mu.RUnlock()

	if ms.currentProvider != nil {
		return ms.currentProvider.Name()
	}
	return ""
}

// GetNodeName returns the current node name
func (ms *NodeMonitoringService) GetNodeName() string {
	ms.mu.RLock()
	defer ms.mu.RUnlock()

	return ms.nodeName
}

// processEvents processes termination events from the cloud provider
func (ms *NodeMonitoringService) processEvents(ctx context.Context) {
	ms.logger.Info("Started event processing", "node_name", ms.nodeName)

	for {
		select {
		case <-ctx.Done():
			ms.logger.Info("Event processing stopped due to context cancellation")
			return

		case event, ok := <-ms.eventChannel:
			if !ok {
				ms.logger.Info("Event channel closed")
				return
			}

			// Ensure the event is for this node
			if event.NodeName != ms.nodeName && event.NodeID != ms.nodeName {
				ms.logger.Warn("Received event for different node, ignoring",
					"event_node_name", event.NodeName,
					"event_node_id", event.NodeID,
					"current_node", ms.nodeName)
				continue
			}

			ms.logger.Warn("Received termination event for this node",
				"node_name", event.NodeName,
				"node_id", event.NodeID,
				"reason", event.Reason,
				"termination_time", event.TerminationTime,
				"grace_period", event.GracePeriod)

			// Process the event with all handlers
			ms.handleEvent(ctx, event)
		}
	}
}

// handleEvent processes a termination event with all registered handlers
func (ms *NodeMonitoringService) handleEvent(ctx context.Context, event TerminationEvent) {
	ms.mu.RLock()
	handlers := make([]EventHandler, len(ms.eventHandlers))
	copy(handlers, ms.eventHandlers)
	ms.mu.RUnlock()

	ms.logger.Info("Processing termination event with handlers",
		"handler_count", len(handlers),
		"node_name", event.NodeName,
		"reason", event.Reason)

	for _, handler := range handlers {
		// Create a timeout context for each handler
		handlerCtx, cancel := context.WithTimeout(ctx, 30*time.Second)

		go func(h EventHandler) {
			defer cancel()

			startTime := time.Now()
			if err := h.HandleTerminationEvent(handlerCtx, event); err != nil {
				ms.logger.Error("Event handler failed",
					"handler", h.Name(),
					"node_name", event.NodeName,
					"duration", time.Since(startTime),
					"error", err)
			} else {
				ms.logger.Info("Event handler completed successfully",
					"handler", h.Name(),
					"node_name", event.NodeName,
					"duration", time.Since(startTime))
			}
		}(handler)
	}
}
