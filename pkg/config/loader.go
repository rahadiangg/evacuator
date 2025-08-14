package config

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// LoadConfig loads configuration using Viper with support for YAML files and environment variables
func LoadConfig() (*Config, error) {
	// Initialize Viper
	v := viper.New()

	// Set defaults
	setDefaults(v)

	// Configure environment variables
	setupEnvironmentVariables(v)

	// Try to load from configuration file
	if err := loadConfigFile(v); err != nil {
		// If config file is not found, that's okay - we'll use defaults + env vars
		if !os.IsNotExist(err) {
			return nil, fmt.Errorf("error loading config file: %w", err)
		}
	}

	// Unmarshal into config struct
	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("error unmarshaling config: %w", err)
	}

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return &cfg, nil
}

// setDefaults sets default values in Viper
func setDefaults(v *viper.Viper) {
	// App defaults
	v.SetDefault("app.dry_run", false)

	// Monitoring defaults
	v.SetDefault("monitoring.provider", "")
	v.SetDefault("monitoring.auto_detect", true)
	v.SetDefault("monitoring.event_buffer_size", 100)
	v.SetDefault("monitoring.poll_interval", 5*time.Second)
	v.SetDefault("monitoring.provider_timeout", 3*time.Second)
	v.SetDefault("monitoring.provider_retries", 3)

	// Handler defaults
	v.SetDefault("handlers.log.enabled", true)
	v.SetDefault("handlers.kubernetes.enabled", true)
	v.SetDefault("handlers.kubernetes.drain_timeout_seconds", 90)
	v.SetDefault("handlers.kubernetes.force_eviction_after", 90*time.Second)
	v.SetDefault("handlers.kubernetes.skip_daemon_sets", true)
	v.SetDefault("handlers.kubernetes.delete_empty_dir_data", false)
	v.SetDefault("handlers.kubernetes.ignore_pod_disruption", true)
	v.SetDefault("handlers.kubernetes.grace_period_seconds", 10)
	v.SetDefault("handlers.telegram.enabled", false)
	v.SetDefault("handlers.telegram.timeout", 10*time.Second)
	v.SetDefault("handlers.telegram.send_raw", false)

	// Kubernetes defaults
	v.SetDefault("kubernetes.in_cluster", true)
	v.SetDefault("kubernetes.kubeconfig", "")
	v.SetDefault("kubernetes.node_name", "")

	// Logging defaults
	v.SetDefault("logging.level", "info")
	v.SetDefault("logging.format", "json")
	v.SetDefault("logging.output", "stdout")
}

// setupEnvironmentVariables configures Viper to read from environment variables
func setupEnvironmentVariables(v *viper.Viper) {
	// Replace dots with underscores for nested config keys
	// This allows APP_DRY_RUN to map to app.dry_run automatically
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	// Enable automatic environment variable reading
	v.AutomaticEnv()

	// Bind environment variables in consistent format that matches YAML structure
	// This supports both APP_DRY_RUN and EVACUATOR_APP_DRY_RUN formats
	// Environment variable mappings that match YAML structure
	consistentEnvMappings := map[string]string{
		"APP_DRY_RUN":                               "app.dry_run",
		"APP_NODE_NAME":                             "app.node_name",
		"MONITORING_PROVIDER":                       "monitoring.provider",
		"MONITORING_AUTO_DETECT":                    "monitoring.auto_detect",
		"MONITORING_EVENT_BUFFER_SIZE":              "monitoring.event_buffer_size",
		"MONITORING_POLL_INTERVAL":                  "monitoring.poll_interval",
		"MONITORING_PROVIDER_TIMEOUT":               "monitoring.provider_timeout",
		"MONITORING_PROVIDER_RETRIES":               "monitoring.provider_retries",
		"HANDLERS_LOG_ENABLED":                      "handlers.log.enabled",
		"HANDLERS_KUBERNETES_ENABLED":               "handlers.kubernetes.enabled",
		"HANDLERS_KUBERNETES_DRAIN_TIMEOUT_SECONDS": "handlers.kubernetes.drain_timeout_seconds",
		"HANDLERS_KUBERNETES_FORCE_EVICTION_AFTER":  "handlers.kubernetes.force_eviction_after",
		"HANDLERS_KUBERNETES_SKIP_DAEMON_SETS":      "handlers.kubernetes.skip_daemon_sets",
		"HANDLERS_KUBERNETES_DELETE_EMPTY_DIR_DATA": "handlers.kubernetes.delete_empty_dir_data",
		"HANDLERS_KUBERNETES_IGNORE_POD_DISRUPTION": "handlers.kubernetes.ignore_pod_disruption",
		"HANDLERS_KUBERNETES_GRACE_PERIOD_SECONDS":  "handlers.kubernetes.grace_period_seconds",
		"HANDLERS_TELEGRAM_ENABLED":                 "handlers.telegram.enabled",
		"HANDLERS_TELEGRAM_BOT_TOKEN":               "handlers.telegram.bot_token",
		"HANDLERS_TELEGRAM_CHAT_ID":                 "handlers.telegram.chat_id",
		"HANDLERS_TELEGRAM_TIMEOUT":                 "handlers.telegram.timeout",
		"HANDLERS_TELEGRAM_SEND_RAW":                "handlers.telegram.send_raw",
		"KUBERNETES_KUBECONFIG":                     "kubernetes.kubeconfig",
		"KUBERNETES_IN_CLUSTER":                     "kubernetes.in_cluster",
		"LOGGING_LEVEL":                             "logging.level",
		"LOGGING_FORMAT":                            "logging.format",
		"LOGGING_OUTPUT":                            "logging.output",
	}

	// Bind environment variables (supports both APP_* and EVACUATOR_APP_* formats)
	for envVar, configKey := range consistentEnvMappings {
		v.BindEnv(configKey, envVar)
		// Also bind with EVACUATOR_ prefix
		v.BindEnv(configKey, "EVACUATOR_"+envVar)
	}

	// Special handling for CONFIG_FILE environment variable
	v.BindEnv("config.file", "CONFIG_FILE")
} // loadConfigFile attempts to load configuration from a file
func loadConfigFile(v *viper.Viper) error {
	// Check if CONFIG_FILE environment variable is set
	if configFile := v.GetString("config.file"); configFile != "" {
		v.SetConfigFile(configFile)
		return v.ReadInConfig()
	}

	// Try default locations
	configPaths := []string{
		"/etc/evacuator",
		".",
		"/usr/local/etc/evacuator",
	}

	v.SetConfigName("config")
	v.SetConfigType("yaml")

	for _, path := range configPaths {
		v.AddConfigPath(path)
	}

	// ReadInConfig will return an error if no config file is found
	// This is acceptable - we'll use defaults + env vars
	err := v.ReadInConfig()
	if err != nil {
		// Check if it's just a "file not found" error
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			// Config file not found; ignore error, use defaults
			return nil
		}
		// Config file was found but another error was produced
		return err
	}

	return nil
}
