package organ

import "github.com/hovsep/fmesh/component"

func GetBloodSystemComponent() *component.Component {
	return component.New("blood_system").
		WithDescription("Blood system").
		AddInputs("time").
		AddOutputs().
		WithActivationFunc(func(this *component.Component) error {
			return nil
		})
}
