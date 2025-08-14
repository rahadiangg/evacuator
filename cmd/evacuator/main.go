package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/rahadiangg/evacuator/pkg/cloud"
	"github.com/rahadiangg/evacuator/pkg/config"
	"github.com/rahadiangg/evacuator/pkg/handlers"
	"github.com/rahadiangg/evacuator/pkg/providers/alibaba"
)

func main() {
	// Load configuration using Viper
	cfg, err := config.LoadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load configuration: %v\n", err)
		os.Exit(1)
	}

	// Setup logger
	logger := setupLogger(cfg.Logging)

	logger.Info("Starting evacuator",
		"dry_run", cfg.App.DryRun,
	)

	// Create cloud provider registry
	registry := cloud.NewRegistry()

	// Register cloud providers
	if err := registerProviders(registry, cfg, logger); err != nil {
		logger.Error("Failed to register providers", "error", err)
		os.Exit(1)
	}

	// Create monitoring service for single-node operation (DaemonSet deployment)
	nodeMonitoringConfig := cloud.NodeMonitoringConfig{
		NodeName:        cfg.App.NodeName, // Primary node name from app config
		EventBufferSize: cfg.Monitoring.EventBufferSize,
		Logger:          logger,
		Provider:        cfg.Monitoring.Provider,   // Manual provider selection
		AutoDetect:      cfg.Monitoring.AutoDetect, // Auto-detection fallback
	}

	monitoringService := cloud.NewNodeMonitoringService(registry, nodeMonitoringConfig) // Register event handlers
	if err := registerEventHandlers(monitoringService, cfg, logger); err != nil {
		logger.Error("Failed to register event handlers", "error", err)
		os.Exit(1)
	}

	// Start the application
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start monitoring
	if err := monitoringService.Start(ctx); err != nil {
		logger.Error("Failed to start monitoring service", "error", err)
		os.Exit(1)
	}

	logger.Info("Evacuator started successfully",
		"current_provider", monitoringService.GetCurrentProvider(),
		"node_name", monitoringService.GetNodeName(),
	)

	// TODO: Start metrics server
	// TODO: Start health check server

	// Wait for termination signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-sigChan:
		logger.Info("Received termination signal", "signal", sig)
	case <-ctx.Done():
		logger.Info("Context cancelled")
	}

	// Graceful shutdown
	logger.Info("Shutting down evacuator...")

	if err := monitoringService.Stop(); err != nil {
		logger.Error("Error stopping monitoring service", "error", err)
	}

	// TODO: Stop metrics server
	// TODO: Stop health check server

	logger.Info("Evacuator stopped")
}

// setupLogger configures the logger based on configuration
func setupLogger(cfg config.LoggingConfig) *slog.Logger {
	var level slog.Level
	switch cfg.Level {
	case "debug":
		level = slog.LevelDebug
	case "info":
		level = slog.LevelInfo
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{
		Level: level,
	}

	var handler slog.Handler
	switch cfg.Format {
	case "json":
		handler = slog.NewJSONHandler(os.Stdout, opts)
	case "text":
		handler = slog.NewTextHandler(os.Stdout, opts)
	default:
		handler = slog.NewJSONHandler(os.Stdout, opts)
	}

	return slog.New(handler)
}

// registerProviders registers all cloud providers
func registerProviders(registry *cloud.Registry, cfg *config.Config, logger *slog.Logger) error {
	// Create provider configuration using monitoring settings
	providerConfig := &cloud.ProviderConfig{
		Name:         "alibaba",
		Enabled:      true,
		PollInterval: cfg.Monitoring.PollInterval,
		Timeout:      cfg.Monitoring.ProviderTimeout,
		Retries:      cfg.Monitoring.ProviderRetries,
	}

	// Register Alibaba Cloud provider
	alibabaProvider := alibaba.NewProvider(providerConfig)
	if err := registry.RegisterProvider(alibabaProvider); err != nil {
		return fmt.Errorf("failed to register Alibaba Cloud provider: %w", err)
	}
	logger.Info("Registered Alibaba Cloud provider", "poll_interval", cfg.Monitoring.PollInterval)

	return nil
}

// registerEventHandlers registers all event handlers
func registerEventHandlers(service *cloud.NodeMonitoringService, cfg *config.Config, logger *slog.Logger) error {
	// Register log handler
	if cfg.Handlers.Log.Enabled {
		logHandler := handlers.NewLogHandler(logger)
		service.AddEventHandler(logHandler)
		logger.Info("Registered log handler")
	}

	// Register Kubernetes handler
	if cfg.Handlers.Kubernetes.Enabled {
		k8sConfig := handlers.KubernetesConfig{
			KubeConfig:          cfg.Kubernetes.KubeConfig,
			InCluster:           cfg.Kubernetes.InCluster,
			NodeName:            service.GetNodeName(), // Use the node name from monitoring service
			DrainTimeoutSeconds: cfg.Handlers.Kubernetes.DrainTimeoutSeconds,
			ForceEvictionAfter:  cfg.Handlers.Kubernetes.ForceEvictionAfter,
			SkipDaemonSets:      cfg.Handlers.Kubernetes.SkipDaemonSets,
			DeleteEmptyDirData:  cfg.Handlers.Kubernetes.DeleteEmptyDirData,
			IgnorePodDisruption: cfg.Handlers.Kubernetes.IgnorePodDisruption,
			GracePeriodSeconds:  cfg.Handlers.Kubernetes.GracePeriodSeconds,
			Logger:              logger,
		}

		k8sHandler, err := handlers.NewKubernetesHandler(k8sConfig)
		if err != nil {
			return fmt.Errorf("failed to create Kubernetes handler: %w", err)
		}
		service.AddEventHandler(k8sHandler)
		logger.Info("Registered Kubernetes handler")
	}

	// Register Telegram handler
	if cfg.Handlers.Telegram.Enabled {
		telegramConfig := handlers.TelegramConfig{
			BotToken: cfg.Handlers.Telegram.BotToken,
			ChatID:   cfg.Handlers.Telegram.ChatID,
			Timeout:  cfg.Handlers.Telegram.Timeout,
			SendRaw:  cfg.Handlers.Telegram.SendRaw,
			Logger:   logger,
		}

		telegramHandler, err := handlers.NewTelegramHandler(telegramConfig)
		if err != nil {
			return fmt.Errorf("failed to create Telegram handler: %w", err)
		}
		service.AddEventHandler(telegramHandler)
		logger.Info("Registered Telegram handler", "chat_id", cfg.Handlers.Telegram.ChatID)
	}

	return nil
}
