package main

import (
	"context"
	"fmt"
	"time"

	"github.com/hovsep/fmesh"
	"github.com/hovsep/fmesh/signal"
)

type Application struct {
	ctx     context.Context
	cancel  context.CancelFunc
	cmdChan chan Command

	REPL *REPL
	sim  *Simulation
}

func NewApp() *Application {
	cmdChan := make(chan Command)
	ctx, cancel := context.WithCancel(context.Background())
	app := &Application{
		ctx:     ctx,
		cancel:  cancel,
		cmdChan: cmdChan,
		REPL:    NewREPL(cmdChan),
		sim: NewSimulation(ctx, cmdChan, GetMesh()).
			Init(func(sim *Simulation) {
				// Add custom commands here
				sim.meshCommands["dummy"] = func(fm *fmesh.FMesh) {
					fm.ComponentByName("bypass").Inputs().ByName("in").PutSignals(signal.New("dummy line"))
				}

				// Init mesh
				sim.fm.ComponentByName("bypass").
					InputByName("in").
					PutSignals(signal.New("start"))
			}),
	}

	return app
}

func (app *Application) Run() {
	fmt.Println("Starting the application...")

	defer func() {
		app.cancel()
		close(app.cmdChan)
		time.Sleep(1 * time.Second) // Just to allow the simulation to shut down gracefully
		fmt.Println("Shutting down the application...")
	}()

	go app.sim.Run()

	app.REPL.Run()
}
