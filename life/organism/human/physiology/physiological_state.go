package physiology

import (
	"fmt"

	"github.com/hovsep/fmesh-examples/life/common"
	"github.com/hovsep/fmesh-examples/life/organism/human/organ"
	. "github.com/hovsep/fmesh-examples/life/unit"
	"github.com/hovsep/fmesh-examples/simulation/mathx"
	"github.com/hovsep/fmesh-examples/simulation/simtime"
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
	MaxGlycemia    = 300.0 // well into hyperglycaemia

	// MinGlycemia is a floor on the arithmetic, not a physiological limit, and
	// it sits well below the level at which a brain stops working (see
	// organ.glucoseFailLevel). It used to be 40, which was above it -- so the
	// brain's own starvation threshold could never be reached and hypoglycaemic
	// collapse was unreachable however long the body went without fuel.
	MinGlycemia = 10.0

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

	// glucoseShareOfExertion is how much of the *extra* fuel effort needs comes
	// out of the blood as sugar. The rest is fat and the muscle's own glycogen,
	// burned where they are stored without ever being blood glucose.
	//
	// It is well under one, and that is why an hour of exercise does not empty
	// the four grams of sugar the circulation holds. A body that drew all of its
	// exertion from blood glucose would go hypoglycaemic within minutes of
	// standing up, which is a mistake worth not making.
	glucoseShareOfExertion = 0.25

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

// GlucosePerKcal converts fuel between the two units the body keeps it in: kcal
// of stored reserve, and mg/dL of sugar dissolved in blood.
//
// It is derived from the liver's basal output rather than chosen, so that a
// resting body's glucose use is exactly what its liver supplies. Pick the two
// numbers independently and blood sugar creeps up or down forever at rest --
// slowly enough to look like physiology and not like the arithmetic error it
// would be. The same trick sets the resting vascular resistance.
var GlucosePerKcal = organ.BasalHepaticGlucoseOutput / RestingBurnKcalPerSec

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
			"absorption",      // gains from the gut
			"losses",          // water leaving through skin and kidneys
			"physical_load",   // current exertion, which sets the burn rate
			"thermal",         // heating/cooling rate from the skin (ambient + sun)
			"hepatic_glucose", // the liver's net release into the blood, mg/dL per second
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
		dt, err := simtime.TickDurationInSec(this.InputByName(common.TimePort).Signals().First())
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
	applyHepaticGlucose(this)
	applyThermal(this)
	return nil
}

// applyHepaticGlucose moves fuel between the reserve and the blood at whatever
// rate the liver has settled on.
//
// This is the only route by which stored fuel becomes blood sugar, and the only
// one under hormonal control. Everything else in the body spends sugar; this
// puts it back. Because the same number debits the reserve and credits the
// blood, fuel is conserved across the transfer whichever way it is going -- and
// a liver storing sugar after a meal is simply this running with the sign
// reversed.
func applyHepaticGlucose(this *component.Component) {
	in := this.InputByName("hepatic_glucose")
	if !in.HasSignals() {
		return
	}

	dt := this.State().Get(StateDt).(float64)
	delta := signal.AsFloat64OrDefault(in.Signals().First(), 0) * dt
	if delta == 0 {
		return
	}

	// A liver cannot release fuel the body does not have. Without this the
	// reserve simply clamps at zero while sugar goes on appearing in the blood,
	// and a starving body is kept comfortable by glucose made out of nothing.
	// Capped, starvation ends the way it really does: the glycogen runs out
	// first, and the hypoglycaemia follows.
	if delta > 0 {
		available := max(this.State().Get(StateEnergyKcal).(float64), 0) * GlucosePerKcal
		if delta = min(delta, available); delta <= 0 {
			return
		}
	}

	this.State().Update(StateGlycemia, func(v any) any {
		return mathx.Clamp(v.(float64)+delta, MinGlycemia, MaxGlycemia)
	})
	this.State().Update(StateEnergyKcal, func(v any) any {
		return mathx.Clamp(v.(float64)-delta/GlucosePerKcal, 0, MaxEnergyKcal)
	})
}

// applyThermal folds the skin's environmental heating/cooling rate into the core
// temperature. It is the disturbance the homeostatic pull in applyExertion works
// against, so mild weather barely moves the core while extremes overwhelm it.
func applyThermal(this *component.Component) {
	in := this.InputByName("thermal")
	if !in.HasSignals() {
		return
	}

	rate := signal.AsFloat64OrDefault(in.Signals().First(), 0)
	dt := this.State().Get(StateDt).(float64)
	this.State().Update(StateCoreTemperature, func(v any) any {
		return mathx.Clamp(v.(float64)+rate*dt, MinCoreTemperature, MaxCoreTemperature)
	})
}

func publishBodyState(this *component.Component) error {
	hydrationPct := mathx.Clamp(this.State().Get(StateHydrationMl).(float64)/TotalBodyWaterMl*100, 0, 100)
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

		// Absorbed food arrives in the blood, and only in the blood. Getting into
		// storage from there is the liver's job and insulin's decision, which is
		// the whole reason both exist.
		//
		// It used to arrive in both at once -- the reserve was credited here and
		// credited again when insulin put the same sugar away -- so a meal was
		// worth roughly twice its calories, and blood sugar could be regulated by
		// a hormone that had nothing left to regulate. One number, one place.
		kcal := sig.Scalars().ValueOrDefault(common.GlucoseKcal, 0)
		this.State().Update(StateGlycemia, func(v any) any {
			return mathx.Clamp(v.(float64)+kcal*GlucosePerKcal, MinGlycemia, MaxGlycemia)
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

// applyExertion burns fuel at a rate set by what the body is doing, and warms it
// in proportion.
//
// Fuel leaves storage by two routes and this splits them, because they behave
// differently and the difference matters. Some is burned where it lies -- fat in
// the muscle that needs it -- and simply disappears from the reserve. The rest is
// taken out of the blood as sugar, which the liver must then replace, and it is
// only that second route that anything regulates. Blood sugar is a small tank in
// the middle of a large flow, which is why it is defended so hard.
func applyExertion(this *component.Component) {
	in := this.InputByName("physical_load")
	if !in.HasSignals() {
		return
	}

	intensity := signal.AsFloat64OrDefault(in.Signals().First(), 1.0)
	dt := this.State().Get(StateDt).(float64)
	burnt := RestingBurnKcalPerSec * intensity * dt

	// What the tissues take out of the blood. At rest it is the whole burn; the
	// extra that effort adds is mostly met from elsewhere.
	glucoseUsed := organ.BasalHepaticGlucoseOutput *
		(1 + glucoseShareOfExertion*(intensity-1)) * dt

	this.State().Update(StateGlycemia, func(v any) any {
		return mathx.Clamp(v.(float64)-glucoseUsed, MinGlycemia, MaxGlycemia)
	})

	// Whatever the burn needed that the blood did not supply comes straight out
	// of reserve. The blood's share is debited when the liver replaces it, so
	// counting it here as well would spend the same fuel twice.
	if direct := burnt - glucoseUsed/GlucosePerKcal; direct > 0 {
		this.State().Update(StateEnergyKcal, func(v any) any {
			return mathx.Clamp(v.(float64)-direct, 0, MaxEnergyKcal)
		})
	}

	// Exertion above rest adds heat; the body sheds it toward normal regardless.
	this.State().Update(StateCoreTemperature, func(v any) any {
		heated := v.(float64) + (intensity-1.0)*heatPerIntensityUnitPerSec*dt
		return mathx.Clamp(
			mathx.DecayToward(heated, NormalCoreTemperature, dt, temperatureHalfLifeSec),
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
		return mathx.Clamp(v.(float64)+deltaMl, 0, TotalBodyWaterMl*1.1)
	})
}
