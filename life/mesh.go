package main

import (
	"fmt"

	"github.com/hovsep/fmesh"
	"github.com/hovsep/fmesh-examples/life/env"
	"github.com/hovsep/fmesh-examples/life/env/factor"
	"github.com/hovsep/fmesh-examples/life/organism/human"
	"github.com/hovsep/fmesh-examples/simulation/step_sim"
	"github.com/hovsep/fmesh/component"
	"github.com/hovsep/fmesh/signal"
)

// getSimulationMesh returns the main mesh of the simulation
func getSimulationMesh() *fmesh.FMesh {
	// Set up the world
	habitat := getHabitat().
		AddOrganisms(human.New("Leon")).
		AddAggregatedState().
		AddAggregatedStatePublisher()

	// Set up the mesh
	habitat.FM.SetupHooks(func(hooks *fmesh.Hooks) {
		// Generate a tick signal before each run (time step simulation)
		hooks.BeforeRun(func(mesh *fmesh.FMesh) error {
			mesh.ComponentByName("time").InputByName("ctl").PutSignals(signal.New("tick"))
			return nil
		})

	})

	return habitat.FM
}

// getHabitat builds the habitat mesh
func getHabitat() *env.Habitat {
	return env.NewHabitat(component.NewCollection().Add(
		factor.GetTimeComponent(),
		factor.GetGasComponent(),
		factor.GetSunComponent(), // @todo: make sun to affect gas temperature
	))
}

// setMeshCommands sets the commands that can be executed on the mesh
func setMeshCommands(mesh *fmesh.FMesh, commands step_sim.MeshCommandMap) {
	timeComponent := mesh.ComponentByName("time")

	//@TODO: ability to pass params to commands
	//@TODO: a cmd that allows to schedule another cmd at\after exact wall\sim time

	// Print current time
	commands["time:now"] = step_sim.NewMeshCommandDescriptor("Print current time", func(_ *fmesh.FMesh) {
		tickCount := timeComponent.State().Get("tick_count")
		simTime := timeComponent.State().Get("sim_time")
		simWallTime := timeComponent.State().Get("sim_wall_time")
		fmt.Println("Current tick count:", tickCount)
		fmt.Println("Simulation duration:", simTime)
		fmt.Println("Simulation wall-clock time:", simWallTime)
	})

	// Print habitat state
	commands["habitat:show"] = step_sim.NewMeshCommandDescriptor("Print habitat state", func(fm *fmesh.FMesh) {
		temperature := fm.ComponentByName("gas").State().Get("temperature")
		fmt.Println("Current gas temperature: ", temperature)
	})

	// Increase temperature
	commands["temp:inc"] = step_sim.NewMeshCommandDescriptor("Increase gas temperature by 1.0 degree", func(fm *fmesh.FMesh) {
		fm.ComponentByName("gas").Inputs().ByName("ctl").PutSignals(signal.New(+1.0).AddLabel("cmd", "change_temperature"))
	})

	// Decrease temperature
	commands["temp:dec"] = step_sim.NewMeshCommandDescriptor("Decrease gas temperature by 1.0 degree", func(fm *fmesh.FMesh) {
		fm.ComponentByName("gas").Inputs().ByName("ctl").PutSignals(signal.New(-1.0).AddLabel("cmd", "change_temperature"))
	})

	// Set the temperature to zero
	commands["temp:zero"] = step_sim.NewMeshCommandDescriptor("Set gas temperature to zeo degrees", func(fm *fmesh.FMesh) {
		mesh.ComponentByName("gas").Inputs().ByName("ctl").PutSignals(signal.New(0.0).AddLabel("cmd", "set_temperature"))
	})

	// Make the gas hot
	commands["temp:hot"] = step_sim.NewMeshCommandDescriptor("Set gas temperature to +38.0", func(fm *fmesh.FMesh) {
		mesh.ComponentByName("gas").Inputs().ByName("ctl").PutSignals(signal.New(+38.0).AddLabel("cmd", "set_temperature"))
	})

	// Make the gas cold
	commands["temp:cold"] = step_sim.NewMeshCommandDescriptor("Set gas temperature to -35.0", func(fm *fmesh.FMesh) {
		mesh.ComponentByName("gas").Inputs().ByName("ctl").PutSignals(signal.New(-35.0).AddLabel("cmd", "set_temperature"))
	})
}
