package llm

import "testing"

func TestValidateLifeContextAndMotivation(t *testing.T) {
	valid := Result{
		Summary: "поездка",
		Events: []Event{{
			ClientRef: "e1", Kind: "life_context", OccurredAt: "2026-07-21T12:00:00Z", TimePrecision: "date_only",
			Data: map[string]any{"period_type": "trip", "place_label": "Новосибирск", "phase": "start"},
		}, {
			ClientRef: "e2", Kind: "wellbeing", OccurredAt: "2026-07-21T18:00:00Z", TimePrecision: "approximate",
			Data: map[string]any{"motivation_score": 4, "energy_score": 5},
		}},
	}
	if err := ValidateResult(valid); err != nil {
		t.Fatalf("valid context/motivation rejected: %v", err)
	}
	invalid := valid
	invalid.Events = []Event{{
		ClientRef: "e1", Kind: "life_context", OccurredAt: "2026-07-21T12:00:00Z", TimePrecision: "date_only",
		Data: map[string]any{"period_type": "holiday"},
	}}
	if err := ValidateResult(invalid); err == nil {
		t.Fatal("invalid period_type must be rejected")
	}
}
