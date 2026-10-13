// This file holds the "table" component. One constructor makes every table,
// whatever its size: a table announces when it is free, hosts a party for the
// length of its meal and hands the party to the report when it leaves.
package main

import (
	"context"
	"fmt"

	"github.com/hovsep/fmesh/component"
	"github.com/hovsep/fmesh/signal"
)

// newTable creates the table with the given index and number of seats. Its
// component name is "table-<index>"; the seats are also kept as metadata so
// diagrams can show them.
//
// The table has no "party" key in its state while it is free. The current
// minute is remembered from the last tick, because the host seats a party a
// moment after the tick, in a later round of the same run.
func newTable(index int, seats int) (*component.Component, error) {
	onTick := component.HasSignalsOn("tick")
	return component.New(fmt.Sprintf("table-%d", index),
		component.WithDescription(fmt.Sprintf("table for %d", seats)),
		component.WithMeta("seats", seats),
		component.WithInputs("seat", "tick"),
		component.WithOutputs("free", "served"),
		component.WithInitialState(func(state component.State) {
			state.Set("minute", 0)
		}),
		// Two kinds of input reach a table: the tick, and the host bringing a
		// party. Each step answers one of them.
		component.WithActivationFunc(component.Sequential(
			component.When(onTick, rememberTheMinute),
			component.When(onTick, openTheDay(index, seats)),
			component.When(onTick, passAMinuteOfTheMeal(index, seats)),
			component.When(component.HasSignalsOn("seat"), seatTheParty),
		)),
	)
}

// rememberTheMinute keeps the minute of the tick for the seating that may
// follow it.
func rememberTheMinute(_ context.Context, this *component.Component) error {
	minute, err := this.InputByName("tick").Signals().FirstAs[int]()
	if err != nil {
		return err
	}
	this.State().Set("minute", minute)
	return nil
}

// openTheDay tells the host once, on the first tick, that this table is free.
func openTheDay(index int, seats int) component.ActivationFunc {
	return func(_ context.Context, this *component.Component) error {
		if this.State().Get("minute") != 0 {
			return nil
		}
		return announceFree(this, index, seats)
	}
}

// passAMinuteOfTheMeal counts the seated party's meal down by one minute. When
// the meal is over the party pays and leaves on "served", and the table
// announces itself free again.
func passAMinuteOfTheMeal(index int, seats int) component.ActivationFunc {
	return func(_ context.Context, this *component.Component) error {
		state := this.State()
		if !state.Has("party") {
			return nil
		}
		minutesLeft, err := state.GetTyped[int]("minutes-left")
		if err != nil {
			return err
		}
		minutesLeft--
		state.Set("minutes-left", minutesLeft)
		if minutesLeft > 0 {
			return nil
		}
		party, err := state.GetTyped[*signal.Signal]("party")
		if err != nil {
			return err
		}
		if err := this.OutputByName("served").PutSignals(party); err != nil {
			return err
		}
		state.Delete("party")
		return announceFree(this, index, seats)
	}
}

// seatTheParty takes the party the host brings: stamp the minute on it and
// start its meal.
func seatTheParty(_ context.Context, this *component.Component) error {
	minute, err := this.State().GetTyped[int]("minute")
	if err != nil {
		return err
	}
	party := this.InputByName("seat").Signals().First().WithMeta("seated-at", minute)
	this.State().Set("party", party)
	this.State().Set("minutes-left", party.Meta().ValueOrDefault("meal-minutes", 0))
	return nil
}

// announceFree tells the host that this table is free. The announcement is
// the table index, labelled with its size.
func announceFree(this *component.Component, index int, seats int) error {
	return this.OutputByName("free").PutSignals(signal.New(index).WithMeta("seats", seats))
}
