package factor

import (
	"github.com/hovsep/fmesh/component"
)

// GetSunComponent returns the sun radiation exposure factor component of the habitat
func GetSunComponent() *component.Component {
	c, err := component.New("sun",
		component.WithDescription("Sun radiation exposure factor"),
		component.WithInputs("time", "ctl"),
		component.WithOutputs("uvi", "lux"), // UV index from 0 to 11, illuminance in lux
		component.WithActivationFunc(func(this *component.Component) error {
			return nil
		}),
	)
	if err != nil {
		panic(err)
	}
	return c
}
