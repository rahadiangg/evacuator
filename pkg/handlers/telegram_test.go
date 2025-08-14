package handlers

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/rahadiangg/evacuator/pkg/cloud"
)

func TestNewTelegramHandler(t *testing.T) {
	tests := []struct {
		name        string
		config      TelegramConfig
		expectError bool
	}{
		{
			name: "valid config",
			config: TelegramConfig{
				BotToken: "bot123456:ABC-DEF1234ghIkl-zyx57W2v1u123ew11",
				ChatID:   "-100123456789",
				Timeout:  10 * time.Second,
				Logger:   slog.New(slog.NewTextHandler(os.Stdout, nil)),
			},
			expectError: false,
		},
		{
			name: "missing bot token",
			config: TelegramConfig{
				ChatID: "-100123456789",
			},
			expectError: true,
		},
		{
			name: "missing chat ID",
			config: TelegramConfig{
				BotToken: "bot123456:ABC-DEF1234ghIkl-zyx57W2v1u123ew11",
			},
			expectError: true,
		},
		{
			name: "with defaults",
			config: TelegramConfig{
				BotToken: "bot123456:ABC-DEF1234ghIkl-zyx57W2v1u123ew11",
				ChatID:   "-100123456789",
				// Timeout, SendRaw, Logger should use defaults
			},
			expectError: false,
		},
		{
			name: "with send_raw enabled",
			config: TelegramConfig{
				BotToken: "bot123456:ABC-DEF1234ghIkl-zyx57W2v1u123ew11",
				ChatID:   "-100123456789",
				SendRaw:  true,
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler, err := NewTelegramHandler(tt.config)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if handler.Name() != "telegram-handler" {
				t.Errorf("expected handler name 'telegram-handler', got '%s'", handler.Name())
			}

			// Test that defaults are set
			if tt.config.Timeout == 0 && handler.httpClient.Timeout != 10*time.Second {
				t.Errorf("expected default timeout 10s, got %v", handler.httpClient.Timeout)
			}
		})
	}
}

func TestValidateConfig(t *testing.T) {
	tests := []struct {
		name        string
		config      TelegramConfig
		expectError bool
	}{
		{
			name: "valid config",
			config: TelegramConfig{
				BotToken: "bot123456:ABC-DEF1234ghIkl-zyx57W2v1u123ew11",
				ChatID:   "-100123456789",
				Timeout:  10 * time.Second,
			},
			expectError: false,
		},
		{
			name: "empty bot token",
			config: TelegramConfig{
				ChatID: "-100123456789",
			},
			expectError: true,
		},
		{
			name: "empty chat ID",
			config: TelegramConfig{
				BotToken: "bot123456:ABC",
			},
			expectError: true,
		},
		{
			name: "negative timeout",
			config: TelegramConfig{
				BotToken: "bot123456:ABC",
				ChatID:   "-100123456789",
				Timeout:  -1 * time.Second,
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateConfig(tt.config)

			if tt.expectError && err == nil {
				t.Errorf("expected error but got none")
			}

			if !tt.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestFormatMessage(t *testing.T) {
	handler := &TelegramHandler{}

	event := cloud.TerminationEvent{
		NodeID:          "i-0abcd1234567890ef",
		NodeName:        "ip-10-0-1-123.us-west-2.compute.internal",
		Reason:          cloud.SpotInstanceTermination,
		TerminationTime: time.Date(2024, 1, 15, 14, 30, 0, 0, time.UTC),
		NoticeTime:      time.Date(2024, 1, 15, 14, 28, 0, 0, time.UTC),
		GracePeriod:     2 * time.Minute,
		CloudProvider:   "alibaba",
		Region:          "us-west-2",
		Zone:            "us-west-2a",
		InstanceType:    "ecs.c5.large",
	}

	message := handler.formatMessage(event)

	// Check that the message contains expected information (accounting for Markdown escaping)
	expectedContents := []string{
		"Node Termination Alert",
		"ip\\-10\\-0\\-1\\-123\\.us\\-west\\-2\\.compute\\.internal", // Escaped version
		"i\\-0abcd1234567890ef",                                      // Escaped version
		"us\\-west\\-2/us\\-west\\-2a",                               // Escaped version
		"alibaba",
		"ecs\\.c5\\.large",              // Escaped version
		"spot\\-instance\\-termination", // Escaped version
		"2024\\-01\\-15 14:30:00",       // Escaped version
		"2\\.0 minutes",                 // Escaped version
	}

	for _, content := range expectedContents {
		if !contains(message, content) {
			t.Errorf("message should contain '%s', but doesn't.\nMessage: %s", content, message)
		}
	}

	// Check that emojis are present
	expectedEmojis := []string{"🚨", "⚠️", "🆔", "📍", "☁️", "💾", "🔴", "⏰", "⏳"}
	for _, emoji := range expectedEmojis {
		if !contains(message, emoji) {
			t.Errorf("message should contain emoji '%s', but doesn't.\nMessage: %s", emoji, message)
		}
	}
}

func TestFormatRawMessage(t *testing.T) {
	handler := &TelegramHandler{}

	event := cloud.TerminationEvent{
		NodeID:          "i-0abcd1234567890ef",
		NodeName:        "ip-10-0-1-123.us-west-2.compute.internal",
		Reason:          cloud.SpotInstanceTermination,
		TerminationTime: time.Date(2024, 1, 15, 14, 30, 0, 0, time.UTC),
		NoticeTime:      time.Date(2024, 1, 15, 14, 28, 0, 0, time.UTC),
		GracePeriod:     2 * time.Minute,
		CloudProvider:   "alibaba",
		Region:          "us-west-2",
		Zone:            "us-west-2a",
		InstanceType:    "ecs.c5.large",
	}

	message := handler.formatRawMessage(event)

	// Check that the message contains expected elements
	expectedContents := []string{
		"📊 *Raw Event Data*",
		"```json",
		"node_id",
		"node_name",
		"reason",
		"termination_time",
		"cloud_provider",
		"region",
		"zone",
		"instance_type",
	}

	for _, content := range expectedContents {
		if !contains(message, content) {
			t.Errorf("raw message should contain '%s', but doesn't.\nMessage: %s", content, message)
		}
	}

	// Check that it contains JSON structure markers
	if !contains(message, "{") || !contains(message, "}") {
		t.Errorf("raw message should contain JSON structure markers")
	}
}

func TestCreateEmergencyRawMessage(t *testing.T) {
	handler := &TelegramHandler{}

	event := cloud.TerminationEvent{
		NodeID:        "i-0abcd1234567890ef",
		NodeName:      "test-node",
		Reason:        cloud.SpotInstanceTermination,
		CloudProvider: "alibaba",
		Region:        "us-west-2",
		Zone:          "us-west-2a",
		InstanceType:  "ecs.c5.large",
	}

	errorContext := "Test emergency scenario"
	message := handler.createEmergencyRawMessage(event, errorContext)

	// Check that the emergency message contains expected elements
	expectedContents := []string{
		"🆘 EMERGENCY RAW DATA CAPTURE",
		"INCIDENT DETECTED",
		"Test emergency scenario",
		"i-0abcd1234567890ef",
		"test-node",
		"spot-instance-termination",
		"alibaba",
		"us-west-2",
		"us-west-2a",
		"ecs.c5.large",
		"INCIDENT TIMESTAMP",
	}

	for _, content := range expectedContents {
		if !contains(message, content) {
			t.Errorf("emergency message should contain '%s', but doesn't.\nMessage: %s", content, message)
		}
	}

	// Test with empty fields
	emptyEvent := cloud.TerminationEvent{}
	emptyMessage := handler.createEmergencyRawMessage(emptyEvent, "Empty event test")

	expectedInEmpty := []string{
		"🆘 EMERGENCY RAW DATA CAPTURE",
		"Empty event test",
		"UNKNOWN", // Should appear for empty fields
	}

	for _, content := range expectedInEmpty {
		if !contains(emptyMessage, content) {
			t.Errorf("emergency message with empty event should contain '%s', but doesn't.\nMessage: %s", content, emptyMessage)
		}
	}
}

func TestEscapeMarkdown(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"simple text", "simple text"},
		{"text_with_underscore", "text\\_with\\_underscore"},
		{"text*with*asterisk", "text\\*with\\*asterisk"},
		{"text[with]brackets", "text\\[with\\]brackets"},
		{"text(with)parentheses", "text\\(with\\)parentheses"},
		{"text.with.dots", "text\\.with\\.dots"},
		{"text!with!exclamation", "text\\!with\\!exclamation"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := escapeMarkdown(tt.input)
			if result != tt.expected {
				t.Errorf("escapeMarkdown(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestHandleTerminationEvent(t *testing.T) {
	// This test would require mocking HTTP requests
	// For now, we just test that the method exists and has the right signature
	handler := &TelegramHandler{
		logger:     slog.Default(),
		botToken:   "test-token",
		chatID:     "test-chat",
		httpClient: &http.Client{Timeout: 1 * time.Second},
	}

	event := cloud.TerminationEvent{
		NodeID:        "test-node",
		NodeName:      "test-node-name",
		Reason:        cloud.SpotInstanceTermination,
		CloudProvider: "test-provider",
	}

	// This will fail because we don't have a real bot token, but we can verify
	// the method exists and returns an error as expected
	err := handler.HandleTerminationEvent(context.Background(), event)
	if err == nil {
		t.Error("expected error with invalid bot token, got nil")
	}
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) &&
		(s[:len(substr)] == substr || s[len(s)-len(substr):] == substr ||
			containsAtPosition(s, substr)))
}

func containsAtPosition(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
