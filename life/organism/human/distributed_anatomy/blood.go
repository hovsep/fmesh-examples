package da

import (
	"fmt"

	"github.com/hovsep/fmesh-examples/life/bloodstream"
	"github.com/hovsep/fmesh-examples/life/common"
	"github.com/hovsep/fmesh-examples/life/helper"
	"github.com/hovsep/fmesh/component"
	"github.com/hovsep/fmesh/signal"
)

var (
	// Saturation is the stored quantity, not tension: organs take oxygen out by
	// the millilitre, and how much they can take depends on how much hemoglobin
	// is there to carry it. PaO₂ is derived from saturation, not the other way
	// round.
	stateSaturation   common.State = "SaO2"
	stateHemoglobin   common.State = "hemoglobin"
	stateVolume       common.State = "volume_l"
	statePaCO2        common.State = "PaCO2"
	stateGlucoseLevel common.State = "glucose_level"
	stateDt           common.State = "dt" // last known tick duration (seconds)
)

const (
	// Inhale approach rates: a fractional-per-second pull toward what the lungs
	// offer, proportional to the remaining gap, so nothing overshoots.
	o2InhaleGain = 6.0
	co2ClearGain = 6.0
)

// defaultDt matches the habitat's per-tick duration (10 ms); used as a fallback
// before the first time signal is seen.
const defaultDt = 0.01

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
			"venous_blood", // composite: gases, content, hemoglobin, volume, glucose
			"spo2",         // plain floats, for observation/rendering
			"pao2",
			"paco2",
			"cao2",
		),
		component.WithActivationFunc(exchangeBloodGases),
		component.WithInitialState(func(state component.State) {
			state.Set(stateSaturation, bloodstream.NormalSaO2)
			state.Set(stateHemoglobin, bloodstream.NormalHemoglobin)
			state.Set(stateVolume, bloodstream.NormalBloodVolume)
			state.Set(statePaCO2, bloodstream.NormalPaCO2)
			state.Set(stateGlucoseLevel, bloodstream.DefaultGlucoseLevel)
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
		this.State().Set(stateGlucoseLevel, helper.AsF64OrDefault(in.Signals().First(), bloodstream.DefaultGlucoseLevel))
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
	saturation := this.State().Get(stateSaturation).(float64)
	hemoglobin := this.State().Get(stateHemoglobin).(float64)
	paCO2 := this.State().Get(statePaCO2).(float64)

	paO2 := bloodstream.TensionAt(saturation)
	content := bloodstream.OxygenContent(hemoglobin, saturation, paO2)

	this.OutputByName("venous_blood").PutSignals(
		signal.New("venous_blood").
			WithLabel("category", "gas").
			WithLabel("type", "venous").
			WithScalar("PaO2", paO2).
			WithScalar("PaCO2", paCO2).
			WithScalar("SpO2", saturation).
			WithScalar("pH", bloodstream.PHAt(paCO2)).
			WithScalar("CaO2", content).
			WithScalar("hemoglobin", hemoglobin).
			WithScalar("volume_l", this.State().Get(stateVolume).(float64)).
			WithScalar("glucose_level", this.State().Get(stateGlucoseLevel).(float64)),
	)
	this.OutputByName("spo2").PutPayloads(saturation)
	this.OutputByName("pao2").PutPayloads(paO2)
	this.OutputByName("paco2").PutPayloads(paCO2)
	this.OutputByName("cao2").PutPayloads(content)
}

func updateBloodLevels(this *component.Component) {
	dt := this.State().Get(stateDt).(float64)
	saturation := this.State().Get(stateSaturation).(float64)
	hemoglobin := this.State().Get(stateHemoglobin).(float64)
	volume := this.State().Get(stateVolume).(float64)
	co2 := this.State().Get(statePaCO2).(float64)

	// Net airflow across both lungs.
	var netFlow float64
	this.InputByName("airflow").Signals().ForEach(func(sig *signal.Signal) error {
		netFlow += helper.AsF64OrDefault(sig, 0)
		return nil
	})

	// On inhale, fresh air loads the blood toward what the lungs can offer:
	// saturation toward what alveolar tension (less the alveolar-arterial
	// gradient) supports, and carbon dioxide down toward what ventilation clears
	// it to. Both pulls are proportional to the remaining gap, so nothing
	// overshoots.
	if netFlow > 0 {
		target := bloodstream.SaturationAt(bloodstream.AlveolarPO2 - bloodstream.AaGradient)
		saturation += (target - saturation) * o2InhaleGain * dt
		co2 -= (co2 - bloodstream.VentilatedPaCO2) * co2ClearGain * dt
	}

	// What the organs took out and put back. Draws are volumes per second, so
	// they have to be turned into a change in saturation, and that conversion is
	// where the blood's carrying capacity enters: the same demand costs far more
	// saturation when there is less hemoglobin, or less blood, to spread it over.
	//
	//	capacity (mL) = Hüfner × hemoglobin (g/dL) × volume (dL)
	//
	// A healthy adult holds about a litre of oxygen this way, and uses a quarter
	// of it a minute. Halve the hemoglobin and the same draw empties it twice as
	// fast -- which is why bleeding suffocates a body whose lungs are perfectly
	// good.
	var o2DrawPerSec, co2LoadPerSec float64
	this.InputByName("secretions").Signals().ForEach(func(sig *signal.Signal) error {
		rate := helper.AsF64OrDefault(sig, 0)
		switch sig.Labels().ValueOrDefault(bloodstream.SubstanceLabel, "") {
		case bloodstream.SubstanceO2Draw:
			o2DrawPerSec += rate
		case bloodstream.SubstanceCO2Load:
			co2LoadPerSec += rate
		}
		return nil
	})

	if capacity := bloodstream.HufnerConstant * hemoglobin * volume * bloodstream.DLPerLiter; capacity > 0 {
		saturation -= (o2DrawPerSec * dt) / capacity * 100.0
	}

	// Carbon dioxide is still carried as a tension: it is far more soluble than
	// oxygen and its content is near enough linear in partial pressure over the
	// range a body survives, so the extra machinery would buy nothing.
	co2 += co2LoadPerSec * dt / bloodstream.CO2StoragePerMmHg

	saturation = helper.Clamp(saturation, bloodstream.MinSaturation, bloodstream.MaxSaturation)
	co2 = helper.Clamp(co2, bloodstream.MinPaCO2, bloodstream.MaxPaCO2)

	this.State().Set(stateSaturation, saturation)
	this.State().Set(statePaCO2, co2)
}
