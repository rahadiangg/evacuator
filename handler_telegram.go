package evacuator

import "log/slog"

type HandlerTelegram struct {
	logger *slog.Logger
}

func NewTelegramHandler(logger *slog.Logger) *HandlerTelegram {
	return &HandlerTelegram{
		logger: logger,
	}
}

func (h *HandlerTelegram) HandleTermination(e <-chan TerminationEvent) {
	h.logger.Info("telegram handler fired")
}
