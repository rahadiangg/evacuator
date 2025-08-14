package handlers

import (
	"context"
	"log/slog"

	"github.com/rahadiangg/evacuator/pkg/cloud"
)

// LogHandler is a simple event handler that logs termination events
type LogHandler struct {
	logger *slog.Logger
}

// NewLogHandler creates a new log handler
func NewLogHandler(logger *slog.Logger) *LogHandler {
	if logger == nil {
		logger = slog.Default()
	}

	return &LogHandler{
		logger: logger,
	}
}

// Name returns the handler name
func (h *LogHandler) Name() string {
	return "log-handler"
}

// HandleTerminationEvent logs the termination event
func (h *LogHandler) HandleTerminationEvent(ctx context.Context, event cloud.TerminationEvent) error {
	h.logger.Warn("Node termination event received",
		"node_id", event.NodeID,
		"node_name", event.NodeName,
		"reason", event.Reason,
		"cloud_provider", event.CloudProvider,
		"region", event.Region,
		"zone", event.Zone,
		"instance_type", event.InstanceType,
		"termination_time", event.TerminationTime,
		"grace_period", event.GracePeriod,
	)

	return nil
}
