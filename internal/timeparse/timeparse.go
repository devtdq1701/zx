// Package timeparse parses zabbix-cli style time expressions.
package timeparse

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Point is one parsed instant: Unix for API queries, Zabbix for the web
// frontend (relative expressions are kept relative).
type Point struct {
	Unix   int64
	Zabbix string
}

const zabbixLayout = "2006-01-02 15:04:05"

var relativePattern = regexp.MustCompile(`^now(?:-(\d+)([smhdwMySHDWY]))?$`)

var unitSeconds = map[string]int64{
	"s": 1, "m": 60, "h": 3600, "d": 86400, "w": 7 * 86400, "M": 30 * 86400, "y": 365 * 86400,
}

func normalizeUnit(u string) string {
	switch u {
	case "m", "M":
		return u
	default:
		return strings.ToLower(u)
	}
}

var absoluteLayouts = []struct {
	layout   string
	dateOnly bool
}{
	{"2006-01-02 15:04:05", false},
	{"2006-01-02 15:04", false},
	{"02/01/2006 15:04:05", false},
	{"02/01/2006 15:04", false},
	{"2006-01-02", true},
	{"02/01/2006", true},
}

func absolute(t time.Time, loc *time.Location) Point {
	return Point{Unix: t.Unix(), Zabbix: t.In(loc).Format(zabbixLayout)}
}

// Parse parses one expression. isEnd selects 23:59:59 for date-only input.
func Parse(s string, isEnd bool, now time.Time, loc *time.Location) (Point, error) {
	text := strings.TrimSpace(s)
	if m := relativePattern.FindStringSubmatch(text); m != nil {
		if m[1] == "" {
			return Point{Unix: now.Unix(), Zabbix: "now"}, nil
		}
		n, _ := strconv.ParseInt(m[1], 10, 64)
		u := normalizeUnit(m[2])
		return Point{Unix: now.Unix() - n*unitSeconds[u], Zabbix: fmt.Sprintf("now-%d%s", n, u)}, nil
	}
	if strings.HasPrefix(strings.ToLower(text), "yesterday") {
		y := now.In(loc).AddDate(0, 0, -1)
		timePart := strings.TrimSpace(text[len("yesterday"):])
		if timePart == "" {
			h, mi, sec := 0, 0, 0
			if isEnd {
				h, mi, sec = 23, 59, 59
			}
			return absolute(time.Date(y.Year(), y.Month(), y.Day(), h, mi, sec, 0, loc), loc), nil
		}
		layout := "15:04"
		if strings.Count(timePart, ":") == 2 {
			layout = "15:04:05"
		}
		tp, err := time.Parse(layout, timePart)
		if err != nil {
			return Point{}, fmt.Errorf("invalid date/time format: '%s'", s)
		}
		return absolute(time.Date(y.Year(), y.Month(), y.Day(), tp.Hour(), tp.Minute(), tp.Second(), 0, loc), loc), nil
	}
	if t, err := time.Parse(time.RFC3339, text); err == nil {
		return absolute(t, loc), nil
	}
	for _, f := range absoluteLayouts {
		t, err := time.ParseInLocation(f.layout, text, loc)
		if err != nil {
			continue
		}
		if f.dateOnly && isEnd {
			t = t.Add(23*time.Hour + 59*time.Minute + 59*time.Second)
		}
		return absolute(t, loc), nil
	}
	return Point{}, fmt.Errorf("invalid date/time format: '%s'", s)
}

// ParseRange resolves --time-range / --from / --to / --days like zabbix-cli.
func ParseRange(rangeStr, from, to string, days int, now time.Time, loc *time.Location) (Point, Point, error) {
	if rangeStr != "" {
		split := false
		for _, sep := range []string{"..", ",", ";"} {
			if i := strings.Index(rangeStr, sep); i >= 0 {
				from, to = strings.TrimSpace(rangeStr[:i]), strings.TrimSpace(rangeStr[i+len(sep):])
				split = true
				break
			}
		}
		if !split {
			from, to = strings.TrimSpace(rangeStr), "now"
		}
	}
	switch {
	case from != "" && to == "":
		to = "now"
	case from == "" && to != "":
		from = fmt.Sprintf("now-%dd", days)
	case from == "" && to == "":
		from, to = fmt.Sprintf("now-%dd", days), "now"
	}
	start, err := Parse(from, false, now, loc)
	if err != nil {
		return Point{}, Point{}, err
	}
	end, err := Parse(to, true, now, loc)
	if err != nil {
		return Point{}, Point{}, err
	}
	if start.Unix >= end.Unix {
		return Point{}, Point{}, fmt.Errorf("start time (%s) must be earlier than end time (%s)", from, to)
	}
	return start, end, nil
}
