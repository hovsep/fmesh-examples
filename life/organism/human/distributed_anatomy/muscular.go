package da

import "github.com/hovsep/fmesh/component"

// GetMuscularSystem returns the muscular system component
func GetMuscularSystem() *component.Component {
	c, err := component.New("da:muscular_system",
		component.WithDescription("Muscular system"),
		component.WithInputs("time", "autonomic_tone"),
		component.WithActivationFunc(func(this *component.Component) error {
			return nil
		}),
	)
	if err != nil {
		panic(err)
	}
	return c
}
