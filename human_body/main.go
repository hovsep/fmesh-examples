package main

import (
	"fmt"
	"os"

	"github.com/hovsep/fmesh"
	"github.com/hovsep/fmesh-examples/internal"
	"github.com/hovsep/fmesh-examples/simulation/des"
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

	des.NewApp(fm, initSim).Run()
}

func initSim(sim *des.Simulation) {
	// Configure simulation
	sim.AutoPause = false

	// Add custom commands
	sim.MeshCommands["time:now"] = func(fm *fmesh.FMesh) {
		timeRel := sim.FM.ComponentByName("time").State().Get("current_time_rel")
		timeAbs := sim.FM.ComponentByName("time").State().Get("current_time_abs")
		fmt.Printf("current time abs: %v time rel: %v \n", timeAbs, timeRel)
	}

	sim.MeshCommands["time:tick"] = func(fm *fmesh.FMesh) {
		sim.FM.ComponentByName("time").InputByName("ctl").PutSignals(signal.New("tick"))
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
