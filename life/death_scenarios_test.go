package main

import (
	"testing"
	"time"

	"github.com/hovsep/fmesh"
	"github.com/hovsep/fmesh-examples/simulation/command"
	"github.com/hovsep/fmesh-examples/simulation/session"
	"github.com/hovsep/fmesh-examples/simulation/simtest"
	"github.com/stretchr/testify/assert"
)

// How long each way of dying takes.
//
// Individually these are covered elsewhere -- the haemorrhage classes, the
// chemoreflex, the carbon monoxide chamber. What this file adds is the
// comparison: whether the body dies of the right things, in the right order, on
// timescales that are recognisably a body's.
//
// The pace is deliberately faster than life, because a scenario nobody can watch
// teaches nobody anything. What is not negotiable is the ordering and the rough
// magnitude -- suffocation in minutes, exsanguination in minutes, exposure in
// tens of minutes, poisoning in the better part of an hour, thirst in hours --
// and that a healthy body left alone does not die at all.
//
// Every figure here was wrong at least once. Haemorrhage could not kill anybody
// because injury was judged on oxygen *tension*, which a bleeding patient keeps
// perfectly normal right up until they die of it. Carbon monoxide could not
// kill for the same reason. A body died of dehydration after a hard workout
// because the injury threshold sat at 3% of body water. Exposure killed in eight
// minutes because a freezing body lost seven degrees of core in two.

// timeToDeath runs a scenario and reports when the body was first declared dead,
// or -1 if it survived the whole run.
func timeToDeath(t *testing.T, setup func(*session.Session), limit time.Duration) time.Duration {
	t.Helper()

	sim := newCommandableSim(t)
	setup(sim)
	alive := bodyValue("is_alive")

	died, elapsed := time.Duration(-1), 0.0
	simMesh(sim).SetupHooks(func(hooks *fmesh.Hooks) {
		hooks.AfterRun(func(*fmesh.FMesh) error {
			elapsed += tickSeconds
			if died < 0 && alive(sim) == 0 {
				died = time.Duration(elapsed * float64(time.Second))
			}
			return nil
		})
	})
	simtest.RunFor(sim, limit, func() {})
	return died
}

func Test_WaysToDie(t *testing.T) {
	if testing.Short() {
		t.Skip("runs several scenarios to their end")
	}

	do := func(cmds ...command.Line) func(*session.Session) {
		return func(sim *session.Session) {
			for _, c := range cmds {
				sim.Do(c)
			}
		}
	}

	tests := []struct {
		name       string
		setup      func(*session.Session)
		limit      time.Duration
		wantAtMost time.Duration // zero means it must survive the whole run
		wantAtLeas time.Duration
	}{
		{
			// Air with almost no oxygen in it. Consciousness goes in under a
			// minute in life and death follows within several.
			name: "suffocation", setup: do("air:oxygen 1"), limit: 20 * time.Minute,
			wantAtLeas: time.Minute, wantAtMost: 10 * time.Minute,
		},
		{
			// Sixty percent of the blood volume, untreated.
			//
			// Fifty percent this body survives, with a destroyed kidney and
			// nothing else -- more resilient than a real patient, who would be
			// in extremis. It compensates by holding cardiac output near two
			// litres a minute, which keeps oxygen delivery above what the
			// tissues are consuming. Past that the tank is too empty for any
			// heart rate to help.
			name: "exsanguination", setup: do("trauma:bleed 3000ml"), limit: 30 * time.Minute,
			wantAtLeas: 2 * time.Minute, wantAtMost: 20 * time.Minute,
		},
		{
			// Naked at thirty-five below. Far quicker than life on purpose, but
			// no longer quicker than a person could react to.
			name: "exposure", setup: do("temp:cold"), limit: 60 * time.Minute,
			wantAtLeas: 5 * time.Minute, wantAtMost: 40 * time.Minute,
		},
		{
			// An engine running in a closed garage.
			name: "carbon monoxide", setup: do("air:mixin car_exhaust 4h"), limit: 2 * time.Hour,
			wantAtLeas: 20 * time.Minute, wantAtMost: 90 * time.Minute,
		},
		{
			// Hard work and nothing to drink. Hours, and it should be thirst
			// that does it rather than the work.
			name: "hard work, no water", setup: do("activity:start 9"), limit: 6 * time.Hour,
			wantAtLeas: 2 * time.Hour, wantAtMost: 5 * time.Hour,
		},
		{
			name: "left alone", setup: do(), limit: time.Hour,
		},
		{
			// A warm day is not a way to die, and neither is a hard session with
			// a sane amount of it.
			name: "hot day at rest", setup: do("temp:hot", "sun:hour 13"), limit: time.Hour,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			died := timeToDeath(t, tt.setup, tt.limit)

			if tt.wantAtMost == 0 {
				assert.EqualValues(t, -1, died, "this body should have survived, and died at %v", died)
				return
			}

			if assert.NotEqualValues(t, -1, died, "this body should have died within %v", tt.limit) {
				assert.Greater(t, died, tt.wantAtLeas, "but not this quickly")
				assert.Less(t, died, tt.wantAtMost, "and not this slowly")
			}
		})
	}
}
