package journal

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Batches struct{ db *pgxpool.Pool }

func NewBatches(db *pgxpool.Pool) *Batches { return &Batches{db} }
func (b *Batches) Confirm(ctx context.Context, userID, id string, version int) error {
	return b.transition(ctx, userID, id, version, "confirmed")
}
func (b *Batches) Reject(ctx context.Context, userID, id string, version int) error {
	return b.transition(ctx, userID, id, version, "rejected")
}
func (b *Batches) transition(ctx context.Context, userID, id string, version int, status string) error {
	tx, err := b.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	tag, err := tx.Exec(ctx, `UPDATE event_batches SET status=$4,version=version+1,confirmed_at=CASE WHEN $4='confirmed' THEN now() ELSE confirmed_at END,rejected_at=CASE WHEN $4='rejected' THEN now() ELSE rejected_at END WHERE id=$1 AND user_id=$2 AND version=$3 AND status='pending'`, id, userID, version, status)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return fmt.Errorf("batch not pending or stale")
	}
	eventStatus := "superseded"
	if status == "confirmed" {
		eventStatus = "confirmed"
	}
	_, err = tx.Exec(ctx, `UPDATE health_events SET status=$2,updated_at=now() WHERE batch_id=$1 AND status='pending'`, id, eventStatus)
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}
