package regulation

import "github.com/hovsep/fmesh/component"

// GetHomeostasis ...
func GetHomeostasis() *component.Component {
	return component.New("regulation:homeostasis").
		WithDescription("Homeostasis regulation system. Runs all the time and tries to keep important levels within ranges").
		AddInputs("time").
		AddOutputs().
		WithActivationFunc(func(this *component.Component) error {
			return nil
		})

}
