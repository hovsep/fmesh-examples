package physiology

import "github.com/hovsep/fmesh/component"

// GetEndocrineAxis ...
func GetEndocrineAxis() *component.Component {
	return component.New("physiology:endocrine_axis").
		WithDescription("Endocrine system").
		AddInputs("time").
		AddOutputs().
		WithActivationFunc(func(this *component.Component) error {
			return nil
		})
}
