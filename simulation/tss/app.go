package tss

import (
	"context"
	"fmt"
	"time"

	"github.com/hovsep/fmesh"
)

type Application struct {
	ctx     context.Context
	cancel  context.CancelFunc
	cmdChan chan Command

	REPL *REPL
	sim  *Simulation
}

func NewApp(fm *fmesh.FMesh, simInitFunc SimInitFunc) *Application {
	cmdChan := make(chan Command)
	ctx, cancel := context.WithCancel(context.Background())

	app := &Application{
		ctx:     ctx,
		cancel:  cancel,
		cmdChan: cmdChan,
		REPL:    NewREPL(cmdChan),
		sim:     NewSimulation(ctx, cmdChan, fm).Init(simInitFunc),
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

	go app.sim.Run()

	app.REPL.Run()
}
