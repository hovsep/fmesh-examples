// This file holds the "table" component. One constructor makes every table,
// whatever its size: a table announces when it is free, hosts a party for the
// length of its meal and reports the visit when the party leaves.
package main

import (
	"context"
	"fmt"

	"github.com/hovsep/fmesh/component"
)

// Keys of a table's state.
const (
	stateIndex       = "index"        // the table's number
	stateSeats       = "seats"        // how many guests fit
	stateParty       = "party"        // the seated Party, or nil when the table is free
	stateMinutesLeft = "minutes-left" // minutes until the seated party leaves
	stateWaited      = "waited"       // how long the seated party waited for the table
	stateAnnounced   = "announced"    // has the host been told that this table is free?
	stateMinute      = "minute"       // the current minute, remembered from the last tick
)

// newTable creates the table with the given index and number of seats. Its
// component name is "table-<index>"; the seats are also kept as metadata so
// diagrams and reports can read them.
func newTable(index int, seats int) (*component.Component, error) {
	return component.New(tableName(index),
		component.WithDescription(fmt.Sprintf("table for %d", seats)),
		component.WithMeta(metaSeats, float64(seats)),
		component.WithInputs(portSeat, portTick),
		component.WithOutputs(portFree, portServed),
		component.WithInitialState(func(state component.State) {
			state.Set(stateIndex, index)
			state.Set(stateSeats, seats)
			state.Set(stateParty, (*Party)(nil))
			state.Set(stateMinutesLeft, 0)
			state.Set(stateWaited, 0)
			state.Set(stateAnnounced, false)
			state.Set(stateMinute, 0)
		}),
		component.WithActivationFunc(func(_ context.Context, this *component.Component) error {
			if this.InputByName(portTick).HasSignals() {
				if err := tableTick(this); err != nil {
					return err
				}
			}
			if this.InputByName(portSeat).HasSignals() {
				return tableSeat(this)
			}
			return nil
		}),
	)
}

// tableTick is one minute passing at the table.
func tableTick(this *component.Component) error {
	state := this.State()
	minute, err := firstPayload[int](this.InputByName(portTick))
	if err != nil {
		return err
	}
	state.Set(stateMinute, minute)

	party := state.Get(stateParty).(*Party)
	if party == nil {
		// A free table tells the host once; it tells again after every visit.
		if state.Get(stateAnnounced).(bool) {
			return nil
		}
		return announceFree(this)
	}

	minutesLeft := state.Get(stateMinutesLeft).(int) - 1
	state.Set(stateMinutesLeft, minutesLeft)
	if minutesLeft > 0 {
		return nil
	}

	// The meal is over: the party pays and leaves, and the table is free again.
	served := Served{
		Party:         *party,
		Seats:         state.Get(stateSeats).(int),
		WaitedMinutes: state.Get(stateWaited).(int),
		LeftAt:        minute,
	}
	if err := this.OutputByName(portServed).PutPayloads(served); err != nil {
		return err
	}
	state.Set(stateParty, (*Party)(nil))
	return announceFree(this)
}

// tableSeat is the host bringing a party to the table.
func tableSeat(this *component.Component) error {
	state := this.State()
	seating, err := firstPayload[Seating](this.InputByName(portSeat))
	if err != nil {
		return err
	}
	party := seating.Party
	state.Set(stateParty, &party)
	state.Set(stateMinutesLeft, party.MealDuration)
	state.Set(stateWaited, state.Get(stateMinute).(int)-party.ArrivedAt)
	state.Set(stateAnnounced, false)
	return nil
}

// announceFree tells the host that this table can take a party.
func announceFree(this *component.Component) error {
	state := this.State()
	state.Set(stateAnnounced, true)
	announcement := TableFree{Index: state.Get(stateIndex).(int), Seats: state.Get(stateSeats).(int)}
	return this.OutputByName(portFree).PutPayloads(announcement)
}
