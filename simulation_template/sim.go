package main

import (
	"context"
	"fmt"
	"time"

	"github.com/hovsep/fmesh"
	"github.com/hovsep/fmesh/cycle"
)

type Simulation struct {
	ctx      context.Context
	fm       *fmesh.FMesh
	cmdChan  chan Command
	isPaused bool
}

func NewSimulation(ctx context.Context, cmdChan chan Command, fm *fmesh.FMesh) *Simulation {
	return &Simulation{
		ctx:     ctx,
		fm:      fm,
		cmdChan: cmdChan,
	}
}

// Init allows initializing the mesh before the simulation starts
func (s *Simulation) Init(initFunc func(fm *fmesh.FMesh)) *Simulation {
	initFunc(s.fm)
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
				// handle user-defined commands or send_signal / get_state
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
