package evacuator

import (
	"errors"
	"log/slog"
)

type TelegramHandler struct {
	logger *slog.Logger
}

func NewTelegramHandler(logger *slog.Logger) *TelegramHandler {
	return &TelegramHandler{
		logger: logger,
	}
}

func (h *TelegramHandler) HandleTermination(e <-chan TerminationEvent) {
	// Receive the single termination event
	event := <-e

	// Validate termination event
	if err := h.validateTerminationEvent(event); err != nil {
		h.logger.Error("invalid termination event received", "error", err.Error())
		return
	}

	h.logger.Info("telegram handler processing termination event",
		"hostname", event.Hostname,
		"instance_id", event.InstanceID,
		"reason", event.Reason)
}

// validateTerminationEvent validates that a termination event has required fields.
func (h *TelegramHandler) validateTerminationEvent(event TerminationEvent) error {
	if event.Hostname == "" {
		return errors.New("hostname cannot be empty")
	}
	if event.InstanceID == "" {
		return errors.New("instance ID cannot be empty")
	}
	if event.Reason == "" {
		return errors.New("termination reason cannot be empty")
	}
	return nil
}
