package helper

import (
	"time"

	"github.com/hovsep/fmesh-examples/simulation/session"
)

// RunSimulationAndThen runs the simulation for a stretch of simulated time and
// then checks what it left behind.
//
// The run is measured by the simulation's own clock, so a test asking for two
// minutes gets two minutes of Leon's life and pays milliseconds of wall clock
// for them.
func RunSimulationAndThen(sim *session.Session, duration time.Duration, then func()) {
	if err := sim.RunFor(duration); err != nil {
		panic("simulation failed: " + err.Error())
	}
	then()
}
