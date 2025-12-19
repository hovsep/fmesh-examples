package factor

import "github.com/hovsep/fmesh/component"

// GetTempComponent returns the temperature component of the environment
func GetTempComponent() *component.Component {
	return component.New("temperature").
		WithDescription("Outside temperature in Celsius degrees").
		AddInputs().
		AddOutputs().
		WithActivationFunc(func(this *component.Component) error {
			return nil
		})
}
