package factor

import (
	"github.com/hovsep/fmesh/component"
	"github.com/hovsep/fmesh/port"
)

// GetSunComponent returns the sun radiation exposure factor component of the habitat
func GetSunComponent() *component.Component {
	return component.New("sun").
		WithDescription("Sun radiation exposure factor").
		AddLabel("category", "habitat-factor").
		AttachInputPorts(
			port.NewInput("time").
				WithDescription("Time signal").
				AddLabel("@autopipe-category", "habitat-factor").
				AddLabel("@autopipe-component", "time").
				AddLabel("@autopipe-port", "tick"),
			port.NewInput("ctl").
				WithDescription("Control signal"),
		).
		AddOutputs("uvi", "lux"). // UV index from 0 to 11, illuminance in lux
		WithActivationFunc(func(this *component.Component) error {

			return nil
		})
}
