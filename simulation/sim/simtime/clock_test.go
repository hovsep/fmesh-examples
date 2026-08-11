package simtime

import (
	"testing"
	"time"
)

func TestParseDuration(t *testing.T) {
	tests := []struct {
		in   string
		want time.Duration
	}{
		{"10s", 10 * time.Second},
		{"1h30m", 90 * time.Minute},
		{"100ms", 100 * time.Millisecond},
		// The unit time.ParseDuration lacks, which is why this function exists.
		{"1d", 24 * time.Hour},
		{"2d", 48 * time.Hour},
		{"1.5d", 36 * time.Hour},
		{"0.5d", 12 * time.Hour},
		{"-1d", -24 * time.Hour},
		// Surrounding whitespace is a typing accident, not an error.
		{" 1d ", 24 * time.Hour},
	}

	for _, tt := range tests {
		got, err := ParseDuration(tt.in)
		if err != nil {
			t.Errorf("ParseDuration(%q): %v", tt.in, err)
			continue
		}
		if got != tt.want {
			t.Errorf("ParseDuration(%q) = %v, want %v", tt.in, got, tt.want)
		}
	}
}

func TestParseDurationRejectsNonsense(t *testing.T) {
	// "d" must not swallow malformed input just because it ends in the letter:
	// only a bare number in front of it means days.
	for _, in := range []string{"", "d", "1h30md", "tomorrow", "5", "1.2.3d"} {
		if got, err := ParseDuration(in); err == nil {
			t.Errorf("ParseDuration(%q) = %v, want an error", in, got)
		}
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		in   time.Duration
		want string
	}{
		// Whole days read back as they were written.
		{24 * time.Hour, "1d"},
		{48 * time.Hour, "2d"},
		{-24 * time.Hour, "-1d"},
		// Empty tails are dropped, but only when they really are empty.
		{time.Hour, "1h"},
		{90 * time.Minute, "1h30m"},
		{time.Minute, "1m"},
		{90 * time.Second, "1m30s"},
		{10 * time.Second, "10s"},
		// A day and a half is not whole days, so it stays in hours.
		{36 * time.Hour, "36h"},
		{0, "0s"},
	}

	for _, tt := range tests {
		if got := FormatDuration(tt.in); got != tt.want {
			t.Errorf("FormatDuration(%v) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestFormatDurationRoundTrips(t *testing.T) {
	// Whatever a job listing prints must parse back to the same instant, or a
	// user cannot copy a listed schedule back into a command.
	for _, d := range []time.Duration{
		10 * time.Second, time.Minute, 90 * time.Second, time.Hour,
		90 * time.Minute, 24 * time.Hour, 72 * time.Hour, 36 * time.Hour,
	} {
		formatted := FormatDuration(d)
		got, err := ParseDuration(formatted)
		if err != nil {
			t.Errorf("ParseDuration(FormatDuration(%v)=%q): %v", d, formatted, err)
			continue
		}
		if got != d {
			t.Errorf("round trip of %v via %q gave %v", d, formatted, got)
		}
	}
}

func TestSinceMeasuresRealTime(t *testing.T) {
	clock := Since(time.Now().Add(-time.Second))
	if elapsed := clock(); elapsed < time.Second {
		t.Fatalf("expected at least a second elapsed, got %v", elapsed)
	}
}
