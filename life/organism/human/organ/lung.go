package organ

import (
	"math"

	"github.com/hovsep/fmesh-examples/life/common"
	"github.com/hovsep/fmesh-examples/life/helper"
	. "github.com/hovsep/fmesh-examples/life/unit"
	"github.com/hovsep/fmesh/component"
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
)

var (
	// FRC is Functional Residual Capacity (equilibrium volume at the end of passive expiration).
	// FRC = V₀ + C·|BasePleuralPressure| = 700 + 100·5 = 1200 mL.
	FRC = restingLungVolume + defaultLungCompliance*math.Abs(BasePleuralPressure)*Milliliter
)

func GetLung(side common.Side) *component.Component {
	return component.New("organ:lung_"+string(side)).
		WithDescription(string(side)+" lung").
		AddInputs(
			"time",
			"pleural_pressure",
			"inspired_gas", // not used yet
		).
		AddOutputs(
			"volume",            // Current lung volume (dynamic)
			"flow",              // Instantaneous airflow
			"alveolar_pressure", // Pressure inside alveoli
			"gas_composition",   // passthrough (not modeled yet)
		).
		WithInitialState(func(state component.State) {
			state.Set(stateVolume, helper.Jitter(FRC, lungVolumeAsymmetry)) // start at equilibrium
			state.Set(stateCompliance, helper.Jitter(defaultLungCompliance, lungComplianceAsymmetry))
			state.Set(stateResistance, helper.Jitter(defaultAirwayResistance, lungResistanceAsymmetry))
			state.Set(statePleuralAsymmetry, helper.Jitter(pleuralPressureAsymmetryBase, pleuralPressureAsymmetry))
		}).
		WithActivationFunc(handleMechanics)
}

func handleMechanics(this *component.Component) error {
	if !this.Inputs().ByNames("time", "pleural_pressure").AllHaveSignals() {
		return component.ErrWaitingForInputsKeep
	}

	dt, err := helper.TickDurationInSec(this.InputByName("time").Signals().First())
	if err != nil {
		return err
	}

	pleuralPressure := helper.AsF64(this.InputByName("pleural_pressure").Signals().First()) + this.State().Get(statePleuralAsymmetry).(float64)

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
	this.OutputByName("gas_composition").PutPayloads(this.State().Get("gas_composition"))

	return nil
}
