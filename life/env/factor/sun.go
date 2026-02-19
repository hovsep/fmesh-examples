package factor

import (
	"github.com/hovsep/fmesh/component"
)

// GetSunComponent returns the sun radiation exposure factor component of the habitat
func GetSunComponent() *component.Component {
	return component.New("sun").
		WithDescription("Sun radiation exposure factor").
		AddInputs("time", "ctl").
		AddOutputs("uvi", "lux"). // UV index from 0 to 11, illuminance in lux
		WithActivationFunc(func(this *component.Component) error {

			return nil
		})
}
