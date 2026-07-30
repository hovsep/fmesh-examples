package main

import (
	"fmt"
	"testing"
	"time"

	"github.com/hovsep/fmesh"
	"github.com/hovsep/fmesh-examples/life/atmosphere"
	"github.com/hovsep/fmesh-examples/life/bloodstream"
	"github.com/hovsep/fmesh-examples/life/organism/human"
	da "github.com/hovsep/fmesh-examples/life/organism/human/distributed_anatomy"
	"github.com/hovsep/fmesh-examples/life/organism/human/organ"
	"github.com/hovsep/fmesh-examples/life/organism/human/physiology"
	"github.com/hovsep/fmesh-examples/life/plugin/damage"
	"github.com/hovsep/fmesh-examples/life/plugin/perfusion"
	"github.com/hovsep/fmesh-examples/life/plugin/receptor"
	"github.com/hovsep/fmesh-examples/simulation/command"
	"github.com/hovsep/fmesh-examples/simulation/mathx"
	"github.com/hovsep/fmesh-examples/simulation/session"
	"github.com/hovsep/fmesh-examples/simulation/simtest"
	"github.com/hovsep/fmesh/component"
	"github.com/hovsep/fmesh/signal"
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
	simtest.RunFor(sim, 30*time.Second, func() {
		require.NotEmpty(t, paO2)

		// Judge the second half, once the body has settled.
		steady := func(xs []float64) float64 { return mathx.Mean(xs[len(xs)/2:]) }

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
// The published resting figures -- brain 50, liver 40, muscle 50, heart 30, gut
// 20, kidneys 18, skin 12, diaphragm 3, pancreas 3, adrenals 2 mL/min -- come to
// 228, against a whole-body resting consumption of roughly 250. The remaining
// twenty-odd belong to bone, connective tissue and the organs this simulation
// still does not have, which is a gap worth being able to see.
//
// Note that gut and liver are quoted together in most sources, as the splanchnic
// bed, at about 60 mL/min; they are split here because the two are separate
// components and only one of them regulates blood sugar.
//
// This assertion is deliberately exact, so that giving the body a new organ
// fails it. That is not a nuisance: adding a tissue changes what the body burns,
// and the change should have to be acknowledged rather than absorbed silently.
func TestReference_WholeBodyOxygenConsumption(t *testing.T) {
	sim := newCommandableSim(t)
	body := human.Find(simMesh(sim))
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

	assert.Len(t, perfused, 10,
		"brain, heart, kidney, liver, pancreas, diaphragm, adrenals, gut, skin and muscle should be perfused")
	assert.InDelta(t, 228, total, 1, "the modelled organs should account for ~228 mL/min")
	assert.Less(t, total, 250.0, "which is still less than a whole body, as it should be")
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

	simtest.RunFor(sim, 90*time.Second, func() {
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
					if v, ok := signal.AsNumber(sig); ok {
						series[p] = append(series[p], v)
					}
				}
			}
			return nil
		})
	})

	simtest.RunFor(sim, 20*time.Second, func() {
		steady := func(p string) float64 {
			xs := series[p]
			require.NotEmpty(t, xs, p)
			return mathx.Mean(xs[len(xs)/2:])
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
		v, _ := signal.AsNumber(sig)
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
			elapsed += tickSeconds
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

	simtest.RunFor(sim, 30*time.Second, func() {
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
			elapsed += tickSeconds
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

	simtest.RunFor(sim, 305*time.Second, func() {
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
//
// The window is exactly one tick wide, and it has to be. It used to be 0.011
// against a tick of 0.01, which is wider than the gap between consecutive
// values -- so whether it matched one tick or two came down to how far the
// accumulated elapsed time had drifted from a round number. A test that fed the
// body one meal at t=300 fed it two at t=30, and the only visible symptom was a
// blood sugar that would not come down.
func within(elapsed, moment float64) bool {
	return elapsed >= moment && elapsed < moment+tickSeconds
}

// tickSeconds is the step every hook in this file counts in.
//
// Derived from the step the sims are actually built with rather than written
// out, because the two drifting apart is silent: the hooks would keep counting
// in hundredths while the body aged in twentieths, and every scheduled moment in
// this file would land at the wrong time.
const tickSeconds = float64(testTick) / float64(time.Second)

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
			v, _ := signal.AsNumber(sig)
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

	simtest.RunFor(sim, forDuration, func() {})
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
			elapsed += tickSeconds
			// Halfway through the bleeding (2 L at 25 mL/s takes 80 s).
			if within(elapsed, 40) {
				duringBleed = hemoglobin()
			}
			afterRefill = hemoglobin()
			return nil
		})
	})

	simtest.RunFor(sim, 300*time.Second, func() {
		assert.InDelta(t, bloodstream.NormalHemoglobin, duringBleed, 0.3,
			"haemoglobin should read normal while the patient is actively bleeding")
		assert.Less(t, afterRefill, duringBleed-1.0,
			"and should fall afterwards, as fluid replaces the volume without the cells")
	})
}

// TestReference_BreathingIsDrivenByCarbonDioxide is the control loop that
// governs ventilation, and the one most people get backwards.
//
// Breathing is regulated to hold PaCO₂ near 40 mmHg, not to hold oxygen up.
// Central chemoreceptors read the pH that CO₂ sets and answer steeply, which is
// why a few mmHg of retained CO₂ is unbearable while a considerable fall in
// oxygen goes unnoticed. It is also why hyperventilating before a breath-hold is
// dangerous: it removes the signal that would have made you surface without
// adding oxygen worth having.
func TestReference_BreathingIsDrivenByCarbonDioxide(t *testing.T) {
	sim := newCommandableSim(t)
	agg := simMesh(sim).ComponentByName("aggregated_state")
	blood := bodyComponent(t, sim, "da:blood_system")

	rate := func() float64 {
		if sig := agg.OutputByName("human-Leon::respiratory_rate").Signals().First(); sig != nil {
			v, _ := signal.AsNumber(sig)
			return v
		}
		return 0
	}

	var atRest, hypercapnic float64
	elapsed := 0.0
	simMesh(sim).SetupHooks(func(hooks *fmesh.Hooks) {
		hooks.AfterRun(func(*fmesh.FMesh) error {
			elapsed += tickSeconds
			switch {
			case within(elapsed, 15):
				atRest = rate()
				// A carbon dioxide load, as if from a rebreathed atmosphere.
				blood.State().Set("PaCO2", 60.0)
			case within(elapsed, 25):
				hypercapnic = rate()
			}
			return nil
		})
	})

	simtest.RunFor(sim, 30*time.Second, func() {
		assert.InDelta(t, 12, atRest, 3, "a resting adult breathes about 12 times a minute")
		assert.Greater(t, hypercapnic, atRest*2,
			"20 mmHg of retained CO₂ should more than double the respiratory rate")
	})
}

// TestReference_FastingBloodSugarIsAnEquilibrium checks that a resting body's
// blood sugar is held there by something, rather than simply written down.
//
// Guyton & Hall, ch. 79: a fasting adult holds blood glucose at about 90 mg/dL,
// and the liver releases roughly 2 mg per kg per minute to keep it there against
// continuous consumption. Those two figures are the same fact seen from either
// end, and the test asserts both -- because a model can easily hold the level
// right while getting the flux through it wrong, and it is the flux that
// everything else depends on.
//
// This used to be a decay toward 90 with a fifteen-minute half-life, which held
// the level and had no flux at all. Nothing could disturb it and nothing could
// break it.
func TestReference_FastingBloodSugarIsAnEquilibrium(t *testing.T) {
	sim := newCommandableSim(t)
	body := organComp(t, sim, "physiology:physiological_state")
	liver := organComp(t, sim, "organ:liver")

	simtest.RunFor(sim, 4*time.Minute, func() {
		glucose := body.State().Get(physiology.StateGlycemia).(float64)
		assert.InDelta(t, physiology.NormalGlycemia, glucose, 0.5,
			"a resting, fasting body should hold its blood sugar at the fasting level")

		// 2 mg/kg/min for a 70 kg adult, arriving in ~50 dL of blood.
		assert.InDelta(t, 2.8/60.0, organ.GlucoseFlux(liver), 0.001,
			"and the liver should be supplying it at the textbook basal rate")
	})
}

// TestReference_TheLiverDefendsBloodSugarDuringExercise is the control loop
// working: a disturbance, a hormone, a correction.
//
// Running raises the body's glucose consumption several-fold. Blood sugar dips,
// the alpha cells answer with glucagon, the liver empties glycogen into the
// blood, and the level comes back. The dip is real and so is the recovery --
// which is the shape of every genuine control loop and cannot be faked by a
// number that decays toward its setpoint.
//
// The dip is deeper and slower here than in a real runner, and the reason is
// worth knowing: glucagon takes minutes to accumulate, and it is the only
// counter-regulator this body has. A real one also gets a fast, feed-forward
// sympathetic signal the moment it starts running, rather than waiting to be
// told that sugar has already fallen.
func TestReference_TheLiverDefendsBloodSugarDuringExercise(t *testing.T) {
	if testing.Short() {
		t.Skip("multi-minute physiological run")
	}
	sim := newCommandableSim(t)
	body := organComp(t, sim, "physiology:physiological_state")
	liver := organComp(t, sim, "organ:liver")

	glucose := func() float64 { return body.State().Get(physiology.StateGlycemia).(float64) }

	nadir := 1000.0
	var glucagonAtNadir, final float64
	elapsed := 0.0

	simMesh(sim).SetupHooks(func(hooks *fmesh.Hooks) {
		hooks.AfterRun(func(*fmesh.FMesh) error {
			elapsed += tickSeconds
			switch {
			case within(elapsed, 30):
				sim.Do("activity:start 8")
			case elapsed > 30:
				final = glucose()
				if final < nadir {
					nadir = final
					glucagonAtNadir = receptor.Level(liver, bloodstream.HormoneGlucagon)
				}
			}
			return nil
		})
	})

	simtest.RunFor(sim, 16*time.Minute, func() {
		assert.Less(t, nadir, 85.0, "running should visibly draw blood sugar down")
		assert.Greater(t, nadir, 60.0,
			"but the counter-regulation should stop well short of hypoglycaemia")
		assert.Greater(t, glucagonAtNadir, 0.2,
			"and glucagon should be what stopped it")
		assert.Greater(t, final, nadir+2,
			"blood sugar should be recovering by the end, not still falling")
	})
}

// TestReference_WithoutThePancreasBloodSugarIsUndefended cuts the loop and shows
// that everything above depended on it.
//
// The islets are destroyed. Nothing else is touched: the liver is intact, it has
// its glycogen, and it goes on releasing glucose at its basal rate forever. What
// it no longer has is anyone to tell it that the body needs more. The same run
// that a healthy body absorbs with a ten-point dip takes this one into
// hypoglycaemia.
//
// This is the half of diabetes that is about signalling rather than sugar, and
// it is worth doing in front of an audience: one component removed, no other
// change anywhere, and a body that cannot look after itself.
func TestReference_WithoutThePancreasBloodSugarIsUndefended(t *testing.T) {
	if testing.Short() {
		t.Skip("multi-minute physiological run")
	}
	sim := newCommandableSim(t)
	body := organComp(t, sim, "physiology:physiological_state")
	liver := organComp(t, sim, "organ:liver")
	pancreas := organComp(t, sim, "organ:pancreas")

	var final, glucagon, flux float64
	elapsed := 0.0

	simMesh(sim).SetupHooks(func(hooks *fmesh.Hooks) {
		hooks.AfterRun(func(*fmesh.FMesh) error {
			elapsed += tickSeconds
			switch {
			case within(elapsed, 30):
				damage.Inflict(pancreas, 2*damage.CriticalLevel)
				sim.Do("activity:start 8")
			case elapsed > 30:
				final = body.State().Get(physiology.StateGlycemia).(float64)
				glucagon = receptor.Level(liver, bloodstream.HormoneGlucagon)
				flux = organ.GlucoseFlux(liver)
			}
			return nil
		})
	})

	simtest.RunFor(sim, 16*time.Minute, func() {
		assert.Less(t, final, 60.0,
			"without the islets, running should carry blood sugar into hypoglycaemia")
		assert.InDelta(t, 0, glucagon, 0.01,
			"because nothing is secreting glucagon any more")
		assert.InDelta(t, organ.BasalHepaticGlucoseOutput, flux, 0.001,
			"and the liver, hearing nothing, never leaves its basal rate")
	})
}

// TestReference_AMealIsClearedByInsulin is the whole chain in one run, and the
// other direction of the same loop.
//
// Food is absorbed into the blood; sugar climbs; the beta cells answer; the
// liver first stops adding sugar of its own and then reverses, pulling it out of
// the blood and into store. The reserve, which was falling all along because the
// body was burning it, turns and starts to fill.
//
// The shape is what a glucose tolerance curve looks like -- a rise over about
// half an hour, a rounded peak, a slow return -- and every part of it is
// produced, not scripted. The peak here is a little higher and earlier than a
// clinic would see, because this body treats a meal as pure rapidly-absorbed
// carbohydrate, which is the worst case rather than the usual one.
func TestReference_AMealIsClearedByInsulin(t *testing.T) {
	if testing.Short() {
		t.Skip("multi-minute physiological run")
	}
	sim := newCommandableSim(t)
	body := organComp(t, sim, "physiology:physiological_state")
	liver := organComp(t, sim, "organ:liver")

	var peakGlucose, finalGlucose, lowestFlux, finalReserve, peakInsulin float64
	lowestFlux = 1000
	elapsed := 0.0

	simMesh(sim).SetupHooks(func(hooks *fmesh.Hooks) {
		hooks.AfterRun(func(*fmesh.FMesh) error {
			elapsed += tickSeconds
			if within(elapsed, 30) {
				sim.Do("intake:food 400kcal")
			}
			if elapsed < 30 {
				return nil
			}
			finalGlucose = body.State().Get(physiology.StateGlycemia).(float64)
			finalReserve = body.State().Get(physiology.StateEnergyKcal).(float64)
			peakGlucose = max(peakGlucose, finalGlucose)
			lowestFlux = min(lowestFlux, organ.GlucoseFlux(liver))
			peakInsulin = max(peakInsulin, receptor.Level(liver, bloodstream.HormoneInsulin))
			return nil
		})
	})

	simtest.RunFor(sim, 36*time.Minute, func() {
		assert.Greater(t, peakGlucose, 130.0, "a large meal should carry blood sugar well above fasting")
		assert.Less(t, peakGlucose, 220.0,
			"but a body with a working pancreas should not reach diabetic levels")
		assert.Greater(t, peakInsulin, 0.2, "the beta cells should have answered it")
		assert.Less(t, lowestFlux, 0.0,
			"and the liver should have reversed: taking sugar out of the blood, not adding it")
		assert.Less(t, finalGlucose, peakGlucose,
			"blood sugar should be coming back down by the end")
		assert.Greater(t, finalReserve, physiology.StartingEnergyKcal,
			"and the meal should have reached storage, which is the only route it has")
	})
}

// TestReference_AlveolarGasEquation checks the equation at the points the
// textbooks and the expedition reports name.
//
// West, "High Life"; Guyton & Hall ch. 43. The equation is
//
//	PAO₂ = FiO₂ × (Pb − PH₂O) − PaCO₂ / R
//
// and the interesting thing about it is the last term, which is the only one a
// body controls. The Everest rows are the point of the whole test: at the
// summit, a body breathing normally arrives at a negative number -- the sum says
// there is no oxygen to be had -- and the same body hyperventilating to a PaCO₂
// of 10 arrives at something survivable. Nobody climbs Everest without oxygen by
// finding more air. They do it by breathing off carbon dioxide.
func TestReference_AlveolarGasEquation(t *testing.T) {
	const roomAir = bloodstream.RoomAirO2Fraction

	tests := []struct {
		barometric, fiO2, paCO2, want, tolerance float64
		note                                     string
	}{
		{760, roomAir, 40, 99.7, 1, "sea level, resting: the textbook ~100 mmHg"},
		{760, 1.00, 40, 663, 2, "sea level on pure oxygen: what a face mask buys"},
		{523, roomAir, 40, 50, 2, "3000 m, not yet acclimatised"},
		{523, roomAir, 30, 62.5, 2, "3000 m, hyperventilating: 10 mmHg of CO₂ buys 12.5 of O₂"},
		{253, roomAir, 40, -6.7, 1, "Everest summit at a normal PaCO₂: the sum says no"},
		{253, roomAir, 10, 30.8, 1, "Everest summit, hyperventilated: how it is actually done"},
	}

	for _, tt := range tests {
		got := bloodstream.AlveolarPO2At(tt.barometric, tt.fiO2, tt.paCO2)
		assert.InDelta(t, tt.want, got, tt.tolerance,
			"PAO₂ at Pb %g, FiO₂ %.2f, PaCO₂ %g (%s)", tt.barometric, tt.fiO2, tt.paCO2, tt.note)
	}

	// Every mmHg of carbon dioxide removed is worth 1/R = 1.25 mmHg of alveolar
	// oxygen, whatever the altitude. This is the exchange rate a climber lives on.
	atForty := bloodstream.AlveolarPO2At(400, roomAir, 40)
	atThirty := bloodstream.AlveolarPO2At(400, roomAir, 30)
	assert.InDelta(t, 12.5, atThirty-atForty, 0.1,
		"blowing off 10 mmHg of CO₂ should buy 12.5 mmHg of alveolar O₂")
}

// TestReference_PressureFallsWithAltitude checks the barometric formula against
// heights people actually go to.
func TestReference_PressureFallsWithAltitude(t *testing.T) {
	tests := []struct{ metres, want, tolerance float64 }{
		{0, 760, 1},
		{2400, 549, 15}, // a high city; mild hypoxia, no acclimatisation needed
		{5500, 361, 25}, // roughly Everest base camp, near half an atmosphere
		{8848, 253, 25}, // the summit
	}
	for _, tt := range tests {
		assert.InDelta(t, tt.want, atmosphere.PressureAtAltitude(tt.metres), tt.tolerance,
			"barometric pressure at %g m", tt.metres)
	}

	// Composition does not change with height, only pressure. This is the fact
	// the model is built to keep straight, and the one people get wrong.
	assert.Greater(t, atmosphere.PressureAtAltitude(0), atmosphere.PressureAtAltitude(3000),
		"air thins with height")
}

// TestReference_AltitudeThinsTheAirAndTheBodyAnswers is the end-to-end version:
// a body is moved to a high city and left there.
//
// Everything that follows was already in the model before altitude existed. The
// lungs work out what their alveoli can offer from the air that arrives; the
// blood loads toward it; the chemoreceptors, which were built to hold carbon
// dioxide at 40 and have never heard of mountains, find that they are being
// driven by hypoxia instead and breathe harder. The result is the textbook
// picture of acute altitude exposure: a saturation in the high eighties, a
// respiratory rate up by half, and a PaCO₂ blown down into respiratory
// alkalosis.
//
// What the model does not have is acclimatisation -- no shift in the
// dissociation curve, no extra red cells, no renal compensation for the
// alkalosis -- so this is a visitor on their first day, not a resident. That is
// also why the altitude here is a survivable one: at 5500 m this body, unable to
// acclimatise, injures its brain and dies, which is a fair description of what
// happens to someone helicoptered there and left.
func TestReference_AltitudeThinsTheAirAndTheBodyAnswers(t *testing.T) {
	if testing.Short() {
		t.Skip("multi-minute physiological run")
	}
	sim := newCommandableSim(t)
	agg := simMesh(sim).ComponentByName("aggregated_state")

	blood := func(name string) float64 {
		if sig := agg.OutputByName("human-Leon::venous_blood").Signals().First(); sig != nil {
			return sig.Scalars().ValueOrDefault(name, 0)
		}
		return 0
	}
	rate := func() float64 {
		if sig := agg.OutputByName("human-Leon::respiratory_rate").Signals().First(); sig != nil {
			v, _ := signal.AsNumber(sig)
			return v
		}
		return 0
	}

	var seaLevel, atAltitude map[string]float64
	snapshot := func() map[string]float64 {
		return map[string]float64{
			"SpO2": blood("SpO2"), "PaO2": blood("PaO2"),
			"PaCO2": blood("PaCO2"), "pH": blood("pH"), "rr": rate(),
		}
	}

	elapsed := 0.0
	simMesh(sim).SetupHooks(func(hooks *fmesh.Hooks) {
		hooks.AfterRun(func(*fmesh.FMesh) error {
			elapsed += tickSeconds
			switch {
			case within(elapsed, 25):
				seaLevel = snapshot()
			case within(elapsed, 30):
				sim.Do("altitude 2400")
			case elapsed > 240 && rate() > 0:
				atAltitude = snapshot()
			}
			return nil
		})
	})

	simtest.RunFor(sim, 250*time.Second, func() {
		require.NotNil(t, seaLevel)
		require.NotNil(t, atAltitude)

		// At sea level, a normal blood gas.
		assert.InDelta(t, 97, seaLevel["SpO2"], 3, "a healthy body at sea level")
		assert.InDelta(t, 40, seaLevel["PaCO2"], 3, "with a normal PaCO₂")

		// At 2400 m, thin air and a body working at it.
		assert.InDelta(t, 88, atAltitude["SpO2"], 4,
			"saturation in the high eighties is what a visitor to a high city has")
		assert.Less(t, atAltitude["PaO2"], 70.0, "and an arterial PO₂ well below sea level's")
		assert.Greater(t, atAltitude["rr"], seaLevel["rr"],
			"the hypoxic drive should have raised the respiratory rate")

		// Blowing off carbon dioxide is not a side effect. It is the mechanism:
		// the alkalosis is the price of the oxygen the hyperventilation buys.
		assert.Less(t, atAltitude["PaCO2"], 38.0,
			"breathing harder should have blown the CO₂ down")
		assert.Greater(t, atAltitude["pH"], 7.42,
			"which is a respiratory alkalosis, the classic finding at altitude")
	})
}

// TestReference_ABarochamberIsADropInForTheAtmosphere swaps the world.
//
// The habitat is built with a sealed chamber where the sky should be. Nothing
// inside the organism is parameterised for it, told about it, or aware of it:
// the body has an input port that air arrives on, and no way to ask where the
// air came from. One argument to getSimulationMeshIn is the whole of the change.
//
// The chamber is then used for the demonstration only a chamber can give. A
// hypobaric setting and a normobaric-hypoxic setting are chosen to deliver the
// same inspired oxygen tension by opposite means -- thin air at ordinary
// mixture, against ordinary air at a thin mixture -- and the body answers both
// the same way, because the alveolar gas equation multiplies the two together
// and has no way to know which one moved. That is the principle an altitude tent
// is sold on, and it is not obvious until you see a body fail to notice the
// difference.
func TestReference_ABarochamberIsADropInForTheAtmosphere(t *testing.T) {
	if testing.Short() {
		t.Skip("multi-minute physiological run")
	}

	// Both settings deliver ~70 mmHg of inspired oxygen, by opposite means.
	const (
		hypobaricPressure = 380.0 // half an atmosphere, ordinary air
		hypoxicOxygen     = 10.0  // a full atmosphere, thin mixture
	)
	assert.InDelta(t,
		bloodstream.RoomAirO2Fraction*(hypobaricPressure-bloodstream.WaterVaporPressure),
		hypoxicOxygen/100*(atmosphere.SeaLevelPressure-bloodstream.WaterVaporPressure),
		2.0, "the two chamber settings should offer the same inspired PO₂")

	settle := func(t *testing.T, setup func(sim *session.Session)) float64 {
		t.Helper()
		sim := newChamberSim(t)
		agg := simMesh(sim).ComponentByName("aggregated_state")

		var final float64
		elapsed := 0.0
		simMesh(sim).SetupHooks(func(hooks *fmesh.Hooks) {
			hooks.AfterRun(func(*fmesh.FMesh) error {
				elapsed += tickSeconds
				if within(elapsed, 20) {
					setup(sim)
				}
				if elapsed > 30 {
					if sig := agg.OutputByName("human-Leon::venous_blood").Signals().First(); sig != nil {
						final = sig.Scalars().ValueOrDefault("SpO2", 0)
					}
				}
				return nil
			})
		})
		simtest.RunFor(sim, 300*time.Second, func() {})
		return final
	}

	// A chamber nobody has touched is a room, and a body in it is a body indoors.
	indoors := settle(t, func(*session.Session) {})
	assert.Greater(t, indoors, 94.0,
		"a body in an unset chamber should be as healthy as one outdoors")

	thinAir := settle(t, func(sim *session.Session) {
		sim.Do(command.Line(fmt.Sprintf("chamber:pressure %g", hypobaricPressure)))
	})
	thinMixture := settle(t, func(sim *session.Session) {
		sim.Do(command.Line(fmt.Sprintf("chamber:oxygen %g", hypoxicOxygen)))
	})

	assert.Less(t, thinAir, 80.0, "half an atmosphere should desaturate a body badly")
	assert.Less(t, thinMixture, 80.0, "and so should a tenth-oxygen mixture at full pressure")
	assert.InDelta(t, thinAir, thinMixture, 6,
		"and to nearly the same degree, because the body multiplies the two and cannot tell them apart")
}

// TestReference_CarbonMonoxideTakesTheCarrierNotTheTension is the poison that
// this blood model was built to be able to express.
//
// Carbon monoxide binds hemoglobin some two hundred times more readily than
// oxygen and does not let go, so a share of the carrier is simply withdrawn from
// service. Nothing else moves. The oxygen dissolved in plasma is untouched, so
// the arterial PO₂ is normal; the hemoglobin that is still working is as
// saturated as ever, so the saturation is normal; and the patient is suffocating.
//
// A model that stored tension, or stored saturation, could not say this at all.
// It is the whole reason the blood carries content.
func TestReference_CarbonMonoxideTakesTheCarrierNotTheTension(t *testing.T) {
	const hemoglobin = 15.0

	healthy := bloodstream.OxygenContentAt(hemoglobin, bloodstream.NormalPaO2)
	poisoned := bloodstream.OxygenContentAt(
		bloodstream.EffectiveHemoglobin(hemoglobin, 0.50), bloodstream.NormalPaO2)

	assert.InDelta(t, healthy/2, poisoned, 0.5,
		"half the hemoglobin out of service should halve the oxygen carried")

	// And the two readings a clinician has both say nothing is wrong. This is the
	// entire clinical problem: 50% carboxyhemoglobin is a critical poisoning, and
	// a blood gas machine reports a normal arterial oxygen tension.
	assert.InDelta(t, bloodstream.NormalPaO2,
		bloodstream.TensionForContent(bloodstream.EffectiveHemoglobin(hemoglobin, 0.50), poisoned),
		0.5, "with a perfectly normal PaO₂")
	assert.Greater(t, bloodstream.SaturationAt(bloodstream.NormalPaO2), 96.0,
		"and a perfectly normal saturation of the hemoglobin that is left")

	// Anaemia of the same severity carries the same oxygen. The comparison is
	// worth making because it shows what the model is actually representing:
	// carbon monoxide is an acute anaemia that a blood count cannot see.
	anaemic := bloodstream.OxygenContentAt(hemoglobin/2, bloodstream.NormalPaO2)
	assert.InDelta(t, anaemic, poisoned, 0.1,
		"losing half the carrier is the same injury whichever way it is lost")
}

// TestReference_COClearanceIsDrivenByOxygen checks the three half-lives every
// toxicology reference quotes, and the reason there are three.
//
// Carbon monoxide and oxygen compete for the same site, so the speed at which
// the poison comes off is set by how hard the oxygen is pushing. That is why the
// treatment for carbon monoxide is oxygen, and why a hyperbaric chamber is worth
// wheeling a patient to.
func TestReference_COClearanceIsDrivenByOxygen(t *testing.T) {
	minutes := func(paO2 float64) float64 { return bloodstream.COHalfLifeAt(paO2) / 60 }

	assert.InDelta(t, 300, minutes(bloodstream.NormalPaO2), 5,
		"room air: about five hours")
	assert.InDelta(t, 85, minutes(600), 15,
		"a mask of pure oxygen at one atmosphere: a reported ~90 minutes")
	assert.InDelta(t, 28, minutes(2100), 10,
		"a hyperbaric chamber: a reported ~23 minutes")

	// The ordering is the part that has to be right, whatever the calibration.
	assert.Less(t, minutes(600), minutes(bloodstream.NormalPaO2), "oxygen speeds it up")
	assert.Less(t, minutes(2100), minutes(600), "and pressure speeds it up further")
}

// TestReference_ABoilerPoisonsAndAChamberRescues runs the whole thing in a body.
//
// A sealed room with a faulty appliance in it fills with carbon monoxide. The
// body breathes it, and the poisoning is invisible to everything that measures:
// the arterial oxygen tension does not move, and the saturation a pulse oximeter
// would report does not move either -- it drifts *up*, because the instrument
// cannot tell carboxyhemoglobin from oxyhemoglobin and adds them together.
// Meanwhile the oxygen actually delivered falls by a fifth.
//
// Then the room becomes a hyperbaric chamber, which is the treatment. Pure
// oxygen at three atmospheres puts the arterial tension past two thousand mmHg,
// and the oxygen merely dissolved in plasma -- a rounding error in a healthy
// body -- carries enough by itself to bring total content back above normal
// while the poison is still on most of the hemoglobin.
//
// Nothing in the body knows any of this. The lungs transfer what the air is
// carrying, the blood counts millilitres, and the clearance reads a tension.
func TestReference_ABoilerPoisonsAndAChamberRescues(t *testing.T) {
	if testing.Short() {
		t.Skip("multi-minute physiological run")
	}
	sim := newChamberSim(t)
	agg := simMesh(sim).ComponentByName("aggregated_state")

	blood := func(name string) float64 {
		if sig := agg.OutputByName("human-Leon::venous_blood").Signals().First(); sig != nil {
			return sig.Scalars().ValueOrDefault(name, 0)
		}
		return 0
	}
	snapshot := func() map[string]float64 {
		return map[string]float64{
			"SpO2": blood("SpO2"), "PaO2": blood("PaO2"),
			"CaO2": blood("CaO2"), "COHb": blood("COHb"),
		}
	}

	var clean, poisoned, treated map[string]float64
	elapsed := 0.0

	simMesh(sim).SetupHooks(func(hooks *fmesh.Hooks) {
		hooks.AfterRun(func(*fmesh.FMesh) error {
			elapsed += tickSeconds
			switch {
			case within(elapsed, 20):
				clean = snapshot()
				sim.Do("air:co 1500")
			case within(elapsed, 1200):
				poisoned = snapshot()
				// The room is cleared and pressurised on pure oxygen.
				sim.Do("air:co 0")
				sim.Do("chamber:pressure 2280")
				sim.Do("chamber:oxygen 100")
			case elapsed > 2300:
				treated = snapshot()
			}
			return nil
		})
	})

	simtest.RunFor(sim, 2350*time.Second, func() {
		require.NotNil(t, clean)
		require.NotNil(t, poisoned)
		require.NotNil(t, treated)

		// Twenty minutes in a contaminated room.
		assert.Greater(t, poisoned["COHb"], 12.0,
			"1500 ppm for twenty minutes should bind a serious fraction of the hemoglobin")
		assert.Less(t, poisoned["CaO2"], clean["CaO2"]*0.9,
			"and take a tenth or more of the oxygen supply with it")

		// And the instruments say nothing is wrong. This is the assertion the
		// whole model exists for.
		// The tension does drift down a little, and the reason is worth reading
		// rather than tuning away: this model holds one pool of blood, so what it
		// reports is nearer a mixed arterial-venous sample than a purely arterial
		// one. Tissues go on extracting the same millilitres from a smaller
		// carrier, so they take more tension with them. That is real -- venous
		// oxygen genuinely does fall in carbon monoxide poisoning, and it is why
		// the tissues are hypoxic -- but the clinically famous fact is about the
		// arterial number, and the arterial number stays in the range a blood gas
		// machine would call normal.
		assert.Greater(t, poisoned["PaO2"], 80.0,
			"while the arterial oxygen tension stays inside the normal range")
		assert.Less(t, clean["PaO2"]-poisoned["PaO2"], 10.0,
			"having barely moved, which is what makes the poisoning invisible")
		assert.GreaterOrEqual(t, poisoned["SpO2"], clean["SpO2"]-0.5,
			"and the pulse oximeter reads no lower -- it counts the poison as if it were oxygen")

		// The chamber.
		assert.Greater(t, treated["PaO2"], 1500.0,
			"pure oxygen at three atmospheres should drive the arterial tension past anything a mask can reach")
		// Eighteen minutes in the chamber against a half-life of about
		// twenty-six: a bit under half of it should be gone.
		assert.Less(t, treated["COHb"], poisoned["COHb"]*0.7,
			"which should have stripped a good share of the poison off the hemoglobin")
		assert.Greater(t, treated["CaO2"], clean["CaO2"],
			"and carried more oxygen than a healthy body does, with plasma making up what the hemoglobin cannot")
	})
}

// TestReference_TheSunWarmsTheAirAndTheBodySweats is the invariant this whole
// habitat exists to demonstrate, end to end and in one test.
//
// It is not a physiological reference in the way the others are -- no textbook
// quotes a number for it -- but it is the chain the simulation is *for*: an
// environmental factor changes, the world it describes changes with it, the body
// notices through an organ, and the body answers. Every link is a separate
// component that knows nothing about the others.
//
//	sun → air temperature → skin → core temperature → sweating
//
// Only the first link was ever missing. The sun published a UV index that the
// skin read directly, so a body would burn in the sun while standing in air the
// sun had no effect on. The air is downstream of the sun now, and the chain
// starts where it should.
func TestReference_TheSunWarmsTheAirAndTheBodySweats(t *testing.T) {
	if testing.Short() {
		t.Skip("multi-minute physiological run")
	}
	sim := newCommandableSim(t)
	agg := simMesh(sim).ComponentByName("aggregated_state")
	gas := simMesh(sim).ComponentByName("gas")

	// A hot day rather than a mild one. In a temperate 26 degrees the body
	// compensates the ambient completely -- which is correct, and which is why
	// the thermoneutral band exists -- so the air has to be genuinely hot before
	// it becomes a load the body must answer rather than absorb.
	sim.Do("temp:hot")    // 38 in the shade
	sim.Do("sun:hour 11") // an hour before the sun peaks

	reading := func(port string) float64 {
		if sig := agg.OutputByName("human-Leon::" + port).Signals().First(); sig != nil {
			return signal.AsFloat64OrDefault(sig, 0)
		}
		return 0
	}
	airTemperature := func() float64 {
		return gas.State().Get("temperature").(float64)
	}

	var airAtEarly, airAtNoon, coreAtNoon, sweatAtNoon float64
	elapsed := 0.0

	simMesh(sim).SetupHooks(func(hooks *fmesh.Hooks) {
		hooks.AfterRun(func(*fmesh.FMesh) error {
			elapsed += tickSeconds
			switch {
			case within(elapsed, 30):
				airAtEarly = airTemperature()
			case elapsed > 3000:
				airAtNoon = airTemperature()
				coreAtNoon = reading("body_temperature")
				sweatAtNoon = reading("sweat_rate")
			}
			return nil
		})
	})

	// The sky is set rather than waited for: a day is twenty-four hours long and
	// this test is not.
	simtest.RunFor(sim, 3100*time.Second, func() {
		require.NotZero(t, airAtEarly, "the air should have been sampled early")

		// Link one: the sun reaches the air.
		assert.Greater(t, airAtNoon, airAtEarly+1,
			"a climbing sun should warm the air above where it started")

		// Link two and three: the air and the sun reach the body, and the body
		// answers. Sweating begins above 37.2, so a sweating body is by
		// definition one whose core has been pushed past where it defends.
		assert.Greater(t, coreAtNoon, da.NormalSkinCoreTemperature,
			"a body in the hot sun should be warmer than a body at rest indoors")
		assert.Positive(t, sweatAtNoon,
			"and having got warm, it should sweat -- which is the whole point")
	})
}
