package factor

import (
	"github.com/hovsep/fmesh/component"
)

// GetTemperatureComponent returns the temperature component of the environment
func GetTemperatureComponent() *component.Component {
	return component.New("temperature").
		WithDescription("Ambient temperature in Celsius degrees").
		AddInputs("time", "ctl").
		AddOutputs().
		WithActivationFunc(func(this *component.Component) error {

			return nil
		})
}
