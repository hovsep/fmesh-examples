package main

import (
	"context"
	"testing"
	"time"

	"github.com/hovsep/fmesh"
	"github.com/hovsep/fmesh-examples/simulation/step_sim"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_SimChecks(t *testing.T) {
	tests := []struct {
		name       string
		assertions func(t *testing.T, sim *step_sim.Simulation)
	}{
		{
			name: "time advances in timer",
			assertions: func(t *testing.T, sim *step_sim.Simulation) {
				timeComponent := sim.FM.ComponentByName("time")
				t1 := timeComponent.State().Get("tick_count")

				assert.Zero(t, t1)

				runAndAssert(t, sim, func(t *testing.T, sim *step_sim.Simulation) {
					time.Sleep(100 * time.Millisecond)

					t2 := timeComponent.State().Get("tick_count")
					assert.Greater(t, t2, t1)
				})

			},
		},
		{
			name: "time advances in aggregated state",
			assertions: func(t *testing.T, sim *step_sim.Simulation) {
				runAndAssert(t, sim, func(t *testing.T, sim *step_sim.Simulation) {
					observedSimTime := []time.Duration{}

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

					// Let the sim to run for some time
					time.Sleep(100 * time.Millisecond)

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

func runAndAssert(t *testing.T, sim *step_sim.Simulation, assertions func(t *testing.T, sim *step_sim.Simulation)) {
	go sim.Run()
	assertions(t, sim)
	sim.SendCommand(step_sim.Exit)
}
