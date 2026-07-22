package llm

import (
	"context"
	"strconv"
	"strings"
	"time"
)

type Fake struct{}

func (Fake) Extract(_ context.Context, text string) (Result, error) {
	lower := strings.ToLower(text)
	now := time.Now().UTC()
	events := []Event{}

	// Multi-day headache narrative used as the regression fixture for detailing.
	if strings.Contains(lower, "цитрамон") && strings.Contains(lower, "голов") {
		day := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
		if strings.Contains(lower, "20 июля") || strings.Contains(lower, "20.07") {
			day = time.Date(now.Year(), time.July, 20, 0, 0, 0, 0, time.UTC)
		}
		events = []Event{
			{ClientRef: "e1", Kind: "pain_observation", OccurredAt: day.Add(12 * time.Hour).Format(time.RFC3339), TimePrecision: "approximate", Data: map[string]any{"symptom_type": "headache", "phase": "start", "locations": []any{"top_of_head"}}},
			{ClientRef: "e2", Kind: "pain_observation", OccurredAt: day.Add(16 * time.Hour).Format(time.RFC3339), TimePrecision: "approximate", Data: map[string]any{"symptom_type": "headache", "phase": "update", "locations": []any{"top_of_head"}}},
			{ClientRef: "e3", Kind: "medication_intake", OccurredAt: day.Add(16 * time.Hour).Format(time.RFC3339), TimePrecision: "approximate", Data: map[string]any{"name_raw": "цитрамон"}},
			{ClientRef: "e4", Kind: "pain_observation", OccurredAt: day.Add(19 * time.Hour).Format(time.RFC3339), TimePrecision: "approximate", Data: map[string]any{"symptom_type": "headache", "phase": "update", "locations": []any{"occiput_neck"}}},
			{ClientRef: "e5", Kind: "medication_intake", OccurredAt: day.Add(24*time.Hour + 30*time.Minute).Format(time.RFC3339), TimePrecision: "approximate", Data: map[string]any{"name_raw": "цитрамон"}},
			{ClientRef: "e6", Kind: "pain_observation", OccurredAt: day.Add(24*time.Hour + 30*time.Minute).Format(time.RFC3339), TimePrecision: "approximate", Data: map[string]any{"symptom_type": "headache", "phase": "end", "locations": []any{"occiput_neck"}}},
			{ClientRef: "e7", Kind: "sleep", OccurredAt: day.Add(25 * time.Hour).Format(time.RFC3339), TimePrecision: "approximate", Data: map[string]any{}},
		}
		result := Result{Summary: "Головная боль с двумя приёмами цитрамона", Events: events}
		return result, ValidateResult(result)
	}

	refN := 1
	add := func(kind string, data map[string]any) {
		events = append(events, Event{
			ClientRef: "e" + strconv.Itoa(refN), Kind: kind, OccurredAt: now.Format(time.RFC3339),
			TimePrecision: "inferred_from_message", Data: data,
		})
		refN++
	}
	if strings.Contains(lower, "голов") {
		add("pain_observation", map[string]any{"symptom_type": "headache", "phase": "start"})
	}
	if strings.Contains(lower, "ибупрофен") {
		add("medication_intake", map[string]any{"name_raw": "ибупрофен", "normalized_name": "ibuprofen"})
	}
	if strings.Contains(lower, "цитрамон") {
		add("medication_intake", map[string]any{"name_raw": "цитрамон"})
	}
	if len(events) == 0 {
		add("note", map[string]any{})
	}
	result := Result{Summary: "Черновик записи", Events: events}
	return result, ValidateResult(result)
}
