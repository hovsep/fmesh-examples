package main

import (
	"fmt"
	"os"

	"github.com/hovsep/fmesh-examples/human_body/body"
	"github.com/hovsep/fmesh-examples/human_body/env"
)

// @TODO: implement basic abstractions: ResourcePool, Oscillator ,Gauge, Controller, Router

func main() {

	// @TODO: implement web server, start , stop and stream

	// Create the world
	world := env.GetMesh()

	// Create the human being
	humanBeing := body.GetComponent()

	// Put human being into the world
	world.AddComponents(humanBeing)

	_, err := world.Run()

	if err != nil {
		fmt.Println("Simulation finished with error: ", err)
		os.Exit(1)
	}

	fmt.Println("Simulation finished successfully")
}
