package main

import (
	"fmt"
	"os"

	"github.com/hovsep/fmesh"
	"github.com/hovsep/fmesh-examples/internal"
	"github.com/hovsep/fmesh-examples/simulation/tss"
	"github.com/hovsep/fmesh/signal"
)

// This example shows how to turn your fmesh into a simple Time Step Simulation (TSS) program
// @TODO: make it more interesting
func main() {
	fm := getMesh()
	// Generate graphs if needed
	err := internal.HandleGraphFlag(fm)
	if err != nil {
		fmt.Println("Failed to generate graph: ", err)
		os.Exit(1)
	}

	tss.NewApp(fm, initSim).Run()
}

func initSim(sim *tss.Simulation) {
	// Configure simulation
	sim.AutoPause = true

	// Add custom commands
	sim.MeshCommands["dummy"] = func(fm *fmesh.FMesh) {
		fm.ComponentByName("bypass").Inputs().ByName("in").PutSignals(signal.New("dummy line"))
	}

	// Init mesh
	sim.FM.ComponentByName("bypass").
		InputByName("in").
		PutSignals(signal.New("start"))
}
