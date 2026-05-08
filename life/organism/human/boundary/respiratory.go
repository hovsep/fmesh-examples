package boundary

import (
	"github.com/hovsep/fmesh-examples/life/helper"
	"github.com/hovsep/fmesh/component"
	"github.com/hovsep/fmesh/port"
)

func GetRespiratory() *component.Component {
	return component.New("boundary:respiratory").
		WithDescription("Transforms environmental gas signals into chemical levels and lung input for circulation").
		AddInputs(
			"time",
			"environmental_gas",
		).
		AddOutputs(
			"inspired_gas", // to lungs
		).
		WithActivationFunc(helper.Pipeline(
			getFilterAF(),
			getHumidifyAF(),
		))
}

// getFilterAF returns an activation function in which environmental gas is filtered
func getFilterAF() component.ActivationFunc {
	return func(this *component.Component) error {
		return port.ForwardSignals(this.InputByName("environmental_gas"), this.OutputByName("inspired_gas"))
	}
}

func getHumidifyAF() component.ActivationFunc {
	return func(this *component.Component) error {
		return port.ForwardSignals(this.InputByName("environmental_gas"), this.OutputByName("inspired_gas"))
	}
}
