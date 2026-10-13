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
// State: the seated party (no "party" key while the table is free), the
// minutes left of its meal, and the current minute, remembered from the last
// tick because the party is seated a moment after the tick.
func newTable(index int, seats int) (*component.Component, error) {
	return component.New(fmt.Sprintf("table-%d", index),
		component.WithDescription(fmt.Sprintf("table for %d", seats)),
		component.WithMeta("seats", float64(seats)),
		component.WithInputs("seat", "tick"),
		component.WithOutputs("free", "served"),
		component.WithInitialState(func(state component.State) {
			state.Set("minute", 0)
		}),
		component.WithActivationFunc(func(_ context.Context, this *component.Component) error {
			state := this.State()

			// One minute passes at the table.
			if this.InputByName("tick").HasSignals() {
				minute, err := this.InputByName("tick").Signals().FirstAs[int]()
				if err != nil {
					return err
				}
				state.Set("minute", minute)

				if !state.Has("party") {
					// The day starts: tell the host once that this table is free.
					// The announcement is the table index, labelled with its size.
					if minute == 0 {
						announcement := signal.New(index).WithMeta("seats", float64(seats))
						if err := this.OutputByName("free").PutSignals(announcement); err != nil {
							return err
						}
					}
				} else {
					minutesLeft, err := state.GetTyped[int]("minutes-left")
					if err != nil {
						return err
					}
					minutesLeft--
					state.Set("minutes-left", minutesLeft)
					if minutesLeft == 0 {
						// The meal is over: the party pays and leaves, and the table is free again.
						party, err := state.GetTyped[*signal.Signal]("party")
						if err != nil {
							return err
						}
						if err := this.OutputByName("served").PutSignals(party); err != nil {
							return err
						}
						state.Delete("party")
						announcement := signal.New(index).WithMeta("seats", float64(seats))
						if err := this.OutputByName("free").PutSignals(announcement); err != nil {
							return err
						}
					}
				}
			}

			// The host brings a party: stamp the minute on it and start the meal.
			if this.InputByName("seat").HasSignals() {
				minute, err := state.GetTyped[int]("minute")
				if err != nil {
					return err
				}
				party := this.InputByName("seat").Signals().First().WithMeta("seated", float64(minute))
				state.Set("party", party)
				state.Set("minutes-left", int(party.Meta().ValueOrDefault("meal", 0.0)))
			}
			return nil
		}),
	)
}
