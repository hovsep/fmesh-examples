package controller

import "github.com/hovsep/fmesh/component"

// GetExcretion returns the excretion controller component
func GetExcretion() *component.Component {
	c, err := component.New("controller:excretion",
		component.WithDescription("Manages urine and feces excretion"),
		component.WithInputs("time"),
		component.WithOutputs("urine_out", "feces_out"),
		component.WithActivationFunc(func(this *component.Component) error {
			return nil
		}),
	)
	if err != nil {
		panic(err)
	}
	return c
}
