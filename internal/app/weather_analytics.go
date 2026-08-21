package app

import (
	"net/http"
	"time"

	"health-diary/internal/analytics"
)

func (a *App) weatherAnalyticsV1(w http.ResponseWriter, r *http.Request) {
	user := sessionUser(r)
	from, to, fields := parseRange(r.URL.Query().Get("from"), r.URL.Query().Get("to"), user.Timezone, user.DayStart)
	if len(fields) > 0 || from == nil || to == nil {
		writeAPIError(w, r, http.StatusUnprocessableEntity, "validation_failed", "Проверьте период", map[string]string{"from": "required YYYY-MM-DD", "to": "required YYYY-MM-DD"})
		return
	}
	events, err := analytics.New(a.db).Events(r.Context(), user.ID, *from, *to)
	if err != nil {
		writeAPIError(w, r, http.StatusInternalServerError, "internal_error", "Не удалось рассчитать погоду", nil)
		return
	}
	dailyRows, err := a.db.Query(r.Context(), `SELECT local_date,temp_mean_c::float8,pressure_mean_hpa::float8,pressure_delta_24h_hpa::float8,humidity_mean_pct::float8,precipitation_mm::float8,is_complete
		FROM daily_weather d
		WHERE d.user_id=$1 AND d.local_date >= $2::date AND d.local_date < $3::date
		AND d.place_id=COALESCE(
			(SELECT c.place_id FROM context_periods c
			 WHERE c.user_id=$1 AND c.status IN ('open','closed') AND c.place_id IS NOT NULL
			 AND c.started_on <= d.local_date AND (c.ended_on IS NULL OR c.ended_on >= d.local_date)
			 ORDER BY c.started_on DESC,c.created_at DESC LIMIT 1),
			NULLIF((SELECT settings->>'home_place_id' FROM users WHERE id=$1),'')::uuid,
			'00000000-0000-4000-8000-000000000001'::uuid)
		ORDER BY d.local_date`, user.ID, from.Format("2006-01-02"), to.Format("2006-01-02"))
	if err != nil {
		writeAPIError(w, r, http.StatusInternalServerError, "internal_error", "Не удалось загрузить погоду", nil)
		return
	}
	defer dailyRows.Close()
	days := make([]analytics.WeatherDay, 0)
	for dailyRows.Next() {
		var localDate time.Time
		var day analytics.WeatherDay
		if err := dailyRows.Scan(&localDate, &day.TempMeanC, &day.PressureMeanHPa, &day.PressureDelta24HPa, &day.HumidityMeanPct, &day.PrecipitationMM, &day.IsComplete); err != nil {
			writeAPIError(w, r, http.StatusInternalServerError, "internal_error", "Не удалось прочитать погоду", nil)
			return
		}
		day.Date = localDate.Format("2006-01-02")
		days = append(days, day)
	}
	if err := dailyRows.Err(); err != nil {
		writeAPIError(w, r, http.StatusInternalServerError, "internal_error", "Не удалось прочитать погоду", nil)
		return
	}
	hourlyRows, err := a.db.Query(r.Context(), `SELECT h.place_id::text,h.observed_at,h.pressure_msl_hpa::float8
		FROM hourly_weather h
		WHERE h.user_id=$1 AND h.is_complete AND h.observed_at >= $2 AND h.observed_at < $3
		AND h.place_id=COALESCE(
			(SELECT c.place_id FROM context_periods c
			 WHERE c.user_id=$1 AND c.status IN ('open','closed') AND c.place_id IS NOT NULL
			 AND c.started_on <= h.local_date AND (c.ended_on IS NULL OR c.ended_on >= h.local_date)
			 ORDER BY c.started_on DESC,c.created_at DESC LIMIT 1),
			NULLIF((SELECT settings->>'home_place_id' FROM users WHERE id=$1),'')::uuid,
			'00000000-0000-4000-8000-000000000001'::uuid)
		ORDER BY h.observed_at`, user.ID, from.Add(-24*time.Hour), to)
	if err != nil {
		writeAPIError(w, r, http.StatusInternalServerError, "internal_error", "Не удалось загрузить почасовую погоду", nil)
		return
	}
	defer hourlyRows.Close()
	hours := make([]analytics.WeatherHour, 0)
	for hourlyRows.Next() {
		var hour analytics.WeatherHour
		if err := hourlyRows.Scan(&hour.PlaceID, &hour.ObservedAt, &hour.PressureMSLHPa); err != nil {
			writeAPIError(w, r, http.StatusInternalServerError, "internal_error", "Не удалось прочитать почасовую погоду", nil)
			return
		}
		hours = append(hours, hour)
	}
	if err := hourlyRows.Err(); err != nil {
		writeAPIError(w, r, http.StatusInternalServerError, "internal_error", "Не удалось прочитать почасовую погоду", nil)
		return
	}
	result := analytics.BuildWeatherAnalytics(events, days, hours, user.Timezone, user.DayStart)
	if !a.config.WeatherAssociations {
		result.Status = "descriptive"
		for index := range result.Factors {
			result.Factors[index].Status = "disabled"
			result.Factors[index].RiskRatio = nil
			result.Factors[index].Description = "Расчёт возможных связей временно отключён; показаны только исходные наблюдения"
		}
	}
	writeJSON(w, http.StatusOK, result)
}
