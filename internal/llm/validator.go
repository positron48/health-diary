package llm

import (
	"fmt"
	"strings"
	"time"
)

var allowedKinds = map[string]bool{
	"pain_observation": true, "medication_intake": true, "wellbeing": true, "activity": true,
	"sleep": true, "food_drink": true, "measurement": true, "note": true,
}
var allowedPrecisions = map[string]bool{
	"exact": true, "approximate": true, "date_only": true, "inferred_from_message": true,
}
var allowedPhases = map[string]bool{"start": true, "update": true, "end": true}
var allowedLaterality = map[string]bool{
	"left": true, "right": true, "bilateral": true, "center": true, "unknown": true,
}

// ValidateResult is the application boundary for provider output. It rejects
// the whole response: partial extraction would silently turn malformed output
// into an apparently trustworthy health record.
func ValidateResult(result Result) error {
	if len(result.Events) == 0 || len(result.Events) > 12 {
		return fmt.Errorf("event count must be between 1 and 12")
	}
	if len([]rune(result.Summary)) > 500 {
		return fmt.Errorf("summary is too long")
	}
	refs := map[string]bool{}
	for index, event := range result.Events {
		if strings.TrimSpace(event.ClientRef) == "" || len(event.ClientRef) > 64 || refs[event.ClientRef] {
			return fmt.Errorf("event %d has invalid client_ref", index)
		}
		refs[event.ClientRef] = true
		if !allowedKinds[event.Kind] {
			return fmt.Errorf("event %d has unsupported kind", index)
		}
		if !allowedPrecisions[event.TimePrecision] {
			return fmt.Errorf("event %d has unsupported time_precision", index)
		}
		if _, err := time.Parse(time.RFC3339, event.OccurredAt); err != nil {
			return fmt.Errorf("event %d has invalid occurred_at", index)
		}
		if event.Data == nil {
			return fmt.Errorf("event %d has no data object", index)
		}
		if len(event.Data) > 40 {
			return fmt.Errorf("event %d has too many fields", index)
		}
		if err := validateEventData(event.Kind, event.Data); err != nil {
			return fmt.Errorf("event %d: %w", index, err)
		}
	}
	return nil
}

// NormalizeTimes converts provider timestamps to UTC. A trailing Z is treated
// as a provider mistake containing user-local wall-clock digits; the prompt
// requires an explicit numeric offset for correctly resolved local input.
func NormalizeTimes(result *Result, timezone string, reference time.Time) error {
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		return fmt.Errorf("invalid user timezone")
	}
	for index := range result.Events {
		raw := result.Events[index].OccurredAt
		parsed, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			return fmt.Errorf("event %d has invalid occurred_at", index)
		}
		if strings.HasSuffix(strings.ToUpper(raw), "Z") && loc != time.UTC {
			parsed = time.Date(parsed.Year(), parsed.Month(), parsed.Day(), parsed.Hour(), parsed.Minute(), parsed.Second(), parsed.Nanosecond(), loc)
		}
		if !reference.IsZero() {
			delta := parsed.Sub(reference)
			if delta < -5*365*24*time.Hour || delta > 30*24*time.Hour {
				return fmt.Errorf("event %d occurred_at is outside the allowed window", index)
			}
		}
		result.Events[index].OccurredAt = parsed.UTC().Format(time.RFC3339)
	}
	return nil
}

func validateEventData(kind string, data map[string]any) error {
	switch kind {
	case "pain_observation":
		if phase, ok := data["phase"]; ok && phase != nil {
			text, ok := phase.(string)
			if !ok || !allowedPhases[text] {
				return fmt.Errorf("phase must be start, update or end")
			}
		}
		if symptom, ok := data["symptom_type"]; ok && symptom != nil {
			text, ok := symptom.(string)
			if !ok || strings.TrimSpace(text) == "" {
				return fmt.Errorf("symptom_type must be a non-empty string")
			}
		}
		if err := checkOptionalIntRange(data, "intensity", 0, 10); err != nil {
			return err
		}
		if err := checkOptionalIntRange(data, "functional_impact", 0, 3); err != nil {
			return err
		}
		if laterality, ok := data["laterality"]; ok && laterality != nil {
			text, ok := laterality.(string)
			if !ok || !allowedLaterality[text] {
				return fmt.Errorf("laterality is unsupported")
			}
		}
		for _, key := range []string{"locations", "qualities", "associated_symptoms"} {
			if err := checkOptionalStringArray(data, key); err != nil {
				return err
			}
		}
	case "medication_intake":
		if name, ok := data["name_raw"]; ok && name != nil {
			text, ok := name.(string)
			if !ok || len([]rune(text)) > 120 {
				return fmt.Errorf("name_raw must be a short string")
			}
		}
		if value, ok := data["dose_value"]; ok && value != nil {
			number, ok := asFloat(value)
			if !ok || number <= 0 {
				return fmt.Errorf("dose_value must be positive")
			}
		}
		if err := checkOptionalIntRange(data, "effect_rating", -2, 2); err != nil {
			return err
		}
	}
	return nil
}

func checkOptionalIntRange(data map[string]any, key string, min, max float64) error {
	value, ok := data[key]
	if !ok || value == nil {
		return nil
	}
	number, ok := asFloat(value)
	if !ok || number < min || number > max || number != float64(int(number)) {
		return fmt.Errorf("%s must be an integer between %v and %v", key, min, max)
	}
	return nil
}

func checkOptionalStringArray(data map[string]any, key string) error {
	value, ok := data[key]
	if !ok || value == nil {
		return nil
	}
	items, ok := value.([]any)
	if !ok {
		if _, isStrings := value.([]string); isStrings {
			return nil
		}
		return fmt.Errorf("%s must be an array of strings", key)
	}
	for _, item := range items {
		if _, ok := item.(string); !ok {
			return fmt.Errorf("%s must be an array of strings", key)
		}
	}
	return nil
}

func asFloat(value any) (float64, bool) {
	switch typed := value.(type) {
	case float64:
		return typed, true
	case int:
		return float64(typed), true
	case int64:
		return float64(typed), true
	default:
		return 0, false
	}
}
