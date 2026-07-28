package schedule

import (
	"testing"
	"time"
)

func TestTimeline_RejectsRepeatCountsThatMeanNothing(t *testing.T) {
	tl := New()

	// A count of zero used to decrement past zero on the first run and then
	// repeat forever -- the exact opposite of what was asked for.
	for _, times := range []int{0, -2, -100} {
		if _, err := tl.Every(0, time.Hour, "tank:drain", times); err == nil {
			t.Errorf("Every(..., %d) was accepted; a count must be positive or Forever", times)
		}
	}

	if _, err := tl.Every(0, time.Hour, "tank:drain", Forever); err != nil {
		t.Errorf("Forever must be accepted: %v", err)
	}
	if _, err := tl.Every(0, time.Hour, "tank:drain", 3); err != nil {
		t.Errorf("a positive count must be accepted: %v", err)
	}
}

func TestTimeline_NextDue(t *testing.T) {
	tl := New()

	// Nothing scheduled: nothing to jump to.
	if _, ok := tl.NextDue(); ok {
		t.Fatal("an empty timeline reported something due")
	}

	tl.After(3*time.Hour, "third")
	tl.After(time.Hour, "first")
	tl.After(2*time.Hour, "second")

	// An event-driven engine advances its clock straight to this.
	due, ok := tl.NextDue()
	if !ok || due != time.Hour {
		t.Fatalf("NextDue() = %v, %v; want 1h", due, ok)
	}

	// Once the earliest has fired, the next one is what's next.
	tl.Advance(time.Hour)
	if due, ok := tl.NextDue(); !ok || due != 2*time.Hour {
		t.Fatalf("after firing the first job, NextDue() = %v, %v; want 2h", due, ok)
	}
}

func TestTimeline_NextDueIsNowForRunnableScenarios(t *testing.T) {
	steps, err := ParseSteps("valve:open; valve:close")
	if err != nil {
		t.Fatal(err)
	}

	tl := New()
	tl.After(time.Hour, "later")
	tl.Start("flush", steps)

	// A scenario that has not started waiting has work to do right now, which
	// must win over any future job.
	if due, ok := tl.NextDue(); !ok || due != 0 {
		t.Fatalf("NextDue() = %v, %v; want 0 (work outstanding now)", due, ok)
	}
}

func TestEntry_DescribeDoesNotCountBackwards(t *testing.T) {
	tl := New()
	tl.After(time.Hour, "valve:open")

	// Listing an overdue job (time has passed it but it has not been advanced
	// yet) must not read "in -30m".
	jobs := tl.Jobs()
	if got := jobs[0].Describe(90 * time.Minute); got != "[1] in 0s: valve:open" {
		t.Fatalf("Describe() = %q, want a non-negative countdown", got)
	}
}
