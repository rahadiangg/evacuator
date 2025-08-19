package evacuator

import (
	"context"
	"log/slog"
)

type LogHandler struct {
	logger *slog.Logger
}

func NewLogHandler(logger *slog.Logger) *LogHandler {
	return &LogHandler{
		logger: logger,
	}
}

func (h *LogHandler) HandleTermination(ctx context.Context, event TerminationEvent) error {
	h.logger.Info("log handler fired",
		"hostname", event.Hostname,
		"private_ip", event.PrivateIP,
		"instance_id", event.InstanceID,
		"reason", event.Reason,
	)
	return nil
}

func (h *LogHandler) Name() string {
	return "log"
}
