package evacuator

import "log/slog"

type HandlerTelegram struct {
	logger *slog.Logger
}

func NewTelegramHandler() *HandlerTelegram {
	return &HandlerTelegram{}
}

func (h *HandlerTelegram) HandleTermination(e <-chan TerminationEvent) {
	h.logger.Info("telegram handler fired")
}
