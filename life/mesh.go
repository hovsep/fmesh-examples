package main

import (
	"fmt"
	"os"

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
	fmt.Println("Building simulation mesh...")
	fmt.Println()

	// Set up the world
	fmt.Println("Phase 1: Setting up habitat...")
	habitat, err := getHabitat()
	if err != nil {
		return nil, fmt.Errorf("getHabitat: %w", err)
	}
	fmt.Println("Habitat ready (time + gas + sun factors).")
	fmt.Println()

	fmt.Println("Phase 2: Creating human organism 'Leon'...")
	leon, err := human.New("Leon")
	if err != nil {
		return nil, fmt.Errorf("human.New: %w", err)
	}
	fmt.Println("Human organism created with organs, boundaries, controllers,")
	fmt.Println("distributed anatomy (blood, nerves, skin), and physiology.")
	fmt.Println()

	fmt.Println("Phase 3: Placing organism into habitat...")
	habitat, err = habitat.AddOrganisms(leon)
	if err != nil {
		return nil, fmt.Errorf("AddOrganisms: %w", err)
	}
	fmt.Println("Leon placed in habitat.")
	fmt.Println()

	fmt.Println("Phase 4: Adding aggregated state tracking...")
	habitat, err = habitat.AddAggregatedState()
	if err != nil {
		return nil, fmt.Errorf("AddAggregatedState: %w", err)
	}

	fmt.Println("Phase 5: Adding state publisher (Unix socket stream)...")
	habitat, err = habitat.AddAggregatedStatePublisher()
	if err != nil {
		return nil, fmt.Errorf("AddAggregatedStatePublisher: %w", err)
	}

	// Set up the mesh
	fmt.Println()
	fmt.Println("Phase 6: Wiring environment factors (time tick generator)...")
	habitat.FM.SetupHooks(func(hooks *fmesh.Hooks) {
		// Generate a tick signal before each run (time step simulation)
		hooks.BeforeRun(func(mesh *fmesh.FMesh) error {
			mesh.ComponentByName("time").InputByName("ctl").PutSignals(signal.New("tick"))
			return nil
		})

	})
	fmt.Println("Tick generator wired: the 'time' component receives a 'tick' signal before each run.")

	err = internal.HandleGraphFlag(habitat.FM, false)
	if err != nil {
		fmt.Println("Failed to generate graph:", err)
		os.Exit(1)
	}

	fmt.Println()
	fmt.Println("Simulation mesh fully built and ready.")
	return habitat.FM, nil
}

// getHabitat builds the habitat mesh
func getHabitat() (*env.Habitat, error) {
	fmt.Println("  Creating habitat factors...")
	factors := component.NewCollection()

	fmt.Println("    - Time factor (drives step simulation)")
	timeComponent, err := factor.GetTimeComponent()
	if err != nil {
		return nil, fmt.Errorf("failed to build habitat factors: %w", err)
	}
	fmt.Println("    - Gas factor (temperature, humidity, gas composition)")
	gasComponent, err := factor.GetGasComponent()
	if err != nil {
		return nil, fmt.Errorf("failed to build habitat factors: %w", err)
	}
	fmt.Println("    - Sun factor (sunlight / radiation)")
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
func setMeshCommands(mesh *fmesh.FMesh, commands step_sim.MeshCommandMap) {
	timeComponent := mesh.ComponentByName("time")

	//@TODO: ability to pass params to commands
	//@TODO: a cmd that allows to schedule another cmd at\after exact wall\sim time

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
