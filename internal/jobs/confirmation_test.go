package jobs

import (
	"strings"
	"testing"

	"health-diary/internal/llm"
)

func TestConfirmationTextShowsExtractedFacts(t *testing.T) {
	text := confirmationText([]llm.Event{
		{Kind: "pain_observation", OccurredAt: "2026-07-21T15:00:00Z", TimePrecision: "exact", Data: map[string]any{"symptom_type": "headache", "intensity": float64(6)}},
		{Kind: "medication_intake", OccurredAt: "2026-07-21T15:00:00Z", TimePrecision: "exact", Data: map[string]any{"name_raw": "ибупрофен", "dose_value": float64(400), "dose_unit": "мг"}},
	})
	for _, expected := range []string{"Головная боль — 6/10, в 15:00", "Приём лекарства — ибупрофен, 400 мг, в 15:00", "Подтвердите весь список"} {
		if !strings.Contains(text, expected) {
			t.Fatalf("confirmation text missing %q: %s", expected, text)
		}
	}
}
