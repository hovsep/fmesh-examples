// This file holds the "clock" component: the restaurant's wall clock. It does
// not run by itself. Before every mesh run the program nudges it, and it
// answers with the minute of the day on its "tick" output, which is piped to
// every component that keeps time.
package main

import (
	"context"

	"github.com/hovsep/fmesh/component"
)

// newClock creates the clock. It starts at minute 0 and moves on by one minute
// every time it is nudged on "advance"; the payload of the nudge does not matter.
func newClock() (*component.Component, error) {
	return component.New("clock",
		component.WithDescription("the wall clock: one tick per mesh run, carrying the minute of the day"),
		component.WithInputs("advance"),
		component.WithOutputs("tick"),
		component.WithInitialState(func(state component.State) {
			state.Set("minute", 0)
		}),
		component.WithActivationFunc(func(_ context.Context, this *component.Component) error {
			minute, err := this.State().GetTyped[int]("minute")
			if err != nil {
				return err
			}
			if err := this.OutputByName("tick").PutPayloads(minute); err != nil {
				return err
			}
			this.State().Set("minute", minute+1)
			return nil
		}),
	)
}
