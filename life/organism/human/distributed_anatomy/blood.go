package da

import (
	"fmt"
	"math"

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
	stateVentilation  common.State = "ventilation"

	// Circulating hormone levels, keyed by hormone name.
	stateHormonePrefix              = "hormone_"
	stateDt            common.State = "dt" // last known tick duration (seconds)
)

const (
	// Inhale approach rates: a fractional-per-second pull toward what the lungs
	// offer, proportional to the remaining gap, so nothing overshoots.
	o2InhaleGain = 6.0
)

// defaultDt matches the habitat's per-tick duration (10 ms); used as a fallback
// before the first time signal is seen.
const defaultDt = 0.01

func GetBloodSystem() (*component.Component, error) {
	c, err := component.New("da:blood_system",
		component.WithDescription("Blood system tracking O2 and CO2 levels"),
		component.WithInputs(
			"time",
			"airflow",      // lung airflow: >0 inhaling (fresh air), <0 exhaling
			"alveolar_po2", // what the lungs are offering, mmHg (one signal per lung)
			"secretions",   // shared bus: any organ emits labeled substance signals here
			"glucose",      // current blood sugar from the reservoir, carried to organs
			"blood_loss",   // mL leaving the body through a wound
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
			state.Set(stateVentilation, 1.0)
			for _, hormone := range bloodstream.Hormones {
				state.Set(hormoneState(hormone), 0.0)
			}
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

	// Blood leaving through a wound takes its hemoglobin with it, which is the
	// part that matters: what is lost is not water but carrying capacity.
	if in := this.InputByName("blood_loss"); in.HasSignals() {
		var lost float64
		in.Signals().ForEach(func(sig *signal.Signal) error {
			lost += helper.AsF64OrDefault(sig, 0)
			return nil
		})
		bleed(this, lost)
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
			WithScalar("glucose_level", this.State().Get(stateGlucoseLevel).(float64)).
			WithScalars(circulatingHormones(this)),
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
	//
	// How much of that happens depends on how much air is actually moving. This
	// is what makes breathing worth controlling: without it, gasping and quiet
	// breathing would exchange identically, a chemoreflex would have nothing to
	// achieve, and hypoventilation would not be dangerous.
	// How much air is moving, right now. It is deliberately not smoothed: carbon
	// dioxide leaves the body in bursts, one per breath, while the tissues make
	// it continuously. Averaging the airflow first would cancel the two against
	// each other and hold PaCO₂ almost perfectly flat -- which is not what an
	// arterial line shows, and which cost this model most of its respiratory
	// swing before the tests noticed.
	ventilation := helper.Clamp(
		math.Abs(netFlow)/bloodstream.ReferenceVentilation, 0, bloodstream.MaxVentilationFactor)
	this.State().Set(stateVentilation, ventilation)

	// Oxygen is loaded on inhale, toward what the alveoli can offer. Moving more
	// air loads it faster, but only up to a point: blood leaving a normally
	// ventilated lung is already nearly saturated, which is why hyperventilating
	// blows off carbon dioxide without adding much oxygen.
	if netFlow > 0 {
		target := bloodstream.SaturationAt(alveolarPO2(this) - bloodstream.AaGradient)
		saturation += (target - saturation) * o2InhaleGain * ventilation * dt
	}

	// Carbon dioxide leaves in proportion to how much air is moving and how much
	// of it there is to carry away, so its arterial tension settles wherever
	// production divided by ventilation puts it.
	co2 -= bloodstream.CO2EliminationPerMmHg * ventilation * co2 * dt / bloodstream.CO2StoragePerMmHg

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
	hormoneRates := map[string]float64{}
	this.InputByName("secretions").Signals().ForEach(func(sig *signal.Signal) error {
		rate := helper.AsF64OrDefault(sig, 0)
		switch sig.Labels().ValueOrDefault(bloodstream.SubstanceLabel, "") {
		case bloodstream.SubstanceO2Draw:
			o2DrawPerSec += rate
		case bloodstream.SubstanceCO2Load:
			co2LoadPerSec += rate
		case bloodstream.SubstanceHormone:
			hormoneRates[sig.Labels().ValueOrDefault(bloodstream.HormoneLabel, "")] += rate
		}
		return nil
	})

	updateHormones(this, hormoneRates, dt)

	refill(this, dt)

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

func hormoneState(hormone string) common.State {
	return common.State(stateHormonePrefix + hormone)
}

// updateHormones folds this tick's secretion into each circulating level and
// lets the rest decay away.
//
// A hormone level is therefore never a command that has to be cancelled: a
// gland that stops secreting is a level that fades on its own, at whatever pace
// that hormone is cleared. Adrenaline is gone in minutes; cortisol takes hours.
// That difference is the whole reason a body has both.
func updateHormones(this *component.Component, secreted map[string]float64, dt float64) {
	for _, hormone := range bloodstream.Hormones {
		key := hormoneState(hormone)
		level, _ := this.State().Get(key).(float64)

		level += secreted[hormone] * dt
		level = helper.DecayToward(level, 0, dt, bloodstream.HormoneHalfLife(hormone))

		this.State().Set(key, helper.Clamp(level, 0, 1))
	}
}

// circulatingHormones is what the blood is currently carrying, ready to ride on
// the venous signal for any organ with the receptors to feel it.
func circulatingHormones(this *component.Component) map[string]float64 {
	levels := make(map[string]float64, len(bloodstream.Hormones))
	for _, hormone := range bloodstream.Hormones {
		levels[hormone], _ = this.State().Get(hormoneState(hormone)).(float64)
	}
	return levels
}

// MinSurvivableVolume is the blood a circulation cannot go below and still be a
// circulation, in litres.
const MinSurvivableVolume = 1.5 // litres

// bleed takes whole blood out of the circulation.
//
// Whole blood, not water: what leaves a wound is blood, cells and all, so the
// concentration of what remains is unchanged. This is the reason a haemoglobin
// measured early in a haemorrhage is a trap -- it reads normal while the patient
// is exsanguinating, because the patient is losing red cells and plasma in the
// proportion they were already in.
func bleed(this *component.Component, lostMl float64) {
	if lostMl <= 0 {
		return
	}

	volume := this.State().Get(stateVolume).(float64)
	this.State().Set(stateVolume, max(volume-lostMl/1000.0, MinSurvivableVolume))
}

// transcapillaryRefillHalfLifeSec is how fast the body pulls fluid in from its
// own tissues to replace a lost volume.
//
// It is slow -- this is an hours-long process, not a minutes-long one -- and its
// slowness is the whole clinical picture of the first hour after an injury: the
// pressure has to be defended by the heart and the vessels because the volume is
// not coming back yet. When it does come back it arrives as fluid without cells,
// which is why the haemoglobin falls *after* the bleeding has stopped.
const transcapillaryRefillHalfLifeSec = 20 * 60.0

// refill moves interstitial fluid into the circulation, toward the volume the
// body wants. It buys back pressure at the cost of concentration.
func refill(this *component.Component, dt float64) {
	volume := this.State().Get(stateVolume).(float64)
	if volume >= bloodstream.NormalBloodVolume {
		return
	}

	restored := helper.DecayToward(volume, bloodstream.NormalBloodVolume, dt, transcapillaryRefillHalfLifeSec)

	// The red cells are however many there were; they are now spread through a
	// larger volume.
	hemoglobin := this.State().Get(stateHemoglobin).(float64)
	if restored > 0 {
		this.State().Set(stateHemoglobin, hemoglobin*volume/restored)
	}
	this.State().Set(stateVolume, restored)
}

// alveolarPO2 is what the lungs are currently offering, averaged across them.
//
// Arterial blood is the mixture of what came back from every ventilated part of
// the lung, so an average is the right summary -- and it is why one destroyed
// lung halves neither the oxygen nor the body, but does drag the average down.
// Before a lung has said anything, the sea-level figure stands in.
func alveolarPO2(this *component.Component) float64 {
	in := this.InputByName("alveolar_po2")
	if in == nil || !in.HasSignals() {
		return bloodstream.SeaLevelAlveolarPO2
	}

	var sum float64
	var n int
	_ = in.Signals().ForEach(func(sig *signal.Signal) error {
		sum += helper.AsF64OrDefault(sig, bloodstream.SeaLevelAlveolarPO2)
		n++
		return nil
	})
	if n == 0 {
		return bloodstream.SeaLevelAlveolarPO2
	}
	return sum / float64(n)
}
