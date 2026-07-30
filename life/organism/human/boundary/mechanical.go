package boundary

import (
	"fmt"

	"github.com/hovsep/fmesh/component"
)

// @TODO: check if all from description is implemented, can we command something like "posture:sit 1h"?
func GetMechanical() (*component.Component, error) {
	c, err := component.New("boundary:mechanical",
		component.WithDescription("Transforms mechanical stimuli (loads, movement, posture) into signals for musculoskeletal and cardiovascular systems"),
		component.WithInputs(
			"time",
			"physical_activity", // operator command (e.g., exercise intensity)
			//@TODO: check if this port is used
			"external_forces", // habitat/environment forces, if any
		),
		component.WithOutputs(
			"muscle_load",     // to muscles
			"skeletal_stress", // to skeletal system
			"cardio_load",     // to heart/circulation
		),
		component.WithActivationFunc(func(this *component.Component) error {
			return nil
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("boundary:mechanical: %w", err)
	}
	return c, nil
}
