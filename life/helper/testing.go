package helper

import (
	"time"

	"github.com/hovsep/fmesh-examples/simulation/step_sim"
)

// WithRunningSimulation is a helper function that runs the simulation and executes a callback after a given duration
func WithRunningSimulation(sim *step_sim.Simulation, duration time.Duration, f func()) {
	go sim.Run()
	defer func() {
		sim.SendCommand(step_sim.Exit)
	}()

	// Let the simulation run for a while
	time.Sleep(duration)

	f()
}
