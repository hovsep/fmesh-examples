package factor

import (
	"time"

	"github.com/hovsep/fmesh/component"
	"github.com/hovsep/fmesh/signal"
)

const durationPerTick = 10 * time.Millisecond

// GetTimeComponent returns the time component of the environment
func GetTimeComponent() *component.Component {
	c := component.New("time").
		WithDescription("Time").
		WithInitialState(func(state component.State) {
			state.Set("current_time_rel", uint64(0))        // Monotonic integer counter
			state.Set("current_time_abs", time.Duration(0)) // Elapsed time in milliseconds
		}).
		AddInputs("ctl").
		AddOutputs("tick").
		WithActivationFunc(func(this *component.Component) error {
			tick := signal.New(durationPerTick)

			currentTimeRel := this.State().Get("current_time_rel").(uint64)
			currentTimeAbs := this.State().Get("current_time_abs").(time.Duration)

			defer func() {
				this.State().Set("current_time_rel", currentTimeRel+1)
				this.State().Set("current_time_abs", currentTimeAbs+tick.PayloadOrNil().(time.Duration))

			}()
			this.OutputByName("tick").PutSignals(tick)
			return nil
		})

	return c
}
