package controller

import (
	"fmt"

	"github.com/hovsep/fmesh-examples/life/common"
	"github.com/hovsep/fmesh-examples/life/helper"
	. "github.com/hovsep/fmesh-examples/life/unit"
	"github.com/hovsep/fmesh/component"
	"github.com/hovsep/fmesh/meta"
	"github.com/hovsep/fmesh/signal"
)

// Trauma command verbs.
const (
	VerbBleed = "bleed"
)

// Scalars a trauma command carries.
const (
	ScalarVolumeMl = "ml"
)

// Trauma controller state.
const (
	// PendingBleedMl is blood asked for but not yet taken, in mL. It is a
	// running total rather than a flag: two wounds bleed twice.
	PendingBleedMl common.State = "pending_bleed_ml"

	// TotalBledMl is what the body has lost over the run, for observation.
	TotalBledMl common.State = "total_bled_ml"
)

// BleedRateMlPerSec is how fast blood actually leaves once a wound is opened.
//
// Injuries are not instantaneous, and the difference matters: a body that loses
// a litre over a minute has time to answer, and a body that loses it in one tick
// does not. Roughly a brisk arterial bleed.
const BleedRateMlPerSec = 25.0 * Milliliter

// GetTrauma returns the controller that turns injuries into physiology.
//
// Like every other controller it is a pure command surface: it says how much
// blood is leaving, and leaves it to the circulation to work out what that
// costs. That separation is what lets one command -- trauma:bleed -- reach all
// the way through to a hormone without anything in between knowing about
// injuries.
func GetTrauma() (*component.Component, error) {
	c, err := component.New("controller:trauma",
		component.WithDescription("Turns injuries into blood loss"),
		component.WithInputs(common.TimePort, common.ControlPort),
		component.WithOutputs("blood_loss"),
		component.WithActivationFunc(helper.SequentialActivationFunc(
			acceptTraumaCommands,
			emitBloodLoss,
		)),
		component.WithInitialState(func(state component.State) {
			state.Set(PendingBleedMl, 0.0)
			state.Set(TotalBledMl, 0.0)
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("controller:trauma: %w", err)
	}
	return c, nil
}

func acceptTraumaCommands(this *component.Component) error {
	return helper.ForEachCommand(this, common.ControlPort, func(name string, args *meta.Scalars) error {
		switch helper.CommandVerb(name) {
		case VerbBleed:
			volume := args.ValueOrDefault(ScalarVolumeMl, 0)
			if volume <= 0 {
				return nil
			}
			this.State().Update(PendingBleedMl, func(v any) any {
				return v.(float64) + volume
			})
			this.Logger().Printf("bleeding %.0f mL", volume)
		}
		return nil
	})
}

// emitBloodLoss lets out this tick's share of whatever is still bleeding.
func emitBloodLoss(this *component.Component) error {
	if !this.InputByName(common.TimePort).HasSignals() {
		return nil
	}

	pending := this.State().Get(PendingBleedMl).(float64)
	if pending <= 0 {
		return nil
	}

	dt, err := helper.TickDurationInSec(this.InputByName(common.TimePort).Signals().First())
	if err != nil {
		return err
	}

	lost := min(BleedRateMlPerSec*dt, pending)
	this.State().Set(PendingBleedMl, pending-lost)
	this.State().Update(TotalBledMl, func(v any) any { return v.(float64) + lost })

	return this.OutputByName("blood_loss").PutSignals(
		signal.New(lost).WithLabel("category", "trauma"),
	)
}
