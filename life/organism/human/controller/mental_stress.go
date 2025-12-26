package controller

import "github.com/hovsep/fmesh/component"

// GetMentalStressComponent returns the mental stress component of the human being
func GetMentalStressComponent() *component.Component {
	return component.New("mental_stress").
		WithDescription("Mental stress perception of the human being").
		AddInputs("time", "emotional_stimulus").
		AddOutputs(). //Emit loads on organs
		WithActivationFunc(func(this *component.Component) error {
			return nil
		})
}
