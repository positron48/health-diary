package analytics

import (
	"encoding/json"
	"testing"
	"time"
)

func TestBuildWeatherAnalyticsReturnsDescriptiveCountsBeforeGates(t *testing.T) {
	from := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	delta := -7.0
	days := []WeatherDay{{Date: "2026-01-01", PressureDelta24HPa: &delta, IsComplete: true}}
	attrs, _ := json.Marshal(map[string]any{"phase": "start", "intensity": 6})
	events := []Event{{Kind: "pain_observation", OccurredAt: from, Attributes: attrs}}
	pressureNow, pressure6, pressure24 := 1000.0, 1004.0, 1008.0
	hours := []WeatherHour{
		{PlaceID: "place", ObservedAt: from.Add(-24 * time.Hour), PressureMSLHPa: &pressure24},
		{PlaceID: "place", ObservedAt: from.Add(-6 * time.Hour), PressureMSLHPa: &pressure6},
		{PlaceID: "place", ObservedAt: from, PressureMSLHPa: &pressureNow},
	}
	result := BuildWeatherAnalytics(events, days, hours, "UTC", "00:00")
	if result.Status != "descriptive" || result.Coverage["headache_starts"] != 1 {
		t.Fatalf("unexpected result: %#v", result)
	}
	if len(result.PainWindows) != 1 || result.PainWindows[0].PressureDelta6H == nil || *result.PainWindows[0].PressureDelta6H != -4 {
		t.Fatalf("unexpected pain window: %#v", result.PainWindows)
	}
	if result.Factors[0].ExposedStartDays != 1 || result.Factors[0].Status != "insufficient_data" {
		t.Fatalf("unexpected factor: %#v", result.Factors[0])
	}
}

func TestBuildWeatherAnalyticsIgnoresPainUpdates(t *testing.T) {
	attrs, _ := json.Marshal(map[string]any{"phase": "update"})
	result := BuildWeatherAnalytics([]Event{{Kind: "pain_observation", OccurredAt: time.Now(), Attributes: attrs}}, nil, nil, "UTC", "00:00")
	if result.Coverage["headache_starts"] != 0 {
		t.Fatalf("updates must not count as starts: %#v", result.Coverage)
	}
}

func TestBuildWeatherAnalyticsUnlocksVersionedFactorAfterGates(t *testing.T) {
	from := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	days := make([]WeatherDay, 0, 60)
	events := make([]Event, 0, 8)
	attrs, _ := json.Marshal(map[string]any{"phase": "start"})
	for index := 0; index < 60; index++ {
		date := from.AddDate(0, 0, index)
		delta := 1.0
		if index < 10 {
			delta = -6
		}
		days = append(days, WeatherDay{Date: date.Format("2006-01-02"), PressureDelta24HPa: &delta, IsComplete: true})
		if index < 6 || index == 20 || index == 40 {
			events = append(events, Event{Kind: "pain_observation", OccurredAt: date, Attributes: attrs})
		}
	}
	result := BuildWeatherAnalytics(events, days, nil, "UTC", "00:00")
	if result.Factors[0].Status != "possible_association" || result.Factors[0].RiskRatio == nil {
		t.Fatalf("expected gated pressure-drop association: %#v", result.Factors[0])
	}
	if len(result.Factors) != 3 {
		t.Fatalf("all registered factors must be returned: %#v", result.Factors)
	}
}
