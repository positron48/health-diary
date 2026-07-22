package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"health-diary/internal/analytics"
	"health-diary/internal/auth"
	"health-diary/internal/episode"
	"health-diary/internal/ingest"
	"health-diary/internal/userday"

	"github.com/jackc/pgx/v5"
)

var eventKinds = map[string]bool{
	"pain_observation": true, "medication_intake": true, "wellbeing": true,
	"activity": true, "sleep": true, "food_drink": true, "measurement": true, "note": true,
}

func (a *App) createEntry(w http.ResponseWriter, r *http.Request) {
	if a.ingest == nil {
		writeAPIError(w, r, http.StatusServiceUnavailable, "capture_unavailable", "Добавление записей временно недоступно", nil)
		return
	}
	idempotencyKey := r.Header.Get("Idempotency-Key")
	if len(idempotencyKey) < 8 || len(idempotencyKey) > 200 {
		writeAPIError(w, r, 422, "validation_failed", "Проверьте введённые данные", map[string]string{"idempotency_key": "must contain 8 to 200 characters"})
		return
	}
	var input struct {
		Text string `json:"text"`
		Date string `json:"date,omitempty"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		writeAPIError(w, r, 400, "malformed_request", "Некорректный JSON", nil)
		return
	}
	input.Text = strings.TrimSpace(input.Text)
	if input.Text == "" || len([]rune(input.Text)) > 4000 {
		writeAPIError(w, r, 422, "validation_failed", "Проверьте введённые данные", map[string]string{"text": "must contain 1 to 4000 characters"})
		return
	}
	user := sessionUser(r)
	var reference time.Time
	if input.Date != "" {
		loc := userLocation(user.Timezone)
		day, err := time.ParseInLocation("2006-01-02", input.Date, loc)
		if err != nil {
			writeAPIError(w, r, 422, "validation_failed", "Проверьте дату", map[string]string{"date": "must be YYYY-MM-DD"})
			return
		}
		now := time.Now().In(loc)
		reference = day.Add(time.Duration(now.Hour())*time.Hour + time.Duration(now.Minute())*time.Minute + time.Duration(now.Second())*time.Second)
	}
	result, err := a.ingest.CaptureWebText(r.Context(), ingest.WebCapture{
		UserID: user.ID, Text: input.Text, IdempotencyKey: idempotencyKey, SentAt: time.Now().UTC(), ReferenceAt: reference,
	})
	if err != nil {
		writeAPIError(w, r, 500, "internal_error", "Не удалось сохранить запись", nil)
		return
	}
	status := http.StatusCreated
	if result.Duplicate {
		status = http.StatusOK
	}
	writeJSON(w, status, map[string]any{"entry_id": result.EntryID, "status": "queued"})
}

type eventDTO struct {
	ID            string          `json:"id"`
	EntryID       string          `json:"entry_id"`
	EpisodeID     *string         `json:"episode_id"`
	Kind          string          `json:"kind"`
	OccurredAt    time.Time       `json:"occurred_at"`
	EndedAt       *time.Time      `json:"ended_at"`
	TimePrecision string          `json:"time_precision"`
	Data          json.RawMessage `json:"data"`
	Revision      int             `json:"revision"`
}

func (a *App) eventsV1(w http.ResponseWriter, r *http.Request) {
	user := sessionUser(r)
	q := r.URL.Query()
	limit, _ := strconv.Atoi(q.Get("limit"))
	if limit == 0 {
		limit = 50
	}
	if limit < 1 || limit > 100 {
		writeAPIError(w, r, 422, "validation_failed", "Проверьте параметры", map[string]string{"limit": "must be between 1 and 100"})
		return
	}
	from, to, fields := parseRange(q.Get("from"), q.Get("to"), user.Timezone, user.DayStart)
	kinds := q["kind"]
	for _, kind := range kinds {
		if !eventKinds[kind] {
			fields["kind"] = "unsupported event kind"
		}
	}
	if q.Get("status") != "" && q.Get("status") != "confirmed" {
		fields["status"] = "only confirmed events are available"
	}
	if len(fields) > 0 {
		writeAPIError(w, r, 422, "validation_failed", "Проверьте параметры", fields)
		return
	}
	rows, err := a.db.Query(r.Context(), `SELECT e.id::text,e.entry_id::text,COALESCE(p.episode_id,m.episode_id)::text,e.kind,e.occurred_at,e.ended_at,e.time_precision,e.attributes,e.revision
		FROM health_events e
		LEFT JOIN pain_observations p ON p.event_id=e.id
		LEFT JOIN medication_intakes m ON m.event_id=e.id
		WHERE e.user_id=$1 AND e.status='confirmed' AND e.deleted_at IS NULL
		AND ($2::timestamptz IS NULL OR e.occurred_at >= $2) AND ($3::timestamptz IS NULL OR e.occurred_at < $3)
		AND (cardinality($4::text[])=0 OR e.kind=ANY($4)) AND ($5='' OR (e.occurred_at,e.id) < (
			SELECT occurred_at,id FROM health_events WHERE id=NULLIF($5,'')::uuid AND user_id=$1))
		ORDER BY e.occurred_at DESC,e.id DESC LIMIT $6`, user.ID, from, to, kinds, q.Get("cursor"), limit+1)
	if err != nil {
		writeAPIError(w, r, 500, "internal_error", "Не удалось загрузить события", nil)
		return
	}
	defer rows.Close()
	items := []eventDTO{}
	for rows.Next() {
		var item eventDTO
		var episodeID *string
		if err := rows.Scan(&item.ID, &item.EntryID, &episodeID, &item.Kind, &item.OccurredAt, &item.EndedAt, &item.TimePrecision, &item.Data, &item.Revision); err != nil {
			writeAPIError(w, r, 500, "internal_error", "Не удалось загрузить события", nil)
			return
		}
		item.EpisodeID = episodeID
		items = append(items, item)
	}
	var cursor string
	if len(items) > limit {
		cursor = items[limit-1].ID
		items = items[:limit]
	}
	writeJSON(w, 200, map[string]any{"events": items, "next_cursor": cursor})
}

func (a *App) eventDetail(w http.ResponseWriter, r *http.Request) {
	user := sessionUser(r)
	var item eventDTO
	err := a.db.QueryRow(r.Context(), `SELECT e.id::text,e.entry_id::text,COALESCE(p.episode_id,m.episode_id)::text,e.kind,e.occurred_at,e.ended_at,e.time_precision,e.attributes,e.revision
		FROM health_events e
		LEFT JOIN pain_observations p ON p.event_id=e.id
		LEFT JOIN medication_intakes m ON m.event_id=e.id
		WHERE e.id=$1 AND e.user_id=$2 AND e.status='confirmed' AND e.deleted_at IS NULL`,
		r.PathValue("id"), user.ID).Scan(&item.ID, &item.EntryID, &item.EpisodeID, &item.Kind, &item.OccurredAt, &item.EndedAt, &item.TimePrecision, &item.Data, &item.Revision)
	if errors.Is(err, pgx.ErrNoRows) {
		writeAPIError(w, r, 404, "event_not_found", "Событие не найдено", nil)
		return
	}
	if err != nil {
		writeAPIError(w, r, 500, "internal_error", "Не удалось загрузить событие", nil)
		return
	}
	writeJSON(w, 200, item)
}

type eventPatch struct {
	Revision      int             `json:"revision"`
	OccurredAt    *time.Time      `json:"occurred_at"`
	EndedAt       **time.Time     `json:"ended_at"`
	TimePrecision *string         `json:"time_precision"`
	Data          json.RawMessage `json:"data"`
}

func (a *App) patchEvent(w http.ResponseWriter, r *http.Request) {
	var input eventPatch
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		writeAPIError(w, r, 400, "malformed_request", "Некорректный JSON", nil)
		return
	}
	if input.Revision < 1 {
		writeAPIError(w, r, 422, "validation_failed", "Проверьте введённые данные", map[string]string{"revision": "must be positive"})
		return
	}
	user := sessionUser(r)
	tx, err := a.db.Begin(r.Context())
	if err != nil {
		writeAPIError(w, r, 500, "internal_error", "Не удалось сохранить событие", nil)
		return
	}
	defer tx.Rollback(r.Context())
	var current eventDTO
	err = tx.QueryRow(r.Context(), `SELECT id::text,entry_id::text,kind,occurred_at,ended_at,time_precision,attributes,revision
		FROM health_events WHERE id=$1 AND user_id=$2 AND status='confirmed' AND deleted_at IS NULL FOR UPDATE`,
		r.PathValue("id"), user.ID).Scan(&current.ID, &current.EntryID, &current.Kind, &current.OccurredAt, &current.EndedAt, &current.TimePrecision, &current.Data, &current.Revision)
	if errors.Is(err, pgx.ErrNoRows) {
		writeAPIError(w, r, 404, "event_not_found", "Событие не найдено", nil)
		return
	}
	if err != nil {
		writeAPIError(w, r, 500, "internal_error", "Не удалось сохранить событие", nil)
		return
	}
	if current.Revision != input.Revision {
		writeAPIError(w, r, 409, "revision_conflict", "Событие уже изменилось", nil)
		return
	}
	next := current
	if input.OccurredAt != nil {
		next.OccurredAt = *input.OccurredAt
	}
	if input.EndedAt != nil {
		next.EndedAt = *input.EndedAt
	}
	if input.TimePrecision != nil {
		next.TimePrecision = *input.TimePrecision
	}
	if input.Data != nil {
		merged, err := mergeEventData(current.Data, input.Data)
		if err != nil {
			writeAPIError(w, r, 422, "validation_failed", "Проверьте введённые данные", map[string]string{"data": "must be a JSON object"})
			return
		}
		next.Data = merged
	}
	if fields := validateEvent(next); len(fields) > 0 {
		writeAPIError(w, r, 422, "validation_failed", "Проверьте введённые данные", fields)
		return
	}
	before, _ := json.Marshal(current)
	next.Revision++
	after, _ := json.Marshal(next)
	tag, err := tx.Exec(r.Context(), `UPDATE health_events SET occurred_at=$3,ended_at=$4,time_precision=$5,attributes=$6,revision=$7,updated_at=now()
		WHERE id=$1 AND user_id=$2 AND revision=$8`, current.ID, user.ID, next.OccurredAt, next.EndedAt, next.TimePrecision, next.Data, next.Revision, current.Revision)
	if err == nil && tag.RowsAffected() == 1 {
		_, err = tx.Exec(r.Context(), `INSERT INTO event_revisions(event_id,revision,changed_by,before_data,after_data,reason)
			VALUES($1,$2,'web_user',$3,$4,'manual edit')`, current.ID, next.Revision, before, after)
	}
	if err != nil {
		writeAPIError(w, r, 500, "internal_error", "Не удалось сохранить событие", nil)
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		writeAPIError(w, r, 500, "internal_error", "Не удалось сохранить событие", nil)
		return
	}
	_ = episode.SyncConfirmed(r.Context(), a.db, a.cipher, user.ID)
	writeJSON(w, 200, next)
}

func mergeEventData(current, patch json.RawMessage) (json.RawMessage, error) {
	base := map[string]any{}
	if len(current) > 0 {
		if err := json.Unmarshal(current, &base); err != nil {
			return nil, err
		}
	}
	updates := map[string]any{}
	if err := json.Unmarshal(patch, &updates); err != nil {
		return nil, err
	}
	for key, value := range updates {
		if value == nil {
			delete(base, key)
			continue
		}
		base[key] = value
	}
	return json.Marshal(base)
}

func validateEvent(event eventDTO) map[string]string {
	fields := map[string]string{}
	switch event.TimePrecision {
	case "exact", "approximate", "date_only", "inferred_from_message":
	default:
		fields["time_precision"] = "unsupported precision"
	}
	if event.EndedAt != nil && event.EndedAt.Before(event.OccurredAt) {
		fields["ended_at"] = "must not be before occurred_at"
	}
	var data map[string]any
	if len(event.Data) == 0 || json.Unmarshal(event.Data, &data) != nil {
		fields["data"] = "must be a JSON object"
		return fields
	}
	checkRange := func(name string, min, max float64) {
		if value, ok := data[name]; ok && value != nil {
			number, ok := value.(float64)
			if !ok || number < min || number > max {
				fields["data."+name] = fmt.Sprintf("must be between %v and %v", min, max)
			}
		}
	}
	switch event.Kind {
	case "pain_observation":
		checkRange("intensity", 0, 10)
		checkRange("functional_impact", 0, 3)
		if value, ok := data["phase"]; ok && value != nil {
			text, ok := value.(string)
			if !ok || (text != "start" && text != "update" && text != "end") {
				fields["data.phase"] = "must be start, update or end"
			}
		}
	case "medication_intake":
		if value, ok := data["dose_value"].(float64); ok && value <= 0 {
			fields["data.dose_value"] = "must be positive"
		}
		checkRange("effect_rating", -2, 2)
		if value, ok := data["name_raw"]; ok && value != nil {
			if _, ok := value.(string); !ok {
				fields["data.name_raw"] = "must be a string"
			}
		}
	case "wellbeing":
		for _, name := range []string{"wellbeing_score", "energy_score", "mood_score", "stress_score", "sleep_quality"} {
			checkRange(name, 0, 10)
		}
	case "activity", "sleep":
		if value, ok := data["duration_minutes"].(float64); ok && value <= 0 {
			fields["data.duration_minutes"] = "must be positive"
		}
	}
	return fields
}

func (a *App) calendarV1(w http.ResponseWriter, r *http.Request) {
	user := sessionUser(r)
	month, err := time.Parse("2006-01", r.URL.Query().Get("month"))
	mode := r.URL.Query().Get("mode")
	if mode == "" {
		mode = "overview"
	}
	validModes := map[string]bool{"overview": true, "pain": true, "medication": true, "activity": true, "sleep": true, "wellbeing": true}
	fields := map[string]string{}
	if err != nil {
		fields["month"] = "must be YYYY-MM"
	}
	if !validModes[mode] {
		fields["mode"] = "unsupported mode"
	}
	if len(fields) > 0 {
		writeAPIError(w, r, 422, "validation_failed", "Проверьте параметры", fields)
		return
	}
	loc := userLocation(user.Timezone)
	dayStart := userday.Start(user.DayStart)
	start, _, _ := userday.Bounds(month.Format("2006-01-02"), loc, dayStart)
	nextMonth := month.AddDate(0, 1, 0)
	end, _, _ := userday.Bounds(nextMonth.Format("2006-01-02"), loc, dayStart)
	rows, err := a.db.Query(r.Context(), `SELECT occurred_at,kind,attributes FROM health_events
		WHERE user_id=$1 AND status='confirmed' AND deleted_at IS NULL AND occurred_at >= $2 AND occurred_at < $3 ORDER BY occurred_at`, user.ID, start, end)
	if err != nil {
		writeAPIError(w, r, 500, "internal_error", "Не удалось загрузить календарь", nil)
		return
	}
	defer rows.Close()
	days := map[string]*calendarDay{}
	for rows.Next() {
		var at time.Time
		var kind string
		var data json.RawMessage
		if rows.Scan(&at, &kind, &data) != nil {
			writeAPIError(w, r, 500, "internal_error", "Не удалось загрузить календарь", nil)
			return
		}
		date := userday.Date(at, loc, dayStart)
		day := days[date]
		if day == nil {
			day = &calendarDay{Date: date, HasData: true}
			days[date] = day
		}
		day.add(kind, data)
	}
	pendingRows, _ := a.db.Query(r.Context(), `SELECT e.occurred_at
		FROM health_events e WHERE e.user_id=$1 AND e.status='pending' AND e.deleted_at IS NULL
		AND e.occurred_at >= $2 AND e.occurred_at < $3`, user.ID, start, end)
	if pendingRows != nil {
		defer pendingRows.Close()
		for pendingRows.Next() {
			var at time.Time
			_ = pendingRows.Scan(&at)
			date := userday.Date(at, loc, dayStart)
			day := days[date]
			if day == nil {
				day = &calendarDay{Date: date}
				days[date] = day
			}
			day.PendingCount++
		}
	}
	result := []calendarDay{}
	for day := month; day.Before(nextMonth); day = day.AddDate(0, 0, 1) {
		date := day.Format("2006-01-02")
		if value := days[date]; value != nil {
			result = append(result, *value)
		} else {
			result = append(result, calendarDay{Date: date})
		}
	}
	writeJSON(w, 200, map[string]any{"month": month.Format("2006-01"), "mode": mode, "timezone": user.Timezone, "days": result})
}

type calendarDay struct {
	Date         string         `json:"date"`
	HasData      bool           `json:"has_data"`
	Pain         map[string]any `json:"pain,omitempty"`
	Medication   map[string]any `json:"medication,omitempty"`
	Activity     map[string]any `json:"activity,omitempty"`
	Sleep        map[string]any `json:"sleep,omitempty"`
	Wellbeing    map[string]any `json:"wellbeing,omitempty"`
	PendingCount int            `json:"pending_count"`
}

func (d *calendarDay) add(kind string, raw json.RawMessage) {
	var data map[string]any
	_ = json.Unmarshal(raw, &data)
	switch kind {
	case "pain_observation":
		if d.Pain == nil {
			d.Pain = map[string]any{"episodes": 0, "open": false}
		}
		d.Pain["episodes"] = d.Pain["episodes"].(int) + 1
		if value, ok := data["intensity"].(float64); ok {
			if old, ok := d.Pain["max_intensity"].(float64); !ok || value > old {
				d.Pain["max_intensity"] = value
			}
		}
	case "medication_intake":
		if d.Medication == nil {
			d.Medication = map[string]any{"intakes": 0}
		}
		d.Medication["intakes"] = d.Medication["intakes"].(int) + 1
	case "activity":
		if d.Activity == nil {
			d.Activity = map[string]any{"minutes": 0}
		}
		if value, ok := data["duration_minutes"].(float64); ok {
			d.Activity["minutes"] = d.Activity["minutes"].(int) + int(value)
		}
	case "sleep":
		if d.Sleep == nil {
			d.Sleep = map[string]any{}
		}
		d.Sleep["minutes"] = data["duration_minutes"]
		d.Sleep["quality"] = data["quality_score"]
	case "wellbeing":
		if d.Wellbeing == nil {
			d.Wellbeing = map[string]any{}
		}
		d.Wellbeing["score"] = data["wellbeing_score"]
	}
}

func (a *App) dayTimeline(w http.ResponseWriter, r *http.Request) {
	user := sessionUser(r)
	from, to, err := userday.Bounds(r.PathValue("date"), userLocation(user.Timezone), userday.Start(user.DayStart))
	if err != nil {
		writeAPIError(w, r, 422, "validation_failed", "Проверьте дату", map[string]string{"date": "must be YYYY-MM-DD"})
		return
	}
	rows, err := a.db.Query(r.Context(), `SELECT e.id::text,e.entry_id::text,COALESCE(p.episode_id,m.episode_id)::text,e.kind,e.occurred_at,e.ended_at,e.time_precision,e.attributes,e.revision
		FROM health_events e
		LEFT JOIN pain_observations p ON p.event_id=e.id
		LEFT JOIN medication_intakes m ON m.event_id=e.id
		WHERE e.user_id=$1 AND e.status='confirmed' AND e.deleted_at IS NULL AND e.occurred_at >= $2 AND e.occurred_at < $3 ORDER BY e.occurred_at,e.id`,
		user.ID, from, to)
	if err != nil {
		writeAPIError(w, r, 500, "internal_error", "Не удалось загрузить день", nil)
		return
	}
	defer rows.Close()
	items := []eventDTO{}
	for rows.Next() {
		var item eventDTO
		_ = rows.Scan(&item.ID, &item.EntryID, &item.EpisodeID, &item.Kind, &item.OccurredAt, &item.EndedAt, &item.TimePrecision, &item.Data, &item.Revision)
		items = append(items, item)
	}
	var pending int
	_ = a.db.QueryRow(r.Context(), `SELECT count(*) FROM health_events WHERE user_id=$1 AND status='pending' AND deleted_at IS NULL AND occurred_at >= $2 AND occurred_at < $3`,
		user.ID, from, to).Scan(&pending)
	writeJSON(w, 200, map[string]any{"date": r.PathValue("date"), "timezone": user.Timezone, "events": items, "confirmed_count": len(items), "pending_count": pending})
}

func (a *App) pendingBatchesV1(w http.ResponseWriter, r *http.Request) {
	if status := r.URL.Query().Get("status"); status != "" && status != "pending" {
		writeAPIError(w, r, 422, "validation_failed", "Проверьте параметры", map[string]string{"status": "only pending is supported"})
		return
	}
	user := sessionUser(r)
	rows, err := a.db.Query(r.Context(), `SELECT b.id::text,b.entry_id::text,b.version,b.created_at,j.source_sent_at,j.source_type,
		e.id::text,e.kind,e.occurred_at,e.time_precision,e.attributes,e.revision
		FROM event_batches b JOIN journal_entries j ON j.id=b.entry_id JOIN health_events e ON e.batch_id=b.id
		WHERE b.user_id=$1 AND b.status='pending' AND e.status='pending' AND e.deleted_at IS NULL ORDER BY b.created_at DESC,e.occurred_at`, user.ID)
	if err != nil {
		writeAPIError(w, r, 500, "internal_error", "Не удалось загрузить входящие", nil)
		return
	}
	defer rows.Close()
	type batch struct {
		ID         string           `json:"id"`
		EntryID    string           `json:"entry_id"`
		SourceType string           `json:"source_type"`
		Version    int              `json:"version"`
		CreatedAt  time.Time        `json:"created_at"`
		MessageAt  time.Time        `json:"message_at"`
		Events     []map[string]any `json:"events"`
	}
	ordered := []*batch{}
	index := map[string]*batch{}
	for rows.Next() {
		var id, entryID, source, eventID, kind, precision string
		var version, revision int
		var created, messageAt, occurred time.Time
		var data json.RawMessage
		_ = rows.Scan(&id, &entryID, &version, &created, &messageAt, &source, &eventID, &kind, &occurred, &precision, &data, &revision)
		item := index[id]
		if item == nil {
			item = &batch{ID: id, EntryID: entryID, Version: version, CreatedAt: created, MessageAt: messageAt, SourceType: source}
			index[id] = item
			ordered = append(ordered, item)
		}
		item.Events = append(item.Events, map[string]any{"id": eventID, "kind": kind, "occurred_at": occurred, "time_precision": precision, "data": data, "revision": revision})
	}
	writeJSON(w, 200, map[string]any{"batches": ordered, "count": len(ordered)})
}

func (a *App) sourceEntry(w http.ResponseWriter, r *http.Request) {
	if a.cipher == nil {
		writeAPIError(w, r, 503, "dependency_unavailable", "Расшифровка временно недоступна", nil)
		return
	}
	user := sessionUser(r)
	tx, err := a.db.Begin(r.Context())
	if err != nil {
		writeAPIError(w, r, 500, "internal_error", "Не удалось загрузить запись", nil)
		return
	}
	defer tx.Rollback(r.Context())
	var sealed []byte
	var source string
	var sent time.Time
	err = tx.QueryRow(r.Context(), `SELECT raw_text_ciphertext,source_type,source_sent_at FROM journal_entries WHERE id=$1 AND user_id=$2 AND deleted_at IS NULL`,
		r.PathValue("id"), user.ID).Scan(&sealed, &source, &sent)
	if errors.Is(err, pgx.ErrNoRows) {
		writeAPIError(w, r, 404, "entry_not_found", "Исходная запись не найдена", nil)
		return
	}
	if err != nil {
		writeAPIError(w, r, 500, "internal_error", "Не удалось загрузить запись", nil)
		return
	}
	plain, err := a.cipher.Decrypt(sealed, []byte(user.ID))
	if err != nil {
		writeAPIError(w, r, 500, "source_decryption_failed", "Не удалось расшифровать запись", nil)
		return
	}
	if _, err = tx.Exec(r.Context(), `INSERT INTO source_entry_access_audits(user_id,entry_id) VALUES($1,$2)`, user.ID, r.PathValue("id")); err != nil || tx.Commit(r.Context()) != nil {
		writeAPIError(w, r, 500, "audit_failed", "Не удалось зарегистрировать доступ", nil)
		return
	}
	w.Header().Set("Pragma", "no-cache")
	writeJSON(w, 200, map[string]any{"id": r.PathValue("id"), "source_type": source, "source_sent_at": sent, "text": string(plain)})
}

func (a *App) patchMe(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Timezone *string         `json:"timezone"`
		Locale   *string         `json:"locale"`
		Settings json.RawMessage `json:"settings"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&input) != nil {
		writeAPIError(w, r, 400, "malformed_request", "Некорректный JSON", nil)
		return
	}
	fields := map[string]string{}
	if input.Timezone != nil {
		if _, err := time.LoadLocation(*input.Timezone); err != nil {
			fields["timezone"] = "must be an IANA timezone"
		}
	}
	if input.Locale != nil && *input.Locale != "ru" {
		fields["locale"] = "only ru is supported"
	}
	if len(input.Settings) > 0 {
		var settings map[string]any
		if json.Unmarshal(input.Settings, &settings) != nil {
			fields["settings"] = "must be an object"
		} else if value, ok := settings["day_start_time"]; ok {
			text, ok := value.(string)
			if !ok {
				fields["settings.day_start_time"] = "must be HH:MM"
			} else if _, err := userday.ParseStart(text); err != nil {
				fields["settings.day_start_time"] = "must be HH:MM"
			}
		}
	}
	if len(fields) > 0 {
		writeAPIError(w, r, 422, "validation_failed", "Проверьте настройки", fields)
		return
	}
	user := sessionUser(r)
	var timezone, locale string
	var settings json.RawMessage
	err := a.db.QueryRow(r.Context(), `UPDATE users SET timezone=COALESCE($2,timezone),locale=COALESCE($3,locale),
		settings=CASE WHEN $4::jsonb IS NULL THEN settings ELSE settings || $4::jsonb END,updated_at=now()
		WHERE id=$1 RETURNING timezone,locale,settings`, user.ID, input.Timezone, input.Locale, nullJSON(input.Settings)).Scan(&timezone, &locale, &settings)
	if err != nil {
		writeAPIError(w, r, 500, "internal_error", "Не удалось сохранить настройки", nil)
		return
	}
	writeJSON(w, 200, map[string]any{"id": user.ID, "timezone": timezone, "locale": locale, "settings": settings})
}

func (a *App) revokeSessions(w http.ResponseWriter, r *http.Request) {
	user := sessionUser(r)
	exceptCurrent := r.URL.Query().Get("keep_current") == "true"
	var currentHash []byte
	if exceptCurrent {
		if cookie, err := r.Cookie(a.config.SessionCookieName); err == nil {
			currentHash = auth.Hash(cookie.Value)
		}
	}
	tag, err := a.db.Exec(r.Context(), `UPDATE web_sessions SET revoked_at=now() WHERE user_id=$1 AND revoked_at IS NULL AND ($2::bytea IS NULL OR token_hash<>$2)`, user.ID, currentHash)
	if err != nil {
		writeAPIError(w, r, 500, "internal_error", "Не удалось отозвать сессии", nil)
		return
	}
	writeJSON(w, 200, map[string]any{"revoked": tag.RowsAffected(), "kept_current": exceptCurrent})
}

func nullJSON(raw json.RawMessage) any {
	if len(raw) == 0 {
		return nil
	}
	return raw
}

func parseRange(fromText, toText, timezone, dayStartText string) (*time.Time, *time.Time, map[string]string) {
	fields := map[string]string{}
	loc := userLocation(timezone)
	dayStart := userday.Start(dayStartText)
	parse := func(value, name string, inclusiveEnd bool) *time.Time {
		if value == "" {
			return nil
		}
		from, to, err := userday.Bounds(value, loc, dayStart)
		if err != nil {
			fields[name] = "must be YYYY-MM-DD"
			return nil
		}
		if inclusiveEnd {
			return &to
		}
		return &from
	}
	from, to := parse(fromText, "from", false), parse(toText, "to", true)
	if from != nil && to != nil && !from.Before(*to) {
		fields["to"] = "must not be before from"
	}
	return from, to, fields
}

func sessionUser(r *http.Request) auth.SessionUser {
	return r.Context().Value(sessionContextKey{}).(auth.SessionUser)
}

func (a *App) analyticsSummaryV1(w http.ResponseWriter, r *http.Request) {
	user := sessionUser(r)
	from, to, fields := parseRange(r.URL.Query().Get("from"), r.URL.Query().Get("to"), user.Timezone, user.DayStart)
	if len(fields) > 0 || from == nil || to == nil {
		if from == nil {
			fields["from"] = "is required"
		}
		if to == nil {
			fields["to"] = "is required"
		}
		writeAPIError(w, r, 422, "validation_failed", "Проверьте период", fields)
		return
	}
	events, err := analytics.New(a.db).Events(r.Context(), user.ID, *from, *to)
	if err != nil {
		writeAPIError(w, r, 500, "internal_error", "Не удалось рассчитать аналитику", nil)
		return
	}
	summary := analytics.BuildSummary(events, from.In(userLocation(user.Timezone)), to.In(userLocation(user.Timezone)), user.Timezone, userday.Start(user.DayStart))
	var pending, episodes, closed int
	_ = a.db.QueryRow(r.Context(), `SELECT count(*) FROM health_events WHERE user_id=$1 AND status='pending' AND deleted_at IS NULL AND occurred_at >= $2 AND occurred_at < $3`, user.ID, from, to).Scan(&pending)
	_ = a.db.QueryRow(r.Context(), `SELECT count(*),count(*) FILTER (WHERE status='closed') FROM symptom_episodes WHERE user_id=$1 AND started_at < $3 AND COALESCE(ended_at,$3)>=$2`, user.ID, from, to).Scan(&episodes, &closed)
	writeJSON(w, 200, map[string]any{"coverage": map[string]any{"observation_days": summary.ObservationDays, "diary_days": summary.DiaryDays, "confirmed_events": summary.ConfirmedEvents, "pending_events": pending, "episodes": episodes, "closed_episodes": closed}, "metrics": summary, "formula_version": analytics.FormulaVersion})
}

func (a *App) analyticsAssociations(w http.ResponseWriter, r *http.Request) {
	user := sessionUser(r)
	from, to, fields := parseRange(r.URL.Query().Get("from"), r.URL.Query().Get("to"), user.Timezone, user.DayStart)
	if len(fields) > 0 || from == nil || to == nil {
		writeAPIError(w, r, 422, "validation_failed", "Проверьте период", map[string]string{"from": "required YYYY-MM-DD", "to": "required YYYY-MM-DD"})
		return
	}
	events, err := analytics.New(a.db).Events(r.Context(), user.ID, *from, *to)
	if err != nil {
		writeAPIError(w, r, 500, "internal_error", "Не удалось рассчитать аналитику", nil)
		return
	}
	days := int(to.Sub(*from).Hours() / 24)
	starts := 0
	exposureDays := map[string]bool{}
	for _, event := range events {
		if event.Kind == "pain_observation" {
			var data map[string]any
			_ = json.Unmarshal(event.Attributes, &data)
			if data["phase"] == "start" || data["phase"] == nil {
				starts++
			}
		}
		if event.Kind == "sleep" || event.Kind == "activity" || event.Kind == "wellbeing" {
			exposureDays[userday.Date(event.OccurredAt, userLocation(user.Timezone), userday.Start(user.DayStart))] = true
		}
	}
	requirements := map[string]any{
		"observation_days": map[string]int{"actual": days, "required": 56},
		"headache_starts":  map[string]int{"actual": starts, "required": 8},
		"exposure_days":    map[string]int{"actual": len(exposureDays), "required": 10},
	}
	// MVP deliberately does not emit an association until comparable exposed
	// and unexposed windows are represented by a versioned rule.
	writeJSON(w, 200, map[string]any{"status": "insufficient_data", "requirements": requirements, "associations": []any{}, "formula_version": "health-diary-associations-v1", "limitation": "Записи описывают возможные связи и не доказывают причинность"})
}

func (a *App) analyticsMedications(w http.ResponseWriter, r *http.Request) {
	user := sessionUser(r)
	from, to, fields := parseRange(r.URL.Query().Get("from"), r.URL.Query().Get("to"), user.Timezone, user.DayStart)
	if len(fields) > 0 || from == nil || to == nil {
		writeAPIError(w, r, 422, "validation_failed", "Проверьте период", map[string]string{"from": "required YYYY-MM-DD", "to": "required YYYY-MM-DD"})
		return
	}
	rows, err := a.db.Query(r.Context(), `SELECT COALESCE(NULLIF(attributes->>'normalized_name',''),NULLIF(attributes->>'medication_name_normalized',''),NULLIF(attributes->>'name_raw',''),NULLIF(attributes->>'name',''),'Не указано'),
		count(*),count(DISTINCT ((occurred_at AT TIME ZONE $4) - make_interval(mins => $5))::date),count(*) FILTER (WHERE attributes->'effect_rating' IS NOT NULL AND attributes->>'effect_rating' <> 'null')
		FROM health_events WHERE user_id=$1 AND status='confirmed' AND deleted_at IS NULL AND kind='medication_intake'
		AND occurred_at >= $2 AND occurred_at < $3 GROUP BY 1 ORDER BY count(*) DESC`, user.ID, from, to, user.Timezone, int(userday.Start(user.DayStart).Minutes()))
	if err != nil {
		writeAPIError(w, r, 500, "internal_error", "Не удалось рассчитать лекарства", nil)
		return
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var name string
		var intakes, days, effects int
		_ = rows.Scan(&name, &intakes, &days, &effects)
		items = append(items, map[string]any{"name": name, "intakes": intakes, "intake_days": days, "recorded_effect_n": effects})
	}
	writeJSON(w, 200, map[string]any{"medications": items, "formula_version": "health-diary-medications-v1", "limitation": "Отсутствие записи об эффекте не означает отсутствие эффекта"})
}

func (a *App) syncEpisodeProjection(ctx context.Context, userID string) error {
	return episode.SyncConfirmed(ctx, a.db, a.cipher, userID)
}

func (a *App) episodes(w http.ResponseWriter, r *http.Request) {
	user := sessionUser(r)
	_ = episode.SyncConfirmed(r.Context(), a.db, a.cipher, user.ID)
	status := r.URL.Query().Get("status")
	if status != "" && status != "open" && status != "closed" {
		writeAPIError(w, r, 422, "validation_failed", "Проверьте параметры", map[string]string{"status": "must be open or closed"})
		return
	}
	rows, err := a.db.Query(r.Context(), `SELECT id::text,started_at,ended_at,start_precision,end_precision,status,max_intensity,revision
		FROM symptom_episodes WHERE user_id=$1 AND ($2='' OR status=$2) ORDER BY started_at DESC LIMIT 100`, user.ID, status)
	if err != nil {
		writeAPIError(w, r, 500, "internal_error", "Не удалось загрузить эпизоды", nil)
		return
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var id, startPrecision, state string
		var endPrecision *string
		var started time.Time
		var ended *time.Time
		var intensity *int
		var revision int
		_ = rows.Scan(&id, &started, &ended, &startPrecision, &endPrecision, &state, &intensity, &revision)
		items = append(items, episodeMap(id, started, ended, startPrecision, endPrecision, state, intensity, revision))
	}
	writeJSON(w, 200, map[string]any{"episodes": items})
}

func episodeMap(id string, started time.Time, ended *time.Time, startPrecision string, endPrecision *string, status string, intensity *int, revision int) map[string]any {
	var duration any
	if ended != nil {
		duration = int(ended.Sub(started).Minutes())
	}
	return map[string]any{"id": id, "started_at": started, "ended_at": ended, "start_precision": startPrecision, "end_precision": endPrecision, "status": status, "max_intensity": intensity, "duration_minutes": duration, "revision": revision}
}

func (a *App) episodeDetail(w http.ResponseWriter, r *http.Request) {
	user := sessionUser(r)
	_ = episode.SyncConfirmed(r.Context(), a.db, a.cipher, user.ID)
	var id, startPrecision, status string
	var endPrecision *string
	var started time.Time
	var ended *time.Time
	var intensity *int
	var revision int
	err := a.db.QueryRow(r.Context(), `SELECT id::text,started_at,ended_at,start_precision,end_precision,status,max_intensity,revision FROM symptom_episodes WHERE id=$1 AND user_id=$2`, r.PathValue("id"), user.ID).
		Scan(&id, &started, &ended, &startPrecision, &endPrecision, &status, &intensity, &revision)
	if errors.Is(err, pgx.ErrNoRows) {
		writeAPIError(w, r, 404, "episode_not_found", "Эпизод не найден", nil)
		return
	}
	rows, err := a.db.Query(r.Context(), `SELECT e.id::text,e.entry_id::text,COALESCE(p.episode_id,m.episode_id)::text,e.kind,e.occurred_at,e.ended_at,e.time_precision,e.attributes,e.revision
		FROM health_events e LEFT JOIN pain_observations p ON p.event_id=e.id LEFT JOIN medication_intakes m ON m.event_id=e.id
		WHERE e.user_id=$1 AND e.status='confirmed' AND e.deleted_at IS NULL AND (p.episode_id=$2 OR m.episode_id=$2) ORDER BY e.occurred_at`, user.ID, id)
	if err != nil {
		writeAPIError(w, r, 500, "internal_error", "Не удалось загрузить эпизод", nil)
		return
	}
	defer rows.Close()
	events := []eventDTO{}
	for rows.Next() {
		var item eventDTO
		_ = rows.Scan(&item.ID, &item.EntryID, &item.EpisodeID, &item.Kind, &item.OccurredAt, &item.EndedAt, &item.TimePrecision, &item.Data, &item.Revision)
		events = append(events, item)
	}
	value := episodeMap(id, started, ended, startPrecision, endPrecision, status, intensity, revision)
	value["events"] = events
	value["observation_count"] = len(events)
	writeJSON(w, 200, value)
}

func (a *App) closeEpisode(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Revision  int       `json:"revision"`
		EndedAt   time.Time `json:"ended_at"`
		Precision string    `json:"precision"`
	}
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&input) != nil {
		writeAPIError(w, r, 400, "malformed_request", "Некорректный JSON", nil)
		return
	}
	if input.Revision < 1 || input.EndedAt.IsZero() || (input.Precision != "exact" && input.Precision != "approximate" && input.Precision != "date_only") {
		writeAPIError(w, r, 422, "validation_failed", "Проверьте данные завершения", nil)
		return
	}
	a.mutateEpisode(w, r, input.Revision, "close", &input.EndedAt, &input.Precision)
}

func (a *App) reopenEpisode(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Revision int `json:"revision"`
	}
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 1024)).Decode(&input) != nil || input.Revision < 1 {
		writeAPIError(w, r, 422, "validation_failed", "Требуется актуальная revision", map[string]string{"revision": "must be positive"})
		return
	}
	a.mutateEpisode(w, r, input.Revision, "reopen", nil, nil)
}

func (a *App) mutateEpisode(w http.ResponseWriter, r *http.Request, revision int, action string, endedAt *time.Time, precision *string) {
	user := sessionUser(r)
	tx, err := a.db.Begin(r.Context())
	if err != nil {
		writeAPIError(w, r, 500, "internal_error", "Не удалось изменить эпизод", nil)
		return
	}
	defer tx.Rollback(r.Context())
	var started time.Time
	var currentStatus string
	var oldEnded *time.Time
	var oldPrecision *string
	var currentRevision int
	err = tx.QueryRow(r.Context(), `SELECT started_at,ended_at,end_precision,status,revision FROM symptom_episodes WHERE id=$1 AND user_id=$2 FOR UPDATE`, r.PathValue("id"), user.ID).
		Scan(&started, &oldEnded, &oldPrecision, &currentStatus, &currentRevision)
	if errors.Is(err, pgx.ErrNoRows) {
		writeAPIError(w, r, 404, "episode_not_found", "Эпизод не найден", nil)
		return
	}
	if currentRevision != revision || (action == "close" && currentStatus != "open") || (action == "reopen" && currentStatus != "closed") {
		writeAPIError(w, r, 409, "episode_state_conflict", "Состояние эпизода уже изменилось", nil)
		return
	}
	if endedAt != nil && endedAt.Before(started) {
		writeAPIError(w, r, 422, "validation_failed", "Время завершения раньше начала", map[string]string{"ended_at": "must not be before started_at"})
		return
	}
	nextStatus := "closed"
	if action == "reopen" {
		nextStatus, endedAt, precision = "open", nil, nil
	}
	before, _ := json.Marshal(map[string]any{"status": currentStatus, "ended_at": oldEnded, "end_precision": oldPrecision, "revision": currentRevision})
	after, _ := json.Marshal(map[string]any{"status": nextStatus, "ended_at": endedAt, "end_precision": precision, "revision": currentRevision + 1})
	_, err = tx.Exec(r.Context(), `UPDATE symptom_episodes SET status=$3,ended_at=$4,end_precision=$5,revision=revision+1,updated_at=now() WHERE id=$1 AND user_id=$2`, r.PathValue("id"), user.ID, nextStatus, endedAt, precision)
	if err == nil {
		_, err = tx.Exec(r.Context(), `INSERT INTO episode_revisions(episode_id,user_id,revision,action,before_data,after_data) VALUES($1,$2,$3,$4,$5,$6)`, r.PathValue("id"), user.ID, currentRevision+1, action, before, after)
	}
	if err != nil || tx.Commit(r.Context()) != nil {
		writeAPIError(w, r, 500, "internal_error", "Не удалось изменить эпизод", nil)
		return
	}
	writeJSON(w, 200, map[string]any{"id": r.PathValue("id"), "status": nextStatus, "ended_at": endedAt, "end_precision": precision, "revision": currentRevision + 1})
}

func (a *App) afterEventMutation(r *http.Request) {
	_ = episode.SyncConfirmed(r.Context(), a.db, a.cipher, sessionUser(r).ID)
}
