package main

import (
	"testing"
	"time"

	"github.com/hovsep/fmesh"
	"github.com/hovsep/fmesh-examples/life/bloodstream"
	"github.com/hovsep/fmesh-examples/life/helper"
	"github.com/hovsep/fmesh-examples/life/organism/human"
	da "github.com/hovsep/fmesh-examples/life/organism/human/distributed_anatomy"
	"github.com/hovsep/fmesh-examples/life/plugin/damage"
	"github.com/hovsep/fmesh-examples/life/plugin/perfusion"
	"github.com/hovsep/fmesh-examples/simulation/command"
	"github.com/hovsep/fmesh/component"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// These tests check the simulation against values a physiologist can look up,
// rather than against whatever the model happened to produce. They are the
// argument that the numbers on screen mean something: if a change to the model
// moves a reading out of its clinical range, one of these fails and names the
// reference it broke.
//
// References are given per assertion. Where the simulation is deliberately
// simplified (no metabolic acid-base, no shunt fraction) the test says so
// instead of pretending to a precision the model does not have.

// TestReference_OxyhemoglobinDissociationCurve checks the curve every physiology
// course draws, at the points those courses name.
//
// Guyton & Hall, Textbook of Medical Physiology, ch. 41: the curve is sigmoid,
// with saturation ~97% at a normal arterial PaO₂ of ~95 mmHg, ~90% at 60 mmHg
// (the threshold at which supplemental oxygen is given), ~75% at 40 mmHg (mixed
// venous blood) and 50% at P50 = 26.6 mmHg.
func TestReference_OxyhemoglobinDissociationCurve(t *testing.T) {
	tests := []struct {
		paO2, wantSpO2, tolerance float64
		note                      string
	}{
		{paO2: 100, wantSpO2: 97.5, tolerance: 1.5, note: "alveolar-end capillary blood"},
		{paO2: 95, wantSpO2: 97.0, tolerance: 1.5, note: "normal arterial blood"},
		{paO2: 60, wantSpO2: 90.0, tolerance: 2.0, note: "the clinical corner: oxygen is given below this"},
		{paO2: 40, wantSpO2: 75.0, tolerance: 3.0, note: "mixed venous blood"},
		{paO2: 26.6, wantSpO2: 50.0, tolerance: 0.5, note: "P50 by definition"},
	}

	for _, tt := range tests {
		got := bloodstream.SaturationAt(tt.paO2)
		assert.InDelta(t, tt.wantSpO2, got, tt.tolerance,
			"SpO₂ at PaO₂ %g mmHg (%s)", tt.paO2, tt.note)
	}
}

// TestReference_CurveIsSigmoid checks the shape rather than the points: flat on
// the upper plateau, steep below the shoulder. This is why a patient can lose a
// third of their PaO₂ with little change in saturation, and then desaturate
// precipitously once past the shoulder -- the single most clinically important
// fact about the curve.
func TestReference_CurveIsSigmoid(t *testing.T) {
	// Upper plateau: 100 -> 80 mmHg costs only a couple of points.
	plateauLoss := bloodstream.SaturationAt(100) - bloodstream.SaturationAt(80)
	assert.Less(t, plateauLoss, 3.0, "the plateau should be flat: 20 mmHg costs almost no saturation")

	// Steep part: the same 20 mmHg below the shoulder costs far more.
	steepLoss := bloodstream.SaturationAt(60) - bloodstream.SaturationAt(40)
	assert.Greater(t, steepLoss, 10.0, "below the shoulder the same drop should cost much more")
	assert.Greater(t, steepLoss, 3*plateauLoss, "the steep part should be far steeper than the plateau")
}

// TestReference_VentilationSetsPH checks the respiratory arm of acid-base
// balance: pH moves about 0.08 per 10 mmHg of acute PaCO₂ change (the standard
// bedside rule). Hypoventilation is an acidosis, hyperventilation an alkalosis.
//
// The simulation models only the respiratory arm; metabolic disturbance and
// renal compensation are out of scope, so these are acute values.
func TestReference_VentilationSetsPH(t *testing.T) {
	assert.InDelta(t, 7.40, bloodstream.PHAt(40), 0.005, "normal PaCO₂ gives a normal pH")

	// Acute respiratory acidosis: hypoventilation at 50 mmHg.
	assert.InDelta(t, 7.32, bloodstream.PHAt(50), 0.01, "10 mmHg of retained CO₂ should cost ~0.08 pH")

	// Acute respiratory alkalosis: hyperventilation at 30 mmHg.
	assert.InDelta(t, 7.48, bloodstream.PHAt(30), 0.01, "blowing off 10 mmHg should raise pH ~0.08")

	// Direction, stated plainly, because it is the part students reverse.
	assert.Less(t, bloodstream.PHAt(60), bloodstream.PHAt(40), "retaining CO₂ acidifies the blood")
	assert.Greater(t, bloodstream.PHAt(20), bloodstream.PHAt(40), "blowing off CO₂ alkalinises it")
}

// TestReference_RestingArterialBloodGas runs the whole body and checks that a
// healthy person at rest reports an arterial blood gas a clinician would call
// normal: PaO₂ 80-100 mmHg, PaCO₂ 35-45, SpO₂ 95-100%, pH 7.35-7.45.
//
// This is the end-to-end version of the tests above: not the formulas in
// isolation, but what the breathing, circulation and metabolism actually settle
// on when left alone.
func TestReference_RestingArterialBloodGas(t *testing.T) {
	sim := newCommandableSim(t)

	aggState := simMesh(sim).ComponentByName("aggregated_state")
	require.NotNil(t, aggState)

	var paO2, paCO2, spO2, pH []float64
	simMesh(sim).SetupHooks(func(hooks *fmesh.Hooks) {
		hooks.AfterRun(func(*fmesh.FMesh) error {
			sig := aggState.OutputByName("human-Leon::venous_blood").Signals().First()
			if sig == nil {
				return nil
			}
			s := sig.Scalars()
			paO2 = append(paO2, s.ValueOrDefault("PaO2", 0))
			paCO2 = append(paCO2, s.ValueOrDefault("PaCO2", 0))
			spO2 = append(spO2, s.ValueOrDefault("SpO2", 0))
			pH = append(pH, s.ValueOrDefault("pH", 0))
			return nil
		})
	})

	// Long enough to settle out of the initial transient and average over
	// several breaths.
	helper.RunSimulationAndThen(sim, 30*time.Second, func() {
		require.NotEmpty(t, paO2)

		// Judge the second half, once the body has settled.
		steady := func(xs []float64) float64 { return helper.Mean(xs[len(xs)/2:]) }

		assert.InDelta(t, 90, steady(paO2), 15, "resting PaO₂ should be in the normal range (80-100 mmHg)")
		assert.InDelta(t, 40, steady(paCO2), 5, "resting PaCO₂ should be in the normal range (35-45 mmHg)")
		assert.InDelta(t, 97, steady(spO2), 3, "resting SpO₂ should be in the normal range (95-100%)")
		assert.InDelta(t, 7.40, steady(pH), 0.05, "resting pH should be in the normal range (7.35-7.45)")
	})
}

// TestReference_OxygenContentIsWhatTissuesGet is the distinction the blood model
// exists to make: partial pressure is a reading, content is the supply.
//
// A healthy arterial sample carries about 20 mL of oxygen per dL, almost all of
// it on hemoglobin (Hüfner 1.34 mL/g) and only ~0.3 dissolved. Halve the
// hemoglobin -- by bleeding, by anemia, or by binding it up with carbon monoxide
// -- and the supply halves while PaO₂ and SpO₂ stay exactly where they were.
// That is why a patient can be dying with a normal blood gas.
func TestReference_OxygenContentIsWhatTissuesGet(t *testing.T) {
	healthy := bloodstream.OxygenContent(15, 97, 95)
	assert.InDelta(t, 19.8, healthy, 0.5, "normal arterial oxygen content is ~20 mL/dL")

	// Hemoglobin carries essentially all of it.
	dissolved := bloodstream.DissolvedPerMmHg * 95
	assert.Less(t, dissolved/healthy, 0.02, "dissolved oxygen should be under 2% of the total")

	// The same blood gas, half the carrier.
	anemic := bloodstream.OxygenContent(7.5, 97, 95)
	assert.InDelta(t, healthy/2, anemic, 0.5,
		"halving hemoglobin should halve the supply, at an unchanged PaO₂ and SpO₂")
}

// TestReference_OxygenDeliveryAndExtraction checks the arithmetic that decides
// whether a body is in shock.
//
// Delivery is flow times content. A resting adult delivers about 1000 mL/min and
// consumes about 250, extracting a quarter. Tissues can raise extraction to
// roughly 60% before they must respire anaerobically, which is the line between
// compensated and decompensated shock.
func TestReference_OxygenDeliveryAndExtraction(t *testing.T) {
	const consumption = 250.0 // mL/min at rest

	healthy := bloodstream.OxygenDelivery(5.0, bloodstream.OxygenContent(15, 97, 95))
	assert.InDelta(t, 990, healthy, 60, "a resting adult delivers about 1 L of oxygen a minute")
	assert.InDelta(t, 0.25, consumption/healthy, 0.05, "and extracts about a quarter of it")

	// Class III hemorrhage: a third of the volume gone, the marrow has not had
	// time to replace anything, and the heart cannot fully make up the flow.
	bled := bloodstream.OxygenDelivery(4.0, bloodstream.OxygenContent(9, 97, 95))
	assert.Less(t, bled, healthy/2, "losing a third of the blood should more than halve delivery")

	extraction := consumption / bled
	assert.Greater(t, extraction, 0.5, "which forces the tissues to extract far more")
	assert.Less(t, extraction, 0.75, "though not yet beyond what extraction can cover")
}

// TestReference_WholeBodyOxygenConsumption checks that the body's oxygen use is
// not a number anybody typed in: it is the sum of what its organs ask for.
//
// The published resting figures -- brain 50, heart 30, kidneys 18, gut 50, muscle
// 50, skin 12, diaphragm 3, adrenals 2 mL/min -- come to about 215, against a
// whole-body resting consumption of roughly 250. The rest belongs to organs this
// simulation does not have yet (liver most of all), which is a gap worth being
// able to see.
//
// This assertion is deliberately exact, so that giving the body a new organ
// fails it. That is not a nuisance: adding a tissue changes what the body burns,
// and the change should have to be acknowledged rather than absorbed silently.
func TestReference_WholeBodyOxygenConsumption(t *testing.T) {
	sim := newCommandableSim(t)
	body := helper.FindHumanComponent(simMesh(sim))
	require.NotNil(t, body)

	inner := human.InnerMesh(body)
	require.NotNil(t, inner)

	var total float64
	perfused := map[string]float64{}
	require.NoError(t, inner.Components().ForEach(func(c *component.Component) error {
		if demand := perfusion.O2PerMinute(c); demand > 0 {
			perfused[c.Name()] = demand
			total += demand
		}
		return nil
	}))

	assert.Len(t, perfused, 8, "brain, heart, kidney, diaphragm, adrenals, gut, skin and muscle should be perfused")
	assert.InDelta(t, 215, total, 1, "the modelled organs should account for ~215 mL/min")
	assert.Less(t, total, 250.0, "which is less than a whole body: the liver is still missing")
}

// TestReference_ApneaDesaturates checks that a body which stops breathing goes
// hypoxic and hypercapnic at a plausible pace, rather than instantly or never.
//
// Both lungs are destroyed, which stops ventilation while metabolism continues.
// In a real apnoeic adult breathing room air, saturation holds for tens of
// seconds and then falls away as PaO₂ crosses the shoulder of the curve, while
// CO₂ climbs a few mmHg per minute.
func TestReference_ApneaDesaturates(t *testing.T) {
	if testing.Short() {
		t.Skip("multi-minute physiological run")
	}
	sim := newCommandableSim(t)

	lungs := []*component.Component{
		organComp(t, sim, "organ:lung_left"),
		organComp(t, sim, "organ:lung_right"),
	}

	aggState := simMesh(sim).ComponentByName("aggregated_state")
	require.NotNil(t, aggState)

	var lastPaO2, lastPaCO2, lastSpO2 float64
	destroyed := false
	simMesh(sim).SetupHooks(func(hooks *fmesh.Hooks) {
		hooks.AfterRun(func(*fmesh.FMesh) error {
			// Destroy both lungs on the first run: ventilation stops from here
			// on while metabolism carries on drawing oxygen out of the blood.
			if !destroyed {
				for _, lung := range lungs {
					damage.Inflict(lung, 2*damage.CriticalLevel)
				}
				destroyed = true
			}

			if sig := aggState.OutputByName("human-Leon::venous_blood").Signals().First(); sig != nil {
				lastPaO2 = sig.Scalars().ValueOrDefault("PaO2", 0)
				lastPaCO2 = sig.Scalars().ValueOrDefault("PaCO2", 0)
				lastSpO2 = sig.Scalars().ValueOrDefault("SpO2", 0)
			}
			return nil
		})
	})

	helper.RunSimulationAndThen(sim, 90*time.Second, func() {
		assert.Less(t, lastPaO2, 60.0,
			"after a minute and a half without breathing, PaO₂ should be well below the 60 mmHg shoulder")
		assert.Less(t, lastSpO2, 90.0, "and saturation should have fallen off the plateau")
		assert.Greater(t, lastPaCO2, bloodstream.NormalPaCO2,
			"carbon dioxide should accumulate once it cannot be blown off")
	})
}

// TestReference_RestingCirculation checks that a resting body settles on a
// circulation a clinician would call normal, and that it does so because the
// numbers agree rather than because any one of them was set.
//
// Mean arterial pressure 70-105 mmHg, cardiac output 4-8 L/min, stroke volume
// 55-90 mL, systemic vascular resistance 12-24 Wood units. The pressure is the
// product of the other two plus venous pressure, so getting all four right at
// once is the check.
func TestReference_RestingCirculation(t *testing.T) {
	sim := newCommandableSim(t)
	agg := simMesh(sim).ComponentByName("aggregated_state")
	require.NotNil(t, agg)

	series := map[string][]float64{}
	ports := []string{"mean_arterial_pressure", "cardiac_output", "stroke_volume", "vascular_resistance", "heart_rate"}

	simMesh(sim).SetupHooks(func(hooks *fmesh.Hooks) {
		hooks.AfterRun(func(*fmesh.FMesh) error {
			for _, p := range ports {
				if sig := agg.OutputByName("human-Leon::" + p).Signals().First(); sig != nil {
					if v, ok := helper.NumericPayload(sig); ok {
						series[p] = append(series[p], v)
					}
				}
			}
			return nil
		})
	})

	helper.RunSimulationAndThen(sim, 20*time.Second, func() {
		steady := func(p string) float64 {
			xs := series[p]
			require.NotEmpty(t, xs, p)
			return helper.Mean(xs[len(xs)/2:])
		}

		mapPressure := steady("mean_arterial_pressure")
		output := steady("cardiac_output")
		resistance := steady("vascular_resistance")

		assert.InDelta(t, 90, mapPressure, 20, "resting MAP should be 70-105 mmHg")
		assert.InDelta(t, 5.0, output, 1.5, "resting cardiac output should be 4-8 L/min")
		assert.InDelta(t, 70, steady("stroke_volume"), 15, "resting stroke volume should be 55-90 mL")
		assert.InDelta(t, 18, resistance, 6, "resting SVR should be 12-24 Wood units")
		assert.InDelta(t, 65, steady("heart_rate"), 20, "a resting heart rate")

		// The identity that ties them together: pressure is flow against
		// resistance, on top of the pressure blood returns at.
		assert.InDelta(t, mapPressure, output*resistance+da.CentralVenousPressure, 3,
			"MAP should be cardiac output x resistance + venous pressure")
	})
}

// TestReference_BaroreflexDefendsPressure is the compensated half of shock.
//
// A class III hemorrhage (30-40% of blood volume) costs the heart most of its
// preload, and stroke volume falls with it. What keeps the patient conscious is
// that pressure is not flow: the baroreflex answers within seconds by speeding
// the heart and tightening the vessels, so pressure falls far less than output
// does. This is why a bleeding patient can look deceptively well, and why a
// falling blood pressure is a late sign rather than an early one.
func TestReference_BaroreflexDefendsPressure(t *testing.T) {
	if testing.Short() {
		t.Skip("multi-second physiological run")
	}
	sim := newCommandableSim(t)
	agg := simMesh(sim).ComponentByName("aggregated_state")
	blood := bodyComponent(t, sim, "da:blood_system")

	read := func(port string) float64 {
		sig := agg.OutputByName("human-Leon::" + port).Signals().First()
		if sig == nil {
			return 0
		}
		v, _ := helper.NumericPayload(sig)
		return v
	}

	var before, after map[string]float64
	snapshot := func() map[string]float64 {
		return map[string]float64{
			"map": read("mean_arterial_pressure"),
			"co":  read("cardiac_output"),
			"sv":  read("stroke_volume"),
			"svr": read("vascular_resistance"),
			"hr":  read("heart_rate"),
		}
	}

	elapsed, bled := 0.0, false
	simMesh(sim).SetupHooks(func(hooks *fmesh.Hooks) {
		hooks.AfterRun(func(*fmesh.FMesh) error {
			elapsed += 0.01
			if elapsed > 9.9 && !bled {
				before = snapshot()
				// 1.5 L lost, and the hemoglobin that was in it.
				blood.State().Set("volume_l", 3.5)
				blood.State().Set("hemoglobin", 10.5)
				bled = true
			}
			if elapsed > 29 {
				after = snapshot()
			}
			return nil
		})
	})

	helper.RunSimulationAndThen(sim, 30*time.Second, func() {
		require.NotNil(t, before)
		require.NotNil(t, after)

		// Preload is gone, so each beat ejects far less.
		assert.Less(t, after["sv"], before["sv"]*0.6, "stroke volume should fall with preload")

		// The reflex answers on both arms.
		assert.Greater(t, after["hr"], before["hr"]*1.2, "the heart should speed up")
		assert.Greater(t, after["svr"], before["svr"]*1.15, "the vessels should tighten")

		// And the point of it all: pressure is defended far better than flow.
		pressureLoss := 1 - after["map"]/before["map"]
		flowLoss := 1 - after["co"]/before["co"]
		assert.Less(t, pressureLoss, flowLoss*0.6,
			"pressure should fall proportionally much less than cardiac output")
		assert.Greater(t, after["map"], 65.0,
			"a compensated class III hemorrhage should still be perfusing")

		// Without the reflex, pressure would be flow against an unchanged
		// resistance -- which is how much of the fall it actually prevented.
		uncompensated := after["co"]*before["svr"] + da.CentralVenousPressure
		assert.Greater(t, after["map"], uncompensated+5,
			"the reflex should be holding pressure well above what it would be without it")
	})
}

// TestReference_StressHormonesRunOnTwoClocks is why a body bothers with an
// endocrine system at all when it already has nerves.
//
// The same stressor provokes both adrenal outputs, but they answer on different
// timescales: adrenaline arrives within seconds and is cleared within minutes,
// cortisol takes minutes to arrive and hours to leave. A fright and a siege are
// not the same problem, and the chemistry that solves one does not solve the
// other.
//
// The body is bled, held there, and then given its volume back, so both the
// rise and the fall can be compared.
func TestReference_StressHormonesRunOnTwoClocks(t *testing.T) {
	if testing.Short() {
		t.Skip("multi-minute physiological run")
	}
	sim := newCommandableSim(t)
	agg := simMesh(sim).ComponentByName("aggregated_state")
	blood := bodyComponent(t, sim, "da:blood_system")

	level := func(hormone string) float64 {
		sig := agg.OutputByName("human-Leon::venous_blood").Signals().First()
		if sig == nil {
			return 0
		}
		return sig.Scalars().ValueOrDefault(hormone, 0)
	}

	var atRest, earlyAdrenaline, earlyCortisol, peakAdrenaline, peakCortisol float64
	var afterRecoveryAdrenaline, afterRecoveryCortisol float64

	elapsed := 0.0
	simMesh(sim).SetupHooks(func(hooks *fmesh.Hooks) {
		hooks.AfterRun(func(*fmesh.FMesh) error {
			elapsed += 0.01
			switch {
			case within(elapsed, 5):
				atRest = level(bloodstream.HormoneAdrenaline)
				blood.State().Set("volume_l", 3.5)
				blood.State().Set("hemoglobin", 10.5)
			case within(elapsed, 15):
				// Ten seconds in: the fast arm should already be working.
				earlyAdrenaline = level(bloodstream.HormoneAdrenaline)
				earlyCortisol = level(bloodstream.HormoneCortisol)
			case within(elapsed, 120):
				peakAdrenaline = level(bloodstream.HormoneAdrenaline)
				peakCortisol = level(bloodstream.HormoneCortisol)
				// Transfused: the stressor is gone.
				blood.State().Set("volume_l", bloodstream.NormalBloodVolume)
				blood.State().Set("hemoglobin", bloodstream.NormalHemoglobin)
			case within(elapsed, 300):
				afterRecoveryAdrenaline = level(bloodstream.HormoneAdrenaline)
				afterRecoveryCortisol = level(bloodstream.HormoneCortisol)
			}
			return nil
		})
	})

	helper.RunSimulationAndThen(sim, 305*time.Second, func() {
		// An unstressed body is not marinating in stress hormones.
		assert.InDelta(t, 0, atRest, 0.01, "nothing should circulate at rest")

		// Ten seconds after the injury, adrenaline is already doing its job and
		// cortisol has barely started.
		assert.Greater(t, earlyAdrenaline, 0.05, "adrenaline should arrive within seconds")
		assert.Less(t, earlyCortisol, earlyAdrenaline/5,
			"cortisol should still be far behind at ten seconds")

		// Both keep climbing while the stressor lasts, adrenaline far higher.
		assert.Greater(t, peakAdrenaline, earlyAdrenaline, "adrenaline should keep rising under a sustained stressor")
		assert.Greater(t, peakCortisol, earlyCortisol, "so should cortisol, more slowly")
		assert.Greater(t, peakAdrenaline, peakCortisol*5, "adrenaline should dominate the acute response")

		// And the point: three minutes after the stressor is removed, the fast
		// hormone has largely gone and the slow one has not.
		assert.Less(t, afterRecoveryAdrenaline, peakAdrenaline*0.5,
			"adrenaline should clear within minutes of the stressor ending")
		assert.Greater(t, afterRecoveryCortisol, peakCortisol*0.8,
			"cortisol should still be circulating long after")
	})
}

// within reports whether the run has just passed a given moment, so a hook can
// act on it exactly once.
func within(elapsed, moment float64) bool {
	return elapsed >= moment && elapsed < moment+0.011
}

// bleedAndWatch runs a hemorrhage of the given size and reports the worst
// pressure reached, plus the state of the body at the end.
func bleedAndWatch(t *testing.T, volume string, forDuration time.Duration) (worstMAP float64, final map[string]float64) {
	t.Helper()

	sim := newCommandableSim(t)
	agg := simMesh(sim).ComponentByName("aggregated_state")
	require.NotNil(t, agg)

	sim.Do(command.Line("trauma:bleed " + volume))

	read := func(p string) float64 {
		if sig := agg.OutputByName("human-Leon::" + p).Signals().First(); sig != nil {
			v, _ := helper.NumericPayload(sig)
			return v
		}
		return 0
	}
	scalar := func(name string) float64 {
		if sig := agg.OutputByName("human-Leon::venous_blood").Signals().First(); sig != nil {
			return sig.Scalars().ValueOrDefault(name, 0)
		}
		return 0
	}

	worstMAP = 1000
	settled := false
	simMesh(sim).SetupHooks(func(hooks *fmesh.Hooks) {
		hooks.AfterRun(func(*fmesh.FMesh) error {
			// Ignore the first moments, before the circulation has published
			// anything, or the "worst" pressure would be a zero that never was.
			if p := read("mean_arterial_pressure"); p > 0 {
				settled = true
				worstMAP = min(worstMAP, p)
			}
			if settled {
				final = map[string]float64{
					"map":           read("mean_arterial_pressure"),
					"hr":            read("heart_rate"),
					"co":            read("cardiac_output"),
					"kidney_damage": read("kidney_damage"),
					"brain_damage":  read("brain_damage"),
					"volume":        scalar("volume_l"),
					"hemoglobin":    scalar("hemoglobin"),
					"spo2":          scalar("SpO2"),
					"adrenaline":    scalar(bloodstream.HormoneAdrenaline),
				}
			}
			return nil
		})
	})

	helper.RunSimulationAndThen(sim, forDuration, func() {})
	return worstMAP, final
}

// TestReference_ClassIIIHemorrhageIsCompensated: losing 30-40% of blood volume
// is survivable, and survivable specifically because of the reflexes.
//
// The patient is tachycardic and vasoconstricted, their cardiac output is well
// down, and their blood pressure is very nearly normal. That combination is the
// trap the ATLS classification exists to teach: a normal blood pressure does not
// mean a stable patient.
func TestReference_ClassIIIHemorrhageIsCompensated(t *testing.T) {
	if testing.Short() {
		t.Skip("multi-minute physiological run")
	}
	worstMAP, final := bleedAndWatch(t, "2000ml", 300*time.Second)

	assert.Greater(t, worstMAP, 60.0, "a class III hemorrhage should stay above the perfusion floor")
	assert.Greater(t, final["hr"], 100.0, "the patient should be tachycardic")
	assert.Less(t, final["co"], 4.0, "with a cardiac output well below normal")
	assert.Greater(t, final["adrenaline"], 0.3, "and a substantial adrenaline response")

	// Nothing is injured: this is compensation, not damage.
	//
	// Not quite zero, though. Every organ ages the whole time, so five minutes of
	// living leaves about 1e-7 of damage behind. Hypoperfusion injures four
	// orders of magnitude faster than that, so the threshold separates an organ
	// that merely got older from one that was hurt.
	const agingOnly = 1e-4
	assert.Less(t, final["kidney_damage"], agingOnly, "a compensated hemorrhage should not injure the kidney")
	assert.Less(t, final["brain_damage"], agingOnly, "nor the brain")

	// And the blood gas -- the thing a monitor shows -- is untouched throughout.
	assert.Greater(t, final["spo2"], 94.0,
		"saturation stays normal while the patient bleeds: the supply fails, not the gas exchange")
}

// TestReference_ClassIVHemorrhageDecompensates: past roughly 40% the reflexes
// run out of room, and the pressure they were defending collapses.
//
// Below about 60 mmHg the organs that autoregulate can no longer hold their own
// supply. The kidney goes first -- it is given a fifth of the cardiac output to
// filter with and is the first bed sacrificed -- which is why acute kidney injury
// is the classic complication of a shock the patient survived.
func TestReference_ClassIVHemorrhageDecompensates(t *testing.T) {
	if testing.Short() {
		t.Skip("multi-minute physiological run")
	}
	worstMAP, final := bleedAndWatch(t, "3000ml", 300*time.Second)

	assert.Less(t, worstMAP, 60.0, "a class IV hemorrhage should break through the perfusion floor")
	assert.Greater(t, final["hr"], 140.0, "the heart should be running as fast as it can")
	assert.Greater(t, final["adrenaline"], 0.9, "and the glands should be saturated")

	// The kidney is injured, and stays injured: damage does not heal.
	assert.Greater(t, final["kidney_damage"], 0.05,
		"hypoperfusion should injure the kidney")
	assert.Greater(t, final["kidney_damage"], final["brain_damage"],
		"the kidney should suffer before the brain, which defends its own supply harder")

	// Saturation is still normal. Nothing is wrong with this patient's lungs.
	assert.Greater(t, final["spo2"], 94.0, "a dying patient with a perfect blood gas")
}

// TestReference_HemoglobinFallsAfterTheBleedingStops is a detail worth having
// because it catches people out.
//
// What leaves a wound is whole blood, so the concentration of what remains is
// unchanged: a haemoglobin taken during an acute bleed reads normal. It falls
// afterwards, as the body pulls fluid in from its tissues to replace the volume
// and dilutes the red cells that are left.
func TestReference_HemoglobinFallsAfterTheBleedingStops(t *testing.T) {
	if testing.Short() {
		t.Skip("multi-minute physiological run")
	}
	sim := newCommandableSim(t)
	agg := simMesh(sim).ComponentByName("aggregated_state")

	sim.Do("trauma:bleed 2000ml")

	hemoglobin := func() float64 {
		if sig := agg.OutputByName("human-Leon::venous_blood").Signals().First(); sig != nil {
			return sig.Scalars().ValueOrDefault("hemoglobin", 0)
		}
		return 0
	}

	var duringBleed, afterRefill float64
	elapsed := 0.0
	simMesh(sim).SetupHooks(func(hooks *fmesh.Hooks) {
		hooks.AfterRun(func(*fmesh.FMesh) error {
			elapsed += 0.01
			// Halfway through the bleeding (2 L at 25 mL/s takes 80 s).
			if within(elapsed, 40) {
				duringBleed = hemoglobin()
			}
			afterRefill = hemoglobin()
			return nil
		})
	})

	helper.RunSimulationAndThen(sim, 300*time.Second, func() {
		assert.InDelta(t, bloodstream.NormalHemoglobin, duringBleed, 0.3,
			"haemoglobin should read normal while the patient is actively bleeding")
		assert.Less(t, afterRefill, duringBleed-1.0,
			"and should fall afterwards, as fluid replaces the volume without the cells")
	})
}
