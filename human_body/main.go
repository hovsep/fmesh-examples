package main

import (
	"fmt"
	"os"

	"github.com/hovsep/fmesh"
	"github.com/hovsep/fmesh-examples/internal"
	"github.com/hovsep/fmesh-examples/simulation/tss"
	"github.com/hovsep/fmesh/signal"
)

// @TODO: implement basic abstractions: ResourcePool, Oscillator ,Gauge, Controller, Router

func main() {
	fm := getMesh()

	// Generate graphs if needed
	err := internal.HandleGraphFlag(fm)
	if err != nil {
		fmt.Println("Failed to generate graph: ", err)
		os.Exit(1)
	}

	// Run the mesh as Time Step Simulation
	tss.NewApp(fm, initSim).Run()
}

func initSim(sim *tss.Simulation) {
	// Configure simulation
	sim.AutoPause = false

	// Add custom commands
	// Print current time
	sim.MeshCommands["time:now"] = func(fm *fmesh.FMesh) {
		tickCount := sim.FM.ComponentByName("time").State().Get("tick_count")
		simTime := sim.FM.ComponentByName("time").State().Get("sim_time")
		simWallTime := sim.FM.ComponentByName("time").State().Get("sim_wall_time")
		fmt.Println("Current tick count: ", tickCount, " sim duration", simTime, " wall time:", simWallTime)
	}

	// Setup mesh
	sim.FM.SetupHooks(func(hooks *fmesh.Hooks) {
		// Let the time tick monotonically
		hooks.BeforeRun(func(mesh *fmesh.FMesh) error {
			mesh.ComponentByName("time").InputByName("ctl").PutSignals(signal.New("tick"))
			return nil
		})
	})
}
