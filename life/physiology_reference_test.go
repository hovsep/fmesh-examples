package main

import (
	"testing"
	"time"

	"github.com/hovsep/fmesh"
	"github.com/hovsep/fmesh-examples/life/helper"
	da "github.com/hovsep/fmesh-examples/life/organism/human/distributed_anatomy"
	"github.com/hovsep/fmesh-examples/life/plugin/damage"
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
		got := da.SaturationAt(tt.paO2)
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
	plateauLoss := da.SaturationAt(100) - da.SaturationAt(80)
	assert.Less(t, plateauLoss, 3.0, "the plateau should be flat: 20 mmHg costs almost no saturation")

	// Steep part: the same 20 mmHg below the shoulder costs far more.
	steepLoss := da.SaturationAt(60) - da.SaturationAt(40)
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
	assert.InDelta(t, 7.40, da.PHAt(40), 0.005, "normal PaCO₂ gives a normal pH")

	// Acute respiratory acidosis: hypoventilation at 50 mmHg.
	assert.InDelta(t, 7.32, da.PHAt(50), 0.01, "10 mmHg of retained CO₂ should cost ~0.08 pH")

	// Acute respiratory alkalosis: hyperventilation at 30 mmHg.
	assert.InDelta(t, 7.48, da.PHAt(30), 0.01, "blowing off 10 mmHg should raise pH ~0.08")

	// Direction, stated plainly, because it is the part students reverse.
	assert.Less(t, da.PHAt(60), da.PHAt(40), "retaining CO₂ acidifies the blood")
	assert.Greater(t, da.PHAt(20), da.PHAt(40), "blowing off CO₂ alkalinises it")
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
		assert.Greater(t, lastPaCO2, da.NormalPaCO2,
			"carbon dioxide should accumulate once it cannot be blown off")
	})
}
