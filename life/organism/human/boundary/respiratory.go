package boundary

import (
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
		WithActivationFunc(func(this *component.Component) error {
			return port.ForwardSignals(this.InputByName("environmental_gas"), this.OutputByName("inspired_gas"))
		})
}
