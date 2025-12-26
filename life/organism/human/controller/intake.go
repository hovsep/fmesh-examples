package controller

import "github.com/hovsep/fmesh/component"

func GetIntakeComponent() *component.Component {
	return component.New("intake").
		WithDescription("Intake (e.g., water, food, smell, substances)").
		AddInputs("time", "intake").
		AddOutputs(). //Emit loads on organs
		WithActivationFunc(func(this *component.Component) error {
			return nil
		})
}
