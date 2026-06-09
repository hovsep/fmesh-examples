package main

import (
	"fmt"

	"github.com/hovsep/fmesh"
	"github.com/hovsep/fmesh/component"
	"github.com/hovsep/fmesh/port"
	"github.com/hovsep/fmesh/signal"
)

const meshName = "simulation_template"

// GetMesh returns the main mesh for the simulation
func getMesh() *fmesh.FMesh {
	bypassComponent, err := component.New("bypass",
		component.WithDescription("Bypasses all signals"),
		component.WithInputs("in"),
		component.WithOutputs("out"),
		component.WithActivationFunc(func(this *component.Component) error {
			return port.ForwardSignals(this.InputByName("in"), this.OutputByName("out"))
		}),
	)
	if err != nil {
		panic(fmt.Sprintf("failed to create bypass component: %v", err))
	}

	loggerComponent, err := component.New("logger",
		component.WithDescription("Simple logger"),
		component.WithInputs("line"),
		component.WithActivationFunc(func(this *component.Component) error {
			this.InputByName("line").Signals().ForEach(func(sig *signal.Signal) error {
				this.Logger().Println(sig.PayloadOrNil())
				return nil
			})

			return nil
		}),
	)
	if err != nil {
		panic(fmt.Sprintf("failed to create logger component: %v", err))
	}

	if err := bypassComponent.OutputByName("out").PipeTo(loggerComponent.InputByName("line")); err != nil {
		panic(fmt.Sprintf("failed to pipe bypass to logger: %v", err))
	}

	fm, err := fmesh.New(meshName,
		fmesh.WithUnlimitedCycles(),
		fmesh.WithUnlimitedTime(),
	)
	if err != nil {
		panic(fmt.Sprintf("failed to create mesh: %v", err))
	}

	if err := fm.AddComponents(bypassComponent, loggerComponent); err != nil {
		panic(fmt.Sprintf("failed to add components: %v", err))
	}

	return fm
}
