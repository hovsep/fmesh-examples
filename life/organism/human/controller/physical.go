package controller

import (
	"context"
	"fmt"

	"github.com/hovsep/fmesh-examples/simulation"
	"github.com/hovsep/fmesh-examples/simulation/command"
	"github.com/hovsep/fmesh-examples/simulation/simtime"
	"github.com/hovsep/fmesh/component"
	"github.com/hovsep/fmesh/meta"
	"github.com/hovsep/fmesh/signal"
)

// Physical activity command verbs.
const (
	VerbStart = "start"
	VerbStop  = "stop"
)

// Physical controller state.
const (
	// ActivityIntensity is the metabolic demand of what the body is currently
	// doing, as a multiple of rest: 1 is sitting still, ~8 is running.
	ActivityIntensity string = "activity_intensity"
	// ActivityRemaining counts down the requested duration in seconds; a
	// negative value means "until told to stop".
	ActivityRemaining string = "activity_remaining_s"
)

// Scalar names on the activity command and the emitted load signal.
const (
	ScalarIntensity = "intensity"
	ScalarDurationS = "duration_s"
)

// Resting metabolic demand, the floor the body returns to when nothing is happening.
const RestingIntensity = 1.0

// Indefinite marks an activity with no requested end.
const Indefinite = -1.0

// GetPhysical returns the physical stress controller.
//
// It holds "what the body is currently doing" as a single intensity, started and
// stopped by command and counted down as simulated time passes. Organs read the
// resulting load rather than knowing about running or lifting.
func GetPhysical() (*component.Component, error) {
	c, err := component.New("controller:physical_stress",
		component.WithDescription("Turns physical activity commands into a sustained metabolic load"),
		component.WithInputs(simulation.TimePort, simulation.ControlPort),
		component.WithOutputs("physical_load"),
		component.WithActivationFunc(component.Sequential(
			acceptActivityCommands,
			emitPhysicalLoad,
		)),
		component.WithInitialState(func(state component.State) {
			state.Set(ActivityIntensity, RestingIntensity)
			state.Set(ActivityRemaining, 0.0)
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("controller:physical_stress: %w", err)
	}
	return c, nil
}

func acceptActivityCommands(_ context.Context, this *component.Component) error {
	return command.ForEach(this, simulation.ControlPort, func(name string, args *meta.Scalars) error {
		switch command.Verb(name) {
		case VerbStart:
			intensity := args.ValueOrDefault(ScalarIntensity, RestingIntensity)
			if intensity < RestingIntensity {
				this.Logger().Printf("activity intensity %v is below resting, clamping\n", intensity)
				intensity = RestingIntensity
			}
			this.State().Set(ActivityIntensity, intensity)
			// Absent a duration the activity runs until stopped.
			this.State().Set(ActivityRemaining, args.ValueOrDefault(ScalarDurationS, Indefinite))
		case VerbStop:
			stopActivity(this)
		default:
			this.Logger().Printf("unknown activity command %q\n", name)
		}
		return nil
	})
}

// emitPhysicalLoad publishes the current demand and ages the activity out.
func emitPhysicalLoad(_ context.Context, this *component.Component) error {
	tick := this.InputByName(simulation.TimePort).Signals().First()
	if tick == nil {
		return nil
	}

	dt, err := simtime.TickDurationInSec(tick)
	if err != nil {
		return fmt.Errorf("physical controller tick: %w", err)
	}

	remaining := this.State().Get(ActivityRemaining).(float64)
	if remaining > 0 {
		remaining -= dt
		if remaining <= 0 {
			// The requested duration has elapsed, so the body settles back down
			// on its own without needing an explicit stop.
			stopActivity(this)
		} else {
			this.State().Set(ActivityRemaining, remaining)
		}
	}

	return this.OutputByName("physical_load").PutSignals(
		signal.New(this.State().Get(ActivityIntensity).(float64)).
			WithLabel("category", "load").
			WithScalar(ScalarIntensity, this.State().Get(ActivityIntensity).(float64)),
	)
}

func stopActivity(this *component.Component) {
	this.State().Set(ActivityIntensity, RestingIntensity)
	this.State().Set(ActivityRemaining, 0.0)
}
