package main

import (
	"context"
	"testing"

	"github.com/hovsep/fmesh"
	"github.com/hovsep/fmesh/component"
	"github.com/hovsep/fmesh/port"
	"github.com/hovsep/fmesh/signal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// trip is one run of the engine from the key to standstill, with the driver
// following a script, stroke by stroke.
type trip struct {
	fm     *fmesh.FMesh
	driver *Driver
	rpm    []float64   // by stroke
	fired  map[int]int // stroke → the cylinder that fired on it
	modes  map[string]bool
}

func newTrip(t *testing.T, setup map[string]map[string]any) *trip {
	t.Helper()
	tr := &trip{driver: &Driver{}, modes: map[string]bool{}, fired: map[int]int{}}
	fm, err := getMesh(tr.driver)
	require.NoError(t, err)
	for name, state := range setup {
		for k, v := range state {
			fm.ComponentByName(name).State().Set(k, v)
		}
	}
	tr.fm = fm
	return tr
}

// run turns the key and lets the engine run, calling script at every stroke.
func (tr *trip) run(t *testing.T, script func(stroke int, d *DriverInput)) {
	t.Helper()
	tr.driver.Set(func(d *DriverInput) { d.Ignition = true })
	last := -1
	tr.fm.SetupHooks(func(h *fmesh.Hooks) {
		h.AfterCycle(func(_ context.Context, cc *fmesh.CycleContext) error {
			// Every part waits for one signal per stroke on each input: two
			// on one port would mean a part fell a stroke behind.
			for _, c := range tr.fm.Components().AllOrdered() {
				_ = c.Inputs().ForEach(func(p *port.Port) error {
					assert.LessOrEqual(t, p.Signals().Len(), 1, "%s.%s", c.Name(), p.Name())
					return nil
				})
			}
			for i := 1; i <= cylinders; i++ {
				if r := cc.Cycle.ActivationResults().ByName(cylinderName(i)); r != nil && !component.IsWaitingForInput(r) && state[bool](tr.fm, cylinderName(i), "fired") {
					tr.fired[state[Crank](tr.fm, "crankshaft", "crank").Stroke] = i
				}
			}
			crank := state[Crank](tr.fm, "crankshaft", "crank")
			if crank.Stroke != last {
				last = crank.Stroke
				tr.rpm = append(tr.rpm, crank.RPM)
				tr.modes[state[string](tr.fm, "ecu", "mode")] = true
				tr.driver.Set(func(d *DriverInput) { script(crank.Stroke, d) })
			}
			return nil
		})
	})
	require.NoError(t, tr.fm.ComponentByName("crankshaft").InputByName("key").PutSignals(signal.New("start")))
	_, err := tr.fm.Run(context.Background())
	require.NoError(t, err, "the mesh stops by itself once the crankshaft stops")
}

func maxOf(xs []float64) float64 {
	m := 0.0
	for _, x := range xs {
		m = max(m, x)
	}
	return m
}

func avg(xs []float64) float64 {
	s := 0.0
	for _, x := range xs {
		s += x
	}
	return s / float64(len(xs))
}

func TestStartIdleRevAndStop(t *testing.T) {
	tr := newTrip(t, nil)
	var warmLambda LambdaReading
	tr.run(t, func(stroke int, d *DriverInput) {
		if stroke == 1590 {
			warmLambda = state[LambdaReading](tr.fm, "instrument-cluster", "lambda")
		}
		switch {
		case stroke >= 1600:
			d.Ignition = false
		case stroke >= 1000:
			d.Pedal = 0
		case stroke >= 600:
			d.Pedal = 1
		}
	})

	t.Logf("idle %.0f, top %.0f, back to idle %.0f, strokes %d", avg(tr.rpm[400:600]), maxOf(tr.rpm[600:1000]), avg(tr.rpm[1400:1600]), len(tr.rpm))
	assert.True(t, tr.modes["cranking"])
	assert.InDelta(t, idleRPM+200, avg(tr.rpm[400:600]), 150, "fast idle while cold")
	assert.InDelta(t, revLimit, maxOf(tr.rpm[600:1000]), 200, "flat out it bounces off the rev limiter")
	assert.True(t, tr.modes["rev limit"])
	assert.InDelta(t, idleRPM, avg(tr.rpm[1400:1600]), 100, "warm idle")
	assert.Zero(t, state[Crank](tr.fm, "crankshaft", "crank").RPM, "key off: it spins down and stops")

	// At idle every stroke is a power stroke, in the firing order 1-3-4-2.
	for n := 400; n < 600; n++ {
		require.Equal(t, firingOrder[n%cylinders], tr.fired[n], "stroke %d", n)
	}

	assert.Greater(t, state[float64](tr.fm, "engine-block", "temp"), ambient+5, "it warmed up")
	assert.True(t, warmLambda.Ready, "the oxygen sensor warmed up")
	assert.InDelta(t, 1, warmLambda.Lambda, 0.03, "warm idle burns at lambda 1")
	assert.Equal(t, "the key was turned off", stopReason(tr.fm))
}

func TestFlatBatteryOnlyClicks(t *testing.T) {
	tr := newTrip(t, map[string]map[string]any{"battery": {"soc": 0.05}})
	tr.run(t, func(int, *DriverInput) {})

	// The starter's relay closes, the battery sags below what it needs, and
	// with nothing turning the crankshaft the mesh stops on the first stroke.
	assert.Len(t, tr.rpm, 1)
	assert.False(t, state[bool](tr.fm, "starter", "engaged"))
	assert.Less(t, state[float64](tr.fm, "starter", "volts"), minVolts)
	assert.Equal(t, "the battery is too weak to turn the starter", stopReason(tr.fm))
}

func TestHalfChargedBatteryStarts(t *testing.T) {
	tr := newTrip(t, map[string]map[string]any{"battery": {"soc": 0.5}})
	tr.run(t, func(stroke int, d *DriverInput) { d.Ignition = stroke < 200 })
	assert.True(t, tr.modes["running"])
}

func TestTankRunsDry(t *testing.T) {
	tr := newTrip(t, map[string]map[string]any{"fuel-tank": {"liters": 0.05}})
	tr.run(t, func(stroke int, d *DriverInput) { d.Pedal = 0.6 })

	assert.Zero(t, state[float64](tr.fm, "fuel-tank", "liters"))
	assert.Equal(t, "stalled", state[string](tr.fm, "ecu", "mode"))
	assert.Equal(t, "the tank is empty", stopReason(tr.fm))
}

func TestHotEngineRunsTheFanAndTheThermostat(t *testing.T) {
	tr := newTrip(t, map[string]map[string]any{"engine-block": {"temp": 95.0}})
	fanRan := false
	peak := 0.0
	tr.run(t, func(stroke int, d *DriverInput) {
		fanRan = fanRan || state[float64](tr.fm, "cooling-fan", "speed") > 0.5
		peak = max(peak, state[float64](tr.fm, "engine-block", "temp"))
		d.Pedal = 0.5
		if stroke > 4000 {
			d.Ignition = false
		}
	})
	t.Logf("peak %.1f °C, end %.1f °C", peak, state[float64](tr.fm, "engine-block", "temp"))
	assert.True(t, fanRan)
	assert.Positive(t, state[float64](tr.fm, "thermostat", "opening"))
	assert.Less(t, peak, overheatAt, "the fan keeps it from overheating")
}

func TestACLoadsTheEngine(t *testing.T) {
	tr := newTrip(t, map[string]map[string]any{"engine-block": {"temp": 85.0}})
	var engaged bool
	tr.run(t, func(stroke int, d *DriverInput) {
		d.AC = stroke >= 400
		d.Lights = stroke >= 400
		engaged = engaged || state[bool](tr.fm, "ac-compressor", "engaged")
		if stroke >= 1200 {
			d.Ignition = false
		}
	})
	t.Logf("idle without AC %.0f, with AC %.0f", avg(tr.rpm[200:400]), avg(tr.rpm[1000:1200]))
	assert.True(t, engaged)
	assert.InDelta(t, idleRPM+100, avg(tr.rpm[1000:1200]), 120, "the ECU raises the idle for the compressor")
	assert.True(t, state[bool](tr.fm, "headlights", "on"))
}
