package physiology

import (
	"fmt"

	"github.com/hovsep/fmesh-examples/life/common"
	"github.com/hovsep/fmesh-examples/life/helper"
	da "github.com/hovsep/fmesh-examples/life/organism/human/distributed_anatomy"
	"github.com/hovsep/fmesh-examples/life/organism/human/organ"
	. "github.com/hovsep/fmesh-examples/life/unit"
	"github.com/hovsep/fmesh/component"
	"github.com/hovsep/fmesh/signal"
)

const (
	criticalNeuralDrive               = organ.MaxNeuralDrive * 0.01 * DNCS
	defaultAutonomicCoordinationNoise = 0.05 * DNCS
	defaultRegionalBiasJitter         = 0.05 * Proportion
)

// The baroreflex: the fastest control loop in the body.
//
// Stretch receptors in the carotid sinus and aortic arch report arterial
// pressure; the brainstem compares it against a set point and answers within a
// beat or two. It is what stops you fainting when you stand up, and it is what
// holds a bleeding patient's pressure normal until it cannot.
//
// The answer is not uniform, and that is the interesting part: the same falling
// pressure speeds the heart, tightens the vessels and shuts the gut down. Blood
// is taken from where it can be spared and given to where it cannot.
const (
	// baroreflexGain converts a fractional pressure error into sympathetic
	// drive. It is deliberately brisk: a 30% fall in pressure should produce
	// most of the response the body has.
	baroreflexGain = 1.5

	// The response is bounded. Even a maximal reflex cannot double the heart
	// rate indefinitely, and a pressure that is too high can only be answered by
	// withdrawing tone, not by reversing it.
	maxBaroreflexResponse = 0.5
	minBaroreflexResponse = -0.3

	// Regional weights: how much of the response each bed receives. The gut's is
	// negative because it is what gets sacrificed -- splanchnic vasoconstriction
	// is where the body finds the blood it redistributes.
	vascularWeight    = 1.0
	respiratoryWeight = 0.4
	giWeight          = -0.8
)

// baroreflexResponse returns the sympathetic drive called for by the difference
// between the pressure the body wants and the pressure it has.
func baroreflexResponse(meanArterialPressure float64) float64 {
	if meanArterialPressure <= 0 {
		return maxBaroreflexResponse
	}
	err := (da.NormalMAP - meanArterialPressure) / da.NormalMAP
	return helper.Clamp(baroreflexGain*err, minBaroreflexResponse, maxBaroreflexResponse)
}

// stateLastMAP latches the arterial pressure the reflex is answering.
const stateLastMAP common.State = "last_map"

// GetAutonomicCoordination ...
func GetAutonomicCoordination() (*component.Component, error) {
	c, err := component.New("physiology:autonomic_coordination",
		component.WithDescription("Autonomic coordination system"),
		// No time input on purpose: this component is driven entirely by the
		// brain, and the tick is fanned out to every component that declares one
		// (see human_component.go sense). Declaring a port it never reads made it
		// activate on the tick alone, a cycle before neural_drive could arrive,
		// and report the brain missing on every single tick.
		//
		// Pressure is the exception: it arrives from the circulation on its own
		// cycle and is latched, so a reflex answering last tick's pressure is
		// still answering a pressure that was real.
		component.WithInputs("neural_drive", "map"),
		component.WithOutputs("autonomic_tone"),
		component.WithActivationFunc(func(this *component.Component) error {
			if in := this.InputByName("map"); in.HasSignals() {
				this.State().Set(stateLastMAP, helper.AsF64OrDefault(in.Signals().First(), da.NormalMAP))
			}

			if !this.InputByName("neural_drive").HasSignals() {
				return nil
			}

			neuralDrive := helper.AsF64OrDefault(this.InputByName("neural_drive").Signals().First(), 0.0)

			if neuralDrive <= criticalNeuralDrive {
				this.Logger().Println("Neural drive too low")
				return nil
			}

			pressure, _ := this.State().Get(stateLastMAP).(float64)
			this.OutputByName("autonomic_tone").PutSignals(getAutonomicToneSignal(neuralDrive, pressure))
			return nil
		}),
		component.WithInitialState(func(state component.State) {
			state.Set(stateLastMAP, da.NormalMAP)
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("physiology:autonomic_coordination: %w", err)
	}
	return c, nil
}

// getAutonomicToneSignal turns the brain's drive, corrected by the baroreflex,
// into the tone each region of the body receives.
//
// The regional biases used to be one number with four different jitters on it,
// which meant they could never express anything: the heart, the vessels, the
// airway and the gut all did the same thing at once. Under a pressure error they
// now diverge, which is what the autonomic system is for.
func getAutonomicToneSignal(neuralDrive, meanArterialPressure float64) *signal.Signal {
	reflex := baroreflexResponse(meanArterialPressure)

	// Sympathetic level rises with drive, and with a pressure that needs
	// defending.
	sym := helper.Clamp(neuralDrive+reflex, 0, 1)
	paraSym := helper.Clamp(1.0-sym, 0.0, 1.0)
	gain := sym

	// Regional biases as a fraction of drive, each shifted by its own share of
	// the reflex, with a little variability left on top.
	base := neuralDrive * 0.5
	bias := func(weight float64) float64 {
		return helper.Clamp(helper.Jitter(base+reflex*weight, defaultRegionalBiasJitter), 0, 1)
	}

	return helper.PackAutonomicTone(
		sym, paraSym, defaultAutonomicCoordinationNoise, gain,
		bias(1.0),               // cardiac: beat faster
		bias(vascularWeight),    // vascular: squeeze
		bias(respiratoryWeight), // respiratory: breathe harder
		bias(giWeight),          // gut: give up its share
	)
}
