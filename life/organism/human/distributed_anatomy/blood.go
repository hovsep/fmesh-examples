package da

import "github.com/hovsep/fmesh/component"

// GetBloodSystem returns the blood system component
func GetBloodSystem() *component.Component {
	c, err := component.New("da:blood_system",
		component.WithDescription("Blood system"),
		component.WithInputs("time"),
		component.WithActivationFunc(func(this *component.Component) error {
			return nil
		}),
		component.WithInitialState(func(state component.State) {
			state.Set("PO2", 0.0)
			state.Set("PCO2", 0.0)
		}),
	)
	if err != nil {
		panic(err)
	}
	return c
}
