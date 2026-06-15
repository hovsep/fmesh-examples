package main

import (
	"context"
	"testing"
	"time"

	"github.com/hovsep/fmesh"
	"github.com/hovsep/fmesh-examples/life/helper"
	da "github.com/hovsep/fmesh-examples/life/organism/human/distributed_anatomy"
	"github.com/hovsep/fmesh-examples/life/organism/human/organ"
	"github.com/hovsep/fmesh-examples/simulation/step_sim"
	"github.com/hovsep/fmesh-examples/simulation/step_sim/sink"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_HumanLiveness(t *testing.T) {
	tests := []struct {
		name       string
		assertions func(t *testing.T, sim *step_sim.Simulation)
	}{
		{
			name: "human is alive",
			assertions: func(t *testing.T, sim *step_sim.Simulation) {
				var observedIsAlive []bool

				aggState := sim.FM.ComponentByName("aggregated_state")
				require.NotNil(t, aggState)

				sim.FM.SetupHooks(func(hooks *fmesh.Hooks) {
					hooks.AfterRun(func(mesh *fmesh.FMesh) error {
						sig := aggState.OutputByName("human-Leon::is_alive").Signals().First()
						if sig == nil {
							return nil
						}
						observedIsAlive = append(observedIsAlive, helper.AsBoolOrFalse(sig))
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
			assertions: func(t *testing.T, sim *step_sim.Simulation) {
				var observedCardiacActivity []float64
				var observedHeartRate []int

				aggState := sim.FM.ComponentByName("aggregated_state")
				require.NotNil(t, aggState)

				sim.FM.SetupHooks(func(hooks *fmesh.Hooks) {
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
			assertions: func(t *testing.T, sim *step_sim.Simulation) {
				var observedPleuralPressure []float64
				var observedRespiratoryRate []int

				aggState := sim.FM.ComponentByName("aggregated_state")
				require.NotNil(t, aggState)

				sim.FM.SetupHooks(func(hooks *fmesh.Hooks) {
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
			assertions: func(t *testing.T, sim *step_sim.Simulation) {
				var observedLeftFlow, observedRightFlow []float64

				aggState := sim.FM.ComponentByName("aggregated_state")
				require.NotNil(t, aggState)

				sim.FM.SetupHooks(func(hooks *fmesh.Hooks) {
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
			assertions: func(t *testing.T, sim *step_sim.Simulation) {
				aggState := sim.FM.ComponentByName("aggregated_state")
				require.NotNil(t, aggState)

				var envP, envTemp, envHum float64
				var inspN, inspO, inspA, inspP, inspTemp, inspHum float64

				sim.FM.SetupHooks(func(hooks *fmesh.Hooks) {
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
			assertions: func(t *testing.T, sim *step_sim.Simulation) {
				aggState := sim.FM.ComponentByName("aggregated_state")
				require.NotNil(t, aggState)

				var observedO2, observedCO2, observedGlucose []float64

				sim.FM.SetupHooks(func(hooks *fmesh.Hooks) {
					hooks.AfterRun(func(mesh *fmesh.FMesh) error {
						sig := aggState.OutputByName("human-Leon::venous_blood").Signals().First()
						if sig == nil {
							return nil
						}
						observedO2 = append(observedO2, sig.Scalars().GetOrDefault("O2_level", 0))
						observedCO2 = append(observedCO2, sig.Scalars().GetOrDefault("CO2_level", 0))
						observedGlucose = append(observedGlucose, sig.Scalars().GetOrDefault("glucose_level", 0))
						return nil
					})
				})

				helper.RunSimulationAndThen(sim, 10*time.Second, func() {
					require.NotEmpty(t, observedO2, "should collect blood O2 samples")
					require.NotEmpty(t, observedCO2, "should collect blood CO2 samples")
					require.NotEmpty(t, observedGlucose, "should collect blood glucose samples")

					for _, v := range observedO2 {
						assert.GreaterOrEqual(t, v, 50.0, "O2 should stay above min level")
						assert.LessOrEqual(t, v, 250.0, "O2 should stay below max level")
					}
					for _, v := range observedCO2 {
						assert.GreaterOrEqual(t, v, 20.0, "CO2 should stay above min level")
						assert.LessOrEqual(t, v, 80.0, "CO2 should stay below max level")
					}
					for _, v := range observedGlucose {
						assert.GreaterOrEqual(t, v, 2.0, "glucose should stay above min level")
						assert.LessOrEqual(t, v, 7.0, "glucose should stay below max level")
					}

					meanO2 := helper.Mean(observedO2)
					meanCO2 := helper.Mean(observedCO2)
					meanGlu := helper.Mean(observedGlucose)

					assert.Greater(t, meanO2, 100.0, "mean O2 should be above 100 mL")
					assert.Greater(t, meanCO2, 30.0, "mean CO2 should be above 30 mL")
					assert.Greater(t, meanGlu, 2.5, "mean glucose should be above 2.5")

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

					// 0.5 mL is a meaningful exchange signal; floating-point drift alone cannot reach this
					assert.Greater(t, o2Range, 0.5, "O2 should fluctuate (gas exchange active)")
					assert.Greater(t, co2Range, 0.5, "CO2 should fluctuate (gas exchange active)")

					// Mean O2 should stay near the physiological resting value, not drift to a clamp boundary
					assert.InDelta(t, da.DefaultO2Level, meanO2, 50.0, "mean O2 should be near resting level")
				})
			},
		},
		{
			name: "exhaled gas is different from inspired",
			assertions: func(t *testing.T, sim *step_sim.Simulation) {
				aggState := sim.FM.ComponentByName("aggregated_state")
				require.NotNil(t, aggState)

				var left, right int
				var inspO, inspTemp, inspHum float64

				sim.FM.SetupHooks(func(hooks *fmesh.Hooks) {
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

					lCO2 := lSig.Scalars().GetOrDefault("composition:carbon_dioxide", 0)
					rCO2 := rSig.Scalars().GetOrDefault("composition:carbon_dioxide", 0)

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
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmdChan := make(chan step_sim.Command)
			fm, err := getSimulationMesh()
			require.NoError(t, err)
			sim := step_sim.NewSimulation(context.Background(), fm, cmdChan, sink.NewNoopSink())

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
