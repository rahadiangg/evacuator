package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/rahadiangg/evacuator"
)

// HandlerResult represents the result of processing a termination event by a handler
type HandlerResult struct {
	HandlerName string
	Error       error
	ProcessedAt time.Time
}

// Application configuration constants
const (
	// HTTP client timeout for external requests
	HTTPClientTimeout = 10 * time.Second

	// Dummy provider detection wait time for testing
	DummyProviderDetectionWait = 3 * time.Second

	// Handler processing timeout - time allowed for each handler to process termination event
	// Set to 75 seconds to ensure completion within AWS 2-minute spot termination window
	// This allows 33 seconds safety buffer before AWS force-terminates the instance
	HandlerProcessingTimeout = 75 * time.Second

	// Graceful shutdown timeout - maximum time to wait for goroutines to finish
	// Reduced to 10 seconds since handlers should already be complete
	// This is just for final cleanup before AWS termination deadline
	GracefulShutdownTimeout = 10 * time.Second
)

func main() {
	// Create default HTTP client with reasonable timeout
	httpClient := &http.Client{
		Timeout: HTTPClientTimeout,
	}

	// Create default logger with text output to stdout
	logopt := slog.HandlerOptions{
		Level: slog.LevelDebug,
	}
	logger := slog.New(slog.NewTextHandler(os.Stdout, &logopt))

	// Register all available providers
	providers := []evacuator.Provider{
		evacuator.NewAwsProvider(httpClient, logger),
		evacuator.NewAlicloudProvider(httpClient, logger),
		evacuator.NewDummyProvider(logger, DummyProviderDetectionWait),
	}

	// Register all configured handlers
	var handlers []evacuator.Handler

	// Add Kubernetes handler if configured
	kubernetesHandler, err := evacuator.NewKubernetesHandler(&evacuator.KubernetesHandlerConfig{
		InCluster:            true, // Set to false and provide CustomKubeConfigPath if running outside cluster
		CustomKubeConfigPath: "",   // Set path if InCluster is false
	}, logger)
	if err != nil {
		logger.Error("failed to create kubernetes handler", "error", err)
		os.Exit(1)
	}
	handlers = append(handlers, kubernetesHandler)
	logger.Info("kubernetes handler registered")

	// Add Telegram handler if configured
	telegramHandler, err := evacuator.NewTelegramHandler(logger, os.Getenv("TELEGRAM_BOT_TOKEN"), os.Getenv("TELEGRAM_CHAT_ID"))
	if err != nil {
		logger.Error("failed to create telegram handler", "error", err)
		os.Exit(1)
	}
	handlers = append(handlers, telegramHandler)
	logger.Info("telegram handler registered")

	// Detect the current cloud provider environment
	provider := DetectProvider(providers, logger)
	if provider == nil {
		logger.Error("no supported provider detected")
		os.Exit(1)
	}

	// Create root context for coordinated shutdown
	rootCtx, rootCancel := context.WithCancel(context.Background())
	defer rootCancel()

	// Create channel for termination events from provider
	terminationEvent := make(chan evacuator.TerminationEvent)
	var wg sync.WaitGroup

	// Start provider monitoring in background goroutine
	wg.Add(1)
	go func() {
		defer wg.Done()
		provider.StartMonitoring(rootCtx, terminationEvent)
	}()

	// Setup signal handling for graceful shutdown
	shutdownSignal := make(chan os.Signal, 1)
	signal.Notify(shutdownSignal, syscall.SIGINT, syscall.SIGTERM)

	// Start event broadcaster to distribute events to all handlers
	wg.Add(1)
	go func() {
		defer wg.Done()
		broadcastTerminationEvents(rootCtx, terminationEvent, handlers, logger)
	}()

	// Wait for shutdown signal (SIGINT or SIGTERM)
	<-shutdownSignal
	logger.Info("shutdown signal received, stopping gracefully...")

	// Cancel context to signal all goroutines to stop
	rootCancel()

	// Wait for all goroutines to finish with timeout protection
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	// Either all goroutines finish or timeout after configured duration
	select {
	case <-done:
		logger.Info("all goroutines stopped successfully")
	case <-time.After(GracefulShutdownTimeout):
		logger.Warn("timeout waiting for goroutines to stop")
	}

	logger.Info("shutdown complete")
}

// DetectProvider automatically detects which cloud provider is currently running.
// It iterates through all registered providers and returns the first one that
// reports it is supported in the current environment.
func DetectProvider(providers []evacuator.Provider, logger *slog.Logger) evacuator.Provider {

	for _, p := range providers {
		if p.IsSupported() {
			logger.Info("provider detected", "provider_name", p.Name())
			return p
		}
	}

	logger.Debug("no supported provider detected")
	return nil
}

// broadcastTerminationEvents distributes termination events to all handlers.
// This function processes each event through all handlers sequentially and collects results.
func broadcastTerminationEvents(ctx context.Context, terminationEvent <-chan evacuator.TerminationEvent, handlers []evacuator.Handler, logger *slog.Logger) {
	for {
		select {
		case event := <-terminationEvent:
			logger.Info("termination event received, processing through all handlers")

			// Process event through all handlers and collect results
			var handlerWg sync.WaitGroup
			results := make(chan HandlerResult, len(handlers))

			for _, handler := range handlers {
				handlerWg.Add(1)
				go func(h evacuator.Handler) {
					defer handlerWg.Done()

					// Create context with timeout for handler processing
					// Uses HandlerProcessingTimeout constant for consistency
					handlerCtx, cancel := context.WithTimeout(ctx, HandlerProcessingTimeout)
					defer cancel()

					logger.Debug("processing termination event with handler", "handler_name", h.Name())

					err := h.HandleTermination(handlerCtx, event)
					results <- HandlerResult{
						HandlerName: h.Name(),
						Error:       err,
						ProcessedAt: time.Now(),
					}
				}(handler)
			}

			// Wait for all handlers to complete and collect results
			go func() {
				handlerWg.Wait()
				close(results)
			}()

			// Process results
			successCount := 0
			for result := range results {
				if result.Error != nil {
					logger.Error("handler failed to process termination event",
						"handler_name", result.HandlerName,
						"error", result.Error.Error(),
						"processed_at", result.ProcessedAt)
				} else {
					logger.Info("handler successfully processed termination event",
						"handler_name", result.HandlerName,
						"processed_at", result.ProcessedAt)
					successCount++
				}
			}

			logger.Info("termination event processing completed",
				"total_handlers", len(handlers),
				"successful_handlers", successCount,
				"failed_handlers", len(handlers)-successCount)

		case <-ctx.Done():
			logger.Debug("termination event broadcaster stopping")
			return
		}
	}
}
