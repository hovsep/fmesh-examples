package organ

import "github.com/hovsep/fmesh/component"

func GetHeartComponent() *component.Component {
	return component.New("heart").
		WithDescription("Heart of the human body").
		AddInputs().
		AddOutputs().
		WithActivationFunc(func(this *component.Component) error {
			return nil
		})
}
