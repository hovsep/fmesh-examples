package simtime

import (
	"testing"
	"time"
)

func TestPacer_UncappedByDefault(t *testing.T) {
	p := New()

	if p.Enabled() {
		t.Fatal("a new pacer must not pace until a factor is set")
	}

	// Ready must never withhold a step when uncapped, however many run.
	for i := range 1000 {
		if !p.Ready() {
			t.Fatalf("uncapped pacer withheld step %d", i)
		}
		p.Advanced(10 * time.Millisecond)
	}
}

func TestPacer_HoldsBackUntilTheStepIsDue(t *testing.T) {
	p := New()
	// A simulated second per wall second: a 50ms step costs 50ms of wall clock.
	p.SetFactor(1)

	// The first step is always due immediately; only afterwards is there a
	// schedule to keep.
	if !p.Ready() {
		t.Fatal("the first step should be due immediately")
	}
	p.Advanced(50 * time.Millisecond)

	if p.Ready() {
		t.Fatal("the next step is not due yet and must be withheld")
	}
	if got := p.WallTimePerStep(); got != 50*time.Millisecond {
		t.Fatalf("wall budget per step = %v, want 50ms", got)
	}

	// Napping repeatedly must eventually let it through, and must not overshoot
	// the deadline by much.
	start := time.Now()
	for !p.Ready() {
		p.Nap()
		if time.Since(start) > time.Second {
			t.Fatal("pacer never became ready")
		}
	}
	if waited := time.Since(start); waited > 250*time.Millisecond {
		t.Fatalf("waited %v for a 50ms budget", waited)
	}
}

func TestPacer_FactorScalesTheBudget(t *testing.T) {
	p := New()
	// 60 simulated seconds per wall second: a simulated minute costs a second.
	p.SetFactor(60)
	p.Advanced(time.Minute)

	if got := p.WallTimePerStep(); got != time.Second {
		t.Fatalf("wall budget = %v, want 1s", got)
	}
}

func TestPacer_StepsThatMoveNoTimeCostNothing(t *testing.T) {
	p := New()
	p.SetFactor(1)

	// An engine that advanced without moving the clock (an idle tick, a
	// zero-delay event) must not buy itself a wall-clock wait.
	p.Advanced(0)
	if !p.Ready() {
		t.Fatal("an advance covering no simulated time must not delay the next one")
	}
	if got := p.WallTimePerStep(); got != 0 {
		t.Fatalf("wall budget = %v, want 0", got)
	}
}

func TestPacer_DoesNotBurstAfterFallingBehind(t *testing.T) {
	p := New()
	p.SetFactor(1)

	// Simulate having fallen far behind: a step whose budget is tiny compared to
	// the time that has really passed.
	p.Advanced(time.Millisecond)
	time.Sleep(50 * time.Millisecond) // way past maxCatchUpSteps budgets

	// Rebased on now, so the next step is due one budget from here rather than
	// a burst of ~50 catch-up steps all being due at once.
	p.Advanced(time.Millisecond)
	if p.Ready() {
		t.Fatal("after rebasing, the next step should be one budget away, not overdue")
	}
}

func TestPacer_ResetDropsTheSchedule(t *testing.T) {
	p := New()
	p.SetFactor(1)
	p.Advanced(time.Second)

	if p.Ready() {
		t.Fatal("a second-long step should hold the next one back")
	}

	// Time passed while the simulation was paused; none of it was meant to be
	// simulated, so the next step is due at once.
	p.Reset()
	if !p.Ready() {
		t.Fatal("Reset must make the next step due immediately")
	}
}

func TestPacer_Describe(t *testing.T) {
	p := New()
	if got, want := p.Describe(), "max (uncapped)"; got != want {
		t.Fatalf("Describe() = %q, want %q", got, want)
	}

	p.SetFactor(60)
	if got, want := p.Describe(), "60x real time"; got != want {
		t.Fatalf("Describe() = %q, want %q", got, want)
	}

	p.Advanced(time.Minute)
	if got, want := p.Describe(), "60x real time (1s of wall clock per step)"; got != want {
		t.Fatalf("Describe() = %q, want %q", got, want)
	}
}
