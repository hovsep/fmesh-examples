package helper

import (
	"fmt"
	"time"

	"github.com/hovsep/fmesh/signal"
)

// PackTick builds a tick signal with scalars for numeric fields and wall time
func PackTick(seq uint64, simDuration time.Duration, simWallTime time.Time, duration time.Duration) *signal.Signal {
	return signal.New("tick").
		WithLabel("category", "time").
		WithLabel("type", "tick").
		WithScalar("tick_count", float64(seq)).
		WithScalar("sim_duration_ms", float64(simDuration.Milliseconds())).
		WithScalar("sim_wall_time_sec", float64(simWallTime.Unix())).
		WithScalar("sim_wall_time_nsec", float64(simWallTime.Nanosecond())).
		WithScalar("delta_t_ms", float64(duration.Milliseconds()))
}

// UnpackTick returns components of a tick signal
func UnpackTick(tick *signal.Signal) (seq uint64, simDuration time.Duration, simWallTime time.Time, duration time.Duration, err error) {
	if tick == nil {
		return 0, 0, time.Time{}, 0, fmt.Errorf("tick signal cannot be nil")
	}

	s := tick.Scalars()
	seq = uint64(s.ValueOrDefault("tick_count", 0))
	simDuration = time.Duration(s.ValueOrDefault("sim_duration_ms", 0)) * time.Millisecond
	simWallTime = time.Unix(int64(s.ValueOrDefault("sim_wall_time_sec", 0)), int64(s.ValueOrDefault("sim_wall_time_nsec", 0)))
	duration = time.Duration(s.ValueOrDefault("delta_t_ms", 0)) * time.Millisecond
	return
}

// TickDurationInSec returns the duration of a tick in seconds
func TickDurationInSec(tick *signal.Signal) (float64, error) {
	_, _, _, duration, err := UnpackTick(tick)
	return float64(duration) / float64(time.Second), err
}
