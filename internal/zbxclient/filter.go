package zbxclient

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// HourRange is a half-open [Start, End) hour-of-day range.
type HourRange struct{ Start, End int }

// MarshalJSON serializes HourRange as a 2-element array [start, end].
func (r HourRange) MarshalJSON() ([]byte, error) {
	return []byte(fmt.Sprintf("[%d,%d]", r.Start, r.End)), nil
}

// ParseHourRanges parses "8-12,13-17".
func ParseHourRanges(s string) ([]HourRange, error) {
	var out []HourRange
	for _, part := range strings.Split(s, ",") {
		p := strings.TrimSpace(part)
		if p == "" {
			continue
		}
		bounds := strings.Split(p, "-")
		if len(bounds) != 2 {
			return nil, fmt.Errorf("invalid range format '%s'; expected 'START-END' (e.g. '8-12')", p)
		}
		start, err1 := strconv.Atoi(strings.TrimSpace(bounds[0]))
		end, err2 := strconv.Atoi(strings.TrimSpace(bounds[1]))
		if err1 != nil || err2 != nil {
			return nil, fmt.Errorf("non-integer hour in '%s'", p)
		}
		if start < 0 || start > 23 || end < 1 || end > 24 || start >= end {
			return nil, fmt.Errorf("invalid hour boundaries in '%s'; start 0-23, end 1-24, start < end", p)
		}
		out = append(out, HourRange{start, end})
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no valid hour ranges found in '%s'", s)
	}
	return out, nil
}

// TrendFilter keeps samples by local hour-of-day and weekday.
type TrendFilter struct {
	HourRanges   []HourRange
	WeekdaysOnly bool
	Location     *time.Location
}

// NewTrendFilter builds the filter from CLI flags.
func NewTrendFilter(businessHours bool, hours string, weekdaysOnly bool, loc *time.Location) (TrendFilter, error) {
	f := TrendFilter{WeekdaysOnly: weekdaysOnly || businessHours, Location: loc}
	if businessHours {
		f.HourRanges = []HourRange{{8, 12}, {13, 17}}
	}
	if hours != "" {
		r, err := ParseHourRanges(hours)
		if err != nil {
			return TrendFilter{}, err
		}
		f.HourRanges = r
	}
	return f, nil
}

// Keep reports whether a sample at clock passes the filter.
func (f TrendFilter) Keep(clock int64) bool {
	loc := f.Location
	if loc == nil {
		loc = time.UTC
	}
	t := time.Unix(clock, 0).In(loc)
	if f.WeekdaysOnly && (t.Weekday() == time.Saturday || t.Weekday() == time.Sunday) {
		return false
	}
	if len(f.HourRanges) == 0 {
		return true
	}
	for _, r := range f.HourRanges {
		if t.Hour() >= r.Start && t.Hour() < r.End {
			return true
		}
	}
	return false
}
