package organ

import (
	"github.com/hovsep/fmesh-examples/life/common"
	"github.com/hovsep/fmesh-examples/life/helper"
	"github.com/hovsep/fmesh-examples/life/unit"
	"github.com/hovsep/fmesh/component"
)

const (
	// Functional tidal baseline volume
	defaultVolume = 1500.0 * unit.Milliliter

	// Typical lung compliance in a healthy adult lung
	defaultCompliance = 200.0 * unit.MlPerCmH2O

	// Airway resistance (healthy resting value approximation)
	defaultResistance = 2.0 * unit.CmH2OPerMlPerSecond

	// Atmospheric mouth pressure reference
	mouthPressure = 0.0 * unit.CmH2O

	// Jitters make left and right lungs a little bit different
	lungVolumeAsymmetry     = 5 * unit.Percent
	lungComplianceAsymmetry = 7 * unit.Percent
	lungResistanceAsymmetry = 10 * unit.Percent
)

// GetLung creates a lung component for a given side (left/right).
func GetLung(side common.Side) *component.Component {
	return component.New("organ:lung_"+string(side)).
		WithDescription(side+" lung").
		AddInputs(
			"time", "pleural_pressure", "inspired_gas",
		).
		AddOutputs(
			"volume",            // Current lung volume
			"flow",              // Instantaneous airflow
			"alveolar_pressure", // Pressure inside alveoli
			"gas_composition",   // Alveolar gas mixture
		).
		WithInitialState(func(state component.State) {
			// Lungs have slightly different initial volumes
			lungVolume := helper.Jitter(defaultVolume, lungVolumeAsymmetry)
			// Mechanical state
			state.Set("volume", lungVolume)
			state.Set("prev_volume", lungVolume)

			// Lung mechanics (tunable for disease simulation)
			state.Set("compliance", helper.Jitter(defaultCompliance, lungComplianceAsymmetry))
			state.Set("resistance", helper.Jitter(defaultResistance, lungResistanceAsymmetry))
			state.Set("pleural_asymmetry", helper.Jitter(0.0, 0.2*unit.CmH2O))
		}).
		WithActivationFunc(helper.Pipeline(
			handleMechanics,
			publishOutputs,
		))
}

func handleMechanics(this *component.Component) error {
	if !this.InputByName("time").HasSignals() ||
		!this.InputByName("pleural_pressure").HasSignals() {
		return nil
	}

	// Δt for integration
	dt, err := helper.TickDurationInSec(this.InputByName("time").Signals().First())
	if err != nil {
		return err
	}

	// External driving pressure (from diaphragm)
	pleuralPressureSig := this.InputByName("pleural_pressure").Signals().First()

	// Anatomic inaccuracies between lungs
	pleuralPressure := helper.AsF64(pleuralPressureSig) + this.State().Get("pleural_asymmetry").(float64)

	// Mechanical parameters
	V := this.State().Get("volume").(float64)
	C := this.State().Get("compliance").(float64)
	R := this.State().Get("resistance").(float64)

	alveolarPressure := pleuralPressure + V/C

	flow := -(alveolarPressure - mouthPressure) / R

	// Volume integration (Euler step)
	Vnext := V + flow*dt

	// Physiological constraints (prevent numerical blow-up)
	Vnext = helper.Clamp(Vnext, 800, 3500)

	// Commit state
	this.State().Set("volume", Vnext)
	this.State().Set("prev_volume", V)
	this.State().Set("flow", flow)
	this.State().Set("alveolar_pressure", alveolarPressure)

	return nil
}

func publishOutputs(this *component.Component) error {
	this.OutputByName("volume").PutPayloads(this.State().Get("volume"))
	this.OutputByName("flow").PutPayloads(this.State().Get("flow"))
	this.OutputByName("alveolar_pressure").PutPayloads(this.State().Get("alveolar_pressure"))
	this.OutputByName("gas_composition").PutPayloads(this.State().Get("gas_composition"))

	return nil
}
