package bot

import (
	"context"
	"log/slog"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func RunLongPolling(ctx context.Context, token, socks5Address string, handler *Handler, log *slog.Logger) error {
	api, err := NewAPI(token, socks5Address)
	if err != nil {
		return err
	}
	if _, err = api.Request(tgbotapi.DeleteWebhookConfig{DropPendingUpdates: false}); err != nil {
		return telegramAPIError("disable Telegram webhook for long polling")
	}
	// Keep Telegram's long-poll request below the HTTP client's 40 second
	// timeout. Leaving Timeout at the library default (zero) is unreliable
	// through the production SOCKS5 tunnel and causes retry loops.
	updateConfig := tgbotapi.NewUpdate(0)
	updateConfig.Timeout = 25
	updateConfig.AllowedUpdates = []string{"message", "callback_query"}
	updates := api.GetUpdatesChan(updateConfig)
	defer api.StopReceivingUpdates()
	outboxTicker := time.NewTicker(time.Second)
	defer outboxTicker.Stop()
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
		case <-outboxTicker.C:
			if err := DispatchOneOutbox(ctx, api, handler.db); err != nil {
				log.Error("telegram outbox dispatch failed", "error", err)
			}
		}
	}
}
