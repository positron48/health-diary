package ingest

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"strings"
	"time"

	"health-diary/internal/crypto"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Service struct {
	db          *pgxpool.Pool
	cipher      *crypto.Cipher
	maxAttempts int
}
type Capture struct {
	UpdateID, TelegramUserID, MessageID int64
	Username, Text                      string
	SentAt                              time.Time
}
type Result struct {
	EntryID   string
	Duplicate bool
}

type WebCapture struct {
	UserID         string
	Text           string
	IdempotencyKey string
	SentAt         time.Time
	ReferenceAt    time.Time
}

func New(db *pgxpool.Pool, cipher *crypto.Cipher, maxAttempts int) *Service {
	return &Service{db: db, cipher: cipher, maxAttempts: maxAttempts}
}

func (s *Service) CaptureTelegramText(ctx context.Context, in Capture) (Result, error) {
	if in.UpdateID <= 0 || in.TelegramUserID <= 0 || in.MessageID <= 0 || in.Text == "" {
		return Result{}, fmt.Errorf("telegram update, user, message and text are required")
	}
	if in.SentAt.IsZero() {
		in.SentAt = time.Now().UTC()
	}
	tx, err := s.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return Result{}, err
	}
	defer tx.Rollback(ctx)
	var inserted bool
	err = tx.QueryRow(ctx, `INSERT INTO telegram_updates(update_id,update_kind,result) VALUES($1,'message','accepted') ON CONFLICT (update_id) DO NOTHING RETURNING true`, in.UpdateID).Scan(&inserted)
	if err == pgx.ErrNoRows {
		return Result{Duplicate: true}, nil
	}
	if err != nil {
		return Result{}, err
	}
	var userID string
	err = tx.QueryRow(ctx, `INSERT INTO users(telegram_user_id,telegram_username) VALUES($1,NULLIF($2,'')) ON CONFLICT(telegram_user_id) DO UPDATE SET telegram_username=EXCLUDED.telegram_username,updated_at=now() RETURNING id::text`, in.TelegramUserID, in.Username).Scan(&userID)
	if err != nil {
		return Result{}, err
	}
	if _, err = tx.Exec(ctx, `UPDATE telegram_updates SET user_id=$2,processed_at=now() WHERE update_id=$1`, in.UpdateID, userID); err != nil {
		return Result{}, err
	}
	sealed, err := s.cipher.Encrypt([]byte(in.Text), []byte(userID))
	if err != nil {
		return Result{}, err
	}
	digest := sha256.Sum256([]byte(in.Text))
	var entryID string
	err = tx.QueryRow(ctx, `INSERT INTO journal_entries(user_id,source_type,source_message_id,source_sent_at,raw_text_ciphertext,encryption_key_version,content_sha256) VALUES($1,'telegram_text',$2,$3,$4,$5,$6) ON CONFLICT (user_id,source_type,source_message_id) WHERE source_message_id IS NOT NULL DO UPDATE SET source_message_id=EXCLUDED.source_message_id RETURNING id::text`, userID, in.MessageID, in.SentAt, sealed, s.cipher.Version(), digest[:]).Scan(&entryID)
	if err != nil {
		return Result{}, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO jobs(kind,payload,max_attempts) VALUES('extract_entry',jsonb_build_object('entry_id',$1::text),$2)`, entryID, s.maxAttempts); err != nil {
		return Result{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return Result{}, err
	}
	return Result{EntryID: entryID}, nil
}

func nullableTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.UTC().Format(time.RFC3339)
}

func (s *Service) CaptureWebText(ctx context.Context, in WebCapture) (Result, error) {
	in.Text = strings.TrimSpace(in.Text)
	if in.UserID == "" || in.Text == "" || strings.TrimSpace(in.IdempotencyKey) == "" {
		return Result{}, fmt.Errorf("user, text and idempotency key are required")
	}
	if in.SentAt.IsZero() {
		in.SentAt = time.Now().UTC()
	}
	keyDigest := sha256.Sum256([]byte(in.IdempotencyKey))
	messageID := int64(binary.BigEndian.Uint64(keyDigest[:8]) & uint64(^uint64(0)>>1))
	textDigest := sha256.Sum256([]byte(in.Text))
	sealed, err := s.cipher.Encrypt([]byte(in.Text), []byte(in.UserID))
	if err != nil {
		return Result{}, err
	}
	tx, err := s.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return Result{}, err
	}
	defer tx.Rollback(ctx)
	var entryID string
	err = tx.QueryRow(ctx, `INSERT INTO journal_entries(user_id,source_type,source_message_id,source_sent_at,raw_text_ciphertext,encryption_key_version,content_sha256)
		VALUES($1,'web',$2,$3,$4,$5,$6)
		ON CONFLICT (user_id,source_type,source_message_id) WHERE source_message_id IS NOT NULL DO NOTHING
		RETURNING id::text`, in.UserID, messageID, in.SentAt, sealed, s.cipher.Version(), textDigest[:]).Scan(&entryID)
	if err == pgx.ErrNoRows {
		if err = tx.QueryRow(ctx, `SELECT id::text FROM journal_entries WHERE user_id=$1 AND source_type='web' AND source_message_id=$2`, in.UserID, messageID).Scan(&entryID); err != nil {
			return Result{}, err
		}
		return Result{EntryID: entryID, Duplicate: true}, nil
	}
	if err != nil {
		return Result{}, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO jobs(kind,payload,max_attempts) VALUES('extract_entry',jsonb_strip_nulls(jsonb_build_object('entry_id',$1::text,'reference_at',NULLIF($2::text,'')::timestamptz)),$3)`, entryID, nullableTime(in.ReferenceAt), s.maxAttempts); err != nil {
		return Result{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return Result{}, err
	}
	return Result{EntryID: entryID}, nil
}
