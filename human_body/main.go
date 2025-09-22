package main

import (
	"fmt"
	"os"

	"github.com/hovsep/fmesh"
)

// This example demonstrates simple human body simulation
func main() {

	fm := fmesh.NewWithConfig("adam", &fmesh.Config{
		ErrorHandlingStrategy: fmesh.StopOnFirstErrorOrPanic,
		Debug:                 false,
	})

	runResult, err := fm.Run()
	if err != nil {
		fmt.Println("The mesh finished with error: ", err)
		os.Exit(1)
	}

	fmt.Printf("Mesh stopped after %d cycles and %s", runResult.Cycles.Len(), runResult.Duration)
}
