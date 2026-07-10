package da

import (
	"fmt"

	"github.com/hovsep/fmesh-examples/life/common"
	"github.com/hovsep/fmesh-examples/life/helper"
	. "github.com/hovsep/fmesh-examples/life/unit"
	"github.com/hovsep/fmesh/component"
	"github.com/hovsep/fmesh/signal"
)

// Blood tracks two game-style "levels" (0-100%, not real physiology).
// Breathing (fresh air on inhale) pushes O2 up and CO2 down; other organs
// (brain, heart, ...) continuously consume O2 and return CO2, so the levels
// oscillate in phase with the breath.
const (
	// O2/CO2 level bounds (0-100% game scale). O2 has a floor > 0 so it never hits zero.
	MinO2Level     = 20.0 * Percent
	MaxO2Level     = 100.0 * Percent
	DefaultO2Level = 100.0 * Percent

	MinCO2Level     = 0.0 * Percent
	MaxCO2Level     = 100.0 * Percent
	DefaultCO2Level = 30.0 * Percent

	// CO2ExcretionFraction is kept for the lung component, which references it.
	CO2ExcretionFraction = 0.15 * Proportion

	// Inhale approach rates: fractional-per-second pull toward the healthy target
	// (proportional to the remaining gap, so levels can't overshoot or pin flat).
	o2InhaleGain = 6.0 * PerSecond
	co2ClearGain = 6.0 * PerSecond
)

var (
	stateO2Level  common.State = "O2_level"
	stateCO2Level common.State = "CO2_level"
	stateDt       common.State = "dt" // last known tick duration (seconds)
)

// defaultDt matches the habitat's per-tick duration (10 ms); used as a fallback
// before the first time signal is seen.
const defaultDt = 0.01

// The blood is the organism's shared bus: any organ can secrete a substance into it by
// emitting a signal on its "blood" output. Each signal is tagged with a substance label
// so the blood component knows how to handle it. Organs may send one, several, or none
// per tick (e.g. only a hormone), and unknown substances are simply ignored for now.
const SubstanceLabel = "substance"

const (
	// SubstanceO2Consumption: payload is an O2 demand rate in %/s (decreases blood O2).
	SubstanceO2Consumption = "o2_consumption"
	// SubstanceCO2Production: payload is a CO2 return rate in %/s (increases blood CO2).
	SubstanceCO2Production = "co2_production"
	// Future: toxins, hormones, nutrients, ... just add a case in updateBloodLevels.
)

// Secretion builds a substance signal for an organ to emit on its "blood" output.
func Secretion(substance string, rate float64) *signal.Signal {
	return signal.New(rate).WithLabel(SubstanceLabel, substance)
}

func GetBloodSystem() (*component.Component, error) {
	c, err := component.New("da:blood_system",
		component.WithDescription("Blood system tracking O2 and CO2 levels"),
		component.WithInputs(
			"time",
			"airflow",    // lung airflow: >0 inhaling (fresh air), <0 exhaling
			"secretions", // shared bus: any organ emits labeled substance signals here
		),
		component.WithOutputs(
			"venous_blood", // composite signal (O2_level, CO2_level) broadcast to organs
			"o2_level",     // plain float, for observation/rendering
			"co2_level",    // plain float, for observation/rendering
		),
		component.WithActivationFunc(exchangeBloodGases),
		component.WithInitialState(func(state component.State) {
			state.Set(stateO2Level, DefaultO2Level)
			state.Set(stateCO2Level, DefaultCO2Level)
			state.Set(stateDt, defaultDt)
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("da:blood_system: %w", err)
	}
	return c, nil
}

// exchangeBloodGases runs in two phases within a single inner-mesh run, because the
// breathing signal (lung airflow) only reaches this component a couple of cycles after
// the time tick does:
//
//   - Phase A (time present): publish the current O2/CO2 levels right away. Emitting
//     venous_blood early lets the lungs consume it together with their other inputs in
//     the same run, which keeps the feedback loop from deadlocking the mesh.
//   - Phase B (airflow present): fold this run's breathing and organ metabolism into the
//     stored levels, so the next tick publishes fresh values (a negligible ~10 ms lag).
//
// While only partial inputs have arrived (e.g. organ metabolism but not yet airflow) the
// component keeps them and waits, so nothing is dropped.
func exchangeBloodGases(this *component.Component) error {
	// Phase A: time tick -> publish current levels and remember dt.
	if this.InputByName("time").HasSignals() {
		dt, err := helper.TickDurationInSec(this.InputByName("time").Signals().First())
		if err != nil {
			return err
		}
		this.State().Set(stateDt, dt)
		publishBloodLevels(this)
		return nil
	}

	// Phase B: breathing airflow -> update levels for the next tick.
	if this.InputByName("airflow").HasSignals() {
		updateBloodLevels(this)
		return nil
	}

	// Partial inputs (e.g. organ metabolism) arrived before airflow; keep and wait.
	return component.ErrWaitingForInputsKeep
}

func publishBloodLevels(this *component.Component) {
	o2 := this.State().Get(stateO2Level).(float64)
	co2 := this.State().Get(stateCO2Level).(float64)

	this.OutputByName("venous_blood").PutSignals(
		signal.New("venous_blood").
			WithLabel("category", "gas").
			WithLabel("type", "venous").
			WithScalar("O2_level", o2).
			WithScalar("CO2_level", co2),
	)
	this.OutputByName("o2_level").PutPayloads(o2)
	this.OutputByName("co2_level").PutPayloads(co2)
}

func updateBloodLevels(this *component.Component) {
	dt := this.State().Get(stateDt).(float64)
	o2 := this.State().Get(stateO2Level).(float64)
	co2 := this.State().Get(stateCO2Level).(float64)

	// Net airflow across both lungs.
	var netFlow float64
	this.InputByName("airflow").Signals().ForEach(func(sig *signal.Signal) error {
		netFlow += helper.AsF64OrDefault(sig, 0)
		return nil
	})

	// On inhale (fresh air) pull both gases toward their healthy targets. Magnitude is
	// proportional to the remaining gap, giving smooth waves that never overshoot.
	if netFlow > 0 {
		o2 += (MaxO2Level - o2) * o2InhaleGain * dt
		co2 -= (co2 - MinCO2Level) * co2ClearGain * dt
	}

	// Organ metabolism: read every substance secreted into the blood this tick and apply
	// it by kind. Rates are in %/s, integrated over dt. Unknown substances are ignored.
	var o2DemandRate, co2ReturnRate float64
	this.InputByName("secretions").Signals().ForEach(func(sig *signal.Signal) error {
		rate := helper.AsF64OrDefault(sig, 0)
		switch sig.Labels().ValueOrDefault(SubstanceLabel, "") {
		case SubstanceO2Consumption:
			o2DemandRate += rate
		case SubstanceCO2Production:
			co2ReturnRate += rate
		}
		return nil
	})
	o2 -= o2DemandRate * dt
	co2 += co2ReturnRate * dt

	// Safety-net clamp (the approach math already keeps levels in range).
	o2 = helper.Clamp(o2, MinO2Level, MaxO2Level)
	co2 = helper.Clamp(co2, MinCO2Level, MaxCO2Level)

	this.State().Set(stateO2Level, o2)
	this.State().Set(stateCO2Level, co2)
}
