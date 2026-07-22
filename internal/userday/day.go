package userday

import (
	"fmt"
	"regexp"
	"strconv"
	"time"
)

const DefaultStart = "00:00"

var startPattern = regexp.MustCompile(`^([01]\d|2[0-3]):([0-5]\d)$`)

func ParseStart(value string) (time.Duration, error) {
	if value == "" {
		value = DefaultStart
	}
	match := startPattern.FindStringSubmatch(value)
	if match == nil {
		return 0, fmt.Errorf("must be HH:MM")
	}
	hours, _ := strconv.Atoi(match[1])
	minutes, _ := strconv.Atoi(match[2])
	return time.Duration(hours)*time.Hour + time.Duration(minutes)*time.Minute, nil
}

func Start(value string) time.Duration {
	start, err := ParseStart(value)
	if err != nil {
		return 0
	}
	return start
}

func Date(at time.Time, loc *time.Location, start time.Duration) string {
	return at.In(loc).Add(-start).Format("2006-01-02")
}

func Bounds(date string, loc *time.Location, start time.Duration) (time.Time, time.Time, error) {
	day, err := time.ParseInLocation("2006-01-02", date, loc)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	from := day.Add(start)
	return from.UTC(), from.AddDate(0, 0, 1).UTC(), nil
}

func CurrentDate(now time.Time, loc *time.Location, start time.Duration) string {
	return Date(now, loc, start)
}
