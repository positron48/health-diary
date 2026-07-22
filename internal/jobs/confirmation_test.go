package jobs

import (
	"strings"
	"testing"

	"health-diary/internal/llm"
)

func TestConfirmationTextShowsExtractedFacts(t *testing.T) {
	text := confirmationText([]llm.Event{
		{Kind: "pain_observation", OccurredAt: "2026-07-21T15:00:00Z", TimePrecision: "exact", Data: map[string]any{"symptom_type": "headache", "phase": "start", "intensity": float64(6), "laterality": "right"}},
		{Kind: "medication_intake", OccurredAt: "2026-07-21T15:00:00Z", TimePrecision: "exact", Data: map[string]any{"name_raw": "ибупрофен", "dose_value": float64(400), "dose_unit": "мг"}},
	})
	for _, expected := range []string{"Головная боль — началась, 6/10, справа, в 15:00", "Приём лекарства — ибупрофен, 400 мг, в 15:00", "Подтвердите весь список"} {
		if !strings.Contains(text, expected) {
			t.Fatalf("confirmation text missing %q: %s", expected, text)
		}
	}
}

func TestConfirmationTextLocalizesLocationsWithoutInventedIntensity(t *testing.T) {
	text := confirmationText([]llm.Event{
		{Kind: "pain_observation", OccurredAt: "2026-07-20T12:00:00Z", TimePrecision: "approximate", Data: map[string]any{"symptom_type": "headache", "phase": "start", "locations": []any{"top_of_head"}}},
		{Kind: "medication_intake", OccurredAt: "2026-07-20T16:00:00Z", TimePrecision: "approximate", Data: map[string]any{"name_raw": "цитрамон"}},
	})
	for _, expected := range []string{"Головная боль — началась, верхняя часть головы, около 12:00", "Приём лекарства — цитрамон, около 16:00"} {
		if !strings.Contains(text, expected) {
			t.Fatalf("confirmation text missing %q: %s", expected, text)
		}
	}
	if strings.Contains(text, "/10") {
		t.Fatalf("must not invent intensity: %s", text)
	}
}
