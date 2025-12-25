package factor

import (
	"github.com/hovsep/fmesh/component"
	"github.com/hovsep/fmesh/port"
)

// GetAirComponent returns the air component of the environment
func GetAirComponent() *component.Component {
	return component.New("air").
		WithDescription("Air quality factor").
		AddLabel("category", "env-factor").
		AttachInputPorts(
			port.NewInput("time").
				WithDescription("Time signal").
				AddLabel("@autopipe-category", "env-factor").
				AddLabel("@autopipe-component", "time").
				AddLabel("@autopipe-port", "tick"),
			port.NewInput("ctl").
				WithDescription("Control signal"),
		).
		AddOutputs("composition", "humidity").
		WithActivationFunc(func(this *component.Component) error {

			return nil
		})
}
