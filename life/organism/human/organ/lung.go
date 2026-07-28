package organ

import (
	"fmt"
	"math"

	"github.com/hovsep/fmesh-examples/life/common"
	"github.com/hovsep/fmesh-examples/life/helper"
	da "github.com/hovsep/fmesh-examples/life/organism/human/distributed_anatomy"
	"github.com/hovsep/fmesh-examples/life/plugin/damage"
	. "github.com/hovsep/fmesh-examples/life/unit"
	"github.com/hovsep/fmesh/component"
	"github.com/hovsep/fmesh/signal"
)

const (
	restingLungVolume = 700.0 * Milliliter

	ResidualLungVolume = 600.0 * Milliliter
	TotalLungCapacity  = 3000.0 * Milliliter

	defaultLungCompliance   = 100.0 * MlPerCmH2O
	defaultAirwayResistance = 0.002 * CmH2OPerMlPerSecond

	lungVolumeAsymmetry     = 5 * Percent
	lungComplianceAsymmetry = 5 * Percent
	lungResistanceAsymmetry = 5 * Percent

	pleuralPressureAsymmetryBase = 0.3 * CmH2O
	pleuralPressureAsymmetry     = 30 * Percent

	baseO2Consumption     = 7 * Percent // fraction of inspired O2 consumed at rest
	o2ConsumptionMaxDelta = 5 * Percent // additional O2 consumption when blood is O2-depleted

	// ExhaledCO2AtNormalPaCO2 is the carbon dioxide fraction of exhaled air when
	// the blood is at a normal PaCO₂ -- room air carries almost none, breath
	// carries about a twentieth.
	ExhaledCO2AtNormalPaCO2 = 5.0 * Percent

	// Below are per-instance lung params that makes left and
	// right lungs slightly different (anatomically, the left one has less space due to the heart).
	statePleuralAsymmetry common.State = "pleural_asymmetry"
	stateCompliance       common.State = "compliance"
	stateVolume           common.State = "volume"
	stateResistance       common.State = "resistance"
)

var (
	// FRC is Functional Residual Capacity (equilibrium volume at the end of passive expiration).
	// FRC = V₀ + C·|BasePleuralPressure| = 700 + 100·5 = 1200 mL.
	FRC = restingLungVolume + defaultLungCompliance*math.Abs(BasePleuralPressure)*Milliliter
)

func GetLung(side common.Side) (*component.Component, error) {
	c, err := component.New("organ:lung_"+string(side),
		component.WithDescription(string(side)+" lung"),
		component.WithPlugins(damage.New(damage.Config{Organ: "lung_" + string(side)})),
		component.WithInputs("time", "pleural_pressure", "inspired_gas", "venous_blood"),
		component.WithOutputs("volume", "flow", "alveolar_pressure", "exhaled_gas", "alveolar_gas"),
		component.WithActivationFunc(damage.FlatlineWhenFailed(
			handleMechanics,
			handleGasExchange,
		)),
		component.WithInitialState(func(state component.State) {
			state.Set(stateVolume, helper.Jitter(FRC, lungVolumeAsymmetry)) // start at equilibrium
			state.Set(stateCompliance, helper.Jitter(defaultLungCompliance, lungComplianceAsymmetry))
			state.Set(stateResistance, helper.Jitter(defaultAirwayResistance, lungResistanceAsymmetry))
			state.Set(statePleuralAsymmetry, helper.Jitter(pleuralPressureAsymmetryBase, pleuralPressureAsymmetry))
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("organ:lung_%s: %w", side, err)
	}
	return c, nil
}

func handleMechanics(this *component.Component) error {
	// A breathing cycle is driven by the tick. If there is no tick, this activation
	// came from some other input arriving out of phase (e.g. an inhaled toxin on
	// the damage port); do nothing rather than keep waiting for a tick that has
	// already passed, which would stall the mesh.
	if !this.InputByName("time").HasSignals() {
		return nil
	}
	if !this.Inputs().ByNames("time", "pleural_pressure", "inspired_gas").AllHaveSignals() {
		return component.ErrWaitingForInputsKeep
	}

	dt, err := helper.TickDurationInSec(this.InputByName("time").Signals().First())
	if err != nil {
		return err
	}

	pp, err := helper.AsF64(this.InputByName("pleural_pressure").Signals().First())
	if err != nil {
		return err
	}
	pleuralPressure := pp + this.State().Get(statePleuralAsymmetry).(float64)

	V := this.State().Get(stateVolume).(float64)
	C := this.State().Get(stateCompliance).(float64)
	R := this.State().Get(stateResistance).(float64)

	alveolarPressure := pleuralPressure + (V-restingLungVolume)/C
	flow := -alveolarPressure / R
	Vnext := helper.ClampAndLogAnomaly(V+flow*dt, ResidualLungVolume, TotalLungCapacity, this.Logger(), "lung volume")

	this.State().Set(stateVolume, Vnext)

	this.OutputByName("volume").PutPayloads(Vnext)
	this.OutputByName("flow").PutPayloads(flow)
	this.OutputByName("alveolar_pressure").PutPayloads(alveolarPressure)

	return nil
}

func handleGasExchange(this *component.Component) error {
	if !this.Inputs().ByNames("inspired_gas").AllHaveSignals() {
		return nil
	}

	gas := this.InputByName("inspired_gas").Signals().First()
	n, o, a, _, _, _, err := helper.UnpackAir(gas)
	if err != nil {
		return err
	}

	flowSig := this.OutputByName("flow").Signals().First()
	if flowSig == nil {
		return nil
	}
	flow, err := helper.AsF64(flowSig)
	if err != nil {
		return err
	}

	dt, err := helper.TickDurationInSec(this.InputByName("time").Signals().First())
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

	// Blood that arrives short of oxygen takes more of it out of the air, so the
	// exhaled fraction falls as the deficit grows.
	o2Deficit := helper.Clamp((da.NormalPaO2-bloodO2)/da.NormalPaO2, 0, 1)
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
	co2Frac := ExhaledCO2AtNormalPaCO2 * (bloodCO2 / da.NormalPaCO2)
	if co2Frac < 0 {
		co2Frac = 0
	}
	pollNew := 0.0
	tempNew := 37.0 * Celsius // @TODO: get current temp instead of hardcoding
	humidNew := 100.0 * Percent

	// Normalize composition to sum to 100%
	compSum := n + o2New + a + pollNew + co2Frac
	if compSum > 0 {
		scale := (100.0 * Percent) / compSum
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
