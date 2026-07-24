package physiology

import (
	"fmt"

	"github.com/hovsep/fmesh-examples/life/common"
	"github.com/hovsep/fmesh-examples/life/helper"
	. "github.com/hovsep/fmesh-examples/life/unit"
	"github.com/hovsep/fmesh/component"
	"github.com/hovsep/fmesh/signal"
)

// Reservoir levels, held as component state.
const (
	StateHydrationMl     common.State = "hydration_ml"
	StateGlycemia        common.State = "glycemia"
	StateEnergyKcal      common.State = "energy_kcal"
	StateCoreTemperature common.State = "core_temperature"
	StateDt              common.State = "dt"
)

// Reference values for a resting adult.
const (
	// TotalBodyWaterMl is the water a fully hydrated body holds. Hydration is
	// reported as a percentage of it, which keeps "thirsty" meaningful without
	// anyone having to know the absolute figure.
	TotalBodyWaterMl = 42000.0 * Milliliter

	NormalGlycemia = 90.0  // mg/dL, fasting
	MinGlycemia    = 40.0  // hypoglycaemia; below this consciousness suffers
	MaxGlycemia    = 300.0 // well into hyperglycaemia

	// StartingEnergyKcal is roughly a day's worth of readily usable reserve.
	StartingEnergyKcal = 2000.0
	MaxEnergyKcal      = 4000.0

	NormalCoreTemperature = 37.0 * Celsius
	MinCoreTemperature    = 30.0 * Celsius
	MaxCoreTemperature    = 43.0 * Celsius
)

const (
	// RestingBurnKcalPerSec is basal metabolism: about 1700 kcal a day.
	RestingBurnKcalPerSec = 1700.0 / 86400.0

	// glucoseDrawPerKcal converts energy moving in or out of reserve into a
	// change in blood glucose.
	glucoseDrawPerKcal = 0.35

	// glucoseRestoreHalfLifeSec is how fast blood glucose is pulled back toward
	// fasting level. This stands in for a liver until there is one.
	glucoseRestoreHalfLifeSec = 900.0

	// Exertion produces heat, and the body sheds it toward normal at rest.
	//
	// The two constants set the equilibrium together: sustained effort settles at
	// roughly heatPerIntensityUnitPerSec * (intensity-1) * halfLife / ln2 above
	// normal. At intensity 8 that is about +2.4 C, so hard exercise reaches ~39.4
	// C and stays there. Getting this wrong is not subtle -- an earlier value ten
	// times larger cooked the body to 43 C during a twenty-minute run.
	heatPerIntensityUnitPerSec = 0.0004 * Celsius
	temperatureHalfLifeSec     = 600.0
)

// GetPhysiologicalState returns the body's internal reservoirs: how hydrated,
// how fuelled and how warm it is.
//
// It is the single owner of those numbers. Organs contribute per-tick gains and
// losses and read the levels back off the body_state signal, rather than each
// keeping its own idea of how much water is left.
//
// Like da:blood_system it publishes before it integrates, so components
// downstream see this tick's values without waiting for contributions that only
// arrive later in the same run.
func GetPhysiologicalState() (*component.Component, error) {
	c, err := component.New("physiology:physiological_state",
		component.WithDescription("Internal physiological state (hydration, glycemia, energy, core temperature)"),
		component.WithInputs(
			common.TimePort,
			"absorption",    // gains from the gut
			"losses",        // water leaving through skin and kidneys
			"physical_load", // current exertion, which sets the burn rate
		),
		component.WithOutputs(
			"body_state", // composite, broadcast to everything that needs to know
			"hydration",
			"glycemia",
			"energy",
			"body_temperature",
		),
		component.WithActivationFunc(updatePhysiologicalState),
		component.WithInitialState(func(state component.State) {
			state.Set(StateHydrationMl, TotalBodyWaterMl)
			state.Set(StateGlycemia, NormalGlycemia)
			state.Set(StateEnergyKcal, StartingEnergyKcal)
			state.Set(StateCoreTemperature, NormalCoreTemperature)
			state.Set(StateDt, 0.01)
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("physiology:physiological_state: %w", err)
	}
	return c, nil
}

func updatePhysiologicalState(this *component.Component) error {
	// Phase A: the tick publishes current levels straight away, so organs reading
	// body_state are never a run behind.
	if this.InputByName(common.TimePort).HasSignals() {
		dt, err := helper.TickDurationInSec(this.InputByName(common.TimePort).Signals().First())
		if err != nil {
			return err
		}
		this.State().Set(StateDt, dt)
		return publishBodyState(this)
	}

	// Phase B: fold in whatever has arrived. Contributions reach this component
	// across several mesh cycles (the gut is four hops from the mouth), so each
	// is absorbed as it lands rather than waiting for all of them together.
	applyAbsorption(this)
	applyLosses(this)
	applyExertion(this)
	return nil
}

func publishBodyState(this *component.Component) error {
	hydrationPct := helper.Clamp(this.State().Get(StateHydrationMl).(float64)/TotalBodyWaterMl*100, 0, 100)
	glycemia := this.State().Get(StateGlycemia).(float64)
	energy := this.State().Get(StateEnergyKcal).(float64)
	temperature := this.State().Get(StateCoreTemperature).(float64)

	if err := this.OutputByName("body_state").PutSignals(
		signal.New("body_state").
			WithLabel("category", "physiology").
			WithScalar(common.HydrationPct, hydrationPct).
			WithScalar(common.Glycemia, glycemia).
			WithScalar(common.EnergyKcal, energy).
			WithScalar(common.CoreTemperature, temperature),
	); err != nil {
		return err
	}

	// Plain numbers alongside the composite, for observation and rendering.
	if err := this.OutputByName("hydration").PutPayloads(hydrationPct); err != nil {
		return err
	}
	if err := this.OutputByName("glycemia").PutPayloads(glycemia); err != nil {
		return err
	}
	if err := this.OutputByName("energy").PutPayloads(energy); err != nil {
		return err
	}
	return this.OutputByName("body_temperature").PutPayloads(temperature)
}

func applyAbsorption(this *component.Component) {
	in := this.InputByName("absorption")
	if !in.HasSignals() {
		return
	}

	_ = in.Signals().ForEach(func(sig *signal.Signal) error {
		addHydration(this, sig.Scalars().ValueOrDefault(common.WaterMl, 0))

		kcal := sig.Scalars().ValueOrDefault(common.GlucoseKcal, 0)
		this.State().Update(StateEnergyKcal, func(v any) any {
			return helper.Clamp(v.(float64)+kcal, 0, MaxEnergyKcal)
		})
		// Food reaches the blood before it becomes reserve, which is what makes
		// eating feel different from simply having eaten.
		this.State().Update(StateGlycemia, func(v any) any {
			return helper.Clamp(v.(float64)+kcal*glucoseDrawPerKcal, MinGlycemia, MaxGlycemia)
		})
		return nil
	})
}

func applyLosses(this *component.Component) {
	in := this.InputByName("losses")
	if !in.HasSignals() {
		return
	}

	_ = in.Signals().ForEach(func(sig *signal.Signal) error {
		addHydration(this, -sig.Scalars().ValueOrDefault(common.WaterMl, 0))
		return nil
	})
}

// applyExertion burns energy at a rate set by what the body is doing, and warms
// it in proportion.
func applyExertion(this *component.Component) {
	in := this.InputByName("physical_load")
	if !in.HasSignals() {
		return
	}

	intensity := helper.AsF64OrDefault(in.Signals().First(), 1.0)
	dt := this.State().Get(StateDt).(float64)
	burnt := RestingBurnKcalPerSec * intensity * dt

	this.State().Update(StateEnergyKcal, func(v any) any {
		return helper.Clamp(v.(float64)-burnt, 0, MaxEnergyKcal)
	})

	// Glucose is drawn down by the burn and topped back up toward fasting level,
	// so exertion dips it and rest recovers it.
	this.State().Update(StateGlycemia, func(v any) any {
		drawn := v.(float64) - burnt*glucoseDrawPerKcal
		return helper.Clamp(
			helper.DecayToward(drawn, NormalGlycemia, dt, glucoseRestoreHalfLifeSec),
			MinGlycemia, MaxGlycemia)
	})

	// Exertion above rest adds heat; the body sheds it toward normal regardless.
	this.State().Update(StateCoreTemperature, func(v any) any {
		heated := v.(float64) + (intensity-1.0)*heatPerIntensityUnitPerSec*dt
		return helper.Clamp(
			helper.DecayToward(heated, NormalCoreTemperature, dt, temperatureHalfLifeSec),
			MinCoreTemperature, MaxCoreTemperature)
	})
}

func addHydration(this *component.Component, deltaMl float64) {
	if deltaMl == 0 {
		return
	}
	this.State().Update(StateHydrationMl, func(v any) any {
		// A little above full is possible right after drinking, before the
		// kidneys catch up.
		return helper.Clamp(v.(float64)+deltaMl, 0, TotalBodyWaterMl*1.1)
	})
}
