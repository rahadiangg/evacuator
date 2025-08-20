package evacuator

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"

	"github.com/mymmrac/telego"
)

type TelegramHandler struct {
	logger *slog.Logger
	bot    *telego.Bot
	chatID telego.ChatID
}

func NewTelegramHandler(logger *slog.Logger, botToken string, chatID string) (*TelegramHandler, error) {

	bot, err := telego.NewBot(botToken)
	if err != nil {
		return nil, fmt.Errorf("failed to create telegram bot: %w", err)
	}

	// Parse chat ID to int64
	chatIDInt, err := strconv.ParseInt(chatID, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid chat ID format: %w", err)
	}

	return &TelegramHandler{
		logger: logger,
		bot:    bot,
		chatID: telego.ChatID{ID: chatIDInt},
	}, nil
}

func (h *TelegramHandler) Name() string {
	return "telegram"
}

func (h *TelegramHandler) HandleTermination(ctx context.Context, event TerminationEvent) error {
	// Validate termination event first
	if err := h.validateTerminationEvent(event); err != nil {
		h.logger.Error("invalid termination event received", "error", err.Error(), "handler", h.Name())
		return err
	}

	// Format the message
	message := fmt.Sprintf(`🚨 *Node Termination Alert*

⚠️ *Hostname:* %s
🆔 *Instance ID:* %s
🌐 *Private IP:* %s
🔴 *Reason:* %s

Node evacuation process has been initiated\.`,
		escapeMarkdown(event.Hostname),
		escapeMarkdown(event.InstanceID),
		escapeMarkdown(event.PrivateIP),
		escapeMarkdown(string(event.Reason)),
	)

	// Send message using telego
	_, err := h.bot.SendMessage(ctx, &telego.SendMessageParams{
		ChatID:    h.chatID,
		Text:      message,
		ParseMode: telego.ModeMarkdownV2,
	})

	if err != nil {
		h.logger.Error("failed to send telegram message", "error", err.Error(), "handler", h.Name())
		return fmt.Errorf("failed to send telegram notification: %w", err)
	}

	h.logger.Info("termination event processed successfully", "handler", h.Name())
	return nil
}

// validateTerminationEvent validates that a termination event has required fields.
func (h *TelegramHandler) validateTerminationEvent(event TerminationEvent) error {
	if event.Hostname == "" {
		return errors.New("hostname cannot be empty")
	}
	if event.InstanceID == "" {
		return errors.New("instance ID cannot be empty")
	}
	if event.Reason == "" {
		return errors.New("termination reason cannot be empty")
	}
	return nil
}

// escapeMarkdown escapes special characters for Telegram's MarkdownV2 parser
func escapeMarkdown(text string) string {
	// Characters that need to be escaped in MarkdownV2: \ _ * [ ] ( ) ~ ` > # + - = | { } . !
	// Note: backslash must be escaped first to avoid double-escaping

	result := text

	// Escape backslash first
	result = strings.ReplaceAll(result, "\\", "\\\\")

	// Then escape other special characters
	specialChars := []string{"_", "*", "[", "]", "(", ")", "~", "`", ">", "#", "+", "-", "=", "|", "{", "}", ".", "!"}
	for _, char := range specialChars {
		result = strings.ReplaceAll(result, char, "\\"+char)
	}

	return result
}
