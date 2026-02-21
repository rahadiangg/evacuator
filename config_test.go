package evacuator

import (
	"testing"
	"time"

	"github.com/spf13/viper"
)

func TestValidateConfig_TelegramValidation(t *testing.T) {
	tests := []struct {
		name    string
		config  *Config
		wantErr bool
		errMsg  string
	}{
		{
			name: "telegram disabled - no validation needed",
			config: &Config{
				Provider: ProviderConfig{
					Name:              "aws",
					PollInterval:      5 * time.Second,
					RequestTimeout:    2 * time.Second,
				},
				Handler: HandlerConfig{
					ProcessingTimeout: 30 * time.Second,
					Telegram: TelegramConfig{
						Enabled:  false,
						BotToken: "",
						ChatID:   "",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "both empty",
			config: &Config{
				Provider: ProviderConfig{
					Name:              "aws",
					PollInterval:      5 * time.Second,
					RequestTimeout:    2 * time.Second,
				},
				Handler: HandlerConfig{
					ProcessingTimeout: 30 * time.Second,
					Telegram: TelegramConfig{
						Enabled:  true,
						BotToken: "",
						ChatID:   "",
					},
				},
			},
			wantErr: true,
			errMsg:  "handler.telegram.bot_token and handler.telegram.chat_id must both be set",
		},
		{
			name: "only token",
			config: &Config{
				Provider: ProviderConfig{
					Name:              "aws",
					PollInterval:      5 * time.Second,
					RequestTimeout:    2 * time.Second,
				},
				Handler: HandlerConfig{
					ProcessingTimeout: 30 * time.Second,
					Telegram: TelegramConfig{
						Enabled:  true,
						BotToken: "abc123",
						ChatID:   "",
					},
				},
			},
			wantErr: true,
			errMsg:  "handler.telegram.bot_token and handler.telegram.chat_id must both be set",
		},
		{
			name: "only chatid",
			config: &Config{
				Provider: ProviderConfig{
					Name:              "aws",
					PollInterval:      5 * time.Second,
					RequestTimeout:    2 * time.Second,
				},
				Handler: HandlerConfig{
					ProcessingTimeout: 30 * time.Second,
					Telegram: TelegramConfig{
						Enabled:  true,
						BotToken: "",
						ChatID:   "123",
					},
				},
			},
			wantErr: true,
			errMsg:  "handler.telegram.bot_token and handler.telegram.chat_id must both be set",
		},
		{
			name: "both set",
			config: &Config{
				Provider: ProviderConfig{
					Name:              "aws",
					PollInterval:      5 * time.Second,
					RequestTimeout:    2 * time.Second,
				},
				Handler: HandlerConfig{
					ProcessingTimeout: 30 * time.Second,
					Telegram: TelegramConfig{
						Enabled:  true,
						BotToken: "abc123",
						ChatID:   "123",
					},
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateConfig(tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateConfig() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && err.Error() != tt.errMsg {
				t.Errorf("validateConfig() error message = %v, want %v", err.Error(), tt.errMsg)
			}
		})
	}
}

func TestValidateConfig_ProviderValidation(t *testing.T) {
	tests := []struct {
		name    string
		config  *Config
		wantErr bool
	}{
		{
			name: "valid provider with name",
			config: &Config{
				Provider: ProviderConfig{
					Name:              "aws",
					AutoDetect:        false,
					PollInterval:      5 * time.Second,
					RequestTimeout:    2 * time.Second,
				},
				Handler: HandlerConfig{
					ProcessingTimeout: 30 * time.Second,
				},
			},
			wantErr: false,
		},
		{
			name: "valid provider with auto detect",
			config: &Config{
				Provider: ProviderConfig{
					Name:              "",
					AutoDetect:        true,
					PollInterval:      5 * time.Second,
					RequestTimeout:    2 * time.Second,
				},
				Handler: HandlerConfig{
					ProcessingTimeout: 30 * time.Second,
				},
			},
			wantErr: false,
		},
		{
			name: "invalid provider - no name and no auto detect",
			config: &Config{
				Provider: ProviderConfig{
					Name:              "",
					AutoDetect:        false,
					PollInterval:      5 * time.Second,
					RequestTimeout:    2 * time.Second,
				},
				Handler: HandlerConfig{
					ProcessingTimeout: 30 * time.Second,
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateConfig(tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateConfig() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateConfig_DurationValidation(t *testing.T) {
	tests := []struct {
		name    string
		config  *Config
		wantErr bool
	}{
		{
			name: "valid durations",
			config: &Config{
				Provider: ProviderConfig{
					Name:              "aws",
					PollInterval:      5 * time.Second,
					RequestTimeout:    2 * time.Second,
				},
				Handler: HandlerConfig{
					ProcessingTimeout: 30 * time.Second,
				},
			},
			wantErr: false,
		},
		{
			name: "poll interval too low",
			config: &Config{
				Provider: ProviderConfig{
					Name:              "aws",
					PollInterval:      2 * time.Second,
					RequestTimeout:    2 * time.Second,
				},
				Handler: HandlerConfig{
					ProcessingTimeout: 30 * time.Second,
				},
			},
			wantErr: true,
		},
		{
			name: "poll interval too high",
			config: &Config{
				Provider: ProviderConfig{
					Name:              "aws",
					PollInterval:      15 * time.Second,
					RequestTimeout:    2 * time.Second,
				},
				Handler: HandlerConfig{
					ProcessingTimeout: 30 * time.Second,
				},
			},
			wantErr: true,
		},
		{
			name: "request timeout too low",
			config: &Config{
				Provider: ProviderConfig{
					Name:              "aws",
					PollInterval:      5 * time.Second,
					RequestTimeout:    500 * time.Millisecond,
				},
				Handler: HandlerConfig{
					ProcessingTimeout: 30 * time.Second,
				},
			},
			wantErr: true,
		},
		{
			name: "processing timeout too high",
			config: &Config{
				Provider: ProviderConfig{
					Name:              "aws",
					PollInterval:      5 * time.Second,
					RequestTimeout:    2 * time.Second,
				},
				Handler: HandlerConfig{
					ProcessingTimeout: 100 * time.Second,
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateConfig(tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateConfig() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestLoadConfig(t *testing.T) {
	// Test loading config with environment variables
	t.Setenv("PROVIDER_NAME", "dummy")
	t.Setenv("PROVIDER_AUTO_DETECT", "false")
	t.Setenv("PROVIDER_POLL_INTERVAL", "5s")
	t.Setenv("PROVIDER_REQUEST_TIMEOUT", "2s")
	t.Setenv("HANDLER_PROCESSING_TIMEOUT", "30s")

	v := viper.New()
	config, err := LoadConfig("", v)
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}

	if config.Provider.Name != "dummy" {
		t.Errorf("Provider.Name = %v, want dummy", config.Provider.Name)
	}
	if config.Provider.PollInterval != 5*time.Second {
		t.Errorf("Provider.PollInterval = %v, want 5s", config.Provider.PollInterval)
	}
	if config.Provider.RequestTimeout != 2*time.Second {
		t.Errorf("Provider.RequestTimeout = %v, want 2s", config.Provider.RequestTimeout)
	}
	if config.Handler.ProcessingTimeout != 30*time.Second {
		t.Errorf("Handler.ProcessingTimeout = %v, want 30s", config.Handler.ProcessingTimeout)
	}
}
