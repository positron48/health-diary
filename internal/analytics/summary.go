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
	MedicationIntakes          int            `json:"medication_intakes"`
	RecordedMedicationEffects  int            `json:"recorded_medication_effects"`
	PainIntensityKnown         int            `json:"pain_intensity_known"`
	PainIntensityAverage       *float64       `json:"pain_intensity_average"`
	PainIntensityMaximum       *float64       `json:"pain_intensity_maximum"`
	ActivityMinutes            int            `json:"activity_minutes"`
	ActivityRecords            int            `json:"activity_records"`
	SleepMinutes               int            `json:"sleep_minutes"`
	SleepRecords               int            `json:"sleep_records"`
	SleepQualityKnown          int            `json:"sleep_quality_known"`
	WellbeingRecords           int            `json:"wellbeing_records"`
	WellbeingScoreKnown        int            `json:"wellbeing_score_known"`
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
	medicationIntakes, recordedEffects := 0, 0
	intensityN, intensitySum := 0, 0.0
	var intensityMax *float64
	activityMinutes, activityRecords := 0, 0
	sleepMinutes, sleepRecords, sleepQualityKnown := 0, 0, 0
	wellbeingRecords, wellbeingScoreKnown := 0, 0
	for _, event := range events {
		day := event.OccurredAt.In(loc).Format("2006-01-02")
		daySet[day] = true
		counts[event.Kind]++
		if event.Kind == "pain_observation" {
			headacheSet[day] = true
		}
		if event.Kind == "medication_intake" {
			medicationIntakes++
			medicationSet[day] = true
			var data map[string]any
			if json.Unmarshal(event.Attributes, &data) != nil || data["dose_value"] == nil {
				unknownDose++
			}
			if data["effect_rating"] != nil {
				recordedEffects++
			}
		}
		var data map[string]any
		_ = json.Unmarshal(event.Attributes, &data)
		switch event.Kind {
		case "pain_observation":
			if value, ok := data["intensity"].(float64); ok {
				intensityN++
				intensitySum += value
				if intensityMax == nil || value > *intensityMax {
					copy := value
					intensityMax = &copy
				}
			}
		case "activity":
			activityRecords++
			if value, ok := data["duration_minutes"].(float64); ok {
				activityMinutes += int(value)
			}
		case "sleep":
			sleepRecords++
			if value, ok := data["duration_minutes"].(float64); ok {
				sleepMinutes += int(value)
			}
			if data["quality_score"] != nil {
				sleepQualityKnown++
			}
		case "wellbeing":
			wellbeingRecords++
			if data["wellbeing_score"] != nil {
				wellbeingScoreKnown++
			}
		}
	}
	days := int(to.Sub(from).Hours() / 24)
	var intensityAverage *float64
	if intensityN > 0 {
		value := intensitySum / float64(intensityN)
		intensityAverage = &value
	}
	return Summary{FormulaVersion: FormulaVersion, From: from.In(loc).Format("2006-01-02"), To: to.In(loc).Format("2006-01-02"), Timezone: timezone, ObservationDays: days, DiaryDays: len(daySet), ConfirmedEvents: len(events), Counts: counts, HeadacheDays: len(headacheSet), MedicationDays: len(medicationSet), UnknownMedicationDoseCount: unknownDose, MedicationIntakes: medicationIntakes, RecordedMedicationEffects: recordedEffects, PainIntensityKnown: intensityN, PainIntensityAverage: intensityAverage, PainIntensityMaximum: intensityMax, ActivityMinutes: activityMinutes, ActivityRecords: activityRecords, SleepMinutes: sleepMinutes, SleepRecords: sleepRecords, SleepQualityKnown: sleepQualityKnown, WellbeingRecords: wellbeingRecords, WellbeingScoreKnown: wellbeingScoreKnown}
}
