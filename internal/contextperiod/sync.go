package contextperiod

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"health-diary/internal/userday"

	"github.com/jackc/pgx/v5/pgxpool"
)

var allowedTypes = map[string]bool{
	"vacation": true, "trip": true, "temporary_stay": true, "relocation": true, "other": true,
}

// SyncConfirmed projects confirmed life_context events into context_periods.
func SyncConfirmed(ctx context.Context, db *pgxpool.Pool, userID, timezone, dayStart string) error {
	rows, err := db.Query(ctx, `SELECT e.id::text,e.entry_id::text,e.occurred_at,e.ended_at,e.attributes
		FROM health_events e
		WHERE e.user_id=$1 AND e.kind='life_context' AND e.status='confirmed' AND e.deleted_at IS NULL
		AND NOT EXISTS (SELECT 1 FROM context_periods c WHERE c.created_from_event_id=e.id)
		ORDER BY e.occurred_at,e.id`, userID)
	if err != nil {
		return err
	}
	defer rows.Close()
	type item struct {
		id, entryID string
		at          time.Time
		ended       *time.Time
		data        map[string]any
	}
	var pending []item
	for rows.Next() {
		var row item
		var raw json.RawMessage
		if err := rows.Scan(&row.id, &row.entryID, &row.at, &row.ended, &raw); err != nil {
			return err
		}
		_ = json.Unmarshal(raw, &row.data)
		pending = append(pending, row)
	}
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		loc = time.UTC
	}
	boundary := userday.Start(dayStart)
	for _, row := range pending {
		periodType, _ := row.data["period_type"].(string)
		if !allowedTypes[periodType] {
			periodType = "other"
		}
		phase, _ := row.data["phase"].(string)
		placeLabel := strings.TrimSpace(stringValue(row.data["place_label"]))
		if placeLabel == "" {
			placeLabel = strings.TrimSpace(stringValue(row.data["city"]))
		}
		startedOn := userday.Date(row.at, loc, boundary)
		var endedOn any
		if row.ended != nil {
			endedOn = userday.Date(*row.ended, loc, boundary)
		} else if endDate, ok := row.data["ended_on"].(string); ok && endDate != "" {
			endedOn = endDate
		}
		status := "open"
		if endedOn != nil {
			status = "closed"
		}

		// Returning home closes the latest open period.
		if phase == "end" || phase == "return" || strings.EqualFold(placeLabel, "липецк") && periodType == "other" {
			if _, err := db.Exec(ctx, `UPDATE context_periods SET ended_on=LEAST(COALESCE(ended_on,$2::date),$2::date),status='closed',updated_at=now(),revision=revision+1
				WHERE id=(SELECT id FROM context_periods WHERE user_id=$1 AND status='open' ORDER BY started_on DESC LIMIT 1)`,
				userID, startedOn); err != nil {
				return err
			}
			if phase == "end" || phase == "return" {
				if _, err := db.Exec(ctx, `INSERT INTO context_periods(user_id,period_type,place_label,started_on,ended_on,status,source_entry_id,created_from_event_id)
					VALUES($1,'other',$2,$3,$3,'closed',$4,$5)`, userID, nullIfEmpty(placeLabel), startedOn, row.entryID, row.id); err != nil {
					return err
				}
				continue
			}
		}

		var placeID any
		if placeLabel != "" {
			var id string
			_ = db.QueryRow(ctx, `SELECT id::text FROM places WHERE lower(label)=lower($1) ORDER BY user_id NULLS FIRST LIMIT 1`, placeLabel).Scan(&id)
			if id != "" {
				placeID = id
			}
		}
		if _, err := db.Exec(ctx, `INSERT INTO context_periods(user_id,period_type,place_id,place_label,started_on,ended_on,status,source_entry_id,created_from_event_id)
			VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
			userID, periodType, placeID, nullIfEmpty(placeLabel), startedOn, endedOn, status, row.entryID, row.id); err != nil {
			return err
		}
		// Open relocation/trip can close previous open temporary periods of other types.
		if status == "open" {
			if _, err := db.Exec(ctx, `UPDATE context_periods SET ended_on=$2::date - 1,status='closed',updated_at=now(),revision=revision+1
				WHERE user_id=$1 AND status='open' AND created_from_event_id IS DISTINCT FROM $3::uuid AND started_on < $2::date`,
				userID, startedOn, row.id); err != nil {
				return err
			}
		}
	}
	return nil
}

// ActivePlaceForDate returns the context place for a local date, if any.
func ActivePlaceForDate(ctx context.Context, db *pgxpool.Pool, userID, localDate string) (placeID, placeLabel, periodType, segment string, err error) {
	err = db.QueryRow(ctx, `SELECT COALESCE(place_id::text,''), COALESCE(place_label,''), period_type,
		CASE WHEN started_on=$2::date THEN 'start' WHEN ended_on=$2::date THEN 'end' ELSE 'middle' END
		FROM context_periods
		WHERE user_id=$1 AND status IN ('open','closed')
		AND started_on <= $2::date AND (ended_on IS NULL OR ended_on >= $2::date)
		ORDER BY started_on DESC LIMIT 1`, userID, localDate).Scan(&placeID, &placeLabel, &periodType, &segment)
	return
}

func stringValue(value any) string {
	text, _ := value.(string)
	return text
}

func nullIfEmpty(value string) any {
	if value == "" {
		return nil
	}
	return value
}
