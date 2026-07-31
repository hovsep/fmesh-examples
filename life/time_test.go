package main

import (
	"context"
	"testing"
	"time"

	"github.com/hovsep/fmesh-examples/simulation/session"
	"github.com/hovsep/fmesh-examples/simulation/simtest"
	"github.com/hovsep/fmesh-examples/simulation/simtime"
	"github.com/hovsep/fmesh/component"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_Time(t *testing.T) {
	tests := []struct {
		name       string
		assertions func(t *testing.T, sim *session.Session)
	}{
		{
			name: "time advances in timer",
			assertions: func(t *testing.T, sim *session.Session) {
				var observedSimWallTime []time.Time

				timeComponent := simMesh(sim).ComponentByName("time")
				require.NotNil(t, timeComponent)

				timeComponent.SetupHooks(func(hooks *component.Hooks) {
					hooks.AfterActivation(func(_ context.Context, activationContext *component.ActivationContext) error {
						tickSig := timeComponent.OutputByName("tick").Signals().First()
						require.NotNil(t, tickSig)

						_, _, simWallTime, _, err := simtime.UnpackTick(tickSig)
						require.NoError(t, err)

						// Observe and collect sim wall time after every iteration
						observedSimWallTime = append(observedSimWallTime, simWallTime)
						return nil
					})
				})

				simtest.RunFor(sim, time.Millisecond*100, func() {
					assert.NotEmpty(t, observedSimWallTime)
					assert.IsIncreasing(t, observedSimWallTime)
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
