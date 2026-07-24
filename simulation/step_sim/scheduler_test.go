package step_sim

import (
	"testing"
	"time"
)

func TestScheduler_OneShotFiresOnceWhenDue(t *testing.T) {
	s := NewScheduler()
	s.At(time.Hour, "intake:water 250ml")

	if due := s.Due(30 * time.Minute); len(due) != 0 {
		t.Fatalf("job fired early: %v", due)
	}

	due := s.Due(time.Hour)
	if len(due) != 1 || due[0] != "intake:water 250ml" {
		t.Fatalf("expected the job at its due time, got %v", due)
	}

	if due := s.Due(2 * time.Hour); len(due) != 0 {
		t.Fatalf("one-shot job fired again: %v", due)
	}
	if len(s.Jobs()) != 0 {
		t.Fatal("a spent one-shot job is still queued")
	}
}

func TestScheduler_RepeatDoesNotFireImmediately(t *testing.T) {
	s := NewScheduler()
	// "every 1d" typed at time zero should not also mean "and right now".
	if _, err := s.Every(0, 24*time.Hour, "excretion:defecate", Forever); err != nil {
		t.Fatal(err)
	}

	if due := s.Due(0); len(due) != 0 {
		t.Fatalf("repeating job fired at the moment it was scheduled: %v", due)
	}
	if due := s.Due(24 * time.Hour); len(due) != 1 {
		t.Fatalf("repeating job did not fire after one interval: %v", due)
	}
}

func TestScheduler_RepeatKeepsGoing(t *testing.T) {
	s := NewScheduler()
	if _, err := s.Every(0, time.Hour, "intake:water 250ml", Forever); err != nil {
		t.Fatal(err)
	}

	for hour := 1; hour <= 5; hour++ {
		due := s.Due(time.Duration(hour) * time.Hour)
		if len(due) != 1 {
			t.Fatalf("hour %d: expected one command, got %v", hour, due)
		}
	}
	if len(s.Jobs()) != 1 {
		t.Fatal("an endless job should stay queued")
	}
}

func TestScheduler_RepeatRespectsACount(t *testing.T) {
	s := NewScheduler()
	if _, err := s.Every(0, time.Hour, "intake:food 100kcal", 3); err != nil {
		t.Fatal(err)
	}

	var fired int
	for hour := 1; hour <= 10; hour++ {
		fired += len(s.Due(time.Duration(hour) * time.Hour))
	}

	if fired != 3 {
		t.Fatalf("expected exactly 3 runs, got %d", fired)
	}
	if len(s.Jobs()) != 0 {
		t.Fatal("a job that ran its course is still queued")
	}
}

func TestScheduler_DoesNotReplayMissedOccurrences(t *testing.T) {
	s := NewScheduler()
	if _, err := s.Every(0, time.Hour, "intake:food 500kcal", Forever); err != nil {
		t.Fatal(err)
	}

	// Simulated time jumps a full day past the first due time. Firing 24 meals
	// at once would be worse than useless, so the job should run once and
	// reschedule from here.
	due := s.Due(24 * time.Hour)
	if len(due) != 1 {
		t.Fatalf("expected a single catch-up run, got %d", len(due))
	}

	if due := s.Due(24*time.Hour + 30*time.Minute); len(due) != 0 {
		t.Fatalf("job fired before a full interval had passed: %v", due)
	}
	if due := s.Due(25 * time.Hour); len(due) != 1 {
		t.Fatalf("job did not resume on its interval: %v", due)
	}
}

func TestScheduler_RejectsNonPositiveInterval(t *testing.T) {
	s := NewScheduler()
	if _, err := s.Every(0, 0, "intake:water 1ml", Forever); err == nil {
		t.Fatal("a zero interval would busy-loop the scheduler; it must be rejected")
	}
	if _, err := s.Every(0, -time.Hour, "intake:water 1ml", Forever); err == nil {
		t.Fatal("a negative interval must be rejected")
	}
}

func TestScheduler_Cancel(t *testing.T) {
	s := NewScheduler()
	job := s.At(time.Hour, "intake:water 250ml")

	if !s.Cancel(job.ID) {
		t.Fatal("Cancel reported the job did not exist")
	}
	if s.Cancel(job.ID) {
		t.Fatal("Cancel reported success twice for the same job")
	}
	if due := s.Due(2 * time.Hour); len(due) != 0 {
		t.Fatalf("a cancelled job still fired: %v", due)
	}
}

func TestScheduler_JobsAreListedSoonestFirst(t *testing.T) {
	s := NewScheduler()
	s.At(3*time.Hour, "third")
	s.At(time.Hour, "first")
	s.At(2*time.Hour, "second")

	jobs := s.Jobs()
	want := []Command{"first", "second", "third"}
	for i, w := range want {
		if jobs[i].Command != w {
			t.Fatalf("job %d is %q, want %q; the listing is not ordered by due time", i, jobs[i].Command, w)
		}
	}
}

func TestParseSimDuration(t *testing.T) {
	tests := []struct {
		in      string
		want    time.Duration
		wantErr bool
	}{
		{in: "30m", want: 30 * time.Minute},
		{in: "1h", want: time.Hour},
		{in: "1d", want: 24 * time.Hour},
		{in: "2.5d", want: 60 * time.Hour},
		{in: "1h30m", want: 90 * time.Minute},
		{in: "500ms", want: 500 * time.Millisecond},
		{in: "banana", wantErr: true},
		{in: "", wantErr: true},
	}

	for _, tt := range tests {
		got, err := ParseSimDuration(tt.in)
		if tt.wantErr {
			if err == nil {
				t.Errorf("ParseSimDuration(%q) accepted an invalid duration", tt.in)
			}
			continue
		}
		if err != nil {
			t.Errorf("ParseSimDuration(%q): %v", tt.in, err)
			continue
		}
		if got != tt.want {
			t.Errorf("ParseSimDuration(%q) = %v, want %v", tt.in, got, tt.want)
		}
	}
}

func TestFormatSimDurationRoundTrips(t *testing.T) {
	// Whole days read back as days, so "every 1d" is listed the way it was typed.
	if got := FormatSimDuration(24 * time.Hour); got != "1d" {
		t.Errorf("FormatSimDuration(24h) = %q, want %q", got, "1d")
	}
	if got := FormatSimDuration(90 * time.Minute); got != "1h30m" {
		t.Errorf("FormatSimDuration(90m) = %q, want %q", got, "1h30m")
	}

	// Values without an empty tail are left alone.
	for in, want := range map[time.Duration]string{
		10 * time.Second:       "10s",
		500 * time.Millisecond: "500ms",
		time.Hour:              "1h",
		30 * time.Minute:       "30m",
	} {
		if got := FormatSimDuration(in); got != want {
			t.Errorf("FormatSimDuration(%v) = %q, want %q", in, got, want)
		}
	}
}
