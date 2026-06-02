package da

import "github.com/hovsep/fmesh/component"

// GetNervousSystem returns the nervous system component
func GetNervousSystem() *component.Component {
	c, err := component.New("da:nervous_system",
		component.WithDescription("Nervous system"),
		component.WithInputs("time"),
		component.WithActivationFunc(func(this *component.Component) error {
			return nil
		}),
	)
	if err != nil {
		panic(err)
	}
	return c
}
