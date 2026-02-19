package main

import (
	"context"
	"testing"
	"time"

	"github.com/hovsep/fmesh"
	"github.com/hovsep/fmesh-examples/life/helper"
	"github.com/hovsep/fmesh-examples/simulation/step_sim"
	"github.com/hovsep/fmesh/component"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const defaultSimulationDuration = 100 * time.Millisecond

func Test_Time(t *testing.T) {
	tests := []struct {
		name       string
		assertions func(t *testing.T, sim *step_sim.Simulation)
	}{
		{
			name: "time advances in timer",
			assertions: func(t *testing.T, sim *step_sim.Simulation) {
				var observedSimWallTime []time.Time

				timeComponent := sim.FM.ComponentByName("time")
				require.NotNil(t, timeComponent)

				timeComponent.SetupHooks(func(hooks *component.Hooks) {
					hooks.AfterActivation(func(activationContext *component.ActivationContext) error {
						tickSig := timeComponent.OutputByName("tick").Signals().First()
						require.NotNil(t, tickSig)

						_, _, simWallTime, _, err := helper.UnpackTick(tickSig)
						require.NoError(t, err)

						// Observe and collect sim wall time after every iteration
						observedSimWallTime = append(observedSimWallTime, simWallTime)
						return nil
					})
				})

				helper.WithRunningSimulation(sim, defaultSimulationDuration, func() {
					assert.IsIncreasing(t, observedSimWallTime)
				})

			},
		},
		{
			name: "time advances in aggregated state",
			assertions: func(t *testing.T, sim *step_sim.Simulation) {
				var observedSimTime []time.Duration

				sim.FM.SetupHooks(func(hooks *fmesh.Hooks) {
					hooks.AfterRun(func(mesh *fmesh.FMesh) error {
						aggState := mesh.ComponentByName("aggregated_state")
						require.NotNil(t, aggState)

						simTimeSig := aggState.OutputByName("time::sim_time").Signals().First()
						require.NotNil(t, simTimeSig)
						assert.NotZero(t, simTimeSig.PayloadOrNil())

						// Observe and collect sim time after every iteration
						observedSimTime = append(observedSimTime, simTimeSig.PayloadOrNil().(time.Duration))
						return nil
					})
				})

				helper.WithRunningSimulation(sim, defaultSimulationDuration, func() {
					// Check if time advances actually
					assert.IsIncreasing(t, observedSimTime)
				})

			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmdChan := make(chan step_sim.Command)
			fm := getSimulationMesh()
			sim := step_sim.NewSimulation(context.Background(), cmdChan, fm)

			if tt.assertions != nil {
				tt.assertions(t, sim)
			}
		})
	}
}

func Test_Human(t *testing.T) {
	tests := []struct {
		name       string
		assertions func(t *testing.T, sim *step_sim.Simulation)
	}{
		{
			name: "human is alive",
			assertions: func(t *testing.T, sim *step_sim.Simulation) {
				humanComponent := helper.FindHumanComponent(sim.FM)
				require.NotNil(t, humanComponent)

				helper.WithRunningSimulation(sim, 100*time.Millisecond, func() {

				})
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmdChan := make(chan step_sim.Command)
			fm := getSimulationMesh()
			sim := step_sim.NewSimulation(context.Background(), cmdChan, fm)

			if tt.assertions != nil {
				tt.assertions(t, sim)
			}
		})
	}
}
