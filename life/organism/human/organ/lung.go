package organ

import (
	"github.com/hovsep/fmesh-examples/life/common"
	"github.com/hovsep/fmesh-examples/life/helper"
	"github.com/hovsep/fmesh-examples/life/unit"
	"github.com/hovsep/fmesh/component"
	"github.com/hovsep/fmesh/port"
)

const (
	statePleuralAsymmetry common.State = "pleural_asymmetry"

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
			"time", // There is no guarantee that a time signal arrives with other inputs. Each organ must track its own clock
			"pleural_pressure",
			"inspired_gas",
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
			state.Set(common.Volume, lungVolume)

			// Lung mechanics (tunable for disease simulation)
			state.Set(common.Compliance, helper.Jitter(defaultCompliance, lungComplianceAsymmetry))
			state.Set(common.Resistance, helper.Jitter(defaultResistance, lungResistanceAsymmetry))
			state.Set(statePleuralAsymmetry, helper.Jitter(0.0, 0.2*unit.CmH2O))
		}).
		WithActivationFunc(helper.Pipeline(
			handleMechanics,
		))
}

func handleMechanics(this *component.Component) error {
	// Time signal comes earlier, so we can just wait for both
	if !this.Inputs().Filter(func(p *port.Port) bool {
		return p.Name() == "time" || p.Name() == "pleural_pressure"
	}).AllHaveSignals() {
		return component.NewErrWaitForInputs(component.KeepAllInputs)
	}

	// Δt for integration
	dt, err := helper.TickDurationInSec(currentTickSig)
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

	this.OutputByName("volume").PutPayloads(Vnext)
	this.OutputByName("flow").PutPayloads(flow)
	this.OutputByName("alveolar_pressure").PutPayloads(alveolarPressure)
	this.OutputByName("gas_composition").PutPayloads(this.State().Get("gas_composition"))

	return nil
}
