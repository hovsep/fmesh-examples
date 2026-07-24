package main

import (
	"context"
	"testing"
	"time"

	"github.com/hovsep/fmesh-examples/life/helper"
	"github.com/hovsep/fmesh-examples/life/organism/human"
	"github.com/hovsep/fmesh-examples/life/organism/human/controller"
	"github.com/hovsep/fmesh-examples/simulation/step_sim"
	"github.com/hovsep/fmesh-examples/simulation/step_sim/sink"
	"github.com/hovsep/fmesh/component"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newCommandableSim builds a simulation with the body commands registered, plus
// a command channel deep enough to queue a scenario before the loop starts.
func newCommandableSim(t *testing.T) (*step_sim.Simulation, chan step_sim.Command) {
	t.Helper()

	fm, err := getSimulationMesh()
	require.NoError(t, err)

	cmdChan := make(chan step_sim.Command, 16)
	sim := step_sim.NewSimulation(context.Background(), fm, cmdChan, sink.NewNoopSink()).Init(initSim)
	// initSim paces the interactive sim to real time; tests run flat out so a
	// simulated hour costs milliseconds rather than an hour.
	sim.Pacer.SetFactor(step_sim.Uncapped)
	return sim, cmdChan
}

// bodyComponent returns a component from inside the human's mesh.
func bodyComponent(t *testing.T, sim *step_sim.Simulation, name string) *component.Component {
	t.Helper()

	body := helper.FindHumanComponent(sim.FM)
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
		commands   []step_sim.Command
		component  string
		assertions func(t *testing.T, state component.State)
	}{
		{
			name:      "drinking water reaches the intake controller",
			commands:  []step_sim.Command{"intake:water 500ml"},
			component: "controller:intake",
			assertions: func(t *testing.T, state component.State) {
				assert.Equal(t, 500.0, state.Get(controller.TotalWaterMl))
				// The swallow is released on the next tick, so nothing is left pending.
				assert.Equal(t, 0.0, state.Get(controller.PendingWaterMl))
			},
		},
		{
			name:      "volume units are interchangeable",
			commands:  []step_sim.Command{"intake:water 0.5l"},
			component: "controller:intake",
			assertions: func(t *testing.T, state component.State) {
				assert.Equal(t, 500.0, state.Get(controller.TotalWaterMl))
			},
		},
		{
			name:      "eating reaches the intake controller",
			commands:  []step_sim.Command{"intake:food 200kcal"},
			component: "controller:intake",
			assertions: func(t *testing.T, state component.State) {
				assert.Equal(t, 200.0, state.Get(controller.TotalFoodKcal))
			},
		},
		{
			name: "several commands accumulate",
			commands: []step_sim.Command{
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
			commands:  []step_sim.Command{"activity:start 8 200ms"},
			component: "controller:physical_stress",
			assertions: func(t *testing.T, state component.State) {
				// The run is longer than the activity, so the body is back at rest.
				assert.Equal(t, controller.RestingIntensity, state.Get(controller.ActivityIntensity))
				assert.Equal(t, 0.0, state.Get(controller.ActivityRemaining))
			},
		},
		{
			name:      "an activity without a duration keeps going",
			commands:  []step_sim.Command{"activity:start 8"},
			component: "controller:physical_stress",
			assertions: func(t *testing.T, state component.State) {
				assert.Equal(t, 8.0, state.Get(controller.ActivityIntensity))
				assert.Equal(t, controller.Indefinite, state.Get(controller.ActivityRemaining))
			},
		},
		{
			name:      "stopping returns the body to rest",
			commands:  []step_sim.Command{"activity:start 8", "activity:stop"},
			component: "controller:physical_stress",
			assertions: func(t *testing.T, state component.State) {
				assert.Equal(t, controller.RestingIntensity, state.Get(controller.ActivityIntensity))
			},
		},
		{
			name:      "voiding reaches the excretion controller",
			commands:  []step_sim.Command{"excretion:urinate", "excretion:defecate"},
			component: "controller:excretion",
			assertions: func(t *testing.T, state component.State) {
				assert.Equal(t, 1.0, state.Get(controller.TotalUrinations))
				assert.Equal(t, 1.0, state.Get(controller.TotalDefecations))
			},
		},
		{
			name:      "an emotional stimulus raises arousal and then fades",
			commands:  []step_sim.Command{"emotion:stimulus 0.8 -0.5"},
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
			sim, cmdChan := newCommandableSim(t)

			// Queue the scenario before the loop starts; the sim drains all
			// pending commands at the top of its first iteration.
			for _, cmd := range tt.commands {
				cmdChan <- cmd
			}

			target := bodyComponent(t, sim, tt.component)
			helper.RunSimulationAndThen(sim, 500*time.Millisecond, func() {
				tt.assertions(t, target.State())
			})
		})
	}
}

// Scheduling is expressed in simulated time. These use short intervals so the
// test runs quickly; the interval arithmetic itself is covered exhaustively by
// the scheduler's own unit tests against a fake clock.

func Test_ScheduledCommandsReachTheBody(t *testing.T) {
	sim, cmdChan := newCommandableSim(t)

	// Six intervals fit in the run, and the first fires one interval in rather
	// than immediately, so five to six swallows are expected.
	cmdChan <- "every 50ms intake:water 250ml"

	intake := bodyComponent(t, sim, "controller:intake")
	helper.RunSimulationAndThen(sim, 300*time.Millisecond, func() {
		total := intake.State().Get(controller.TotalWaterMl).(float64)
		assert.GreaterOrEqual(t, total, 4*250.0, "repeating job fired too few times")
		assert.LessOrEqual(t, total, 6*250.0, "repeating job fired too many times")
	})
}

func Test_ScheduledCommandRespectsItsDelay(t *testing.T) {
	sim, cmdChan := newCommandableSim(t)

	// Due well after the run ends, so it must never fire.
	cmdChan <- "after 10s intake:water 500ml"

	intake := bodyComponent(t, sim, "controller:intake")
	helper.RunSimulationAndThen(sim, 200*time.Millisecond, func() {
		assert.Equal(t, 0.0, intake.State().Get(controller.TotalWaterMl),
			"a job scheduled beyond the run fired early")
		assert.Len(t, sim.Scheduler.Jobs(), 1, "the pending job should still be queued")
	})
}

func Test_CancelledJobNeverFires(t *testing.T) {
	sim, cmdChan := newCommandableSim(t)

	cmdChan <- "every 50ms intake:water 250ml"
	cmdChan <- "cancel all"

	intake := bodyComponent(t, sim, "controller:intake")
	helper.RunSimulationAndThen(sim, 300*time.Millisecond, func() {
		assert.Equal(t, 0.0, intake.State().Get(controller.TotalWaterMl))
		assert.Empty(t, sim.Scheduler.Jobs())
	})
}

func Test_ScenarioRunsItsStepsInOrder(t *testing.T) {
	sim, cmdChan := newCommandableSim(t)

	// The shape the northstar asked for: eat, let time pass, then exert.
	cmdChan <- "intake:food 200kcal; wait 100ms; activity:start 8"

	intake := bodyComponent(t, sim, "controller:intake")
	physical := bodyComponent(t, sim, "controller:physical_stress")

	helper.RunSimulationAndThen(sim, 300*time.Millisecond, func() {
		assert.Equal(t, 200.0, intake.State().Get(controller.TotalFoodKcal),
			"the pre-wait step did not run")
		assert.Equal(t, 8.0, physical.State().Get(controller.ActivityIntensity),
			"the post-wait step did not run")
		assert.Empty(t, sim.Programs.Running(), "the scenario should have finished")
	})
}

func Test_ScenarioWaitsBeforeItsLaterSteps(t *testing.T) {
	sim, cmdChan := newCommandableSim(t)

	// The wait outlasts the run, so only the first step should ever happen.
	cmdChan <- "intake:food 200kcal; wait 10s; activity:start 8"

	intake := bodyComponent(t, sim, "controller:intake")
	physical := bodyComponent(t, sim, "controller:physical_stress")

	helper.RunSimulationAndThen(sim, 200*time.Millisecond, func() {
		assert.Equal(t, 200.0, intake.State().Get(controller.TotalFoodKcal),
			"the pre-wait step did not run")
		assert.Equal(t, controller.RestingIntensity, physical.State().Get(controller.ActivityIntensity),
			"the post-wait step ran before its wait elapsed")
		assert.Len(t, sim.Programs.Running(), 1, "the scenario should still be waiting")
	})
}

func Test_NamedScenarioIsReusable(t *testing.T) {
	sim, cmdChan := newCommandableSim(t)

	cmdChan <- "script hydrate intake:water 250ml; wait 50ms; intake:water 250ml"
	cmdChan <- "run hydrate"

	intake := bodyComponent(t, sim, "controller:intake")
	helper.RunSimulationAndThen(sim, 300*time.Millisecond, func() {
		assert.Equal(t, 500.0, intake.State().Get(controller.TotalWaterMl),
			"both steps of the named scenario should have run")

		// Defining a scenario must not consume it.
		_, defined := sim.Programs.Steps("hydrate")
		assert.True(t, defined, "the definition should survive being run")
	})
}

func Test_DefiningAScenarioDoesNotRunIt(t *testing.T) {
	sim, cmdChan := newCommandableSim(t)

	// The definition contains step separators; treating it as a scenario to run
	// would drink the water at definition time.
	cmdChan <- "script hydrate intake:water 250ml; wait 50ms; intake:water 250ml"

	intake := bodyComponent(t, sim, "controller:intake")
	helper.RunSimulationAndThen(sim, 200*time.Millisecond, func() {
		assert.Equal(t, 0.0, intake.State().Get(controller.TotalWaterMl))
		assert.Empty(t, sim.Programs.Running())
	})
}

func Test_UnknownBodyCommandDoesNotStopTheSimulation(t *testing.T) {
	sim, cmdChan := newCommandableSim(t)

	// A command addressed to a namespace no controller owns must be reported and
	// dropped: an activation error would halt the entire mesh, which would mean a
	// typo could kill the patient.
	cmdChan <- "intake:plutonium 1kg"

	intake := bodyComponent(t, sim, "controller:intake")
	helper.RunSimulationAndThen(sim, 200*time.Millisecond, func() {
		assert.Equal(t, 0.0, intake.State().Get(controller.TotalWaterMl))
		assert.Equal(t, 0.0, intake.State().Get(controller.TotalFoodKcal))
	})
}
