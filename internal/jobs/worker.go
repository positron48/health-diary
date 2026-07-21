package jobs

import (
	"context"
	"encoding/json"
	"fmt"

	"health-diary/internal/auth"
	"health-diary/internal/crypto"
	"health-diary/internal/llm"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Worker struct {
	db        *pgxpool.Pool
	queue     *Repository
	cipher    *crypto.Cipher
	extractor llm.Extractor
	workerID  string
	provider  string
	model     string
}

func NewWorker(db *pgxpool.Pool, cipher *crypto.Cipher, extractor llm.Extractor, workerID, provider, model string) *Worker {
	return &Worker{db: db, queue: NewRepository(db), cipher: cipher, extractor: extractor, workerID: workerID, provider: provider, model: model}
}
func (w *Worker) RunOnce(ctx context.Context) error {
	job, err := w.queue.Claim(ctx, w.workerID)
	if err != nil || job == nil {
		return err
	}
	if job.Kind == "delete_user" {
		return w.deleteUser(ctx, job)
	}
	if job.Kind != "extract_entry" {
		return w.queue.Finish(ctx, job.ID, false, "unsupported_job")
	}
	var payload struct {
		EntryID string `json:"entry_id"`
	}
	if err := json.Unmarshal(job.Payload, &payload); err != nil {
		return w.queue.Finish(ctx, job.ID, false, "invalid_payload")
	}
	if err := w.extract(ctx, payload.EntryID, job.Attempts); err != nil {
		_ = w.queue.Finish(ctx, job.ID, true, "extraction_failed")
		return err
	}
	return w.queue.Finish(ctx, job.ID, false, "")
}

func (w *Worker) deleteUser(ctx context.Context, job *Job) error {
	var payload struct {
		UserID  string `json:"user_id"`
		AuditID string `json:"audit_id"`
	}
	if err := json.Unmarshal(job.Payload, &payload); err != nil {
		return w.queue.Finish(ctx, job.ID, false, "invalid_payload")
	}
	tx, err := w.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	// Explicit order keeps audit/reference tables valid while removing every
	// application-held health record for the account.
	queries := []string{
		`DELETE FROM telegram_callback_actions WHERE user_id=$1`, `DELETE FROM outbox_messages WHERE user_id=$1`, `DELETE FROM auth_challenges WHERE user_id=$1`, `DELETE FROM web_sessions WHERE user_id=$1`,
		`DELETE FROM event_revisions WHERE event_id IN (SELECT id FROM health_events WHERE user_id=$1)`, `DELETE FROM health_events WHERE user_id=$1`, `DELETE FROM event_batches WHERE user_id=$1`, `DELETE FROM extraction_runs WHERE entry_id IN (SELECT id FROM journal_entries WHERE user_id=$1)`, `DELETE FROM jobs WHERE kind='extract_entry' AND payload->>'entry_id' IN (SELECT id::text FROM journal_entries WHERE user_id=$1)`, `DELETE FROM journal_entries WHERE user_id=$1`, `DELETE FROM users WHERE id=$1 AND status='deletion_pending'`,
	}
	for _, query := range queries {
		if _, err = tx.Exec(ctx, query, payload.UserID); err != nil {
			return err
		}
	}
	if _, err = tx.Exec(ctx, `UPDATE deletion_audits SET status='completed',completed_at=now() WHERE id=$1 AND status='queued'`, payload.AuditID); err != nil {
		return err
	}
	if err = tx.Commit(ctx); err != nil {
		return err
	}
	// The completed job itself must not retain the deleted account identifier.
	_, err = w.db.Exec(ctx, `UPDATE jobs SET status='succeeded',payload='{}'::jsonb,locked_at=NULL,locked_by=NULL,last_error_code=NULL,updated_at=now() WHERE id=$1 AND status='running'`, job.ID)
	return err
}
func (w *Worker) extract(ctx context.Context, entryID string, attempt int) error {
	tx, err := w.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var userID string
	var telegramUserID *int64
	var sealed []byte
	if err := tx.QueryRow(ctx, `SELECT e.user_id::text,u.telegram_user_id,e.raw_text_ciphertext FROM journal_entries e JOIN users u ON u.id=e.user_id WHERE e.id=$1 AND e.deleted_at IS NULL FOR UPDATE`, entryID).Scan(&userID, &telegramUserID, &sealed); err != nil {
		return err
	}
	plain, err := w.cipher.Decrypt(sealed, []byte(userID))
	if err != nil {
		return err
	}
	result, err := w.extractor.Extract(ctx, string(plain))
	if err != nil {
		return err
	}
	if err := llm.ValidateResult(result); err != nil {
		return fmt.Errorf("validate extraction result: %w", err)
	}
	validatedResult, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("marshal validated result: %w", err)
	}
	var runID, batchID string
	if err := tx.QueryRow(ctx, `INSERT INTO extraction_runs(entry_id,attempt,provider,model,prompt_version,schema_version,context_fingerprint,status,validated_result,finished_at) VALUES($1,$2,$3,$4,'health-entry-v1','health-entry-v1','', 'succeeded',$5,now()) RETURNING id::text`, entryID, attempt, w.provider, w.model, validatedResult).Scan(&runID); err != nil {
		return err
	}
	if err := tx.QueryRow(ctx, `INSERT INTO event_batches(user_id,entry_id,extraction_run_id) VALUES($1,$2,$3) RETURNING id::text`, userID, entryID, runID).Scan(&batchID); err != nil {
		return err
	}
	for _, event := range result.Events {
		attributes, err := json.Marshal(event.Data)
		if err != nil {
			return fmt.Errorf("marshal event attributes: %w", err)
		}
		if _, err := tx.Exec(ctx, `INSERT INTO health_events(user_id,batch_id,entry_id,kind,occurred_at,time_precision,client_ref,attributes) VALUES($1,$2,$3,$4,$5,$6,$7,$8)`, userID, batchID, entryID, event.Kind, event.OccurredAt, event.TimePrecision, event.ClientRef, attributes); err != nil {
			return fmt.Errorf("insert event: %w", err)
		}
	}
	if telegramUserID != nil {
		confirmToken, confirmHash, err := auth.NewOpaqueToken()
		if err != nil {
			return err
		}
		rejectToken, rejectHash, err := auth.NewOpaqueToken()
		if err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `INSERT INTO telegram_callback_actions(token_hash,user_id,batch_id,batch_version,action,expires_at) VALUES($1,$2,$3,1,'confirm',now()+interval '7 days'),($4,$2,$3,1,'reject',now()+interval '7 days')`, confirmHash, userID, batchID, rejectHash); err != nil {
			return err
		}
		payload := map[string]any{"chat_id": *telegramUserID, "text": fmt.Sprintf("Распознано событий: %d. Подтвердите только те факты, которые верны.", len(result.Events)), "confirm_token": confirmToken, "reject_token": rejectToken}
		if _, err = tx.Exec(ctx, `INSERT INTO outbox_messages(user_id,kind,payload) VALUES($1,'telegram_confirmation',$2)`, userID, payload); err != nil {
			return err
		}
	}
	_, err = tx.Exec(ctx, `UPDATE journal_entries SET processing_status='parsed' WHERE id=$1`, entryID)
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}
