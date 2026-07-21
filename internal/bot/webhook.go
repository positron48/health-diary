package bot

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"net/http"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ConfigureWebhook registers Telegram's HTTPS endpoint with its secret header.
func ConfigureWebhook(token, url, secret, socks5Address string) (*tgbotapi.BotAPI, error) {
	api, err := NewAPI(token, socks5Address)
	if err != nil {
		return nil, err
	}
	params := tgbotapi.Params{"url": url, "secret_token": secret, "allowed_updates": `["message","callback_query"]`}
	if _, err = api.MakeRequest("setWebhook", params); err != nil {
		return nil, err
	}
	return api, nil
}

func WebhookHandler(api *tgbotapi.BotAPI, handler *Handler, secret string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || subtle.ConstantTimeCompare([]byte(r.Header.Get("X-Telegram-Bot-Api-Secret-Token")), []byte(secret)) != 1 {
			http.NotFound(w, r)
			return
		}
		defer r.Body.Close()
		var update tgbotapi.Update
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&update); err != nil {
			http.Error(w, "invalid update", 400)
			return
		}
		if err := handler.Handle(r.Context(), api, update); err != nil {
			http.Error(w, "unable to process update", 500)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
}

func RunOutbox(ctx context.Context, api *tgbotapi.BotAPI, db *pgxpool.Pool) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_ = DispatchOneOutbox(ctx, api, db)
		}
	}
}
