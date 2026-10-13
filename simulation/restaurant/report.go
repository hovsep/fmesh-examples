// This file holds the "report" component, the bookkeeper of the day: it adds
// up every served and every lost party and keeps the figures a restaurant
// owner asks for in its state, where the program reads them after the day.
package main

import (
	"context"

	"github.com/hovsep/fmesh/component"
)

// newReport creates the bookkeeper for a restaurant with that many tables.
//
// State, all counts since the day began:
//
//	minutes             minutes the restaurant has been open
//	parties-served      parties that ate and paid, and guests-served
//	parties-lost        parties that gave up waiting, and guests-lost
//	wait-minutes        waiting minutes summed over served parties
//	busy-table-minutes  table-minutes with a party at the table
//
// and the figures derived from them: average-wait (minutes per served party),
// utilisation (percent of table-minutes in use), revenue and lost-revenue (euros).
func newReport(numberOfTables int, revenuePerGuest int) (*component.Component, error) {
	return component.New("report",
		component.WithDescription("the bookkeeper: counts served and lost parties, waiting time and revenue"),
		component.WithInputs("served", "lost", "tick"),
		component.WithInitialState(func(state component.State) {
			state.Set("minutes", 0)
			state.Set("parties-served", 0)
			state.Set("guests-served", 0)
			state.Set("parties-lost", 0)
			state.Set("guests-lost", 0)
			state.Set("wait-minutes", 0)
			state.Set("busy-table-minutes", 0)
			state.Set("average-wait", 0.0)
			state.Set("utilisation", 0.0)
			state.Set("revenue", 0)
			state.Set("lost-revenue", 0)
		}),
		component.WithActivationFunc(func(_ context.Context, this *component.Component) error {
			state := this.State()
			add := func(key string, n int) { state.Set(key, state.Get(key).(int)+n) }

			add("minutes", this.InputByName("tick").Signals().Len())

			// A served party carries its whole story in its labels.
			for _, party := range this.InputByName("served").Signals().All() {
				guests, err := party.As[int]()
				if err != nil {
					return err
				}
				labels := party.Meta()
				add("parties-served", 1)
				add("guests-served", guests)
				add("wait-minutes", int(labels.ValueOrDefault("seated", 0.0)-labels.ValueOrDefault("arrived", 0.0)))
				add("busy-table-minutes", int(labels.ValueOrDefault("meal", 0.0)))
			}

			for _, party := range this.InputByName("lost").Signals().All() {
				guests, err := party.As[int]()
				if err != nil {
					return err
				}
				add("parties-lost", 1)
				add("guests-lost", guests)
			}

			// The figures the owner asks for, kept up to date.
			if served := state.Get("parties-served").(int); served > 0 {
				state.Set("average-wait", float64(state.Get("wait-minutes").(int))/float64(served))
			}
			if minutes := state.Get("minutes").(int); minutes > 0 {
				state.Set("utilisation", 100*float64(state.Get("busy-table-minutes").(int))/float64(numberOfTables*minutes))
			}
			state.Set("revenue", state.Get("guests-served").(int)*revenuePerGuest)
			state.Set("lost-revenue", state.Get("guests-lost").(int)*revenuePerGuest)
			return nil
		}),
	)
}
