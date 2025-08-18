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

func main() {

	// Create default HTTP client
	httpClient := &http.Client{
		Timeout: 10 * time.Second,
	}

	// Create default logger
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	// register providers
	providers := []evacuator.Provider{
		evacuator.NewAwsProvider(httpClient, logger),
		evacuator.NewDummyProvider(httpClient, logger, 3*time.Second),
	}

	// register handlers
	handlers := []evacuator.Handler{
		evacuator.NewTelegramHandler(logger),
	}

	// Detect the current provider
	provider := DetectProvider(providers, logger)
	if provider == nil {
		logger.Error("no supported provider detected")
		os.Exit(1)
	}

	rootCtx, rootCancel := context.WithCancel(context.Background())
	defer rootCancel()

	terminationEvent := make(chan evacuator.TerminationEvent)
	var wg sync.WaitGroup

	// Start monitoring in a goroutine
	wg.Add(1)
	go func() {
		defer wg.Done()
		provider.StartMonitoring(rootCtx, terminationEvent)
	}()

	// Setup graceful shutdown
	shutdownSignal := make(chan os.Signal, 1)
	signal.Notify(shutdownSignal, syscall.SIGINT, syscall.SIGTERM)

	// Start handlers in separate goroutines
	for _, h := range handlers {
		wg.Add(1)
		go func(handler evacuator.Handler) {
			defer wg.Done()
			handler.HandleTermination(terminationEvent)
		}(h)
	}

	// Wait for shutdown signal
	<-shutdownSignal
	logger.Info("shutdown signal received, stopping gracefully...")

	// Cancel context to stop all goroutines
	rootCancel()

	// Wait for all goroutines to finish with timeout
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		logger.Info("all goroutines stopped successfully")
	case <-time.After(5 * time.Second):
		logger.Warn("timeout waiting for goroutines to stop")
	}

	logger.Info("shutdown complete")
}

// DetectProvider automatically detects which cloud provider is currently running
func DetectProvider(providers []evacuator.Provider, logger *slog.Logger) evacuator.Provider {
	for _, p := range providers {
		if p.IsSupported() {
			return p
		}
	}

	logger.Debug("no supported provider detected")
	return nil // No provider detected
}
