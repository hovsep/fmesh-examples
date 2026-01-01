package physiology

import "github.com/hovsep/fmesh/component"

// GetPhysiologicalState ...
func GetPhysiologicalState() *component.Component {
	return component.New("physiology:physiological_state").
		WithDescription("Internal physiological state (e.g., temperature, blood pressure etc)").
		AddInputs("time").
		AddOutputs().
		WithActivationFunc(func(this *component.Component) error {
			return nil
		})
}
