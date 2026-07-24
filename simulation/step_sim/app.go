package step_sim

import (
	"context"
	"fmt"
	"sync"

	"github.com/hovsep/fmesh"
	step_sim_sink "github.com/hovsep/fmesh-examples/simulation/step_sim/sink"
)

// CommandSource feeds commands into a running simulation.
//
// Run blocks until the source is finished and the application should shut down.
// The source owns cmdChan and is the only thing that may close it (see the
// ownership note on REPL), so Simulation.SendCommand is unsafe once Run returns.
type CommandSource interface {
	Run(cmdChan chan Command)
}

type Application struct {
	cancel  context.CancelFunc
	cmdChan chan Command
	sink    step_sim_sink.Sink
	source  CommandSource

	REPL *REPL
	Sim  *Simulation
}

// Option configures an Application before it starts.
type Option func(*Application)

// WithSink overrides the default sink (e.g. for tests or alternative transports).
func WithSink(s step_sim_sink.Sink) Option {
	return func(app *Application) {
		app.sink = s
	}
}

// WithCommandSource replaces the plain stdin REPL with another way of entering
// commands, such as a full-screen console.
//
// It exists so richer front ends can live in the module that wants them: this
// package stays free of UI dependencies, and examples that are happy with a
// line-at-a-time prompt are unaffected.
func WithCommandSource(source CommandSource) Option {
	return func(app *Application) {
		app.source = source
	}
}

func NewApp(fm *fmesh.FMesh, simInitFunc SimInitFunc, opts ...Option) *Application {
	// Small buffer so the REPL can enqueue commands without blocking during a
	// slow (but bounded) mesh run.
	cmdChan := make(chan Command, 16)
	ctx, cancel := context.WithCancel(context.Background())

	app := &Application{
		cancel:  cancel,
		cmdChan: cmdChan,
	}

	// Default to a no-op sink; callers that want telemetry streaming opt in
	// explicitly via WithSink (e.g. WithSink(step_sim_sink.NewUnixSocketSink(...))).
	app.sink = step_sim_sink.NewNoopSink()

	for _, opt := range opts {
		opt(app)
	}

	app.REPL = NewREPL()
	if app.source == nil {
		app.source = app.REPL
	}
	app.Sim = NewSimulation(ctx, fm, cmdChan, app.sink).Init(simInitFunc)

	return app
}

func (app *Application) Run() {
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		app.Sim.Run()
	}()

	// Blocks until the user exits the command source (which closes cmdChan)
	app.source.Run(app.cmdChan)

	// Stop the simulation if it has not already stopped via the closed cmdChan,
	// then wait for it to fully finish so no further Publish calls race the sink shutdown.
	app.cancel()
	wg.Wait()

	if err := app.sink.Close(); err != nil {
		fmt.Println("error closing sink:", err)
	}
	fmt.Println("Shutting down the application...")
}
