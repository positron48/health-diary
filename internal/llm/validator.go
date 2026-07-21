package llm

import (
	"fmt"
	"strings"
	"time"
)

var allowedKinds = map[string]bool{"pain_observation": true, "medication_intake": true, "wellbeing": true, "activity": true, "sleep": true, "food_drink": true, "measurement": true, "note": true}
var allowedPrecisions = map[string]bool{"exact": true, "approximate": true, "date_only": true, "inferred_from_message": true}

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
	}
	return nil
}
