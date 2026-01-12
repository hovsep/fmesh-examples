package controller

import "github.com/hovsep/fmesh/component"

// GetPhysical returns the physical stress controller component
func GetPhysical() *component.Component {
	return component.New("controller:physical_stress").
		WithDescription("Physical stress perception of the human being").
		AddInputs("time", "physical_activity").
		AddOutputs(). //Emit loads on organs
		WithActivationFunc(func(this *component.Component) error {
			return nil
		})
}
