package da

import (
	"fmt"
	"math"

	"github.com/hovsep/fmesh-examples/life/common"
	"github.com/hovsep/fmesh-examples/life/helper"
	. "github.com/hovsep/fmesh-examples/life/unit"
	"github.com/hovsep/fmesh/component"
	"github.com/hovsep/fmesh/signal"
)

// Blood carries arterial blood gases in the units they are read in clinically:
// oxygen and carbon dioxide as partial pressures in mmHg, saturation as a
// percentage, and pH. Breathing pulls the gases toward what the alveoli offer;
// every organ draws oxygen out and returns carbon dioxide, so the values
// oscillate with the breath and drift away when breathing stops.
//
// Reference values for a resting adult breathing room air at sea level.
const (
	NormalPaO2  = 95.0 * MmHg
	NormalPaCO2 = 40.0 * MmHg
	NormalPH    = 7.40

	// AlveolarPO2 is the oxygen tension inside the alveoli on room air at sea
	// level: the pressure driving oxygen into the blood. It is a constant here
	// and becomes a function of barometric pressure and inspired fraction once
	// the airway carries them -- which is what makes altitude work.
	AlveolarPO2 = 104.0 * MmHg

	// AaGradient is the alveolar-arterial difference. Blood leaves the lungs a
	// little short of alveolar tension because some of it passes unventilated
	// alveoli, which is why a healthy PaO₂ is about 95 and not 104.
	AaGradient = 9.0 * MmHg

	// VentilatedPaCO2 is what ventilation pulls carbon dioxide down towards;
	// metabolism pushes it back up and the two settle near 40.
	VentilatedPaCO2 = 38.0 * MmHg

	// Survivable bounds. The oxygen ceiling is what hyperbaric therapy reaches.
	MinPaO2  = 5.0 * MmHg
	MaxPaO2  = 600.0 * MmHg
	MinPaCO2 = 10.0 * MmHg
	MaxPaCO2 = 150.0 * MmHg

	// CO2ExcretionFraction is kept for the lung component, which references it.
	CO2ExcretionFraction = 0.15 * Proportion

	// Inhale approach rates: fractional-per-second pull toward what the lungs
	// offer, proportional to the remaining gap, so tensions cannot overshoot.
	o2InhaleGain = 6.0 * PerSecond
	co2ClearGain = 6.0 * PerSecond
)

// The oxyhemoglobin dissociation curve.
const (
	// P50 is the oxygen tension at which hemoglobin is half saturated.
	P50 = 26.6 * MmHg

	// hillCoefficient is the curve's cooperativity: binding one oxygen molecule
	// makes hemoglobin readier to bind the next, which is what gives the curve
	// its S shape.
	hillCoefficient = 2.7
)

// SaturationAt returns hemoglobin saturation (%) at an arterial oxygen tension,
// from the Hill equation.
//
// This is the curve every physiology course draws. It is flat above about
// 80 mmHg, so a large fall in PaO₂ costs almost no saturation, and steep below
// about 60 mmHg, where saturation collapses. PaO₂ 60 → SpO₂ ≈ 90% is the corner
// it is known by, and the point at which supplemental oxygen is given.
func SaturationAt(paO2 float64) float64 {
	if paO2 <= 0 {
		return 0
	}
	bound := math.Pow(paO2, hillCoefficient)
	return 100.0 * bound / (bound + math.Pow(P50, hillCoefficient))
}

// PHAt returns blood pH at an arterial carbon dioxide tension.
//
// Carbon dioxide dissolves into carbonic acid, so ventilation sets pH:
// hypoventilation is an acidosis, hyperventilation an alkalosis. Acutely the
// shift is about 0.008 per mmHg -- the textbook "0.08 per 10 mmHg". Metabolic
// acid-base disturbance and renal compensation are not modelled.
func PHAt(paCO2 float64) float64 {
	return NormalPH - 0.008*(paCO2-NormalPaCO2)
}

var (
	statePaO2         common.State = "PaO2"
	statePaCO2        common.State = "PaCO2"
	stateGlucoseLevel common.State = "glucose_level"
	stateDt           common.State = "dt" // last known tick duration (seconds)
)

// DefaultGlucoseLevel is the fasting blood sugar the blood assumes before the
// reservoir (physiology:physiological_state) reports otherwise. Blood is only the
// transport here; the glucose reservoir lives in physiology.
const DefaultGlucoseLevel = 90.0 // mg/dL

// defaultDt matches the habitat's per-tick duration (10 ms); used as a fallback
// before the first time signal is seen.
const defaultDt = 0.01

// The blood is the organism's shared bus: any organ can secrete a substance into it by
// emitting a signal on its "blood" output. Each signal is tagged with a substance label
// so the blood component knows how to handle it. Organs may send one, several, or none
// per tick (e.g. only a hormone), and unknown substances are simply ignored for now.
const SubstanceLabel = "substance"

const (
	// SubstanceO2Consumption: payload is an oxygen demand in mmHg/s (lowers PaO₂).
	SubstanceO2Consumption = "o2_consumption"
	// SubstanceCO2Production: payload is a carbon dioxide return in mmHg/s (raises PaCO₂).
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
			"glucose",    // current blood sugar from the reservoir, carried to organs
		),
		component.WithOutputs(
			"venous_blood", // composite signal (PaO₂, PaCO₂, SpO₂, pH) broadcast to organs
			"spo2",         // plain floats, for observation/rendering
			"pao2",
			"paco2",
		),
		component.WithActivationFunc(exchangeBloodGases),
		component.WithInitialState(func(state component.State) {
			state.Set(statePaO2, NormalPaO2)
			state.Set(statePaCO2, NormalPaCO2)
			state.Set(stateGlucoseLevel, DefaultGlucoseLevel)
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
	// Latch the blood sugar the reservoir reports, whenever it arrives (on its
	// own mesh cycle), so the published venous blood always carries a value.
	if in := this.InputByName("glucose"); in.HasSignals() {
		this.State().Set(stateGlucoseLevel, helper.AsF64OrDefault(in.Signals().First(), DefaultGlucoseLevel))
	}

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

	// Phase B: fold in breathing (airflow) and organ metabolism (secretions) as
	// they arrive. This must not wait indefinitely for airflow: if both lungs have
	// failed it never comes, yet metabolism has to keep running -- O2 falls and CO2
	// rises, which is exactly what stopping the breathing should do.
	// updateBloodLevels treats absent airflow as zero net flow, so no fresh air is
	// pulled in.
	if this.InputByName("airflow").HasSignals() || this.InputByName("secretions").HasSignals() {
		updateBloodLevels(this)
	}
	return nil
}

func publishBloodLevels(this *component.Component) {
	paO2 := this.State().Get(statePaO2).(float64)
	paCO2 := this.State().Get(statePaCO2).(float64)
	spO2 := SaturationAt(paO2)

	this.OutputByName("venous_blood").PutSignals(
		signal.New("venous_blood").
			WithLabel("category", "gas").
			WithLabel("type", "venous").
			WithScalar("PaO2", paO2).
			WithScalar("PaCO2", paCO2).
			WithScalar("SpO2", spO2).
			WithScalar("pH", PHAt(paCO2)).
			WithScalar("glucose_level", this.State().Get(stateGlucoseLevel).(float64)),
	)
	this.OutputByName("spo2").PutPayloads(spO2)
	this.OutputByName("pao2").PutPayloads(paO2)
	this.OutputByName("paco2").PutPayloads(paCO2)
}

func updateBloodLevels(this *component.Component) {
	dt := this.State().Get(stateDt).(float64)
	o2 := this.State().Get(statePaO2).(float64)
	co2 := this.State().Get(statePaCO2).(float64)

	// Net airflow across both lungs.
	var netFlow float64
	this.InputByName("airflow").Signals().ForEach(func(sig *signal.Signal) error {
		netFlow += helper.AsF64OrDefault(sig, 0)
		return nil
	})

	// On inhale, fresh air pulls the gases toward what the lungs offer: oxygen
	// toward alveolar tension less the alveolar-arterial gradient, carbon dioxide
	// down toward what ventilation clears it to. The pull is proportional to the
	// remaining gap, giving smooth waves that never overshoot.
	if netFlow > 0 {
		o2 += (AlveolarPO2 - AaGradient - o2) * o2InhaleGain * dt
		co2 -= (co2 - VentilatedPaCO2) * co2ClearGain * dt
	}

	// Organ metabolism: read every substance secreted into the blood this tick and apply
	// it by kind. Rates are in mmHg/s, integrated over dt. Unknown substances are ignored.
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

	// Safety-net clamp (the approach math already keeps tensions in range).
	o2 = helper.Clamp(o2, MinPaO2, MaxPaO2)
	co2 = helper.Clamp(co2, MinPaCO2, MaxPaCO2)

	this.State().Set(statePaO2, o2)
	this.State().Set(statePaCO2, co2)
}
