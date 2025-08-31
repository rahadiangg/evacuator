package evacuator

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	NodeName string         `mapstructure:"node_name"`
	Provider ProviderConfig `mapstructure:"provider"`
	Handler  HandlerConfig  `mapstructure:"handler"`
	Log      LogConfig      `mapstructure:"log"`
}

type HandlerConfig struct {
	ProcessingTimeoutRaw string        `mapstructure:"processing_timeout"`
	ProcessingTimeout    time.Duration `mapstructure:"-"`

	Kubernetes KubernetesConfig `mapstructure:"kubernetes"`
	Nomad      NomadConfig      `mapstructure:"nomad"`
	Telegram   TelegramConfig   `mapstructure:"telegram"`
}

type KubernetesConfig struct {
	Enabled            bool   `mapstructure:"enabled"`
	SkipDaemonSets     bool   `mapstructure:"skip_daemon_sets"`
	DeleteEmptyDirData bool   `mapstructure:"delete_empty_dir_data"`
	Kubeconfig         string `mapstructure:"kubeconfig"`
	InCluster          bool   `mapstructure:"in_cluster"`
}

type NomadConfig struct {
	Enabled bool `mapstructure:"enabled"`
	Force   bool `mapstructure:"force"`
}

type TelegramConfig struct {
	Enabled  bool   `mapstructure:"enabled"`
	BotToken string `mapstructure:"bot_token"`
	ChatID   string `mapstructure:"chat_id"`
}

type ProviderConfig struct {
	Name              string              `mapstructure:"name"`
	AutoDetect        bool                `mapstructure:"auto_detect"`
	PollIntervalRaw   string              `mapstructure:"poll_interval"`
	PollInterval      time.Duration       `mapstructure:"-"`
	RequestTimeoutRaw string              `mapstructure:"request_timeout"`
	RequestTimeout    time.Duration       `mapstructure:"-"`
	Dummy             ProviderConfigDummy `mapstructure:"dummy"`
}

type LogConfig struct {
	Level  string `mapstructure:"level"`
	Format string `mapstructure:"format"`
}

type ProviderConfigDummy struct {
	DetectionWait string `mapstructure:"detection_wait"`
}

func LoadConfig(configPath string, v *viper.Viper) (*Config, error) {
	// Set defaults first (lowest priority)
	setDefaults(v)

	// Setup environment variable mapping (highest priority)
	setupEnvironmentMapping(v)

	// Read config file if provided (middle priority)
	if configPath != "" {
		fileName := filepath.Base(configPath)
		filePath := filepath.Dir(configPath)

		// Remove file extension for viper
		configName := strings.TrimSuffix(fileName, filepath.Ext(fileName))

		v.SetConfigName(configName)
		v.AddConfigPath(filePath)
		v.SetConfigType("yaml")

		// Try to read the config file, but don't fail if it doesn't exist
		if err := v.ReadInConfig(); err != nil {
			// Only return error for actual read errors, not missing file
			if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
				return nil, err
			}
			// Config file not found is OK when optional
		}
	}

	var config Config
	if err := v.Unmarshal(&config); err != nil {
		return nil, err
	}

	// Parse duration strings into time.Duration fields
	if err := parseDurationFields(&config); err != nil {
		return nil, err
	}

	// Validate the config after parsing duration fields
	if err := validateConfig(&config); err != nil {
		return nil, err
	}

	return &config, nil
}

func parseDurationFields(c *Config) error {
	// Parse handler processing timeout
	processingTimeout, err := time.ParseDuration(c.Handler.ProcessingTimeoutRaw)
	if err != nil {
		return fmt.Errorf("handler.processing_timeout must be a valid duration: %w", err)
	}
	c.Handler.ProcessingTimeout = processingTimeout

	providerPollInterval, err := time.ParseDuration(c.Provider.PollIntervalRaw)
	if err != nil {
		return fmt.Errorf("provider.poll_interval must be a valid duration: %w", err)
	}
	c.Provider.PollInterval = providerPollInterval

	providerRequestTimeout, err := time.ParseDuration(c.Provider.RequestTimeoutRaw)
	if err != nil {
		return fmt.Errorf("provider.request_timeout must be a valid duration: %w", err)
	}
	c.Provider.RequestTimeout = providerRequestTimeout

	return nil
}

func validateConfig(c *Config) error {

	// provider
	if c.Provider.Name == "" && !c.Provider.AutoDetect {
		return fmt.Errorf("provider.name must be set")
	}

	// duration things
	if c.Provider.PollInterval < 3*time.Second || c.Provider.PollInterval > 10*time.Second {
		return fmt.Errorf("provider.poll_interval must be between 3s and 10s")
	}

	if c.Provider.RequestTimeout < 1*time.Second || c.Provider.RequestTimeout > 5*time.Second {
		return fmt.Errorf("provider.request_timeout must be between 1s and 5s")
	}

	if c.Handler.ProcessingTimeout > 75*time.Second {
		// for warning not actual error
		return fmt.Errorf("handler.processing_timeout more than 75s that makes it ineffective")
	}

	// telegram
	if c.Handler.Telegram.Enabled {
		if c.Handler.Telegram.BotToken == "" && c.Handler.Telegram.ChatID == "" {
			return fmt.Errorf("handler.telegram.bot_token and handler.telegram.chat_id must be set")
		}
	}

	// kubernetes
	if c.Handler.Kubernetes.Enabled {
		if c.Handler.Kubernetes.Kubeconfig == "" && !c.Handler.Kubernetes.InCluster {
			return fmt.Errorf("handler.kubernetes.kubeconfig must be set if not running in-cluster")
		}
	}

	// nomad
	if c.Handler.Kubernetes.Enabled && c.Handler.Nomad.Enabled {
		return fmt.Errorf("handler.kubernetes and handler.nomad cannot be enabled at the same time")
	}

	return nil
}

// ConfigItem represents a configuration item with its environment variable mapping and default value
type ConfigItem struct {
	EnvVar       string
	YamlKey      string
	DefaultValue interface{}
}

// configItems defines all configuration items with their environment mappings and defaults
var configItems = []ConfigItem{
	{"NODE_NAME", "node_name", ""},
	{"PROVIDER_NAME", "provider.name", ""},
	{"PROVIDER_AUTO_DETECT", "provider.auto_detect", true},
	{"PROVIDER_POLL_INTERVAL", "provider.poll_interval", "3s"},
	{"PROVIDER_REQUEST_TIMEOUT", "provider.request_timeout", "2s"},
	{"PROVIDER_DUMMY_DETECTION_WAIT", "provider.dummy.detection_wait", "10s"},
	{"LOG_LEVEL", "log.level", "info"},
	{"LOG_FORMAT", "log.format", "json"},
	{"HANDLER_PROCESSING_TIMEOUT", "handler.processing_timeout", "75s"},
	{"HANDLER_KUBERNETES_ENABLED", "handler.kubernetes.enabled", false},
	{"HANDLER_KUBERNETES_SKIP_DAEMON_SETS", "handler.kubernetes.skip_daemon_sets", true},
	{"HANDLER_KUBERNETES_DELETE_EMPTY_DIR_DATA", "handler.kubernetes.delete_empty_dir_data", false},
	{"HANDLER_KUBERNETES_KUBECONFIG", "handler.kubernetes.kubeconfig", ""},
	{"HANDLER_KUBERNETES_IN_CLUSTER", "handler.kubernetes.in_cluster", true},
	{"HANDLER_TELEGRAM_ENABLED", "handler.telegram.enabled", false},
	{"HANDLER_TELEGRAM_BOT_TOKEN", "handler.telegram.bot_token", ""},
	{"HANDLER_TELEGRAM_CHAT_ID", "handler.telegram.chat_id", ""},
	{"HANDLER_NOMAD_ENABLED", "handler.nomad.enabled", false},
	{"HANDLER_NOMAD_FORCE", "handler.nomad.force", false},
}

// setDefaults sets default values for configuration
func setDefaults(v *viper.Viper) {
	for _, item := range configItems {
		if item.DefaultValue != nil {
			v.SetDefault(item.YamlKey, item.DefaultValue)
		}
	}
}

// setupEnvironmentMapping configures environment variable to config key mapping
func setupEnvironmentMapping(v *viper.Viper) {
	v.AutomaticEnv() // Automatically read environment variables
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	for _, item := range configItems {
		v.BindEnv(item.YamlKey, item.EnvVar)
	}
}

// Global configuration management
var (
	// globalConfig holds the application configuration
	// Set once at startup, read-only during runtime
	globalConfig *Config
)

// SetGlobalConfig sets the global configuration for the application
// This should be called once during application startup
func SetGlobalConfig(config *Config) {
	globalConfig = config
}

// GetGlobalConfig returns the global configuration
// Returns nil if configuration hasn't been initialized
func GetGlobalConfig() *Config {
	return globalConfig
}

// GetProviderConfig returns the provider configuration
func GetProviderConfig() ProviderConfig {
	if globalConfig == nil {
		return ProviderConfig{}
	}
	return globalConfig.Provider
}

// GetHandlerConfig returns the handler configuration
func GetHandlerConfig() HandlerConfig {
	if globalConfig == nil {
		return HandlerConfig{}
	}
	return globalConfig.Handler
}

// GetLogConfig returns the log configuration
func GetLogConfig() LogConfig {
	if globalConfig == nil {
		return LogConfig{}
	}
	return globalConfig.Log
}

// GetNodeName returns the configured node name
func GetNodeName() string {
	if globalConfig == nil {
		return ""
	}
	return globalConfig.NodeName
}
