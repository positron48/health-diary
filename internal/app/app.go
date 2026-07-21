package app

import (
	"context"
	"embed"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"health-diary/internal/analytics"
	"health-diary/internal/auth"
	"health-diary/internal/bot"
	"health-diary/internal/config"
	"health-diary/internal/crypto"
	"health-diary/internal/database"
	"health-diary/internal/ingest"
	"health-diary/internal/jobs"
	"health-diary/internal/journal"
	"health-diary/internal/llm"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// webAssets contains the production Vue bundle. Docker replaces the small
// checked-in fallback page with the Vite build before compiling the server.
//
//go:embed web/dist/*
var webAssets embed.FS

type App struct {
	config config.Config
	logger *slog.Logger
	db     *pgxpool.Pool
	auth   *auth.Service
}

func New(cfg config.Config, logger *slog.Logger) *App {
	return &App{config: cfg, logger: logger}
}

func (a *App) Run(ctx context.Context, shutdownTimeout time.Duration) error {
	if a.config.DatabaseURL != "" {
		pool, err := database.Open(ctx, a.config.DatabaseURL)
		if err != nil {
			return err
		}
		defer pool.Close()
		a.db = pool
		a.auth = auth.NewService(pool, a.config.AuthCodeTTL, a.config.SessionTTL, a.config.AuthMaxAttempts)
	}
	if a.config.Telegram.Token != "" {
		if a.config.DataEncryptionKey == "" || len(a.config.Telegram.AllowedUserIDs) == 0 {
			return fmt.Errorf("telegram requires DATA_ENCRYPTION_KEY and TELEGRAM_ALLOWED_USER_IDS")
		}
		cipher, err := crypto.New(a.config.DataEncryptionKey, a.config.DataEncryptionKeyVersion)
		if err != nil {
			return err
		}
		pool := a.db
		if pool == nil {
			return fmt.Errorf("telegram requires DATABASE_URL")
		}
		handler := bot.NewHandler(pool, ingest.New(pool, cipher, a.config.JobMaxAttempts), a.auth, a.config.Telegram.AllowedUserIDs, a.logger)
		var extractor llm.Extractor = llm.Fake{}
		provider, model := "fake", "fake"
		if a.config.LLMAPIKey != "" {
			extractor = llm.NewOpenAICompatible(a.config.LLMBaseURL, a.config.LLMModel, a.config.LLMAPIKey, &http.Client{Timeout: 30 * time.Second})
			provider, model = "polza", a.config.LLMModel
		}
		worker := jobs.NewWorker(pool, cipher, extractor, "app-1", provider, model)
		go func() {
			ticker := time.NewTicker(time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					if err := worker.RunOnce(ctx); err != nil {
						a.logger.Error("job processing failed", "error", err)
					}
				}
			}
		}()
		go func() {
			if err := bot.RunLongPolling(ctx, a.config.Telegram.Token, handler, a.logger); err != nil {
				a.logger.Error("telegram polling stopped", "error", err)
			}
		}()
	}
	server := &http.Server{Addr: a.config.HTTPAddr, Handler: a.Handler(), ReadHeaderTimeout: 5 * time.Second}
	errCh := make(chan error, 1)
	go func() { errCh <- server.ListenAndServe() }()
	a.logger.Info("http server started", "addr", a.config.HTTPAddr)

	select {
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		return server.Shutdown(shutdownCtx)
	}
}

func (a *App) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) { writeText(w, http.StatusOK, "ok\n") })
	mux.HandleFunc("GET /readyz", a.ready)
	mux.HandleFunc("GET /metrics", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
		writeText(w, http.StatusOK, "health_diary_up 1\n")
	})
	mux.HandleFunc("POST /auth/challenges", a.createChallenge)
	mux.HandleFunc("POST /auth/challenges/{id}/verify", a.verifyChallenge)
	mux.Handle("GET /api/me", a.requireSession(http.HandlerFunc(a.me)))
	mux.Handle("GET /calendar", a.requireSession(http.HandlerFunc(a.calendar)))
	mux.Handle("GET /events", a.requireSession(http.HandlerFunc(a.events)))
	mux.Handle("GET /batches/pending", a.requireSession(http.HandlerFunc(a.pendingBatches)))
	mux.Handle("GET /exports", a.requireSession(http.HandlerFunc(a.exportEvents)))
	mux.Handle("DELETE /events/{id}", a.requireSession(http.HandlerFunc(a.deleteEvent)))
	mux.Handle("POST /events/{id}/restore", a.requireSession(http.HandlerFunc(a.restoreEvent)))
	mux.Handle("POST /batches/{id}/confirm", a.requireSession(http.HandlerFunc(a.confirmBatch)))
	mux.Handle("POST /batches/{id}/reject", a.requireSession(http.HandlerFunc(a.rejectBatch)))
	mux.HandleFunc("GET /api/health-data", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_, _ = w.Write([]byte(`{"items":[]}`))
	})
	web, err := fs.Sub(webAssets, "web/dist")
	if err != nil {
		panic(err)
	}
	mux.Handle("GET /", http.FileServer(http.FS(web)))
	return securityHeaders(mux)
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Permissions-Policy", "geolocation=(), microphone=(), camera=()")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; base-uri 'none'; frame-ancestors 'none'; form-action 'self'; connect-src 'self'; style-src 'self' 'unsafe-inline'")
		next.ServeHTTP(w, r)
	})
}

func (a *App) confirmBatch(w http.ResponseWriter, r *http.Request) { a.transitionBatch(w, r, true) }
func (a *App) rejectBatch(w http.ResponseWriter, r *http.Request)  { a.transitionBatch(w, r, false) }
func (a *App) transitionBatch(w http.ResponseWriter, r *http.Request, confirmed bool) {
	var input struct {
		Version int `json:"version"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1024)).Decode(&input); err != nil || input.Version < 1 {
		http.Error(w, "version is required", http.StatusBadRequest)
		return
	}
	user := r.Context().Value(sessionContextKey{}).(auth.SessionUser)
	batches := journal.NewBatches(a.db)
	var err error
	if confirmed {
		err = batches.Confirm(r.Context(), user.ID, r.PathValue("id"), input.Version)
	} else {
		err = batches.Reject(r.Context(), user.ID, r.PathValue("id"), input.Version)
	}
	if err != nil {
		http.Error(w, "batch not found or stale", http.StatusConflict)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *App) events(w http.ResponseWriter, r *http.Request) {
	user := r.Context().Value(sessionContextKey{}).(auth.SessionUser)
	rows, err := a.db.Query(r.Context(), `SELECT id::text,kind,occurred_at,attributes,revision,status FROM health_events WHERE user_id=$1 AND deleted_at IS NULL ORDER BY occurred_at DESC LIMIT 200`, user.ID)
	if err != nil {
		http.Error(w, "unable to read events", http.StatusInternalServerError)
		return
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var id, kind, status string
		var occurred time.Time
		var attrs json.RawMessage
		var revision int
		if err := rows.Scan(&id, &kind, &occurred, &attrs, &revision, &status); err != nil {
			http.Error(w, "unable to read events", 500)
			return
		}
		items = append(items, map[string]any{"id": id, "kind": kind, "occurred_at": occurred, "attributes": attrs, "revision": revision, "status": status})
	}
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(map[string]any{"events": items})
}

func (a *App) pendingBatches(w http.ResponseWriter, r *http.Request) {
	user := r.Context().Value(sessionContextKey{}).(auth.SessionUser)
	rows, err := a.db.Query(r.Context(), `SELECT b.id::text,b.version,b.created_at,e.id::text,e.kind,e.occurred_at,e.attributes,e.revision
        FROM event_batches b JOIN health_events e ON e.batch_id=b.id
        WHERE b.user_id=$1 AND b.status='pending' AND e.deleted_at IS NULL
        ORDER BY b.created_at DESC,e.occurred_at ASC`, user.ID)
	if err != nil {
		http.Error(w, "unable to read pending batches", 500)
		return
	}
	defer rows.Close()
	type batch struct {
		ID        string           `json:"id"`
		Version   int              `json:"version"`
		CreatedAt time.Time        `json:"created_at"`
		Events    []map[string]any `json:"events"`
	}
	byID := map[string]*batch{}
	ordered := []*batch{}
	for rows.Next() {
		var id, eventID, kind string
		var version, revision int
		var created, occurred time.Time
		var attrs json.RawMessage
		if err := rows.Scan(&id, &version, &created, &eventID, &kind, &occurred, &attrs, &revision); err != nil {
			http.Error(w, "unable to read pending batches", 500)
			return
		}
		b := byID[id]
		if b == nil {
			b = &batch{ID: id, Version: version, CreatedAt: created}
			byID[id] = b
			ordered = append(ordered, b)
		}
		b.Events = append(b.Events, map[string]any{"id": eventID, "kind": kind, "occurred_at": occurred, "attributes": attrs, "revision": revision})
	}
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(map[string]any{"batches": ordered})
}

func (a *App) exportEvents(w http.ResponseWriter, r *http.Request) {
	user := r.Context().Value(sessionContextKey{}).(auth.SessionUser)
	rows, err := a.db.Query(r.Context(), `SELECT id::text,kind,occurred_at,time_precision,attributes,revision FROM health_events WHERE user_id=$1 AND status='confirmed' AND deleted_at IS NULL ORDER BY occurred_at`, user.ID)
	if err != nil {
		http.Error(w, "unable to export events", 500)
		return
	}
	defer rows.Close()
	type exportedEvent struct {
		ID            string          `json:"id"`
		Kind          string          `json:"kind"`
		OccurredAt    time.Time       `json:"occurred_at"`
		TimePrecision string          `json:"time_precision"`
		Attributes    json.RawMessage `json:"attributes"`
		Revision      int             `json:"revision"`
	}
	items := []exportedEvent{}
	for rows.Next() {
		var item exportedEvent
		if err := rows.Scan(&item.ID, &item.Kind, &item.OccurredAt, &item.TimePrecision, &item.Attributes, &item.Revision); err != nil {
			http.Error(w, "unable to export events", 500)
			return
		}
		items = append(items, item)
	}
	w.Header().Set("Cache-Control", "no-store")
	if strings.EqualFold(r.URL.Query().Get("format"), "csv") {
		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		w.Header().Set("Content-Disposition", `attachment; filename="health-diary-events.csv"`)
		writer := csv.NewWriter(w)
		_ = writer.Write([]string{"id", "kind", "occurred_at", "time_precision", "attributes", "revision"})
		for _, item := range items {
			_ = writer.Write([]string{item.ID, item.Kind, item.OccurredAt.UTC().Format(time.RFC3339), item.TimePrecision, string(item.Attributes), strconv.Itoa(item.Revision)})
		}
		writer.Flush()
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="health-diary-events.json"`)
	_ = json.NewEncoder(w).Encode(map[string]any{"schema_version": "health-diary-export-v1", "generated_at": time.Now().UTC(), "timezone": user.Timezone, "events": items})
}

func (a *App) deleteEvent(w http.ResponseWriter, r *http.Request)  { a.mutateDeletion(w, r, true) }
func (a *App) restoreEvent(w http.ResponseWriter, r *http.Request) { a.mutateDeletion(w, r, false) }
func (a *App) mutateDeletion(w http.ResponseWriter, r *http.Request, deleting bool) {
	user := r.Context().Value(sessionContextKey{}).(auth.SessionUser)
	revision, err := strconv.Atoi(r.URL.Query().Get("revision"))
	if err != nil || revision < 1 {
		http.Error(w, "revision is required", http.StatusBadRequest)
		return
	}
	query := `UPDATE health_events SET deleted_at=now(),status='deleted',revision=revision+1,updated_at=now() WHERE id=$1 AND user_id=$2 AND revision=$3 AND deleted_at IS NULL`
	if !deleting {
		query = `UPDATE health_events SET deleted_at=NULL,status='confirmed',revision=revision+1,updated_at=now() WHERE id=$1 AND user_id=$2 AND revision=$3 AND status='deleted'`
	}
	tag, err := a.db.Exec(r.Context(), query, r.PathValue("id"), user.ID, revision)
	if err != nil {
		http.Error(w, "unable to update event", 500)
		return
	}
	if tag.RowsAffected() != 1 {
		http.Error(w, "event not found or stale", http.StatusConflict)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *App) calendar(w http.ResponseWriter, r *http.Request) {
	month, err := time.Parse("2006-01", r.URL.Query().Get("month"))
	if err != nil {
		http.Error(w, "month must be YYYY-MM", http.StatusBadRequest)
		return
	}
	user := r.Context().Value(sessionContextKey{}).(auth.SessionUser)
	events, err := analytics.New(a.db).Events(r.Context(), user.ID, month.UTC(), month.AddDate(0, 1, 0).UTC())
	if err != nil {
		http.Error(w, "unable to read calendar", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(map[string]any{"month": month.Format("2006-01"), "timezone": user.Timezone, "events": events})
}

type sessionContextKey struct{}

func (a *App) requireSession(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if a.auth == nil {
			http.Error(w, "database is unavailable", http.StatusServiceUnavailable)
			return
		}
		cookie, err := r.Cookie(a.config.SessionCookieName)
		if err != nil {
			http.Error(w, "authentication required", http.StatusUnauthorized)
			return
		}
		user, err := a.auth.SessionUser(r.Context(), cookie.Value)
		if err != nil {
			http.Error(w, "authentication required", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), sessionContextKey{}, user)))
	})
}

func (a *App) me(w http.ResponseWriter, r *http.Request) {
	user := r.Context().Value(sessionContextKey{}).(auth.SessionUser)
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(map[string]string{"id": user.ID, "timezone": user.Timezone})
}

func (a *App) createChallenge(w http.ResponseWriter, r *http.Request) {
	if a.auth == nil {
		http.Error(w, "database is unavailable", http.StatusServiceUnavailable)
		return
	}
	challenge, err := a.auth.CreateChallenge(r.Context())
	if err != nil {
		http.Error(w, "unable to create challenge", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	response := map[string]any{"challenge_id": challenge.ID, "token": challenge.Token, "expires_at": challenge.ExpiresAt}
	if a.config.Telegram.Username != "" {
		response["telegram_url"] = "https://t.me/" + a.config.Telegram.Username + "?start=login_" + challenge.Token
	}
	_ = json.NewEncoder(w).Encode(response)
}

func (a *App) verifyChallenge(w http.ResponseWriter, r *http.Request) {
	if a.auth == nil {
		http.Error(w, "database is unavailable", http.StatusServiceUnavailable)
		return
	}
	var input struct {
		Code string `json:"code"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1024)).Decode(&input); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	_, token, err := a.auth.Verify(r.Context(), r.PathValue("id"), input.Code)
	if err != nil {
		http.Error(w, "invalid or expired code", http.StatusUnauthorized)
		return
	}
	http.SetCookie(w, &http.Cookie{Name: a.config.SessionCookieName, Value: token, Path: "/", HttpOnly: true, Secure: true, SameSite: http.SameSiteLaxMode, MaxAge: int(a.config.SessionTTL.Seconds())})
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusNoContent)
}

func (a *App) ready(w http.ResponseWriter, r *http.Request) {
	if a.config.DatabaseURL == "" {
		http.Error(w, "database is not configured", http.StatusServiceUnavailable)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	conn, err := pgx.Connect(ctx, a.config.DatabaseURL)
	if err == nil {
		err = conn.Ping(ctx)
		conn.Close(ctx)
	}
	if err != nil {
		http.Error(w, "database is not ready", http.StatusServiceUnavailable)
		return
	}
	writeText(w, http.StatusOK, "ready\n")
}

func writeText(w http.ResponseWriter, status int, body string) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(status)
	_, _ = fmt.Fprint(w, body)
}
