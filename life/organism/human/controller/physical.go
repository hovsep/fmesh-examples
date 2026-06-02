package controller

import "github.com/hovsep/fmesh/component"

// GetPhysical returns the physical stress controller component
// possible input commands or events: physical activity (time, level, type), breath-hold(time), yawning, etc.
func GetPhysical() *component.Component {
	c, err := component.New("controller:physical_stress",
		component.WithDescription("Physical stress perception of the human being"),
		component.WithInputs("time", "physical_activity"),
		component.WithActivationFunc(func(this *component.Component) error {
			return nil
		}),
	)
	if err != nil {
		panic(err)
	}
	return c
}
