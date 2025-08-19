package evacuator

import (
	"context"
	"fmt"
	"log/slog"
	"os"
)

type TerminationEvent struct {
	Hostname   string
	PrivateIP  string
	InstanceID string
	Reason     TerminationReason
}

type TerminationReason string

const (
	TerminationReasonSpot        TerminationReason = "spot termination"
	TerminationReasonMaintenance TerminationReason = "maintenance termination"
)

type Handler interface {
	// Process a single termination event and return any error
	HandleTermination(ctx context.Context, event TerminationEvent) error

	// Get handler name for logging and identification
	Name() string
}

// HandlerRegistry manages the registration and creation of handlers
type HandlerRegistry struct {
	logger *slog.Logger
}

// NewHandlerRegistry creates a new handler registry
func NewHandlerRegistry(logger *slog.Logger) *HandlerRegistry {
	return &HandlerRegistry{
		logger: logger,
	}
}

// RegisterHandlers registers and creates all available handlers
func (r *HandlerRegistry) RegisterHandlers() ([]Handler, error) {
	var handlers []Handler
	var errors []error

	// Register Kubernetes handler
	kubernetesHandler, err := r.createKubernetesHandler()
	if err != nil {
		r.logger.Error("failed to create kubernetes handler", "error", err)
		errors = append(errors, fmt.Errorf("kubernetes handler: %w", err))
	} else {
		handlers = append(handlers, kubernetesHandler)
		r.logger.Info("kubernetes handler registered successfully")
	}

	// Register Telegram handler
	telegramHandler, err := r.createTelegramHandler()
	if err != nil {
		r.logger.Error("failed to create telegram handler", "error", err)
		errors = append(errors, fmt.Errorf("telegram handler: %w", err))
	} else {
		handlers = append(handlers, telegramHandler)
		r.logger.Info("telegram handler registered successfully")
	}

	// Return error if no handlers were registered
	if len(handlers) == 0 {
		return nil, fmt.Errorf("no handlers registered")
	}

	// Log summary
	r.logger.Info("handler registration completed",
		"total_handlers", len(handlers),
		"failed_handlers", len(errors))

	return handlers, nil
}

func (r *HandlerRegistry) createKubernetesHandler() (Handler, error) {
	return NewKubernetesHandler(&KubernetesHandlerConfig{
		InCluster:            true, // Default to in-cluster configuration
		CustomKubeConfigPath: "",   // Can be made configurable later
	}, r.logger)
}

func (r *HandlerRegistry) createTelegramHandler() (Handler, error) {
	// Read from environment variables for now
	botToken := os.Getenv("TELEGRAM_BOT_TOKEN")
	chatID := os.Getenv("TELEGRAM_CHAT_ID")

	return NewTelegramHandler(r.logger, botToken, chatID)
}
