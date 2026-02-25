package physiology

import (
	"github.com/hovsep/fmesh-examples/life/helper"
	"github.com/hovsep/fmesh-examples/life/organism/human/organ"
	"github.com/hovsep/fmesh/component"
)

const (
	criticalNeuralDrive               = organ.MaxNeuralDrive * 0.01
	defaultAutonomicCoordinationNoise = 0.05
	defaultRegionalBiasJitter         = 0.05
)

// GetAutonomicCoordination ...
func GetAutonomicCoordination() *component.Component {
	return component.New("physiology:autonomic_coordination").
		WithDescription("Autonomic coordination system").
		AddInputs("time", "neural_drive").
		AddOutputs("autonomic_tone").
		WithActivationFunc(func(this *component.Component) error {
			if !this.InputByName("neural_drive").HasSignals() {
				this.Logger().Println("No signal from brain")
				return nil
			}

			neuralDrive := this.InputByName("neural_drive").Signals().FirstPayloadOrDefault(0.0).(float64)

			if neuralDrive <= criticalNeuralDrive {
				this.Logger().Println("Neural drive too low")
				return nil
			}

			// Sympathetic level rises with ND
			sym, paraSym := neuralDrive, helper.Clamp(1.0-neuralDrive, 0.0, 1.0)

			// Gain proportional to ND
			gain := neuralDrive

			// Regional biases as a fraction of ND, with some small variability
			regionalDriveBaser := neuralDrive * 0.5
			cardiacBias := helper.Jitter(regionalDriveBaser, defaultRegionalBiasJitter) // ±5% jitter
			vascularBias := helper.Jitter(regionalDriveBaser, defaultRegionalBiasJitter)
			respiratoryBias := helper.Jitter(regionalDriveBaser, defaultRegionalBiasJitter)
			giBias := helper.Jitter(regionalDriveBaser, defaultRegionalBiasJitter)

			this.OutputByName("autonomic_tone").PutSignals(NewAutonomicTone(sym, paraSym, defaultAutonomicCoordinationNoise, gain, cardiacBias, vascularBias, respiratoryBias, giBias))
			this.Logger().Println("autonomic tone produced")
			this.Logger().Printf("Autonomic tone produced with gain %f and biases cardiac %f, vascular %f, respiratory %f, GI %f", gain, cardiacBias, vascularBias, respiratoryBias, giBias)
			return nil
		})

}
