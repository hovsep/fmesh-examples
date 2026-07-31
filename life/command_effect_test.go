package main

import (
	"testing"
	"time"

	"github.com/hovsep/fmesh-examples/life/atmosphere"
	"github.com/hovsep/fmesh-examples/simulation/command"
	"github.com/hovsep/fmesh-examples/simulation/session"
	"github.com/hovsep/fmesh-examples/simulation/simtest"
	"github.com/hovsep/fmesh/signal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Every command must actually reach the simulation and move something.
//
// A command that silently does nothing is the failure this whole model is most
// prone to, because nothing reports it: the signal is put on a port, the port is
// drained, and the body carries on exactly as before. It has happened repeatedly
// -- a renamed component left the aggregator wired to a name nobody published, a
// requested hour arrived before the clock and was gone by the time anything
// could use it, the sun reached the air on a cycle where the air was not
// looking. Each was found by accident.
//
// So each command is driven through the real session, and the effect is read
// back off the aggregated telemetry -- the same port the UI draws from -- rather
// than out of the component that handled it. Reading component state would prove
// the handler ran; reading telemetry proves the change left the body.

// probe reads one number the outside world can see.
type probe func(*session.Session) float64

// airScalar reads a scalar off the air the habitat is publishing.
func airScalar(name string) probe {
	return func(sim *session.Session) float64 {
		agg := simMesh(sim).ComponentByName("aggregated_state")
		sig := agg.OutputByName("air::environmental_gas").Signals().First()
		if sig == nil {
			return 0
		}
		return sig.Scalars().ValueOrDefault(name, 0)
	}
}

// bodyValue reads a plain number the body publishes.
//
// AsNumber rather than AsFloat64OrDefault, because the body does not publish one
// numeric type: a heart rate is a whole number of beats and a stomach fill is
// not. Asking for a float64 and defaulting to zero reads a heart rate of zero
// off a perfectly healthy heart -- which is what this probe did at first, and is
// the same "typed accessor infers from the default" trap the accessor's own
// documentation warns about.
func bodyValue(port string) probe {
	return func(sim *session.Session) float64 {
		agg := simMesh(sim).ComponentByName("aggregated_state")
		sig := agg.OutputByName("human-Leon::" + port).Signals().First()
		if sig == nil {
			return 0
		}
		v, _ := signal.AsNumber(sig)
		return v
	}
}

// bodyScalar reads one number off a composite signal the body publishes.
func bodyScalar(port, scalar string) probe {
	return func(sim *session.Session) float64 {
		agg := simMesh(sim).ComponentByName("aggregated_state")
		sig := agg.OutputByName("human-Leon::" + port).Signals().First()
		if sig == nil {
			return 0
		}
		return sig.Scalars().ValueOrDefault(scalar, 0)
	}
}

// sunUVI reads the sky.
func sunUVI(sim *session.Session) float64 {
	agg := simMesh(sim).ComponentByName("aggregated_state")
	sig := agg.OutputByName("sun::uvi").Signals().First()
	if sig == nil {
		return 0
	}
	return signal.AsFloat64OrDefault(sig, 0)
}

func Test_EveryCommandMovesSomething(t *testing.T) {
	if testing.Short() {
		t.Skip("drives every command through a real simulation")
	}

	rose := func(t *testing.T, before, after float64) {
		t.Helper()
		assert.Greater(t, after, before, "the command should have raised this")
	}
	fell := func(t *testing.T, before, after float64) {
		t.Helper()
		assert.Less(t, after, before, "the command should have lowered this")
	}

	tests := []struct {
		name string
		// setup runs before the baseline reading, for commands whose effect is
		// only visible against something already in place.
		setup []command.Line
		// warm is how long to settle before reading the baseline.
		warm time.Duration
		cmd  command.Line
		// settle is how long the effect needs to reach the telemetry.
		settle time.Duration
		probe  probe
		check  func(t *testing.T, before, after float64)
	}{
		// --- the air ---------------------------------------------------------
		{
			name: "air:pressure", cmd: "air:pressure 1500",
			probe: airScalar(atmosphere.ScalarPressure), check: rose,
		},
		{
			name: "air:preset", cmd: "air:preset hyperbaric",
			probe: airScalar(atmosphere.ScalarPressure), check: rose,
		},
		{
			name: "air:oxygen", cmd: "air:oxygen 50",
			probe: airScalar("composition:oxygen"), check: rose,
		},
		{
			name: "altitude", cmd: "altitude 5500",
			probe: airScalar(atmosphere.ScalarPressure), check: fell,
		},
		{
			name: "air:co", cmd: "air:co 800",
			probe: airScalar(atmosphere.ScalarCOppm), check: rose,
		},
		{
			name: "air:mixin", cmd: "air:mixin wood_fire 30m",
			probe: airScalar(atmosphere.ScalarCOppm), check: rose,
		},
		{
			// Only meaningful against air that already has something in it.
			name: "air:clear", setup: []command.Line{"air:mixin wood_fire 30m"},
			warm: time.Second, cmd: "air:clear",
			probe: airScalar(atmosphere.ScalarCOppm), check: fell,
		},
		{
			name: "smoke:cigarette", cmd: "smoke:cigarette",
			probe: airScalar(atmosphere.ScalarCOppm), check: rose,
		},

		// --- the weather and the sky -----------------------------------------
		{
			name: "temp:hot", cmd: "temp:hot",
			probe: airScalar("temperature"), check: rose,
		},
		{
			name: "temp:cold", cmd: "temp:cold",
			probe: airScalar("temperature"), check: fell,
		},
		{
			name: "temp:zero", cmd: "temp:zero",
			probe: airScalar("temperature"), check: fell,
		},
		{
			name: "temp:inc", cmd: "temp:inc",
			probe: airScalar("temperature"), check: rose,
		},
		{
			name: "temp:dec", cmd: "temp:dec",
			probe: airScalar("temperature"), check: fell,
		},
		{
			// The simulation starts at midnight, so any daylight hour is a rise.
			name: "sun:hour", cmd: "sun:hour 12",
			probe: sunUVI, check: rose,
		},

		// --- the body --------------------------------------------------------
		{
			name: "intake:water", cmd: "intake:water 500ml", settle: 5 * time.Second,
			probe: bodyValue("stomach_fill"), check: rose,
		},
		{
			name: "intake:food", cmd: "intake:food 200kcal", settle: 5 * time.Second,
			probe: bodyValue("stomach_fill"), check: rose,
		},
		{
			// Fatigue rather than heart rate, and that is a finding rather than a
			// convenience: eightfold exertion moves this body's pulse by about one
			// beat, and a maximal fright by less than half of one. Breathing,
			// fatigue and temperature all answer exertion properly, so the command
			// plainly arrives; it is the autonomic-to-cardiac gain that is nearly
			// flat. Worth fixing, and not by loosening this test until it passes.
			name: "activity:start", cmd: "activity:start 8", settle: 20 * time.Second,
			probe: bodyValue("muscle_fatigue"), check: rose,
		},
		{
			// Against a body already exerting, so there is something to stop.
			name: "activity:stop", setup: []command.Line{"activity:start 8"},
			warm: 60 * time.Second, cmd: "activity:stop", settle: 60 * time.Second,
			probe: bodyValue("muscle_fatigue"), check: fell,
		},
		{
			name: "emotion:stimulus", cmd: "emotion:stimulus 0.9 -0.8", settle: 5 * time.Second,
			probe: bodyScalar("feelings", "anxious"), check: rose,
		},
		{
			name: "trauma:bleed", cmd: "trauma:bleed 500ml", settle: 10 * time.Second,
			probe: bodyScalar("venous_blood", "volume_l"), check: fell,
		},
		{
			// The kidney fills the bladder on its own; voiding empties it.
			name: "excretion:urinate", warm: 3 * time.Minute,
			cmd: "excretion:urinate", settle: 10 * time.Second,
			probe: bodyValue("bladder_fill"), check: fell,
		},
		{
			name: "excretion:defecate", setup: []command.Line{"intake:food 600kcal"},
			warm: 4 * time.Minute, cmd: "excretion:defecate", settle: 10 * time.Second,
			probe: bodyValue("bowel_fill"), check: fell,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sim := newCommandableSim(t)
			for _, c := range tt.setup {
				sim.Do(c)
			}

			warm := max(tt.warm, time.Second)
			settle := max(tt.settle, time.Second)

			var before, after float64
			simtest.RunFor(sim, warm, func() {
				before = tt.probe(sim)
			})

			sim.Do(tt.cmd)
			simtest.RunFor(sim, settle, func() {
				after = tt.probe(sim)
			})

			tt.check(t, before, after)
		})
	}
}

// Test_EveryRegisteredCommandIsCovered keeps the table above honest.
//
// A command added to the simulation and not to that list would otherwise be
// exactly the thing this file exists to catch, and nothing would say so.
func Test_EveryRegisteredCommandIsCovered(t *testing.T) {
	sim := newCommandableSim(t)

	// The two that only report. They change nothing by design, so there is
	// nothing for the table above to observe.
	readOnly := map[string]bool{"habitat:show": true, "time:now": true}

	// Everything the session brings with it -- pause, step, rate, scheduling --
	// belongs to the loop rather than to this simulation.
	ours := map[string]bool{}
	for _, name := range sim.Commands.Names() {
		if readOnly[name] {
			continue
		}
		switch command.Namespace(name) {
		case "air", "temp", "sun", "altitude", "smoke",
			"intake", "activity", "emotion", "excretion", "trauma":
			ours[name] = true
		}
	}

	covered := map[string]bool{
		"air:pressure": true, "air:preset": true, "air:oxygen": true,
		"altitude": true, "air:co": true, "air:mixin": true, "air:clear": true,
		"smoke:cigarette": true, "temp:hot": true, "temp:cold": true,
		"temp:zero": true, "temp:inc": true, "temp:dec": true, "sun:hour": true,
		"intake:water": true, "intake:food": true, "activity:start": true,
		"activity:stop": true, "emotion:stimulus": true, "trauma:bleed": true,
		"excretion:urinate": true, "excretion:defecate": true,
	}

	for name := range ours {
		assert.True(t, covered[name],
			"%s is registered but no test proves it changes anything", name)
	}
	for name := range covered {
		assert.True(t, ours[name],
			"%s is tested but no longer registered", name)
	}
	require.NotEmpty(t, ours)
}
