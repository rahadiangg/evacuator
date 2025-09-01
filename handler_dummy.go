package evacuator

import (
	"context"
	"log/slog"
)

type DummyHandler struct {
	config DummyHandlerConfig
}

type DummyHandlerConfig struct {
	Logger *slog.Logger
}

func NewDummyHandler(config *DummyHandlerConfig) *DummyHandler {
	return &DummyHandler{
		config: *config,
	}
}

func (h *DummyHandler) HandleTermination(ctx context.Context, event TerminationEvent) error {
	h.config.Logger.Info("dummy handler fired",
		"hostname", event.Hostname,
		"private_ip", event.PrivateIP,
		"instance_id", event.InstanceID,
		"reason", event.Reason,
	)
	return nil
}

func (h *DummyHandler) Name() string {
	return "dummy"
}
