package controller

import "github.com/hovsep/fmesh/component"

// GetIntake returns the intake controller component
func GetIntake() *component.Component {
	return component.New("controller:intake").
		WithDescription("Intake (e.g., water, food etc)").
		AddInputs("time", "intake").
		AddOutputs(). //Emit loads on organs
		WithActivationFunc(func(this *component.Component) error {
			return nil
		})
}
