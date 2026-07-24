package step_sim

import (
	"strconv"
	"strings"
	"time"
)

// SimClock reports how much simulated time has elapsed since the run began.
//
// Scheduling is expressed in simulated time, not wall-clock time: "once a day"
// has to mean a day in the body's life, however fast or slow the simulation is
// actually running. Only the concrete simulation knows where its clock lives, so
// it supplies one of these.
type SimClock func() time.Duration

// wallClockSince returns a SimClock that simply measures elapsed real time. It
// is the fallback for simulations that have no clock of their own, and makes
// scheduling behave sensibly rather than not at all.
func wallClockSince(start time.Time) SimClock {
	return func() time.Duration { return time.Since(start) }
}

// Now returns the current simulated time.
func (s *Simulation) Now() time.Duration {
	if s.SimClock == nil {
		return 0
	}
	return s.SimClock()
}

// ParseSimDuration reads a duration in simulated time. It accepts everything
// time.ParseDuration does, plus "d" for days, since simulated time routinely
// spans days and "24h" is a clumsy way to say "daily".
func ParseSimDuration(s string) (time.Duration, error) {
	trimmed := strings.TrimSpace(s)
	if days, found := strings.CutSuffix(trimmed, "d"); found {
		// Guard against swallowing real duration strings that merely end in "d";
		// there are none today, but "1.5d" must parse and "1h30md" must not.
		if value, err := strconv.ParseFloat(days, 64); err == nil {
			return time.Duration(value * float64(24*time.Hour)), nil
		}
	}
	return time.ParseDuration(trimmed)
}

// FormatSimDuration renders a simulated duration compactly, preferring days once
// there are any, so "once a day" reads back as it was written.
func FormatSimDuration(d time.Duration) string {
	if d >= 24*time.Hour && d%(24*time.Hour) == 0 {
		return strconv.FormatInt(int64(d/(24*time.Hour)), 10) + "d"
	}

	// time.Duration always spells out every component below the largest, so an
	// hour prints as "1h0m0s". Drop the empty tail so a scheduled job lists back
	// as it was typed. The suffixes checked include the preceding unit, so "10s"
	// is left alone.
	s := d.String()
	if strings.HasSuffix(s, "m0s") {
		s = strings.TrimSuffix(s, "0s")
	}
	if strings.HasSuffix(s, "h0m") {
		s = strings.TrimSuffix(s, "0m")
	}
	return s
}
