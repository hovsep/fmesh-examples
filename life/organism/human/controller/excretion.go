package controller

import (
	"fmt"

	"github.com/hovsep/fmesh/component"
)

// GetExcretion returns the excretion controller component
func GetExcretion() (*component.Component, error) {
	c, err := component.New("controller:excretion",
		component.WithDescription("Manages urine and feces excretion"),
		component.WithInputs("time"),
		component.WithOutputs("urine_out", "feces_out"),
		component.WithActivationFunc(func(this *component.Component) error {
			return nil
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("controller:excretion: %w", err)
	}
	return c, nil
}
