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
	if cfg.App.DryRun != false {
		t.Errorf("Expected DryRun to be false, got %v", cfg.App.DryRun)
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

	if cfg.Kubernetes.InCluster != true {
		t.Errorf("Expected InCluster to be true, got %v", cfg.Kubernetes.InCluster)
	}

	if cfg.Logging.Level != "info" {
		t.Errorf("Expected log level to be info, got %v", cfg.Logging.Level)
	}
}

func TestLoadConfig_EnvironmentVariables(t *testing.T) {
	// Clear environment variables first
	clearEnvironmentVariables()

	// Set some environment variables using the new consistent format
	os.Setenv("APP_DRY_RUN", "true")
	os.Setenv("APP_NODE_NAME", "test-node-123")
	os.Setenv("MONITORING_PROVIDER", "alibaba")
	os.Setenv("MONITORING_POLL_INTERVAL", "10s")
	os.Setenv("LOGGING_LEVEL", "debug")
	os.Setenv("HANDLERS_TELEGRAM_BOT_TOKEN", "bot123456:ABC-DEF")
	os.Setenv("HANDLERS_TELEGRAM_CHAT_ID", "-100123456789")

	defer clearEnvironmentVariables()

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Test environment variable overrides
	if cfg.App.DryRun != true {
		t.Errorf("Expected DryRun to be true, got %v", cfg.App.DryRun)
	}

	if cfg.App.NodeName != "test-node-123" {
		t.Errorf("Expected NodeName to be test-node-123, got %v", cfg.App.NodeName)
	}

	if cfg.Monitoring.Provider != "alibaba" {
		t.Errorf("Expected Provider to be alibaba, got %v", cfg.Monitoring.Provider)
	}

	if cfg.Monitoring.PollInterval != 10*time.Second {
		t.Errorf("Expected PollInterval to be 10s, got %v", cfg.Monitoring.PollInterval)
	}

	if cfg.Logging.Level != "debug" {
		t.Errorf("Expected log level to be debug, got %v", cfg.Logging.Level)
	}

	if cfg.Handlers.Telegram.BotToken != "bot123456:ABC-DEF" {
		t.Errorf("Expected bot token to be set, got %v", cfg.Handlers.Telegram.BotToken)
	}

	if cfg.Handlers.Telegram.ChatID != "-100123456789" {
		t.Errorf("Expected chat ID to be set, got %v", cfg.Handlers.Telegram.ChatID)
	}
}

func TestLoadConfig_WithEVACUATORPrefix(t *testing.T) {
	// Clear environment variables first
	clearEnvironmentVariables()

	// Test with EVACUATOR_ prefix
	os.Setenv("EVACUATOR_APP_DRY_RUN", "true")
	os.Setenv("EVACUATOR_APP_NODE_NAME", "evacuator-test-node")
	os.Setenv("EVACUATOR_MONITORING_PROVIDER", "alibaba")
	os.Setenv("EVACUATOR_LOGGING_LEVEL", "warn")

	defer clearEnvironmentVariables()

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Test prefixed environment variables
	if cfg.App.DryRun != true {
		t.Errorf("Expected DryRun to be true with prefix, got %v", cfg.App.DryRun)
	}

	if cfg.App.NodeName != "evacuator-test-node" {
		t.Errorf("Expected NodeName to be evacuator-test-node with prefix, got %v", cfg.App.NodeName)
	}

	if cfg.Monitoring.Provider != "alibaba" {
		t.Errorf("Expected Provider to be alibaba with prefix, got %v", cfg.Monitoring.Provider)
	}

	if cfg.Logging.Level != "warn" {
		t.Errorf("Expected log level to be warn with prefix, got %v", cfg.Logging.Level)
	}
}

// clearEnvironmentVariables clears all test-related environment variables
func clearEnvironmentVariables() {
	envVars := []string{
		// New consistent format environment variables
		"APP_DRY_RUN",
		"APP_NODE_NAME",
		"MONITORING_PROVIDER",
		"MONITORING_AUTO_DETECT",
		"MONITORING_EVENT_BUFFER_SIZE",
		"MONITORING_POLL_INTERVAL",
		"MONITORING_PROVIDER_TIMEOUT",
		"MONITORING_PROVIDER_RETRIES",
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
		"LOGGING_LEVEL",
		"LOGGING_FORMAT",
		"LOGGING_OUTPUT",
		"CONFIG_FILE",
		// Legacy variable names (for cleanup)
		"DRY_RUN",
		"CLOUD_PROVIDER",
		"AUTO_DETECT",
		"EVENT_BUFFER_SIZE",
		"POLL_INTERVAL",
		"PROVIDER_TIMEOUT",
		"PROVIDER_RETRIES",
		"LOG_HANDLER_ENABLED",
		"KUBERNETES_HANDLER_ENABLED",
		"TELEGRAM_HANDLER_ENABLED",
		"TELEGRAM_BOT_TOKEN",
		"TELEGRAM_CHAT_ID",
		"TELEGRAM_TIMEOUT",
		"TELEGRAM_SEND_RAW",
		"KUBECONFIG",
		"NODE_NAME",
		"LOG_LEVEL",
		"LOG_FORMAT",
		"LOG_OUTPUT",
		// Prefixed versions
		"EVACUATOR_APP_DRY_RUN",
		"EVACUATOR_APP_NODE_NAME",
		"EVACUATOR_MONITORING_PROVIDER",
		"EVACUATOR_MONITORING_AUTO_DETECT",
		"EVACUATOR_LOGGING_LEVEL",
		"EVACUATOR_LOGGING_FORMAT",
		"EVACUATOR_LOGGING_OUTPUT",
	}

	for _, envVar := range envVars {
		os.Unsetenv(envVar)
	}
}
