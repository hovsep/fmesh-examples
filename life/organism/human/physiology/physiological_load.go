package physiology

import "github.com/hovsep/fmesh/component"

// GetPhysiologicalLoad ...
func GetPhysiologicalLoad() *component.Component {
	return component.New("physiology:physiological_load").
		WithDescription("Physiological load (e.g., thermal, mechanical, radiation etc)").
		AddInputs("time").
		AddOutputs().
		WithActivationFunc(func(this *component.Component) error {
			return nil
		})
}
