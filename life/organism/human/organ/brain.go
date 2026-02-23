package organ

import (
	"github.com/hovsep/fmesh/component"
	"github.com/hovsep/fmesh/port"
	"github.com/hovsep/fmesh/signal"
)

const (
	stateDamageLevel    = "damage_level"
	criticalDamageLevel = 1.0
)

// GetBrain returns brain organ component
func GetBrain() *component.Component {
	return component.New("organ:brain").
		WithDescription("The Brain").
		WithInitialState(func(state component.State) {
			state.Set(stateDamageLevel, 0.0)
		}).
		AddInputs("time"). // Probably: mental stress, sensory inputs, memories, hormones
		AttachOutputPorts(
			port.NewOutput("neural_drive").WithDescription("Oscillator signal that drives the autonomic phisiology"),
			port.NewOutput("failure").WithDescription("Failure event"),
		).
		WithActivationFunc(func(this *component.Component) error {
			damageLevel := this.State().Get(stateDamageLevel).(float64)
			if damageLevel >= criticalDamageLevel {
				return this.OutputByName("failure").PutSignals(signal.New("brain_failure").AddLabel("type", "acute")).ChainableErr()
			}

			jitter := 1.0
			// neural_drive value can be used as gain level
			return this.OutputByName("neural_drive").PutPayloads(jitter * damageLevel).ChainableErr()
		})
}
