package organ

import "github.com/hovsep/fmesh/component"

func GetNervousSystemComponent() *component.Component {
	return component.New("nervous_system").
		WithDescription("Nervous system").
		AddInputs("time").
		AddOutputs().
		WithActivationFunc(func(this *component.Component) error {
			return nil
		})
}
