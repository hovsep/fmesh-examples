package step_sim

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"time"

	"github.com/hovsep/fmesh"
	"github.com/hovsep/fmesh/cycle"
)

type MeshCommandMap map[Command]MeshCommandDescriptor

type SimInitFunc func(sim *Simulation)

// Simulation is a wrapper around a mesh
// it runs the mesh in a loop and feeds it with commands from outside (e.g., REPL or another system)
type Simulation struct {
	ctx          context.Context
	cmdChan      chan Command
	isPaused     bool
	FM           *fmesh.FMesh
	MeshCommands MeshCommandMap
	AutoPause    bool
}

func NewSimulation(ctx context.Context, cmdChan chan Command, fm *fmesh.FMesh) *Simulation {
	return &Simulation{
		ctx:          ctx,
		FM:           fm,
		cmdChan:      cmdChan,
		MeshCommands: getDefaultMeshCommands(),
	}
}

func getDefaultMeshCommands() MeshCommandMap {
	meshCommands := make(MeshCommandMap)
	// Default commands are handled by the REPL, we add them here just to handle descriptions in one place
	meshCommands[Exit] = NewMeshCommandDescriptor("exit REPL", NoopMeshCommand)
	meshCommands[Pause] = NewMeshCommandDescriptor("pause simulation", NoopMeshCommand)
	meshCommands[Resume] = NewMeshCommandDescriptor("resume simulation", NoopMeshCommand)
	meshCommands[Help] = NewMeshCommandDescriptor("show this help message", func(_ *fmesh.FMesh) {
		showHelp(meshCommands)
	})
	return meshCommands
}

// Init allows initializing the simulation before the simulation starts,
// e.g., adding custom commands or manipulating the mesh before it starts running
func (s *Simulation) Init(initFunc func(sim *Simulation)) *Simulation {
	initFunc(s)
	return s
}

// Run starts the simulation loop
func (s *Simulation) Run() {
	fmt.Println("Starting simulation...")

	for {
		// Process incoming commands
		checkCommands := true
		for checkCommands {
			select {
			case <-s.ctx.Done():
				fmt.Println("Shutting down simulation...")
				return
			case cmd, ok := <-s.cmdChan:
				if !ok {
					fmt.Println("Command channel closed, shutting down simulation...")
					return
				}
				switch cmd {
				case Pause:
					s.Pause()
				case Resume:
					s.Resume()
				case Exit:
					fmt.Println("Exiting simulation...")
					return
				default:
					s.handleCommand(cmd)
				}
			default:
				// No more commands in the channel, break the inner loop
				checkCommands = false
			}
		}

		// Sleep if paused to avoid a busy-wait
		if s.isPaused {
			time.Sleep(time.Second)
			continue
		}

		// Run a single simulation cycle
		runResult, err := s.FM.Run()
		if err != nil {
			fmt.Println("Simulation cycle finished with error:", err)
			return
		}

		// Auto-pause if nothing is happening
		if s.AutoPause && !runResult.Cycles.AnyMatch(func(c *cycle.Cycle) bool {
			return c.HasActivatedComponents()
		}) {
			fmt.Println("Simulation does not progress and will be paused (nothing happens in your mesh)")
			s.Pause()
		}
	}
}

func (s *Simulation) Pause() {
	fmt.Println("Simulation paused")
	s.isPaused = true
}

func (s *Simulation) Resume() {
	fmt.Println("Simulation resumed")
	s.isPaused = false
}

// handleCommand executes a valid command
func (s *Simulation) handleCommand(cmd Command) {
	cmdDescriptor, ok := s.MeshCommands[cmd]
	if !ok {
		fmt.Printf("Unknown command: %v \n", cmd)
		return
	}
	cmdDescriptor.RunWithMesh(s.FM)
}

func (s *Simulation) SendCommand(cmd Command) {
	s.cmdChan <- cmd
}

func showHelp(meshCommands MeshCommandMap) {
	fmt.Println("Available commands:")

	for _, cmd := range slices.Sorted(maps.Keys(meshCommands)) {
		fmt.Printf("  %s - %s\n", cmd, meshCommands[cmd].Description)
	}
}
