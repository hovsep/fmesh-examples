package da

import (
	"fmt"

	"github.com/hovsep/fmesh-examples/life/common"
	"github.com/hovsep/fmesh-examples/life/helper"
	. "github.com/hovsep/fmesh-examples/life/unit"
	"github.com/hovsep/fmesh/component"
	"github.com/hovsep/fmesh/signal"
)

const (
	MetabolicCO2Rate = 200.0 * Milliliter / 60.0 // mL CO2 per second at rest

	DefaultO2Level  = 200.0 * Milliliter
	MaxO2Level      = 250.0 * Milliliter
	MinO2Level      = 50.0 * Milliliter
	DefaultCO2Level = 50.0 * Milliliter
	MaxCO2Level     = 80.0 * Milliliter
	MinCO2Level     = 20.0 * Milliliter
	DefaultGlucose  = 5.0
	MaxGlucose      = 7.0
	MinGlucose      = 2.0

	O2AbsorptionEfficiency = 0.85 // fraction of alveolar O2 absorbed into blood
	CO2ExcretionFraction   = 0.15 // fraction of blood CO2 excreted into alveoli per breath

	O2ConsumptionRate  = 4.0 * Milliliter / 60.0 // mL O2 consumed per second (tissue metabolism)
	GlucoseConsumption = 0.001                   // glucose consumed per second
)

var (
	stateO2Level  common.State = "O2_level"
	stateCO2Level common.State = "CO2_level"
	stateGlucose  common.State = "glucose_level"
)

func GetBloodSystem() (*component.Component, error) {
	c, err := component.New("da:blood_system",
		component.WithDescription("Blood system with O2, CO2, and glucose levels"),
		component.WithInputs(
			"time",
			"alveolar_gas",
		),
		component.WithOutputs(
			"venous_blood",
		),
		component.WithActivationFunc(exchangeBloodGases),
		component.WithInitialState(func(state component.State) {
			state.Set(stateO2Level, DefaultO2Level)
			state.Set(stateCO2Level, DefaultCO2Level)
			state.Set(stateGlucose, DefaultGlucose)
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("da:blood_system: %w", err)
	}
	return c, nil
}

func exchangeBloodGases(this *component.Component) error {
	if !this.InputByName("time").HasSignals() {
		return nil
	}

	dt, err := helper.TickDurationInSec(this.InputByName("time").Signals().First())
	if err != nil {
		return err
	}

	o2 := this.State().Get(stateO2Level).(float64)
	co2 := this.State().Get(stateCO2Level).(float64)
	glucose := this.State().Get(stateGlucose).(float64)

	// Gas exchange with alveoli
	if alveolarSig := this.InputByName("alveolar_gas").Signals().First(); alveolarSig != nil {
		alveolarO2 := alveolarSig.Scalars().GetOrDefault("O2_vol", 0)

		o2Absorbed := alveolarO2 * O2AbsorptionEfficiency
		o2 += o2Absorbed

		co2Excreted := co2 * CO2ExcretionFraction
		co2 -= co2Excreted
	}

	// Continuous tissue metabolism
	o2 -= O2ConsumptionRate * dt
	co2 += MetabolicCO2Rate * dt
	glucose -= GlucoseConsumption * dt

	// Clamp to physiological ranges
	o2 = helper.Clamp(o2, MinO2Level, MaxO2Level)
	co2 = helper.Clamp(co2, MinCO2Level, MaxCO2Level)
	glucose = helper.Clamp(glucose, MinGlucose, MaxGlucose)

	this.State().Set(stateO2Level, o2)
	this.State().Set(stateCO2Level, co2)
	this.State().Set(stateGlucose, glucose)

	this.OutputByName("venous_blood").PutSignals(
		signal.New("venous_blood").
			WithLabel("category", "gas").
			WithLabel("type", "venous").
			WithScalar("O2_level", o2).
			WithScalar("CO2_level", co2).
			WithScalar("glucose_level", glucose),
	)
	return nil
}
