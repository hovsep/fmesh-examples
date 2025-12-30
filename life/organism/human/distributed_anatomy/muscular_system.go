package organ

import "github.com/hovsep/fmesh/component"

func GetMuscularSystemComponent() *component.Component {
	return component.New("muscular_system").
		WithDescription("Muscular system").
		AddInputs("time").
		AddOutputs().
		WithActivationFunc(func(this *component.Component) error {
			return nil
		})
}
