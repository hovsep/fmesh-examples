package organ

import (
	"github.com/hovsep/fmesh-examples/life/common"
	"github.com/hovsep/fmesh-examples/life/helper"
	"github.com/hovsep/fmesh/component"
	"github.com/hovsep/fmesh/port"
	"github.com/hovsep/fmesh/signal"
)

const (
	criticalDamageLevel = 1.0
	defaultDamageLevel  = 0.01
	damageRampRate      = 3.5e-12 //  ~90 years

	NeuralDrive       common.State = "neural_drive"
	NeuralDriveJitter              = 0.02
	MinNeuralDrive                 = 0.0
	MaxNeuralDrive                 = 1.0

	// 0.0 - 0.2 Sleep
	// 0.3 - 0.6 Baseline activity
	// 0.7 - 1.0 Stress, exercise, threat
	defaultNeuralDrive = 0.3
)

// GetBrain returns brain organ component
func GetBrain() *component.Component {
	return component.New("organ:brain").
		WithDescription("The Brain").
		WithInitialState(func(state component.State) {
			state.Set(common.DamageLevel, defaultDamageLevel) // @TODO: add time correlated ramp-up
			state.Set(NeuralDrive, defaultNeuralDrive)
		}).
		AddInputs("time"). // Probably: mental stress, sensory inputs, memories, hormones
		AttachOutputPorts(
			port.NewOutput("neural_drive").WithDescription("Oscillator signal that drives the autonomic phisiology"),
			port.NewOutput("failure").WithDescription("Failure event"),
		).
		WithActivationFunc(func(this *component.Component) error {
			var currentDamage float64

			// Aging
			this.State().Update(common.DamageLevel, func(oldDamage any) any {
				currentDamage = oldDamage.(float64)
				return currentDamage + damageRampRate
			})

			// Brain failure
			if currentDamage >= criticalDamageLevel {
				return this.OutputByName("failure").PutSignals(signal.New("brain_failure").AddLabel("type", "acute")).ChainableErr()
			}

			var nextND float64

			// Normal operation
			this.State().Update(NeuralDrive, func(currentND any) any {

				// Flat ND (we will add more logic later)
				nextND = helper.Clamp(helper.Jitter(currentND.(float64), NeuralDriveJitter), MinNeuralDrive, MaxNeuralDrive)
				return nextND
			})

			return this.OutputByName("neural_drive").PutPayloads(nextND).ChainableErr()
		})
}
