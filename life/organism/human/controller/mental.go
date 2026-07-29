package controller

import (
	"fmt"

	"github.com/hovsep/fmesh-examples/life/common"
	"github.com/hovsep/fmesh-examples/life/helper"
	. "github.com/hovsep/fmesh-examples/life/unit"
	"github.com/hovsep/fmesh-examples/simulation/mathx"
	"github.com/hovsep/fmesh/component"
	"github.com/hovsep/fmesh/meta"
	"github.com/hovsep/fmesh/signal"
)

// Mental stimulus command verb. "emotion:stimulus" applies a one-off emotional
// event; its lasting effect is what this controller models.
const VerbStimulus = "stimulus"

// Mental controller state.
const (
	// Arousal is how activated the mind is, 0 (calm) to 1 (panic). It decays
	// back toward calm on its own.
	Arousal common.State = "arousal"
	// Valence is how pleasant the current mood is, -1 (dread) to +1 (joy). It
	// decays back toward neutral.
	Valence common.State = "valence"
)

// Scalar names on the stimulus command and the emitted mental load signal.
const (
	ScalarArousal = "arousal"
	ScalarValence = "valence"
)

const (
	// A stimulus fades over minutes rather than instantly, which is what makes a
	// fright still felt a little while later.
	arousalHalfLifeSec = 120.0
	valenceHalfLifeSec = 300.0

	maxArousal = 1.0 * Proportion
	minValence = -1.0 * Proportion
	maxValence = 1.0 * Proportion
)

// GetMental returns the mental stress controller.
//
// It holds the emotional weather: a stimulus pushes arousal and valence, and both
// relax back toward calm neutrality as simulated time passes.
func GetMental() (*component.Component, error) {
	c, err := component.New("controller:mental_stress",
		component.WithDescription("Turns emotional stimuli into a decaying mental load"),
		component.WithInputs(common.TimePort, common.ControlPort),
		component.WithOutputs("mental_load"),
		component.WithActivationFunc(helper.SequentialActivationFunc(
			acceptStimulusCommands,
			emitMentalLoad,
		)),
		component.WithInitialState(func(state component.State) {
			state.Set(Arousal, 0.0)
			state.Set(Valence, 0.0)
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("controller:mental_stress: %w", err)
	}
	return c, nil
}

func acceptStimulusCommands(this *component.Component) error {
	return helper.ForEachCommand(this, common.ControlPort, func(name string, args *meta.Scalars) error {
		if helper.CommandVerb(name) != VerbStimulus {
			this.Logger().Printf("unknown emotion command %q\n", name)
			return nil
		}

		// Stimuli add to the current mood rather than replacing it, so two
		// frights land harder than one.
		this.State().Update(Arousal, func(v any) any {
			return mathx.Clamp(v.(float64)+args.ValueOrDefault(ScalarArousal, 0), 0, maxArousal)
		})
		this.State().Update(Valence, func(v any) any {
			return mathx.Clamp(v.(float64)+args.ValueOrDefault(ScalarValence, 0), minValence, maxValence)
		})
		return nil
	})
}

func emitMentalLoad(this *component.Component) error {
	tick := this.InputByName(common.TimePort).Signals().First()
	if tick == nil {
		return nil
	}

	dt, err := helper.TickDurationInSec(tick)
	if err != nil {
		return fmt.Errorf("mental controller tick: %w", err)
	}

	arousal := mathx.DecayToward(this.State().Get(Arousal).(float64), 0, dt, arousalHalfLifeSec)
	valence := mathx.DecayToward(this.State().Get(Valence).(float64), 0, dt, valenceHalfLifeSec)
	this.State().Set(Arousal, arousal)
	this.State().Set(Valence, valence)

	return this.OutputByName("mental_load").PutSignals(
		signal.New(arousal).
			WithLabel("category", "load").
			WithScalar(ScalarArousal, arousal).
			WithScalar(ScalarValence, valence),
	)
}
