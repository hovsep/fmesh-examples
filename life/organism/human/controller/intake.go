package controller

import (
	"fmt"

	"github.com/hovsep/fmesh-examples/life/common"
	"github.com/hovsep/fmesh-examples/life/helper"
	"github.com/hovsep/fmesh/component"
	"github.com/hovsep/fmesh/meta"
	"github.com/hovsep/fmesh/signal"
)

// Intake command verbs. The namespace is fixed ("intake"); the verb names the
// category, and unknown categories are carried through as generic substances so
// new ones need no code change here.
const (
	VerbWater = "water"
	VerbFood  = "food"
)

// Intake controller state.
const (
	// Pending amounts accumulate between ticks, because several commands can
	// arrive in the same tick and they should reach the body as one swallow.
	PendingWaterMl  common.State = "pending_water_ml"
	PendingFoodKcal common.State = "pending_food_kcal"

	// Lifetime totals, useful for observing and for tests.
	TotalWaterMl  common.State = "total_water_ml"
	TotalFoodKcal common.State = "total_food_kcal"
)

// Scalar names carried on the intake_intent signal.
const (
	ScalarWaterMl  = "water_ml"
	ScalarFoodKcal = "food_kcal"
)

// GetIntake returns the intake controller.
//
// It is the body's mouth: it turns commands like "intake:water 500ml" or
// "intake:food 200kcal" into a single ingestion intent per tick. It deliberately
// knows nothing about digestion -- boundary:ingestion and da:gi_tract decide what
// swallowing something actually does.
func GetIntake() (*component.Component, error) {
	c, err := component.New("controller:intake",
		component.WithDescription("Turns eating and drinking commands into ingestion intent"),
		component.WithInputs(common.TimePort, common.ControlPort),
		component.WithOutputs("intake_intent"),
		component.WithActivationFunc(helper.SequentialActivationFunc(
			acceptIntakeCommands,
			emitIntakeIntent,
		)),
		component.WithInitialState(func(state component.State) {
			state.Set(PendingWaterMl, 0.0)
			state.Set(PendingFoodKcal, 0.0)
			state.Set(TotalWaterMl, 0.0)
			state.Set(TotalFoodKcal, 0.0)
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("controller:intake: %w", err)
	}
	return c, nil
}

// acceptIntakeCommands folds arriving commands into the pending swallow.
func acceptIntakeCommands(this *component.Component) error {
	return helper.ForEachCommand(this, common.ControlPort, func(name string, args *meta.Scalars) error {
		switch helper.CommandVerb(name) {
		case VerbWater:
			addPending(this, PendingWaterMl, TotalWaterMl, args.ValueOrDefault(ScalarWaterMl, 0))
		case VerbFood:
			addPending(this, PendingFoodKcal, TotalFoodKcal, args.ValueOrDefault(ScalarFoodKcal, 0))
		default:
			// Arbitrary categories ("intake:vitamin_c") are accepted but have no
			// physiology behind them yet, so say so rather than failing silently.
			this.Logger().Printf("intake category %q is not modelled yet, ignoring\n", helper.CommandVerb(name))
		}
		return nil
	})
}

func addPending(this *component.Component, pendingKey, totalKey common.State, amount float64) {
	if amount <= 0 {
		this.Logger().Printf("ignoring non-positive intake amount %v for %s\n", amount, pendingKey)
		return
	}
	this.State().Update(pendingKey, func(v any) any { return v.(float64) + amount })
	this.State().Update(totalKey, func(v any) any { return v.(float64) + amount })
}

// emitIntakeIntent releases the pending swallow, once per tick.
//
// Gating on the tick (rather than emitting straight from the command handler)
// keeps the controller in step with the rest of the body and means a burst of
// commands in one tick produces one intent rather than several.
func emitIntakeIntent(this *component.Component) error {
	if !this.InputByName(common.TimePort).HasSignals() {
		return nil
	}

	water := this.State().Get(PendingWaterMl).(float64)
	food := this.State().Get(PendingFoodKcal).(float64)
	if water == 0 && food == 0 {
		return nil
	}

	this.State().Set(PendingWaterMl, 0.0)
	this.State().Set(PendingFoodKcal, 0.0)

	return this.OutputByName("intake_intent").PutSignals(
		signal.New("intake_intent").
			WithLabel("category", "intake").
			WithScalar(ScalarWaterMl, water).
			WithScalar(ScalarFoodKcal, food),
	)
}
