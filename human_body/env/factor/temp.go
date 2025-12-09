package factor

import "github.com/hovsep/fmesh/component"

const componentName = "temperature"

// GetTempComponent returns the temperature component of the environment
func GetTempComponent() *component.Component {
	return component.New(componentName).
		WithDescription("Outside temperature in Celsius degrees").
		AddInputs().
		AddOutputs().
		WithActivationFunc(func(this *component.Component) error {
			return nil
		})
}
