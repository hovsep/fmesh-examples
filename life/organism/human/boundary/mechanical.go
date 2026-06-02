package boundary

import "github.com/hovsep/fmesh/component"

func GetMechanical() *component.Component {
	c, err := component.New("boundary:mechanical",
		component.WithDescription("Transforms mechanical stimuli (loads, movement, posture) into signals for musculoskeletal and cardiovascular systems"),
		component.WithInputs(
			"time",
			"physical_activity", // operator command (e.g., exercise intensity)
			"external_forces",   // habitat/environment forces, if any
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
		panic(err)
	}
	return c
}
