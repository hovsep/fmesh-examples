package main

import (
	"fmt"
	"os"
	"slices"
	"time"

	"github.com/hovsep/fmesh"
	"github.com/hovsep/fmesh-examples/internal"
	"github.com/hovsep/fmesh-examples/life/console"
	"github.com/hovsep/fmesh-examples/life/env/factor"
	"github.com/hovsep/fmesh-examples/life/helper"
	"github.com/hovsep/fmesh-examples/life/tui"
	"github.com/hovsep/fmesh-examples/simulation/step_sim"
	"github.com/hovsep/fmesh/signal"
)

// This example demonstrates a basic life simulation.
//
// The simulation consists of two main parts:
//   - Habitat: an external environment model
//   - Human: a physiological model of a human body
//
// The program is implemented as a step simulation model.
//
// Simulation model:
//
//	Time is discrete. Each simulation tick represents 10ms of simulated time.
//	The habitat is forward-predictive:
//	   - The time component generates a tick.
//	   - All habitat factors activate on that tick and compute their next state
//	     (e.g., temperature, humidity, gas composition).
//	The human is reactive:
//	   - The human component activates on the same tick.
//	   - It receives habitat signals and routes them to appropriate internal
//	     subsystems (organs, controllers, or distributed anatomy such as skin,
//	     blood, or nervous system).
//
//	Human internals:
//
//	The human body is implemented as a separate mesh, wrapped as a component.
//	On each external tick, the internal human mesh is executed and the state of each organ is calculated.
//	All communication between components is signal-based; components do not share any mutable state.
//
// Simplifications:
//
//	The feedback loop from human to habitat is intentionally omitted.
//	The simulation is single-directional (habitat → human), as the primary
//	goal is studying human physiology rather than environmental dynamics.
func main() {
	fmt.Println(`Human Physiology Step Simulation ("Life") — each tick = 10ms of simulated time.`)

	simMesh, err := getSimulationMesh()
	if err != nil {
		fmt.Println("Failed to build simulation mesh:", err)
		os.Exit(1)
	}

	// Now run the simulation; the producer is non-blocking
	err = internal.HandleGraphFlag(simMesh, false)
	if err != nil {
		fmt.Println("Failed to generate graph:", err)
		os.Exit(1)
	}

	// Drive the simulation from the integrated dashboard unless asked not to.
	// The plain prompt stays available for piping a script in, and for terminals
	// the full-screen UI cannot drive; that path needs no telemetry sink.
	if !usePlainREPL() {
		// The UI needs the command list for completion, but the simulation that
		// owns it does not exist until NewApp returns. Resolve it lazily, which
		// also means commands registered later are picked up.
		var sim *step_sim.Simulation
		commandNames := func() []string {
			if sim == nil {
				return nil
			}
			return sim.CommandNames()
		}

		// The dashboard is both the command source and the telemetry sink: the
		// simulation publishes straight into it over an in-process channel, so
		// there is no socket and no second process.
		ui := tui.New(commandNames, console.DefaultHistoryPath())
		app := step_sim.NewApp(simMesh, initSim,
			step_sim.WithSink(ui.Sink()),
			step_sim.WithCommandSource(ui))
		sim = app.Sim
		app.Run()
		return
	}

	step_sim.NewApp(simMesh, initSim).Run()
}

// usePlainREPL reports whether to skip the console. A console needs an
// interactive terminal; with input piped in there is nothing to drive it.
func usePlainREPL() bool {
	if slices.Contains(os.Args[1:], "--plain") {
		return true
	}
	stat, err := os.Stdin.Stat()
	return err != nil || stat.Mode()&os.ModeCharDevice == 0
}

// initSim configures simulation and adds custom commands
func initSim(sim *step_sim.Simulation) {
	// Configure simulation. The loop starts unpaused, so it runs on its own
	// without needing an explicit "resume".
	sim.AutoPause = false
	fmt.Println("🚀 Simulation auto-started! Type 'help' for commands ('pause'/'resume', 'exit' to quit).")

	// Publish state to the UI at a bounded rate so the simulation can run at full
	// pace while the stream stays bounded (0 = every cycle). 20ms (50 Hz) is fine
	// enough that fast waveforms like the ECG's R-peaks are sampled cleanly at
	// real time rather than aliased; "rate:ui" adjusts it at runtime. Both it and
	// the hook below run on the sim goroutine (see Simulation.Run), so no
	// synchronization is needed.
	sim.PublishThrottle.SetInterval(20 * time.Millisecond)

	// Teach the pacer how much simulated time a tick is worth, then run at real
	// time by default: one simulated second per wall-clock second, so the body
	// is watchable and interactive out of the box. "rate:sim 60" fast-forwards,
	// "rate:sim max" removes the cap. Tests reset this to uncapped (see
	// newCommandableSim / RunSimulationAndThen) so a simulated hour still costs
	// milliseconds of wall clock.
	sim.Pacer.SetSimTimePerTick(factor.DurationPerTick)
	sim.Pacer.SetFactor(1)

	// Schedule against the simulation's own clock rather than wall time, so
	// "every 1d" means a day in Leon's life however fast the loop is running.
	timeComponent := sim.FM.ComponentByName("time")
	sim.SimClock = func() time.Duration {
		elapsed, _ := timeComponent.State().Get("sim_duration").(time.Duration)
		return elapsed
	}

	// Add custom commands (including "rate:ui" and "rate:sim")
	setMeshCommands(sim)

	// Setup hooks to stream data to UI
	sim.FM.SetupHooks(func(hooks *fmesh.Hooks) {
		hooks.AfterRun(func(mesh *fmesh.FMesh) error {
			// Drop this snapshot if we published too recently; the sim keeps
			// running at full pace regardless.
			if !sim.PublishThrottle.Allow() {
				return nil
			}
			return mesh.ComponentByName("aggregated_state_publisher").OutputByName("stream").Signals().ForEach(func(line *signal.Signal) error {
				s, err := helper.AsString(line)
				if err != nil {
					return err
				}
				return sim.Sink.Publish(s)
			})
		})
	})
}
