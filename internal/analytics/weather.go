package analytics

import (
	"encoding/json"
	"math"
	"sort"
	"time"

	"health-diary/internal/userday"
)

const WeatherFormulaVersion = "health-diary-weather-v1"

type WeatherDay struct {
	Date               string   `json:"date"`
	TempMeanC          *float64 `json:"temp_mean_c,omitempty"`
	PressureMeanHPa    *float64 `json:"pressure_mean_hpa,omitempty"`
	PressureDelta24HPa *float64 `json:"pressure_delta_24h_hpa,omitempty"`
	HumidityMeanPct    *float64 `json:"humidity_mean_pct,omitempty"`
	PrecipitationMM    *float64 `json:"precipitation_mm,omitempty"`
	IsComplete         bool     `json:"is_complete"`
	PainStarts         int      `json:"pain_starts"`
}

type WeatherHour struct {
	PlaceID        string
	ObservedAt     time.Time
	PressureMSLHPa *float64
}

type PainWeatherWindow struct {
	OccurredAt       time.Time `json:"occurred_at"`
	PressureHPa      *float64  `json:"pressure_hpa,omitempty"`
	PressureDelta6H  *float64  `json:"pressure_delta_6h_hpa,omitempty"`
	PressureDelta24H *float64  `json:"pressure_delta_24h_hpa,omitempty"`
	Intensity        *float64  `json:"intensity,omitempty"`
}

type WeatherFactor struct {
	Key                string         `json:"key"`
	Label              string         `json:"label"`
	Status             string         `json:"status"`
	ExposedDays        int            `json:"exposed_days"`
	UnexposedDays      int            `json:"unexposed_days"`
	ExposedStartDays   int            `json:"exposed_start_days"`
	UnexposedStartDays int            `json:"unexposed_start_days"`
	ExposedRate        *float64       `json:"exposed_rate,omitempty"`
	UnexposedRate      *float64       `json:"unexposed_rate,omitempty"`
	RiskRatio          *float64       `json:"risk_ratio,omitempty"`
	Description        string         `json:"description"`
	Requirements       map[string]any `json:"requirements"`
}

type WeatherAnalyticsResult struct {
	Status         string              `json:"status"`
	Coverage       map[string]int      `json:"coverage"`
	Series         []WeatherDay        `json:"series"`
	PainWindows    []PainWeatherWindow `json:"pain_windows"`
	Factors        []WeatherFactor     `json:"factors"`
	FormulaVersion string              `json:"formula_version"`
	Limitation     string              `json:"limitation"`
}

// BuildWeatherAnalytics compares registered daily pressure factors with
// confirmed pain starts. It always returns descriptive counts; association
// wording is unlocked only after the fixed gates pass.
func BuildWeatherAnalytics(events []Event, days []WeatherDay, hours []WeatherHour, timezone, dayStart string) WeatherAnalyticsResult {
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		loc = time.UTC
	}
	boundary := userday.Start(dayStart)
	startDays := map[string]int{}
	starts := make([]Event, 0)
	for _, event := range events {
		if event.Kind != "pain_observation" {
			continue
		}
		var data map[string]any
		_ = json.Unmarshal(event.Attributes, &data)
		phase, _ := data["phase"].(string)
		if phase != "" && phase != "start" {
			continue
		}
		startDays[userday.Date(event.OccurredAt, loc, boundary)]++
		starts = append(starts, event)
	}
	completeDays, pressureDays := 0, 0
	series := make([]WeatherDay, len(days))
	copy(series, days)
	for i := range series {
		series[i].PainStarts = startDays[series[i].Date]
		if series[i].IsComplete {
			completeDays++
			if series[i].PressureDelta24HPa != nil {
				pressureDays++
			}
		}
	}
	sort.Slice(hours, func(i, j int) bool { return hours[i].ObservedAt.Before(hours[j].ObservedAt) })
	windows := make([]PainWeatherWindow, 0, len(starts))
	for _, event := range starts {
		window := PainWeatherWindow{OccurredAt: event.OccurredAt}
		var data map[string]any
		_ = json.Unmarshal(event.Attributes, &data)
		if intensity, ok := data["intensity"].(float64); ok {
			window.Intensity = &intensity
		}
		current, placeID := pressureAt(hours, event.OccurredAt, "")
		window.PressureHPa = current
		if current != nil {
			if previous, _ := pressureAt(hours, event.OccurredAt.Add(-6*time.Hour), placeID); previous != nil {
				value := *current - *previous
				window.PressureDelta6H = &value
			}
			if previous, _ := pressureAt(hours, event.OccurredAt.Add(-24*time.Hour), placeID); previous != nil {
				value := *current - *previous
				window.PressureDelta24H = &value
			}
		}
		windows = append(windows, window)
	}
	rules := []struct {
		key, label string
		match      func(float64) bool
	}{
		{"pressure_drop", "Падение давления на 5 гПа и больше", func(v float64) bool { return v <= -5 }},
		{"pressure_rise", "Рост давления на 5 гПа и больше", func(v float64) bool { return v >= 5 }},
		{"pressure_change", "Перепад давления на 5 гПа и больше", func(v float64) bool { return math.Abs(v) >= 5 }},
	}
	factors := make([]WeatherFactor, 0, len(rules))
	anyReady := false
	for _, rule := range rules {
		factor := evaluateWeatherFactor(rule.key, rule.label, rule.match, series, len(startDays))
		if factor.Status != "insufficient_data" {
			anyReady = true
		}
		factors = append(factors, factor)
	}
	status := "descriptive"
	if anyReady {
		status = "ok"
	}
	return WeatherAnalyticsResult{
		Status: status,
		Coverage: map[string]int{
			"weather_days": completeDays, "pressure_days": pressureDays, "headache_start_days": len(startDays), "headache_starts": len(starts),
		},
		Series: series, PainWindows: windows, Factors: factors,
		FormulaVersion: WeatherFormulaVersion,
		Limitation:     "Совпадение записанных эпизодов и погоды не доказывает причинную связь",
	}
}

func pressureAt(hours []WeatherHour, target time.Time, requiredPlace string) (*float64, string) {
	for i := len(hours) - 1; i >= 0; i-- {
		hour := hours[i]
		if hour.ObservedAt.After(target) || hour.PressureMSLHPa == nil || (requiredPlace != "" && hour.PlaceID != requiredPlace) {
			continue
		}
		if target.Sub(hour.ObservedAt) > 90*time.Minute {
			return nil, ""
		}
		value := *hour.PressureMSLHPa
		return &value, hour.PlaceID
	}
	return nil, ""
}

func evaluateWeatherFactor(key, label string, match func(float64) bool, days []WeatherDay, headacheStartDays int) WeatherFactor {
	factor := WeatherFactor{Key: key, Label: label, Status: "insufficient_data"}
	for _, day := range days {
		if !day.IsComplete || day.PressureDelta24HPa == nil {
			continue
		}
		if match(*day.PressureDelta24HPa) {
			factor.ExposedDays++
			if day.PainStarts > 0 {
				factor.ExposedStartDays++
			}
		} else {
			factor.UnexposedDays++
			if day.PainStarts > 0 {
				factor.UnexposedStartDays++
			}
		}
	}
	observationDays := factor.ExposedDays + factor.UnexposedDays
	factor.Requirements = map[string]any{
		"observation_days": map[string]int{"actual": observationDays, "required": 56},
		"headache_starts":  map[string]int{"actual": headacheStartDays, "required": 8},
		"exposed_days":     map[string]int{"actual": factor.ExposedDays, "required": 5},
		"unexposed_days":   map[string]int{"actual": factor.UnexposedDays, "required": 5},
	}
	if factor.ExposedDays > 0 {
		value := float64(factor.ExposedStartDays) / float64(factor.ExposedDays)
		factor.ExposedRate = &value
	}
	if factor.UnexposedDays > 0 {
		value := float64(factor.UnexposedStartDays) / float64(factor.UnexposedDays)
		factor.UnexposedRate = &value
	}
	if observationDays < 56 || headacheStartDays < 8 || factor.ExposedDays < 5 || factor.UnexposedDays < 5 {
		factor.Description = "Пока недостаточно дней и записанных начал боли для вывода"
		return factor
	}
	if factor.UnexposedRate == nil || *factor.UnexposedRate == 0 {
		factor.Description = "Недостаточно контрольных дней с записанным началом боли"
		return factor
	}
	ratio := *factor.ExposedRate / *factor.UnexposedRate
	factor.RiskRatio = &ratio
	if ratio > 1 {
		factor.Status = "possible_association"
		factor.Description = "В наблюдаемом периоде записанные начала боли встречались чаще; это не доказывает причину"
	} else {
		factor.Status = "no_increased_rate"
		factor.Description = "В наблюдаемом периоде учащения записанных начал боли не видно"
	}
	return factor
}
