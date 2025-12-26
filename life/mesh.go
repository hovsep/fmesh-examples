package main

import (
	"fmt"

	"github.com/hovsep/fmesh"
	"github.com/hovsep/fmesh-examples/life/habitat"
	"github.com/hovsep/fmesh-examples/life/organism/human"
	"github.com/hovsep/fmesh-examples/simulation/tss"
	"github.com/hovsep/fmesh/signal"
)

// getSimulationMesh returns the main mesh of the simulation
func getSimulationMesh() *fmesh.FMesh {
	// Create the world
	habitatMesh := habitat.GetMesh()

	// Create the human being
	humanComponent := human.GetComponent()

	habitat.AddOrganisms(habitatMesh, humanComponent)

	// Set up the mesh
	habitatMesh.SetupHooks(func(hooks *fmesh.Hooks) {
		// Let the time tick monotonically
		hooks.BeforeRun(func(mesh *fmesh.FMesh) error {
			mesh.ComponentByName("time").InputByName("ctl").PutSignals(signal.New("tick"))
			return nil
		})
	})

	return habitatMesh
}

// setMeshCommands sets the commands that can be executed on the mesh
func setMeshCommands(mesh *fmesh.FMesh, commands tss.MeshCommandMap) {
	timeComponent := mesh.ComponentByName("time")

	// Print current time
	commands["time:now"] = func(fm *fmesh.FMesh) {
		tickCount := timeComponent.State().Get("tick_count")
		simTime := timeComponent.State().Get("sim_time")
		simWallTime := timeComponent.State().Get("sim_wall_time")
		fmt.Println("Current tick count: ", tickCount, " sim duration", simTime, " wall time:", simWallTime)
	}

	// Print habitat state
	commands["habitat:show"] = func(fm *fmesh.FMesh) {
		temperature := mesh.ComponentByName("temperature").State().Get("current_temperature")
		fmt.Println("Current temperature: ", temperature)
	}

	// Increase temperature
	commands["temp:inc"] = func(fm *fmesh.FMesh) {
		mesh.ComponentByName("temperature").Inputs().ByName("ctl").PutSignals(signal.New(+5.0).AddLabel("cmd", "change_temperature"))
	}

	// Decrease temperature
	commands["temp:dec"] = func(fm *fmesh.FMesh) {
		mesh.ComponentByName("temperature").Inputs().ByName("ctl").PutSignals(signal.New(-5.0).AddLabel("cmd", "change_temperature"))
	}

	// Set the temperature to zero
	commands["temp:zero"] = func(fm *fmesh.FMesh) {
		mesh.ComponentByName("temperature").Inputs().ByName("ctl").PutSignals(signal.New(0.0).AddLabel("cmd", "set_temperature"))
	}
}
