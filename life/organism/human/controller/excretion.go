package controller

import "github.com/hovsep/fmesh/component"

// GetExcretion returns the excretion controller component
func GetExcretion() *component.Component {
	return component.New("controller:excretion").
		WithDescription("Manages urine and feces excretion").
		AddInputs("time").
		AddOutputs("urine_out", "feces_out").
		WithActivationFunc(func(this *component.Component) error {
			return nil
		})
}
