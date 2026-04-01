package step_sim

import (
	"context"
	"fmt"
	"time"

	"github.com/hovsep/fmesh"
	step_sim_sink "github.com/hovsep/fmesh-examples/simulation/step_sim/sink"
)

type Application struct {
	ctx     context.Context
	cancel  context.CancelFunc
	cmdChan chan Command

	REPL *REPL
	Sim  *Simulation
}

func NewApp(fm *fmesh.FMesh, simInitFunc SimInitFunc) *Application {
	cmdChan := make(chan Command)
	ctx, cancel := context.WithCancel(context.Background())

	// @TODO: make this optional
	sink, err := step_sim_sink.NewUnixSocketSink(ctx, "/tmp/"+fm.Name()+".sock")
	if err != nil {
		panic(err)
	}

	app := &Application{
		ctx:     ctx,
		cancel:  cancel,
		cmdChan: cmdChan,
		REPL:    NewREPL(cmdChan),
		Sim:     NewSimulation(ctx, fm, cmdChan, sink).Init(simInitFunc),
	}

	return app
}

func (app *Application) Run() {
	fmt.Println("Starting the application...")

	defer func() {
		app.cancel()

		time.Sleep(1 * time.Second) // Just to allow the simulation to shut down gracefully (until we implement more elegant synchronization)
		fmt.Println("Shutting down the application...")
	}()

	go app.Sim.Run()

	app.REPL.Run()
}
