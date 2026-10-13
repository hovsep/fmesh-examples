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
//	minutes-open        minutes the restaurant has been open
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
		// No outputs: the state is the result, read by the program after the day.
		component.WithInitialState(func(state component.State) {
			state.Set("minutes-open", 0)
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
		component.WithActivationFunc(component.Sequential(
			countTheMinutes,
			countServedParties,
			countLostParties,
			updateTheFigures(numberOfTables, revenuePerGuest),
		)),
	)
}

// countTheMinutes adds the ticks to the minutes open.
func countTheMinutes(_ context.Context, this *component.Component) error {
	state := this.State()
	state.Set("minutes-open", state.Get("minutes-open").(int)+this.InputByName("tick").Signals().Len())
	return nil
}

// countServedParties books every party that ate and paid. A served party
// carries its whole story in its labels.
func countServedParties(_ context.Context, this *component.Component) error {
	state := this.State()
	partiesServed := state.Get("parties-served").(int)
	guestsServed := state.Get("guests-served").(int)
	waitMinutes := state.Get("wait-minutes").(int)
	busyTableMinutes := state.Get("busy-table-minutes").(int)

	for party := range this.InputByName("served").Signals().All() {
		guests, err := party.As[int]()
		if err != nil {
			return err
		}
		labels := party.Meta()
		partiesServed++
		guestsServed += guests
		waitMinutes += labels.ValueOrDefault("seated-at", 0) - labels.ValueOrDefault("arrived-at", 0)
		busyTableMinutes += labels.ValueOrDefault("meal-minutes", 0)
	}

	state.Set("parties-served", partiesServed)
	state.Set("guests-served", guestsServed)
	state.Set("wait-minutes", waitMinutes)
	state.Set("busy-table-minutes", busyTableMinutes)
	return nil
}

// countLostParties books every party that gave up waiting.
func countLostParties(_ context.Context, this *component.Component) error {
	state := this.State()
	partiesLost := state.Get("parties-lost").(int)
	guestsLost := state.Get("guests-lost").(int)

	for party := range this.InputByName("lost").Signals().All() {
		guests, err := party.As[int]()
		if err != nil {
			return err
		}
		partiesLost++
		guestsLost += guests
	}

	state.Set("parties-lost", partiesLost)
	state.Set("guests-lost", guestsLost)
	return nil
}

// updateTheFigures keeps the figures the owner asks for up to date.
func updateTheFigures(numberOfTables int, revenuePerGuest int) component.ActivationFunc {
	return func(_ context.Context, this *component.Component) error {
		state := this.State()
		minutes := state.Get("minutes-open").(int)
		partiesServed := state.Get("parties-served").(int)
		if partiesServed > 0 {
			state.Set("average-wait", float64(state.Get("wait-minutes").(int))/float64(partiesServed))
		}
		if minutes > 0 {
			state.Set("utilisation", 100*float64(state.Get("busy-table-minutes").(int))/float64(numberOfTables*minutes))
		}
		state.Set("revenue", state.Get("guests-served").(int)*revenuePerGuest)
		state.Set("lost-revenue", state.Get("guests-lost").(int)*revenuePerGuest)
		return nil
	}
}
