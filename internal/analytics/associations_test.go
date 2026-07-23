package analytics

import (
	"encoding/json"
	"testing"
	"time"
)

func TestBuildAssociationsRequiresGates(t *testing.T) {
	from := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	to := from.AddDate(0, 0, 30)
	result := BuildAssociations(nil, nil, from, to, "UTC", "00:00", true)
	if result.Status != "insufficient_data" {
		t.Fatalf("expected insufficient_data, got %s", result.Status)
	}
}

func TestBuildAssociationsEmitsTravelCard(t *testing.T) {
	from := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	to := from.AddDate(0, 0, 60)
	events := []Event{}
	exposures := map[string]DayExposure{}
	for i := 0; i < 60; i++ {
		day := from.AddDate(0, 0, i)
		date := day.Format("2006-01-02")
		exp := DayExposure{}
		if i%3 == 0 {
			exp.TravelDay = true
			attrs, _ := json.Marshal(map[string]any{"phase": "start"})
			events = append(events, Event{Kind: "pain_observation", OccurredAt: day.Add(12 * time.Hour), Attributes: attrs})
		}
		if i%2 == 0 {
			exp.ShortSleep = true
		}
		exposures[date] = exp
	}
	result := BuildAssociations(events, exposures, from, to, "UTC", "00:00", false)
	if result.Status != "ok" && result.Status != "insufficient_data" {
		t.Fatalf("unexpected status: %s", result.Status)
	}
	for _, card := range result.Associations {
		if card.Status == "possible_association" && card.RiskRatio == nil {
			t.Fatalf("possible association must include risk ratio: %#v", card)
		}
	}
}
