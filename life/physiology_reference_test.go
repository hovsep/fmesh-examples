package main

import (
	"testing"
	"time"

	"github.com/hovsep/fmesh"
	"github.com/hovsep/fmesh-examples/life/bloodstream"
	"github.com/hovsep/fmesh-examples/life/helper"
	"github.com/hovsep/fmesh-examples/life/organism/human"
	"github.com/hovsep/fmesh-examples/life/plugin/damage"
	"github.com/hovsep/fmesh-examples/life/plugin/perfusion"
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
// 50, skin 12, diaphragm 3 mL/min -- come to about 213, against a whole-body
// resting consumption of roughly 250. The rest belongs to organs this simulation
// does not have yet (liver most of all), which is a gap worth being able to see.
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

	assert.Len(t, perfused, 7, "brain, heart, kidney, diaphragm, gut, skin and muscle should be perfused")
	assert.InDelta(t, 213, total, 1, "the modelled organs should account for ~213 mL/min")
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
