package da

import "github.com/hovsep/fmesh/component"

// GetNervousSystem ...
func GetNervousSystem() *component.Component {
	return component.New("da:nervous_system").
		WithDescription("Nervous system").
		AddInputs("time").
		AddOutputs().
		WithActivationFunc(func(this *component.Component) error {
			return nil
		})
}
