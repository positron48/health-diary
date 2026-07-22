package episode

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"health-diary/internal/crypto"

	"github.com/jackc/pgx/v5/pgxpool"
)

// SyncConfirmedProjects confirmed pain and medication events into typed tables
// and symptom episode projections. It is idempotent and safe to call after
// Telegram confirm, web confirm, PATCH, delete or restore.
func SyncConfirmed(ctx context.Context, db *pgxpool.Pool, cipher *crypto.Cipher, userID string) error {
	if err := syncPainObservations(ctx, db, userID); err != nil {
		return err
	}
	if err := syncMedicationIntakes(ctx, db, cipher, userID); err != nil {
		return err
	}
	_, err := db.Exec(ctx, `UPDATE symptom_episodes s SET max_intensity=x.max_intensity,updated_at=now()
		FROM (SELECT p.episode_id,max(p.intensity) max_intensity FROM pain_observations p JOIN health_events e ON e.id=p.event_id
		WHERE e.user_id=$1 AND e.status='confirmed' AND e.deleted_at IS NULL GROUP BY p.episode_id) x WHERE s.id=x.episode_id AND s.user_id=$1`, userID)
	return err
}

func syncPainObservations(ctx context.Context, db *pgxpool.Pool, userID string) error {
	rows, err := db.Query(ctx, `SELECT e.id::text,e.occurred_at,e.time_precision,e.attributes
		FROM health_events e LEFT JOIN pain_observations p ON p.event_id=e.id
		WHERE e.user_id=$1 AND e.kind='pain_observation' AND e.status='confirmed' AND e.deleted_at IS NULL AND p.event_id IS NULL
		ORDER BY e.occurred_at,e.id`, userID)
	if err != nil {
		return err
	}
	defer rows.Close()
	type pain struct {
		id, precision string
		at            time.Time
		data          map[string]any
	}
	var pending []pain
	for rows.Next() {
		var item pain
		var raw json.RawMessage
		if err := rows.Scan(&item.id, &item.at, &item.precision, &raw); err != nil {
			return err
		}
		_ = json.Unmarshal(raw, &item.data)
		pending = append(pending, item)
	}
	for _, item := range pending {
		phase, _ := item.data["phase"].(string)
		if phase == "" {
			phase = "update"
		}
		var episodeID string
		if phase != "start" {
			_ = db.QueryRow(ctx, `SELECT id::text FROM symptom_episodes WHERE user_id=$1 AND status='open' AND started_at <= $2 ORDER BY started_at DESC LIMIT 1`, userID, item.at).Scan(&episodeID)
		}
		if episodeID == "" {
			if err := db.QueryRow(ctx, `INSERT INTO symptom_episodes(user_id,started_at,start_precision,created_from_event_id) VALUES($1,$2,$3,$4) RETURNING id::text`, userID, item.at, item.precision, item.id).Scan(&episodeID); err != nil {
				return err
			}
			if phase != "end" {
				phase = "start"
			}
		}
		var intensity any
		if value, ok := asFloat(item.data["intensity"]); ok {
			intensity = int(value)
		}
		locations := stringArray(item.data["locations"])
		qualities := stringArray(item.data["qualities"])
		symptoms := stringArray(item.data["associated_symptoms"])
		var laterality any
		if value, ok := item.data["laterality"].(string); ok && value != "" {
			laterality = value
		}
		var impact any
		if value, ok := asFloat(item.data["functional_impact"]); ok {
			impact = int(value)
		}
		if _, err := db.Exec(ctx, `INSERT INTO pain_observations(event_id,episode_id,phase,intensity,locations,laterality,qualities,associated_symptoms,functional_impact)
			VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9) ON CONFLICT(event_id) DO NOTHING`,
			item.id, episodeID, phase, intensity, locations, laterality, qualities, symptoms, impact); err != nil {
			return err
		}
		if phase == "end" {
			if _, err = db.Exec(ctx, `UPDATE symptom_episodes SET ended_at=$2,end_precision=$3,status='closed',updated_at=now() WHERE id=$1 AND user_id=$4`, episodeID, item.at, item.precision, userID); err != nil {
				return err
			}
		}
	}
	return nil
}

func syncMedicationIntakes(ctx context.Context, db *pgxpool.Pool, cipher *crypto.Cipher, userID string) error {
	if cipher == nil {
		return nil
	}
	rows, err := db.Query(ctx, `SELECT e.id::text,e.occurred_at,e.attributes
		FROM health_events e LEFT JOIN medication_intakes m ON m.event_id=e.id
		WHERE e.user_id=$1 AND e.kind='medication_intake' AND e.status='confirmed' AND e.deleted_at IS NULL AND m.event_id IS NULL
		ORDER BY e.occurred_at,e.id`, userID)
	if err != nil {
		return err
	}
	defer rows.Close()
	type intake struct {
		id   string
		at   time.Time
		data map[string]any
	}
	var pending []intake
	for rows.Next() {
		var item intake
		var raw json.RawMessage
		if err := rows.Scan(&item.id, &item.at, &raw); err != nil {
			return err
		}
		_ = json.Unmarshal(raw, &item.data)
		pending = append(pending, item)
	}
	for _, item := range pending {
		name := firstNonEmpty(stringValue(item.data["name_raw"]), stringValue(item.data["normalized_name"]), stringValue(item.data["name"]), "Не указано")
		sealed, err := cipher.Encrypt([]byte(name), []byte(userID))
		if err != nil {
			return fmt.Errorf("encrypt medication name: %w", err)
		}
		var episodeID any
		var openID string
		_ = db.QueryRow(ctx, `SELECT id::text FROM symptom_episodes WHERE user_id=$1 AND status='open' AND started_at <= $2 ORDER BY started_at DESC LIMIT 1`, userID, item.at).Scan(&openID)
		if openID != "" {
			episodeID = openID
		}
		var dose any
		if value, ok := asFloat(item.data["dose_value"]); ok {
			dose = value
		}
		var effect any
		if value, ok := asFloat(item.data["effect_rating"]); ok {
			effect = int(value)
		}
		normalized := firstNonEmpty(stringValue(item.data["normalized_name"]), stringValue(item.data["medication_name_normalized"]))
		var normalizedAny any
		if normalized != "" {
			normalizedAny = normalized
		}
		if _, err := db.Exec(ctx, `INSERT INTO medication_intakes(event_id,episode_id,medication_name_ciphertext,medication_name_normalized,dose_value,dose_unit,route,reason,effect_rating)
			VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9) ON CONFLICT(event_id) DO NOTHING`,
			item.id, episodeID, sealed, normalizedAny, dose,
			nullString(item.data["dose_unit"]), nullString(item.data["route"]), nullString(item.data["reason"]), effect); err != nil {
			return err
		}
	}
	return nil
}

func stringArray(value any) []string {
	switch typed := value.(type) {
	case []string:
		return typed
	case []any:
		out := make([]string, 0, len(typed))
		for _, item := range typed {
			if text, ok := item.(string); ok && text != "" {
				out = append(out, text)
			}
		}
		return out
	default:
		return []string{}
	}
}

func asFloat(value any) (float64, bool) {
	switch typed := value.(type) {
	case float64:
		return typed, true
	case int:
		return float64(typed), true
	case int64:
		return float64(typed), true
	case json.Number:
		number, err := typed.Float64()
		return number, err == nil
	default:
		return 0, false
	}
}

func stringValue(value any) string {
	text, _ := value.(string)
	return strings.TrimSpace(text)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func nullString(value any) any {
	text := stringValue(value)
	if text == "" {
		return nil
	}
	return text
}
