package main

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/hovsep/fmesh"
	"github.com/hovsep/fmesh-examples/internal"
	"github.com/hovsep/fmesh-examples/life/env"
	"github.com/hovsep/fmesh-examples/life/env/factor"
	"github.com/hovsep/fmesh-examples/life/helper"
	"github.com/hovsep/fmesh-examples/life/organism/human"
	"github.com/hovsep/fmesh-examples/life/organism/human/controller"
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

// bodyCommand registers a command that is delivered into the human's mesh.
//
// The handler turns the raw REPL arguments into the command's scalar arguments;
// returning an error reports a usage problem and the command is not sent, so a
// typo never reaches the body.
func bodyCommand(commands step_sim.MeshCommandMap, name, description string, parseArgs func(args []string) (map[string]float64, error)) {
	if !human.AcceptsCommand(name) {
		// A namespace with no controller behind it would be silently dropped at
		// runtime. Fail loudly at startup instead.
		fmt.Printf("BUG: command %q has no controller (known namespaces: %v)\n", name, human.CommandNamespaces())
		os.Exit(1)
	}

	commands[step_sim.Command(name)] = step_sim.NewMeshCommandDescriptorWithArgs(description,
		func(fm *fmesh.FMesh, args []string) {
			body := helper.FindHumanComponent(fm)
			if body == nil {
				fmt.Println("no human in the simulation")
				return
			}

			scalars, err := parseArgs(args)
			if err != nil {
				fmt.Printf("%s: %v\n", name, err)
				fmt.Println("usage:", description)
				return
			}

			if err := body.InputByName(human.ControlPort).PutSignals(human.Command(name, scalars)); err != nil {
				fmt.Printf("%s: %v\n", name, err)
			}
		})
}

// noArgs is the argument parser for commands that take none.
func noArgs(args []string) (map[string]float64, error) {
	if len(args) > 0 {
		return nil, fmt.Errorf("takes no arguments, got %v", args)
	}
	return nil, nil
}

// setBodyCommands registers the commands addressed to the human being.
func setBodyCommands(commands step_sim.MeshCommandMap) {
	bodyCommand(commands, "intake:water", "drink water, e.g. 'intake:water 500ml'",
		func(args []string) (map[string]float64, error) {
			if len(args) != 1 {
				return nil, fmt.Errorf("expects one volume, e.g. '500ml' or '0.5l'")
			}
			quantity, err := helper.ParseQuantity(args[0])
			if err != nil {
				return nil, err
			}
			ml, err := quantity.Milliliters()
			if err != nil {
				return nil, err
			}
			return map[string]float64{controller.ScalarWaterMl: ml}, nil
		})

	bodyCommand(commands, "intake:food", "eat food, e.g. 'intake:food 200kcal'",
		func(args []string) (map[string]float64, error) {
			if len(args) != 1 {
				return nil, fmt.Errorf("expects one energy, e.g. '200kcal'")
			}
			quantity, err := helper.ParseQuantity(args[0])
			if err != nil {
				return nil, err
			}
			kcal, err := quantity.Kilocalories()
			if err != nil {
				return nil, err
			}
			return map[string]float64{controller.ScalarFoodKcal: kcal}, nil
		})

	bodyCommand(commands, "activity:start",
		"begin exerting, e.g. 'activity:start 8 30m' (intensity as a multiple of rest, optional duration)",
		func(args []string) (map[string]float64, error) {
			if len(args) < 1 || len(args) > 2 {
				return nil, fmt.Errorf("expects an intensity and an optional duration, e.g. '8 30m'")
			}

			intensity, err := strconv.ParseFloat(args[0], 64)
			if err != nil {
				return nil, fmt.Errorf("invalid intensity %q: %w", args[0], err)
			}
			scalars := map[string]float64{controller.ScalarIntensity: intensity}

			if len(args) == 1 {
				// No duration means "keep going until told to stop".
				scalars[controller.ScalarDurationS] = controller.Indefinite
				return scalars, nil
			}

			duration, err := helper.ParseDuration(args[1])
			if err != nil {
				return nil, err
			}
			scalars[controller.ScalarDurationS] = duration.Seconds()
			return scalars, nil
		})

	bodyCommand(commands, "activity:stop", "stop exerting and return to rest", noArgs)
	bodyCommand(commands, "excretion:urinate", "empty the bladder", noArgs)
	bodyCommand(commands, "excretion:defecate", "empty the bowel", noArgs)

	bodyCommand(commands, "emotion:stimulus",
		"apply an emotional event, e.g. 'emotion:stimulus 0.8 -0.5' (arousal 0..1, valence -1..1)",
		func(args []string) (map[string]float64, error) {
			if len(args) != 2 {
				return nil, fmt.Errorf("expects arousal and valence, e.g. '0.8 -0.5'")
			}
			arousal, err := strconv.ParseFloat(args[0], 64)
			if err != nil {
				return nil, fmt.Errorf("invalid arousal %q: %w", args[0], err)
			}
			valence, err := strconv.ParseFloat(args[1], 64)
			if err != nil {
				return nil, fmt.Errorf("invalid valence %q: %w", args[1], err)
			}
			return map[string]float64{
				controller.ScalarArousal: arousal,
				controller.ScalarValence: valence,
			}, nil
		})
}

// describeSimRate renders the current simulation speed for humans.
func describeSimRate(pacer *step_sim.SimPacer) string {
	if !pacer.Enabled() {
		return "max (uncapped)"
	}
	return fmt.Sprintf("%gx real time (%v of wall clock per tick)", pacer.Factor(), pacer.WallTimePerTick())
}

// setMeshCommands sets the commands that can be executed on the mesh
func setMeshCommands(sim *step_sim.Simulation) {
	mesh := sim.FM
	commands := sim.MeshCommands
	timeComponent := mesh.ComponentByName("time")

	//@TODO: a cmd that allows to schedule another cmd at\after exact wall\sim time

	// How often state is published to the UI. This is purely a telemetry
	// concern: it never changes how fast the simulation itself runs (that is
	// "rate:sim"). The two used to share the name "rate", which made the
	// distinction easy to miss.
	setPublishRate := step_sim.NewMeshCommandDescriptorWithArgs(
		"set UI publish interval, e.g. 'rate:ui 100ms' ('rate:ui 0' = publish every cycle)",
		func(_ *fmesh.FMesh, args []string) {
			if len(args) != 1 {
				fmt.Println("usage: rate:ui <duration>   e.g. 'rate:ui 100ms', 'rate:ui 0'")
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
	commands["rate:ui"] = setPublishRate
	commands["rate"] = setPublishRate // kept as an alias for the original name

	// How fast simulated time advances relative to wall-clock time.
	commands["rate:sim"] = step_sim.NewMeshCommandDescriptorWithArgs(
		"set simulation speed in sim-seconds per real second, e.g. 'rate:sim 1' (real time), 'rate:sim 60', 'rate:sim max'",
		func(_ *fmesh.FMesh, args []string) {
			if len(args) != 1 {
				fmt.Println("usage: rate:sim <factor|max>   e.g. 'rate:sim 1', 'rate:sim 60', 'rate:sim max'")
				fmt.Println("current speed:", describeSimRate(sim.Pacer))
				return
			}

			if args[0] == "max" {
				sim.Pacer.SetFactor(step_sim.Uncapped)
				fmt.Println("simulation speed set to max (uncapped)")
				return
			}

			factor, err := strconv.ParseFloat(args[0], 64)
			if err != nil || factor <= 0 {
				fmt.Println("invalid speed:", args[0], "(use a positive number like '1', '60', or 'max')")
				return
			}
			sim.Pacer.SetFactor(factor)
			fmt.Printf("simulation speed set to %gx real time (%v of wall clock per tick)\n",
				factor, sim.Pacer.WallTimePerTick())
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

	setBodyCommands(commands)
}
