package main

import (
	"context"
	"fmt"

	"github.com/hovsep/fmesh"
	"github.com/hovsep/fmesh/signal"
)

// This example shows how to turn your fmesh into a simple Discrete Event Simulation (DES) program
// probably this will be moved to fmesh-sim repo in the future
func main() {
	fmt.Println("Starting the application...")
	ctx, cancel := context.WithCancel(context.Background())
	cmdChan := make(chan Command)

	defer func() {
		cancel()
		close(cmdChan)
		fmt.Println("Shutting down the application...")
	}()

	go NewSimulation(ctx, cmdChan, GetMesh()).
		Init(func(sim *Simulation) {
			// Add custom commands here
			sim.meshCommands["dummy"] = func(fm *fmesh.FMesh) {
				fm.ComponentByName("bypass").Inputs().ByName("in").PutSignals(signal.New("dummy line"))
			}

			// Init mesh
			sim.fm.ComponentByName("bypass").
				InputByName("in").
				PutSignals(signal.New("start"))
		}).Run()

	NewREPL(cmdChan).Run()
}
