package boundary

import (
	"fmt"

	"github.com/hovsep/fmesh/component"
)

func GetThermal() (*component.Component, error) {
	c, err := component.New("boundary:thermal",
		component.WithDescription("Transforms environmental thermal signals into body heat load, cold/heat stress signals"),
		component.WithInputs(
			"time",
			"ambient_temperature",
			"ambient_humidity",
			"radiation", // sun UV / IR exposure
		),
		component.WithOutputs(
			"heat_load", // to skin and cardiovascular system
			"cold_load", // to skin and shivering reflex
		),
		component.WithActivationFunc(func(this *component.Component) error {
			return nil
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("boundary:thermal: %w", err)
	}
	return c, nil
}
