package helper

import (
	"fmt"
	"time"

	"github.com/hovsep/fmesh/signal"
)

// PackTick returns a signal containing the tick meta-data
func PackTick(seq uint64, simTime time.Duration, simWallTime time.Time, duration time.Duration) *signal.Signal {
	return signal.New(signal.NewGroup(
		signal.New(seq).AddLabel("tick_meta", "tick_count"),
		signal.New(simTime).AddLabel("tick_meta", "sim_time"),
		signal.New(simWallTime).AddLabel("tick_meta", "sim_wall_time"),
		signal.New(duration).AddLabel("tick_meta", "dt"),
	))
}

// UnpackTick unpacks a tick signal into its meta-data components
func UnpackTick(tick *signal.Signal) (seq uint64, simTime time.Duration, simWallTime time.Time, duration time.Duration, err error) {
	if tick == nil {
		err = fmt.Errorf("tick signal cannot be nil")
		return
	}

	payload := tick.PayloadOrNil().(*signal.Group)

	if payload == nil {
		err = fmt.Errorf("tick signal payload cannot be nil")
		return
	}

	payload.ForEach(func(tickMeta *signal.Signal) error {
		if tickMeta.Labels().ValueIs("tick_meta", "tick_count") {
			seq = tickMeta.PayloadOrNil().(uint64)
			return nil
		}

		if tickMeta.Labels().ValueIs("tick_meta", "sim_time") {
			simTime = tickMeta.PayloadOrNil().(time.Duration)
			return nil
		}

		if tickMeta.Labels().ValueIs("tick_meta", "sim_wall_time") {
			simWallTime = tickMeta.PayloadOrNil().(time.Time)
			return nil
		}

		if tickMeta.Labels().ValueIs("tick_meta", "dt") {
			duration = tickMeta.PayloadOrNil().(time.Duration)
			return nil
		}

		return fmt.Errorf("unknown tick meta label: %s", tickMeta.Labels().ValueOrDefault("tick_meta", "unknown"))
	})
	return
}
