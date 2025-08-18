package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/rahadiangg/evacuator"
)

var providers []evacuator.Provider

func main() {

	// Create default HTTP client
	httpClient := &http.Client{
		Timeout: 10 * time.Second,
	}

	// Create default logger
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	// register providers
	providers = []evacuator.Provider{
		evacuator.NewAwsProvider(httpClient, logger),
		evacuator.NewDummyProvider(httpClient, logger, 3*time.Second),
	}

	// register handlers
	handlers := []evacuator.Handler{
		evacuator.NewTelegramHandler(logger),
	}

	// Detect the current provider
	provider := DetectProvider()
	if provider == nil {
		logger.Error("no supported provider detected")
		os.Exit(1)
	}

	rootCtx, rootCancel := context.WithCancel(context.Background())
	defer rootCancel()

	terminationEvent := make(chan evacuator.TerminationEvent)

	// Start monitoring in a goroutine
	go func() {
		provider.StartMonitoring(rootCtx, terminationEvent)
	}()

	// Setup graceful shutdown
	shutdownSignal := make(chan os.Signal, 1)
	signal.Notify(shutdownSignal, syscall.SIGINT, syscall.SIGTERM)

	go func(handlers []evacuator.Handler) {
		for _, h := range handlers {
			h.HandleTermination(terminationEvent)
		}
	}(handlers)

	// TODO: fix the graceful shutdown
	<-shutdownSignal
	logger.Info("shutting down gracefully")
}

// DetectProvider automatically detects which cloud provider is currently running
func DetectProvider() evacuator.Provider {
	for _, p := range providers {
		if p.IsSupported() {
			return p
		}
	}

	// TODO: logger in here
	return nil // No provider detected
}
