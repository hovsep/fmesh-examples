package organ

import (
	"fmt"
	"math"

	"github.com/hovsep/fmesh-examples/life/bloodstream"
	"github.com/hovsep/fmesh-examples/life/common"
	"github.com/hovsep/fmesh-examples/life/helper"
	"github.com/hovsep/fmesh-examples/life/plugin/damage"
	"github.com/hovsep/fmesh-examples/life/plugin/perfusion"
	"github.com/hovsep/fmesh-examples/life/plugin/receptor"
	. "github.com/hovsep/fmesh-examples/life/unit"
	"github.com/hovsep/fmesh-examples/simulation/mathx"
	"github.com/hovsep/fmesh/component"
)

const (
	minBPM float64 = 40 * PerMinute
	maxBPM float64 = 200 * PerMinute

	// HeartO2PerMinute is the myocardium's resting oxygen demand in mL/min. The
	// heart never rests and pays for it: it extracts far more of the oxygen
	// passing through it than any other organ, so when supply falls it has
	// almost no reserve left to draw on.
	HeartO2PerMinute = 30.0

	// cardiacRateHalfLifeSec is how quickly the heart follows a change in the
	// rate it is being asked for -- a few beats, not instantly.
	cardiacRateHalfLifeSec = 1.5

	// adrenalineChronotropy is how much fully saturated adrenaline adds to the
	// cardiac bias -- worth roughly another 50 beats a minute on top of what the
	// nerves are asking for.
	adrenalineChronotropy = 0.3

	// stateRateExact holds the unrounded rate the smoothing works on.
	stateRateExact common.State = "rate_exact"
)

// cardiacActivationWave returns ECG-style contraction amplitude for a given phase
// @TODO: check what is going on with ECG diagram, it looks like we fake it in tui, so maybe it makes no sense to generate it there
func cardiacActivationWave(phase float64) float64 {
	if phase < 0.05 {
		return math.Exp(-30 * phase) // R-Peak (Spike)
	}
	if phase >= 0.05 && phase < 0.1 {
		return -0.2 * math.Sin((phase-0.05)*20) // Simple S-Wave dip
	}
	return 0
}

// GetHeart returns heart component
func GetHeart() (*component.Component, error) {
	c, err := component.New("organ:heart",
		component.WithDescription("Heart"),
		component.WithPlugins(
			damage.New(damage.Config{Organ: "heart"}),
			perfusion.New(perfusion.Config{Organ: "heart", O2PerMinute: HeartO2PerMinute}),
			// A heart feels adrenaline directly, which is why fright quickens
			// it faster than any nerve could and why the quickening outlasts
			// the moment that caused it.
			receptor.For(bloodstream.HormoneAdrenaline),
		),
		component.WithInputs("time", "autonomic_tone"), // @TODO: now it looks like heart can only beat, let's make it more interesting and connect to other components and add more effects that can increase\decrease heartbeat or damage
		component.WithOutputs("cardiac_activation", "rate"),
		component.WithActivationFunc(
			damage.FlatlineWhenFailed(
				oscillateHeart,
				handleCardiacBias,
			),
		),
		component.WithInitialState(func(state component.State) {
			state.Set(common.Rate, 60) // Initial BPM
			state.Set(stateRateExact, 60.0)
			state.Set(common.Phase, 0.0) // Phase in the current heartbeat cycle
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("organ:heart: %w", err)
	}
	return c, nil
}

func oscillateHeart(this *component.Component) error {
	if !this.InputByName("time").HasSignals() {
		return nil
	}

	dt, err := helper.TickDurationInSec(this.InputByName("time").Signals().First())
	if err != nil {
		return err
	}

	// Advance phase
	var nextPhase float64
	this.State().Update(common.Phase, func(old any) any {
		currentPhase := old.(float64)
		currentRate := this.State().Get(common.Rate).(int)
		phaseStep := dt / (60.0 / float64(currentRate))
		nextPhase = math.Mod(currentPhase+phaseStep, 1.0)
		return nextPhase
	})

	// Compute cardiac activation
	act := cardiacActivationWave(nextPhase)
	this.OutputByName("cardiac_activation").PutPayloads(act)
	return nil
}

func handleCardiacBias(this *component.Component) error {
	if !this.InputByName("autonomic_tone").HasSignals() {
		return nil
	}

	bias, err := helper.GetBias(this.InputByName("autonomic_tone").Signals().First(), common.Cardiac)
	if err != nil {
		return err
	}

	// A heart takes a few beats to change its mind. Following the demanded rate
	// rather than snapping to it is both what a real heart does and what keeps
	// the reflex that commands it from ringing.
	dt := 0.01
	if in := this.InputByName("time"); in.HasSignals() {
		if d, err := helper.TickDurationInSec(in.Signals().First()); err == nil {
			dt = d
		}
	}

	// The exact rate is kept as a real number and only rounded when reported.
	// Rounding it into the stored value instead would be a silent brake: a tick
	// moves the rate by a fraction of a beat, and truncating that to a whole one
	// discards it, so the heart would sit at its resting rate for ever however
	// hard the reflex called for tachycardia.
	// Circulating adrenaline adds to whatever the nerves are asking for.
	adrenaline := receptor.Level(this, bloodstream.HormoneAdrenaline)
	demanded := mathx.Lerp(minBPM, maxBPM, mathx.Clamp(bias+adrenaline*adrenalineChronotropy, 0, 1))
	this.State().Update(stateRateExact, func(v any) any {
		return mathx.DecayToward(v.(float64), demanded, dt, cardiacRateHalfLifeSec)
	})
	this.State().Set(common.Rate, int(math.Round(this.State().Get(stateRateExact).(float64))))
	this.OutputByName("rate").PutPayloads(this.State().Get(common.Rate).(int))
	return nil
}
