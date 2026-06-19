package subscription

import (
	"fmt"
	"regexp"
	"strconv"
	"time"
)

var monthRegexp = regexp.MustCompile(`^(0[1-9]|1[0-2])-(\d{4})$`)

func ParseMonth(value string) (time.Time, error) {
	matches := monthRegexp.FindStringSubmatch(value)
	if matches == nil {
		return time.Time{}, fmt.Errorf("date must have MM-YYYY format")
	}

	month, err := strconv.Atoi(matches[1])
	if err != nil {
		return time.Time{}, fmt.Errorf("parse month: %w", err)
	}
	year, err := strconv.Atoi(matches[2])
	if err != nil {
		return time.Time{}, fmt.Errorf("parse year: %w", err)
	}

	return time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC), nil
}

func FormatMonth(value time.Time) string {
	return fmt.Sprintf("%02d-%04d", int(value.Month()), value.Year())
}

func MonthsInclusive(start, end time.Time) int {
	return (end.Year()-start.Year())*12 + int(end.Month()-start.Month()) + 1
}
