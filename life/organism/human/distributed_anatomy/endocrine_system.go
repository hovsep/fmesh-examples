package organ

import "github.com/hovsep/fmesh/component"

func GetEndocrineSystemComponent() *component.Component {
	return component.New("endocrine_system").
		WithDescription("Endocrine system").
		AddInputs("time").
		AddOutputs().
		WithActivationFunc(func(this *component.Component) error {
			return nil
		})
}
