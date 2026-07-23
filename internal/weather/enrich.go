package weather

import (
	"context"
	"encoding/json"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Enricher struct {
	db     *pgxpool.Pool
	client *Client
}

func NewEnricher(db *pgxpool.Pool, client *Client) *Enricher {
	return &Enricher{db: db, client: client}
}

// EnqueueRange schedules an idempotent enrich_weather job for a place/date range.
func EnqueueRange(ctx context.Context, db *pgxpool.Pool, userID, placeID, from, to string, maxAttempts int) error {
	if maxAttempts < 1 {
		maxAttempts = 3
	}
	payload, err := json.Marshal(map[string]string{
		"user_id":  userID,
		"place_id": placeID,
		"from":     from,
		"to":       to,
	})
	if err != nil {
		return err
	}
	_, err = db.Exec(ctx, `INSERT INTO jobs(kind,payload,max_attempts)
		SELECT 'enrich_weather',$1::jsonb,$2
		WHERE NOT EXISTS (
			SELECT 1 FROM jobs
			WHERE kind='enrich_weather' AND status IN ('queued','running')
			AND payload->>'user_id'=$3 AND payload->>'place_id'=$4
			AND payload->>'from'=$5 AND payload->>'to'=$6
		)`, payload, maxAttempts, userID, placeID, from, to)
	return err
}

func (e *Enricher) Enrich(ctx context.Context, userID, placeID, from, to string) error {
	var lat, lon float64
	var timezone string
	if err := e.db.QueryRow(ctx, `SELECT latitude::float8,longitude::float8,timezone FROM places WHERE id=$1`, placeID).Scan(&lat, &lon, &timezone); err != nil {
		return err
	}
	start, err := time.Parse("2006-01-02", from)
	if err != nil {
		return err
	}
	end, err := time.Parse("2006-01-02", to)
	if err != nil {
		return err
	}
	obs, err := e.client.FetchDaily(ctx, lat, lon, start, end, timezone)
	if err != nil {
		return err
	}
	var prevPressure *float64
	_ = e.db.QueryRow(ctx, `SELECT pressure_mean_hpa::float8 FROM daily_weather
		WHERE place_id=$1 AND provider=$2 AND local_date < $3::date AND pressure_mean_hpa IS NOT NULL
		ORDER BY local_date DESC LIMIT 1`, placeID, e.client.Provider(), from).Scan(&prevPressure)
	for _, day := range obs {
		var delta any
		if day.PressureMeanHPa != nil && prevPressure != nil {
			value := *day.PressureMeanHPa - *prevPressure
			delta = value
		}
		if day.PressureMeanHPa != nil {
			copy := *day.PressureMeanHPa
			prevPressure = &copy
		}
		_, err := e.db.Exec(ctx, `INSERT INTO daily_weather(
			user_id,place_id,local_date,provider,temp_min_c,temp_max_c,temp_mean_c,
			pressure_mean_hpa,pressure_delta_24h_hpa,humidity_mean_pct,precipitation_mm,weather_code,is_complete,fetched_at)
			VALUES($1,$2,$3::date,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,now())
			ON CONFLICT (place_id,local_date,provider) DO UPDATE SET
				temp_min_c=EXCLUDED.temp_min_c,
				temp_max_c=EXCLUDED.temp_max_c,
				temp_mean_c=EXCLUDED.temp_mean_c,
				pressure_mean_hpa=EXCLUDED.pressure_mean_hpa,
				pressure_delta_24h_hpa=EXCLUDED.pressure_delta_24h_hpa,
				humidity_mean_pct=EXCLUDED.humidity_mean_pct,
				precipitation_mm=EXCLUDED.precipitation_mm,
				weather_code=EXCLUDED.weather_code,
				is_complete=EXCLUDED.is_complete,
				fetched_at=now(),
				user_id=EXCLUDED.user_id`,
			userID, placeID, day.LocalDate.Format("2006-01-02"), e.client.Provider(),
			day.TempMinC, day.TempMaxC, day.TempMeanC, day.PressureMeanHPa, delta,
			day.HumidityMeanPct, day.PrecipitationMM, day.WeatherCode, day.IsComplete)
		if err != nil {
			return err
		}
	}
	return nil
}
