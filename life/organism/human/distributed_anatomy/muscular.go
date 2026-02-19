package da

import "github.com/hovsep/fmesh/component"

// GetMuscularSystem ...
func GetMuscularSystem() *component.Component {
	return component.New("da:muscular_system").
		WithDescription("Muscular system").
		AddInputs("time").
		AddOutputs().
		WithActivationFunc(func(this *component.Component) error {
			return nil
		})
}
