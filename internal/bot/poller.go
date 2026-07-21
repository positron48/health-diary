package bot

import (
	"context"
	"fmt"
	"log/slog"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func RunLongPolling(ctx context.Context, token string, handler *Handler, log *slog.Logger) error {
	api, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return fmt.Errorf("create telegram client: %w", err)
	}
	updates := api.GetUpdatesChan(tgbotapi.NewUpdate(0))
	defer api.StopReceivingUpdates()
	for {
		select {
		case <-ctx.Done():
			return nil
		case update, ok := <-updates:
			if !ok {
				return nil
			}
			if err := handler.Handle(ctx, api, update); err != nil {
				log.Error("telegram update handling failed", "error", err)
			}
		}
	}
}
