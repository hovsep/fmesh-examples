package controller

import (
	"context"
	"fmt"

	"github.com/hovsep/fmesh-examples/simulation"
	"github.com/hovsep/fmesh-examples/simulation/command"
	"github.com/hovsep/fmesh-examples/simulation/simtime"
	"github.com/hovsep/fmesh/component"
	"github.com/hovsep/fmesh/meta"
	"github.com/hovsep/fmesh/signal"
)

// Intake command verbs. The namespace is fixed; the verb names the category.
const (
	VerbWater = "water"
	VerbFood  = "food"
)

// Intake controller state.
const (
	// Processes are the swallows and puffs currently in progress. Drinking,
	// eating and smoking each meter out over time, so several can overlap.
	StateProcesses string = "processes"

	// Lifetime totals of what was commanded (not yet what was delivered), useful
	// for observing and for tests.
	TotalWaterMl  string = "total_water_ml"
	TotalFoodKcal string = "total_food_kcal"
)

// Kinds delivered by intake processes; also the scalar names on intake_intent.
const (
	KindWaterMl  = "water_ml"
	KindFoodKcal = "food_kcal"
)

// Delivery rates. Because a process delivers at a fixed rate, a larger amount
// simply takes proportionally longer -- 500 mL takes ten times as long as 50 mL.
const (
	DrinkRateMlPerSec = 15.0 // a 500 mL glass takes ~33 s
	EatRateKcalPerSec = 3.0  // a 600 kcal meal takes ~3.5 min

)

// GetIntake returns the intake controller.
//
// It is the body's mouth: it turns commands like "intake:water 500ml",
// "intake:food 200kcal" into ingestion that plays out
// over time. It knows nothing about digestion -- boundary:ingestion and
// da:gi_tract decide what swallowing something actually does.
func GetIntake() (*component.Component, error) {
	c, err := component.New("controller:intake",
		component.WithDescription("Turns eating, drinking and smoking commands into ingestion metered over time"),
		component.WithInputs(simulation.TimePort, simulation.ControlPort),
		component.WithOutputs("intake_intent"),
		component.WithActivationFunc(component.Sequential(
			acceptIntakeCommands,
			meterIntake,
		)),
		component.WithInitialState(func(state component.State) {
			state.Set(StateProcesses, &simtime.ProcessSet{})
			state.Set(TotalWaterMl, 0.0)
			state.Set(TotalFoodKcal, 0.0)
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("controller:intake: %w", err)
	}
	return c, nil
}

// acceptIntakeCommands starts a metered process for each arriving command.
func acceptIntakeCommands(_ context.Context, this *component.Component) error {
	processes := this.State().Get(StateProcesses).(*simtime.ProcessSet)

	return command.ForEach(this, simulation.ControlPort, func(name string, args *meta.Scalars) error {
		switch command.Verb(name) {
		case VerbWater:
			ml := args.ValueOrDefault(KindWaterMl, 0)
			processes.Start(&simtime.Process{Kind: KindWaterMl, Remaining: ml, RatePerSec: DrinkRateMlPerSec})
			addTotal(this, TotalWaterMl, ml)
		case VerbFood:
			kcal := args.ValueOrDefault(KindFoodKcal, 0)
			processes.Start(&simtime.Process{Kind: KindFoodKcal, Remaining: kcal, RatePerSec: EatRateKcalPerSec})
			addTotal(this, TotalFoodKcal, kcal)
		default:
			this.Logger().Printf("intake category %q is not modelled yet, ignoring\n", command.Verb(name))
		}
		return nil
	})
}

func addTotal(this *component.Component, key string, amount float64) {
	if amount <= 0 {
		this.Logger().Printf("ignoring non-positive intake amount %v for %s\n", amount, key)
		return
	}
	this.State().Update(key, func(v any) any { return v.(float64) + amount })
}

// meterIntake delivers each in-progress process's portion for this tick and
// emits it as a single ingestion intent.
func meterIntake(_ context.Context, this *component.Component) error {
	tick := this.InputByName(simulation.TimePort).Signals().First()
	if tick == nil {
		return nil
	}

	dt, err := simtime.TickDurationInSec(tick)
	if err != nil {
		return fmt.Errorf("intake controller tick: %w", err)
	}

	delivered := this.State().Get(StateProcesses).(*simtime.ProcessSet).Advance(dt)
	if len(delivered) == 0 {
		return nil
	}

	return this.OutputByName("intake_intent").PutSignals(
		signal.New("intake_intent").
			WithLabel("category", "intake").
			WithScalar(KindWaterMl, delivered[KindWaterMl]).
			WithScalar(KindFoodKcal, delivered[KindFoodKcal]),
	)
}
