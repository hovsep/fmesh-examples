// Package simtime is simulated time: reading it, writing it, and deciding how
// fast it should pass compared to the clock on the wall.
package simtime

import (
	"strconv"
	"strings"
	"time"
)

// Clock reports how much simulated time has elapsed since a run began.
//
// Simulated time is not wall-clock time: "once a day" has to mean a day inside
// the simulation, however fast or slow it is actually being run. Only the
// simulation knows where its time comes from, so it supplies one of these.
type Clock func() time.Duration

// Since returns a Clock measuring elapsed real time, for simulations that have
// no clock of their own and just want time to pass.
func Since(start time.Time) Clock {
	return func() time.Duration { return time.Since(start) }
}

// day is the unit time.ParseDuration is missing. Simulated runs routinely span
// days, and "24h" is a clumsy way to say "daily".
const day = 24 * time.Hour

// ParseDuration reads a duration in simulated time. It accepts everything
// time.ParseDuration does, plus "d" for days ("1d", "1.5d").
func ParseDuration(s string) (time.Duration, error) {
	trimmed := strings.TrimSpace(s)
	if days, found := strings.CutSuffix(trimmed, "d"); found {
		// Only treat the suffix as days when what precedes it is a plain number,
		// so "1.5d" parses while "1h30md" is still rejected as malformed.
		if value, err := strconv.ParseFloat(days, 64); err == nil {
			return time.Duration(value * float64(day)), nil
		}
	}
	return time.ParseDuration(trimmed)
}

// FormatDuration renders a simulated duration compactly, preferring days once
// there are whole ones, so "once a day" reads back as it was written.
func FormatDuration(d time.Duration) string {
	if d != 0 && d%day == 0 {
		return strconv.FormatInt(int64(d/day), 10) + "d"
	}

	// time.Duration spells out every unit below the largest, so an hour prints
	// as "1h0m0s". Drop the empty tail, checking the unit above each zero so
	// that "10s" and "1m30s" are left alone.
	s := d.String()
	if strings.HasSuffix(s, "m0s") {
		s = strings.TrimSuffix(s, "0s")
	}
	if strings.HasSuffix(s, "h0m") {
		s = strings.TrimSuffix(s, "0m")
	}
	return s
}
