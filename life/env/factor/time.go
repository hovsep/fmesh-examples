package factor

import (
	"fmt"
	"time"

	"github.com/hovsep/fmesh-examples/life/helper"
	"github.com/hovsep/fmesh/component"
)

// DurationPerTick is how much simulated time one mesh run represents. It is the
// conversion factor between tick counts and simulated time, so the pacer needs
// it to translate a requested speed into a wall-clock budget per tick.
const DurationPerTick = 10 * time.Millisecond

// GetTimeComponent returns the time component of the habitat
func GetTimeComponent() (*component.Component, error) {
	c, err := component.New("time",
		component.WithDescription("Time management for the simulation"),
		component.WithInputs("ctl"),
		component.WithOutputs("tick"),
		component.WithActivationFunc(func(this *component.Component) error {
			// No need to check for inputs, just tick on every activation

			this.State().Update("tick_count", func(v any) any {
				return v.(uint64) + 1
			})

			this.State().Update("sim_duration", func(v any) any {
				return v.(time.Duration) + DurationPerTick
			})

			simStartTime := this.State().Get("sim_start_time").(time.Time)
			simDuration := this.State().Get("sim_duration").(time.Duration)
			this.State().Update("sim_wall_time", func(v any) any {
				return simStartTime.Add(simDuration)
			})

			nextTick := helper.PackTick(
				this.State().Get("tick_count").(uint64),
				this.State().Get("sim_duration").(time.Duration),
				this.State().Get("sim_wall_time").(time.Time),
				DurationPerTick,
			)
			return this.OutputByName("tick").PutSignals(nextTick)
		}),
		component.WithInitialState(func(state component.State) {
			state.Set("tick_count", uint64(0))          // Discrete step counter
			state.Set("sim_duration", time.Duration(0)) // Elapsed simulated duration
			state.Set("sim_start_time", time.Now())     // Fixed wall-clock anchor
			state.Set("sim_wall_time", time.Now())      // Simulation wall-clock time
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("time component: %w", err)
	}
	return c, nil
}
