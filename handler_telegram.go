package evacuator

import (
	"context"
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

func (h *TelegramHandler) Name() string {
	return "telegram"
}

func (h *TelegramHandler) HandleTermination(ctx context.Context, event TerminationEvent) error {
	// Validate termination event first
	if err := h.validateTerminationEvent(event); err != nil {
		h.logger.Error("invalid termination event received", "error", err.Error())
		return err
	}

	h.logger.Info("telegram handler processing termination event",
		"hostname", event.Hostname,
		"instance_id", event.InstanceID,
		"private_ip", event.PrivateIP,
		"reason", event.Reason)

	// TODO: Implement actual Telegram notification logic here
	// For now, just log the successful processing
	h.logger.Info("termination event processed successfully via telegram")

	return nil
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
