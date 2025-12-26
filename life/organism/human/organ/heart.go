package organ

import "github.com/hovsep/fmesh/component"

func GetHeartComponent() *component.Component {
	return component.New("heart").
		WithDescription("Heart").
		AddInputs("time").
		AddOutputs().
		WithActivationFunc(func(this *component.Component) error {
			return nil
		})
}
