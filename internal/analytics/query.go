package analytics

import (
	"context"
	"github.com/jackc/pgx/v5/pgxpool"
	"time"
)

type Event struct {
	ID, Kind   string
	OccurredAt time.Time
	Attributes []byte
}
type Repository struct{ db *pgxpool.Pool }

func New(db *pgxpool.Pool) *Repository { return &Repository{db} }
func (r *Repository) Events(ctx context.Context, userID string, from, to time.Time) ([]Event, error) {
	rows, err := r.db.Query(ctx, `SELECT id::text,kind,occurred_at,attributes FROM health_events WHERE user_id=$1 AND status='confirmed' AND deleted_at IS NULL AND occurred_at >= $2 AND occurred_at < $3 ORDER BY occurred_at,id`, userID, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Event
	for rows.Next() {
		var e Event
		if err := rows.Scan(&e.ID, &e.Kind, &e.OccurredAt, &e.Attributes); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}
