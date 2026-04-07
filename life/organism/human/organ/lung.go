package organ

import (
	"github.com/hovsep/fmesh-examples/life/common"
	"github.com/hovsep/fmesh-examples/life/helper"
	"github.com/hovsep/fmesh-examples/life/unit"
	"github.com/hovsep/fmesh/component"
)

const (
	statePleuralAsymmetry common.State = "pleural_asymmetry"
	stateRestVolume       common.State = "rest_volume"

	// Functional residual capacity (resting volume, V₀)
	defaultRestVolume = 1100.0 * unit.Milliliter

	// Typical lung compliance in a healthy adult lung
	defaultCompliance = 200.0 * unit.MlPerCmH2O

	// Airway resistance (healthy resting value approximation)
	defaultResistance = 2.0 * unit.CmH2OPerMlPerSecond

	// Atmospheric mouth pressure reference
	mouthPressure = 0.0 * unit.CmH2O

	// Jitters make left and right lungs slightly different
	lungVolumeAsymmetry     = 5 * unit.Percent
	lungComplianceAsymmetry = 7 * unit.Percent
	lungResistanceAsymmetry = 10 * unit.Percent
)

// GetLung creates a lung component for a given side (left/right).
func GetLung(side common.Side) *component.Component {
	return component.New("organ:lung_"+string(side)).
		WithDescription(side+" lung").
		AddInputs(
			"time",
			"pleural_pressure",
			"inspired_gas", // currently unused (mechanics-only model)
		).
		AddOutputs(
			"volume",            // Current lung volume (dynamic)
			"flow",              // Instantaneous airflow
			"alveolar_pressure", // Pressure inside alveoli
			"gas_composition",   // passthrough (not modeled yet)
		).
		WithInitialState(func(state component.State) {
			// Establish resting volume (V₀) with asymmetry
			restVolume := helper.Jitter(defaultRestVolume, lungVolumeAsymmetry)

			// Start at equilibrium (important: avoids artificial initial transients)
			state.Set(common.Volume, restVolume)

			// Mechanics (these are static → fine to keep in state)
			state.Set(common.Compliance, helper.Jitter(defaultCompliance, lungComplianceAsymmetry))
			state.Set(common.Resistance, helper.Jitter(defaultResistance, lungResistanceAsymmetry))

			// Small anatomical asymmetry in pleural pressure
			state.Set(statePleuralAsymmetry, helper.Jitter(0.0, 0.2*unit.CmH2O))
			state.Set(stateRestVolume, restVolume)
		}).
		WithActivationFunc(helper.Pipeline(
			handleMechanics,
		))
}

func handleMechanics(this *component.Component) error {
	// Wait for required inputs
	if !this.Inputs().ByNames("time", "pleural_pressure").AllHaveSignals() {
		return component.NewErrWaitForInputs(component.KeepAllInputs)
	}

	// Δt for integration
	dt, err := helper.TickDurationInSec(this.InputByName("time").Signals().First())
	if err != nil {
		return err
	}

	// External driving pressure (diaphragm)
	pleuralPressure := helper.AsF64(this.InputByName("pleural_pressure").Signals().First()) +
		this.State().Get(statePleuralAsymmetry).(float64)

	// Lung mechanics parameters
	V := this.State().Get(common.Volume).(float64)
	V0 := this.State().Get(stateRestVolume).(float64)
	C := this.State().Get(common.Compliance).(float64)
	R := this.State().Get(common.Resistance).(float64)

	// --- Second-order flow dynamics ---
	// flow inertia (mass of air column, small value for oscillation)
	const airMass = 0.05 // arbitrary scaling to create underdamped oscillation

	// Previous flow (store in state)
	flowPrev := 0.0
	if v := this.State().Get("flow"); v != nil {
		flowPrev = v.(float64)
	}

	// Elastic recoil relative to resting volume
	alveolarPressure := pleuralPressure + (V-V0)/C

	// Flow derivative: dFlow/dt = (PressureGradient - R*flow) / Mass
	flowDot := (-(alveolarPressure - mouthPressure) - R*flowPrev) / airMass
	flowNext := flowPrev + flowDot*dt

	// Volume update
	Vnext := V + flowNext*dt

	// Persist state
	this.State().Set(common.Volume, Vnext)
	this.State().Set("flow", flowNext)

	// Outputs
	this.OutputByName("volume").PutPayloads(Vnext)
	this.OutputByName("flow").PutPayloads(flowNext)
	this.OutputByName("alveolar_pressure").PutPayloads(alveolarPressure)
	this.OutputByName("gas_composition").PutPayloads(this.State().Get("gas_composition"))

	return nil
}
