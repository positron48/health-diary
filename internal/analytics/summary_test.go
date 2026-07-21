package analytics

import (
	"testing"
	"time"
)

func TestBuildSummaryUsesLocalDaysAndOnlyProvidedEvents(t *testing.T) {
	from := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	to := from.AddDate(0, 0, 2)
	events := []Event{{Kind: "pain_observation", OccurredAt: time.Date(2026, 7, 1, 21, 30, 0, 0, time.UTC), Attributes: []byte(`{}`)}, {Kind: "medication_intake", OccurredAt: time.Date(2026, 7, 1, 22, 0, 0, 0, time.UTC), Attributes: []byte(`{}`)}}
	summary := BuildSummary(events, from, to, "Europe/Moscow")
	if summary.DiaryDays != 1 || summary.HeadacheDays != 1 || summary.MedicationDays != 1 || summary.UnknownMedicationDoseCount != 1 {
		t.Fatalf("unexpected summary: %#v", summary)
	}
}
