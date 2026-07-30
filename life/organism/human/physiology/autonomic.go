package physiology

import (
	"fmt"

	"github.com/hovsep/fmesh-examples/life/autonomic"
	da "github.com/hovsep/fmesh-examples/life/organism/human/distributed_anatomy"
	"github.com/hovsep/fmesh-examples/life/organism/human/organ"
	"github.com/hovsep/fmesh-examples/simulation/mathx"
	"github.com/hovsep/fmesh/component"
	"github.com/hovsep/fmesh/signal"
)

const (
	criticalNeuralDrive               = organ.MaxNeuralDrive * 0.01
	defaultAutonomicCoordinationNoise = 0.05
	defaultRegionalBiasJitter         = 0.05
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
	return mathx.Clamp(baroreflexGain*err, minBaroreflexResponse, maxBaroreflexResponse)
}

// The chemoreflex: what actually decides how hard a body breathes.
//
// Ventilation is governed by carbon dioxide, not by oxygen. Central
// chemoreceptors in the brainstem read the pH that PaCO₂ sets and adjust
// breathing to hold it near 40 mmHg, and they do so with a gain that makes a few
// mmHg of retained CO₂ feel unbearable. Oxygen only joins in late: the peripheral
// receptors stay quiet until PaO₂ falls below about 60 mmHg, by which point
// saturation is already off the shoulder of the curve.
//
// That asymmetry is the reason breath-holding is limited by the urge to breathe
// rather than by hypoxia, and the reason hyperventilating before a dive is
// dangerous: it lowers the CO₂ that would have made you surface without adding
// any oxygen worth having.
const (
	// carbonDioxideGain converts a fractional deviation from the set point into
	// respiratory drive. It is steep on purpose.
	carbonDioxideGain = 2.5

	// hypoxicOnset is where the peripheral receptors begin to contribute, and
	// hypoxicFull where they are giving everything they have (mmHg).
	hypoxicOnset = 60.0
	hypoxicFull  = 35.0

	maxChemoreflexResponse = 0.85
	minChemoreflexResponse = -0.15
)

// chemoreflexResponse returns the respiratory drive called for by the blood.
func chemoreflexResponse(paCO2, paO2 float64) float64 {
	carbonDioxide := carbonDioxideGain * (paCO2 - da.NormalPaCO2Reference) / da.NormalPaCO2Reference

	// Hypoxia contributes nothing until it is severe, and then a great deal.
	hypoxic := mathx.Clamp((hypoxicOnset-paO2)/(hypoxicOnset-hypoxicFull), 0, 1)

	return mathx.Clamp(max(carbonDioxide, hypoxic), minChemoreflexResponse, maxChemoreflexResponse)
}

// stateLastMAP latches the arterial pressure the reflex is answering, and the
// blood gases the chemoreflex is answering.
const (
	stateLastMAP   string = "last_map"
	stateLastPaCO2 string = "last_paco2"
	stateLastPaO2  string = "last_pao2"
)

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
		component.WithInputs("neural_drive", "map", "venous_blood"),
		component.WithOutputs("autonomic_tone"),
		component.WithActivationFunc(func(this *component.Component) error {
			if in := this.InputByName("map"); in.HasSignals() {
				this.State().Set(stateLastMAP, signal.AsFloat64OrDefault(in.Signals().First(), da.NormalMAP))
			}
			if in := this.InputByName("venous_blood"); in.HasSignals() {
				if sig := in.Signals().First(); sig != nil {
					this.State().Set(stateLastPaCO2, sig.Scalars().ValueOrDefault("PaCO2", da.NormalPaCO2Reference))
					this.State().Set(stateLastPaO2, sig.Scalars().ValueOrDefault("PaO2", da.NormalPaO2Reference))
				}
			}

			if !this.InputByName("neural_drive").HasSignals() {
				return nil
			}

			neuralDrive := signal.AsFloat64OrDefault(this.InputByName("neural_drive").Signals().First(), 0.0)

			if neuralDrive <= criticalNeuralDrive {
				this.Logger().Println("Neural drive too low")
				return nil
			}

			pressure, _ := this.State().Get(stateLastMAP).(float64)
			paCO2, _ := this.State().Get(stateLastPaCO2).(float64)
			paO2, _ := this.State().Get(stateLastPaO2).(float64)
			this.OutputByName("autonomic_tone").PutSignals(
				getAutonomicToneSignal(neuralDrive, pressure, paCO2, paO2))
			return nil
		}),
		component.WithInitialState(func(state component.State) {
			state.Set(stateLastMAP, da.NormalMAP)
			state.Set(stateLastPaCO2, da.NormalPaCO2Reference)
			state.Set(stateLastPaO2, da.NormalPaO2Reference)
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
func getAutonomicToneSignal(neuralDrive, meanArterialPressure, paCO2, paO2 float64) *signal.Signal {
	reflex := baroreflexResponse(meanArterialPressure)
	chemo := chemoreflexResponse(paCO2, paO2)

	// Sympathetic level rises with drive, and with a pressure that needs
	// defending.
	sym := mathx.Clamp(neuralDrive+reflex, 0, 1)
	paraSym := mathx.Clamp(1.0-sym, 0.0, 1.0)
	gain := sym

	// Regional biases as a fraction of drive, each shifted by its own share of
	// the reflex, with a little variability left on top.
	base := neuralDrive * 0.5
	bias := func(weight float64) float64 {
		return mathx.Clamp(mathx.Jitter(base+reflex*weight, defaultRegionalBiasJitter), 0, 1)
	}

	// Breathing answers to the blood far more than to anything else, so the
	// respiratory bias takes whichever of its two callers is asking for more.
	respiratory := mathx.Clamp(
		mathx.Jitter(max(base+reflex*respiratoryWeight, base+chemo), defaultRegionalBiasJitter), 0, 1)

	return autonomic.Pack(
		sym, paraSym, defaultAutonomicCoordinationNoise, gain,
		bias(1.0),            // cardiac: beat faster
		bias(vascularWeight), // vascular: squeeze
		respiratory,          // respiratory: breathe harder, mostly for the CO₂
		bias(giWeight),       // gut: give up its share
	)
}
