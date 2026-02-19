package physiology

import "github.com/hovsep/fmesh/component"

// GetObservableState ...
func GetObservableState() *component.Component {
	return component.New("physiology:observable_state").
		WithDescription("Observable state of the human being (e.g., temperature, blood pressure etc)").
		AddInputs("time").
		AddOutputs().
		WithActivationFunc(func(this *component.Component) error {
			return nil
		})
}
