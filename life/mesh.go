package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"

	"github.com/hovsep/fmesh"
	"github.com/hovsep/fmesh-examples/internal"
	"github.com/hovsep/fmesh-examples/life/env"
	"github.com/hovsep/fmesh-examples/life/env/factor"
	"github.com/hovsep/fmesh-examples/life/helper"
	"github.com/hovsep/fmesh-examples/life/organism/human"
	"github.com/hovsep/fmesh-examples/life/organism/human/controller"
	"github.com/hovsep/fmesh-examples/simulation/command"
	"github.com/hovsep/fmesh-examples/simulation/session"
	"github.com/hovsep/fmesh-examples/simulation/simtime"
	"github.com/hovsep/fmesh-examples/simulation/stepsim"
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
func bodyCommand(sim *session.Session, name, description string, parseArgs func(args []string) (map[string]float64, error)) {
	if !human.AcceptsCommand(name) {
		// A namespace with no controller behind it would be silently dropped at
		// runtime. Fail loudly at startup instead.
		fmt.Printf("BUG: command %q has no controller (known namespaces: %v)\n", name, human.CommandNamespaces())
		os.Exit(1)
	}

	mesh := sim.Engine.(*stepsim.Engine).Mesh()
	sim.Commands.Add(command.Command{
		Name:        name,
		Group:       "Body",
		Description: description,
		Run: func(_ io.Writer, args []string) error {
			body := helper.FindHumanComponent(mesh)
			if body == nil {
				return errors.New("no human in the simulation")
			}

			scalars, err := parseArgs(args)
			if err != nil {
				return fmt.Errorf("%s: %w\nusage: %s", name, err, description)
			}

			return body.InputByName(human.ControlPort).PutSignals(human.Command(name, scalars))
		},
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
func setBodyCommands(sim *session.Session) {
	bodyCommand(sim, "intake:water", "drink water, e.g. 'intake:water 500ml'",
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
			return map[string]float64{controller.KindWaterMl: ml}, nil
		})

	bodyCommand(sim, "intake:food", "eat food, e.g. 'intake:food 200kcal'",
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
			return map[string]float64{controller.KindFoodKcal: kcal}, nil
		})

	bodyCommand(sim, "smoke:cigarette",
		"smoke a cigarette (takes 5-10 min), e.g. 'smoke:cigarette' or 'smoke:cigarette 2'",
		func(args []string) (map[string]float64, error) {
			count := 1.0
			if len(args) == 1 {
				n, err := strconv.ParseFloat(args[0], 64)
				if err != nil || n <= 0 {
					return nil, fmt.Errorf("invalid cigarette count %q", args[0])
				}
				count = n
			} else if len(args) > 1 {
				return nil, fmt.Errorf("expects an optional count, e.g. '2'")
			}
			return map[string]float64{"count": count}, nil
		})

	bodyCommand(sim, "activity:start",
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

			duration, err := simtime.ParseDuration(args[1])
			if err != nil {
				return nil, fmt.Errorf("invalid duration %q: %w", args[1], err)
			}
			scalars[controller.ScalarDurationS] = duration.Seconds()
			return scalars, nil
		})

	bodyCommand(sim, "trauma:bleed",
		"open a wound, e.g. 'trauma:bleed 1500ml' (blood leaves over time, not at once)",
		func(args []string) (map[string]float64, error) {
			if len(args) != 1 {
				return nil, fmt.Errorf("expects one volume, e.g. '1500ml' or '1.5l'")
			}
			quantity, err := helper.ParseQuantity(args[0])
			if err != nil {
				return nil, err
			}
			ml, err := quantity.Milliliters()
			if err != nil {
				return nil, err
			}
			return map[string]float64{controller.ScalarVolumeMl: ml}, nil
		})

	bodyCommand(sim, "activity:stop", "stop exerting and return to rest", noArgs)
	bodyCommand(sim, "excretion:urinate", "empty the bladder", noArgs)
	bodyCommand(sim, "excretion:defecate", "empty the bowel", noArgs)

	bodyCommand(sim, "emotion:stimulus",
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

// setMeshCommands registers the commands addressed to this world.
//
// Only what is specific to it belongs here: driving the loop itself (pause,
// step, rate:sim, scheduling, scripts) is the session's own business.
func setMeshCommands(sim *session.Session) {
	mesh := sim.Engine.(*stepsim.Engine).Mesh()
	timeComponent := mesh.ComponentByName("time")
	gas := mesh.ComponentByName("gas")

	// setTemperature returns a handler steering the habitat's gas temperature.
	// The habitat takes both a delta and an absolute value, told apart by the
	// command label the signal carries.
	setTemperature := func(cmd string, degrees float64) command.Handler {
		return func(_ io.Writer, _ []string) error {
			return gas.InputByName("ctl").PutSignals(
				signal.New(degrees).WithLabel(helper.CommandLabel, cmd))
		}
	}

	sim.Commands.Add(
		command.Command{
			Name: "time:now", Group: "Environment",
			Description: "print the simulated time and tick count",
			Run: func(out io.Writer, _ []string) error {
				fmt.Fprintln(out, "Current tick count:", timeComponent.State().Get("tick_count"))
				fmt.Fprintln(out, "Simulation duration:", timeComponent.State().Get("sim_duration"))
				fmt.Fprintln(out, "Simulation wall-clock time:", timeComponent.State().Get("sim_wall_time"))
				return nil
			},
		},
		command.Command{
			Name: "habitat:show", Group: "Environment",
			Description: "print the habitat state",
			Run: func(out io.Writer, _ []string) error {
				fmt.Fprintln(out, "Current gas temperature: ", gas.State().Get("temperature"))
				return nil
			},
		},
		command.Command{
			Name: "temp:inc", Group: "Environment",
			Description: "raise the gas temperature by 1 degree",
			Run:         setTemperature("change_temperature", +1.0),
		},
		command.Command{
			Name: "temp:dec", Group: "Environment",
			Description: "lower the gas temperature by 1 degree",
			Run:         setTemperature("change_temperature", -1.0),
		},
		command.Command{
			Name: "temp:zero", Group: "Environment",
			Description: "set the gas temperature to zero",
			Run:         setTemperature("set_temperature", 0.0),
		},
		command.Command{
			Name: "temp:hot", Group: "Environment",
			Description: "set the gas temperature to +38",
			Run:         setTemperature("set_temperature", +38.0),
		},
		command.Command{
			Name: "temp:cold", Group: "Environment",
			Description: "set the gas temperature to -35",
			Run:         setTemperature("set_temperature", -35.0),
		},
	)

	setBodyCommands(sim)
}
