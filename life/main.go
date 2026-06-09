package main

import (
	"fmt"
	"os"
	"time"

	"github.com/hovsep/fmesh"
	"github.com/hovsep/fmesh-examples/internal"
	"github.com/hovsep/fmesh-examples/life/helper"
	"github.com/hovsep/fmesh-examples/simulation/step_sim"
	"github.com/hovsep/fmesh-examples/simulation/step_sim/sink"
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
	fmt.Println("╔══════════════════════════════════════════════════════════════╗")
	fmt.Println("║           Human Physiology Step Simulation (\"Life\")         ║")
	fmt.Println("╚══════════════════════════════════════════════════════════════╝")
	fmt.Println()
	fmt.Println("This simulation models a human organism (\"Leon\") inside a habitat.")
	fmt.Println("It demonstrates how fmesh can compose complex, multi-layered systems")
	fmt.Println("using discrete time steps (each tick = 10 ms of simulated time).")
	fmt.Println()
	fmt.Println("Key subsystems simulated:")
	fmt.Println("  • Habitat  — time, gas composition, temperature, sunlight")
	fmt.Println("  • Organs   — lungs (left/right), heart, kidneys, brain, etc.")
	fmt.Println("  • Controllers  — autonomous regulation (heart rate, breathing)")
	fmt.Println("  • Distributed anatomy — blood, nervous system, skin")
	fmt.Println()
	fmt.Println("The habitat is forward-predictive; the human is reactive.")
	fmt.Println("All communication is signal-based — no shared mutable state.")
	fmt.Println("A Unix socket streams live state for the ASCII TUI visualizer.")
	fmt.Println()

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

	// Now run the simulation; the producer is non-blocking
	err = internal.HandleGraphFlag(simMesh, false)
	if err != nil {
		fmt.Println("Failed to generate graph:", err)
		os.Exit(1)
	}

	// Create the unix socket sink so the TUI can connect and visualize state
	fmt.Println("Creating Unix socket sink for TUI at /tmp/" + simMesh.Name() + ".sock ...")
	uiSink, err := sink.NewUnixSocketSink("/tmp/" + simMesh.Name() + ".sock")
	if err != nil {
		fmt.Println("Failed to create sink:", err)
		os.Exit(1)
	}
	fmt.Println("Socket ready. Start the TUI in another terminal: go run ./life/tui/")
	fmt.Println()

	// Run the mesh in a step simulation
	fmt.Println("Launching step simulation REPL. Enter 'help' for available commands.")
	fmt.Println("Type 'step' (or 's') to advance one tick (10 ms). Use 'run' to auto-run.")
	fmt.Println()
	step_sim.NewApp(simMesh, initSim, step_sim.WithSink(uiSink)).Run()
}

// initSim configures simulation and adds custom commands
func initSim(sim *step_sim.Simulation) {
	// Configure simulation
	sim.AutoPause = false

	// Add custom commands
	setMeshCommands(sim.FM, sim.MeshCommands)

	// Setup hooks to stream data to UI
	sim.FM.SetupHooks(func(hooks *fmesh.Hooks) {
		hooks.AfterRun(func(mesh *fmesh.FMesh) error {
			mesh.ComponentByName("aggregated_state_publisher").OutputByName("stream").Signals().ForEach(func(line *signal.Signal) error {
				s, err := helper.AsString(line)
				if err != nil {
					return err
				}
				return sim.Sink.Publish(s)
			})

			// @TODO: take this delay from flag or cmd to not affect tests
			// Slow down to near real time (useful with TUI)
			time.Sleep(10 * time.Millisecond)
			return nil
		})
	})
}
