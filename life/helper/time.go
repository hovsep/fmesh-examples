package helper

import (
	"fmt"
	"time"

	"github.com/hovsep/fmesh-examples/life/common"
	"github.com/hovsep/fmesh/signal"
)

// PackTick returns a signal containing the tick meta-data
func PackTick(seq uint64, simTime time.Duration, simWallTime time.Time, duration time.Duration) *signal.Signal {
	return signal.New(signal.NewGroup().Add(
		signal.New(seq).AddLabel(common.TickMeta, common.TickCount),
		signal.New(simTime).AddLabel(common.TickMeta, common.SimTime),
		signal.New(simWallTime).AddLabel(common.TickMeta, common.SimWallTime),
		signal.New(duration).AddLabel(common.TickMeta, common.DeltaT),
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

		tickMetaLabel, labelErr := tickMetaSig.Labels().Value(common.TickMeta)
		if labelErr != nil {
			return labelErr
		}

		tickMeta := tickMetaSig.PayloadOrNil()
		if tickMeta == nil {
			return fmt.Errorf("tick signal payload cannot be nil")
		}

		switch tickMetaLabel {
		case common.TickCount:
			seq = tickMeta.(uint64)
			return nil

		case common.SimTime:
			simTime = tickMeta.(time.Duration)
			return nil

		case common.SimWallTime:
			simWallTime = tickMeta.(time.Time)
			return nil

		case common.DeltaT:
			duration = tickMeta.(time.Duration)
			return nil

		default:
			return fmt.Errorf("tick signal label %s not supported", tickMetaLabel)
		}
	})
	return
}

// TickDurationInSec returns the duration of a tick in seconds
func TickDurationInSec(tick *signal.Signal) (float64, error) {
	_, _, _, duration, err := UnpackTick(tick)
	return float64(duration) / float64(time.Second), err
}
