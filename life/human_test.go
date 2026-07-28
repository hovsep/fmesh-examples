package main

import (
	"fmt"
	"testing"
	"time"

	"github.com/hovsep/fmesh"
	"github.com/hovsep/fmesh-examples/life/bloodstream"
	"github.com/hovsep/fmesh-examples/life/helper"
	"github.com/hovsep/fmesh-examples/life/organism/human/organ"
	"github.com/hovsep/fmesh-examples/simulation/session"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_HumanLiveness(t *testing.T) {
	tests := []struct {
		name       string
		assertions func(t *testing.T, sim *session.Session)
	}{
		{
			name: "human is alive",
			assertions: func(t *testing.T, sim *session.Session) {
				var observedIsAlive []bool

				aggState := simMesh(sim).ComponentByName("aggregated_state")
				require.NotNil(t, aggState)

				simMesh(sim).SetupHooks(func(hooks *fmesh.Hooks) {
					hooks.AfterRun(func(mesh *fmesh.FMesh) error {
						sig := aggState.OutputByName("human-Leon::is_alive").Signals().First()
						if sig == nil {
							return nil
						}
						// Liveness travels as 1/0 so it survives the numeric telemetry wire.
						alive, ok := helper.NumericPayload(sig)
						if !ok {
							return fmt.Errorf("is_alive is not numeric: %v", sig.PayloadOrNil())
						}
						observedIsAlive = append(observedIsAlive, alive > 0)
						return nil
					})
				})

				helper.RunSimulationAndThen(sim, time.Second, func() {
					assert.NotEmpty(t, observedIsAlive)
					assert.Contains(t, observedIsAlive, true)
					assert.NotContains(t, observedIsAlive, false)
				})
			},
		},
		{
			name: "heart is beating",
			assertions: func(t *testing.T, sim *session.Session) {
				var observedCardiacActivity []float64
				var observedHeartRate []int

				aggState := simMesh(sim).ComponentByName("aggregated_state")
				require.NotNil(t, aggState)

				simMesh(sim).SetupHooks(func(hooks *fmesh.Hooks) {
					hooks.AfterRun(func(mesh *fmesh.FMesh) error {
						sigAct := aggState.OutputByName("human-Leon::heart_cardiac_activation").Signals().First()
						if sigAct == nil {
							return nil
						}
						v, err := helper.AsF64(sigAct)
						if err != nil {
							return err
						}
						observedCardiacActivity = append(observedCardiacActivity, v)

						sigRate := aggState.OutputByName("human-Leon::heart_rate").Signals().First()
						if sigRate == nil {
							return nil
						}
						vRate, err := helper.AsInt(sigRate)
						if err != nil {
							return err
						}
						observedHeartRate = append(observedHeartRate, vRate)
						return nil
					})
				})

				helper.RunSimulationAndThen(sim, time.Second*10, func() {
					assert.NotEmpty(t, observedCardiacActivity)
					assert.NotEmpty(t, observedHeartRate)
					assertRPeaks(t, observedCardiacActivity)
				})
			},
		},
		{
			name: "pleural pressure is negative",
			assertions: func(t *testing.T, sim *session.Session) {
				var observedPleuralPressure []float64
				var observedRespiratoryRate []int

				aggState := simMesh(sim).ComponentByName("aggregated_state")
				require.NotNil(t, aggState)

				simMesh(sim).SetupHooks(func(hooks *fmesh.Hooks) {
					hooks.AfterRun(func(mesh *fmesh.FMesh) error {
						sigPressure := aggState.OutputByName("human-Leon::pleural_pressure").Signals().First()
						if sigPressure == nil {
							return nil
						}
						v, err := helper.AsF64(sigPressure)
						if err != nil {
							return err
						}
						observedPleuralPressure = append(observedPleuralPressure, v)

						sigRate := aggState.OutputByName("human-Leon::respiratory_rate").Signals().First()
						if sigRate == nil {
							return nil
						}
						vRate, err := helper.AsInt(sigRate)
						if err != nil {
							return err
						}
						observedRespiratoryRate = append(observedRespiratoryRate, vRate)
						return nil
					})
				})

				helper.RunSimulationAndThen(sim, time.Second*10, func() {
					assert.NotEmpty(t, observedPleuralPressure)
					assert.NotEmpty(t, observedRespiratoryRate)

					meanPressure := helper.Mean(observedPleuralPressure)
					meanRespiratoryRate := helper.Mean(observedRespiratoryRate)
					assert.Less(t, meanPressure, 0.0)
					assert.InDelta(t, organ.TidalRespiratoryRate, meanRespiratoryRate, 1)
				})
			},
		},
		{
			name: "lungs are ventilating",
			assertions: func(t *testing.T, sim *session.Session) {
				var observedLeftFlow, observedRightFlow []float64

				aggState := simMesh(sim).ComponentByName("aggregated_state")
				require.NotNil(t, aggState)

				simMesh(sim).SetupHooks(func(hooks *fmesh.Hooks) {
					hooks.AfterRun(func(mesh *fmesh.FMesh) error {
						sigLeft := aggState.OutputByName("human-Leon::lung_left_flow").Signals().First()
						if sigLeft == nil {
							return nil
						}
						vLeft, err := helper.AsF64(sigLeft)
						if err != nil {
							return err
						}
						observedLeftFlow = append(observedLeftFlow, vLeft)

						sigRight := aggState.OutputByName("human-Leon::lung_right_flow").Signals().First()
						if sigRight == nil {
							return nil
						}
						vRight, err := helper.AsF64(sigRight)
						if err != nil {
							return err
						}
						observedRightFlow = append(observedRightFlow, vRight)
						return nil
					})
				})

				helper.RunSimulationAndThen(sim, time.Second*10, func() {
					assert.NotEmpty(t, observedLeftFlow)
					assert.NotEmpty(t, observedRightFlow)
					assertBidirectionalFlow(t, observedLeftFlow, "left")
					assertBidirectionalFlow(t, observedRightFlow, "right")
				})
			},
		},
		{
			name: "inhaled air is changing while passing respiratory boundary",
			assertions: func(t *testing.T, sim *session.Session) {
				aggState := simMesh(sim).ComponentByName("aggregated_state")
				require.NotNil(t, aggState)

				var envP, envTemp, envHum float64
				var inspN, inspO, inspA, inspP, inspTemp, inspHum float64

				simMesh(sim).SetupHooks(func(hooks *fmesh.Hooks) {
					hooks.AfterRun(func(mesh *fmesh.FMesh) error {
						envSig := aggState.OutputByName("gas::environmental_gas").Signals().First()
						inspSig := aggState.OutputByName("human-Leon::inspired_gas").Signals().First()
						if envSig == nil || inspSig == nil {
							return nil
						}

						var err error
						_, _, _, envP, envTemp, envHum, err = helper.UnpackAir(envSig)
						if err != nil {
							return nil
						}
						inspN, inspO, inspA, inspP, inspTemp, inspHum, err = helper.UnpackAir(inspSig)
						if err != nil {
							return nil
						}
						return nil
					})
				})

				helper.RunSimulationAndThen(sim, 100*time.Millisecond, func() {
					// Inspired air should be warmer, cleaner, and more humid than environmental air
					assert.Greater(t, inspTemp, envTemp, "inspired air should be warmer")
					assert.Greater(t, inspHum, envHum, "inspired air should be more humid")
					assert.Less(t, inspP, envP, "inspired air should be cleaner (less pollution)")

					// Composition should still sum to 100% after rebalancing
					compSum := inspN + inspO + inspA + inspP
					assert.InDelta(t, 100.0, compSum, 1e-9, "composition should sum to 100%%")
				})
			},
		},
		{
			name: "blood gas levels are physiological",
			assertions: func(t *testing.T, sim *session.Session) {
				aggState := simMesh(sim).ComponentByName("aggregated_state")
				require.NotNil(t, aggState)

				var observedO2, observedCO2 []float64

				simMesh(sim).SetupHooks(func(hooks *fmesh.Hooks) {
					hooks.AfterRun(func(mesh *fmesh.FMesh) error {
						sig := aggState.OutputByName("human-Leon::venous_blood").Signals().First()
						if sig == nil {
							return nil
						}
						observedO2 = append(observedO2, sig.Scalars().ValueOrDefault("PaO2", 0))
						observedCO2 = append(observedCO2, sig.Scalars().ValueOrDefault("PaCO2", 0))
						return nil
					})
				})

				helper.RunSimulationAndThen(sim, 10*time.Second, func() {
					require.NotEmpty(t, observedO2, "should collect blood O2 samples")
					require.NotEmpty(t, observedCO2, "should collect blood CO2 samples")

					for _, v := range observedO2 {
						assert.GreaterOrEqual(t, v, bloodstream.MinPaO2, "PaO2 should stay above the survivable floor")
						assert.LessOrEqual(t, v, bloodstream.MaxPaO2, "PaO2 should stay below the ceiling")
					}
					for _, v := range observedCO2 {
						assert.GreaterOrEqual(t, v, bloodstream.MinPaCO2, "PaCO2 should stay above the floor")
						assert.LessOrEqual(t, v, bloodstream.MaxPaCO2, "PaCO2 should stay below the ceiling")
					}

					meanO2 := helper.Mean(observedO2)

					o2Min, o2Max := observedO2[0], observedO2[0]
					for _, v := range observedO2 {
						if v < o2Min {
							o2Min = v
						}
						if v > o2Max {
							o2Max = v
						}
					}
					co2Min, co2Max := observedCO2[0], observedCO2[0]
					for _, v := range observedCO2 {
						if v < co2Min {
							co2Min = v
						}
						if v > co2Max {
							co2Max = v
						}
					}
					o2Range := o2Max - o2Min
					co2Range := co2Max - co2Min

					// 0.5 is a meaningful exchange signal; floating-point drift alone cannot reach this
					assert.Greater(t, o2Range, 0.5, "O2 should fluctuate (gas exchange active)")
					assert.Greater(t, co2Range, 0.5, "CO2 should fluctuate (gas exchange active)")

					// Mean O2 should stay near the resting level, not drift to a clamp boundary
					assert.InDelta(t, bloodstream.NormalPaO2, meanO2, 50.0, "mean O2 should be near resting level")
				})
			},
		},
		{
			name: "exhaled gas is different from inspired",
			assertions: func(t *testing.T, sim *session.Session) {
				aggState := simMesh(sim).ComponentByName("aggregated_state")
				require.NotNil(t, aggState)

				var left, right int
				var inspO, inspTemp, inspHum float64

				simMesh(sim).SetupHooks(func(hooks *fmesh.Hooks) {
					hooks.AfterRun(func(mesh *fmesh.FMesh) error {
						leftSig := aggState.OutputByName("human-Leon::lung_left_exhaled_gas").Signals().First()
						rightSig := aggState.OutputByName("human-Leon::lung_right_exhaled_gas").Signals().First()
						inspSig := aggState.OutputByName("human-Leon::inspired_gas").Signals().First()
						if leftSig == nil || rightSig == nil || inspSig == nil {
							return nil
						}
						left++
						right++

						var err error
						_, inspO, _, _, inspTemp, inspHum, err = helper.UnpackAir(inspSig)
						if err != nil {
							return nil
						}
						return nil
					})
				})

				helper.RunSimulationAndThen(sim, 10*time.Second, func() {
					assert.Greater(t, left, 0, "should collect left exhaled gas samples")
					assert.Greater(t, right, 0, "should collect right exhaled gas samples")

					lSig := aggState.OutputByName("human-Leon::lung_left_exhaled_gas").Signals().First()
					lN, lO, lA, lP, lTemp, lHum, err := helper.UnpackAir(lSig)
					require.NoError(t, err)
					rSig := aggState.OutputByName("human-Leon::lung_right_exhaled_gas").Signals().First()
					rN, rO, rA, rP, rTemp, rHum, err := helper.UnpackAir(rSig)
					require.NoError(t, err)

					lCO2 := lSig.Scalars().ValueOrDefault("composition:carbon_dioxide", 0)
					rCO2 := rSig.Scalars().ValueOrDefault("composition:carbon_dioxide", 0)

					// Exhaled gas should be warmer, more humid, composition changed
					assert.Greater(t, lTemp, inspTemp, "left exhaled should be warmer than inspired")
					assert.Greater(t, rTemp, inspTemp, "right exhaled should be warmer than inspired")
					assert.Greater(t, lHum, inspHum, "left exhaled should be more humid")
					assert.Greater(t, rHum, inspHum, "right exhaled should be more humid")

					// Composition should be different (O2 consumed, CO2 produced)
					assert.Less(t, lO, inspO, "left exhaled should have less O2")
					assert.Less(t, rO, inspO, "right exhaled should have less O2")
					assert.Greater(t, lCO2, 0.0, "left exhaled should contain CO2")
					assert.Greater(t, rCO2, 0.0, "right exhaled should contain CO2")

					// Composition still sums to 100%
					assert.InDelta(t, 100.0, lN+lO+lA+lP+lCO2, 1e-9, "left exhaled composition should sum to 100%%")
					assert.InDelta(t, 100.0, rN+rO+rA+rP+rCO2, 1e-9, "right exhaled composition should sum to 100%%")
				})
			},
		},
		{
			name: "blood gas levels are dynamic over time",
			assertions: func(t *testing.T, sim *session.Session) {
				aggState := simMesh(sim).ComponentByName("aggregated_state")
				require.NotNil(t, aggState)

				var o2, co2 []float64

				simMesh(sim).SetupHooks(func(hooks *fmesh.Hooks) {
					hooks.AfterRun(func(mesh *fmesh.FMesh) error {
						sig := aggState.OutputByName("human-Leon::venous_blood").Signals().First()
						if sig == nil {
							return nil
						}
						o2 = append(o2, sig.Scalars().ValueOrDefault("PaO2", 0))
						co2 = append(co2, sig.Scalars().ValueOrDefault("PaCO2", 0))
						return nil
					})
				})

				// Run several breath cycles (one quiet breath ~= 5s) so the steady-state
				// window below spans multiple breaths.
				helper.RunSimulationAndThen(sim, 25*time.Second, func() {
					require.Greater(t, len(o2), 1000, "should collect enough samples")

					// Skip the initial settling transient and analyze the steady state,
					// so we prove the levels keep oscillating (not just drift once and flatten).
					steadyO2 := o2[len(o2)/2:]
					steadyCO2 := co2[len(co2)/2:]

					o2Min, o2Max := minMax(steadyO2)
					co2Min, co2Max := minMax(steadyCO2)

					// Tensions must keep moving (breathing in, organs consuming out).
					// The swing is small on purpose: arterial blood gases barely
					// move within a breath in a healthy body -- a couple of mmHg of
					// PaO₂ and well under one of PaCO₂ -- which is precisely what
					// makes them a stable reading to take.
					assert.Greater(t, o2Max-o2Min, 1.0, "steady-state PaO2 should keep oscillating")
					assert.Greater(t, co2Max-co2Min, 0.1, "steady-state PaCO2 should keep oscillating")

					// ... and repeatedly reverse direction (up and down), not drift monotonically.
					assert.Greater(t, countDirectionChanges(steadyO2), 3, "O2 should rise and fall repeatedly")
					assert.Greater(t, countDirectionChanges(steadyCO2), 3, "CO2 should rise and fall repeatedly")

					// ... and must not be pinned flat at a clamp boundary.
					assert.Greater(t, o2Min, bloodstream.MinPaO2, "O2 should not be stuck at the floor")
					assert.Less(t, o2Max, bloodstream.MaxPaO2, "O2 should not be stuck at the ceiling")
					assert.Less(t, co2Max, bloodstream.MaxPaCO2, "CO2 should not be stuck at the ceiling")
				})
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sim := newCommandableSim(t)

			if tt.assertions != nil {
				tt.assertions(t, sim)
			}
		})
	}
}

func assertRPeaks(t *testing.T, cardiacActivation []float64) {
	t.Helper()
	n := 0
	inPeak := false
	for _, v := range cardiacActivation {
		if v > 0.0 && !inPeak {
			n++
			inPeak = true
			continue
		}
		if v == 0.0 {
			inPeak = false
		}
	}
	assert.Greater(t, n, 0)
}

// minMax returns the minimum and maximum of a non-empty series.
func minMax(xs []float64) (float64, float64) {
	lo, hi := xs[0], xs[0]
	for _, v := range xs {
		if v < lo {
			lo = v
		}
		if v > hi {
			hi = v
		}
	}
	return lo, hi
}

// countDirectionChanges counts how many times the series reverses direction
// (a rising run followed by a falling run or vice versa), i.e. its oscillation count.
func countDirectionChanges(xs []float64) int {
	changes := 0
	prevDir := 0 // -1 down, +1 up, 0 flat
	for i := 1; i < len(xs); i++ {
		dir := 0
		if xs[i] > xs[i-1] {
			dir = 1
		} else if xs[i] < xs[i-1] {
			dir = -1
		}
		if dir != 0 && prevDir != 0 && dir != prevDir {
			changes++
		}
		if dir != 0 {
			prevDir = dir
		}
	}
	return changes
}

// assertBidirectionalFlow checks that lung flow crosses zero over the sample window (quiet breathing).
func assertBidirectionalFlow(t *testing.T, observedFlow []float64, side string) {
	t.Helper()
	var pos, neg bool
	for _, f := range observedFlow {
		if f > 0 {
			pos = true
		}
		if f < 0 {
			neg = true
		}
	}
	assert.True(t, pos && neg, "%s lung: expected both positive and negative flow over the run", side)
}
