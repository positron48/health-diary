package llm

import (
	"context"
	"strings"
	"testing"
	"time"
)

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

func TestValidateResultRejectsInvalidPainAndDose(t *testing.T) {
	badIntensity := Result{Summary: "x", Events: []Event{{
		ClientRef: "e1", Kind: "pain_observation", OccurredAt: "2026-07-21T12:00:00Z", TimePrecision: "exact",
		Data: map[string]any{"symptom_type": "headache", "phase": "start", "intensity": float64(11)},
	}}}
	if err := ValidateResult(badIntensity); err == nil {
		t.Fatal("expected intensity rejection")
	}
	badPhase := Result{Summary: "x", Events: []Event{{
		ClientRef: "e1", Kind: "pain_observation", OccurredAt: "2026-07-21T12:00:00Z", TimePrecision: "exact",
		Data: map[string]any{"phase": "middle"},
	}}}
	if err := ValidateResult(badPhase); err == nil {
		t.Fatal("expected phase rejection")
	}
	badDose := Result{Summary: "x", Events: []Event{{
		ClientRef: "e1", Kind: "medication_intake", OccurredAt: "2026-07-21T12:00:00Z", TimePrecision: "exact",
		Data: map[string]any{"name_raw": "цитрамон", "dose_value": float64(-1)},
	}}}
	if err := ValidateResult(badDose); err == nil {
		t.Fatal("expected dose rejection")
	}
}

func TestFakeJuly20HeadacheNarrative(t *testing.T) {
	text := `20 июля в обед начала слегка болеть голова в верхней части
к 16 часам начала болеть сильнее, выпил 1 цитрамон
стало чуть лучше, но до конца не прошло
вечером после часовой прогулки боль вернулась, уже в районе затылка-шеи
после 12 ночи выпил еще 1 цитрамон, боль успокоилась, уснул ближе к часу, на утро не болела`
	result, err := Fake{}.Extract(context.Background(), text)
	if err != nil {
		t.Fatal(err)
	}
	var painStarts, painUpdates, painEnds, meds, sleeps, morningPain int
	for _, event := range result.Events {
		switch event.Kind {
		case "pain_observation":
			phase, _ := event.Data["phase"].(string)
			switch phase {
			case "start":
				painStarts++
			case "update":
				painUpdates++
			case "end":
				painEnds++
			}
			if strings.Contains(event.OccurredAt, "T08:") {
				morningPain++
			}
			if _, ok := event.Data["intensity"]; ok && event.Data["intensity"] != nil {
				t.Fatalf("intensity must remain null when not stated: %#v", event.Data["intensity"])
			}
		case "medication_intake":
			meds++
			if event.Data["name_raw"] != "цитрамон" {
				t.Fatalf("medication name = %#v", event.Data["name_raw"])
			}
		case "sleep":
			sleeps++
		}
	}
	if painStarts != 1 || painUpdates < 1 || painEnds != 1 || meds != 2 || sleeps != 1 {
		t.Fatalf("unexpected counts start=%d update=%d end=%d meds=%d sleep=%d events=%d", painStarts, painUpdates, painEnds, meds, sleeps, len(result.Events))
	}
	if morningPain != 0 {
		t.Fatal("must not invent morning pain from negation")
	}
}

func TestBuildUserPromptIncludesTimezone(t *testing.T) {
	now := time.Date(2026, 7, 22, 16, 30, 0, 0, time.UTC)
	prompt := BuildUserPrompt("болит голова", now, "Europe/Moscow")
	for _, expected := range []string{"Europe/Moscow", "2026-07-22", "болит голова", "Current UTC"} {
		if !strings.Contains(prompt, expected) {
			t.Fatalf("prompt missing %q: %s", expected, prompt)
		}
	}
}
