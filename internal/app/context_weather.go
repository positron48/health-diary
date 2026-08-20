package app

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"health-diary/internal/auth"
	"health-diary/internal/contextperiod"
	"health-diary/internal/userday"
	"health-diary/internal/weather"

	"github.com/jackc/pgx/v5"
)

func (a *App) syncContextAndWeather(ctx context.Context, user auth.SessionUser) error {
	if !a.config.ContextEnabled {
		return nil
	}
	if err := contextperiod.SyncConfirmed(ctx, a.db, user.ID, user.Timezone, user.DayStart); err != nil {
		return err
	}
	if !a.config.WeatherEnabled {
		return nil
	}
	return a.enqueueWeatherForUser(ctx, user.ID)
}

func (a *App) enqueueWeatherForUser(ctx context.Context, userID string) error {
	rows, err := a.db.Query(ctx, `SELECT COALESCE(place_id::text,''),started_on,COALESCE(ended_on, CURRENT_DATE)
		FROM context_periods WHERE user_id=$1 AND status IN ('open','closed') AND place_id IS NOT NULL`, userID)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var placeID string
		var started, ended time.Time
		if rows.Scan(&placeID, &started, &ended) != nil || placeID == "" {
			continue
		}
		_ = weather.EnqueueRange(ctx, a.db, userID, placeID, started.Format("2006-01-02"), ended.Format("2006-01-02"), a.config.JobMaxAttempts)
	}
	var homePlaceID string
	_ = a.db.QueryRow(ctx, `SELECT COALESCE(settings->>'home_place_id','') FROM users WHERE id=$1`, userID).Scan(&homePlaceID)
	if homePlaceID == "" {
		_ = a.db.QueryRow(ctx, `SELECT id::text FROM places WHERE provider_place_id='lipetsk' LIMIT 1`).Scan(&homePlaceID)
	}
	if homePlaceID != "" {
		to := time.Now().AddDate(0, 0, -1).Format("2006-01-02")
		from := time.Now().AddDate(0, 0, -30).Format("2006-01-02")
		_ = weather.EnqueueRange(ctx, a.db, userID, homePlaceID, from, to, a.config.JobMaxAttempts)
	}
	return nil
}

func (a *App) searchPlaces(w http.ResponseWriter, r *http.Request) {
	if a.weather == nil {
		writeAPIError(w, r, http.StatusServiceUnavailable, "weather_disabled", "Поиск городов временно недоступен", nil)
		return
	}
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if len([]rune(q)) < 2 {
		writeAPIError(w, r, 422, "validation_failed", "Проверьте запрос", map[string]string{"q": "must contain at least 2 characters"})
		return
	}
	results, err := a.weather.SearchPlaces(r.Context(), q, 8)
	if err != nil {
		writeAPIError(w, r, 502, "geocoding_failed", "Не удалось найти город", nil)
		return
	}
	writeJSON(w, 200, map[string]any{"places": results, "attribution": "Weather data by Open-Meteo.com (CC BY 4.0)"})
}

func (a *App) createPlace(w http.ResponseWriter, r *http.Request) {
	user := sessionUser(r)
	var input struct {
		ProviderPlaceID string  `json:"provider_place_id"`
		Label           string  `json:"label"`
		Region          string  `json:"region"`
		CountryCode     string  `json:"country_code"`
		Timezone        string  `json:"timezone"`
		Latitude        float64 `json:"latitude"`
		Longitude       float64 `json:"longitude"`
	}
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 8<<10)).Decode(&input) != nil {
		writeAPIError(w, r, 400, "malformed_request", "Некорректный JSON", nil)
		return
	}
	if strings.TrimSpace(input.ProviderPlaceID) == "" || strings.TrimSpace(input.Label) == "" {
		writeAPIError(w, r, 422, "validation_failed", "Проверьте город", map[string]string{"label": "required", "provider_place_id": "required"})
		return
	}
	if input.CountryCode == "" {
		input.CountryCode = "RU"
	}
	if input.Timezone == "" {
		input.Timezone = user.Timezone
	}
	var id string
	err := a.db.QueryRow(r.Context(), `INSERT INTO places(user_id,label,region,country_code,timezone,provider,provider_place_id,latitude,longitude)
		VALUES($1,$2,NULLIF($3,''),$4,$5,'open-meteo',$6,$7,$8)
		ON CONFLICT (provider,provider_place_id) DO UPDATE SET label=EXCLUDED.label,region=EXCLUDED.region,timezone=EXCLUDED.timezone
		RETURNING id::text`, user.ID, input.Label, input.Region, input.CountryCode, input.Timezone, input.ProviderPlaceID, input.Latitude, input.Longitude).Scan(&id)
	if err != nil {
		writeAPIError(w, r, 500, "internal_error", "Не удалось сохранить город", nil)
		return
	}
	writeJSON(w, 201, map[string]any{"id": id, "label": input.Label})
}

func (a *App) listContextPeriods(w http.ResponseWriter, r *http.Request) {
	if !a.config.ContextEnabled {
		writeAPIError(w, r, http.StatusServiceUnavailable, "context_disabled", "Местоположение временно недоступно", nil)
		return
	}
	user := sessionUser(r)
	rows, err := a.db.Query(r.Context(), `SELECT c.id::text,c.period_type,COALESCE(c.place_id::text,''),COALESCE(NULLIF(c.place_label,''),p.label,''),c.started_on,c.ended_on,c.status,c.revision
		FROM context_periods c LEFT JOIN places p ON p.id=c.place_id
		WHERE c.user_id=$1 AND c.status <> 'cancelled' ORDER BY c.started_on DESC,c.created_at DESC LIMIT 100`, user.ID)
	if err != nil {
		writeAPIError(w, r, 500, "internal_error", "Не удалось загрузить периоды", nil)
		return
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var id, periodType, placeID, placeLabel, status string
		var started time.Time
		var ended *time.Time
		var revision int
		if rows.Scan(&id, &periodType, &placeID, &placeLabel, &started, &ended, &status, &revision) != nil {
			continue
		}
		item := map[string]any{
			"id": id, "period_type": periodType, "place_label": placeLabel,
			"started_on": started.Format("2006-01-02"), "status": status, "revision": revision,
		}
		if placeID != "" {
			item["place_id"] = placeID
		}
		if ended != nil {
			item["ended_on"] = ended.Format("2006-01-02")
		}
		items = append(items, item)
	}
	localDate := userday.CurrentDate(time.Now(), userLocation(user.Timezone), userday.Start(user.DayStart))
	active, err := a.activeLocation(r.Context(), user, localDate)
	if err != nil {
		writeAPIError(w, r, 500, "internal_error", "Не удалось определить текущий город", nil)
		return
	}
	writeJSON(w, 200, map[string]any{"active": active, "periods": items})
}

func (a *App) createContextPeriod(w http.ResponseWriter, r *http.Request) {
	if !a.config.ContextEnabled {
		writeAPIError(w, r, http.StatusServiceUnavailable, "context_disabled", "Местоположение временно недоступно", nil)
		return
	}
	user := sessionUser(r)
	var input struct {
		PeriodType  string   `json:"period_type"`
		PlaceID     *string  `json:"place_id"`
		PlaceLabel  *string  `json:"place_label"`
		StartedOn   string   `json:"started_on"`
		EndedOn     *string  `json:"ended_on"`
		ProviderID  *string  `json:"provider_place_id"`
		Latitude    *float64 `json:"latitude"`
		Longitude   *float64 `json:"longitude"`
		Region      *string  `json:"region"`
		CountryCode *string  `json:"country_code"`
		Timezone    *string  `json:"timezone"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&input) != nil {
		writeAPIError(w, r, 400, "malformed_request", "Некорректный JSON", nil)
		return
	}
	preflight := contextperiod.ManualInput{
		PeriodType: input.PeriodType,
		PlaceID:    "selected-place",
		StartedOn:  input.StartedOn,
		EndedOn:    input.EndedOn,
	}
	if fields := contextperiod.ValidateManualInput(preflight); len(fields) > 0 {
		writeAPIError(w, r, 422, "validation_failed", "Проверьте период", fields)
		return
	}
	placeID := ""
	if input.PlaceID != nil {
		placeID = strings.TrimSpace(*input.PlaceID)
	}
	if placeID == "" && input.ProviderID != nil && input.Latitude != nil && input.Longitude != nil {
		label := ""
		if input.PlaceLabel != nil {
			label = strings.TrimSpace(*input.PlaceLabel)
		}
		region, country, tz := "", "RU", user.Timezone
		if input.Region != nil {
			region = *input.Region
		}
		if input.CountryCode != nil {
			country = *input.CountryCode
		}
		if input.Timezone != nil {
			tz = *input.Timezone
		}
		if label == "" || strings.TrimSpace(*input.ProviderID) == "" {
			writeAPIError(w, r, 422, "validation_failed", "Проверьте город", map[string]string{"place_id": "select a city"})
			return
		}
		if _, err := time.LoadLocation(tz); err != nil {
			writeAPIError(w, r, 422, "validation_failed", "Проверьте город", map[string]string{"timezone": "must be an IANA timezone"})
			return
		}
		if err := a.db.QueryRow(r.Context(), `INSERT INTO places(user_id,label,region,country_code,timezone,provider,provider_place_id,latitude,longitude)
			VALUES($1,$2,$3,$4,$5,'open-meteo',$6,$7,$8)
			ON CONFLICT (provider,provider_place_id) DO UPDATE SET label=EXCLUDED.label
			RETURNING id::text`, user.ID, label, nullIfEmptyPtr(region), country, tz, *input.ProviderID, *input.Latitude, *input.Longitude).Scan(&placeID); err != nil {
			writeAPIError(w, r, 500, "internal_error", "Не удалось сохранить город", nil)
			return
		}
	}
	manual := contextperiod.ManualInput{
		PeriodType: input.PeriodType,
		PlaceID:    placeID,
		StartedOn:  input.StartedOn,
		EndedOn:    input.EndedOn,
	}
	if fields := contextperiod.ValidateManualInput(manual); len(fields) > 0 {
		writeAPIError(w, r, 422, "validation_failed", "Проверьте период", fields)
		return
	}
	period, err := contextperiod.CreateManual(r.Context(), a.db, user.ID, manual)
	if err != nil {
		if errors.Is(err, contextperiod.ErrConflict) {
			writeAPIError(w, r, http.StatusConflict, "context_period_conflict", "На эти даты уже задан другой город", nil)
			return
		}
		if errors.Is(err, contextperiod.ErrPlaceNotFound) {
			writeAPIError(w, r, 422, "validation_failed", "Проверьте город", map[string]string{"place_id": "unknown place"})
			return
		}
		writeAPIError(w, r, 500, "internal_error", "Не удалось сохранить период", nil)
		return
	}
	if placeID != "" && a.config.WeatherEnabled {
		to := time.Now().In(userLocation(user.Timezone)).Format("2006-01-02")
		if input.EndedOn != nil {
			if *input.EndedOn < to {
				to = *input.EndedOn
			}
		}
		if input.StartedOn <= to {
			_ = weather.EnqueueRange(r.Context(), a.db, user.ID, placeID, input.StartedOn, to, a.config.JobMaxAttempts)
		}
	}
	writeJSON(w, 201, period)
}

func (a *App) returnHome(w http.ResponseWriter, r *http.Request) {
	if !a.config.ContextEnabled {
		writeAPIError(w, r, http.StatusServiceUnavailable, "context_disabled", "Местоположение временно недоступно", nil)
		return
	}
	var input struct {
		StartedOn string `json:"started_on"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<10))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&input) != nil {
		writeAPIError(w, r, 400, "malformed_request", "Некорректный JSON", nil)
		return
	}
	if _, err := time.Parse("2006-01-02", input.StartedOn); err != nil {
		writeAPIError(w, r, 422, "validation_failed", "Проверьте дату", map[string]string{"started_on": "must be YYYY-MM-DD"})
		return
	}
	user := sessionUser(r)
	homeID, homeLabel, err := a.homePlace(r.Context(), user.ID)
	if err != nil {
		writeAPIError(w, r, 500, "internal_error", "Не удалось определить домашний город", nil)
		return
	}
	if err := contextperiod.ReturnHome(r.Context(), a.db, user.ID, input.StartedOn); err != nil {
		writeAPIError(w, r, 500, "internal_error", "Не удалось вернуть домашний город", nil)
		return
	}
	if homeID != "" && a.config.WeatherEnabled {
		today := time.Now().In(userLocation(user.Timezone)).Format("2006-01-02")
		if input.StartedOn <= today {
			_ = weather.EnqueueRange(r.Context(), a.db, user.ID, homeID, input.StartedOn, today, a.config.JobMaxAttempts)
		}
	}
	writeJSON(w, 200, map[string]any{"active": map[string]any{
		"place_id": homeID, "place_label": homeLabel, "source": "home", "local_date": input.StartedOn,
	}})
}

func (a *App) activeLocation(ctx context.Context, user auth.SessionUser, localDate string) (map[string]any, error) {
	placeID, placeLabel, periodType, _, err := contextperiod.ActivePlaceForDate(ctx, a.db, user.ID, localDate)
	if err == nil {
		return map[string]any{
			"place_id": placeID, "place_label": placeLabel, "period_type": periodType,
			"source": "period", "local_date": localDate,
		}, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	homeID, homeLabel, err := a.homePlace(ctx, user.ID)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"place_id": homeID, "place_label": homeLabel, "source": "home", "local_date": localDate,
	}, nil
}

func (a *App) homePlace(ctx context.Context, userID string) (string, string, error) {
	const defaultHomeID = "00000000-0000-4000-8000-000000000001"
	placeID := ""
	if err := a.db.QueryRow(ctx, `SELECT COALESCE(settings->>'home_place_id','') FROM users WHERE id=$1`, userID).Scan(&placeID); err != nil {
		return "", "", err
	}
	if placeID == "" {
		placeID = defaultHomeID
	}
	var label string
	if err := a.db.QueryRow(ctx, `SELECT label FROM places WHERE id::text=$1 AND (user_id IS NULL OR user_id=$2)`, placeID, userID).Scan(&label); err != nil {
		return "", "", err
	}
	return placeID, label, nil
}

func (a *App) patchContextPeriod(w http.ResponseWriter, r *http.Request) {
	if !a.config.ContextEnabled {
		writeAPIError(w, r, http.StatusServiceUnavailable, "context_disabled", "Местоположение временно недоступно", nil)
		return
	}
	user := sessionUser(r)
	var input struct {
		Revision   int     `json:"revision"`
		EndedOn    *string `json:"ended_on"`
		Status     *string `json:"status"`
		PlaceLabel *string `json:"place_label"`
	}
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 8<<10)).Decode(&input) != nil || input.Revision < 1 {
		writeAPIError(w, r, 422, "validation_failed", "Требуется revision", map[string]string{"revision": "must be positive"})
		return
	}
	if input.Status != nil {
		allowed := map[string]bool{"open": true, "closed": true, "cancelled": true}
		if !allowed[*input.Status] {
			writeAPIError(w, r, 422, "validation_failed", "Проверьте статус", map[string]string{"status": "unsupported"})
			return
		}
	}
	if input.EndedOn != nil {
		if _, err := time.Parse("2006-01-02", *input.EndedOn); err != nil {
			writeAPIError(w, r, 422, "validation_failed", "Проверьте дату", map[string]string{"ended_on": "must be YYYY-MM-DD"})
			return
		}
	}
	tag, err := a.db.Exec(r.Context(), `UPDATE context_periods SET
		ended_on=COALESCE($3::date,ended_on),
		status=COALESCE($4,status),
		place_label=COALESCE($5,place_label),
		revision=revision+1,
		updated_at=now()
		WHERE id=$1 AND user_id=$2 AND revision=$6`,
		r.PathValue("id"), user.ID, input.EndedOn, input.Status, input.PlaceLabel, input.Revision)
	if err != nil {
		writeAPIError(w, r, 500, "internal_error", "Не удалось обновить период", nil)
		return
	}
	if tag.RowsAffected() == 0 {
		writeAPIError(w, r, 409, "revision_conflict", "Период изменился", nil)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func nullIfEmptyPtr(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return value
}
