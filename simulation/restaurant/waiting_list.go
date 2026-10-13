// This file holds the "waiting-list" component: the queue at the door. It
// remembers who is waiting, shows the list to the host every minute and lets
// impatient parties leave. The queue is a signal group.
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
			waiting, err := this.State().GetTyped[*signal.Group]("waiting")
			if err != nil {
				return err
			}

			// New arrivals join the end of the queue.
			waiting = waiting.Join(this.InputByName("party").Signals())

			// Parties the host has seated leave the queue (matched by id).
			for _, seated := range this.InputByName("seated").Signals().All() {
				id := seated.Meta().ValueOrDefault("id", 0.0)
				waiting = waiting.Filter(signal.Not(signal.MetaEquals("id", id)))
			}

			// On a tick: impatient parties leave, then the host gets the current list.
			if this.InputByName("tick").HasSignals() {
				minute, err := this.InputByName("tick").Signals().FirstAs[int]()
				if err != nil {
					return err
				}
				lost := signal.NewGroup()
				staying := signal.NewGroup()
				for _, party := range waiting.All() {
					arrived := int(party.Meta().ValueOrDefault("arrived", 0.0))
					if minute-arrived > patienceMinutes {
						lost = lost.With(party)
					} else {
						staying = staying.With(party)
					}
				}
				waiting = staying
				if err := this.OutputByName("lost").PutSignalGroups(lost); err != nil {
					return err
				}
				if err := this.OutputByName("waiting").PutSignalGroups(waiting); err != nil {
					return err
				}
			}

			this.State().Set("waiting", waiting)
			return nil
		}),
	)
}
