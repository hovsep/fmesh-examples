package helper

import (
	"fmt"
	"time"

	"github.com/hovsep/fmesh/signal"
)

const (
	LabelTickCount   = "tick_count"
	LabelSimTime     = "sim_time"
	LabelSimWallTime = "sim_wall_time"
	LabelDeltaT      = "dt"
	LabelTickMeta    = "tick_meta"
)

// PackTick returns a signal containing the tick meta-data
func PackTick(seq uint64, simTime time.Duration, simWallTime time.Time, duration time.Duration) *signal.Signal {
	return signal.New(signal.NewGroup().Add(
		signal.New(seq).AddLabel(LabelTickMeta, LabelTickCount),
		signal.New(simTime).AddLabel(LabelTickMeta, LabelSimTime),
		signal.New(simWallTime).AddLabel(LabelTickMeta, LabelSimWallTime),
		signal.New(duration).AddLabel(LabelTickMeta, LabelDeltaT),
	),
	)
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

	payload.ForEach(func(tickMetaSig *signal.Signal) error {

		tickMetaLabel, labelErr := tickMetaSig.Labels().Value(LabelTickMeta)
		if labelErr != nil {
			return labelErr
		}

		fmt.Println(tickMetaLabel)

		tickMeta := tickMetaSig.PayloadOrNil()
		if tickMeta == nil {
			return fmt.Errorf("tick signal payload cannot be nil")
		}

		switch tickMetaLabel {
		case LabelTickCount:
			seq = tickMeta.(uint64)
			return nil

		case LabelSimTime:
			simTime = tickMeta.(time.Duration)
			return nil

		case LabelSimWallTime:
			simWallTime = tickMeta.(time.Time)
			return nil

		case LabelDeltaT:
			duration = tickMeta.(time.Duration)
			return nil

		default:
			return fmt.Errorf("tick signal label %s not supported", tickMetaLabel)
		}
	})
	return
}
