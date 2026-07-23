package step_sim

import (
	"context"
	"fmt"
	"sync"

	"github.com/hovsep/fmesh"
	step_sim_sink "github.com/hovsep/fmesh-examples/simulation/step_sim/sink"
)

type Application struct {
	cancel  context.CancelFunc
	cmdChan chan Command
	sink    step_sim_sink.Sink

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

	app.REPL = NewREPL(cmdChan)
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

	// Blocks until the user exits the REPL (which closes cmdChan)
	app.REPL.Run()

	// Stop the simulation if it has not already stopped via the closed cmdChan,
	// then wait for it to fully finish so no further Publish calls race the sink shutdown.
	app.cancel()
	wg.Wait()

	if err := app.sink.Close(); err != nil {
		fmt.Println("error closing sink:", err)
	}
	fmt.Println("Shutting down the application...")
}
