package organ

import (
	"fmt"

	"github.com/hovsep/fmesh-examples/life/common"
	"github.com/hovsep/fmesh-examples/life/helper"
	"github.com/hovsep/fmesh-examples/life/plugin/damage"
	"github.com/hovsep/fmesh-examples/life/plugin/perfusion"
	. "github.com/hovsep/fmesh-examples/life/unit"
	"github.com/hovsep/fmesh-examples/simulation/mathx"
	"github.com/hovsep/fmesh/component"
	"github.com/hovsep/fmesh/signal"
)

// Kidney state.
const (
	StateBladderMl common.State = "bladder_ml"
	// StateHydrationPct is the last hydration reading the kidney saw. It arrives
	// on its own mesh cycle, so the kidney remembers it rather than pausing urine
	// production on ticks where the signal has not landed.
	StateHydrationPct common.State = "hydration_pct"
)

const (
	// BladderCapacityMl is where the urge becomes impossible to ignore.
	BladderCapacityMl = 500.0 * Milliliter

	// BaseUrineMlPerSec is a normal output: about 1.5 L a day.
	BaseUrineMlPerSec = 1500.0 / 86400.0 * Milliliter

	// Urine production tracks hydration. Well-hydrated bodies pass more; a dry
	// one concentrates and passes far less, which is what makes dehydration
	// something the body defends against rather than simply suffers.
	fullyHydratedPct      = 99.5
	dehydratedPct         = 96.0
	maxDiuresisFactor     = 4.0
	minAntidiuresisFactor = 0.15
)

const KidneyO2PerMinute = 18.0

// GetKidney returns the kidney and its bladder.
//
// It decides how much water to shed based on how hydrated the body is, fills the
// bladder with it, and empties on command.
func GetKidney() (*component.Component, error) {
	c, err := component.New("organ:kidney",
		component.WithDescription("Kidney: sheds or conserves water according to hydration, and fills the bladder"),
		component.WithPlugins(
			damage.New(damage.Config{Organ: "kidney"}),
			// The kidneys take a fifth of the cardiac output to filter with, but
			// burn only a little of the oxygen that passes through them.
			perfusion.New(perfusion.Config{Organ: "kidney", O2PerMinute: KidneyO2PerMinute}),
		),
		component.WithInputs(
			common.TimePort,
			"body_state", // from physiology:physiological_state
			"void",       // from controller:excretion
		),
		component.WithOutputs(
			"losses", // water leaving the body, to physiology:physiological_state
			"bladder_fill",
			"urine_rate",
		),
		component.WithActivationFunc(
			//@TODO: I do not like how we call FlatlineWhenFailed from some plugin
			// let's maybe make FlatlineWhenFailed more general and have it as a separate plugin or find a better solution. Plugins must be only plugged in, we should not call any code from plugin package.
			// Maybe damage plugin must register pre-activation hook and do the check there?
			damage.FlatlineWhenFailed(
				readHydration,
				voidBladder,
				produceUrine,
			)),
		component.WithInitialState(func(state component.State) {
			state.Set(StateBladderMl, 0.0)
			state.Set(StateHydrationPct, 100.0)
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("organ:kidney: %w", err)
	}
	return c, nil
}

func readHydration(this *component.Component) error {
	in := this.InputByName("body_state")
	if !in.HasSignals() {
		return nil
	}
	if sig := in.Signals().First(); sig != nil {
		this.State().Set(StateHydrationPct, sig.Scalars().ValueOrDefault(common.HydrationPct, 100.0))
	}
	return nil
}

func voidBladder(this *component.Component) error {
	if this.InputByName("void").HasSignals() {
		this.State().Set(StateBladderMl, 0.0)
	}
	return nil
}

func produceUrine(this *component.Component) error {
	tick := this.InputByName(common.TimePort).Signals().First()
	if tick == nil {
		return nil
	}

	dt, err := helper.TickDurationInSec(tick)
	if err != nil {
		return fmt.Errorf("kidney tick: %w", err)
	}

	hydration := this.State().Get(StateHydrationPct).(float64)
	rate := BaseUrineMlPerSec * diuresisFactor(hydration)
	produced := rate * dt

	this.State().Update(StateBladderMl, func(v any) any {
		// A full bladder does not stop the kidneys; it simply cannot hold more,
		// so production above capacity is dropped rather than tracked.
		return mathx.Clamp(v.(float64)+produced, 0, BladderCapacityMl)
	})

	if err := this.OutputByName("urine_rate").PutPayloads(rate); err != nil {
		return err
	}
	if err := this.OutputByName("bladder_fill").PutPayloads(
		this.State().Get(StateBladderMl).(float64) / BladderCapacityMl * 100,
	); err != nil {
		return err
	}

	// Water in the bladder has left the body's usable pool, so it is reported as
	// a loss the moment it is produced rather than when it is voided.
	return this.OutputByName("losses").PutSignals(
		signal.New("losses").
			WithLabel("category", "kidney").
			WithScalar(common.WaterMl, produced),
	)
}

// diuresisFactor scales urine output by hydration, between concentrating hard
// when dry and flushing freely when full.
func diuresisFactor(hydrationPct float64) float64 {
	t := (hydrationPct - dehydratedPct) / (fullyHydratedPct - dehydratedPct)
	return mathx.Lerp(minAntidiuresisFactor, maxDiuresisFactor, mathx.Clamp(t, 0, 1))
}
