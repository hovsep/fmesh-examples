// Package stepsim advances a mesh in fixed steps of simulated time.
//
// One advance is one mesh run, and every advance is worth the same amount of
// simulated time. That is the right model whenever the thing being simulated is
// continuous and you are sampling it — a body, a circuit, a climate — as
// opposed to a queue of discrete events.
//
// The mesh is expected to do something on every run, usually because a hook
// injects a tick signal before it:
//
//	fm.SetupHooks(func(hooks *fmesh.Hooks) {
//		hooks.BeforeRun(func(mesh *fmesh.FMesh) error {
//			return mesh.ComponentByName("time").InputByName("ctl").PutSignals(signal.New("tick"))
//		})
//	})
//
//	eng := stepsim.New(fm, 10*time.Millisecond)
//	session.New(eng).Run()
package stepsim

import (
	"time"

	"github.com/hovsep/fmesh"
	"github.com/hovsep/fmesh-examples/simulation"
	"github.com/hovsep/fmesh-examples/simulation/simtime"
	"github.com/hovsep/fmesh/cycle"
)

// Engine runs a mesh once per step of simulated time.
type Engine struct {
	fm    *fmesh.FMesh
	step  time.Duration
	steps uint64
	clock simtime.Clock
}

// Option configures an engine.
type Option func(*Engine)

// WithClock overrides where simulated time is read from.
//
// By default the engine counts its own steps, which is exactly right when the
// mesh has no opinion. Use this when the mesh itself owns time — a component
// that advances a calendar, or one replaying a recorded trace — so that
// scheduling and pacing follow the mesh rather than a parallel count.
func WithClock(clock simtime.Clock) Option {
	return func(e *Engine) { e.clock = clock }
}

// New returns an engine advancing fm one mesh run at a time, each run worth
// step of simulated time.
func New(fm *fmesh.FMesh, step time.Duration, opts ...Option) *Engine {
	e := &Engine{fm: fm, step: step}
	for _, opt := range opts {
		opt(e)
	}
	return e
}

// Mesh returns the mesh being simulated, for commands that need to reach into
// it.
func (e *Engine) Mesh() *fmesh.FMesh { return e.fm }

// Step returns how much simulated time one advance is worth.
func (e *Engine) Step() time.Duration { return e.step }

// Steps returns how many advances have run.
func (e *Engine) Steps() uint64 { return e.steps }

// Now reports elapsed simulated time.
func (e *Engine) Now() time.Duration {
	if e.clock != nil {
		return e.clock()
	}
	return time.Duration(e.steps) * e.step
}

// Advance runs the mesh once.
//
// A run in which no component activated is reported as idle: the mesh has
// settled and running it again would change nothing until something new arrives
// from outside.
func (e *Engine) Advance() (simulation.Result, error) {
	info, err := e.fm.Run()
	if err != nil {
		return simulation.Result{}, err
	}
	e.steps++

	activated := info.Cycles.Count(func(c *cycle.Cycle) bool {
		return c.HasActivatedComponents()
	})
	return simulation.Result{Idle: activated == 0}, nil
}
