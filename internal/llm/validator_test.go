package llm

import "testing"

func TestValidateResultRejectsMalformedProviderOutput(t *testing.T) {
	valid := Result{Summary: "x", Events: []Event{{ClientRef: "e1", Kind: "note", OccurredAt: "2026-07-21T12:00:00Z", TimePrecision: "exact", Data: map[string]any{}}}}
	if err := ValidateResult(valid); err != nil {
		t.Fatalf("valid result: %v", err)
	}
	invalid := valid
	invalid.Events[0].Kind = "diagnosis"
	if err := ValidateResult(invalid); err == nil {
		t.Fatal("expected invalid kind rejection")
	}
	invalid = valid
	invalid.Events[0].OccurredAt = "tomorrow"
	if err := ValidateResult(invalid); err == nil {
		t.Fatal("expected invalid time rejection")
	}
}
