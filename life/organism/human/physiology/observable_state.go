package physiology

import (
	"fmt"

	"github.com/hovsep/fmesh-examples/life/common"
	"github.com/hovsep/fmesh-examples/life/helper"
	"github.com/hovsep/fmesh-examples/life/telemetry"
	. "github.com/hovsep/fmesh-examples/life/unit"
	"github.com/hovsep/fmesh/component"
	"github.com/hovsep/fmesh/signal"
)

const (
	LastBrainActivity                   common.State = "last_brain_activity"
	defaultBrainActivitySmoothingFactor              = 0.1 * DNCS    // alpha in ema
	defaultBrainActivityThreshold                    = 0.0001 * DNCS // epsilon in ema
)

// GetObservableState returns the body's telemetry hub: everything the outside
// world can observe about the human leaves through here.
//
// Its ports and most of its behaviour come from telemetry.Catalog, so publishing
// a new value is a catalog entry rather than an edit here. Only values that need
// deriving (liveness, the smoothed brain activity and its trend) have handlers.
func GetObservableState() (*component.Component, error) {
	c, err := component.New("physiology:observable_state",
		component.WithDescription("Observable state of the human being (e.g., temperature, blood pressure etc)"),
		component.WithInputs(append([]string{common.TimePort}, telemetry.SourcePorts()...)...),
		component.WithOutputs(telemetry.Ports()...),
		component.WithActivationFunc(helper.SequentialActivationFunc(
			handleBrainSignals,
			forwardCatalogSignals,
		)),
		component.WithInitialState(func(st component.State) {
			st.Set(LastBrainActivity, 0.0)
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("physiology:observable_state: %w", err)
	}
	return c, nil
}

// handleBrainSignals derives the values that are more than a passed-through
// reading: whether the body is alive at all, and where its brain activity is
// heading.
func handleBrainSignals(this *component.Component) error {
	if !this.InputByName("brain_activity").HasSignals() {
		return nil
	}

	// Telemetry is numeric end to end, so liveness travels as 1/0 rather than a bool.
	this.OutputByName("is_alive").PutPayloads(1.0)

	// Calculate brain activity trend
	currentBrainActivity, err := helper.AsF64(this.InputByName("brain_activity").Signals().First())
	if err != nil {
		return err
	}
	lastSmoothedBrainActivity := this.State().Get(LastBrainActivity).(float64)

	// Exponential Moving Average helps to determine trend without storing historical data
	ema := helper.NewEMA(defaultBrainActivitySmoothingFactor, lastSmoothedBrainActivity, defaultBrainActivityThreshold)
	smoothedBrainActivity := ema.Update(currentBrainActivity)
	brainActivityTrend := ema.ClassifyTrend(currentBrainActivity)

	this.State().Set(LastBrainActivity, smoothedBrainActivity)
	this.OutputByName("brain_activity").PutPayloads(smoothedBrainActivity)
	// The trend rides as a number so it survives telemetry; the human-readable
	// name stays available in-mesh as a label.
	return this.OutputByName("brain_activity_trend").PutSignals(
		signal.New(helper.TrendCode(brainActivityTrend)).WithLabel("trend", brainActivityTrend),
	)
}

// forwardCatalogSignals passes every non-derived reading straight through.
func forwardCatalogSignals(this *component.Component) error {
	passThrough := telemetry.PassThrough()

	pairs := make([]helper.PortPair, 0, len(passThrough))
	for _, m := range passThrough {
		pairs = append(pairs, helper.PortPair{
			this.InputByName(m.Source),
			this.OutputByName(m.Port),
		})
	}
	return helper.MultiForward(pairs...)
}
