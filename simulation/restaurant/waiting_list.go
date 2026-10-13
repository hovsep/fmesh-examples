// This file holds the "waiting-list" component: the queue at the door. It
// remembers who is waiting, shows the list to the host every minute and lets
// impatient parties leave. The queue is a signal group kept in the state.
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
			state.Set("queue", signal.NewGroup())
		}),
		// An activation function can be composed from smaller ones: Sequential
		// runs them in order, and When runs one only if the named input has
		// signals. The queue admits arrivals and forgets seated parties whenever
		// it wakes up, and shows itself to the host only on a tick.
		component.WithActivationFunc(component.Sequential(
			admitArrivals,
			forgetSeatedParties,
			component.When(component.HasSignalsOn("tick"), showTheQueue(patienceMinutes)),
		)),
	)
}

// admitArrivals puts the new parties at the end of the queue. A signal group
// is an ordered set of signals; Join appends another group to it.
func admitArrivals(_ context.Context, this *component.Component) error {
	queue, err := this.State().GetTyped[*signal.Group]("queue")
	if err != nil {
		return err
	}
	this.State().Set("queue", queue.Join(this.InputByName("party").Signals()))
	return nil
}

// forgetSeatedParties takes the parties the host has seated out of the queue.
// Filter keeps the signals that match; here, every party but the seated one,
// matched by its "id" label.
func forgetSeatedParties(_ context.Context, this *component.Component) error {
	queue, err := this.State().GetTyped[*signal.Group]("queue")
	if err != nil {
		return err
	}
	for seated := range this.InputByName("seated").Signals().All() {
		id := seated.Meta().ValueOrDefault("id", 0)
		queue = queue.Filter(signal.Not(signal.MetaEquals("id", id)))
	}
	this.State().Set("queue", queue)
	return nil
}

// showTheQueue runs on a tick: the parties past their patience leave on
// "lost", and the rest go out on "waiting" for the host to choose from.
func showTheQueue(patienceMinutes int) component.ActivationFunc {
	return func(_ context.Context, this *component.Component) error {
		queue, err := this.State().GetTyped[*signal.Group]("queue")
		if err != nil {
			return err
		}
		minute, err := this.InputByName("tick").Signals().FirstAs[int]()
		if err != nil {
			return err
		}
		lost := signal.NewGroup()
		staying := signal.NewGroup()
		for party := range queue.All() {
			if minute-party.Meta().ValueOrDefault("arrived-at", 0) > patienceMinutes {
				lost = lost.With(party)
			} else {
				staying = staying.With(party)
			}
		}
		this.State().Set("queue", staying)
		if err := this.OutputByName("lost").PutSignalGroups(lost); err != nil {
			return err
		}
		return this.OutputByName("waiting").PutSignalGroups(staying)
	}
}
