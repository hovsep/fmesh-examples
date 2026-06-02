package controller

import "github.com/hovsep/fmesh/component"

// GetIntake returns the intake controller component
func GetIntake() *component.Component {
	c, err := component.New("controller:intake",
		component.WithDescription("Intake (e.g., water, food etc)"),
		component.WithInputs("time", "intake"),
		component.WithActivationFunc(func(this *component.Component) error {
			return nil
		}),
	)
	if err != nil {
		panic(err)
	}
	return c
}
