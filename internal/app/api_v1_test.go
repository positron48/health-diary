package app

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestWriteAPIErrorUsesStableEnvelope(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/api/v1/events", nil)
	request.Header.Set("X-Request-ID", "request-1")
	response := httptest.NewRecorder()
	writeAPIError(response, request, 422, "validation_failed", "Проверьте данные", map[string]string{"revision": "required"})
	var body struct {
		Error apiError `json:"error"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if response.Code != 422 || body.Error.Code != "validation_failed" || body.Error.RequestID != "request-1" || body.Error.Fields["revision"] != "required" {
		t.Fatalf("unexpected response: %#v", body)
	}
}

func TestValidateEventPreservesNullableValuesAndChecksRanges(t *testing.T) {
	event := eventDTO{
		Kind:          "pain_observation",
		OccurredAt:    time.Now(),
		TimePrecision: "approximate",
		Data:          json.RawMessage(`{"intensity":11,"functional_impact":null}`),
	}
	fields := validateEvent(event)
	if fields["data.intensity"] == "" || fields["data.functional_impact"] != "" {
		t.Fatalf("unexpected fields: %#v", fields)
	}
}

func TestMergeEventDataPreservesUnchangedAttributes(t *testing.T) {
	merged, err := mergeEventData(json.RawMessage(`{"symptom_type":"headache","phase":"start","locations":["top_of_head"]}`), json.RawMessage(`{"intensity":5,"locations":null}`))
	if err != nil {
		t.Fatal(err)
	}
	var data map[string]any
	if err := json.Unmarshal(merged, &data); err != nil {
		t.Fatal(err)
	}
	if data["symptom_type"] != "headache" || data["phase"] != "start" || data["intensity"] != float64(5) {
		t.Fatalf("unexpected merge: %#v", data)
	}
	if _, ok := data["locations"]; ok {
		t.Fatalf("null patch should clear locations: %#v", data)
	}
}

func TestCalendarDayAggregatesDeterministically(t *testing.T) {
	day := calendarDay{Date: "2026-07-22", HasData: true}
	day.add("pain_observation", json.RawMessage(`{"intensity":4}`))
	day.add("pain_observation", json.RawMessage(`{"intensity":7}`))
	day.add("activity", json.RawMessage(`{"duration_minutes":30}`))
	day.add("activity", json.RawMessage(`{"duration_minutes":15}`))
	day.add("wellbeing", json.RawMessage(`{"wellbeing_score":6,"motivation_score":4}`))
	if day.Pain["max_intensity"] != float64(7) || day.Activity["minutes"] != 45 || day.Wellbeing["motivation"] != float64(4) {
		t.Fatalf("unexpected aggregate: %#v", day)
	}
}

func TestParseRangeUsesInclusiveLocalDates(t *testing.T) {
	from, to, fields := parseRange("2026-07-01", "2026-07-01", "Europe/Moscow", "05:00")
	if len(fields) != 0 || from == nil || to == nil || to.Sub(*from) != 24*time.Hour {
		t.Fatalf("unexpected range: from=%v to=%v fields=%v", from, to, fields)
	}
	loc, _ := time.LoadLocation("Europe/Moscow")
	if from.In(loc).Format("15:04") != "05:00" || to.In(loc).Format("15:04") != "05:00" {
		t.Fatalf("range must use configured day start: from=%v to=%v", from, to)
	}
}

func TestValidateEventActivityIntensityAndComment(t *testing.T) {
	badIntensity := eventDTO{
		Kind: "activity", OccurredAt: time.Now(), TimePrecision: "exact",
		Data: json.RawMessage(`{"intensity":"hard","duration_minutes":20}`),
	}
	fields := validateEvent(badIntensity)
	if fields["data.intensity"] == "" {
		t.Fatalf("expected intensity validation: %#v", fields)
	}
	ok := eventDTO{
		Kind: "activity", OccurredAt: time.Now(), TimePrecision: "exact",
		Data: json.RawMessage(`{"activity_type":"бег","duration_minutes":40,"intensity":"moderate","comment":"после обеда"}`),
	}
	if got := validateEvent(ok); len(got) != 0 {
		t.Fatalf("unexpected fields: %#v", got)
	}
	longComment := strings.Repeat("а", 1001)
	comment := eventDTO{
		Kind: "note", OccurredAt: time.Now(), TimePrecision: "exact",
		Data: json.RawMessage(`{"comment":"` + longComment + `"}`),
	}
	if fields := validateEvent(comment); fields["data.comment"] == "" {
		t.Fatal("expected comment length validation")
	}
}
