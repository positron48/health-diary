package analytics

import (
	"encoding/json"
	"time"
)

const FormulaVersion = "health-diary-summary-v1"

type Summary struct {
	FormulaVersion             string         `json:"formula_version"`
	From                       string         `json:"from"`
	To                         string         `json:"to"`
	Timezone                   string         `json:"timezone"`
	ObservationDays            int            `json:"observation_days"`
	DiaryDays                  int            `json:"diary_days"`
	ConfirmedEvents            int            `json:"confirmed_events"`
	Counts                     map[string]int `json:"counts"`
	HeadacheDays               int            `json:"headache_days"`
	MedicationDays             int            `json:"medication_days"`
	UnknownMedicationDoseCount int            `json:"unknown_medication_dose_count"`
}

// BuildSummary has no provider dependency and operates only on already
// confirmed, non-deleted events supplied by Repository.Events.
func BuildSummary(events []Event, from, to time.Time, timezone string) Summary {
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		loc = time.UTC
		timezone = "UTC"
	}
	daySet, headacheSet, medicationSet := map[string]bool{}, map[string]bool{}, map[string]bool{}
	counts := map[string]int{}
	unknownDose := 0
	for _, event := range events {
		day := event.OccurredAt.In(loc).Format("2006-01-02")
		daySet[day] = true
		counts[event.Kind]++
		if event.Kind == "pain_observation" {
			headacheSet[day] = true
		}
		if event.Kind == "medication_intake" {
			medicationSet[day] = true
			var data map[string]any
			if json.Unmarshal(event.Attributes, &data) != nil || data["dose_value"] == nil {
				unknownDose++
			}
		}
	}
	days := int(to.Sub(from).Hours() / 24)
	return Summary{FormulaVersion: FormulaVersion, From: from.In(loc).Format("2006-01-02"), To: to.In(loc).Format("2006-01-02"), Timezone: timezone, ObservationDays: days, DiaryDays: len(daySet), ConfirmedEvents: len(events), Counts: counts, HeadacheDays: len(headacheSet), MedicationDays: len(medicationSet), UnknownMedicationDoseCount: unknownDose}
}
