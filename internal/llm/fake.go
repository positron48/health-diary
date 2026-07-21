package llm

import (
	"context"
	"strings"
	"time"
)

type Fake struct{}

func (Fake) Extract(_ context.Context, text string) (Result, error) {
	lower := strings.ToLower(text)
	now := time.Now().UTC().Format(time.RFC3339)
	events := []Event{}
	if strings.Contains(lower, "голов") {
		events = append(events, Event{ClientRef: "e1", Kind: "pain_observation", OccurredAt: now, TimePrecision: "inferred_from_message", Data: map[string]any{"symptom_type": "headache"}})
	}
	if strings.Contains(lower, "ибупрофен") {
		events = append(events, Event{ClientRef: "e2", Kind: "medication_intake", OccurredAt: now, TimePrecision: "inferred_from_message", Data: map[string]any{"name_raw": "ибупрофен", "normalized_name": "ibuprofen"}})
	}
	if len(events) == 0 {
		events = append(events, Event{ClientRef: "e1", Kind: "note", OccurredAt: now, TimePrecision: "inferred_from_message", Data: map[string]any{}})
	}
	return Result{Summary: "Черновик записи", Events: events}, nil
}
