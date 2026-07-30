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
	"github.com/hovsep/fmesh-examples/life/tui"
	"github.com/hovsep/fmesh-examples/simulation/command"
	"github.com/hovsep/fmesh-examples/simulation/session"
	"github.com/hovsep/fmesh-examples/simulation/stepsim"
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

	mesh, err := getSimulationMesh()
	if err != nil {
		fmt.Println("Failed to build simulation mesh:", err)
		os.Exit(1)
	}

	// Now run the simulation; the producer is non-blocking
	err = internal.HandleGraphFlag(mesh, false)
	if err != nil {
		fmt.Println("Failed to generate graph:", err)
		os.Exit(1)
	}

	sim, err := newSession(mesh)
	if err != nil {
		fmt.Println("Failed to set up the simulation:", err)
		os.Exit(1)
	}

	// Drive the simulation from the integrated dashboard unless asked not to.
	// The plain prompt stays available for piping a script in, and for terminals
	// the full-screen UI cannot drive; that path needs no telemetry sink.
	if usePlainREPL() {
		sim.Configure(session.WithSource(command.NewStdin()))
	} else {
		// The dashboard is both the command source and the telemetry sink: the
		// simulation publishes straight into it over an in-process channel, so
		// there is no socket and no second process. It is built after the
		// session because it completes the session's own command names.
		ui := tui.New(sim.Commands.Names, console.DefaultHistoryPath())
		sim.Configure(session.WithSource(ui), session.WithSink(ui.Sink()))
	}

	if err := sim.Run(); err != nil {
		fmt.Println("Simulation ended with an error:", err)
		os.Exit(1)
	}
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

// newSession wraps the mesh in a simulation: a fixed-step engine over it, and
// a session to drive it.
//
// The mesh knows what a tick of it is worth, and the engine counts them, so
// "every 1d" means a day in Leon's life however fast the loop happens to be
// running -- and whatever step this particular simulation was built with.
func newSession(simMesh *fmesh.FMesh) (*session.Session, error) {
	engine := stepsim.New(simMesh, factor.TickOf(simMesh))

	// The mesh renders everything worth watching onto one port; the session
	// forwards those lines to whatever is watching.
	telemetry, err := stepsim.PortLines(simMesh, "aggregated_state_publisher", "stream")
	if err != nil {
		return nil, err
	}

	sim := session.New(engine, session.WithTelemetry(telemetry))

	// Publish at a bounded rate so the simulation can run at full pace while the
	// stream stays bounded. 20ms (50 Hz) is fine enough that fast waveforms like
	// the ECG's R-peaks are sampled cleanly at real time rather than aliased;
	// "rate:publish" adjusts it at runtime.
	sim.Throttle.SetInterval(20 * time.Millisecond)

	// Run at real time by default: one simulated second per wall-clock second,
	// so the body is watchable and interactive out of the box. "rate:sim 60"
	// fast-forwards, "rate:sim max" removes the cap.
	//
	// Tests run uncapped (see Session.RunFor), which removes the pacing but not
	// the work: an hour of simulated time is 360,000 runs of the mesh at the
	// default step, and costs minutes rather than milliseconds. Tests that need
	// physiological stretches of time build themselves a coarser tick instead.
	sim.Pacer.SetFactor(1)

	// Add the commands addressed to this world; the loop's own -- pause, step,
	// rate:sim, scheduling -- come with the session.
	setMeshCommands(sim)

	//@TODO: what auto started means? is not it just started? remove stupic emojis
	fmt.Println("🚀 Simulation auto-started! Type 'help' for commands ('pause'/'resume', 'exit' to quit).")
	return sim, nil
}
