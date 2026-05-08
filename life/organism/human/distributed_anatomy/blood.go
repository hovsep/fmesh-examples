package da

import "github.com/hovsep/fmesh/component"

// GetBloodSystem
func GetBloodSystem() *component.Component {
	return component.New("da:blood_system").
		WithDescription("Blood system").
		AddInputs("time").
		AddOutputs().
		WithInitialState(func(state component.State) {
			state.Set("PO2", 0.0)
			state.Set("PCO2", 0.0)
		}).
		WithActivationFunc(func(this *component.Component) error {
			return nil
		})
}
