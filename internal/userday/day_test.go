package userday

import (
	"testing"
	"time"
)

func TestDateMovesEarlyMorningToPreviousUserDay(t *testing.T) {
	loc, err := time.LoadLocation("Europe/Moscow")
	if err != nil {
		t.Fatal(err)
	}
	start := Start("05:00")
	early := time.Date(2026, 7, 23, 1, 0, 0, 0, loc)
	later := time.Date(2026, 7, 23, 6, 0, 0, 0, loc)
	if Date(early, loc, start) != "2026-07-22" || Date(later, loc, start) != "2026-07-23" {
		t.Fatalf("unexpected dates: early=%s later=%s", Date(early, loc, start), Date(later, loc, start))
	}
}

func TestBoundsBeginAtConfiguredLocalTime(t *testing.T) {
	loc, err := time.LoadLocation("Europe/Moscow")
	if err != nil {
		t.Fatal(err)
	}
	from, to, err := Bounds("2026-07-22", loc, Start("05:00"))
	if err != nil {
		t.Fatal(err)
	}
	if from.In(loc).Format("2006-01-02 15:04") != "2026-07-22 05:00" ||
		to.In(loc).Format("2006-01-02 15:04") != "2026-07-23 05:00" {
		t.Fatalf("unexpected bounds: %v - %v", from, to)
	}
}

func TestParseStartRejectsInvalidValues(t *testing.T) {
	for _, value := range []string{"5:00", "24:00", "05:60", "noon"} {
		if _, err := ParseStart(value); err == nil {
			t.Fatalf("expected %q to be rejected", value)
		}
	}
}
