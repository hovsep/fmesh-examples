package main

import (
	"fmt"
	"os"

	"github.com/hovsep/fmesh-examples/internal"
	"github.com/hovsep/fmesh-examples/simulation/tss"
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

// initSim configures simulation and adds custom commands
func initSim(sim *tss.Simulation) {
	// Configure simulation
	sim.AutoPause = false

	// Add custom commands
	setMeshCommands(sim.FM, sim.MeshCommands)
}
