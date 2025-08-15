package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/rahadiangg/evacuator/pkg/cloud"
)

const (
	// telegramAPIBaseURL is the Telegram Bot API base URL - hardcoded as it rarely changes
	telegramAPIBaseURL = "https://api.telegram.org"
)

// TelegramHandler sends termination event notifications to Telegram
type TelegramHandler struct {
	logger     *slog.Logger
	botToken   string
	chatID     string
	httpClient *http.Client
	sendRaw    bool
}

// TelegramConfig contains configuration for the Telegram handler
type TelegramConfig struct {
	// BotToken is the Telegram bot token (required)
	BotToken string

	// ChatID is the chat ID to send messages to (required)
	ChatID string

	// Timeout is the HTTP request timeout (default: 10 seconds)
	Timeout time.Duration

	// SendRaw indicates whether to send raw event data in addition to formatted message
	SendRaw bool

	// Logger is the logger instance
	Logger *slog.Logger
}

// TelegramMessage represents a Telegram message payload
type TelegramMessage struct {
	ChatID    string `json:"chat_id"`
	Text      string `json:"text"`
	ParseMode string `json:"parse_mode,omitempty"`
}

// TelegramResponse represents a Telegram API response
type TelegramResponse struct {
	OK          bool   `json:"ok"`
	Description string `json:"description,omitempty"`
	ErrorCode   int    `json:"error_code,omitempty"`
}

// NewTelegramHandler creates a new Telegram handler
func NewTelegramHandler(config TelegramConfig) (*TelegramHandler, error) {
	if config.BotToken == "" {
		return nil, fmt.Errorf("bot token is required")
	}

	if config.ChatID == "" {
		return nil, fmt.Errorf("chat ID is required")
	}

	if config.Logger == nil {
		config.Logger = slog.Default()
	}

	if config.Timeout == 0 {
		config.Timeout = 10 * time.Second
	}

	httpClient := &http.Client{
		Timeout: config.Timeout,
	}

	return &TelegramHandler{
		logger:     config.Logger,
		botToken:   config.BotToken,
		chatID:     config.ChatID,
		httpClient: httpClient,
		sendRaw:    config.SendRaw,
	}, nil
}

// Name returns the handler name
func (h *TelegramHandler) Name() string {
	return "telegram-handler"
}

// HandleTerminationEvent sends a termination event notification to Telegram
func (h *TelegramHandler) HandleTerminationEvent(ctx context.Context, event cloud.TerminationEvent) error {
	h.logger.Info("[handlers.telegram] sending termination event notification to telegram",
		"node_id", event.NodeID,
		"node_name", event.NodeName,
		"reason", event.Reason,
		"chat_id", h.chatID,
		"send_raw", h.sendRaw,
	)

	var formattedMessageErr error
	var rawMessageErr error
	var rawDataSent bool

	// PRIORITY 1: If send_raw is enabled, send raw data FIRST and ALWAYS
	// This ensures we capture the incident data before any processing can fail
	if h.sendRaw {
		h.logger.Debug("[handlers.telegram] attempting to send raw data (priority #1)")
		func() {
			defer func() {
				if r := recover(); r != nil {
					h.logger.Error("[handlers.telegram] panic while sending raw data, attempting emergency fallback", "panic", r)
					// Even if formatRawMessage panics, try to send SOMETHING
					emergencyMessage := h.createEmergencyRawMessage(event, fmt.Sprintf("PANIC: %v", r))
					if err := h.sendMessage(ctx, emergencyMessage); err != nil {
						h.logger.Error("[handlers.telegram] emergency raw message also failed", "error", err)
						rawMessageErr = fmt.Errorf("panic and emergency fallback failed: panic=%v, emergency_err=%v", r, err)
					} else {
						h.logger.Warn("[handlers.telegram] emergency raw message sent after panic", "panic", r)
						rawDataSent = true
					}
				}
			}()

			rawMessage := h.formatRawMessage(event)
			if err := h.sendMessage(ctx, rawMessage); err != nil {
				rawMessageErr = err
				h.logger.Error("[handlers.telegram] failed to send raw data (attempting emergency fallback)",
					"node_id", event.NodeID,
					"error", err,
				)

				// Try emergency fallback even if regular raw message fails
				emergencyMessage := h.createEmergencyRawMessage(event, fmt.Sprintf("Raw send failed: %v", err))
				if emergencyErr := h.sendMessage(ctx, emergencyMessage); emergencyErr != nil {
					h.logger.Error("[handlers.telegram] emergency raw message also failed",
						"node_id", event.NodeID,
						"original_error", err,
						"emergency_error", emergencyErr,
					)
					rawMessageErr = fmt.Errorf("both raw and emergency messages failed: raw=%v, emergency=%v", err, emergencyErr)
				} else {
					h.logger.Warn("[handlers.telegram] emergency raw message sent after regular raw message failed",
						"node_id", event.NodeID,
						"original_error", err,
					)
					rawDataSent = true
					rawMessageErr = nil // Clear error since emergency succeeded
				}
			} else {
				h.logger.Info("[handlers.telegram] raw data sent successfully (priority #1)",
					"node_id", event.NodeID,
					"node_name", event.NodeName,
				)
				rawDataSent = true
			}
		}()
	}

	// PRIORITY 2: Try to send formatted message (but don't let this interfere with raw data)
	func() {
		defer func() {
			if r := recover(); r != nil {
				h.logger.Error("[handlers.telegram] panic while formatting message (raw data already sent)", "panic", r)
				formattedMessageErr = fmt.Errorf("panic during message formatting: %v", r)
			}
		}()

		message := h.formatMessage(event)
		if err := h.sendMessage(ctx, message); err != nil {
			formattedMessageErr = err
			h.logger.Warn("[handlers.telegram] failed to send formatted telegram notification",
				"node_id", event.NodeID,
				"node_name", event.NodeName,
				"error", err,
			)
		} else {
			h.logger.Debug("[handlers.telegram] successfully sent formatted notification",
				"node_id", event.NodeID,
				"node_name", event.NodeName,
			)
		}
	}()

	// Determine success/failure based on configuration and what was sent
	if h.sendRaw {
		// When send_raw is enabled, success is determined by whether raw data was sent
		if rawDataSent {
			h.logger.Info("[handlers.telegram] notification SUCCESS - raw data sent (incident captured)",
				"node_id", event.NodeID,
				"node_name", event.NodeName,
				"formatted_sent", formattedMessageErr == nil,
				"raw_sent", true,
			)
			return nil
		} else {
			// This should be extremely rare since we have multiple fallbacks
			h.logger.Error("[handlers.telegram] CRITICAL: failed to send raw data despite all fallbacks",
				"node_id", event.NodeID,
				"node_name", event.NodeName,
				"raw_error", rawMessageErr,
				"formatted_error", formattedMessageErr,
			)
			return fmt.Errorf("CRITICAL: failed to capture incident data - all raw message attempts failed: %v", rawMessageErr)
		}
	} else {
		// Raw data not enabled, only care about formatted message
		if formattedMessageErr != nil {
			h.logger.Error("[handlers.telegram] failed to send telegram notification",
				"node_id", event.NodeID,
				"node_name", event.NodeName,
				"error", formattedMessageErr,
			)
			return fmt.Errorf("failed to send Telegram notification: %w", formattedMessageErr)
		}
		h.logger.Info("[handlers.telegram] successfully sent telegram notification",
			"node_id", event.NodeID,
			"node_name", event.NodeName,
		)
		return nil
	}
}

// formatMessage formats the termination event into a readable Telegram message
func (h *TelegramHandler) formatMessage(event cloud.TerminationEvent) string {
	// Add panic recovery to ensure we always return something
	defer func() {
		if r := recover(); r != nil {
			h.logger.Error("[handlers.telegram] panic in formatMessage", "panic", r)
		}
	}()

	// Safely format termination time
	var terminationTime string
	func() {
		defer func() {
			if r := recover(); r != nil {
				terminationTime = "Invalid time format"
			}
		}()
		if event.TerminationTime.IsZero() {
			terminationTime = "Unknown"
		} else {
			terminationTime = event.TerminationTime.Format("2006-01-02 15:04:05 MST")
		}
	}()

	// Safely format grace period
	var gracePeriodStr string
	func() {
		defer func() {
			if r := recover(); r != nil {
				gracePeriodStr = "Unknown"
			}
		}()
		if event.GracePeriod <= 0 {
			gracePeriodStr = "Unknown"
		} else if event.GracePeriod >= time.Minute {
			gracePeriodStr = fmt.Sprintf("%.1f minutes", event.GracePeriod.Minutes())
		} else {
			gracePeriodStr = fmt.Sprintf("%.0f seconds", event.GracePeriod.Seconds())
		}
	}()

	// Safely escape all fields
	safeNodeName := escapeMarkdown(event.NodeName)
	if safeNodeName == "" {
		safeNodeName = "Unknown"
	}

	safeNodeID := escapeMarkdown(event.NodeID)
	if safeNodeID == "" {
		safeNodeID = "Unknown"
	}

	safeRegion := escapeMarkdown(event.Region)
	if safeRegion == "" {
		safeRegion = "Unknown"
	}

	safeZone := escapeMarkdown(event.Zone)
	if safeZone == "" {
		safeZone = "Unknown"
	}

	safeProvider := escapeMarkdown(event.CloudProvider)
	if safeProvider == "" {
		safeProvider = "Unknown"
	}

	safeInstanceType := escapeMarkdown(event.InstanceType)
	if safeInstanceType == "" {
		safeInstanceType = "Unknown"
	}

	safeReason := escapeMarkdown(string(event.Reason))
	if safeReason == "" {
		safeReason = "Unknown"
	}

	// Create message with emojis and formatting
	message := fmt.Sprintf(`🚨 *Node Termination Alert*

⚠️ *Node:* %s
🆔 *Node ID:* %s
📍 *Location:* %s/%s
☁️ *Provider:* %s
💾 *Instance Type:* %s

🔴 *Reason:* %s
⏰ *Termination Time:* %s
⏳ *Grace Period:* %s

Node evacuation process has been initiated.`,
		safeNodeName,
		safeNodeID,
		safeRegion,
		safeZone,
		safeProvider,
		safeInstanceType,
		safeReason,
		escapeMarkdown(terminationTime),
		escapeMarkdown(gracePeriodStr),
	)

	return message
}

// formatRawMessage formats the termination event as raw JSON data for Telegram
func (h *TelegramHandler) formatRawMessage(event cloud.TerminationEvent) string {
	// Always try to send something, even if JSON marshaling fails
	defer func() {
		if r := recover(); r != nil {
			h.logger.Error("[handlers.telegram] panic in formatRawMessage", "panic", r)
		}
	}()

	// Try to convert event to JSON with pretty formatting
	jsonData, err := json.MarshalIndent(event, "", "  ")
	if err != nil {
		h.logger.Error("[handlers.telegram] failed to marshal event to JSON, sending fallback data", "error", err)

		// Create a fallback message with basic event information
		fallbackData := map[string]interface{}{
			"marshal_error":        err.Error(),
			"node_id":              event.NodeID,
			"node_name":            event.NodeName,
			"reason":               string(event.Reason),
			"cloud_provider":       event.CloudProvider,
			"region":               event.Region,
			"zone":                 event.Zone,
			"instance_type":        event.InstanceType,
			"termination_time":     event.TerminationTime.Format(time.RFC3339),
			"notice_time":          event.NoticeTime.Format(time.RFC3339),
			"grace_period_seconds": event.GracePeriod.Seconds(),
		}

		// Try to marshal the fallback data
		if fallbackJSON, fallbackErr := json.MarshalIndent(fallbackData, "", "  "); fallbackErr == nil {
			jsonData = fallbackJSON
		} else {
			// If even the fallback fails, create a simple text representation
			h.logger.Error("[handlers.telegram] failed to marshal fallback data", "fallback_error", fallbackErr)
			return fmt.Sprintf(`📊 *Raw Event Data* (Fallback)

❌ JSON Marshal Error: %s

Basic Event Info:
• Node ID: %s
• Node Name: %s  
• Reason: %s
• Provider: %s
• Region/Zone: %s/%s
• Instance Type: %s
• Termination Time: %s
• Grace Period: %.0f seconds

Note: This is a fallback representation due to JSON formatting issues.`,
				escapeMarkdown(err.Error()),
				escapeMarkdown(event.NodeID),
				escapeMarkdown(event.NodeName),
				escapeMarkdown(string(event.Reason)),
				escapeMarkdown(event.CloudProvider),
				escapeMarkdown(event.Region),
				escapeMarkdown(event.Zone),
				escapeMarkdown(event.InstanceType),
				escapeMarkdown(event.TerminationTime.Format(time.RFC3339)),
				event.GracePeriod.Seconds(),
			)
		}
	}

	// Create message with raw JSON data
	message := fmt.Sprintf("📊 *Raw Event Data*\n\n```json\n%s\n```", string(jsonData))

	return message
}

// createEmergencyRawMessage creates an ultra-simple emergency message when all else fails
// This is the absolute last resort to ensure incident data is captured
func (h *TelegramHandler) createEmergencyRawMessage(event cloud.TerminationEvent, errorContext string) string {
	// Use the most basic string formatting to avoid any potential JSON/formatting issues
	// This should work even in the most catastrophic failure scenarios

	// Safely extract basic fields without any complex processing
	nodeID := event.NodeID
	if nodeID == "" {
		nodeID = "UNKNOWN"
	}

	nodeName := event.NodeName
	if nodeName == "" {
		nodeName = "UNKNOWN"
	}

	reason := string(event.Reason)
	if reason == "" {
		reason = "UNKNOWN"
	}

	provider := event.CloudProvider
	if provider == "" {
		provider = "UNKNOWN"
	}

	// Create a basic message that should always work
	message := fmt.Sprintf(`🆘 EMERGENCY RAW DATA CAPTURE

⚠️ INCIDENT DETECTED - EMERGENCY FALLBACK ACTIVE

Error Context: %s

=== BASIC INCIDENT DATA ===
Node ID: %s
Node Name: %s
Reason: %s
Provider: %s
Region: %s
Zone: %s
Instance Type: %s

=== TIMESTAMPS ===
Termination Time: %s
Notice Time: %s
Grace Period: %v

=== TECHNICAL INFO ===
This is an emergency capture due to processing failure.
Raw event object could not be fully processed.
Contact system administrators immediately.

INCIDENT TIMESTAMP: %s`,
		errorContext,
		nodeID,
		nodeName,
		reason,
		provider,
		event.Region,
		event.Zone,
		event.InstanceType,
		event.TerminationTime.String(),
		event.NoticeTime.String(),
		event.GracePeriod,
		time.Now().Format(time.RFC3339),
	)

	return message
}

// sendMessage sends a message to Telegram
func (h *TelegramHandler) sendMessage(ctx context.Context, text string) error {
	// Prepare the message payload
	message := TelegramMessage{
		ChatID:    h.chatID,
		Text:      text,
		ParseMode: "Markdown",
	}

	// Marshal to JSON
	payload, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	// Create the request
	url := fmt.Sprintf("%s/bot%s/sendMessage", telegramAPIBaseURL, h.botToken)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(payload))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	h.logger.Debug("[handlers.telegram] sending telegram message", "url", url, "chat_id", h.chatID)

	// Send the request
	resp, err := h.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// Parse the response
	var telegramResp TelegramResponse
	if err := json.NewDecoder(resp.Body).Decode(&telegramResp); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	// Check if the request was successful
	if !telegramResp.OK {
		return fmt.Errorf("telegram API error (code %d): %s", telegramResp.ErrorCode, telegramResp.Description)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	h.logger.Debug("[handlers.telegram] telegram message sent successfully", "chat_id", h.chatID)

	return nil
}

// escapeMarkdown escapes special characters for Telegram's Markdown parser
func escapeMarkdown(text string) string {
	// Escape special Markdown characters for Telegram
	replacer := []string{
		"_", "\\_",
		"*", "\\*",
		"[", "\\[",
		"]", "\\]",
		"(", "\\(",
		")", "\\)",
		"~", "\\~",
		"`", "\\`",
		">", "\\>",
		"#", "\\#",
		"+", "\\+",
		"-", "\\-",
		"=", "\\=",
		"|", "\\|",
		"{", "\\{",
		"}", "\\}",
		".", "\\.",
		"!", "\\!",
	}

	for i := 0; i < len(replacer); i += 2 {
		text = strings.Replace(text, replacer[i], replacer[i+1], -1)
	}

	return text
}

// ValidateConfig validates the Telegram handler configuration
func ValidateConfig(config TelegramConfig) error {
	if config.BotToken == "" {
		return fmt.Errorf("bot token is required")
	}

	if config.ChatID == "" {
		return fmt.Errorf("chat ID is required")
	}

	if config.Timeout < 0 {
		return fmt.Errorf("timeout cannot be negative")
	}

	return nil
}
