package main

import (
	"context"
	"testing"

	"github.com/hovsep/fmesh"
	"github.com/hovsep/fmesh/signal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEngineRunsUntilTheTankIsDry(t *testing.T) {
	fm, err := getMesh()
	require.NoError(t, err)

	// Record which cylinder fired, stroke by stroke, and the top speed.
	var fired []int
	topRPM := 0.0
	fm.SetupHooks(func(h *fmesh.Hooks) {
		h.AfterCycle(func(_ context.Context, cc *fmesh.CycleContext) error {
			for i := 1; i <= cylinders; i++ {
				if cc.Cycle.ActivationResults().ByName(cylinderName(i)) != nil && state[bool](fm, cylinderName(i), "fired") {
					fired = append(fired, i)
				}
			}
			topRPM = max(topRPM, state[Crank](fm, "crankshaft", "crank").RPM)
			return nil
		})
	})

	require.NoError(t, fm.ComponentByName("crankshaft").InputByName(portKey).PutSignals(signal.New("on")))
	_, err = fm.Run(context.Background())
	require.NoError(t, err, "the mesh stops by itself once the crankshaft stops")

	crank := state[Crank](fm, "crankshaft", "crank")
	assert.Zero(t, crank.RPM)
	assert.Zero(t, state[float64](fm, "fuel-tank", "level"))
	assert.Greater(t, crank.Stroke, 200)
	assert.InDelta(t, 2700, topRPM, 50, "full throttle settles where torque meets friction")

	// Every power stroke fires, in the firing order 1-3-4-2.
	require.Greater(t, len(fired), 100)
	start := 0
	for fired[start] != 1 {
		start++
	}
	for n, cyl := range fired[start : start+100] {
		assert.Equal(t, firingOrder[n%cylinders], cyl, "power stroke %d", n)
	}
}

func TestDeadBatteryNeverStarts(t *testing.T) {
	fm, err := getMesh()
	require.NoError(t, err)
	fm.ComponentByName("battery").State().Set("level", 0.0)

	require.NoError(t, fm.ComponentByName("crankshaft").InputByName(portKey).PutSignals(signal.New("on")))
	_, err = fm.Run(context.Background())
	require.NoError(t, err)

	assert.Zero(t, state[Crank](fm, "crankshaft", "crank").RPM)
	assert.False(t, state[bool](fm, "ecu", "started"))
}
