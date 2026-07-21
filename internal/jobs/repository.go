package jobs

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type Job struct {
	ID                    string
	Kind                  string
	Payload               json.RawMessage
	Attempts, MaxAttempts int
}
type Repository struct {
	db interface {
		Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
		QueryRow(context.Context, string, ...any) pgx.Row
	}
}

func NewRepository(db interface {
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
	QueryRow(context.Context, string, ...any) pgx.Row
}) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Enqueue(ctx context.Context, kind string, payload any, maxAttempts int) error {
	if kind == "" || maxAttempts < 1 {
		return fmt.Errorf("job kind and positive max attempts are required")
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal job payload: %w", err)
	}
	_, err = r.db.Exec(ctx, `INSERT INTO jobs (kind,payload,max_attempts) VALUES ($1,$2,$3)`, kind, body, maxAttempts)
	return err
}

func (r *Repository) Claim(ctx context.Context, workerID string) (*Job, error) {
	if workerID == "" {
		return nil, fmt.Errorf("worker id is required")
	}
	row := r.db.QueryRow(ctx, `WITH candidate AS (SELECT id FROM jobs WHERE status IN ('queued','retryable_failed') AND available_at <= now() ORDER BY available_at,id FOR UPDATE SKIP LOCKED LIMIT 1) UPDATE jobs j SET status='running',locked_at=now(),locked_by=$1,attempts=j.attempts+1,updated_at=now() FROM candidate WHERE j.id=candidate.id RETURNING j.id::text,j.kind,j.payload,j.attempts,j.max_attempts`, workerID)
	var job Job
	if err := row.Scan(&job.ID, &job.Kind, &job.Payload, &job.Attempts, &job.MaxAttempts); err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &job, nil
}

func (r *Repository) Finish(ctx context.Context, id string, retryable bool, errorCode string) error {
	status := "succeeded"
	availableAt := time.Now()
	if errorCode != "" {
		status = "terminal_failed"
		if retryable {
			status = "retryable_failed"
			availableAt = time.Now().Add(time.Minute)
		}
	}
	_, err := r.db.Exec(ctx, `UPDATE jobs SET status=CASE WHEN $2='retryable_failed' AND attempts >= max_attempts THEN 'terminal_failed' ELSE $2 END,available_at=$3,last_error_code=NULLIF($4,''),locked_at=NULL,locked_by=NULL,updated_at=now() WHERE id=$1 AND status='running'`, id, status, availableAt, errorCode)
	return err
}
