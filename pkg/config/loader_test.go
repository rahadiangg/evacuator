package config

import (
	"os"
	"testing"
	"time"
)

func TestLoadConfig_Defaults(t *testing.T) {
	// Clear environment variables that might affect the test
	clearEnvironmentVariables()

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Test default values
	if cfg.DryRun != false {
		t.Errorf("Expected DryRun to be false, got %v", cfg.DryRun)
	}

	if cfg.Monitoring.Provider != "" {
		t.Errorf("Expected Provider to be empty, got %v", cfg.Monitoring.Provider)
	}

	if cfg.Monitoring.AutoDetect != true {
		t.Errorf("Expected AutoDetect to be true, got %v", cfg.Monitoring.AutoDetect)
	}

	if cfg.Monitoring.PollInterval != 5*time.Second {
		t.Errorf("Expected PollInterval to be 5s, got %v", cfg.Monitoring.PollInterval)
	}

	if cfg.Handlers.Log.Enabled != true {
		t.Errorf("Expected Log handler to be enabled, got %v", cfg.Handlers.Log.Enabled)
	}

	if cfg.Handlers.Kubernetes.InCluster != true {
		t.Errorf("Expected Handlers.Kubernetes.InCluster to be true, got %v", cfg.Handlers.Kubernetes.InCluster)
	}

	if cfg.Log.Level != "info" {
		t.Errorf("Expected log level to be info, got %v", cfg.Log.Level)
	}
}

func TestLoadConfig_EnvironmentVariables(t *testing.T) {
	// Clear environment variables first
	clearEnvironmentVariables()

	// Set some environment variables using the new consistent format
	os.Setenv("DRY_RUN", "true")
	os.Setenv("NODE_NAME", "test-node-123")
	os.Setenv("MONITORING_PROVIDER", "alibaba")
	os.Setenv("MONITORING_POLL_INTERVAL", "10s")
	os.Setenv("LOG_LEVEL", "debug")
	os.Setenv("HANDLERS_TELEGRAM_BOT_TOKEN", "bot123456:ABC-DEF")
	os.Setenv("HANDLERS_TELEGRAM_CHAT_ID", "-100123456789")

	defer clearEnvironmentVariables()

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Test environment variable overrides
	if cfg.DryRun != true {
		t.Errorf("Expected DryRun to be true, got %v", cfg.DryRun)
	}

	if cfg.NodeName != "test-node-123" {
		t.Errorf("Expected NodeName to be test-node-123, got %v", cfg.NodeName)
	}

	if cfg.Monitoring.Provider != "alibaba" {
		t.Errorf("Expected Provider to be alibaba, got %v", cfg.Monitoring.Provider)
	}

	if cfg.Monitoring.PollInterval != 10*time.Second {
		t.Errorf("Expected PollInterval to be 10s, got %v", cfg.Monitoring.PollInterval)
	}

	if cfg.Log.Level != "debug" {
		t.Errorf("Expected log level to be debug, got %v", cfg.Log.Level)
	}

	if cfg.Handlers.Telegram.BotToken != "bot123456:ABC-DEF" {
		t.Errorf("Expected bot token to be set, got %v", cfg.Handlers.Telegram.BotToken)
	}

	if cfg.Handlers.Telegram.ChatID != "-100123456789" {
		t.Errorf("Expected chat ID to be set, got %v", cfg.Handlers.Telegram.ChatID)
	}
}

// clearEnvironmentVariables clears all test-related environment variables
func clearEnvironmentVariables() {
	envVars := []string{
		// New consistent format environment variables
		"DRY_RUN",
		"NODE_NAME",
		"MONITORING_PROVIDER",
		"MONITORING_AUTO_DETECT",
		"MONITORING_EVENT_BUFFER_SIZE",
		"MONITORING_POLL_INTERVAL",
		"MONITORING_PROVIDER_TIMEOUT",
		"HANDLERS_LOG_ENABLED",
		"HANDLERS_KUBERNETES_ENABLED",
		"HANDLERS_KUBERNETES_DRAIN_TIMEOUT_SECONDS",
		"HANDLERS_KUBERNETES_FORCE_EVICTION_AFTER",
		"HANDLERS_KUBERNETES_SKIP_DAEMON_SETS",
		"HANDLERS_KUBERNETES_DELETE_EMPTY_DIR_DATA",
		"HANDLERS_KUBERNETES_IGNORE_POD_DISRUPTION",
		"HANDLERS_KUBERNETES_GRACE_PERIOD_SECONDS",
		"HANDLERS_TELEGRAM_ENABLED",
		"HANDLERS_TELEGRAM_BOT_TOKEN",
		"HANDLERS_TELEGRAM_CHAT_ID",
		"HANDLERS_TELEGRAM_TIMEOUT",
		"HANDLERS_TELEGRAM_SEND_RAW",
		"KUBERNETES_KUBECONFIG",
		"KUBERNETES_IN_CLUSTER",
		"KUBERNETES_NODE_NAME",
		"LOG_LEVEL",
		"LOG_FORMAT",
		"CONFIG_FILE",
		// Legacy variable names (for cleanup)
		"APP_DRY_RUN",
		"APP_NODE_NAME",
		"CLOUD_PROVIDER",
		"AUTO_DETECT",
		"EVENT_BUFFER_SIZE",
		"POLL_INTERVAL",
		"PROVIDER_TIMEOUT",
		"LOG_HANDLER_ENABLED",
		"KUBERNETES_HANDLER_ENABLED",
		"TELEGRAM_HANDLER_ENABLED",
		"TELEGRAM_BOT_TOKEN",
		"TELEGRAM_CHAT_ID",
		"TELEGRAM_TIMEOUT",
		"TELEGRAM_SEND_RAW",
		"KUBECONFIG",
	}

	for _, envVar := range envVars {
		os.Unsetenv(envVar)
	}
}
