package da

import (
	"context"
	"fmt"

	"github.com/hovsep/fmesh-examples/simulation/life/autonomic"
	"github.com/hovsep/fmesh-examples/simulation/life/bloodstream"
	"github.com/hovsep/fmesh-examples/simulation/life/organism/human/controller"
	"github.com/hovsep/fmesh-examples/simulation/life/plugin/receptor"
	"github.com/hovsep/fmesh-examples/simulation/sim"
	"github.com/hovsep/fmesh-examples/simulation/sim/mathx"
	"github.com/hovsep/fmesh-examples/simulation/sim/simtime"
	"github.com/hovsep/fmesh/component"
	"github.com/hovsep/fmesh/signal"
)

// The circulation as a pump against a resistance.
//
// Three numbers, and the arithmetic that ties them together:
//
//	stroke volume  ← how full the heart is before it contracts (preload)
//	cardiac output = heart rate × stroke volume
//	pressure       = cardiac output × resistance + venous pressure
//
// That last line is why blood pressure is not a thing the body has so much as a
// thing it maintains: lose volume and stroke volume falls, and pressure holds
// only for as long as rate and resistance can be raised to cover it. Watching
// exactly that fail is the point of modelling it.
const (
	// NormalStrokeVolume is what a well-filled adult heart ejects per beat, mL.
	NormalStrokeVolume = 70.0

	// UnstressedVolume is the blood that fills the circulation without
	// stretching it, in litres. Only what is above this stretches the ventricle
	// and contributes to preload -- which is why the first litre lost costs
	// little and the second costs everything.
	UnstressedVolume = 2.0

	// NormalSVR is systemic vascular resistance in a resting adult, in
	// mmHg·min/L (Wood units). Arterioles set it, and the sympathetic system
	// sets them.
	//
	// Its value is not free: it is what makes the resting body an equilibrium.
	// At rest the autonomic bias sits at restingVascularTone, which asks the
	// heart for about 64 beats a minute; 64 beats of 70 mL is 4.5 L/min, and
	// 4.5 L/min has to come out at 93 mmHg for the baroreflex to have nothing to
	// correct. Get this wrong and the reflex spends its life fighting the body's
	// own resting state -- which is exactly what a first cut of this did, settling
	// the poor man at a pressure of 198.
	NormalSVR = (NormalMAP - CentralVenousPressure) / RestingCardiacOutput

	// RestingCardiacOutput is what the heart produces at resting tone, in L/min.
	RestingCardiacOutput = restingHeartRate * NormalStrokeVolume / 1000.0

	// restingHeartRate is what the cardiac bias asks for when nothing is wrong.
	// It follows from the heart's own rate mapping at restingVascularTone.
	restingHeartRate = 64.0

	// Effectors have inertia, and modelling them as instantaneous is what makes
	// a control loop ring. Arterioles take the longer of the two: a vessel
	// cannot be squeezed as fast as a heart can be sped up.
	vascularToneHalfLifeSec = 3.0

	// adrenalineVasoconstriction is how much fully saturated adrenaline adds to
	// vascular tone, on top of what the nerves are asking for.
	//
	// It is small, and smaller than it looks like it should be, because
	// circulating adrenaline is a poor vasoconstrictor: it tightens skin and gut
	// but opens working muscle, and the two nearly cancel. Constriction is the
	// nerves' job, and it arrives here as tone. This term was 0.15 for as long as
	// nothing was piped into the receptor that fed it -- it read 0.0 for the
	// component's whole life -- so the first run that actually used it put a
	// frightened body at a mean pressure of 150.
	adrenalineVasoconstriction = 0.05

	// CentralVenousPressure is the pressure blood returns at, mmHg. It is small
	// next to arterial pressure but is what the arithmetic sits on top of.
	CentralVenousPressure = 5.0

	// NormalMAP is mean arterial pressure in a healthy adult: the set point the
	// baroreflex defends, and roughly what 5 L/min through a normal resistance
	// produces.
	NormalMAP = 93.0

	// vasoconstrictionGain is how hard sympathetic tone can squeeze the
	// arterioles. At full tone resistance roughly doubles, which is the body's
	// main defence of pressure when volume is gone.
	vasoconstrictionGain = 2.0

	// MaxStrokeVolumeFactor caps how much a very full heart can compensate.
	// Starling's law flattens: a ventricle can only stretch so far.
	MaxStrokeVolumeFactor = 1.2

	// inotropyGain is how much fully saturated adrenaline adds to the force of
	// each contraction.
	//
	// Preload is not the only thing that sets stroke volume, and while it was the
	// only thing modelled here the number could not move: through a three-minute
	// effort that took the heart from 64 to 155 beats, stroke volume sat at
	// exactly 70.00 mL and every extra litre of output came from rate alone. A
	// working heart does not merely beat faster, it beats harder, and about a
	// third of the rise in cardiac output is this.
	inotropyGain = 0.35

	// maxMetabolicVasodilation is how far working muscle can open the
	// circulation, as a fraction of the resistance the nerves are asking for.
	//
	// This is not a reflex. Muscle that is working produces the metabolites that
	// relax the arterioles feeding it, and it does so regardless of what the
	// sympathetic system wants -- which is why the same sympathetic outflow that
	// makes a frightened body pale leaves a running one flushed.
	//
	// The value comes from the operating point rather than from taste. Hard work
	// at intensity 8 asks for something near 16 L/min, and a healthy adult
	// carries that at a mean pressure around 110-120 mmHg, so the circulation has
	// to be open to roughly (115-5)/16 ≈ 7 Wood units. Without this the vessels
	// bottomed out at ResistanceAt(0) = 13.75 and the same effort produced a mean
	// pressure of 154, which is not a hard workout but a hypertensive crisis.
	maxMetabolicVasodilation = 0.8
)

const (
	stateMAP           string = "map"
	stateCardiacOutput string = "cardiac_output"
	stateSVR           string = "svr"
	stateStrokeVolume  string = "stroke_volume"
	stateHeartRate     string = "heart_rate"
	stateVascularTone  string = "vascular_tone"
	stateBloodVolume   string = "blood_volume"
	stateVascDt        string = "dt"

	// stateExertion latches the most recent physical load. Like every other
	// non-tick input here it arrives on its own mesh cycle, and recomputeCirculation
	// runs once per arrival rather than once per tick, so it has to be read back
	// out of state rather than folded in where it lands.
	stateExertion string = "exertion"
)

// The set points the reflexes defend, re-exported here so the physiology package
// can compare against them without importing the bloodstream's whole vocabulary.
const (
	NormalPaCO2Reference = bloodstream.NormalPaCO2
	NormalPaO2Reference  = bloodstream.NormalPaO2
)

// restingVascularTone is the sympathetic bias the vessels sit at when nothing is
// wrong, and the point resistance is normal at.
const restingVascularTone = 0.15

// GetVasculature returns the circulation: the vessels the heart pumps into.
//
// Like da:blood_system it publishes before it integrates, so the components that
// read pressure -- the baroreflex above all -- see last tick's value and the loop
// between them cannot deadlock.
func GetVasculature() (*component.Component, error) {
	c, err := component.New("da:vasculature",
		component.WithDescription("Systemic circulation: stroke volume, cardiac output and arterial pressure"),
		component.WithInputs(
			sim.TimePort,
			"heart_rate",     // beats per minute, from the heart
			"autonomic_tone", // vascular bias sets the resistance
			"venous_blood",   // for the volume that fills the heart
			"physical_load",  // working muscle opens its own supply
		),
		component.WithOutputs(
			"map",            // mean arterial pressure, mmHg
			"cardiac_output", // L/min
			"svr",            // systemic vascular resistance
			"stroke_volume",  // mL per beat
		),
		component.WithPlugins(
			// Adrenaline constricts on its own account, which is why the
			// pressure a frightened body holds outlasts the nerve traffic that
			// started it.
			receptor.For(bloodstream.HormoneAdrenaline),
		),
		component.WithActivationFunc(circulate),
		component.WithInitialState(func(state component.State) {
			state.Set(stateMAP, NormalMAP)
			state.Set(stateCardiacOutput, 5.0)
			state.Set(stateSVR, NormalSVR)
			state.Set(stateStrokeVolume, NormalStrokeVolume)
			state.Set(stateHeartRate, 70.0)
			state.Set(stateVascularTone, restingVascularTone)
			state.Set(stateBloodVolume, bloodstream.NormalBloodVolume)
			state.Set(stateVascDt, defaultDt)
			state.Set(stateExertion, controller.RestingIntensity)
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("da:vasculature: %w", err)
	}
	return c, nil
}

func circulate(_ context.Context, this *component.Component) error {
	// Phase A: the tick publishes what the circulation was doing, so the
	// baroreflex has a pressure to react to without waiting on this tick's
	// heart rate -- which is the thing its own reaction will change.
	if this.InputByName(sim.TimePort).HasSignals() {
		if dt, err := simtime.TickDurationInSec(this.InputByName(sim.TimePort).Signals().First()); err == nil {
			this.State().Set(stateVascDt, dt)
		}
		publishCirculation(this)
		return nil
	}

	// Phase B: fold in whatever has arrived.
	if in := this.InputByName("heart_rate"); in.HasSignals() {
		if rate, err := in.Signals().First().As[int](); err == nil {
			this.State().Set(stateHeartRate, float64(rate))
		}
	}
	if in := this.InputByName("autonomic_tone"); in.HasSignals() {
		if target, err := autonomic.Bias(in.Signals().First(), autonomic.Vascular); err == nil {
			// Vessels follow the order they are given, they do not snap to it.
			dt := this.State().Get(stateVascDt).(float64)
			this.State().Update(stateVascularTone, func(current any) any {
				return mathx.DecayToward(current.(float64), target, dt, vascularToneHalfLifeSec)
			})
		}
	}
	if in := this.InputByName("venous_blood"); in.HasSignals() {
		if sig := in.Signals().First(); sig != nil {
			this.State().Set(stateBloodVolume,
				sig.Scalars().ValueOrDefault("volume_l", bloodstream.NormalBloodVolume))
		}
	}
	if in := this.InputByName("physical_load"); in.HasSignals() {
		if sig := in.Signals().First(); sig != nil {
			this.State().Set(stateExertion,
				sig.Scalars().ValueOrDefault(controller.ScalarIntensity, controller.RestingIntensity))
		}
	}

	recomputeCirculation(this)
	return nil
}

// recomputeCirculation works out what the circulation is now achieving.
func recomputeCirculation(this *component.Component) {
	volume := this.State().Get(stateBloodVolume).(float64)
	rate := this.State().Get(stateHeartRate).(float64)
	tone := this.State().Get(stateVascularTone).(float64)
	exertion := this.State().Get(stateExertion).(float64)
	adrenaline := receptor.Level(this, bloodstream.HormoneAdrenaline)

	strokeVolume := StrokeVolumeAt(volume, adrenaline)
	cardiacOutput := rate * strokeVolume / 1000.0 // mL/beat × beats/min → L/min

	// What the nerves are asking for, and what the working muscle does about it.
	svr := ResistanceAt(tone+adrenaline*adrenalineVasoconstriction) *
		MetabolicVasodilation(exertion)

	this.State().Set(stateStrokeVolume, strokeVolume)
	this.State().Set(stateCardiacOutput, cardiacOutput)
	this.State().Set(stateSVR, svr)
	this.State().Set(stateMAP, cardiacOutput*svr+CentralVenousPressure)
}

// StrokeVolumeAt returns how much the heart ejects per beat: what preload has
// put in the ventricle, and how hard the adrenaline in the blood makes it
// squeeze.
//
// Preload comes first, by Starling's law -- the fuller the ventricle, the harder
// it contracts, until it cannot stretch further. Contractility is a second,
// independent axis, and multiplies what preload delivered, which is why it is
// applied outside the MaxStrokeVolumeFactor clamp: that clamp is how far the
// muscle can be stretched, and it has nothing to say about how hard it then pulls.
//
// The inotropic effect is scaled by how full the ventricle is, because an empty
// one cannot be squeezed into ejecting what it does not contain. That is not a
// convenience: it is why a haemorrhaging patient with every gland wide open
// still loses stroke volume, and why adrenaline is not a treatment for
// hypovolaemia.
func StrokeVolumeAt(volumeL, adrenaline float64) float64 {
	filling := mathx.Clamp(
		(volumeL-UnstressedVolume)/(bloodstream.NormalBloodVolume-UnstressedVolume),
		0, MaxStrokeVolumeFactor)
	return NormalStrokeVolume * filling * Contractility(filling, adrenaline)
}

// Contractility is how much harder than baseline the ventricle is pulling, from
// 1.0 in a calm body upward.
func Contractility(filling, adrenaline float64) float64 {
	return 1 + inotropyGain*mathx.Clamp(adrenaline, 0, 1)*min(filling, 1)
}

// MetabolicVasodilation returns the fraction of the asked-for resistance that
// working muscle leaves standing: 1.0 at rest, falling as effort rises.
func MetabolicVasodilation(exertion float64) float64 {
	effort := mathx.Clamp(
		(exertion-controller.RestingIntensity)/(controller.MaxIntensity-controller.RestingIntensity), 0, 1)
	return 1 - maxMetabolicVasodilation*effort
}

// ResistanceAt returns systemic vascular resistance at a given sympathetic tone.
// Squeezing the arterioles is how the body defends its pressure when there is
// not enough blood to do it with flow.
func ResistanceAt(tone float64) float64 {
	return NormalSVR * (1 + vasoconstrictionGain*(tone-restingVascularTone))
}

func publishCirculation(this *component.Component) {
	get := func(key string) float64 { return this.State().Get(key).(float64) }

	this.OutputByName("map").PutSignals(
		signal.New(get(stateMAP)).WithLabel("category", "circulation"),
	)
	this.OutputByName("cardiac_output").PutPayloads(get(stateCardiacOutput))
	this.OutputByName("svr").PutPayloads(get(stateSVR))
	this.OutputByName("stroke_volume").PutPayloads(get(stateStrokeVolume))
}
