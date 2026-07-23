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

	// Create the unix socket sink so the TUI can connect and visualize state
	uiSink, err := sink.NewUnixSocketSink("/tmp/" + simMesh.Name() + ".sock")
	if err != nil {
		fmt.Println("Failed to create sink:", err)
		os.Exit(1)
	}
	fmt.Println("TUI: go run ./life/tuiv2/ /tmp/" + simMesh.Name() + ".sock")

	step_sim.NewApp(simMesh, initSim, step_sim.WithSink(uiSink)).Run()
}

// initSim configures simulation and adds custom commands
func initSim(sim *step_sim.Simulation) {
	// Configure simulation. The loop starts unpaused, so it runs on its own
	// without needing an explicit "resume".
	sim.AutoPause = false
	fmt.Println("🚀 Simulation auto-started! Type 'help' for commands ('pause'/'resume', 'exit' to quit).")

	// Publish state to the UI at a bounded rate so the simulation can run at full
	// pace while the stream stays bounded (0 = every cycle). The "rate" command
	// adjusts this at runtime; both it and the hook below run on the sim
	// goroutine (see Simulation.Run), so no synchronization is needed.
	sim.PublishThrottle.SetInterval(50 * time.Millisecond)

	// Add custom commands (including "rate", which drives sim.PublishThrottle)
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
