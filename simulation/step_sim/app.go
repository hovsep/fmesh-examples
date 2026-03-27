package step_sim

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/hovsep/fmesh"
)

type Application struct {
	ctx     context.Context
	cancel  context.CancelFunc
	cmdChan chan Command
	reader  io.Reader

	REPL *REPL
	Sim  *Simulation
}

func NewApp(fm *fmesh.FMesh, simInitFunc SimInitFunc, reader io.Reader) *Application {
	cmdChan := make(chan Command)
	ctx, cancel := context.WithCancel(context.Background())

	// @TODO: make this optional
	broadcaster, err := NewStreamBroadcaster(ctx, "/tmp/"+fm.Name()+".sock")
	if err != nil {
		panic(err)
	}

	app := &Application{
		ctx:     ctx,
		cancel:  cancel,
		cmdChan: cmdChan,
		reader:  reader,
		REPL:    NewREPL(cmdChan),
		Sim:     NewSimulation(ctx, fm, cmdChan, broadcaster.Stream).Init(simInitFunc),
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

	app.REPL.Run(app.reader)
}
