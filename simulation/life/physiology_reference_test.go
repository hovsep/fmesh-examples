package main

import (
	"context"
	"testing"
	"time"

	"github.com/hovsep/fmesh"
	"github.com/hovsep/fmesh-examples/simulation/life/atmosphere"
	"github.com/hovsep/fmesh-examples/simulation/life/bloodstream"
	"github.com/hovsep/fmesh-examples/simulation/life/organism/human"
	da "github.com/hovsep/fmesh-examples/simulation/life/organism/human/distributed_anatomy"
	"github.com/hovsep/fmesh-examples/simulation/life/organism/human/organ"
	"github.com/hovsep/fmesh-examples/simulation/life/organism/human/physiology"
	"github.com/hovsep/fmesh-examples/simulation/life/plugin/perfusion"
	"github.com/hovsep/fmesh-examples/simulation/sim/mathx"
	"github.com/hovsep/fmesh-examples/simulation/sim/simtest"
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
	t.Parallel() // each test builds its own simulation and shares nothing
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
	t.Parallel() // each test builds its own simulation and shares nothing
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
	t.Parallel() // each test builds its own simulation and shares nothing
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
	t.Parallel() // each test builds its own simulation and shares nothing
	sim := newCommandableSim(t)

	aggState := simMesh(sim).ComponentByName("aggregated_state")
	require.NotNil(t, aggState)

	var paO2, paCO2, spO2, pH []float64
	simMesh(sim).SetupHooks(func(hooks *fmesh.Hooks) {
		hooks.AfterRun(func(context.Context, *fmesh.FMesh) error {
			sig := aggState.OutputByName("human-Leon::venous_blood").Signals().First()
			if sig == nil {
				return nil
			}
			s := sig.Meta()
			paO2 = append(paO2, s.ValueOrDefault("PaO2", 0.0))
			paCO2 = append(paCO2, s.ValueOrDefault("PaCO2", 0.0))
			spO2 = append(spO2, s.ValueOrDefault("SpO2", 0.0))
			pH = append(pH, s.ValueOrDefault("pH", 0.0))
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
	t.Parallel() // each test builds its own simulation and shares nothing
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
	t.Parallel()              // each test builds its own simulation and shares nothing
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
	t.Parallel() // each test builds its own simulation and shares nothing
	sim := newCommandableSim(t)
	body := human.Find(simMesh(sim))
	require.NotNil(t, body)

	inner := human.InnerMesh(body)
	require.NotNil(t, inner)

	var total float64
	perfused := map[string]float64{}
	for c := range inner.Components().All() {
		if demand := perfusion.O2PerMinute(c); demand > 0 {
			perfused[c.Name()] = demand
			total += demand
		}
	}

	assert.Len(t, perfused, 10,
		"brain, heart, kidney, liver, pancreas, diaphragm, adrenals, gut, skin and muscle should be perfused")
	assert.InDelta(t, 228, total, 1, "the modelled organs should account for ~228 mL/min")
	assert.Less(t, total, 250.0, "which is still less than a whole body, as it should be")
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
	t.Parallel() // each test builds its own simulation and shares nothing
	sim := newCommandableSim(t)
	agg := simMesh(sim).ComponentByName("aggregated_state")
	require.NotNil(t, agg)

	series := map[string][]float64{}
	ports := []string{"mean_arterial_pressure", "cardiac_output", "stroke_volume", "vascular_resistance", "heart_rate"}

	simMesh(sim).SetupHooks(func(hooks *fmesh.Hooks) {
		hooks.AfterRun(func(context.Context, *fmesh.FMesh) error {
			for _, p := range ports {
				if sig := agg.OutputByName("human-Leon::" + p).Signals().First(); sig != nil {
					if v, ok := sig.AsNumber(); ok {
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
	t.Parallel() // each test builds its own simulation and shares nothing
	sim := newCommandableSim(t)
	agg := simMesh(sim).ComponentByName("aggregated_state")
	blood := bodyComponent(t, sim, "da:blood_system")

	rate := func() float64 {
		if sig := agg.OutputByName("human-Leon::respiratory_rate").Signals().First(); sig != nil {
			v, _ := sig.AsNumber()
			return v
		}
		return 0
	}

	var atRest, hypercapnic float64
	elapsed := 0.0
	simMesh(sim).SetupHooks(func(hooks *fmesh.Hooks) {
		hooks.AfterRun(func(context.Context, *fmesh.FMesh) error {
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
	t.Parallel() // each test builds its own simulation and shares nothing
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
	t.Parallel() // each test builds its own simulation and shares nothing
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
	t.Parallel() // each test builds its own simulation and shares nothing
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
	t.Parallel() // each test builds its own simulation and shares nothing
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
	t.Parallel() // each test builds its own simulation and shares nothing
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
