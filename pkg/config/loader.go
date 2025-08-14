package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"gopkg.in/yaml.v3"
)

// LoadFromFile loads configuration from a YAML file
func LoadFromFile(filename string) (*Config, error) {
	// Expand relative paths
	if !filepath.IsAbs(filename) {
		wd, err := os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("failed to get working directory: %w", err)
		}
		filename = filepath.Join(wd, filename)
	}

	// Read file
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file %s: %w", filename, err)
	}

	// Parse YAML
	cfg := DefaultConfig()
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file %s: %w", filename, err)
	}

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration in %s: %w", filename, err)
	}

	return cfg, nil
}

// LoadFromEnv loads configuration from environment variables
func LoadFromEnv() *Config {
	cfg := DefaultConfig()

	// App configuration - only dry_run is configurable
	if val := os.Getenv("DRY_RUN"); val == "true" {
		cfg.App.DryRun = true
	}

	// Monitoring configuration
	if val := os.Getenv("CLOUD_PROVIDER"); val != "" {
		cfg.Monitoring.Provider = val
	}
	if val := os.Getenv("AUTO_DETECT"); val == "false" {
		cfg.Monitoring.AutoDetect = false
	}
	if val := os.Getenv("POLL_INTERVAL"); val != "" {
		if duration, err := time.ParseDuration(val); err == nil {
			cfg.Monitoring.PollInterval = duration
		}
	}
	if val := os.Getenv("PROVIDER_TIMEOUT"); val != "" {
		if duration, err := time.ParseDuration(val); err == nil {
			cfg.Monitoring.ProviderTimeout = duration
		}
	}
	if val := os.Getenv("PROVIDER_RETRIES"); val != "" {
		if retries, err := strconv.Atoi(val); err == nil {
			cfg.Monitoring.ProviderRetries = retries
		}
	}

	// Handler configuration
	if val := os.Getenv("LOG_HANDLER_ENABLED"); val != "" {
		if enabled, err := strconv.ParseBool(val); err == nil {
			cfg.Handlers.Log.Enabled = enabled
		}
	}
	if val := os.Getenv("KUBERNETES_HANDLER_ENABLED"); val != "" {
		if enabled, err := strconv.ParseBool(val); err == nil {
			cfg.Handlers.Kubernetes.Enabled = enabled
		}
	}

	// Kubernetes configuration
	if val := os.Getenv("NODE_NAME"); val != "" {
		cfg.Kubernetes.NodeName = val
	}
	if val := os.Getenv("KUBECONFIG"); val != "" {
		cfg.Kubernetes.KubeConfig = val
		cfg.Kubernetes.InCluster = false
	}
	if val := os.Getenv("KUBERNETES_IN_CLUSTER"); val != "" {
		if inCluster, err := strconv.ParseBool(val); err == nil {
			cfg.Kubernetes.InCluster = inCluster
		}
	}

	// Logging configuration
	if val := os.Getenv("LOG_LEVEL"); val != "" {
		cfg.Logging.Level = val
	}
	if val := os.Getenv("LOG_FORMAT"); val != "" {
		cfg.Logging.Format = val
	}
	if val := os.Getenv("LOG_FORMAT"); val != "" {
		cfg.Logging.Format = val
	}

	return cfg
}

// GetConfigPath determines the configuration file path
func GetConfigPath() string {
	// Check environment variable first
	if path := os.Getenv("CONFIG_FILE"); path != "" {
		return path
	}

	// Check common locations (user-created config files only)
	candidates := []string{
		"/etc/evacuator/config.yaml", // Container mount point
		"./config.yaml",              // Current directory
		"/usr/local/etc/evacuator/config.yaml",
	}

	for _, path := range candidates {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}

	// Return default
	return "/etc/evacuator/config.yaml"
}
