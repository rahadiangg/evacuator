package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/rahadiangg/evacuator"
	"github.com/spf13/viper"
)

// HandlerResult represents the result of processing a termination event by a handler
type HandlerResult struct {
	HandlerName string
	Error       error
	ProcessedAt time.Time
}

// Application configuration constants
const (

	// Handler processing timeout - time allowed for each handler to process termination event
	// Set to 75 seconds to ensure completion within 2-minute spot termination window
	// This allows 33 seconds safety buffer before force-terminates the instance
	HandlerProcessingTimeout = 75 * time.Second

	// Graceful shutdown timeout - maximum time to wait for goroutines to finish
	// Reduced to 10 seconds since handlers should already be complete
	// This is just for final cleanup before termination deadline
	GracefulShutdownTimeout = 10 * time.Second
)

func main() {
	// Parse command-line flags
	var configPath = flag.String("config", "", "path to config file (optional)")
	flag.Parse()

	v := viper.New()
	config, err := evacuator.LoadConfig(*configPath, v)
	if err != nil {
		fmt.Printf("failed to load config file: %v", err)
		os.Exit(1)
	}

	logger := setupLogger()

	// Set the global configuration
	evacuator.SetGlobalConfig(config)

	// Log the configuration source
	if *configPath != "" {
		if v.ConfigFileUsed() != "" {
			logger.Info("loaded configuration from file", "file", v.ConfigFileUsed())
		} else {
			logger.Info("config file specified but not found, using environment variables and defaults", "file", *configPath)
		}
	} else {
		logger.Info("no config file specified, using environment variables and defaults")
	}

	// Create default HTTP client with reasonable timeout
	parsedTimeout, err := time.ParseDuration(config.Provider.RequestTimeout)
	if err != nil {
		logger.Error("failed to parse provider request timeout", "error", err)
		os.Exit(1)
	}
	providerHttpClient := &http.Client{
		Timeout: parsedTimeout,
	}

	dummyDetectionWait, err := time.ParseDuration(config.Provider.PollInterval)
	if err != nil {
		logger.Error("failed to parse dummy provider detection wait time", "error", err)
		os.Exit(1)
	}

	// Register all available providers
	providers := []evacuator.Provider{
		evacuator.NewAwsProvider(providerHttpClient, logger),
		evacuator.NewAlicloudProvider(providerHttpClient, logger),
		evacuator.NewDummyProvider(logger, dummyDetectionWait*3),
	}

	// Register all configured handlers
	handlerRegistry := evacuator.NewHandlerRegistry(logger)
	handlers, err := handlerRegistry.RegisterHandlers()
	if err != nil {
		logger.Error("failed to register handlers", "error", err)
		os.Exit(1)
	}

	// Create root context for coordinated shutdown
	rootCtx, rootCancel := context.WithCancel(context.Background())
	defer rootCancel()

	// Detect the current cloud provider environment
	provider := DetectProvider(rootCtx, providers, logger)
	if provider == nil {
		logger.Error("no supported provider detected")
		os.Exit(1)
	}

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
// It first checks if a specific provider is configured, then falls back to auto-detection
// if auto_detect is enabled. Uses global configuration.
func DetectProvider(ctx context.Context, providers []evacuator.Provider, logger *slog.Logger) evacuator.Provider {
	providerConfig := evacuator.GetProviderConfig()

	// when specified provider configured use it
	if providerConfig.Name != "" {
		for _, p := range providers {
			if strings.EqualFold(string(p.Name()), providerConfig.Name) {
				if p.IsSupported(ctx) {
					logger.Info("configured provider detected and supported", "provider_name", p.Name())
					return p
				} else {
					logger.Warn("configured provider not supported in current environment", "provider_name", p.Name())
					return nil
				}
			}
		}
		logger.Error("configured provider not found", "provider_name", providerConfig.Name)
		return nil
	}

	// If auto-detection is disabled and no provider is specified, return nil
	if !providerConfig.AutoDetect {
		logger.Error("auto-detection disabled and no provider specified")
		return nil
	}

	// Auto-detect provider
	logger.Info("auto-detecting cloud provider")
	for _, p := range providers {
		if p.IsSupported(ctx) {
			logger.Info("provider auto-detected", "provider_name", p.Name())
			return p
		}
	}

	logger.Debug("no supported provider detected during auto-detection")
	return nil
}

// broadcastTerminationEvents distributes termination events to all handlers.
// This function processes each event through all handlers sequentially and collects results.
func broadcastTerminationEvents(ctx context.Context, terminationEvent <-chan evacuator.TerminationEvent, handlers []evacuator.Handler, logger *slog.Logger) {

	config := evacuator.GetGlobalConfig()

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

					// if node.name configured, use it as hostname
					if config.NodeName != "" {
						event.Hostname = config.NodeName
					}

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

func setupLogger() *slog.Logger {
	var logLeveler slog.Level

	config := evacuator.GetLogConfig()

	switch config.Level {
	case "debug":
		logLeveler = slog.LevelDebug
	case "info":
		logLeveler = slog.LevelInfo
	case "warn":
		logLeveler = slog.LevelWarn
	case "error":
		logLeveler = slog.LevelError
	default:
		logLeveler = slog.LevelInfo
	}

	// Create default logger with text output to stdout
	logOpts := slog.HandlerOptions{
		Level: logLeveler,
	}

	var logger *slog.Logger
	switch config.Format {
	case "text":
		logger = slog.New(slog.NewTextHandler(os.Stdout, &logOpts))
	case "json":
		logger = slog.New(slog.NewJSONHandler(os.Stdout, &logOpts))
	default:
		logger = slog.New(slog.NewTextHandler(os.Stdout, &logOpts))
	}

	return logger
}
