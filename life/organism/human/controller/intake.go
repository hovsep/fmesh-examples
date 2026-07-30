package controller

import (
	"fmt"

	"github.com/hovsep/fmesh-examples/life/common"
	"github.com/hovsep/fmesh-examples/simulation/command"
	"github.com/hovsep/fmesh-examples/simulation/mathx"
	"github.com/hovsep/fmesh-examples/simulation/simtime"
	"github.com/hovsep/fmesh/component"
	"github.com/hovsep/fmesh/meta"
	"github.com/hovsep/fmesh/signal"
)

// Intake command verbs. The namespace is fixed; the verb names the category.
const (
	VerbWater     = "water"
	VerbFood      = "food"
	VerbCigarette = "cigarette" // reached via the "smoke" namespace
)

// Intake controller state.
const (
	// Processes are the swallows and puffs currently in progress. Drinking,
	// eating and smoking each meter out over time, so several can overlap.
	StateProcesses common.State = "processes"

	// Lifetime totals of what was commanded (not yet what was delivered), useful
	// for observing and for tests.
	TotalWaterMl    common.State = "total_water_ml"
	TotalFoodKcal   common.State = "total_food_kcal"
	TotalCigarettes common.State = "total_cigarettes"
)

// Kinds delivered by intake processes; also the scalar names on intake_intent.
const (
	KindWaterMl  = "water_ml"
	KindFoodKcal = "food_kcal"
	KindToxin    = "toxin"
)

// Delivery rates. Because a process delivers at a fixed rate, a larger amount
// simply takes proportionally longer -- 500 mL takes ten times as long as 50 mL.
const (
	DrinkRateMlPerSec = 15.0 // a 500 mL glass takes ~33 s
	EatRateKcalPerSec = 3.0  // a 600 kcal meal takes ~3.5 min

	// A cigarette delivers its toxin over a randomised 5-10 minutes. The toxin is
	// measured in lung-damage units (0..1 to fail an organ): one cigarette does a
	// small fraction, so it takes many hundreds to wreck a lung -- harmful over a
	// long habit, not instantly fatal.
	cigaretteMeanDurationSec = 7.5 * 60.0
	cigaretteDurationJitter  = 33.0   // percent, giving roughly 5-10 minutes
	ToxinPerCigarette        = 0.0015 // ~650 cigarettes to fail a lung
)

// GetIntake returns the intake controller.
//
// It is the body's mouth: it turns commands like "intake:water 500ml",
// "intake:food 200kcal" and "smoke:cigarette 1" into ingestion that plays out
// over time. It knows nothing about digestion -- boundary:ingestion and
// da:gi_tract decide what swallowing something actually does.
func GetIntake() (*component.Component, error) {
	c, err := component.New("controller:intake",
		component.WithDescription("Turns eating, drinking and smoking commands into ingestion metered over time"),
		component.WithInputs(common.TimePort, common.ControlPort),
		component.WithOutputs("intake_intent"),
		component.WithActivationFunc(component.Sequential(
			acceptIntakeCommands,
			meterIntake,
		)),
		component.WithInitialState(func(state component.State) {
			state.Set(StateProcesses, &simtime.ProcessSet{})
			state.Set(TotalWaterMl, 0.0)
			state.Set(TotalFoodKcal, 0.0)
			state.Set(TotalCigarettes, 0.0)
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("controller:intake: %w", err)
	}
	return c, nil
}

// acceptIntakeCommands starts a metered process for each arriving command.
func acceptIntakeCommands(this *component.Component) error {
	processes := this.State().Get(StateProcesses).(*simtime.ProcessSet)

	return command.ForEach(this, common.ControlPort, func(name string, args *meta.Scalars) error {
		switch command.Verb(name) {
		case VerbWater:
			ml := args.ValueOrDefault(KindWaterMl, 0)
			processes.Start(&simtime.Process{Kind: KindWaterMl, Remaining: ml, RatePerSec: DrinkRateMlPerSec})
			addTotal(this, TotalWaterMl, ml)
		case VerbFood:
			kcal := args.ValueOrDefault(KindFoodKcal, 0)
			processes.Start(&simtime.Process{Kind: KindFoodKcal, Remaining: kcal, RatePerSec: EatRateKcalPerSec})
			addTotal(this, TotalFoodKcal, kcal)
		case VerbCigarette:
			count := args.ValueOrDefault("count", 1)
			// One puff-stream: `count` cigarettes' worth of toxin metered at the
			// pace of a single cigarette, so more cigarettes simply take longer.
			// The duration is randomised so no two are identical.
			durationSec := mathx.Jitter(cigaretteMeanDurationSec, cigaretteDurationJitter)
			processes.Start(&simtime.Process{
				Kind:       KindToxin,
				Remaining:  count * ToxinPerCigarette,
				RatePerSec: ToxinPerCigarette / durationSec,
			})
			addTotal(this, TotalCigarettes, count)
		default:
			this.Logger().Printf("intake category %q is not modelled yet, ignoring\n", command.Verb(name))
		}
		return nil
	})
}

func addTotal(this *component.Component, key common.State, amount float64) {
	if amount <= 0 {
		this.Logger().Printf("ignoring non-positive intake amount %v for %s\n", amount, key)
		return
	}
	this.State().Update(key, func(v any) any { return v.(float64) + amount })
}

// meterIntake delivers each in-progress process's portion for this tick and
// emits it as a single ingestion intent.
func meterIntake(this *component.Component) error {
	tick := this.InputByName(common.TimePort).Signals().First()
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
			WithScalar(KindFoodKcal, delivered[KindFoodKcal]).
			WithScalar(KindToxin, delivered[KindToxin]),
	)
}
