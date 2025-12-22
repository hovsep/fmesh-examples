package des

import (
	"context"
	"fmt"
	"time"

	"github.com/hovsep/fmesh"
	"github.com/hovsep/fmesh/cycle"
)

type MeshCommandMap map[Command]func(fm *fmesh.FMesh)

type SimInitFunc func(sim *Simulation)

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
		MeshCommands: make(MeshCommandMap),
	}
}

// Init allows initializing the simulation before the simulation starts
func (s *Simulation) Init(initFunc func(sim *Simulation)) *Simulation {
	initFunc(s)
	return s
}

func (s *Simulation) Run() {
	fmt.Println("Starting simulation...")

	for {
		// Process all pending commands
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
				case cmdPause:
					s.Pause()
				case cmdResume:
					s.Resume()
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

func (s *Simulation) handleCommand(cmd Command) {
	cmdFunc, ok := s.MeshCommands[cmd]
	if !ok {
		fmt.Printf("Unknown command: %v \n", cmd)
		return
	}
	fmt.Println("Executing command: ", cmd)
	cmdFunc(s.FM)
}
