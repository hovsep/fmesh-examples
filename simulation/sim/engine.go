package sim

import (
	"context"
	"time"
)

// Engine advances a simulation.
//
// What one advance means is the engine's business: a fixed time step, the next
// scheduled event, one replication of a batch. A session drives the engine and
// never assumes any of it — which is what lets the same loop, commands,
// scheduling and telemetry serve every kind of simulation.
//
// An engine is used from a single goroutine (the one calling Advance), so
// implementations need no locking of their own.
type Engine interface {
	// Advance performs one unit of simulation work and reports what came of it.
	//
	// The context bounds this one advance: cancelling it stops the work in
	// progress, so a session that is being torn down does not have to wait for
	// the advance it happens to be in the middle of.
	//
	// An error means this advance failed, not that the simulation is over; the
	// caller decides what to do (a session pauses and reports it).
	Advance(ctx context.Context) (Result, error)

	// Now reports how much simulated time has elapsed since the run began.
	// It is the simulation's only notion of time: scheduling, pacing and
	// deadlines are all expressed in it.
	Now() time.Duration
}

// Result is what one advance amounted to.
type Result struct {
	// Idle reports that nothing happened: no state changed, so advancing again
	// without new input would not change anything either. Sessions with
	// auto-pause stop on it rather than spin.
	Idle bool

	// Done reports that the engine has no work left at all — a discrete-event
	// calendar that has emptied, a batch that has run every replication. Unlike
	// Idle, this cannot be undone by new input.
	Done bool
}
