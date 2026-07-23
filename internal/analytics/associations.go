package analytics

import (
	"encoding/json"
	"math"
	"time"

	"health-diary/internal/userday"
)

const AssociationsFormulaVersion = "health-diary-associations-v2"

type AssociationCard struct {
	Key             string   `json:"key"`
	Label           string   `json:"label"`
	Status          string   `json:"status"`
	ExposedN        int      `json:"exposed_n"`
	UnexposedN      int      `json:"unexposed_n"`
	ExposedStarts   int      `json:"exposed_starts"`
	UnexposedStarts int      `json:"unexposed_starts"`
	RiskRatio       *float64 `json:"risk_ratio,omitempty"`
	SampleSize      int      `json:"sample_size"`
	Description     string   `json:"description"`
}

type AssociationsResult struct {
	Status         string            `json:"status"`
	Requirements   map[string]any    `json:"requirements"`
	Associations   []AssociationCard `json:"associations"`
	FormulaVersion string            `json:"formula_version"`
	Limitation     string            `json:"limitation"`
}

type DayExposure struct {
	ShortSleep      bool
	HighStress      bool
	LowEnergy       bool
	LowMotivation   bool
	IntenseActivity bool
	TravelDay       bool
	PressureDrop    bool
}

// BuildAssociations computes gated possible associations. Weather cards are included
// only when weatherAssociations is true and weather exposures are supplied.
func BuildAssociations(events []Event, exposures map[string]DayExposure, from, to time.Time, timezone, dayStart string, weatherAssociations bool) AssociationsResult {
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		loc = time.UTC
	}
	boundary := userday.Start(dayStart)
	days := int(to.Sub(from).Hours() / 24)
	startDays := []string{}
	for _, event := range events {
		if event.Kind != "pain_observation" {
			continue
		}
		var data map[string]any
		_ = json.Unmarshal(event.Attributes, &data)
		if data["phase"] == "start" || data["phase"] == nil {
			startDays = append(startDays, userday.Date(event.OccurredAt, loc, boundary))
		}
	}
	startSet := map[string]bool{}
	for _, day := range startDays {
		startSet[day] = true
	}

	// Fill exposures from events when not already provided.
	if exposures == nil {
		exposures = map[string]DayExposure{}
	}
	for _, event := range events {
		day := userday.Date(event.OccurredAt, loc, boundary)
		exp := exposures[day]
		var data map[string]any
		_ = json.Unmarshal(event.Attributes, &data)
		switch event.Kind {
		case "sleep":
			if value, ok := data["duration_minutes"].(float64); ok && value < 360 {
				exp.ShortSleep = true
			}
		case "wellbeing":
			if value, ok := data["stress_score"].(float64); ok && value >= 7 {
				exp.HighStress = true
			}
			if value, ok := data["energy_score"].(float64); ok && value <= 3 {
				exp.LowEnergy = true
			}
			if value, ok := data["motivation_score"].(float64); ok && value <= 3 {
				exp.LowMotivation = true
			}
		case "activity":
			if data["intensity"] == "high" {
				exp.IntenseActivity = true
			}
			if value, ok := data["duration_minutes"].(float64); ok && value >= 90 {
				exp.IntenseActivity = true
			}
		case "life_context":
			exp.TravelDay = true
		}
		exposures[day] = exp
	}

	exposureDays := 0
	for _, exp := range exposures {
		if exp.ShortSleep || exp.HighStress || exp.LowEnergy || exp.LowMotivation || exp.IntenseActivity || exp.TravelDay || (weatherAssociations && exp.PressureDrop) {
			exposureDays++
		}
	}
	requirements := map[string]any{
		"observation_days": map[string]int{"actual": days, "required": 56},
		"headache_starts":  map[string]int{"actual": len(startSet), "required": 8},
		"exposure_days":    map[string]int{"actual": exposureDays, "required": 10},
	}
	result := AssociationsResult{
		Status:         "insufficient_data",
		Requirements:   requirements,
		Associations:   []AssociationCard{},
		FormulaVersion: AssociationsFormulaVersion,
		Limitation:     "Записи описывают возможные связи и не доказывают причинность",
	}
	if days < 56 || len(startSet) < 8 || exposureDays < 10 {
		return result
	}

	type rule struct {
		key, label string
		check      func(DayExposure) bool
	}
	rules := []rule{
		{"short_sleep", "Короткий сон", func(e DayExposure) bool { return e.ShortSleep }},
		{"high_stress", "Высокий стресс", func(e DayExposure) bool { return e.HighStress }},
		{"low_energy", "Низкая энергия", func(e DayExposure) bool { return e.LowEnergy }},
		{"low_motivation", "Низкая мотивация", func(e DayExposure) bool { return e.LowMotivation }},
		{"intense_activity", "Интенсивная активность", func(e DayExposure) bool { return e.IntenseActivity }},
		{"travel_day", "День поездки/отпуска", func(e DayExposure) bool { return e.TravelDay }},
	}
	if weatherAssociations {
		rules = append(rules, rule{"pressure_drop", "Падение давления", func(e DayExposure) bool { return e.PressureDrop }})
	}

	cards := []AssociationCard{}
	for _, rule := range rules {
		card := evaluateRule(rule.key, rule.label, rule.check, exposures, startSet, from, to, loc)
		if card.Status == "possible_association" {
			cards = append(cards, card)
		}
	}
	if len(cards) == 0 {
		return result
	}
	result.Status = "ok"
	result.Associations = cards
	return result
}

func evaluateRule(key, label string, check func(DayExposure) bool, exposures map[string]DayExposure, starts map[string]bool, from, to time.Time, loc *time.Location) AssociationCard {
	exposedN, unexposedN, exposedStarts, unexposedStarts := 0, 0, 0, 0
	for day := from.In(loc); day.Before(to.In(loc)); day = day.AddDate(0, 0, 1) {
		date := day.Format("2006-01-02")
		exp := exposures[date]
		exposed := check(exp)
		if exposed {
			exposedN++
			if starts[date] {
				exposedStarts++
			}
		} else {
			unexposedN++
			if starts[date] {
				unexposedStarts++
			}
		}
	}
	card := AssociationCard{
		Key: key, Label: label, ExposedN: exposedN, UnexposedN: unexposedN,
		ExposedStarts: exposedStarts, UnexposedStarts: unexposedStarts,
		SampleSize: exposedN + unexposedN,
	}
	if exposedN < 5 || unexposedN < 5 {
		card.Status = "insufficient_data"
		card.Description = "Недостаточно сопоставимых дней с воздействием и без него"
		return card
	}
	exposedRate := float64(exposedStarts) / float64(exposedN)
	unexposedRate := float64(unexposedStarts) / float64(unexposedN)
	if unexposedRate == 0 {
		card.Status = "insufficient_data"
		card.Description = "Нет контрольных дней с головной болью"
		return card
	}
	rr := exposedRate / unexposedRate
	if math.IsNaN(rr) || math.IsInf(rr, 0) {
		card.Status = "insufficient_data"
		return card
	}
	card.RiskRatio = &rr
	card.Status = "possible_association"
	card.Description = "Возможная связь: в дни с фактором головная боль начиналась чаще. Это наблюдение, не причина."
	return card
}
