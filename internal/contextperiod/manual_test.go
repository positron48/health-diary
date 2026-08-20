package contextperiod

import "testing"

func TestValidateManualInput(t *testing.T) {
	endBefore := "2026-08-19"
	fields := ValidateManualInput(ManualInput{
		PeriodType: "temporary_stay",
		PlaceID:    "place-1",
		StartedOn:  "2026-08-20",
		EndedOn:    &endBefore,
	})
	if fields["ended_on"] == "" {
		t.Fatalf("end before start must be rejected: %#v", fields)
	}

	end := "2026-08-25"
	fields = ValidateManualInput(ManualInput{
		PeriodType: "relocation",
		PlaceID:    "place-1",
		StartedOn:  "2026-08-20",
		EndedOn:    &end,
	})
	if len(fields) != 0 {
		t.Fatalf("valid period rejected: %#v", fields)
	}
}

func TestValidateManualInputRequiresKnownTypePlaceAndDates(t *testing.T) {
	fields := ValidateManualInput(ManualInput{PeriodType: "holiday", StartedOn: "20.08.2026"})
	for _, key := range []string{"period_type", "place_id", "started_on"} {
		if fields[key] == "" {
			t.Fatalf("%s must be rejected: %#v", key, fields)
		}
	}
}
