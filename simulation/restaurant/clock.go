// This file holds the "clock" component: the restaurant's wall clock. It does
// not run by itself. Before every mesh run a hook nudges it (see mesh.go), and
// it answers with the minute of the day on its "tick" output, which is piped
// to every component that keeps time.
package main

import (
	"context"

	"github.com/hovsep/fmesh/component"
)

// newClock creates the clock, the simplest component here and a first look at
// what every component is made of: a name and a description, input and output
// ports, a state that lives on between runs, and an activation function that
// F-Mesh calls whenever a signal is waiting on an input.
//
// The clock starts at minute 0 and moves on by one minute every time it is
// nudged on "advance"; the payload of the nudge does not matter.
func newClock() (*component.Component, error) {
	return component.New("clock",
		component.WithDescription("the wall clock: one tick per mesh run, carrying the minute of the day"),
		component.WithInputs("advance"),
		component.WithOutputs("tick"),
		component.WithInitialState(func(state component.State) {
			state.Set("minute", 0)
		}),
		component.WithActivationFunc(func(_ context.Context, this *component.Component) error {
			// The state is a map of named values; GetTyped reads one back as
			// the type it was stored with.
			minute, err := this.State().GetTyped[int]("minute")
			if err != nil {
				return err
			}
			// PutPayloads wraps the value in a signal and puts it on the
			// output port; the pipes take it from there.
			if err := this.OutputByName("tick").PutPayloads(minute); err != nil {
				return err
			}
			this.State().Set("minute", minute+1)
			return nil
		}),
	)
}
