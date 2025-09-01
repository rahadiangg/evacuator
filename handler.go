package evacuator

import (
	"context"
	"fmt"
	"log/slog"
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

type HandlerName string

const (
	HandlerNameLog        HandlerName = "log"
	HandlerNameKubernetes HandlerName = "kubernetes"
	HandlerNameTelegram   HandlerName = "telegram"
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

	// Get global configuration
	handlerConfig := GetHandlerConfig()
	provderConfig := GetProviderConfig()

	// dummy handler registerd when prover is dummy
	if provderConfig.Name == "dummy" {
		logHandler := NewDummyHandler(&DummyHandlerConfig{
			Logger: r.logger,
		})
		handlers = append(handlers, logHandler)
		r.logger.Info("dummy handler registered successfully")
	}

	// Register Kubernetes handler if enabled
	if handlerConfig.Kubernetes.Enabled {
		kubernetesHandler, err := r.createKubernetesHandler()
		if err != nil {
			r.logger.Error("failed to create kubernetes handler", "error", err)
			errors = append(errors, fmt.Errorf("kubernetes handler: %w", err))
		}

		handlers = append(handlers, kubernetesHandler)
		r.logger.Info("kubernetes handler registered successfully")
	}

	// Register Telegram handler if enabled
	if handlerConfig.Telegram.Enabled {
		telegramHandler, err := r.createTelegramHandler()
		if err != nil {
			r.logger.Error("failed to create telegram handler", "error", err)
			errors = append(errors, fmt.Errorf("telegram handler: %w", err))
		}

		handlers = append(handlers, telegramHandler)
		r.logger.Info("telegram handler registered successfully")
	}

	// Register Nomad handler if enabled
	if handlerConfig.Nomad.Enabled {
		nomadHandler, err := r.createNomadHandler()
		if err != nil {
			r.logger.Error("failed to create nomad handler", "error", err)
			errors = append(errors, fmt.Errorf("nomad handler: %w", err))
		}

		handlers = append(handlers, nomadHandler)
		r.logger.Info("nomad handler registered successfully")
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
	handlerConfig := GetHandlerConfig()
	return NewKubernetesHandler(&KubernetesHandlerConfig{
		Logger:             r.logger,
		InCluster:          handlerConfig.Kubernetes.InCluster,
		Kubeconfig:         handlerConfig.Kubernetes.Kubeconfig,
		SkipDaemonSets:     handlerConfig.Kubernetes.SkipDaemonSets,
		DeleteEmptyDirData: handlerConfig.Kubernetes.DeleteEmptyDirData,
	})
}

func (r *HandlerRegistry) createTelegramHandler() (Handler, error) {
	handlerConfig := GetHandlerConfig()

	return NewTelegramHandler(&TelegramHandlerConfig{
		Logger:   r.logger,
		BotToken: handlerConfig.Telegram.BotToken,
		ChatID:   handlerConfig.Telegram.ChatID,
	})
}

func (r *HandlerRegistry) createNomadHandler() (Handler, error) {
	return NewNomadHandler(&NomadHandlerConfig{
		Logger: r.logger,
	})
}
