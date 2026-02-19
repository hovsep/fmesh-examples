package boundary

import "github.com/hovsep/fmesh/component"

func GetSensory() *component.Component {
	return component.New("boundary:sensory").
		WithDescription("Collects sensory signals from the environment and translates them into body load signals").
		AddInputs(
			"time",
			//@todo:
			//Discomfort
			//
			//Pain
			//
			//Overstimulation
		).
		AddOutputs().
		WithActivationFunc(func(this *component.Component) error {
			return nil
		})
}
