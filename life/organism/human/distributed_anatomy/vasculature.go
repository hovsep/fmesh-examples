package da

import (
	"fmt"

	"github.com/hovsep/fmesh-examples/life/bloodstream"
	"github.com/hovsep/fmesh-examples/life/common"
	"github.com/hovsep/fmesh-examples/life/helper"
	"github.com/hovsep/fmesh-examples/life/plugin/receptor"
	. "github.com/hovsep/fmesh-examples/life/unit"
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
	NormalStrokeVolume = 70.0 * Milliliter

	// UnstressedVolume is the blood that fills the circulation without
	// stretching it, in litres. Only what is above this stretches the ventricle
	// and contributes to preload -- which is why the first litre lost costs
	// little and the second costs everything.
	UnstressedVolume = 2.0 * Liter

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
	NormalSVR = (NormalMAP - CentralVenousPressure) / restingCardiacOutput

	// restingCardiacOutput is what the heart produces at resting tone, in L/min.
	restingCardiacOutput = restingHeartRate * NormalStrokeVolume / 1000.0

	// restingHeartRate is what the cardiac bias asks for when nothing is wrong.
	// It follows from the heart's own rate mapping at restingVascularTone.
	restingHeartRate = 64.0

	// Effectors have inertia, and modelling them as instantaneous is what makes
	// a control loop ring. Arterioles take the longer of the two: a vessel
	// cannot be squeezed as fast as a heart can be sped up.
	vascularToneHalfLifeSec = 3.0

	// adrenalineVasoconstriction is how much fully saturated adrenaline adds to
	// vascular tone, on top of what the nerves are asking for.
	adrenalineVasoconstriction = 0.15

	// CentralVenousPressure is the pressure blood returns at, mmHg. It is small
	// next to arterial pressure but is what the arithmetic sits on top of.
	CentralVenousPressure = 5.0 * MmHg

	// NormalMAP is mean arterial pressure in a healthy adult: the set point the
	// baroreflex defends, and roughly what 5 L/min through a normal resistance
	// produces.
	NormalMAP = 93.0 * MmHg

	// vasoconstrictionGain is how hard sympathetic tone can squeeze the
	// arterioles. At full tone resistance roughly doubles, which is the body's
	// main defence of pressure when volume is gone.
	vasoconstrictionGain = 2.0

	// MaxStrokeVolumeFactor caps how much a very full heart can compensate.
	// Starling's law flattens: a ventricle can only stretch so far.
	MaxStrokeVolumeFactor = 1.2
)

const (
	stateMAP           common.State = "map"
	stateCardiacOutput common.State = "cardiac_output"
	stateSVR           common.State = "svr"
	stateStrokeVolume  common.State = "stroke_volume"
	stateHeartRate     common.State = "heart_rate"
	stateVascularTone  common.State = "vascular_tone"
	stateBloodVolume   common.State = "blood_volume"
	stateVascDt        common.State = "dt"
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
			common.TimePort,
			"heart_rate",     // beats per minute, from the heart
			"autonomic_tone", // vascular bias sets the resistance
			"venous_blood",   // for the volume that fills the heart
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
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("da:vasculature: %w", err)
	}
	return c, nil
}

func circulate(this *component.Component) error {
	// Phase A: the tick publishes what the circulation was doing, so the
	// baroreflex has a pressure to react to without waiting on this tick's
	// heart rate -- which is the thing its own reaction will change.
	if this.InputByName(common.TimePort).HasSignals() {
		if dt, err := helper.TickDurationInSec(this.InputByName(common.TimePort).Signals().First()); err == nil {
			this.State().Set(stateVascDt, dt)
		}
		publishCirculation(this)
		return nil
	}

	// Phase B: fold in whatever has arrived.
	if in := this.InputByName("heart_rate"); in.HasSignals() {
		if rate, err := helper.AsInt(in.Signals().First()); err == nil {
			this.State().Set(stateHeartRate, float64(rate))
		}
	}
	if in := this.InputByName("autonomic_tone"); in.HasSignals() {
		if target, err := helper.GetBias(in.Signals().First(), common.Vascular); err == nil {
			// Vessels follow the order they are given, they do not snap to it.
			dt := this.State().Get(stateVascDt).(float64)
			this.State().Update(stateVascularTone, func(current any) any {
				return helper.DecayToward(current.(float64), target, dt, vascularToneHalfLifeSec)
			})
		}
	}
	if in := this.InputByName("venous_blood"); in.HasSignals() {
		if sig := in.Signals().First(); sig != nil {
			this.State().Set(stateBloodVolume,
				sig.Scalars().ValueOrDefault("volume_l", bloodstream.NormalBloodVolume))
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

	strokeVolume := StrokeVolumeAt(volume)
	cardiacOutput := rate * strokeVolume / 1000.0 // mL/beat × beats/min → L/min
	svr := ResistanceAt(tone + receptor.Level(this, bloodstream.HormoneAdrenaline)*adrenalineVasoconstriction)

	this.State().Set(stateStrokeVolume, strokeVolume)
	this.State().Set(stateCardiacOutput, cardiacOutput)
	this.State().Set(stateSVR, svr)
	this.State().Set(stateMAP, cardiacOutput*svr+CentralVenousPressure)
}

// StrokeVolumeAt returns how much the heart ejects per beat at a given blood
// volume, by Starling's law: the fuller the ventricle, the harder it contracts,
// until it cannot stretch further.
func StrokeVolumeAt(volumeL float64) float64 {
	filling := (volumeL - UnstressedVolume) / (bloodstream.NormalBloodVolume - UnstressedVolume)
	return NormalStrokeVolume * helper.Clamp(filling, 0, MaxStrokeVolumeFactor)
}

// ResistanceAt returns systemic vascular resistance at a given sympathetic tone.
// Squeezing the arterioles is how the body defends its pressure when there is
// not enough blood to do it with flow.
func ResistanceAt(tone float64) float64 {
	return NormalSVR * (1 + vasoconstrictionGain*(tone-restingVascularTone))
}

func publishCirculation(this *component.Component) {
	get := func(key common.State) float64 { return this.State().Get(key).(float64) }

	this.OutputByName("map").PutSignals(
		signal.New(get(stateMAP)).WithLabel("category", "circulation"),
	)
	this.OutputByName("cardiac_output").PutPayloads(get(stateCardiacOutput))
	this.OutputByName("svr").PutPayloads(get(stateSVR))
	this.OutputByName("stroke_volume").PutPayloads(get(stateStrokeVolume))
}
