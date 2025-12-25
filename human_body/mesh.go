package main

import (
	"fmt"

	"github.com/hovsep/fmesh"
	"github.com/hovsep/fmesh-examples/human_body/body"
	"github.com/hovsep/fmesh-examples/human_body/env"
	"github.com/hovsep/fmesh-examples/simulation/tss"
	"github.com/hovsep/fmesh/signal"
)

// getMesh returns the main mesh of the simulation
func getMesh() *fmesh.FMesh {
	// Create the world
	world := env.GetMesh()

	// Create the human being
	humanBeing := body.GetComponent()

	env.AddOrganisms(world, humanBeing)

	// Setup the mesh
	world.SetupHooks(func(hooks *fmesh.Hooks) {
		// Let the time tick monotonically
		hooks.BeforeRun(func(mesh *fmesh.FMesh) error {
			mesh.ComponentByName("time").InputByName("ctl").PutSignals(signal.New("tick"))
			return nil
		})
	})

	return world
}

func setMeshCommands(mesh *fmesh.FMesh, commands tss.MeshCommandMap) {
	timeComponent := mesh.ComponentByName("time")

	// Print current time
	commands["time:now"] = func(fm *fmesh.FMesh) {
		tickCount := timeComponent.State().Get("tick_count")
		simTime := timeComponent.State().Get("sim_time")
		simWallTime := timeComponent.State().Get("sim_wall_time")
		fmt.Println("Current tick count: ", tickCount, " sim duration", simTime, " wall time:", simWallTime)
	}
}
