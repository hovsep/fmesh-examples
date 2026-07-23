package main

import (
	"fmt"
	"os"
	"time"

	"github.com/hovsep/fmesh"
	"github.com/hovsep/fmesh-examples/internal"
	"github.com/hovsep/fmesh-examples/life/env"
	"github.com/hovsep/fmesh-examples/life/env/factor"
	"github.com/hovsep/fmesh-examples/life/organism/human"
	"github.com/hovsep/fmesh-examples/simulation/step_sim"
	"github.com/hovsep/fmesh/component"
	"github.com/hovsep/fmesh/signal"
)

// getSimulationMesh returns the main mesh of the simulation
func getSimulationMesh() (*fmesh.FMesh, error) {
	// Set up the world
	habitat, err := getHabitat()
	if err != nil {
		return nil, fmt.Errorf("getHabitat: %w", err)
	}

	leon, err := human.New("Leon")
	if err != nil {
		return nil, fmt.Errorf("human.New: %w", err)
	}

	habitat, err = habitat.AddOrganisms(leon)
	if err != nil {
		return nil, fmt.Errorf("AddOrganisms: %w", err)
	}

	habitat, err = habitat.AddAggregatedState()
	if err != nil {
		return nil, fmt.Errorf("AddAggregatedState: %w", err)
	}

	habitat, err = habitat.AddAggregatedStatePublisher()
	if err != nil {
		return nil, fmt.Errorf("AddAggregatedStatePublisher: %w", err)
	}

	// Generate a tick signal before each run (time step simulation)
	habitat.FM.SetupHooks(func(hooks *fmesh.Hooks) {
		hooks.BeforeRun(func(mesh *fmesh.FMesh) error {
			mesh.ComponentByName("time").InputByName("ctl").PutSignals(signal.New("tick"))
			return nil
		})
	})

	err = internal.HandleGraphFlag(habitat.FM, false)
	if err != nil {
		fmt.Println("Failed to generate graph:", err)
		os.Exit(1)
	}

	return habitat.FM, nil
}

// getHabitat builds the habitat mesh
func getHabitat() (*env.Habitat, error) {
	factors := component.NewCollection()

	timeComponent, err := factor.GetTimeComponent()
	if err != nil {
		return nil, fmt.Errorf("failed to build habitat factors: %w", err)
	}
	gasComponent, err := factor.GetGasComponent()
	if err != nil {
		return nil, fmt.Errorf("failed to build habitat factors: %w", err)
	}
	sunComponent, err := factor.GetSunComponent()
	if err != nil {
		return nil, fmt.Errorf("failed to build habitat factors: %w", err)
	}

	if err := factors.Add(
		timeComponent,
		gasComponent,
		sunComponent, // @todo: make sun to affect gas temperature
	); err != nil {
		return nil, fmt.Errorf("failed to build habitat factors: %w", err)
	}
	return env.NewHabitat(factors)
}

// setMeshCommands sets the commands that can be executed on the mesh
func setMeshCommands(sim *step_sim.Simulation) {
	mesh := sim.FM
	commands := sim.MeshCommands
	timeComponent := mesh.ComponentByName("time")

	//@TODO: a cmd that allows to schedule another cmd at\after exact wall\sim time

	// Set how often state is published to the UI (decoupled from sim speed)
	commands["rate"] = step_sim.NewMeshCommandDescriptorWithArgs(
		"set UI publish interval, e.g. 'rate 100ms' ('rate 0' = publish every cycle)",
		func(_ *fmesh.FMesh, args []string) {
			if len(args) != 1 {
				fmt.Println("usage: rate <duration>   e.g. 'rate 100ms', 'rate 0'")
				fmt.Println("current publish interval:", sim.PublishThrottle.Interval())
				return
			}
			d, err := time.ParseDuration(args[0])
			if err != nil || d < 0 {
				fmt.Println("invalid duration:", args[0], "(use e.g. '100ms', '1s', or '0')")
				return
			}
			sim.PublishThrottle.SetInterval(d)
			fmt.Println("publish interval set to", d)
		})

	// Print current time
	commands["time:now"] = step_sim.NewMeshCommandDescriptor("Print current time", func(_ *fmesh.FMesh) {
		tickCount := timeComponent.State().Get("tick_count")
		simTime := timeComponent.State().Get("sim_duration")
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
		fm.ComponentByName("gas").Inputs().ByName("ctl").PutSignals(signal.New(+1.0).WithLabel("cmd", "change_temperature"))
	})

	// Decrease temperature
	commands["temp:dec"] = step_sim.NewMeshCommandDescriptor("Decrease gas temperature by 1.0 degree", func(fm *fmesh.FMesh) {
		fm.ComponentByName("gas").Inputs().ByName("ctl").PutSignals(signal.New(-1.0).WithLabel("cmd", "change_temperature"))
	})

	// Set the temperature to zero
	commands["temp:zero"] = step_sim.NewMeshCommandDescriptor("Set gas temperature to zeo degrees", func(fm *fmesh.FMesh) {
		mesh.ComponentByName("gas").Inputs().ByName("ctl").PutSignals(signal.New(0.0).WithLabel("cmd", "set_temperature"))
	})

	// Make the gas hot
	commands["temp:hot"] = step_sim.NewMeshCommandDescriptor("Set gas temperature to +38.0", func(fm *fmesh.FMesh) {
		mesh.ComponentByName("gas").Inputs().ByName("ctl").PutSignals(signal.New(+38.0).WithLabel("cmd", "set_temperature"))
	})

	// Make the gas cold
	commands["temp:cold"] = step_sim.NewMeshCommandDescriptor("Set gas temperature to -35.0", func(fm *fmesh.FMesh) {
		mesh.ComponentByName("gas").Inputs().ByName("ctl").PutSignals(signal.New(-35.0).WithLabel("cmd", "set_temperature"))
	})
}
