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
	_ "time/tzdata"

	"health-diary/internal/auth"
	"health-diary/internal/bot"
	"health-diary/internal/config"
	"health-diary/internal/crypto"
	"health-diary/internal/database"
	"health-diary/internal/episode"
	"health-diary/internal/ingest"
	"health-diary/internal/jobs"
	"health-diary/internal/journal"
	"health-diary/internal/llm"
	"health-diary/internal/userday"
	"health-diary/internal/weather"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// webAssets contains the production Vue bundle. Docker replaces the small
// checked-in fallback page with the Vite build before compiling the server.
//
//go:embed web/dist/*
var webAssets embed.FS

type App struct {
	config          config.Config
	logger          *slog.Logger
	db              *pgxpool.Pool
	auth            *auth.Service
	cipher          *crypto.Cipher
	ingest          *ingest.Service
	weather         *weather.Client
	telegramWebhook http.Handler
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
	if a.config.DataEncryptionKey != "" {
		cipher, err := crypto.New(a.config.DataEncryptionKey, a.config.DataEncryptionKeyVersion)
		if err != nil {
			return err
		}
		a.cipher = cipher
	}
	if a.db != nil && a.cipher != nil {
		a.ingest = ingest.New(a.db, a.cipher, a.config.JobMaxAttempts)
		var extractor llm.Extractor = llm.Fake{}
		provider, model := "fake", "fake"
		if a.config.LLMAPIKey != "" {
			extractor = llm.NewOpenAICompatible(a.config.LLMBaseURL, a.config.LLMModel, a.config.LLMAPIKey, &http.Client{Timeout: 30 * time.Second})
			provider, model = "polza", a.config.LLMModel
		}
		var enricher *weather.Enricher
		if a.config.WeatherEnabled {
			a.weather = weather.NewClient(
				a.config.Weather.BaseURL,
				a.config.Weather.ForecastURL,
				a.config.Weather.GeocodingURL,
				a.config.Weather.Provider,
				a.config.Weather.Timeout,
				&http.Client{Timeout: a.config.Weather.Timeout},
			)
			enricher = weather.NewEnricher(a.db, a.weather)
		}
		worker := jobs.NewWorker(a.db, a.cipher, extractor, "app-1", provider, model, enricher)
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
	}
	if a.config.Telegram.Token != "" {
		if a.config.DataEncryptionKey == "" || len(a.config.Telegram.AllowedUserIDs) == 0 {
			return fmt.Errorf("telegram requires DATA_ENCRYPTION_KEY and TELEGRAM_ALLOWED_USER_IDS")
		}
		pool := a.db
		if pool == nil {
			return fmt.Errorf("telegram requires DATABASE_URL")
		}
		handler := bot.NewHandler(pool, a.ingest, a.auth, a.cipher, a.config.Telegram.AllowedUserIDs, a.logger)
		if a.config.Telegram.Mode == "webhook" {
			api, err := bot.ConfigureWebhook(a.config.Telegram.Token, a.config.Telegram.WebhookURL, a.config.Telegram.WebhookSecret, a.config.Telegram.SOCKS5ProxyAddr)
			if err != nil {
				return fmt.Errorf("configure telegram webhook: %w", err)
			}
			a.telegramWebhook = bot.WebhookHandler(api, handler, a.config.Telegram.WebhookSecret)
			go bot.RunOutbox(ctx, api, pool)
		} else {
			go func() {
				if err := bot.RunLongPolling(ctx, a.config.Telegram.Token, a.config.Telegram.SOCKS5ProxyAddr, handler, a.logger); err != nil {
					a.logger.Error("telegram polling stopped", "error", err)
				}
			}()
		}
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
	if a.telegramWebhook != nil {
		mux.Handle("POST /telegram/webhook", a.telegramWebhook)
	}

	// Public API lives only under /api/v1 so SPA routes like /calendar never collide.
	mux.HandleFunc("POST /api/v1/auth/challenges", a.createChallenge)
	mux.HandleFunc("POST /api/v1/auth/challenges/{id}/verify", a.verifyChallenge)
	mux.Handle("GET /api/v1/auth/session", a.requireSession(http.HandlerFunc(a.me)))
	mux.Handle("DELETE /api/v1/auth/session", a.requireSession(http.HandlerFunc(a.logout)))
	mux.Handle("DELETE /api/v1/auth/sessions", a.requireSession(http.HandlerFunc(a.revokeSessions)))
	mux.Handle("GET /api/v1/me", a.requireSession(http.HandlerFunc(a.me)))
	mux.Handle("PATCH /api/v1/me", a.requireSession(http.HandlerFunc(a.patchMe)))
	mux.Handle("GET /api/v1/calendar", a.requireSession(http.HandlerFunc(a.calendarV1)))
	mux.Handle("GET /api/v1/days/{date}", a.requireSession(http.HandlerFunc(a.dayTimeline)))
	mux.Handle("POST /api/v1/entries", a.requireSession(http.HandlerFunc(a.createEntry)))
	mux.Handle("GET /api/v1/analytics/summary", a.requireSession(http.HandlerFunc(a.analyticsSummaryV1)))
	mux.Handle("GET /api/v1/analytics/associations", a.requireSession(http.HandlerFunc(a.analyticsAssociations)))
	mux.Handle("GET /api/v1/analytics/medications", a.requireSession(http.HandlerFunc(a.analyticsMedications)))
	mux.Handle("GET /api/v1/events", a.requireSession(http.HandlerFunc(a.eventsV1)))
	mux.Handle("GET /api/v1/events/{id}", a.requireSession(http.HandlerFunc(a.eventDetail)))
	mux.Handle("PATCH /api/v1/events/{id}", a.requireSession(http.HandlerFunc(a.patchEvent)))
	mux.Handle("GET /api/v1/batches", a.requireSession(http.HandlerFunc(a.pendingBatchesV1)))
	mux.Handle("GET /api/v1/inbox", a.requireSession(http.HandlerFunc(a.inboxV1)))
	mux.Handle("GET /api/v1/entries/{id}", a.requireSession(http.HandlerFunc(a.sourceEntry)))
	mux.Handle("DELETE /api/v1/entries/{id}", a.requireSession(http.HandlerFunc(a.deleteEntry)))
	mux.Handle("GET /api/v1/exports", a.requireSession(http.HandlerFunc(a.exportEvents)))
	mux.Handle("DELETE /api/v1/events/{id}", a.requireSession(http.HandlerFunc(a.deleteEvent)))
	mux.Handle("POST /api/v1/events/{id}/restore", a.requireSession(http.HandlerFunc(a.restoreEvent)))
	mux.Handle("POST /api/v1/batches/{id}/confirm", a.requireSession(http.HandlerFunc(a.confirmBatch)))
	mux.Handle("POST /api/v1/batches/{id}/reject", a.requireSession(http.HandlerFunc(a.rejectBatch)))
	mux.Handle("GET /api/v1/places/search", a.requireSession(http.HandlerFunc(a.searchPlaces)))
	mux.Handle("POST /api/v1/places", a.requireSession(http.HandlerFunc(a.createPlace)))
	mux.Handle("GET /api/v1/context-periods", a.requireSession(http.HandlerFunc(a.listContextPeriods)))
	mux.Handle("POST /api/v1/context-periods", a.requireSession(http.HandlerFunc(a.createContextPeriod)))
	mux.Handle("PATCH /api/v1/context-periods/{id}", a.requireSession(http.HandlerFunc(a.patchContextPeriod)))
	mux.Handle("GET /api/v1/episodes", a.requireSession(http.HandlerFunc(a.episodes)))
	mux.Handle("GET /api/v1/episodes/{id}", a.requireSession(http.HandlerFunc(a.episodeDetail)))
	mux.Handle("POST /api/v1/episodes/{id}/close", a.requireSession(http.HandlerFunc(a.closeEpisode)))
	mux.Handle("POST /api/v1/episodes/{id}/reopen", a.requireSession(http.HandlerFunc(a.reopenEpisode)))
	mux.Handle("POST /api/v1/me/deletion-request", a.requireSession(http.HandlerFunc(a.deletionRequest)))

	web, err := fs.Sub(webAssets, "web/dist")
	if err != nil {
		panic(err)
	}
	mux.Handle("GET /", spaHandler(web))
	return securityHeaders(mux)
}

func spaHandler(web fs.FS) http.Handler {
	files := http.FileServer(http.FS(web))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/")
		if path == "" {
			path = "index.html"
		}
		if _, err := fs.Stat(web, path); err == nil {
			files.ServeHTTP(w, r)
			return
		}
		index, err := fs.ReadFile(web, "index.html")
		if err != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(index)
	})
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
		writeAPIError(w, r, http.StatusUnprocessableEntity, "validation_failed", "Требуется версия пакета", map[string]string{"version": "must be positive"})
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
		writeAPIError(w, r, http.StatusConflict, "batch_state_conflict", "Пакет уже обработан или изменился", nil)
		return
	}
	if confirmed {
		_ = episode.SyncConfirmed(r.Context(), a.db, a.cipher, user.ID)
		_ = a.syncContextAndWeather(r.Context(), user)
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *App) exportEvents(w http.ResponseWriter, r *http.Request) {
	user := r.Context().Value(sessionContextKey{}).(auth.SessionUser)
	rows, err := a.db.Query(r.Context(), `SELECT id::text,kind,occurred_at,ended_at,time_precision,attributes,revision FROM health_events WHERE user_id=$1 AND status='confirmed' AND deleted_at IS NULL ORDER BY occurred_at`, user.ID)
	if err != nil {
		writeAPIError(w, r, 500, "internal_error", "Не удалось подготовить экспорт", nil)
		return
	}
	defer rows.Close()
	type exportedEvent struct {
		ID            string          `json:"id"`
		Kind          string          `json:"kind"`
		OccurredAt    time.Time       `json:"occurred_at"`
		EndedAt       *time.Time      `json:"ended_at,omitempty"`
		TimePrecision string          `json:"time_precision"`
		Attributes    json.RawMessage `json:"attributes"`
		Revision      int             `json:"revision"`
	}
	items := []exportedEvent{}
	for rows.Next() {
		var item exportedEvent
		if err := rows.Scan(&item.ID, &item.Kind, &item.OccurredAt, &item.EndedAt, &item.TimePrecision, &item.Attributes, &item.Revision); err != nil {
			writeAPIError(w, r, 500, "internal_error", "Не удалось подготовить экспорт", nil)
			return
		}
		items = append(items, item)
	}
	periods := []map[string]any{}
	periodRows, _ := a.db.Query(r.Context(), `SELECT id::text,period_type,COALESCE(place_label,''),started_on,ended_on,status FROM context_periods WHERE user_id=$1 AND status<>'cancelled' ORDER BY started_on`, user.ID)
	if periodRows != nil {
		defer periodRows.Close()
		for periodRows.Next() {
			var id, periodType, placeLabel, status string
			var started time.Time
			var ended *time.Time
			if periodRows.Scan(&id, &periodType, &placeLabel, &started, &ended, &status) != nil {
				continue
			}
			item := map[string]any{"id": id, "period_type": periodType, "place_label": placeLabel, "started_on": started.Format("2006-01-02"), "status": status}
			if ended != nil {
				item["ended_on"] = ended.Format("2006-01-02")
			}
			periods = append(periods, item)
		}
	}
	weatherItems := []map[string]any{}
	weatherRows, _ := a.db.Query(r.Context(), `SELECT place_id::text,local_date,temp_mean_c,pressure_mean_hpa,pressure_delta_24h_hpa,humidity_mean_pct,precipitation_mm,weather_code,is_complete
		FROM daily_weather WHERE user_id=$1 ORDER BY local_date`, user.ID)
	if weatherRows != nil {
		defer weatherRows.Close()
		for weatherRows.Next() {
			var placeID string
			var localDate time.Time
			var temp, pressure, delta, humidity, precip *float64
			var code *int
			var complete bool
			if weatherRows.Scan(&placeID, &localDate, &temp, &pressure, &delta, &humidity, &precip, &code, &complete) != nil {
				continue
			}
			weatherItems = append(weatherItems, map[string]any{
				"place_id": placeID, "local_date": localDate.Format("2006-01-02"), "temp_mean_c": temp,
				"pressure_mean_hpa": pressure, "pressure_delta_24h_hpa": delta, "humidity_mean_pct": humidity,
				"precipitation_mm": precip, "weather_code": code, "is_complete": complete,
			})
		}
	}
	w.Header().Set("Cache-Control", "no-store")
	if strings.EqualFold(r.URL.Query().Get("format"), "csv") {
		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		w.Header().Set("Content-Disposition", `attachment; filename="health-diary-events.csv"`)
		writer := csv.NewWriter(w)
		_ = writer.Write([]string{"id", "kind", "occurred_at", "ended_at", "time_precision", "attributes", "revision"})
		for _, item := range items {
			ended := ""
			if item.EndedAt != nil {
				ended = item.EndedAt.UTC().Format(time.RFC3339)
			}
			_ = writer.Write([]string{item.ID, item.Kind, item.OccurredAt.UTC().Format(time.RFC3339), ended, item.TimePrecision, string(item.Attributes), strconv.Itoa(item.Revision)})
		}
		writer.Flush()
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="health-diary-events.json"`)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"schema_version":  "health-diary-export-v2",
		"generated_at":    time.Now().UTC(),
		"timezone":        user.Timezone,
		"events":          items,
		"context_periods": periods,
		"daily_weather":   weatherItems,
		"attribution":     "Weather data by Open-Meteo.com (CC BY 4.0)",
	})
}

func (a *App) deleteEvent(w http.ResponseWriter, r *http.Request)  { a.mutateDeletion(w, r, true) }
func (a *App) restoreEvent(w http.ResponseWriter, r *http.Request) { a.mutateDeletion(w, r, false) }
func (a *App) mutateDeletion(w http.ResponseWriter, r *http.Request, deleting bool) {
	user := r.Context().Value(sessionContextKey{}).(auth.SessionUser)
	revision, err := strconv.Atoi(r.URL.Query().Get("revision"))
	if err != nil || revision < 1 {
		writeAPIError(w, r, http.StatusUnprocessableEntity, "validation_failed", "Требуется актуальная revision", map[string]string{"revision": "must be positive"})
		return
	}
	query := `WITH before AS (SELECT id,status,revision,attributes FROM health_events WHERE id=$1 AND user_id=$2 AND revision=$3 AND deleted_at IS NULL FOR UPDATE), updated AS (UPDATE health_events e SET deleted_from_status=b.status,deleted_at=now(),status='deleted',revision=e.revision+1,updated_at=now() FROM before b WHERE e.id=b.id RETURNING e.id,e.revision,e.status,e.attributes,b.status AS old_status,b.revision AS old_revision,b.attributes AS old_attributes) INSERT INTO event_revisions(event_id,revision,changed_by,before_data,after_data,reason) SELECT id,revision,'web_user',jsonb_build_object('status',old_status,'revision',old_revision,'attributes',old_attributes),jsonb_build_object('status',status,'revision',revision,'attributes',attributes),'user deletion' FROM updated`
	if !deleting {
		query = `WITH before AS (SELECT id,status,revision,attributes,deleted_from_status FROM health_events WHERE id=$1 AND user_id=$2 AND revision=$3 AND status='deleted' FOR UPDATE), updated AS (UPDATE health_events e SET deleted_at=NULL,status=COALESCE(b.deleted_from_status,'confirmed'),deleted_from_status=NULL,revision=e.revision+1,updated_at=now() FROM before b WHERE e.id=b.id RETURNING e.id,e.revision,e.status,e.attributes,b.status AS old_status,b.revision AS old_revision,b.attributes AS old_attributes) INSERT INTO event_revisions(event_id,revision,changed_by,before_data,after_data,reason) SELECT id,revision,'web_user',jsonb_build_object('status',old_status,'revision',old_revision,'attributes',old_attributes),jsonb_build_object('status',status,'revision',revision,'attributes',attributes),'user restore' FROM updated`
	}
	tag, err := a.db.Exec(r.Context(), query, r.PathValue("id"), user.ID, revision)
	if err != nil {
		writeAPIError(w, r, 500, "internal_error", "Не удалось изменить событие", nil)
		return
	}
	if tag.RowsAffected() != 1 {
		writeAPIError(w, r, http.StatusConflict, "revision_conflict", "Событие уже изменилось или не найдено", nil)
		return
	}
	a.afterEventMutation(r)
	w.WriteHeader(http.StatusNoContent)
}

func userLocation(timezone string) *time.Location {
	if strings.TrimSpace(timezone) != "" {
		if loc, err := time.LoadLocation(timezone); err == nil {
			return loc
		}
	}
	// Existing imported/early accounts may have an empty or invalid timezone.
	// The product default is Moscow, and analytics must remain available.
	if loc, err := time.LoadLocation("Europe/Moscow"); err == nil {
		return loc
	}
	return time.UTC
}

type sessionContextKey struct{}

func (a *App) requireSession(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if a.auth == nil {
			writeAPIError(w, r, http.StatusServiceUnavailable, "dependency_unavailable", "Сервис временно недоступен", nil)
			return
		}
		cookie, err := r.Cookie(a.config.SessionCookieName)
		if err != nil {
			writeAPIError(w, r, http.StatusUnauthorized, "authentication_required", "Требуется вход", nil)
			return
		}
		user, err := a.auth.SessionUser(r.Context(), cookie.Value)
		if err != nil {
			writeAPIError(w, r, http.StatusUnauthorized, "authentication_required", "Требуется вход", nil)
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), sessionContextKey{}, user)))
	})
}

func (a *App) me(w http.ResponseWriter, r *http.Request) {
	user := r.Context().Value(sessionContextKey{}).(auth.SessionUser)
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	settings := json.RawMessage(user.Settings)
	if len(settings) == 0 {
		settings = json.RawMessage(`{}`)
	}
	_ = json.NewEncoder(w).Encode(map[string]any{
		"id": user.ID, "timezone": user.Timezone, "locale": user.Locale, "settings": settings,
		"current_local_date": userday.CurrentDate(time.Now(), userLocation(user.Timezone), userday.Start(user.DayStart)),
	})
}

func (a *App) logout(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(a.config.SessionCookieName)
	if err == nil {
		_ = a.auth.RevokeSession(r.Context(), cookie.Value)
	}
	http.SetCookie(w, &http.Cookie{Name: a.config.SessionCookieName, Value: "", Path: "/", HttpOnly: true, Secure: true, SameSite: http.SameSiteLaxMode, MaxAge: -1})
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusNoContent)
}

func (a *App) deletionRequest(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Confirm string `json:"confirm"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1024)).Decode(&input); err != nil || input.Confirm != "DELETE_MY_DATA" {
		writeAPIError(w, r, 422, "validation_failed", "Требуется явное подтверждение", map[string]string{"confirm": "invalid confirmation"})
		return
	}
	user := r.Context().Value(sessionContextKey{}).(auth.SessionUser)
	if time.Since(user.CreatedAt) > 10*time.Minute {
		writeAPIError(w, r, 401, "recent_authentication_required", "Требуется недавний вход", nil)
		return
	}
	tx, err := a.db.Begin(r.Context())
	if err != nil {
		writeAPIError(w, r, 500, "internal_error", "Не удалось запросить удаление", nil)
		return
	}
	defer tx.Rollback(r.Context())
	var auditID string
	if err = tx.QueryRow(r.Context(), `INSERT INTO deletion_audits(status) VALUES('queued') RETURNING id::text`).Scan(&auditID); err != nil {
		writeAPIError(w, r, 500, "internal_error", "Не удалось запросить удаление", nil)
		return
	}
	tag, err := tx.Exec(r.Context(), `UPDATE users SET status='deletion_pending',updated_at=now() WHERE id=$1 AND status='active'`, user.ID)
	if err != nil || tag.RowsAffected() != 1 {
		writeAPIError(w, r, 409, "deletion_already_pending", "Удаление уже запрошено", nil)
		return
	}
	if _, err = tx.Exec(r.Context(), `UPDATE web_sessions SET revoked_at=now() WHERE user_id=$1 AND revoked_at IS NULL`, user.ID); err != nil {
		writeAPIError(w, r, 500, "internal_error", "Не удалось запросить удаление", nil)
		return
	}
	if _, err = tx.Exec(r.Context(), `INSERT INTO jobs(kind,payload,max_attempts) VALUES('delete_user',jsonb_build_object('user_id',$1::text,'audit_id',$2::text),3)`, user.ID, auditID); err != nil {
		writeAPIError(w, r, 500, "internal_error", "Не удалось запросить удаление", nil)
		return
	}
	if err = tx.Commit(r.Context()); err != nil {
		writeAPIError(w, r, 500, "internal_error", "Не удалось запросить удаление", nil)
		return
	}
	http.SetCookie(w, &http.Cookie{Name: a.config.SessionCookieName, Value: "", Path: "/", HttpOnly: true, Secure: true, SameSite: http.SameSiteLaxMode, MaxAge: -1})
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(map[string]string{"deletion_request_id": auditID, "status": "queued"})
}

func (a *App) createChallenge(w http.ResponseWriter, r *http.Request) {
	if a.auth == nil {
		writeAPIError(w, r, 503, "dependency_unavailable", "Сервис временно недоступен", nil)
		return
	}
	challenge, err := a.auth.CreateChallenge(r.Context())
	if err != nil {
		writeAPIError(w, r, 500, "internal_error", "Не удалось создать вход", nil)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	response := map[string]any{"challenge_id": challenge.ID, "token": challenge.Token, "expires_at": challenge.ExpiresAt}
	if a.config.Telegram.Username != "" {
		response["telegram_deep_link"] = "https://t.me/" + a.config.Telegram.Username + "?start=login_" + challenge.Token
		response["telegram_url"] = response["telegram_deep_link"]
	}
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(response)
}

func (a *App) verifyChallenge(w http.ResponseWriter, r *http.Request) {
	if a.auth == nil {
		writeAPIError(w, r, 503, "dependency_unavailable", "Сервис временно недоступен", nil)
		return
	}
	var input struct {
		Code string `json:"code"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1024)).Decode(&input); err != nil {
		writeAPIError(w, r, 400, "malformed_request", "Некорректный JSON", nil)
		return
	}
	_, token, err := a.auth.Verify(r.Context(), r.PathValue("id"), input.Code)
	if err != nil {
		code := "invalid_code"
		message := "Неверный код"
		if errors.Is(err, auth.ErrChallengeExpired) {
			code, message = "challenge_expired", "Срок действия кода истёк"
		} else if errors.Is(err, auth.ErrChallengeLocked) {
			code, message = "challenge_locked", "Превышено число попыток"
		}
		writeAPIError(w, r, http.StatusUnauthorized, code, message, nil)
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
