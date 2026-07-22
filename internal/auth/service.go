package auth

import (
	"context"
	"crypto/subtle"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Service struct {
	db                  *pgxpool.Pool
	codeTTL, sessionTTL time.Duration
	maxAttempts         int
}
type Challenge struct {
	ID, Token string
	ExpiresAt time.Time
}

type SessionUser struct {
	ID, Timezone string
	CreatedAt    time.Time
}

func NewService(db *pgxpool.Pool, codeTTL, sessionTTL time.Duration, maxAttempts int) *Service {
	return &Service{db, codeTTL, sessionTTL, maxAttempts}
}

func (s *Service) CreateChallenge(ctx context.Context) (Challenge, error) {
	token, hash, err := NewOpaqueToken()
	if err != nil {
		return Challenge{}, err
	}
	var id string
	expires := time.Now().Add(s.codeTTL)
	err = s.db.QueryRow(ctx, `INSERT INTO auth_challenges(public_token_hash,expires_at,max_attempts) VALUES($1,$2,$3) RETURNING id::text`, hash, expires, s.maxAttempts).Scan(&id)
	return Challenge{ID: id, Token: token, ExpiresAt: expires}, err
}

// BindTelegram creates or refreshes the allowed Telegram user's local account
// before binding a one-time web-login challenge. A first login therefore does
// not depend on the user having sent a journal entry beforehand.
func (s *Service) BindTelegram(ctx context.Context, token string, telegramUserID int64, username string) (string, error) {
	code, codeHash, err := NewCode()
	if err != nil {
		return "", err
	}
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer tx.Rollback(ctx)
	var userID string
	if err = tx.QueryRow(ctx, `INSERT INTO users(telegram_user_id,telegram_username)
        VALUES($1,$2)
        ON CONFLICT (telegram_user_id) DO UPDATE SET telegram_username=EXCLUDED.telegram_username,updated_at=now()
        RETURNING id::text`, telegramUserID, username).Scan(&userID); err != nil {
		return "", err
	}
	tag, err := tx.Exec(ctx, `UPDATE auth_challenges SET user_id=$2,code_hash=$3,bound_at=now() WHERE public_token_hash=$1 AND expires_at>now() AND consumed_at IS NULL AND locked_at IS NULL`, Hash(token), userID, codeHash)
	if err != nil {
		return "", err
	}
	if tag.RowsAffected() != 1 {
		return "", fmt.Errorf("challenge unavailable")
	}
	return code, tx.Commit(ctx)
}

func (s *Service) Verify(ctx context.Context, challengeID, code string) (string, string, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return "", "", err
	}
	defer tx.Rollback(ctx)
	var userID string
	var hash []byte
	var attempts, max int
	var expires time.Time
	var locked, used *time.Time
	err = tx.QueryRow(ctx, `SELECT user_id::text,code_hash,attempt_count,max_attempts,expires_at,locked_at,consumed_at FROM auth_challenges WHERE id=$1 FOR UPDATE`, challengeID).Scan(&userID, &hash, &attempts, &max, &expires, &locked, &used)
	if err != nil {
		return "", "", err
	}
	if used != nil || locked != nil || time.Now().After(expires) || len(hash) == 0 {
		return "", "", fmt.Errorf("challenge unavailable")
	}
	if subtle.ConstantTimeCompare(hash, Hash(code)) != 1 {
		attempts++
		_, err = tx.Exec(ctx, `UPDATE auth_challenges SET attempt_count=$2,locked_at=CASE WHEN $2 >= max_attempts THEN now() ELSE NULL END WHERE id=$1`, challengeID, attempts)
		if err != nil {
			return "", "", err
		}
		if err = tx.Commit(ctx); err != nil {
			return "", "", err
		}
		return "", "", fmt.Errorf("invalid code")
	}
	token, tokenHash, err := NewOpaqueToken()
	if err != nil {
		return "", "", err
	}
	var sessionID string
	err = tx.QueryRow(ctx, `INSERT INTO web_sessions(user_id,token_hash,expires_at) VALUES($1,$2,$3) RETURNING id::text`, userID, tokenHash, time.Now().Add(s.sessionTTL)).Scan(&sessionID)
	if err != nil {
		return "", "", err
	}
	_, err = tx.Exec(ctx, `UPDATE auth_challenges SET consumed_at=now() WHERE id=$1`, challengeID)
	if err != nil {
		return "", "", err
	}
	if err = tx.Commit(ctx); err != nil {
		return "", "", err
	}
	return sessionID, token, nil
}

func (s *Service) SessionUser(ctx context.Context, token string) (SessionUser, error) {
	var user SessionUser
	err := s.db.QueryRow(ctx, `SELECT u.id::text,COALESCE(NULLIF(btrim(u.timezone),''),'Europe/Moscow'),s.created_at FROM web_sessions s JOIN users u ON u.id=s.user_id WHERE s.token_hash=$1 AND s.expires_at>now() AND s.revoked_at IS NULL AND u.status='active'`, Hash(token)).Scan(&user.ID, &user.Timezone, &user.CreatedAt)
	return user, err
}

func (s *Service) RevokeSession(ctx context.Context, token string) error {
	_, err := s.db.Exec(ctx, `UPDATE web_sessions SET revoked_at=now() WHERE token_hash=$1 AND revoked_at IS NULL`, Hash(token))
	return err
}
