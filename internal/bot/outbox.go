package bot

import (
	"context"
	"encoding/json"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// DispatchOneOutbox sends one durable confirmation. The message body contains
// only derived facts; callback tokens are random and are stored hashed.
func DispatchOneOutbox(ctx context.Context, api *tgbotapi.BotAPI, db *pgxpool.Pool) error {
	tx, err := db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var id string
	var payload json.RawMessage
	err = tx.QueryRow(ctx, `WITH candidate AS (SELECT id FROM outbox_messages WHERE kind IN ('telegram_confirmation','telegram_processing_failed') AND status IN ('queued','retryable_failed') AND available_at<=now() ORDER BY available_at,id FOR UPDATE SKIP LOCKED LIMIT 1) UPDATE outbox_messages o SET status='running',attempts=attempts+1,updated_at=now() FROM candidate WHERE o.id=candidate.id RETURNING o.id::text,o.payload`).Scan(&id, &payload)
	if err == pgx.ErrNoRows {
		return nil
	}
	if err != nil {
		return err
	}
	if err = tx.Commit(ctx); err != nil {
		return err
	}
	var message struct {
		ChatID       int64  `json:"chat_id"`
		Text         string `json:"text"`
		ConfirmToken string `json:"confirm_token"`
		RejectToken  string `json:"reject_token"`
	}
	if err = json.Unmarshal(payload, &message); err != nil {
		return finishOutbox(ctx, db, id, false, "invalid_payload")
	}
	config := tgbotapi.NewMessage(message.ChatID, message.Text)
	if message.ConfirmToken != "" && message.RejectToken != "" {
		config.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData("Верно", "v1:"+message.ConfirmToken+":confirm"), tgbotapi.NewInlineKeyboardButtonData("Отклонить", "v1:"+message.RejectToken+":reject")))
	}
	if _, err = api.Send(config); err != nil {
		return finishOutbox(ctx, db, id, true, "telegram_send_failed")
	}
	return finishOutbox(ctx, db, id, false, "")
}

func finishOutbox(ctx context.Context, db *pgxpool.Pool, id string, retry bool, code string) error {
	query := `UPDATE outbox_messages SET status=CASE WHEN $2 THEN 'retryable_failed' WHEN $3<>'' THEN 'terminal_failed' ELSE 'succeeded' END,available_at=CASE WHEN $2 THEN now()+interval '1 minute' ELSE available_at END,last_error_code=NULLIF($3,''),updated_at=now() WHERE id=$1 AND status='running'`
	_, err := db.Exec(ctx, query, id, retry, code)
	return err
}
