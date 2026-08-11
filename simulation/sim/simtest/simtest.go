// Package simtest is the small amount of scaffolding a test needs to drive a
// simulation and then look at what it left behind.
//
// It panics rather than returning errors, which is why it lives in its own
// package rather than on Session: that is the right shape for a fixture and the
// wrong shape for anything a program depends on.
package simtest

import (
	"time"

	"github.com/hovsep/fmesh-examples/simulation/sim/session"
)

// RunFor runs the simulation for a stretch of simulated time and then checks
// what it left behind.
//
// The run is measured by the simulation's own clock and paced by nothing, so a
// test asking for two minutes gets two minutes of the simulated world. That is
// not free, and the cost is not the duration but the number of steps: an hour at
// a ten-millisecond step is 360,000 runs of the mesh. A test that wants
// physiological stretches of time should build itself a coarser step rather than
// expect the clock to skip.
func RunFor(sim *session.Session, duration time.Duration, then func()) {
	if err := sim.RunFor(duration); err != nil {
		panic("simulation failed: " + err.Error())
	}
	then()
}
