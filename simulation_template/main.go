package main

import (
	"fmt"
	"os"

	"github.com/hovsep/fmesh-examples/internal"
)

// This example shows how to turn your fmesh into a simple Discrete Event Simulation (DES) program
// probably this project be moved to "fmesh-sim" repo in the future
func main() {

	fm := GetMesh()
	// Generate graphs if needed
	err := internal.HandleGraphFlag(fm)
	if err != nil {
		fmt.Println("Failed to generate graph: ", err)
		os.Exit(1)
	}

	NewApp(fm).Run()
}
