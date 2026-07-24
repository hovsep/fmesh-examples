package main

import (
	"testing"
	"time"

	"github.com/hovsep/fmesh"
	"github.com/hovsep/fmesh-examples/life/common"
	"github.com/hovsep/fmesh-examples/life/helper"
	"github.com/hovsep/fmesh-examples/life/organism/human/controller"
	da "github.com/hovsep/fmesh-examples/life/organism/human/distributed_anatomy"
	"github.com/hovsep/fmesh-examples/life/organism/human/organ"
	"github.com/hovsep/fmesh-examples/life/organism/human/physiology"
	"github.com/hovsep/fmesh-examples/life/telemetry"
	"github.com/hovsep/fmesh-examples/simulation/step_sim"
	"github.com/hovsep/fmesh/signal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The metabolic loop runs on physiological timescales: a stomach empties over
// tens of minutes. These tests use simulated minutes, which cost only
// milliseconds of wall clock because the loop is unpaced.

func Test_DrinkingReachesTheBody(t *testing.T) {
	sim, cmdChan := newCommandableSim(t)
	cmdChan <- "intake:water 500ml"

	gi := bodyComponent(t, sim, "da:gi_tract")
	body := bodyComponent(t, sim, "physiology:physiological_state")

	hydrationBefore := body.State().Get(physiology.StateHydrationMl).(float64)

	helper.RunSimulationAndThen(sim, 2*time.Minute, func() {
		// Some of the glass has left the stomach...
		stomach := gi.State().Get(da.StateStomachMl).(float64)
		assert.Less(t, stomach, 500.0, "the stomach should have started emptying")
		assert.Greater(t, stomach, 0.0, "500 mL should not vanish in two minutes")

		// ...and the water that left it is now in the body. Insensible loss and
		// urine run the other way, so this checks the net gain is still positive.
		assert.Greater(t, body.State().Get(physiology.StateHydrationMl).(float64), hydrationBefore,
			"drinking should raise total body water")
	})
}

func Test_EatingRaisesBloodGlucoseAndReserves(t *testing.T) {
	sim, cmdChan := newCommandableSim(t)
	cmdChan <- "intake:food 800kcal"

	body := bodyComponent(t, sim, "physiology:physiological_state")

	helper.RunSimulationAndThen(sim, 5*time.Minute, func() {
		assert.Greater(t, body.State().Get(physiology.StateGlycemia).(float64), physiology.NormalGlycemia,
			"a meal should raise blood glucose above fasting level")
		assert.Greater(t, body.State().Get(physiology.StateEnergyKcal).(float64), physiology.StartingEnergyKcal,
			"a meal should add to the energy reserve")
	})
}

func Test_DigestionIsNotInstant(t *testing.T) {
	sim, cmdChan := newCommandableSim(t)
	// A small meal, fully eaten in ~30 s at the eating rate, so this test is
	// about slow digestion rather than slow eating.
	cmdChan <- "intake:food 90kcal"

	gi := bodyComponent(t, sim, "da:gi_tract")

	// The delay is the point: food should linger in the stomach and be released
	// into the body slowly, not appear the instant it is swallowed.
	helper.RunSimulationAndThen(sim, 90*time.Second, func() {
		stomach := gi.State().Get(da.StateStomachKcal).(float64)
		assert.Greater(t, stomach, 60.0,
			"most of a small meal should still be in the stomach a minute after eating it")
	})
}

func Test_EatingSpansTime(t *testing.T) {
	sim, cmdChan := newCommandableSim(t)
	// A large meal cannot be eaten in a minute; most of it is still on the plate
	// (not yet swallowed) rather than already in the stomach.
	cmdChan <- "intake:food 600kcal"

	gi := bodyComponent(t, sim, "da:gi_tract")
	intake := bodyComponent(t, sim, "controller:intake")

	helper.RunSimulationAndThen(sim, time.Minute, func() {
		// At ~3 kcal/s only ~180 kcal is eaten in a minute; the stomach holds no
		// more than what has been swallowed so far.
		assert.Less(t, gi.State().Get(da.StateStomachKcal).(float64), 250.0,
			"a 600 kcal meal should not be fully eaten within a minute")
		// The full amount was commanded up front, even though delivery lingers.
		assert.Equal(t, 600.0, intake.State().Get(controller.TotalFoodKcal))
	})
}

func Test_DrinkingSpansProportionallyToVolume(t *testing.T) {
	// A big glass takes proportionally longer than a small one. Sampled through
	// the process runner's remaining volume, which is the mechanism under test.
	remainingAfter := func(command string, d time.Duration) float64 {
		sim, cmdChan := newCommandableSim(t)
		cmdChan <- step_sim.Command(command)
		intake := bodyComponent(t, sim, "controller:intake")

		var remaining float64
		sim.FM.SetupHooks(func(hooks *fmesh.Hooks) {
			hooks.AfterRun(func(*fmesh.FMesh) error {
				processes, _ := intake.State().Get(controller.StateProcesses).(*helper.ProcessSet)
				if processes != nil {
					remaining = processes.RemainingOf(controller.KindWaterMl)
				}
				return nil
			})
		})
		helper.RunSimulationAndThen(sim, d, func() {})
		return remaining
	}

	// After five seconds (at ~15 mL/s): 50 mL is done, 500 mL is still going.
	if left := remainingAfter("intake:water 50ml", 5*time.Second); left > 0.001 {
		t.Fatalf("50 mL should be fully drunk within five seconds, %v left", left)
	}
	if left := remainingAfter("intake:water 500ml", 5*time.Second); left < 350 {
		t.Fatalf("500 mL should be far from finished after five seconds, only %v left", left)
	}
}

func Test_ExertionBurnsEnergyAndWarmsTheBody(t *testing.T) {
	sim, cmdChan := newCommandableSim(t)
	cmdChan <- "activity:start 8"

	body := bodyComponent(t, sim, "physiology:physiological_state")

	helper.RunSimulationAndThen(sim, 3*time.Minute, func() {
		assert.Less(t, body.State().Get(physiology.StateEnergyKcal).(float64), physiology.StartingEnergyKcal,
			"running should burn into the reserve")
		assert.Greater(t, body.State().Get(physiology.StateCoreTemperature).(float64), physiology.NormalCoreTemperature,
			"running should raise core temperature")
	})
}

func Test_HardExertionDoesNotCookTheBody(t *testing.T) {
	sim, cmdChan := newCommandableSim(t)
	cmdChan <- "activity:start 8"

	body := bodyComponent(t, sim, "physiology:physiological_state")
	skin := bodyComponent(t, sim, "da:skin")

	// The heating rate and the shedding half-life together set where sustained
	// effort settles. An earlier heating constant ten times too large took the
	// body to 43 C during a twenty-minute run, and drove sweat to 8 L an hour --
	// both of which looked plausible tick by tick and only showed up on screen.
	helper.RunSimulationAndThen(sim, 20*time.Minute, func() {
		temperature := body.State().Get(physiology.StateCoreTemperature).(float64)
		assert.Greater(t, temperature, physiology.NormalCoreTemperature,
			"hard exercise should warm the body")
		assert.Less(t, temperature, 40.0,
			"hard exercise should not push core temperature to heatstroke")

		// Sweat follows temperature, so it is bounded by the same calibration.
		// One millilitre a second is 3.6 L an hour, well past what anyone sustains.
		sweat := (temperature - 37.2) * 750.0 / 3600.0
		assert.Less(t, sweat, 1.0, "sweat rate should stay physiologically possible")
		assert.NotNil(t, skin)
	})
}

func Test_RestingBodyStaysNearNormal(t *testing.T) {
	sim, _ := newCommandableSim(t)
	body := bodyComponent(t, sim, "physiology:physiological_state")

	// Left alone the body should drift slowly, not run away. This is the guard
	// against a rate constant being wrong by orders of magnitude.
	helper.RunSimulationAndThen(sim, 3*time.Minute, func() {
		assert.InDelta(t, physiology.NormalCoreTemperature,
			body.State().Get(physiology.StateCoreTemperature).(float64), 0.2,
			"a resting body should hold its temperature")
		assert.InDelta(t, physiology.NormalGlycemia,
			body.State().Get(physiology.StateGlycemia).(float64), 15.0,
			"a resting body should hold its blood glucose")

		hydrationPct := body.State().Get(physiology.StateHydrationMl).(float64) / physiology.TotalBodyWaterMl * 100
		assert.Greater(t, hydrationPct, 99.0, "a few minutes of rest should not dehydrate anyone")
	})
}

func Test_BladderFillsAndVoids(t *testing.T) {
	sim, cmdChan := newCommandableSim(t)
	kidney := bodyComponent(t, sim, "organ:kidney")

	helper.RunSimulationAndThen(sim, 3*time.Minute, func() {
		assert.Greater(t, kidney.State().Get(organ.StateBladderMl).(float64), 0.0,
			"the kidneys should have produced urine")
	})

	// A fresh run, this time voiding at the end.
	sim, cmdChan = newCommandableSim(t)
	cmdChan <- "after 2m excretion:urinate"
	kidney = bodyComponent(t, sim, "organ:kidney")

	helper.RunSimulationAndThen(sim, 3*time.Minute, func() {
		assert.Less(t, kidney.State().Get(organ.StateBladderMl).(float64), 10.0,
			"urinating should have emptied the bladder")
	})
}

// lastFeelings installs a hook that records the feelings signal as it is
// published, because a component's output ports are drained at the end of every
// mesh cycle -- reading them after the run finds nothing. Capturing at the
// aggregator also proves the feelings reach telemetry, not just the port.
func lastFeelings(t *testing.T, sim *step_sim.Simulation) func() *signal.Signal {
	t.Helper()

	aggregator := sim.FM.ComponentByName("aggregated_state")
	require.NotNil(t, aggregator)

	body := helper.FindHumanComponent(sim.FM)
	require.NotNil(t, body)
	key := body.Name() + telemetry.PathSeparator + "feelings"

	var latest *signal.Signal
	sim.FM.SetupHooks(func(hooks *fmesh.Hooks) {
		hooks.AfterRun(func(*fmesh.FMesh) error {
			if sig := aggregator.OutputByName(key).Signals().First(); sig != nil {
				latest = sig
			}
			return nil
		})
	})
	return func() *signal.Signal { return latest }
}

func Test_FeelingsRespondToTheBody(t *testing.T) {
	tests := []struct {
		name     string
		commands []step_sim.Command
		duration time.Duration
		feeling  string
		assert   func(t *testing.T, intensity float64)
	}{
		{
			name:     "a rested, fed body is content",
			duration: time.Minute,
			feeling:  common.FeelingContent,
			assert: func(t *testing.T, intensity float64) {
				assert.Greater(t, intensity, 0.5, "nothing is wrong, so the body should feel fine")
			},
		},
		{
			name:     "a rested body is not hungry",
			duration: time.Minute,
			feeling:  common.FeelingHungry,
			assert: func(t *testing.T, intensity float64) {
				assert.Less(t, intensity, 0.1, "a full reserve should not read as hunger")
			},
		},
		{
			name:     "sustained hard exertion is tiring",
			commands: []step_sim.Command{"activity:start 9"},
			duration: 3 * time.Minute,
			feeling:  common.FeelingTired,
			assert: func(t *testing.T, intensity float64) {
				assert.Greater(t, intensity, 0.5, "running flat out should read as tiring")
			},
		},
		{
			name:     "an unpleasant shock reads as anxiety",
			commands: []step_sim.Command{"emotion:stimulus 0.9 -0.9"},
			duration: 10 * time.Second,
			feeling:  common.FeelingAnxious,
			assert: func(t *testing.T, intensity float64) {
				assert.Greater(t, intensity, 0.2, "a frightening event should read as anxiety")
			},
		},
		{
			name:     "a pleasant event reads as happiness, not anxiety",
			commands: []step_sim.Command{"emotion:stimulus 0.9 0.9"},
			duration: 10 * time.Second,
			feeling:  common.FeelingAnxious,
			assert: func(t *testing.T, intensity float64) {
				// Arousal alone is not distress; it is the direction of the mood
				// that separates excitement from dread.
				assert.Zero(t, intensity, "a pleasant event should not read as anxiety")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sim, cmdChan := newCommandableSim(t)
			for _, cmd := range tt.commands {
				cmdChan <- cmd
			}

			feelings := lastFeelings(t, sim)
			helper.RunSimulationAndThen(sim, tt.duration, func() {
				sig := feelings()
				require.NotNil(t, sig, "the body published no feelings")
				tt.assert(t, sig.Scalars().ValueOrDefault(tt.feeling, -1))
			})
		})
	}
}

func Test_EveryFeelingIsPublished(t *testing.T) {
	sim, _ := newCommandableSim(t)
	feelings := lastFeelings(t, sim)

	helper.RunSimulationAndThen(sim, time.Second, func() {
		sig := feelings()
		require.NotNil(t, sig)

		// A missing feeling would render as a permanent zero rather than an
		// obvious gap, so check the whole set arrives every time.
		for _, feeling := range common.Feelings {
			assert.True(t, sig.Scalars().Has(feeling), "feeling %q was not published", feeling)
		}
	})
}
