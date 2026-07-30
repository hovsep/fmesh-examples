package boundary

import (
	"fmt"

	"github.com/hovsep/fmesh/component"
)

func GetSensory() (*component.Component, error) {
	c, err := component.New("boundary:sensory",
		component.WithDescription("Collects sensory signals from the environment and translates them into body load signals"),
		component.WithInputs(
			"time",
			//@todo:
			//Discomfort
			//
			//Pain
			//
			//Overstimulation
		),
		component.WithActivationFunc(func(this *component.Component) error {
			//@TODO: we need to implement or drop it completely
			// first check if there are still things not covered by other components
			// maybe this one can handle things like cognitive load or sadness or boringness (probably from some controller, not directly from env)
			return nil
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("boundary:sensory: %w", err)
	}
	return c, nil
}
