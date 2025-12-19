package main

import (
	"fmt"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"github.com/chzyer/readline"
	"github.com/hovsep/fmesh"
	"github.com/hovsep/fmesh-examples/human_body/body"
	"github.com/hovsep/fmesh-examples/human_body/env"
	"github.com/hovsep/fmesh/signal"
)

// Simulation holds the simulation state and controls
type Simulation struct {
	world    *fmesh.FMesh
	paused   atomic.Bool
	quit     atomic.Bool
	commands chan string
}

// newSimulation creates and initializes the simulation world
func newSimulation() *Simulation {
	// Create the world
	world := env.GetMesh()

	// Create the human being
	humanBeing := body.GetComponent()

	// Put human being into the world
	world.AddComponents(humanBeing)

	return &Simulation{
		world:    world,
		commands: make(chan string, 100),
	}
}

// runTimeLoop runs the simulation time loop in background
func (s *Simulation) start() {
	for !s.quit.Load() {
		if s.paused.Load() {
			time.Sleep(50 * time.Millisecond)
			continue
		}

		// Apply pending commands before running
		s.applyPendingCommands()

		s.world.ComponentByName("time").InputByName("ctl").PutSignals(signal.New("tick"))

		_, err := s.world.Run()
		if err != nil {
			fmt.Println("Simulation cycle finished with error: ", err)
			os.Exit(1)
		}
	}
}

// applyPendingCommands drains the command queue and applies commands to the simulation
func (s *Simulation) applyPendingCommands() {
	for {
		select {
		case cmd := <-s.commands:
			if strings.HasPrefix(cmd, "temp:") {
				s.world.ComponentByName("temperature").InputByName("ctl").PutSignals(signal.New(strings.TrimPrefix(cmd, "temp:")))
			}
		default:
			return
		}
	}
}

// runREPL runs the interactive command loop
func (s *Simulation) runREPL() {
	rl, _ := readline.New("> ")
	defer rl.Close()

	fmt.Println("Commands: 'time:show', 'time:stop', 'time:continue', 'quit'")

	for !s.quit.Load() {
		cmd, err := rl.Readline()
		if err != nil {
			break
		}

		s.handleCommand(strings.ToLower(cmd))
	}
}

// handleCommand processes a single REPL command
func (s *Simulation) handleCommand(cmd string) {
	parts := strings.Fields(cmd)
	if len(parts) == 0 {
		return
	}

	switch parts[0] {
	case "time:show":
		timeRel := s.world.ComponentByName("time").State().Get("current_time_rel")
		timeAbs := s.world.ComponentByName("time").State().Get("current_time_abs")
		fmt.Printf("current time abs: %v time rel: %v \n", timeAbs, timeRel)
	case "time:stop":
		s.paused.Store(true)
		fmt.Println("Time paused")
	case "time:continue":
		s.paused.Store(false)
		fmt.Println("Time resumed")
	case "quit", "exit":
		s.quit.Store(true)
	default:
		s.commands <- cmd
	}
}
