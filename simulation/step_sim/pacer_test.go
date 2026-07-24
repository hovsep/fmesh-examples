package step_sim

import (
	"testing"
	"time"
)

func TestSimPacer_UncappedByDefault(t *testing.T) {
	p := NewSimPacer(10 * time.Millisecond)

	if p.Enabled() {
		t.Fatal("a new pacer must not pace until a factor is set")
	}
	if got := p.WallTimePerTick(); got != 0 {
		t.Fatalf("uncapped pacer should have no wall budget per tick, got %v", got)
	}

	// Ready must never block when uncapped, however many ticks run.
	for i := range 1000 {
		if !p.Ready() {
			t.Fatalf("uncapped pacer withheld tick %d", i)
		}
		p.Advance()
	}
}

func TestSimPacer_InertWithoutTickDuration(t *testing.T) {
	// step_sim cannot know how much simulated time a tick is worth, so a pacer
	// that was never told must stay out of the way even if a factor is set.
	p := NewSimPacer(0)
	p.SetFactor(1)

	if p.Enabled() {
		t.Fatal("pacer must stay inert until it knows the simulated time per tick")
	}
	if !p.Ready() {
		t.Fatal("inert pacer must not withhold ticks")
	}
}

func TestSimPacer_WallTimePerTick(t *testing.T) {
	tests := []struct {
		name     string
		perTick  time.Duration
		factor   float64
		expected time.Duration
	}{
		{"real time", 10 * time.Millisecond, 1, 10 * time.Millisecond},
		{"60x faster", 10 * time.Millisecond, 60, time.Duration(166666)},
		{"half speed", 10 * time.Millisecond, 0.5, 20 * time.Millisecond},
		{"uncapped", 10 * time.Millisecond, Uncapped, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewSimPacer(tt.perTick)
			p.SetFactor(tt.factor)
			if got := p.WallTimePerTick(); got != tt.expected {
				t.Fatalf("WallTimePerTick() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestSimPacer_WithholdsTicksUntilDue(t *testing.T) {
	// One tick per 50ms of wall clock.
	p := NewSimPacer(50 * time.Millisecond)
	p.SetFactor(1)

	// The first tick is always due immediately.
	if !p.Ready() {
		t.Fatal("first tick should be due immediately")
	}
	p.Advance()

	if p.Ready() {
		t.Fatal("second tick should not be due right after the first")
	}

	time.Sleep(60 * time.Millisecond)
	if !p.Ready() {
		t.Fatal("tick should be due after its wall-clock budget elapsed")
	}
}

func TestSimPacer_ResetDropsSchedule(t *testing.T) {
	p := NewSimPacer(time.Second)
	p.SetFactor(1)
	p.Advance() // next tick is now a full second away

	if p.Ready() {
		t.Fatal("precondition: next tick should be pending")
	}

	// Resetting models a pause: wall time passed but simulated time did not, so
	// the loop should resume immediately rather than owing itself ticks.
	p.Reset()
	if !p.Ready() {
		t.Fatal("Reset must make the next tick due immediately")
	}
}

func TestSimPacer_DoesNotBurstAfterFallingBehind(t *testing.T) {
	p := NewSimPacer(time.Millisecond)
	p.SetFactor(1) // 1ms of wall clock per tick

	p.Advance()
	// Simulate the loop being stalled far longer than one tick's budget.
	time.Sleep(20 * time.Millisecond)

	// Catching up would mean ~20 immediately-due ticks. Instead the pacer
	// rebases, so exactly one tick is due and the next is pushed out again.
	if !p.Ready() {
		t.Fatal("a tick should be due after the stall")
	}
	p.Advance()
	if p.Ready() {
		t.Fatal("pacer burst through backlogged ticks instead of rebasing")
	}
}

func TestSimPacer_SetFactorRebases(t *testing.T) {
	p := NewSimPacer(time.Second)
	p.SetFactor(1)
	p.Advance() // next tick a second out

	// Speeding up should take effect at once, not after the old budget expires.
	p.SetFactor(1000)
	if !p.Ready() {
		t.Fatal("changing the factor must drop the schedule computed for the old one")
	}
}
