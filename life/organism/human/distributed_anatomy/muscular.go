package da

import (
	"fmt"

	"github.com/hovsep/fmesh-examples/life/common"
	"github.com/hovsep/fmesh-examples/life/helper"
	"github.com/hovsep/fmesh/component"
)

// Muscle fatigue is a 0..100 scale: exertion builds it up, rest works it off.
const StateFatigue common.State = "fatigue"

// stateIntensity latches the most recent exertion, since physical_load arrives on
// its own mesh cycle rather than together with the tick.
const stateIntensity common.State = "intensity"

const (
	// fatiguePerIntensityPerSec is how fast hard work tires the muscles. At
	// intensity 8 (a run) fatigue climbs to its ceiling in a few minutes.
	fatiguePerIntensityPerSec = 0.12

	// fatigueRecoveryHalfLifeSec is how fast fatigue fades at rest.
	fatigueRecoveryHalfLifeSec = 20 * 60.0

	maxFatigue = 100.0
)

// GetMuscularSystem returns the muscular system.
//
// It turns sustained exertion into fatigue that lingers and recovers slowly, so
// running has a cost that outlasts the run. That fatigue is observable and, when
// it maxes out, becomes a source of muscle strain damage (wired separately).
func GetMuscularSystem() (*component.Component, error) {
	c, err := component.New("da:muscular_system",
		component.WithDescription("Muscular system: accrues fatigue under exertion, recovers at rest"),
		component.WithInputs(common.TimePort, "physical_load"),
		component.WithOutputs("fatigue"),
		component.WithActivationFunc(helper.SequentialActivationFunc(
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
func latchExertion(this *component.Component) error {
	if in := this.InputByName("physical_load"); in.HasSignals() {
		this.State().Set(stateIntensity, helper.AsF64OrDefault(in.Signals().First(), 1.0))
	}
	return nil
}

func integrateFatigue(this *component.Component) error {
	tick := this.InputByName(common.TimePort).Signals().First()
	if tick == nil {
		return nil
	}

	dt, err := helper.TickDurationInSec(tick)
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
			fatigue = helper.DecayToward(fatigue, 0, dt, fatigueRecoveryHalfLifeSec)
		}
		return helper.Clamp(fatigue, 0, maxFatigue)
	})

	return this.OutputByName("fatigue").PutPayloads(this.State().Get(StateFatigue).(float64))
}
