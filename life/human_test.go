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

					rPeaksFound := 0
					inPeak := false
					// Check for R-peaks
					for _, v := range observedCardiacActivity {
						if v > 0.0 && !inPeak {
							rPeaksFound++
							inPeak = true
							continue
						}

						if v == 0.0 {
							inPeak = false
						}

					}
					assert.Greater(t, rPeaksFound, 0)
					assert.Less(t, rPeaksFound, 10)
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
				type lungObservation struct {
					Volume           []float64
					Flow             []float64
					AlveolarPressure []float64
				}

				var leftLungObservation, rightLungObservation lungObservation

				aggState := sim.FM.ComponentByName("aggregated_state")
				require.NotNil(t, aggState)

				sim.FM.SetupHooks(func(hooks *fmesh.Hooks) {
					hooks.AfterRun(func(mesh *fmesh.FMesh) error {

						leftLungObservation.Volume = append(leftLungObservation.Volume, helper.AsF64(aggState.OutputByName("human-Leon::lung_left_volume").Signals().First()))
						leftLungObservation.Flow = append(leftLungObservation.Flow, helper.AsF64(aggState.OutputByName("human-Leon::lung_left_flow").Signals().First()))
						leftLungObservation.AlveolarPressure = append(leftLungObservation.AlveolarPressure, helper.AsF64(aggState.OutputByName("human-Leon::lung_left_alveolar_pressure").Signals().First()))

						rightLungObservation.Volume = append(rightLungObservation.Volume, helper.AsF64(aggState.OutputByName("human-Leon::lung_right_volume").Signals().First()))
						rightLungObservation.Flow = append(rightLungObservation.Flow, helper.AsF64(aggState.OutputByName("human-Leon::lung_right_flow").Signals().First()))
						rightLungObservation.AlveolarPressure = append(rightLungObservation.AlveolarPressure, helper.AsF64(aggState.OutputByName("human-Leon::lung_right_alveolar_pressure").Signals().First()))
						return nil
					})
				})

				helper.RunSimulationAndThen(sim, helper.DefaultSimulationDuration, func() {
					assert.NotEmpty(t, leftLungObservation.Flow)
					assert.NotEmpty(t, rightLungObservation.Flow)
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
