package factor

import (
	"github.com/hovsep/fmesh/component"
)

// GetAirComponent returns the air component of the environment
func GetAirComponent() *component.Component {
	return component.New("air").
		WithDescription("Air quality factor").
		AddInputs("time", "ctl").
		AddOutputs("composition", "pressure", "humidity").
		WithActivationFunc(func(this *component.Component) error {

			return nil
		})
}
