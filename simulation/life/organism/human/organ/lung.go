package organ

import (
	"context"
	"fmt"
	"math"

	"github.com/hovsep/fmesh-examples/simulation/life/atmosphere"
	"github.com/hovsep/fmesh-examples/simulation/life/bloodstream"
	"github.com/hovsep/fmesh-examples/simulation/life/body"
	"github.com/hovsep/fmesh-examples/simulation/life/plugin/damage"
	"github.com/hovsep/fmesh-examples/simulation/sim/mathx"
	"github.com/hovsep/fmesh-examples/simulation/sim/simtime"
	"github.com/hovsep/fmesh/component"
	"github.com/hovsep/fmesh/signal"
)

const (
	restingLungVolume = 700.0

	ResidualLungVolume = 600.0
	TotalLungCapacity  = 3000.0

	defaultLungCompliance   = 100.0
	defaultAirwayResistance = 0.002

	lungVolumeAsymmetry     = 5
	lungComplianceAsymmetry = 5
	lungResistanceAsymmetry = 5

	pleuralPressureAsymmetryBase = 0.3
	pleuralPressureAsymmetry     = 30

	baseO2Consumption     = 7 // fraction of inspired O2 consumed at rest
	o2ConsumptionMaxDelta = 5 // additional O2 consumption when blood is O2-depleted

	// ExhaledCO2AtNormalPaCO2 is the carbon dioxide fraction of exhaled air when
	// the blood is at a normal PaCO₂ -- room air carries almost none, breath
	// carries about a twentieth.
	ExhaledCO2AtNormalPaCO2 = 5.0

	// Below are per-instance lung params that makes left and
	// right lungs slightly different (anatomically, the left one has less space due to the heart).
	statePleuralAsymmetry string = "pleural_asymmetry"
	stateCompliance       string = "compliance"
	stateVolume           string = "volume"
	stateResistance       string = "resistance"
)

var (
	// FRC is Functional Residual Capacity (equilibrium volume at the end of passive expiration).
	// FRC = V₀ + C·|BasePleuralPressure| = 700 + 100·5 = 1200 mL.
	FRC = restingLungVolume + defaultLungCompliance*math.Abs(BasePleuralPressure)
)

func GetLung(side string) (*component.Component, error) {
	c, err := component.New("organ:lung_"+string(side),
		component.WithDescription(string(side)+" lung"),
		component.WithPlugins(damage.New(damage.Config{Organ: "lung_" + string(side)})),
		component.WithInputs("time", "pleural_pressure", "inspired_gas", "venous_blood"),
		component.WithOutputs(
			"volume", "flow", "alveolar_pressure", "exhaled_gas", "alveolar_gas", "alveolar_po2",
			"carbon_monoxide", // whatever the air was carrying, handed to the blood
		),
		component.WithActivationFunc(component.When(damage.Working,
			component.Sequential(
				handleMechanics,
				handleGasExchange,
			))),
		component.WithInitialState(func(state component.State) {
			state.Set(stateVolume, mathx.Jitter(FRC, lungVolumeAsymmetry)) // start at equilibrium
			state.Set(stateCompliance, mathx.Jitter(defaultLungCompliance, lungComplianceAsymmetry))
			state.Set(stateResistance, mathx.Jitter(defaultAirwayResistance, lungResistanceAsymmetry))
			state.Set(statePleuralAsymmetry, mathx.Jitter(pleuralPressureAsymmetryBase, pleuralPressureAsymmetry))
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("organ:lung_%s: %w", side, err)
	}
	return c, nil
}

func handleMechanics(_ context.Context, this *component.Component) error {
	// A breathing cycle is driven by the tick. If there is no tick, this activation
	// came from some other input arriving out of phase (e.g. an inhaled toxin on
	// the damage port); do nothing rather than keep waiting for a tick that has
	// already passed, which would stall the mesh.
	if !this.InputByName("time").HasSignals() {
		return nil
	}
	if !this.Inputs().ByNames("time", "pleural_pressure", "inspired_gas").AllHaveSignals() {
		return component.ErrWaitKeepingInputs
	}

	dt, err := simtime.TickDurationInSec(this.InputByName("time").Signals().First())
	if err != nil {
		return err
	}

	// More than one thing may be driving the chest -- a diaphragm, a machine, or
	// a failed diaphragm sitting at a constant pressure while a machine works.
	// The lungs follow whichever is pulling hardest, which is what lets a
	// ventilator take over from a muscle that has stopped without either of them
	// needing to know about the other.
	pp, err := strongestInspiratoryEffort(this.InputByName("pleural_pressure").Signals())
	if err != nil {
		return err
	}
	pleuralPressure := pp + this.State().Get(statePleuralAsymmetry).(float64)

	V := this.State().Get(stateVolume).(float64)
	C := this.State().Get(stateCompliance).(float64)
	R := this.State().Get(stateResistance).(float64)

	alveolarPressure := pleuralPressure + (V-restingLungVolume)/C
	flow := -alveolarPressure / R
	Vnext := mathx.ClampAndLogAnomaly(V+flow*dt, ResidualLungVolume, TotalLungCapacity, this.Logger(), "lung volume")

	this.State().Set(stateVolume, Vnext)

	this.OutputByName("volume").PutPayloads(Vnext)
	this.OutputByName("flow").PutPayloads(flow)
	this.OutputByName("alveolar_pressure").PutPayloads(alveolarPressure)

	return nil
}

func handleGasExchange(_ context.Context, this *component.Component) error {
	if !this.Inputs().ByNames("inspired_gas").AllHaveSignals() {
		return nil
	}

	gas := this.InputByName("inspired_gas").Signals().First()
	n, o, a, _, _, _, err := atmosphere.Unpack(gas)
	if err != nil {
		return err
	}

	flowSig := this.OutputByName("flow").Signals().First()
	if flowSig == nil {
		return nil
	}
	flow, err := signal.AsFloat64(flowSig)
	if err != nil {
		return err
	}

	dt, err := simtime.TickDurationInSec(this.InputByName("time").Signals().First())
	if err != nil {
		return err
	}

	tickVolume := math.Abs(flow) * dt
	if tickVolume <= 0 {
		return nil
	}

	var bloodCO2, bloodO2 float64
	if bloodSig := this.InputByName("venous_blood").Signals().First(); bloodSig != nil {
		bloodCO2 = bloodSig.Scalars().ValueOrDefault("PaCO2", 0)
		bloodO2 = bloodSig.Scalars().ValueOrDefault("PaO2", 0)
	}

	// Carbon monoxide crosses here like anything else, and the lung is the only
	// place it can: it is in the air, and this is where air meets blood. How much
	// crosses depends on how foul the air is and on how much of it is being moved,
	// which is why exertion in a contaminated space is so much worse than rest.
	if ppm := atmosphere.CarbonMonoxide(gas); ppm > 0 {
		if err := this.OutputByName("carbon_monoxide").PutSignals(
			bloodstream.Secretion(bloodstream.SubstanceCOLoad,
				bloodstream.COUptakePerPpmPerMl*ppm*tickVolume),
		); err != nil {
			return err
		}
	}

	// What the alveoli are actually offering the blood.
	//
	// This is the lung's own business and nowhere else's: it is the only
	// component that holds both the air arriving (how much of it there is, and
	// what fraction is oxygen) and the blood arriving (how much carbon dioxide it
	// brought). The blood used to assume a fixed 104 mmHg, which meant the body
	// breathed the same air on a mountain as in a diving bell.
	if err := publishAlveolarPO2(this, gas, o, bloodCO2); err != nil {
		return err
	}

	// Blood that arrives short of oxygen takes more of it out of the air, so the
	// exhaled fraction falls as the deficit grows.
	o2Deficit := mathx.Clamp((bloodstream.NormalPaO2-bloodO2)/bloodstream.NormalPaO2, 0, 1)
	o2Consumed := baseO2Consumption + o2ConsumptionMaxDelta*o2Deficit

	o2New := o - o2Consumed
	if o2New < 0 {
		o2New = 0
	}
	// Carbon dioxide leaves in proportion to the tension pushing it out: exhaled
	// air is about 5% CO₂ at a normal PaCO₂, and richer when the blood is
	// carrying more. (The previous formula divided by the tick's air volume,
	// which produced fractions of several hundred percent that only looked
	// sane after the normalisation below.)
	co2Frac := ExhaledCO2AtNormalPaCO2 * (bloodCO2 / bloodstream.NormalPaCO2)
	if co2Frac < 0 {
		co2Frac = 0
	}
	pollNew := 0.0
	// Air leaves the airway at body temperature and fully saturated, whatever it
	// arrived as. Nothing downstream reads either back, so the body core is not
	// tracked here: a feverish person exhaling warmer air is real and changes
	// nothing any part of this simulation can see.
	tempNew := body.NormalCoreTemperature
	humidNew := 100.0

	// Normalize composition to sum to 100%
	compSum := n + o2New + a + pollNew + co2Frac
	if compSum > 0 {
		scale := (100.0) / compSum
		n = n * scale
		o2New = o2New * scale
		a = a * scale
		co2Frac = co2Frac * scale
	}

	// Emit exhaled_gas (same structure as inspired — composition fractions)
	exhaled := signal.New("air").
		WithLabel("category", "gas").
		WithLabel("type", "air").
		WithLabel("distribution:composition", "true").
		WithScalar("composition:nitrogen", n).
		WithScalar("composition:oxygen", o2New).
		WithScalar("composition:argon", a).
		WithScalar("composition:pollution", pollNew).
		WithScalar("composition:carbon_dioxide", co2Frac).
		WithScalar("temperature", tempNew).
		WithScalar("humidity", humidNew)
	this.OutputByName("exhaled_gas").PutSignals(exhaled)

	// Emit alveolar_gas (actual gas VOLUMES per tick, mL)
	alveolar := signal.New("alveolar_gas").
		WithLabel("category", "gas").
		WithLabel("type", "alveolar").
		WithScalar("O2_vol", o2New/100.0*tickVolume).
		WithScalar("CO2_vol", co2Frac/100.0*tickVolume).
		WithScalar("N2_vol", n/100.0*tickVolume).
		WithScalar("Ar_vol", a/100.0*tickVolume).
		WithScalar("tick_volume", tickVolume).
		WithScalar("temperature", tempNew).
		WithScalar("humidity", humidNew)
	this.OutputByName("alveolar_gas").PutSignals(alveolar)

	return nil
}

// strongestInspiratoryEffort returns the most negative pleural pressure offered,
// since a lower pressure is a stronger pull on the chest.
func strongestInspiratoryEffort(signals *signal.Group) (float64, error) {
	strongest := math.Inf(1)
	err := signals.ForEach(func(sig *signal.Signal) error {
		p, err := signal.AsFloat64(sig)
		if err != nil {
			return err
		}
		strongest = min(strongest, p)
		return nil
	})
	if err != nil {
		return 0, err
	}
	if math.IsInf(strongest, 1) {
		return 0, fmt.Errorf("no pleural pressure offered")
	}
	return strongest, nil
}

// publishAlveolarPO2 works out the oxygen tension in the alveoli and offers it to
// the blood.
//
// Everything the equation needs arrives on a port. Nothing here knows whether
// the air came from an atmosphere, a mountain, a barochamber or a cylinder --
// only what its pressure and oxygen fraction are -- which is exactly why an
// environment can be swapped for a different one without the lungs noticing.
func publishAlveolarPO2(this *component.Component, gas *signal.Signal, oxygenPct, bloodCO2 float64) error {
	if bloodCO2 <= 0 {
		bloodCO2 = bloodstream.NormalPaCO2
	}

	pAO2 := bloodstream.AlveolarPO2At(
		atmosphere.Pressure(gas), oxygenPct/100.0, bloodCO2)

	// The sum can come out negative, and that is not an error to be hidden: it is
	// the arithmetic saying this air cannot sustain a body at this PaCO₂. Held at
	// the floor, the blood desaturates and the chemoreflex is left to find the
	// hyperventilation that makes the sum work -- which is what a climber does.
	return this.OutputByName("alveolar_po2").PutPayloads(max(pAO2, bloodstream.MinPaO2))
}
