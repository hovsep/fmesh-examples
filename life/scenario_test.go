package main

import (
	"testing"
	"time"

	"github.com/hovsep/fmesh"
	"github.com/hovsep/fmesh-examples/life/env/factor"
	"github.com/hovsep/fmesh-examples/life/organism/human"
	"github.com/hovsep/fmesh-examples/life/organism/human/controller"
	"github.com/hovsep/fmesh-examples/simulation/command"
	"github.com/hovsep/fmesh-examples/simulation/session"
	"github.com/hovsep/fmesh-examples/simulation/simtest"
	"github.com/hovsep/fmesh-examples/simulation/stepsim"
	"github.com/hovsep/fmesh/component"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testTick is the step every test in this package runs at.
//
// The app steps at ten milliseconds because of the ECG, which is a display
// concern (see factor.DefaultTickDuration). No test draws an ECG, and every test
// pays for the step five times over: the tick is what a simulated second costs,
// so a suite that watches hours of chemistry is a suite that spends most of its
// life integrating a waveform nobody is looking at.
//
// Fifty milliseconds is still twenty samples a second -- far finer than
// breathing, circulation, or anything chemical the reference tests assert on --
// and it takes the suite from thirteen minutes to under three.
const testTick = 50 * time.Millisecond

// newCommandableSim builds the simulation with every command registered, ready
// to be given commands and run for a stretch of simulated time.
//
// Commands queued with Do before the run are applied on its first pass, so a
// test can set up a scenario and then let time pass.
func newCommandableSim(t *testing.T) *session.Session {
	t.Helper()
	return newSimIn(t, factor.GetGasComponent)
}

// newChamberSim builds the same simulation inside a barochamber instead of the
// open atmosphere. Nothing about the body changes; only the world does.
func newChamberSim(t *testing.T) *session.Session {
	t.Helper()
	return newSimIn(t, factor.GetBarochamberComponent)
}

// newSimIn builds a test simulation in the given world, at the test step.
func newSimIn(t *testing.T, environment func() (*component.Component, error)) *session.Session {
	t.Helper()

	fm, err := getSimulationMeshIn(environment, testTick)
	require.NoError(t, err)

	sim, err := newSession(fm)
	require.NoError(t, err)
	return sim
}

// simMesh returns the mesh the session is driving.
func simMesh(sim *session.Session) *fmesh.FMesh {
	return sim.Engine.(*stepsim.Engine).Mesh()
}

// bodyComponent returns a component from inside the human's mesh.
func bodyComponent(t *testing.T, sim *session.Session, name string) *component.Component {
	t.Helper()

	body := human.Find(simMesh(sim))
	require.NotNil(t, body, "no human in the simulation")

	inner := human.InnerMesh(body)
	require.NotNil(t, inner, "human component does not expose its inner mesh")

	c := inner.ComponentByName(name)
	require.NotNil(t, c, "no component %q in the human mesh", name)
	return c
}

func Test_BodyCommands(t *testing.T) {
	tests := []struct {
		name       string
		commands   []command.Line
		component  string
		assertions func(t *testing.T, state component.State)
	}{
		{
			name:      "drinking water reaches the intake controller",
			commands:  []command.Line{"intake:water 500ml"},
			component: "controller:intake",
			assertions: func(t *testing.T, state component.State) {
				// The full amount is commanded immediately; delivery is metered
				// out over time by the process runner.
				assert.Equal(t, 500.0, state.Get(controller.TotalWaterMl))
			},
		},
		{
			name:      "volume units are interchangeable",
			commands:  []command.Line{"intake:water 0.5l"},
			component: "controller:intake",
			assertions: func(t *testing.T, state component.State) {
				assert.Equal(t, 500.0, state.Get(controller.TotalWaterMl))
			},
		},
		{
			name:      "eating reaches the intake controller",
			commands:  []command.Line{"intake:food 200kcal"},
			component: "controller:intake",
			assertions: func(t *testing.T, state component.State) {
				assert.Equal(t, 200.0, state.Get(controller.TotalFoodKcal))
			},
		},
		{
			name: "several commands accumulate",
			commands: []command.Line{
				"intake:water 250ml",
				"intake:water 250ml",
				"intake:food 100kcal",
			},
			component: "controller:intake",
			assertions: func(t *testing.T, state component.State) {
				assert.Equal(t, 500.0, state.Get(controller.TotalWaterMl))
				assert.Equal(t, 100.0, state.Get(controller.TotalFoodKcal))
			},
		},
		{
			name:      "an activity with a duration ends on its own",
			commands:  []command.Line{"activity:start 8 200ms"},
			component: "controller:physical_stress",
			assertions: func(t *testing.T, state component.State) {
				// The run is longer than the activity, so the body is back at rest.
				assert.Equal(t, controller.RestingIntensity, state.Get(controller.ActivityIntensity))
				assert.Equal(t, 0.0, state.Get(controller.ActivityRemaining))
			},
		},
		{
			name:      "an activity without a duration keeps going",
			commands:  []command.Line{"activity:start 8"},
			component: "controller:physical_stress",
			assertions: func(t *testing.T, state component.State) {
				assert.Equal(t, 8.0, state.Get(controller.ActivityIntensity))
				assert.Equal(t, controller.Indefinite, state.Get(controller.ActivityRemaining))
			},
		},
		{
			name:      "stopping returns the body to rest",
			commands:  []command.Line{"activity:start 8", "activity:stop"},
			component: "controller:physical_stress",
			assertions: func(t *testing.T, state component.State) {
				assert.Equal(t, controller.RestingIntensity, state.Get(controller.ActivityIntensity))
			},
		},
		{
			name:      "voiding reaches the excretion controller",
			commands:  []command.Line{"excretion:urinate", "excretion:defecate"},
			component: "controller:excretion",
			assertions: func(t *testing.T, state component.State) {
				assert.Equal(t, 1.0, state.Get(controller.TotalUrinations))
				assert.Equal(t, 1.0, state.Get(controller.TotalDefecations))
			},
		},
		{
			name:      "an emotional stimulus raises arousal and then fades",
			commands:  []command.Line{"emotion:stimulus 0.8 -0.5"},
			component: "controller:mental_stress",
			assertions: func(t *testing.T, state component.State) {
				arousal := state.Get(controller.Arousal).(float64)
				valence := state.Get(controller.Valence).(float64)

				// Decay has begun, so the peak has passed...
				assert.Less(t, arousal, 0.8, "arousal should be decaying")
				// ...but a fright is still felt a moment later.
				assert.Greater(t, arousal, 0.0, "arousal should not vanish instantly")
				assert.Less(t, valence, 0.0, "an unpleasant stimulus should still feel unpleasant")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sim := newCommandableSim(t)

			// Queue the scenario before the loop starts; the sim drains all
			// pending commands at the top of its first iteration.
			for _, cmd := range tt.commands {
				sim.Do(cmd)
			}

			target := bodyComponent(t, sim, tt.component)
			simtest.RunFor(sim, 500*time.Millisecond, func() {
				tt.assertions(t, target.State())
			})
		})
	}
}

// Scheduling is expressed in simulated time. These use short intervals so the
// test runs quickly; the interval arithmetic itself is covered exhaustively by
// the scheduler's own unit tests against a fake clock.

func Test_ScheduledCommandsReachTheBody(t *testing.T) {
	sim := newCommandableSim(t)

	// Six intervals fit in the run, and the first fires one interval in rather
	// than immediately, so five to six swallows are expected.
	sim.Do("every 50ms intake:water 250ml")

	intake := bodyComponent(t, sim, "controller:intake")
	simtest.RunFor(sim, 300*time.Millisecond, func() {
		total := intake.State().Get(controller.TotalWaterMl).(float64)
		assert.GreaterOrEqual(t, total, 4*250.0, "repeating job fired too few times")
		assert.LessOrEqual(t, total, 6*250.0, "repeating job fired too many times")
	})
}

func Test_ScheduledCommandRespectsItsDelay(t *testing.T) {
	sim := newCommandableSim(t)

	// Due well after the run ends, so it must never fire.
	sim.Do("after 10s intake:water 500ml")

	intake := bodyComponent(t, sim, "controller:intake")
	simtest.RunFor(sim, 200*time.Millisecond, func() {
		assert.Equal(t, 0.0, intake.State().Get(controller.TotalWaterMl),
			"a job scheduled beyond the run fired early")
		assert.Len(t, sim.Timeline.Jobs(), 1, "the pending job should still be queued")
	})
}

func Test_CancelledJobNeverFires(t *testing.T) {
	sim := newCommandableSim(t)

	sim.Do("every 50ms intake:water 250ml")
	sim.Do("cancel all")

	intake := bodyComponent(t, sim, "controller:intake")
	simtest.RunFor(sim, 300*time.Millisecond, func() {
		assert.Equal(t, 0.0, intake.State().Get(controller.TotalWaterMl))
		assert.Empty(t, sim.Timeline.Jobs())
	})
}

func Test_ScenarioRunsItsStepsInOrder(t *testing.T) {
	sim := newCommandableSim(t)

	// The shape the northstar asked for: eat, let time pass, then exert.
	sim.Do("intake:food 200kcal; wait 100ms; activity:start 8")

	intake := bodyComponent(t, sim, "controller:intake")
	physical := bodyComponent(t, sim, "controller:physical_stress")

	simtest.RunFor(sim, 300*time.Millisecond, func() {
		assert.Equal(t, 200.0, intake.State().Get(controller.TotalFoodKcal),
			"the pre-wait step did not run")
		assert.Equal(t, 8.0, physical.State().Get(controller.ActivityIntensity),
			"the post-wait step did not run")
		assert.Empty(t, sim.Timeline.Scenarios(), "the scenario should have finished")
	})
}

func Test_ScenarioWaitsBeforeItsLaterSteps(t *testing.T) {
	sim := newCommandableSim(t)

	// The wait outlasts the run, so only the first step should ever happen.
	sim.Do("intake:food 200kcal; wait 10s; activity:start 8")

	intake := bodyComponent(t, sim, "controller:intake")
	physical := bodyComponent(t, sim, "controller:physical_stress")

	simtest.RunFor(sim, 200*time.Millisecond, func() {
		assert.Equal(t, 200.0, intake.State().Get(controller.TotalFoodKcal),
			"the pre-wait step did not run")
		assert.Equal(t, controller.RestingIntensity, physical.State().Get(controller.ActivityIntensity),
			"the post-wait step ran before its wait elapsed")
		assert.Len(t, sim.Timeline.Scenarios(), 1, "the scenario should still be waiting")
	})
}

func Test_NamedScenarioIsReusable(t *testing.T) {
	sim := newCommandableSim(t)

	sim.Do("script hydrate intake:water 250ml; wait 50ms; intake:water 250ml")
	sim.Do("run hydrate")

	intake := bodyComponent(t, sim, "controller:intake")
	simtest.RunFor(sim, 300*time.Millisecond, func() {
		assert.Equal(t, 500.0, intake.State().Get(controller.TotalWaterMl),
			"both steps of the named scenario should have run")

		// Defining a scenario must not consume it.
		_, defined := sim.Timeline.Steps("hydrate")
		assert.True(t, defined, "the definition should survive being run")
	})
}

func Test_DefiningAScenarioDoesNotRunIt(t *testing.T) {
	sim := newCommandableSim(t)

	// The definition contains step separators; treating it as a scenario to run
	// would drink the water at definition time.
	sim.Do("script hydrate intake:water 250ml; wait 50ms; intake:water 250ml")

	intake := bodyComponent(t, sim, "controller:intake")
	simtest.RunFor(sim, 200*time.Millisecond, func() {
		assert.Equal(t, 0.0, intake.State().Get(controller.TotalWaterMl))
		assert.Empty(t, sim.Timeline.Scenarios())
	})
}

func Test_UnknownBodyCommandDoesNotStopTheSimulation(t *testing.T) {
	sim := newCommandableSim(t)

	// A command addressed to a namespace no controller owns must be reported and
	// dropped: an activation error would halt the entire mesh, which would mean a
	// typo could kill the patient.
	sim.Do("intake:plutonium 1kg")

	intake := bodyComponent(t, sim, "controller:intake")
	simtest.RunFor(sim, 200*time.Millisecond, func() {
		assert.Equal(t, 0.0, intake.State().Get(controller.TotalWaterMl))
		assert.Equal(t, 0.0, intake.State().Get(controller.TotalFoodKcal))
	})
}
