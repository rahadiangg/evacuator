package evacuator

import (
	"context"
	"log/slog"
)

type DummyHandler struct {
	logger *slog.Logger
}

func NewDummyHandler(logger *slog.Logger) *DummyHandler {
	return &DummyHandler{
		logger: logger,
	}
}

func (h *DummyHandler) HandleTermination(ctx context.Context, event TerminationEvent) error {
	h.logger.Info("dummy handler fired",
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
