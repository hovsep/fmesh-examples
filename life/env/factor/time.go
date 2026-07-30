package factor

import (
	"fmt"
	"time"

	"github.com/hovsep/fmesh"
	"github.com/hovsep/fmesh-examples/simulation/simtime"
	"github.com/hovsep/fmesh/component"
)

// DefaultTickDuration is how much simulated time one mesh run represents when
// nothing asks for anything else.
//
// Ten milliseconds is a display decision rather than a physiological one: the
// fastest thing worth watching is the ECG, whose R-peaks alias into nonsense if
// they are sampled much coarser than this. Nothing in the body needs the
// resolution -- breathing and the slow chemistry would look the same at a tenth
// of it.
//
// That matters because the tick is the simulation's whole cost. A hundred mesh
// runs buy one simulated second, so a run of any physiological length is a
// hundred times more work than its duration suggests, and the choice of tick is
// the choice of how long the test suite takes. The app pays for the ECG; tests
// that are watching an hour of chemistry step coarser and finish in a quarter of
// the time.
const DefaultTickDuration = 10 * time.Millisecond

// stateTickDuration holds the step this particular simulation was built with.
const stateTickDuration = "tick_duration"

// tickScalar is where a built mesh records its step, in nanoseconds.
//
// Two things need to agree about how long a tick is: the time component, which
// advances simulated time by it, and the stepping engine, which converts a
// requested speed into a wall-clock budget for one. They used to agree by both
// reading the same constant. Once the step became a choice, that stopped being
// enough -- passing it to the two of them separately is an invitation to pass
// two different values and get a simulation whose clock and pace disagree.
//
// So the mesh carries it. Whoever builds the mesh writes it down, and whoever
// needs it reads it back off the thing itself.
const tickScalar = "tick_duration_ns"

// RecordTick notes on the mesh how much simulated time one run of it represents.
func RecordTick(fm *fmesh.FMesh, tick time.Duration) {
	fm.AddScalar(tickScalar, float64(tick))
}

// TickOf reports the step a mesh was built with.
func TickOf(fm *fmesh.FMesh) time.Duration {
	return time.Duration(fm.Scalars().ValueOrDefault(tickScalar, float64(DefaultTickDuration)))
}

// GetTimeComponent returns the time component of the habitat, stepping the given
// amount of simulated time per mesh run.
func GetTimeComponent(tick time.Duration) (*component.Component, error) {
	c, err := component.New("time",
		component.WithDescription("Time management for the simulation"),
		component.WithInputs("ctl"),
		component.WithOutputs("tick"),
		component.WithActivationFunc(func(this *component.Component) error {
			// No need to check for inputs, just tick on every activation

			step := this.State().Get(stateTickDuration).(time.Duration)

			this.State().Update("tick_count", func(v any) any {
				return v.(uint64) + 1
			})

			this.State().Update("sim_duration", func(v any) any {
				return v.(time.Duration) + step
			})

			simStartTime := this.State().Get("sim_start_time").(time.Time)
			simDuration := this.State().Get("sim_duration").(time.Duration)
			this.State().Update("sim_wall_time", func(v any) any {
				return simStartTime.Add(simDuration)
			})

			nextTick := simtime.PackTick(
				this.State().Get("tick_count").(uint64),
				this.State().Get("sim_duration").(time.Duration),
				this.State().Get("sim_wall_time").(time.Time),
				step,
			)
			return this.OutputByName("tick").PutSignals(nextTick)
		}),
		component.WithInitialState(func(state component.State) {
			state.Set("tick_count", uint64(0))          // Discrete step counter
			state.Set("sim_duration", time.Duration(0)) // Elapsed simulated duration
			state.Set("sim_start_time", time.Now())     // Fixed wall-clock anchor
			state.Set("sim_wall_time", time.Now())      // Simulation wall-clock time
			state.Set(stateTickDuration, tick)
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("time component: %w", err)
	}
	return c, nil
}
