package main

import (
	"fmt"
	"os"

	"github.com/hovsep/fmesh"
	"github.com/hovsep/fmesh-examples/internal"
	"github.com/hovsep/fmesh-examples/simulation/step_sim"
	"github.com/hovsep/fmesh/signal"
)

// This example shows how to turn your fmesh into a simple step simulation program
// @TODO: make it more interesting
func main() {
	fmt.Println("=== Step Simulation Template ===")
	fmt.Println("This example wraps an fmesh in an interactive step simulation — a REPL-driven")
	fmt.Println("environment where you advance time one tick at a time. Each tick (step) runs all")
	fmt.Println("mesh cycles. Auto-pause stops after every tick so you can inspect state, inject")
	fmt.Println("signals, or issue custom commands before stepping again.")
	fmt.Println()

	fm, err := getMesh()
	if err != nil {
		fmt.Println("Failed to build mesh:", err)
		os.Exit(1)
	}
	// Generate graphs if needed
	err = internal.HandleGraphFlag(fm, true)
	if err != nil {
		fmt.Println("Failed to generate graph: ", err)
		os.Exit(1)
	}

	step_sim.NewApp(fm, initSim).Run()
}

func initSim(sim *step_sim.Simulation) {
	// Configure simulation
	sim.AutoPause = true
	fmt.Println("Auto-pause is ON — the simulator pauses after each tick.")
	fmt.Println("Enter 'step' (or 's') to advance, 'help' for all commands.")
	fmt.Println()

	// Add custom commands
	sim.MeshCommands["dummy"] = step_sim.NewMeshCommand("send one signal to bypass component", func(fm *fmesh.FMesh, _ []string) {
		fm.ComponentByName("bypass").Inputs().ByName("in").PutSignals(signal.New("dummy line"))
	})
	fmt.Println("Registered custom command: 'dummy' — sends a signal through the bypass component.")

	// Init mesh
	sim.FM.ComponentByName("bypass").
		InputByName("in").
		PutSignals(signal.New("start"))
	fmt.Println("Initial signal 'start' injected into 'bypass'. Ready for stepping.")
}
