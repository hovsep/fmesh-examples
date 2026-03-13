package main

import (
	"fmt"
	"net"
	"os"
	"sync"
	"time"

	"github.com/hovsep/fmesh"
	"github.com/hovsep/fmesh-examples/internal"
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
//	     (e.g., temperature, humidity, air composition).
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
	const sockPath = "/tmp/sim.sock"

	// Remove old socket if exists
	os.Remove(sockPath)

	l, err := net.Listen("unix", sockPath)
	if err != nil {
		panic(err)
	}
	defer l.Close()
	fmt.Println("Producer listening on", sockPath)

	simMesh := getSimulationMesh()

	streamChan := make(chan string, 1000)

	simMesh.SetupHooks(func(hooks *fmesh.Hooks) {
		hooks.AfterRun(func(mesh *fmesh.FMesh) error {
			mesh.ComponentByName("aggregated_state_publisher").OutputByName("stream").Signals().ForEach(func(line *signal.Signal) error {
				streamChan <- line.PayloadOrNil().(string)
				return nil
			})

			time.Sleep(7 * time.Millisecond) // Near real time
			return nil
		})
	})

	// Accept socket connections in a separate goroutine
	clients := make(map[net.Conn]struct{})
	clientsMu := sync.Mutex{}

	go func() {
		for {
			conn, err := l.Accept()
			if err != nil {
				fmt.Println("accept error:", err)
				continue
			}

			fmt.Println("New client connected")
			clientsMu.Lock()
			clients[conn] = struct{}{}
			clientsMu.Unlock()
		}
	}()

	// Start a goroutine to broadcast streamChan to clients
	go func() {
		for line := range streamChan {
			clientsMu.Lock()
			for c := range clients {
				_, err := fmt.Fprintln(c, line)
				if err != nil {
					// Remove disconnected clients
					c.Close()
					delete(clients, c)
				}
			}
			clientsMu.Unlock()
		}
	}()

	// Now run the simulation; the producer is non-blocking
	err = internal.HandleGraphFlag(simMesh)
	if err != nil {
		fmt.Println("Failed to generate graph:", err)
		os.Exit(1)
	}

	// Run the mesh in a step simulation
	step_sim.NewApp(simMesh, initSim, os.Stdin).Run()
}

// initSim configures simulation and adds custom commands
func initSim(sim *step_sim.Simulation) {
	// Configure simulation
	sim.AutoPause = false

	// Add custom commands
	setMeshCommands(sim.FM, sim.MeshCommands)
}
