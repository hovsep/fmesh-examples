package main

import (
	"fmt"
	"os"

	"github.com/hovsep/fmesh"
	"github.com/hovsep/fmesh-examples/internal"
	"github.com/hovsep/fmesh-examples/simulation/des"
	"github.com/hovsep/fmesh/signal"
)

// This example shows how to turn your fmesh into a simple Discrete Event Simulation (DES) program
// probably this project be moved to "fmesh-sim" repo in the future
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
