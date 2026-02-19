package organ

import "github.com/hovsep/fmesh/component"

// GetBrain returns brain organ component
func GetBrain() *component.Component {
	return component.New("organ:brain").
		WithDescription("Brain").
		AddInputs("time"). // Probably: mental stress, sensory inputs, memories, hormones
		AddOutputs().      // Control signals like heart rate increase, cortisol release
		WithActivationFunc(func(this *component.Component) error {
			return nil
		})
}
