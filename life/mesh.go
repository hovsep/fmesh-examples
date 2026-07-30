package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/hovsep/fmesh"
	"github.com/hovsep/fmesh-examples/internal"
	"github.com/hovsep/fmesh-examples/life/atmosphere"
	"github.com/hovsep/fmesh-examples/life/bloodstream"
	"github.com/hovsep/fmesh-examples/life/env"
	"github.com/hovsep/fmesh-examples/life/env/factor"
	"github.com/hovsep/fmesh-examples/life/organism/human"
	"github.com/hovsep/fmesh-examples/life/organism/human/controller"
	"github.com/hovsep/fmesh-examples/simulation/command"
	"github.com/hovsep/fmesh-examples/simulation/session"
	"github.com/hovsep/fmesh-examples/simulation/simtime"
	"github.com/hovsep/fmesh-examples/simulation/stepsim"
	"github.com/hovsep/fmesh/component"
	"github.com/hovsep/fmesh/signal"
)

// getSimulationMesh returns the main mesh of the simulation, in the world the
// body normally lives in.
func getSimulationMesh() (*fmesh.FMesh, error) {
	return getSimulationMeshIn(factor.GetGasComponent, factor.DefaultTickDuration)
}

// getSimulationMeshIn builds the same simulation inside a different environment.
//
// The parameter is the whole of the drop-in argument. Anything that publishes
// environmental_gas under the name "gas" is a world this body can live in: the
// atmosphere, a barochamber, and whatever else is written later. Nothing inside
// the organism is parameterised, because nothing inside it needs to be -- it
// receives air on a port and has no way to ask where the air came from.
//
// The tick is the other axis: how much simulated time one run of this mesh is
// worth. It is a property of the simulation rather than of the body, and the
// body cannot tell the difference.
func getSimulationMeshIn(environment func() (*component.Component, error), tick time.Duration) (*fmesh.FMesh, error) {
	// Set up the world
	habitat, err := getHabitat(environment, tick)
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

	// Note the step on the mesh so the engine driving it does not have to be told
	// separately, and cannot be told something different.
	factor.RecordTick(habitat.FM, tick)

	err = internal.HandleGraphFlag(habitat.FM, false)
	if err != nil {
		fmt.Println("Failed to generate graph:", err)
		os.Exit(1)
	}

	return habitat.FM, nil
}

// getHabitat builds the habitat mesh around the given environment.
func getHabitat(environment func() (*component.Component, error), tick time.Duration) (*env.Habitat, error) {
	factors := component.NewCollection()

	timeComponent, err := factor.GetTimeComponent(tick)
	if err != nil {
		return nil, fmt.Errorf("failed to build habitat factors: %w", err)
	}
	gasComponent, err := environment()
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
		sunComponent,
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
			body := human.Find(mesh)
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
			quantity, err := command.ParseQuantity(args[0])
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
			quantity, err := command.ParseQuantity(args[0])
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
			quantity, err := command.ParseQuantity(args[0])
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
	sun := mesh.ComponentByName("sun")

	// setTemperature returns a handler steering the habitat's gas temperature.
	// The habitat takes both a delta and an absolute value, told apart by the
	// command label the signal carries.
	setTemperature := func(cmd string, degrees float64) command.Handler {
		return func(_ io.Writer, _ []string) error {
			return gas.InputByName("ctl").PutSignals(
				signal.New(degrees).WithLabel(command.Label, cmd))
		}
	}

	// gasSetting returns a handler that sends one numeric setting to whatever is
	// currently playing the part of the environment.
	gasSetting := func(cmd string, fallback float64) command.Handler {
		return func(out io.Writer, args []string) error {
			value := fallback
			if len(args) > 0 {
				parsed, err := strconv.ParseFloat(args[0], 64)
				if err != nil {
					return fmt.Errorf("expected a number, got %q", args[0])
				}
				value = parsed
			}
			return gas.InputByName("ctl").PutSignals(
				signal.New(value).WithLabel(command.Label, cmd))
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
			Description: "print what the environment is currently offering",
			Run: func(out io.Writer, _ []string) error {
				// Read the air rather than the environment's private state. An
				// atmosphere keeps an altitude and a chamber keeps a pressure, and
				// neither has the other's; what they have in common is the breath
				// they publish, which is the only thing the body sees either.
				air := gas.OutputByName("environmental_gas").Signals().First()
				if air == nil {
					fmt.Fprintln(out, "the environment has not published any air yet")
					return nil
				}

				_, oxygen, _, _, temperature, humidity, err := atmosphere.Unpack(air)
				if err != nil {
					return err
				}
				pressure := atmosphere.Pressure(air)

				fmt.Fprintf(out, "%s (%s)\n", gas.Name(), gas.Description())
				fmt.Fprintf(out, "  pressure     %.0f mmHg (%.2f atm)\n", pressure, pressure/atmosphere.SeaLevelPressure)
				fmt.Fprintf(out, "  oxygen       %.1f%% -> inspired PO₂ %.0f mmHg\n",
					oxygen, oxygen/100*(pressure-bloodstream.WaterVaporPressure))
				fmt.Fprintf(out, "  temperature  %.1f °C\n", temperature)
				fmt.Fprintf(out, "  humidity     %.0f%%\n", humidity)
				return nil
			},
		},
		command.Command{
			Name: "sun:hour", Group: "Environment",
			Description: "set the hour of the day, e.g. `sun:hour 12` for midday (6 to 20 is daylight)",
			Run: func(_ io.Writer, args []string) error {
				if len(args) != 1 {
					return errors.New("expects one hour, e.g. '12'")
				}
				hour, err := strconv.ParseFloat(args[0], 64)
				if err != nil || hour < 0 || hour > 24 {
					return fmt.Errorf("invalid hour %q (0 to 24)", args[0])
				}
				return sun.InputByName("ctl").PutSignals(
					command.Pack("set_hour", map[string]float64{"hour": hour}))
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
		command.Command{
			Name: "altitude", Group: "Environment",
			Description: "move the habitat to a height above sea level, e.g. `altitude 5500`",
			Run:         gasSetting("set_altitude", 0),
		},
		// The chamber commands are registered whatever the world is, because the
		// alternative is a command list that changes shape depending on which
		// environment was built -- and a body in the open air simply ignores an
		// instruction to pressurise, exactly as the atmosphere ignores it.
		command.Command{
			Name: "air:pressure", Group: "Environment",
			Description: "set the air pressure in mmHg (760 is sea level), e.g. `air:pressure 2280`",
			Run:         gasSetting("set_pressure", atmosphere.SeaLevelPressure),
		},
		command.Command{
			Name: "air:preset", Group: "Environment",
			Description: "put the body somewhere, e.g. `air:preset hyperbaric` (" +
				strings.Join(factor.PresetNames(), ", ") + ")",
			Run: func(_ io.Writer, args []string) error {
				if len(args) != 1 {
					return fmt.Errorf("expects one preset: %v", factor.PresetNames())
				}
				return gas.InputByName("ctl").PutSignals(
					signal.New(args[0]).
						WithLabel(command.Label, "set_preset").
						WithLabel("preset", args[0]))
			},
		},
		command.Command{
			Name: "air:co", Group: "Environment",
			Description: "put carbon monoxide in the air, in ppm, e.g. `air:co 800` (0 to clear it)",
			Run:         gasSetting("set_co", 0),
		},
		command.Command{
			Name: "air:oxygen", Group: "Environment",
			Description: "set the oxygen percentage of the mixture, e.g. `air:oxygen 100`",
			Run:         gasSetting("set_oxygen", 21),
		},
	)

	setBodyCommands(sim)
}
