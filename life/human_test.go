package main

import (
	"context"
	"testing"

	"github.com/hovsep/fmesh"
	"github.com/hovsep/fmesh-examples/life/helper"
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
						require.NotNil(t, sig)
						observedIsAlive = append(observedIsAlive, helper.AsBoolOrFalse(sig))
						return nil
					})
				})

				helper.RunSimulationAndThen(sim, helper.DefaultSimulationDuration, func() {
					assert.NotEmpty(t, observedIsAlive)
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
						require.NotNil(t, sigAct)
						observedCardiacActivity = append(observedCardiacActivity, helper.AsF64(sigAct))

						sigRate := aggState.OutputByName("human-Leon::heart_rate").Signals().First()
						require.NotNil(t, sigRate)
						observedHeartRate = append(observedHeartRate, helper.AsInt(sigRate))
						return nil
					})
				})

				helper.RunSimulationAndThen(sim, helper.DefaultSimulationDuration, func() {
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
						require.NotNil(t, sigPressure)
						observedPleuralPressure = append(observedPleuralPressure, helper.AsF64(sigPressure))

						sigRate := aggState.OutputByName("human-Leon::respiratory_rate").Signals().First()
						require.NotNil(t, sigRate)
						observedRespiratoryRate = append(observedRespiratoryRate, helper.AsInt(sigRate))
						return nil
					})
				})

				helper.RunSimulationAndThen(sim, helper.DefaultSimulationDuration, func() {
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
						require.NotNil(t, sigLeft)
						observedLeftFlow = append(observedLeftFlow, helper.AsF64(sigLeft))

						sigRight := aggState.OutputByName("human-Leon::lung_right_flow").Signals().First()
						require.NotNil(t, sigRight)
						observedRightFlow = append(observedRightFlow, helper.AsF64(sigRight))
						return nil
					})
				})

				helper.RunSimulationAndThen(sim, helper.DefaultSimulationDuration, func() {
					assert.NotEmpty(t, observedLeftFlow)
					assert.NotEmpty(t, observedRightFlow)
					assertBidirectionalFlow(t, observedLeftFlow, "left")
					assertBidirectionalFlow(t, observedRightFlow, "right")
				})
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmdChan := make(chan step_sim.Command)
			fm := getSimulationMesh()
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
