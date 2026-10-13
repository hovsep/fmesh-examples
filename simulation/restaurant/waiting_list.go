// This file holds the "waiting-list" component: the queue at the door. It
// remembers who is waiting, shows the list to the host every minute and lets
// impatient parties leave. The queue is a signal group, so joining, leaving
// and giving up are group operations.
package main

import (
	"context"

	"github.com/hovsep/fmesh/component"
	"github.com/hovsep/fmesh/signal"
)

// newWaitingList creates the queue. A party that has waited longer than
// patienceMinutes gives up and is reported as lost.
func newWaitingList(patienceMinutes int) (*component.Component, error) {
	return component.New("waiting-list",
		component.WithDescription("the queue at the door: parties wait for a table or give up"),
		component.WithInputs("party", "seated", "tick"),
		component.WithOutputs("waiting", "lost"),
		component.WithInitialState(func(state component.State) {
			state.Set("waiting", signal.NewGroup())
		}),
		component.WithActivationFunc(func(_ context.Context, this *component.Component) error {
			waiting := this.State().Get("waiting").(*signal.Group)
			defer func() { this.State().Set("waiting", waiting) }()

			// New arrivals join the end of the queue.
			waiting = waiting.Join(this.InputByName("party").Signals())

			// Parties the host has seated leave the queue (matched by id).
			seated := this.InputByName("seated").Signals()
			waiting = waiting.Filter(func(party *signal.Signal) bool {
				id := party.Meta().ValueOrDefault("id", 0.0)
				return seated.Filter(signal.MetaEquals("id", id)).IsEmpty()
			})

			// On a tick: impatient parties leave, then the host gets the current list.
			if !this.InputByName("tick").HasSignals() {
				return nil
			}
			minute, err := this.InputByName("tick").Signals().FirstAs[int]()
			if err != nil {
				return err
			}
			impatient := func(party *signal.Signal) bool {
				arrived := int(party.Meta().ValueOrDefault("arrived", 0.0))
				return minute-arrived > patienceMinutes
			}
			if err := this.OutputByName("lost").PutSignalGroups(waiting.Filter(impatient)); err != nil {
				return err
			}
			waiting = waiting.Filter(signal.Not(impatient))
			return this.OutputByName("waiting").PutSignalGroups(waiting)
		}),
	)
}
