package controller

import (
	"fmt"

	"github.com/hovsep/fmesh/component"
)

// GetMental returns the mental stress component of the human being
func GetMental() (*component.Component, error) {
	c, err := component.New("controller:mental_stress",
		component.WithDescription("Mental stress perception of the human being"),
		component.WithInputs("time", "emotional_stimulus"),
		component.WithActivationFunc(func(this *component.Component) error {
			return nil
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("controller:mental_stress: %w", err)
	}
	return c, nil
}
