package main

import (
	"github.com/hovsep/fmesh"
	"github.com/hovsep/fmesh/component"
	"github.com/hovsep/fmesh/port"
	"github.com/hovsep/fmesh/signal"
)

const meshName = "simulation_template"

// GetMesh returns the main mesh for the simulation
func getMesh() *fmesh.FMesh {
	bypassComponent := component.New("bypass").
		WithDescription("Bypasses all signals").
		AddInputs("in").
		AddOutputs("out").WithActivationFunc(func(this *component.Component) error {
		return port.ForwardSignals(this.InputByName("in"), this.OutputByName("out"))
	})

	loggerComponent := component.New("logger").
		WithDescription("Simple logger").
		AddInputs("line").
		WithActivationFunc(func(this *component.Component) error {
			this.InputByName("line").Signals().ForEach(func(sig *signal.Signal) error {
				this.Logger().Println(sig.PayloadOrNil())
				return nil
			})

			return nil
		})

	bypassComponent.OutputByName("out").PipeTo(loggerComponent.InputByName("line"))

	return fmesh.NewWithConfig(meshName, &fmesh.Config{
		ErrorHandlingStrategy: 0,
		Debug:                 false,
		CyclesLimit:           fmesh.UnlimitedCycles,
		TimeLimit:             fmesh.UnlimitedTime,
	}).AddComponents(bypassComponent, loggerComponent)
}
