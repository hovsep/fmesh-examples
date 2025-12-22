package main

import (
	"context"
	"fmt"
	"time"

	"github.com/hovsep/fmesh"
	"github.com/hovsep/fmesh/cycle"
)

type MeshCommandMap map[Command]func(fm *fmesh.FMesh)

type Simulation struct {
	ctx          context.Context
	fm           *fmesh.FMesh
	cmdChan      chan Command
	isPaused     bool
	meshCommands MeshCommandMap
}

func NewSimulation(ctx context.Context, cmdChan chan Command, fm *fmesh.FMesh) *Simulation {
	return &Simulation{
		ctx:          ctx,
		fm:           fm,
		cmdChan:      cmdChan,
		meshCommands: make(MeshCommandMap),
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
		select {
		case <-s.ctx.Done():
			fmt.Println("Shutting down simulation...")
			return
		case cmd := <-s.cmdChan:
			switch cmd {
			case cmdPause:
				s.Pause()
			case cmdResume:
				s.Resume()
			default:
				s.handleCommand(cmd)
			}
		default:
			if s.isPaused {
				time.Sleep(time.Second)
				continue
			}

			runResult, err := s.fm.Run()
			if err != nil {
				fmt.Println("Simulation cycle finished with error: ", err)
				return
			}

			fmt.Println("Simulation cycle finished successfully after ", runResult.Cycles.Len(), " cycles")

			if !runResult.Cycles.AnyMatch(func(c *cycle.Cycle) bool {
				return c.HasActivatedComponents()
			}) {
				fmt.Println("Simulation does not progress and will be paused (nothing happens in your mesh)")
				fmt.Println("")
				s.Pause()
			}
		}
	}
}

func (s *Simulation) Pause() {
	s.isPaused = true
}

func (s *Simulation) Resume() {
	s.isPaused = false
}

func (s *Simulation) handleCommand(cmd Command) {
	cmdFunc, ok := s.meshCommands[cmd]
	if !ok {
		fmt.Println("Unknown command: ", cmd)
		return
	}
	cmdFunc(s.fm)
}
