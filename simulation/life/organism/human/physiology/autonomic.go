package physiology

import (
	"context"
	"fmt"

	"github.com/hovsep/fmesh-examples/simulation/life/autonomic"
	"github.com/hovsep/fmesh-examples/simulation/life/organism/human/controller"
	da "github.com/hovsep/fmesh-examples/simulation/life/organism/human/distributed_anatomy"
	"github.com/hovsep/fmesh-examples/simulation/life/organism/human/organ"
	"github.com/hovsep/fmesh-examples/simulation/sim/mathx"
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

	// demandCardiacWeight is how much of a full effort reaches the heart.
	//
	// Chosen so that eightfold exertion asks for about 150 beats, which is what
	// hard work costs a healthy adult. Before this the heart heard nothing about
	// exertion at all: its bias came from the brain's drive and the baroreflex,
	// and a body sprinting had the pulse of a body reading a book.
	demandCardiacWeight = 0.7

	// arousalIsMilderThanEffort scales fright against exercise. A fright empties
	// the same glands, but a frightened body is not doing the work a running one
	// is, and its pulse should not read as though it were.
	arousalIsMilderThanEffort = 0.6

	// demandVascularWeight is how much of a full effort reaches the arterioles.
	//
	// The vessels used to hear nothing about demand at all, and the result was
	// backwards: a fright dropped systemic resistance from 19.6 to 13.8, because
	// the only thing the vascular bias could hear was a baroreflex withdrawing
	// tone from a pressure that had already risen. Sympathetic outflow constricts;
	// that it does so is the whole reason a frightened body goes pale.
	//
	// It is smaller than the cardiac weight because the answer to effort is
	// mostly flow rather than pressure, and because working muscle is dilating at
	// the same time (see da.MetabolicVasodilation). The two are deliberately in
	// different components: this one is the nerves, that one is the muscle's own
	// chemistry, and which of them wins is what tells exercise apart from fright.
	demandVascularWeight = 0.3
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
	stateLastMAP      string = "last_map"
	stateLastPaCO2    string = "last_paco2"
	stateLastPaO2     string = "last_pao2"
	stateLastExertion string = "last_exertion"
	stateLastArousal  string = "last_arousal"

	// stateCollapsed remembers whether the drive was already gone last time, so
	// the collapse is reported when it happens rather than for as long as it
	// lasts. Without it this component logged on every activation: an altitude
	// run produced thousands of identical lines a second, and the console's ring
	// buffer quietly evicted everything else in it.
	stateCollapsed string = "drive_collapsed"
)

// collapseTone is what the body is told when the brain has stopped telling it
// anything: no sympathetic drive, and every region at its unstimulated floor.
//
// The alternative -- publishing nothing at all, which is what this used to do --
// is worse than it sounds. Downstream effectors are not waiting for an
// instruction, they are holding the last one they were given, and several of
// them publish it again on every tick regardless of whether anything is still
// driving them. A body whose neural drive collapsed at 8848 m went on breathing
// at 27 a minute and circulating 4.27 L/min at a mean pressure of 86 mmHg for
// five simulated minutes, because those were the numbers in flight when the
// brain went quiet. Nothing was wrong with the arithmetic; there was simply
// nobody left to change it.
//
// So the collapse is an instruction like any other, and the body answers it: the
// heart falls toward its floor, the vessels relax, breathing slows and CO₂
// climbs. What kills the body is then a mechanism the simulation can show,
// rather than a screen that stops updating.
func collapseTone() *signal.Signal {
	return autonomic.Pack(
		0, 1, // no sympathetic drive; parasympathetic is what is left
		defaultAutonomicCoordinationNoise, 0, // no gain
		0, 0, 0, 0, // cardiac, vascular, respiratory, gi
	)
}

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
		component.WithInputs(
			"neural_drive", "map", "venous_blood",
			// What the body is being asked to do, and how alarmed it is about it.
			"physical_load", "mental_load",
		),
		component.WithOutputs("autonomic_tone"),
		component.WithActivationFunc(func(_ context.Context, this *component.Component) error {
			if in := this.InputByName("map"); in.HasSignals() {
				this.State().Set(stateLastMAP, in.Signals().First().Float64OrDefault(da.NormalMAP))
			}
			// Latched on arrival: neither load comes in on the tick, and a port
			// is drained before the next one arrives.
			if in := this.InputByName("physical_load"); in.HasSignals() {
				if sig := in.Signals().First(); sig != nil {
					this.State().Set(stateLastExertion,
						sig.Scalars().ValueOrDefault(controller.ScalarIntensity, controller.RestingIntensity))
				}
			}
			if in := this.InputByName("mental_load"); in.HasSignals() {
				if sig := in.Signals().First(); sig != nil {
					this.State().Set(stateLastArousal,
						sig.Scalars().ValueOrDefault(controller.ScalarArousal, 0))
				}
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

			neuralDrive := this.InputByName("neural_drive").Signals().First().Float64OrDefault(0.0)

			// The collapse is a state, so it is reported on the two edges rather
			// than for every cycle it lasts.
			collapsed := neuralDrive <= criticalNeuralDrive
			if was, _ := this.State().Get(stateCollapsed).(bool); collapsed != was {
				this.State().Set(stateCollapsed, collapsed)
				if collapsed {
					this.Logger().Println("neural drive has collapsed; the body is no longer being driven")
				} else {
					this.Logger().Println("neural drive has recovered")
				}
			}
			if collapsed {
				return this.OutputByName("autonomic_tone").PutSignals(collapseTone())
			}

			pressure, _ := this.State().Get(stateLastMAP).(float64)
			paCO2, _ := this.State().Get(stateLastPaCO2).(float64)
			paO2, _ := this.State().Get(stateLastPaO2).(float64)
			exertion, _ := this.State().Get(stateLastExertion).(float64)
			arousal, _ := this.State().Get(stateLastArousal).(float64)
			this.OutputByName("autonomic_tone").PutSignals(
				getAutonomicToneSignal(neuralDrive, pressure, paCO2, paO2,
					sympatheticDemand(exertion, arousal)))
			return nil
		}),
		component.WithInitialState(func(state component.State) {
			state.Set(stateLastMAP, da.NormalMAP)
			state.Set(stateLastPaCO2, da.NormalPaCO2Reference)
			state.Set(stateLastPaO2, da.NormalPaO2Reference)
			state.Set(stateLastExertion, controller.RestingIntensity)
			state.Set(stateLastArousal, 0.0)
			state.Set(stateCollapsed, false)
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
// sympatheticDemand is how hard the body is being asked to work, from nought at
// rest to one at full effort.
//
// Exercise and fright are one number here because the body answers them the same
// way -- the same nerves, the same glands -- and takes whichever is asking for
// more rather than adding them, so a frightened runner is not asked for twice
// what either alone would cost.
func sympatheticDemand(exertion, arousal float64) float64 {
	effort := mathx.Clamp(
		(exertion-controller.RestingIntensity)/(controller.MaxIntensity-controller.RestingIntensity), 0, 1)
	return max(effort, mathx.Clamp(arousal, 0, 1)*arousalIsMilderThanEffort)
}

func getAutonomicToneSignal(neuralDrive, meanArterialPressure, paCO2, paO2, demand float64) *signal.Signal {
	reflex := baroreflexResponse(meanArterialPressure)
	chemo := chemoreflexResponse(paCO2, paO2)

	// Sympathetic level rises with drive, and with a pressure that needs
	// defending.
	sym := mathx.Clamp(neuralDrive+reflex+demand, 0, 1)
	paraSym := mathx.Clamp(1.0-sym, 0.0, 1.0)
	gain := sym

	// Regional biases as a fraction of drive, each shifted by its own share of
	// the reflex, with a little variability left on top.
	base := neuralDrive * 0.5
	bias := func(weight float64) float64 {
		return mathx.Clamp(mathx.Jitter(base+reflex*weight, defaultRegionalBiasJitter), 0, 1)
	}

	// The heart hears the demand directly, which is the whole point of this
	// number reaching here: a running or frightened body needs its pulse to
	// answer, and the baroreflex alone was never going to say so.
	cardiac := mathx.Clamp(
		mathx.Jitter(base+reflex+demand*demandCardiacWeight, defaultRegionalBiasJitter), 0, 1)

	// Breathing answers to the blood far more than to anything else, so the
	// respiratory bias takes whichever of its two callers is asking for more.
	respiratory := mathx.Clamp(
		mathx.Jitter(max(base+reflex*respiratoryWeight, base+chemo), defaultRegionalBiasJitter), 0, 1)

	// The vessels hear the demand too, and squeeze. What the working muscle does
	// about that is the muscle's business, not the nerves' -- see
	// da.MetabolicVasodilation.
	vascular := mathx.Clamp(
		mathx.Jitter(base+reflex*vascularWeight+demand*demandVascularWeight, defaultRegionalBiasJitter), 0, 1)

	return autonomic.Pack(
		sym, paraSym, defaultAutonomicCoordinationNoise, gain,
		cardiac,        // cardiac: beat faster
		vascular,       // vascular: squeeze
		respiratory,    // respiratory: breathe harder, mostly for the CO₂
		bias(giWeight), // gut: give up its share
	)
}
