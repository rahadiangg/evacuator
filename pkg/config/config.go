package config

import (
	"fmt"
	"time"
)

// Config represents the application configuration
type Config struct {
	// Application settings
	App AppConfig `yaml:"app" json:"app" mapstructure:"app"`

	// Monitoring settings
	Monitoring MonitoringConfig `yaml:"monitoring" json:"monitoring" mapstructure:"monitoring"`

	// Event handler configurations
	Handlers HandlersConfig `yaml:"handlers" json:"handlers" mapstructure:"handlers"`

	// Kubernetes settings
	Kubernetes KubernetesConfig `yaml:"kubernetes" json:"kubernetes" mapstructure:"kubernetes"`

	// Logging settings
	Logging LoggingConfig `yaml:"logging" json:"logging" mapstructure:"logging"`
}

// AppConfig contains general application settings
type AppConfig struct {
	// DryRun enables dry-run mode (no actual actions)
	DryRun bool `yaml:"dry_run" json:"dry_run" mapstructure:"dry_run"`

	// NodeName is the node name to monitor for termination events
	// Auto-detected from APP_NODE_NAME or NODE_NAME environment variable if empty
	NodeName string `yaml:"node_name" json:"node_name" mapstructure:"node_name"`
}

// MonitoringConfig contains monitoring settings
type MonitoringConfig struct {
	// Provider specifies which cloud provider to use (alibaba)
	// If empty, auto-detection will be used
	Provider string `yaml:"provider" json:"provider" mapstructure:"provider"`

	// AutoDetect enables automatic detection of supported providers
	// Only used when Provider is empty
	AutoDetect bool `yaml:"auto_detect" json:"auto_detect" mapstructure:"auto_detect"`

	// EventBufferSize is the buffer size for event channels
	EventBufferSize int `yaml:"event_buffer_size" json:"event_buffer_size" mapstructure:"event_buffer_size"`

	// PollInterval is how often to check for termination events
	PollInterval time.Duration `yaml:"poll_interval" json:"poll_interval" mapstructure:"poll_interval"`

	// ProviderTimeout is the timeout for cloud provider API calls
	ProviderTimeout time.Duration `yaml:"provider_timeout" json:"provider_timeout" mapstructure:"provider_timeout"`

	// ProviderRetries is the number of retries for failed cloud provider requests
	ProviderRetries int `yaml:"provider_retries" json:"provider_retries" mapstructure:"provider_retries"`
}

// HandlersConfig contains event handler configurations
type HandlersConfig struct {
	// Log handler settings
	Log LogHandlerConfig `yaml:"log" json:"log" mapstructure:"log"`

	// Kubernetes handler settings
	Kubernetes KubernetesHandlerConfig `yaml:"kubernetes" json:"kubernetes" mapstructure:"kubernetes"`

	// Telegram handler settings
	Telegram TelegramHandlerConfig `yaml:"telegram" json:"telegram" mapstructure:"telegram"`
}

// LogHandlerConfig contains log handler settings
type LogHandlerConfig struct {
	// Enabled indicates if the log handler is enabled
	Enabled bool `yaml:"enabled" json:"enabled" mapstructure:"enabled"`
}

// KubernetesHandlerConfig contains Kubernetes handler settings
type KubernetesHandlerConfig struct {
	// Enabled indicates if the Kubernetes handler is enabled
	Enabled bool `yaml:"enabled" json:"enabled" mapstructure:"enabled"`

	// DrainTimeoutSeconds is the timeout for draining nodes
	DrainTimeoutSeconds int `yaml:"drain_timeout_seconds" json:"drain_timeout_seconds" mapstructure:"drain_timeout_seconds"`

	// ForceEvictionAfter is the duration after which to force evict pods
	ForceEvictionAfter time.Duration `yaml:"force_eviction_after" json:"force_eviction_after" mapstructure:"force_eviction_after"`

	// SkipDaemonSets ignores DaemonSet-managed pods during drain
	SkipDaemonSets bool `yaml:"skip_daemon_sets" json:"skip_daemon_sets" mapstructure:"skip_daemon_sets"`

	// DeleteEmptyDirData deletes local data in empty dir volumes
	DeleteEmptyDirData bool `yaml:"delete_empty_dir_data" json:"delete_empty_dir_data" mapstructure:"delete_empty_dir_data"`

	// IgnorePodDisruption ignores pod disruption budgets
	IgnorePodDisruption bool `yaml:"ignore_pod_disruption" json:"ignore_pod_disruption" mapstructure:"ignore_pod_disruption"`

	// GracePeriodSeconds is the grace period for pod termination
	GracePeriodSeconds int `yaml:"grace_period_seconds" json:"grace_period_seconds" mapstructure:"grace_period_seconds"`
}

// TelegramHandlerConfig contains Telegram handler settings
type TelegramHandlerConfig struct {
	// Enabled indicates if the Telegram handler is enabled
	Enabled bool `yaml:"enabled" json:"enabled" mapstructure:"enabled"`

	// BotToken is the Telegram bot token
	BotToken string `yaml:"bot_token" json:"bot_token" mapstructure:"bot_token"`

	// ChatID is the chat ID to send messages to
	ChatID string `yaml:"chat_id" json:"chat_id" mapstructure:"chat_id"`

	// Timeout is the HTTP request timeout
	Timeout time.Duration `yaml:"timeout" json:"timeout" mapstructure:"timeout"`

	// SendRaw indicates whether to send raw event data in addition to formatted message
	SendRaw bool `yaml:"send_raw" json:"send_raw" mapstructure:"send_raw"`
}

// KubernetesConfig contains Kubernetes client settings
type KubernetesConfig struct {
	// KubeConfig is the path to the kubeconfig file
	KubeConfig string `yaml:"kubeconfig" json:"kubeconfig" mapstructure:"kubeconfig"`

	// InCluster indicates if running inside a Kubernetes cluster
	InCluster bool `yaml:"in_cluster" json:"in_cluster" mapstructure:"in_cluster"`
}

// LoggingConfig contains logging settings
type LoggingConfig struct {
	// Level is the log level (debug, info, warn, error)
	Level string `yaml:"level" json:"level" mapstructure:"level"`

	// Format is the log format (json, text)
	Format string `yaml:"format" json:"format" mapstructure:"format"`

	// Output is the log output (stdout, stderr, file path)
	Output string `yaml:"output" json:"output" mapstructure:"output"`
}

// DefaultConfig returns a default configuration
func DefaultConfig() *Config {
	return &Config{
		App: AppConfig{
			DryRun:   false,
			NodeName: "", // Auto-detected from environment or cloud provider
		},
		Monitoring: MonitoringConfig{
			Provider:        "", // Empty means auto-detect
			AutoDetect:      true,
			EventBufferSize: 100,
			PollInterval:    5 * time.Second,
			ProviderTimeout: 3 * time.Second,
			ProviderRetries: 3,
		},
		Handlers: HandlersConfig{
			Log: LogHandlerConfig{
				Enabled: true,
			},
			Kubernetes: KubernetesHandlerConfig{
				Enabled:             true,
				DrainTimeoutSeconds: 90,               // 1.5 minutes - suitable for 2-minute spot termination
				ForceEvictionAfter:  90 * time.Second, // Force evict after 1.5 minutes
				SkipDaemonSets:      true,
				DeleteEmptyDirData:  false,
				IgnorePodDisruption: true, // Ignore PDBs during emergency evacuation
				GracePeriodSeconds:  10,   // Shorter grace period for faster evacuation
			},
			Telegram: TelegramHandlerConfig{
				Enabled: false, // Disabled by default, requires manual configuration
				Timeout: 10 * time.Second,
				SendRaw: false, // Don't send raw data by default
			},
		},
		Kubernetes: KubernetesConfig{
			InCluster: true,
		},
		Logging: LoggingConfig{
			Level:  "info",
			Format: "json",
			Output: "stdout",
		},
	}
}

// Validate validates the configuration
func (c *Config) Validate() error {
	// Validate monitoring config
	if c.Monitoring.EventBufferSize <= 0 {
		return fmt.Errorf("event buffer size must be positive")
	}

	if c.Monitoring.PollInterval <= 0 {
		return fmt.Errorf("poll interval must be positive")
	}

	if c.Monitoring.PollInterval < 3*time.Second {
		return fmt.Errorf("poll interval must be at least 3 seconds, got %v", c.Monitoring.PollInterval)
	}

	if c.Monitoring.PollInterval > 30*time.Second {
		return fmt.Errorf("poll interval must be at most 30 seconds, got %v", c.Monitoring.PollInterval)
	}

	if c.Monitoring.ProviderTimeout <= 0 {
		return fmt.Errorf("provider timeout must be positive")
	}

	if c.Monitoring.ProviderRetries < 0 {
		return fmt.Errorf("provider retries cannot be negative")
	}

	// Validate manual provider selection
	if c.Monitoring.Provider != "" {
		validProviders := map[string]bool{
			"alibaba": true,
		}
		if !validProviders[c.Monitoring.Provider] {
			return fmt.Errorf("invalid provider: %s (valid options: alibaba)", c.Monitoring.Provider)
		}
	}

	// Validate Kubernetes handler config
	if c.Handlers.Kubernetes.Enabled {
		if c.Handlers.Kubernetes.DrainTimeoutSeconds <= 0 {
			return fmt.Errorf("drain timeout must be positive")
		}

		if c.Handlers.Kubernetes.ForceEvictionAfter <= 0 {
			return fmt.Errorf("force eviction timeout must be positive")
		}

		if c.Handlers.Kubernetes.GracePeriodSeconds < 0 {
			return fmt.Errorf("grace period cannot be negative")
		}
	}

	// Validate Telegram handler config
	if c.Handlers.Telegram.Enabled {
		if c.Handlers.Telegram.BotToken == "" {
			return fmt.Errorf("telegram bot token is required when telegram handler is enabled")
		}

		if c.Handlers.Telegram.ChatID == "" {
			return fmt.Errorf("telegram chat ID is required when telegram handler is enabled")
		}

		if c.Handlers.Telegram.Timeout <= 0 {
			return fmt.Errorf("telegram timeout must be positive")
		}
	}

	// Validate logging config
	validLevels := map[string]bool{
		"debug": true,
		"info":  true,
		"warn":  true,
		"error": true,
	}

	if !validLevels[c.Logging.Level] {
		return fmt.Errorf("invalid log level: %s", c.Logging.Level)
	}

	validFormats := map[string]bool{
		"json": true,
		"text": true,
	}

	if !validFormats[c.Logging.Format] {
		return fmt.Errorf("invalid log format: %s", c.Logging.Format)
	}

	return nil
}
