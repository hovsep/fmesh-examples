package main

import (
	"fmt"

	"github.com/hovsep/fmesh"
	"github.com/hovsep/fmesh/component"
	"github.com/hovsep/fmesh/port"
	"github.com/hovsep/fmesh/signal"
)

const meshName = "simulation_template"

func getMesh() (*fmesh.FMesh, error) {
	fmt.Println("Mesh layout: [bypass] → [logger]")
	fmt.Println("  bypass: forwards all input signals to its output unmodified")
	fmt.Println("  logger: prints every signal payload it receives")
	fmt.Println()

	fmt.Println("Creating 'bypass' component...")
	bypassComponent, err := component.New("bypass",
		component.WithDescription("Bypasses all signals"),
		component.WithInputs("in"),
		component.WithOutputs("out"),
		component.WithActivationFunc(func(this *component.Component) error {
			return port.ForwardSignals(this.InputByName("in"), this.OutputByName("out"))
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("bypass component: %w", err)
	}

	fmt.Println("Creating 'logger' component...")
	loggerComponent, err := component.New("logger",
		component.WithDescription("Simple logger"),
		component.WithInputs("line"),
		component.WithActivationFunc(func(this *component.Component) error {
			return this.InputByName("line").Signals().ForEach(func(sig *signal.Signal) error {
				this.Logger().Println(sig.PayloadOrNil())
				return nil
			})
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("logger component: %w", err)
	}

	fmt.Println("Wiring bypass → logger...")
	if err := bypassComponent.OutputByName("out").PipeTo(loggerComponent.InputByName("line")); err != nil {
		return nil, fmt.Errorf("pipe bypass→logger: %w", err)
	}

	fmt.Println("Creating fmesh instance...")
	fm, err := fmesh.New(meshName,
		fmesh.WithUnlimitedCycles(),
		fmesh.WithUnlimitedTime(),
	)
	if err != nil {
		return nil, fmt.Errorf("new mesh: %w", err)
	}

	fmt.Println("Adding components to mesh...")
	if err := fm.AddComponents(bypassComponent, loggerComponent); err != nil {
		return nil, fmt.Errorf("add components: %w", err)
	}

	fmt.Println("Simulation mesh ready.")
	return fm, nil
}
