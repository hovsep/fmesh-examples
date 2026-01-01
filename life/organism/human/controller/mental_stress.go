package controller

import "github.com/hovsep/fmesh/component"

// GetMentalStress returns the mental stress component of the human being
func GetMentalStress() *component.Component {
	return component.New("controller:mental_stress").
		WithDescription("Mental stress perception of the human being").
		AddInputs("time", "emotional_stimulus").
		AddOutputs(). //Emit loads on organs
		WithActivationFunc(func(this *component.Component) error {
			return nil
		})
}
