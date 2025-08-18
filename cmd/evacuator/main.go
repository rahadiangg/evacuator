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

// Application configuration constants
const (
	// HTTP client timeout for external requests
	HTTPClientTimeout = 10 * time.Second

	// Dummy provider detection wait time for testing
	DummyProviderDetectionWait = 3 * time.Second

	// Graceful shutdown timeout - maximum time to wait for goroutines to finish
	GracefulShutdownTimeout = 5 * time.Second
)

func main() {
	// Create default HTTP client with reasonable timeout
	httpClient := &http.Client{
		Timeout: HTTPClientTimeout,
	}

	// Create default logger with text output to stdout
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	// Register all available providers
	providers := []evacuator.Provider{
		evacuator.NewAwsProvider(httpClient, logger),
		evacuator.NewDummyProvider(httpClient, logger, DummyProviderDetectionWait),
	}

	// Register all configured handlers
	handlers := []evacuator.Handler{
		evacuator.NewTelegramHandler(logger),
	}

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
			return p
		}
	}

	logger.Debug("no supported provider detected")
	return nil
}

// broadcastTerminationEvents distributes termination events to all handlers.
// This function ensures that every registered handler receives every termination
// event by creating individual channels for each handler, preventing race conditions
// and ensuring no handler misses an event.
func broadcastTerminationEvents(ctx context.Context, terminationEvent <-chan evacuator.TerminationEvent, handlers []evacuator.Handler, logger *slog.Logger) {
	for {
		select {
		case event := <-terminationEvent:
			logger.Info("termination event received, broadcasting to all handlers")

			// Create individual channels for each handler to prevent race conditions
			var handlerWg sync.WaitGroup
			for i, handler := range handlers {
				handlerWg.Add(1)
				go func(h evacuator.Handler, handlerIndex int) {
					defer handlerWg.Done()

					// Create a dedicated channel for this handler
					handlerChan := make(chan evacuator.TerminationEvent, 1)
					handlerChan <- event
					close(handlerChan)

					logger.Debug("sending termination event to handler", "handler_index", handlerIndex)
					h.HandleTermination(handlerChan)
				}(handler, i)
			}

			// Wait for all handlers to process the event before continuing
			handlerWg.Wait()
			logger.Info("all handlers completed processing termination event")

		case <-ctx.Done():
			logger.Debug("termination event broadcaster stopping")
			return
		}
	}
}
