package organ

import (
	"fmt"
	"math"

	"github.com/hovsep/fmesh-examples/life/common"
	"github.com/hovsep/fmesh-examples/life/helper"
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

	// Below are per-instance lung params that makes left and
	// right lungs slightly different (anatomically, the left one has less space due to the heart).
	statePleuralAsymmetry common.State = "pleural_asymmetry"
	stateCompliance       common.State = "compliance"
	stateVolume           common.State = "volume"
	stateResistance       common.State = "resistance"
	stateBloodCO2         common.State = "last_blood_co2"
)

var (
	// FRC is Functional Residual Capacity (equilibrium volume at the end of passive expiration).
	// FRC = V₀ + C·|BasePleuralPressure| = 700 + 100·5 = 1200 mL.
	FRC = restingLungVolume + defaultLungCompliance*math.Abs(BasePleuralPressure)*Milliliter
)

func GetLung(side common.Side) (*component.Component, error) {
	c, err := component.New("organ:lung_"+string(side),
		component.WithDescription(string(side)+" lung"),
		component.WithInputs(
			"time",
			"pleural_pressure",
			"inspired_gas",
			"blood_co2",
		),
		component.WithOutputs(
			"volume",            // Current lung volume (dynamic)
			"flow",              // Instantaneous airflow
			"alveolar_pressure", // Pressure inside alveoli
			"exhaled_gas",       // passthrough (not modeled yet)
			"alveolar_gas",      // what goes to bloodstream (not modeled yet)
		),
		component.WithActivationFunc(helper.SequentialActivationFunc(
			handleMechanics,
			handleGasExchange,
		)),
		component.WithInitialState(func(state component.State) {
			state.Set(stateVolume, helper.Jitter(FRC, lungVolumeAsymmetry)) // start at equilibrium
			state.Set(stateCompliance, helper.Jitter(defaultLungCompliance, lungComplianceAsymmetry))
			state.Set(stateResistance, helper.Jitter(defaultAirwayResistance, lungResistanceAsymmetry))
			state.Set(statePleuralAsymmetry, helper.Jitter(pleuralPressureAsymmetryBase, pleuralPressureAsymmetry))
			state.Set(stateBloodCO2, 0.0)
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("organ:lung_%s: %w", side, err)
	}
	return c, nil
}

func handleMechanics(this *component.Component) error {
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
	// Store incoming blood_co2 in state (even if we can't process it right now)
	if bloodSig := this.InputByName("blood_co2").Signals().First(); bloodSig != nil {
		this.State().Set(stateBloodCO2, bloodSig.Scalars().GetOrDefault("CO2_volume", 0))
	}

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

	var co2FromBlood float64
	if stored, ok := this.State().Get(stateBloodCO2).(float64); ok {
		co2FromBlood = stored
	}

	// Post-exchange composition fractions (in Percent units)
	o2New := o - 7*Percent
	if o2New < 0 {
		o2New = 0
	}
	co2Frac := (co2FromBlood / tickVolume) * 100 * Percent
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
