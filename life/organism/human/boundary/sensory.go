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
			return nil
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("boundary:sensory: %w", err)
	}
	return c, nil
}
