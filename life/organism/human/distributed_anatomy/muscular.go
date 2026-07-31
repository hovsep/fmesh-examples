package da

import (
	"context"
	"fmt"

	"github.com/hovsep/fmesh-examples/life/plugin/perfusion"
	"github.com/hovsep/fmesh-examples/simulation"
	"github.com/hovsep/fmesh-examples/simulation/mathx"
	"github.com/hovsep/fmesh-examples/simulation/simtime"
	"github.com/hovsep/fmesh/component"
	"github.com/hovsep/fmesh/signal"
)

// Muscle fatigue is a 0..100 scale: exertion builds it up, rest works it off.
const StateFatigue string = "fatigue"

// stateIntensity latches the most recent exertion, since physical_load arrives on
// its own mesh cycle rather than together with the tick.
const stateIntensity string = "intensity"

const (
	// fatiguePerIntensityPerSec is how fast hard work tires the muscles. At
	// intensity 8 (a run) fatigue climbs to its ceiling in a few minutes.
	fatiguePerIntensityPerSec = 0.12

	// fatigueRecoveryHalfLifeSec is how fast fatigue fades at rest.
	fatigueRecoveryHalfLifeSec = 20 * 60.0

	maxFatigue = 100.0
)

// MuscleO2PerMinute is resting skeletal muscle's oxygen demand, in mL/min.
const MuscleO2PerMinute = 50.0

// exertionDemand scales the muscles' oxygen draw by what they are being asked to
// do. Exertion is already expressed as a multiple of resting metabolism, so it
// is the multiplier -- a body running at intensity 8 asks its muscles for eight
// times their resting share.
func exertionDemand(c *component.Component) float64 {
	intensity, ok := c.State().Get(stateIntensity).(float64)
	if !ok {
		return 1
	}
	return max(intensity, 1)
}

// GetMuscularSystem returns the muscular system.
//
// It turns sustained exertion into fatigue that lingers and recovers slowly, so
// running has a cost that outlasts the run. That fatigue is observable and, when
// it maxes out, becomes a source of muscle strain damage (wired separately).
func GetMuscularSystem() (*component.Component, error) {
	c, err := component.New("da:muscular_system",
		component.WithDescription("Muscular system: accrues fatigue under exertion, recovers at rest"),
		component.WithPlugins(
			// Muscle is what makes oxygen demand a variable rather than a
			// constant. At rest it takes about a fifth of the body's oxygen; at
			// hard work it can take ten times its own resting share, which is why
			// exercise is the thing that stresses every other system at once.
			perfusion.New(perfusion.Config{
				Organ:       "muscular_system",
				O2PerMinute: MuscleO2PerMinute,
				Demand:      exertionDemand,
			}),
		),
		component.WithInputs(simulation.TimePort, "physical_load"),
		component.WithOutputs("fatigue"),
		component.WithActivationFunc(component.Sequential(
			latchExertion,
			integrateFatigue,
		)),
		component.WithInitialState(func(state component.State) {
			state.Set(StateFatigue, 0.0)
			state.Set(stateIntensity, 1.0)
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("da:muscular_system: %w", err)
	}
	return c, nil
}

// latchExertion remembers the current exertion whenever it arrives.
func latchExertion(_ context.Context, this *component.Component) error {
	if in := this.InputByName("physical_load"); in.HasSignals() {
		this.State().Set(stateIntensity, signal.AsFloat64OrDefault(in.Signals().First(), 1.0))
	}
	return nil
}

func integrateFatigue(_ context.Context, this *component.Component) error {
	tick := this.InputByName(simulation.TimePort).Signals().First()
	if tick == nil {
		return nil
	}

	dt, err := simtime.TickDurationInSec(tick)
	if err != nil {
		return fmt.Errorf("muscular tick: %w", err)
	}

	// Rest intensity is 1; anything above it tires the muscles, anything at or
	// below lets them recover.
	intensity := this.State().Get(stateIntensity).(float64)

	this.State().Update(StateFatigue, func(v any) any {
		fatigue := v.(float64)
		if intensity > 1.0 {
			fatigue += (intensity - 1.0) * fatiguePerIntensityPerSec * dt
		} else {
			fatigue = mathx.DecayToward(fatigue, 0, dt, fatigueRecoveryHalfLifeSec)
		}
		return mathx.Clamp(fatigue, 0, maxFatigue)
	})

	return this.OutputByName("fatigue").PutPayloads(this.State().Get(StateFatigue).(float64))
}
