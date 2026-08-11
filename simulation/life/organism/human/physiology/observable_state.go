package physiology

import (
	"context"
	"fmt"

	"github.com/hovsep/fmesh-examples/simulation/life/body"
	"github.com/hovsep/fmesh-examples/simulation/life/telemetry"
	"github.com/hovsep/fmesh-examples/simulation/sim"
	"github.com/hovsep/fmesh/component"
	"github.com/hovsep/fmesh/port"
	"github.com/hovsep/fmesh/signal"
)

const (
	LastBrainActivity                   string = "last_brain_activity"
	defaultBrainActivitySmoothingFactor        = 0.1    // alpha in ema
	defaultBrainActivityThreshold              = 0.0001 // epsilon in ema

	// Death is derived from the brain: once the body has been alive, a whole tick
	// with no brain activity (the brain has failed and stopped emitting) means it
	// is dead. Death latches -- there is no coming back.
	stateEverAlive string = "ever_alive"
	stateDead      string = "dead"
	stateSawBrain  string = "saw_brain_this_tick"
)

// It does derive as well as forward, and the catalog marks which: liveness is
// concluded from brain activity rather than reported by anything, and the brain
// activity trend is smoothed here rather than at the source.
//
// It stops short of indices like cancer risk or expected lifespan on purpose.
// Every other number on these screens is produced by something in the body --
// an organ measured it, or the blood carries it -- and a percentage invented
// here would be the one figure on screen with nothing behind it. This
// simulation shows mechanisms; a risk score is a guess wearing a mechanism's
// clothes.

// GetObservableState returns the body's telemetry hub: everything the outside
// world can observe about the human leaves through here.
//
// Its ports and most of its behaviour come from telemetry.Catalog, so publishing
// a new value is a catalog entry rather than an edit here. Only values that need
// deriving (liveness, the smoothed brain activity and its trend) have handlers.
func GetObservableState() (*component.Component, error) {
	c, err := component.New("physiology:observable_state",
		component.WithDescription("Observable state of the human being (e.g., temperature, blood pressure etc)"),
		component.WithInputs(append([]string{sim.TimePort}, telemetry.SourcePorts()...)...),
		component.WithOutputs(telemetry.Ports()...),
		component.WithActivationFunc(component.Sequential(
			handleBrainSignals,
			forwardCatalogSignals,
		)),
		component.WithInitialState(func(st component.State) {
			st.Set(LastBrainActivity, 0.0)
			st.Set(stateEverAlive, false)
			st.Set(stateDead, false)
			st.Set(stateSawBrain, false)
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
//
// It runs on two kinds of cycle. When brain activity arrives, the body is alive
// and the trend is computed. On the bare tick (no brain activity this cycle), it
// checks whether the brain produced anything during the previous tick: once the
// body has been alive, a whole tick of silence means the brain has failed and the
// body is dead.
func handleBrainSignals(ctx context.Context, this *component.Component) error {
	if this.State().Get(stateDead).(bool) {
		this.OutputByName("is_alive").PutPayloads(0.0)
		return nil
	}

	if !this.InputByName("brain_activity").HasSignals() {
		return checkDeath(ctx, this)
	}

	this.State().Set(stateEverAlive, true)
	this.State().Set(stateSawBrain, true)

	// Telemetry is numeric end to end, so liveness travels as 1/0 rather than a bool.
	this.OutputByName("is_alive").PutPayloads(1.0)

	// Calculate brain activity trend
	currentBrainActivity, err := signal.AsFloat64(this.InputByName("brain_activity").Signals().First())
	if err != nil {
		return err
	}
	lastSmoothedBrainActivity := this.State().Get(LastBrainActivity).(float64)

	// Exponential Moving Average helps to determine trend without storing historical data
	ema := body.NewEMA(defaultBrainActivitySmoothingFactor, lastSmoothedBrainActivity, defaultBrainActivityThreshold)
	smoothedBrainActivity := ema.Update(currentBrainActivity)
	brainActivityTrend := ema.ClassifyTrend(currentBrainActivity)

	this.State().Set(LastBrainActivity, smoothedBrainActivity)
	this.OutputByName("brain_activity").PutPayloads(smoothedBrainActivity)
	// The trend rides as a number so it survives telemetry; the human-readable
	// name stays available in-mesh as a label.
	return this.OutputByName("brain_activity_trend").PutSignals(
		signal.New(body.TrendCode(brainActivityTrend)).WithLabel("trend", brainActivityTrend),
	)
}

// checkDeath runs on a bare tick (no brain activity this cycle). If the body has
// been alive and the brain produced nothing during the whole previous tick, the
// brain has failed and death latches. Otherwise it keeps the alive flag steady.
func checkDeath(_ context.Context, this *component.Component) error {
	if !this.InputByName(sim.TimePort).HasSignals() {
		return nil // not a tick cycle, nothing to decide
	}

	everAlive := this.State().Get(stateEverAlive).(bool)
	sawBrain := this.State().Get(stateSawBrain).(bool)
	// Reset for the coming tick; brain activity, if any, will set it again.
	this.State().Set(stateSawBrain, false)

	switch {
	case everAlive && !sawBrain:
		// A full tick with no brain activity: the brain has stopped.
		this.State().Set(stateDead, true)
		this.OutputByName("is_alive").PutPayloads(0.0)
	case everAlive:
		this.OutputByName("is_alive").PutPayloads(1.0)
	}
	return nil
}

// forwardCatalogSignals passes every non-derived reading straight through.
func forwardCatalogSignals(ctx context.Context, this *component.Component) error {
	passThrough := telemetry.PassThrough()

	pairs := make([]port.Pair, 0, len(passThrough))
	for _, m := range passThrough {
		pairs = append(pairs, port.Pair{
			From: this.InputByName(m.Source),
			To:   this.OutputByName(m.Port),
		})
	}
	return port.MultiForward(ctx, pairs...)
}
