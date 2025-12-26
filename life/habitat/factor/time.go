package factor

import (
	"time"

	"github.com/hovsep/fmesh/component"
	"github.com/hovsep/fmesh/signal"
)

const durationPerTick = 10 * time.Millisecond

// GetTimeComponent returns the time component of the habitat
func GetTimeComponent() *component.Component {
	c := component.New("time").
		WithDescription("Time management for the simulation").
		AddLabel("category", "habitat-factor").
		WithInitialState(func(state component.State) {
			state.Set("tick_count", uint64(0))      // Discrete step counter
			state.Set("sim_time", time.Duration(0)) // Elapsed simulated duration
			state.Set("sim_start_time", time.Now()) // Fixed wall-clock anchor
			state.Set("sim_wall_time", time.Now())  // Simulation wall-clock time
		}).
		AddInputs("ctl").
		AddOutputs("tick").
		WithActivationFunc(func(this *component.Component) error {
			// No need to check for inputs, just tick on every activation

			this.State().Update("tick_count", func(v any) any {
				return v.(uint64) + 1
			})

			this.State().Update("sim_time", func(v any) any {
				return v.(time.Duration) + durationPerTick
			})

			simStartTime := this.State().Get("sim_start_time").(time.Time)
			simTime := this.State().Get("sim_time").(time.Duration)
			this.State().Update("sim_wall_time", func(v any) any {
				return simStartTime.Add(simTime)
			})

			tickMeta := signal.NewGroup(
				signal.New(this.State().Get("tick_count")).AddLabel("tick_meta", "index"),
				signal.New(this.State().Get("sim_time")).AddLabel("tick_meta", "sim_time"),
				signal.New(this.State().Get("sim_wall_time")).AddLabel("tick_meta", "sim_wall_time"),
			)
			this.OutputByName("tick").PutSignals(signal.New(tickMeta))
			return nil
		})

	return c
}
