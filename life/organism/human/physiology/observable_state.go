package physiology

import (
	"github.com/hovsep/fmesh-examples/life/common"
	"github.com/hovsep/fmesh-examples/life/helper"
	"github.com/hovsep/fmesh/component"
	"github.com/hovsep/fmesh/signal"
)

const (
	LastBrainActivity                   common.State = "last_brain_activity"
	defaultBrainActivitySmoothingFactor              = 0.1  // alpha in ema
	defaultBrainActivityThreshold                    = 0.05 // epsilon in ema
)

// GetObservableState ...
func GetObservableState() *component.Component {
	return component.New("physiology:observable_state").
		WithDescription("Observable state of the human being (e.g., temperature, blood pressure etc)").
		WithInitialState(func(st component.State) {
			st.Set(LastBrainActivity, 0.0)
		}).
		AddInputs("time", "brain_activity").
		AddOutputs("brain_activity", "brain_activity_trend", "is_alive").
		WithActivationFunc(func(this *component.Component) error {
			// Are we alive?
			this.OutputByName("is_alive").PutPayloads(this.InputByName("brain_activity").HasSignals())

			// Calculate brain activity trend
			currentBrainActivity := helper.AsF64(this.InputByName("brain_activity").Signals().First())

			lastSmoothedBrainActivity := this.State().Get(LastBrainActivity).(float64)
			ema := helper.NewEMA(defaultBrainActivitySmoothingFactor, lastSmoothedBrainActivity, defaultBrainActivityThreshold)

			smoothedBrainActivity := ema.Update(currentBrainActivity)
			brainActivityTrend := common.Balanced
			switch ema.ClassifyTrend(currentBrainActivity) {
			case +1:
				brainActivityTrend = common.Rising
				break
			case -1:
				brainActivityTrend = common.Falling
				break
			default:

			}

			this.State().Set(LastBrainActivity, smoothedBrainActivity)
			this.Logger().Printf("Brain activity: %f, trend: %s", smoothedBrainActivity, brainActivityTrend)
			this.OutputByName("brain_activity").PutSignals(signal.New(smoothedBrainActivity))
			return this.OutputByName("brain_activity_trend").PutSignals(signal.New(brainActivityTrend)).ChainableErr()
		})
}
