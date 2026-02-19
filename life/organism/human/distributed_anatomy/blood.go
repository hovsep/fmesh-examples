package da

import "github.com/hovsep/fmesh/component"

// GetBloodSystem
func GetBloodSystem() *component.Component {
	return component.New("da:blood_system").
		WithDescription("Blood system").
		AddInputs("time").
		AddOutputs().
		WithActivationFunc(func(this *component.Component) error {
			return nil
		})
}
