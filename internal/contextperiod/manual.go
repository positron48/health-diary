package contextperiod

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrConflict      = errors.New("context period overlaps an existing period")
	ErrPlaceNotFound = errors.New("place is not available to user")
)

type ManualInput struct {
	PeriodType string
	PlaceID    string
	StartedOn  string
	EndedOn    *string
}

type ManualPeriod struct {
	ID         string  `json:"id"`
	PeriodType string  `json:"period_type"`
	PlaceID    string  `json:"place_id"`
	PlaceLabel string  `json:"place_label"`
	StartedOn  string  `json:"started_on"`
	EndedOn    *string `json:"ended_on,omitempty"`
	Status     string  `json:"status"`
	Revision   int     `json:"revision"`
}

// ValidateManualInput validates the date-only contract before any write.
func ValidateManualInput(input ManualInput) map[string]string {
	fields := map[string]string{}
	if !allowedTypes[input.PeriodType] {
		fields["period_type"] = "unsupported"
	}
	start, err := time.Parse("2006-01-02", input.StartedOn)
	if err != nil {
		fields["started_on"] = "must be YYYY-MM-DD"
	}
	if input.EndedOn != nil {
		end, endErr := time.Parse("2006-01-02", *input.EndedOn)
		if endErr != nil {
			fields["ended_on"] = "must be YYYY-MM-DD"
		} else if err == nil && end.Before(start) {
			fields["ended_on"] = "must be on or after started_on"
		}
	}
	if input.PlaceID == "" {
		fields["place_id"] = "required"
	}
	return fields
}

// CreateManual stores a user-confirmed period. The preceding open override is
// closed on the prior local date; all other overlaps are rejected.
func CreateManual(ctx context.Context, db *pgxpool.Pool, userID string, input ManualInput) (ManualPeriod, error) {
	if len(ValidateManualInput(input)) > 0 {
		return ManualPeriod{}, errors.New("invalid manual context period")
	}
	tx, err := db.Begin(ctx)
	if err != nil {
		return ManualPeriod{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var placeLabel string
	if err := tx.QueryRow(ctx, `SELECT label FROM places WHERE id=$1 AND (user_id IS NULL OR user_id=$2)`, input.PlaceID, userID).Scan(&placeLabel); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ManualPeriod{}, ErrPlaceNotFound
		}
		return ManualPeriod{}, err
	}

	var conflict bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS (
		SELECT 1 FROM context_periods
		WHERE user_id=$1 AND status IN ('open','closed')
		AND NOT (status='open' AND started_on < $2::date)
		AND started_on <= COALESCE($3::date,'infinity'::date)
		AND COALESCE(ended_on,'infinity'::date) >= $2::date
	)`, userID, input.StartedOn, input.EndedOn).Scan(&conflict); err != nil {
		return ManualPeriod{}, err
	}
	if conflict {
		return ManualPeriod{}, ErrConflict
	}

	if _, err := tx.Exec(ctx, `UPDATE context_periods SET
		ended_on=$2::date - 1,status='closed',revision=revision+1,updated_at=now()
		WHERE user_id=$1 AND status='open' AND started_on < $2::date`, userID, input.StartedOn); err != nil {
		return ManualPeriod{}, err
	}

	status := "open"
	if input.EndedOn != nil {
		status = "closed"
	}
	period := ManualPeriod{
		PeriodType: input.PeriodType,
		PlaceID:    input.PlaceID,
		PlaceLabel: placeLabel,
		StartedOn:  input.StartedOn,
		EndedOn:    input.EndedOn,
		Status:     status,
	}
	if err := tx.QueryRow(ctx, `INSERT INTO context_periods(
		user_id,period_type,place_id,place_label,started_on,ended_on,status)
		VALUES($1,$2,$3,$4,$5::date,$6::date,$7)
		RETURNING id::text,revision`, userID, input.PeriodType, input.PlaceID, placeLabel, input.StartedOn, input.EndedOn, status).
		Scan(&period.ID, &period.Revision); err != nil {
		return ManualPeriod{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return ManualPeriod{}, err
	}
	return period, nil
}

// ReturnHome ends every override active on startedOn. A period that starts on
// that date is cancelled; an earlier period is closed on the previous date.
func ReturnHome(ctx context.Context, db *pgxpool.Pool, userID, startedOn string) error {
	if _, err := time.Parse("2006-01-02", startedOn); err != nil {
		return err
	}
	tx, err := db.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `UPDATE context_periods SET
		status='cancelled',revision=revision+1,updated_at=now()
		WHERE user_id=$1 AND status IN ('open','closed')
		AND started_on=$2::date AND (ended_on IS NULL OR ended_on >= $2::date)`, userID, startedOn); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `UPDATE context_periods SET
		ended_on=$2::date - 1,status='closed',revision=revision+1,updated_at=now()
		WHERE user_id=$1 AND status IN ('open','closed')
		AND started_on < $2::date AND (ended_on IS NULL OR ended_on >= $2::date)`, userID, startedOn); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
