package physiology

import (
	"github.com/hovsep/fmesh-examples/life/common"
	"github.com/hovsep/fmesh-examples/life/helper"
	"github.com/hovsep/fmesh/component"
	"github.com/hovsep/fmesh/signal"
)

const (
	LastBrainActivity                   common.State = "last_brain_activity"
	defaultBrainActivitySmoothingFactor              = 0.1    // alpha in ema
	defaultBrainActivityThreshold                    = 0.0001 // epsilon in ema
)

// GetObservableState ...
func GetObservableState() *component.Component {
	return component.New("physiology:observable_state").
		WithDescription("Observable state of the human being (e.g., temperature, blood pressure etc)").
		WithInitialState(func(st component.State) {
			st.Set(LastBrainActivity, 0.0)
		}).
		AddInputs("time", "brain_activity", "heart_cardiac_activation", "heart_rate").
		AddOutputs("brain_activity", "brain_activity_trend", "is_alive", "heart_cardiac_activation", "heart_rate").
		WithActivationFunc(func(this *component.Component) error {
			// Are we alive?
			this.OutputByName("is_alive").PutPayloads(this.InputByName("brain_activity").HasSignals())

			// Calculate brain activity trend
			currentBrainActivity := helper.AsF64(this.InputByName("brain_activity").Signals().First())

			lastSmoothedBrainActivity := this.State().Get(LastBrainActivity).(float64)
			ema := helper.NewEMA(defaultBrainActivitySmoothingFactor, lastSmoothedBrainActivity, defaultBrainActivityThreshold)

			smoothedBrainActivity := ema.Update(currentBrainActivity)
			brainActivityTrend := ema.ClassifyTrend(currentBrainActivity)

			this.State().Set(LastBrainActivity, smoothedBrainActivity)
			this.Logger().Printf("Brain activity: %f, trend: %s", smoothedBrainActivity, brainActivityTrend)
			this.OutputByName("brain_activity").PutSignals(signal.New(smoothedBrainActivity))
			this.OutputByName("brain_activity_trend").PutSignals(signal.New(brainActivityTrend))

			// Heart signals
			helper.MultiForward(
				helper.PortPair{
					this.InputByName("heart_cardiac_activation"),
					this.OutputByName("heart_cardiac_activation"),
				},
				helper.PortPair{
					this.InputByName("heart_rate"),
					this.OutputByName("heart_rate"),
				})

			return nil
		})
}
