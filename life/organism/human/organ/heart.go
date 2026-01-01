package organ

import "github.com/hovsep/fmesh/component"

// GetHeart returns heart component
func GetHeart() *component.Component {
	return component.New("organ:heart").
		WithDescription("Heart").
		AddInputs("time").
		AddOutputs().
		WithActivationFunc(func(this *component.Component) error {
			return nil
		})
}
