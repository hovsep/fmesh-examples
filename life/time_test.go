package main

import (
	"context"
	"testing"
	"time"

	"github.com/hovsep/fmesh-examples/life/helper"
	"github.com/hovsep/fmesh-examples/simulation/step_sim"
	"github.com/hovsep/fmesh-examples/simulation/step_sim/sink"
	"github.com/hovsep/fmesh/component"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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

				helper.WithRunningSimulation(sim, helper.DefaultSimulationDuration, func() {
					assert.IsIncreasing(t, observedSimWallTime)
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
