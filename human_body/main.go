package main

import (
	"fmt"
)

// @TODO: implement basic abstractions: ResourcePool, Oscillator ,Gauge, Controller, Router

func main() {
	sim := newSimulation()

	go sim.start()

	sim.runREPL()

	fmt.Println("Simulation finished successfully")
}
