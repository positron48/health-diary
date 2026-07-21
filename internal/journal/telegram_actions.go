package journal

import (
	"context"
	"fmt"

	"health-diary/internal/auth"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ApplyTelegramAction consumes a one-time opaque callback action and applies
// its versioned batch transition in the same transaction.
func ApplyTelegramAction(ctx context.Context, db *pgxpool.Pool, telegramUserID int64, token, action string) error {
	tx, err := db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var userID, batchID string
	var version int
	err = tx.QueryRow(ctx, `SELECT a.user_id::text,a.batch_id::text,a.batch_version FROM telegram_callback_actions a
        JOIN users u ON u.id=a.user_id
        WHERE a.token_hash=$1 AND a.action=$2 AND a.used_at IS NULL AND a.expires_at>now() AND u.telegram_user_id=$3
        FOR UPDATE`, auth.Hash(token), action, telegramUserID).Scan(&userID, &batchID, &version)
	if err != nil {
		if err == pgx.ErrNoRows {
			return fmt.Errorf("action unavailable")
		}
		return err
	}
	status, eventStatus := "rejected", "superseded"
	if action == "confirm" {
		status, eventStatus = "confirmed", "confirmed"
	}
	tag, err := tx.Exec(ctx, `UPDATE event_batches SET status=$4,version=version+1,confirmed_at=CASE WHEN $4='confirmed' THEN now() ELSE confirmed_at END,rejected_at=CASE WHEN $4='rejected' THEN now() ELSE rejected_at END WHERE id=$1 AND user_id=$2 AND version=$3 AND status='pending'`, batchID, userID, version, status)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return fmt.Errorf("batch unavailable")
	}
	if _, err = tx.Exec(ctx, `UPDATE health_events SET status=$2,updated_at=now() WHERE batch_id=$1 AND status='pending'`, batchID, eventStatus); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `UPDATE telegram_callback_actions SET used_at=now() WHERE token_hash=$1`, auth.Hash(token)); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
