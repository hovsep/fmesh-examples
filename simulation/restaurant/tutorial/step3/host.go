// This file holds the "host" component: the person who decides which waiting
// party gets which free table. Two hosts exist, with the same ports and the
// same code; only the seating policy differs.
package main

import (
	"context"
	"maps"
	"slices"

	"github.com/hovsep/fmesh/component"
)

// Key of the host's state: the free tables, table index -> number of seats.
const stateFreeTables = "free-tables"

// freeTables maps the index of every free table to its number of seats.
type freeTables map[int]int

// seatingPolicy picks seatings for the waiting parties (in arrival order) given
// the free tables. It must not use a table twice.
type seatingPolicy func(waiting []Party, free freeTables) []Seating

// newFIFOHost seats parties first come, first served: each party takes the
// first free table where it fits, in table order. A couple can take the 6-top.
func newFIFOHost(numberOfTables int) (*component.Component, error) {
	return newHost("seats parties first come, first served at the first table where they fit", numberOfTables, chooseFirstFit)
}

// newHost creates a host with the given seating policy. The host learns which
// tables are free from their announcements and, whenever it sees the waiting
// list, seats as many parties as the policy allows.
func newHost(description string, numberOfTables int, choose seatingPolicy) (*component.Component, error) {
	return component.New(hostName,
		component.WithDescription("the host "+description),
		component.WithInputs(portWaiting, portFree, portTick),
		component.WithOutputs(portSeated),
		component.WithIndexedOutputs(tablePortPrefix, 0, numberOfTables-1),
		component.WithInitialState(func(state component.State) {
			state.Set(stateFreeTables, freeTables{})
		}),
		component.WithActivationFunc(func(_ context.Context, this *component.Component) error {
			free := this.State().Get(stateFreeTables).(freeTables)

			// Tables announce themselves when they become free.
			announcements, err := payloads[TableFree](this.InputByName(portFree))
			if err != nil {
				return err
			}
			for _, announcement := range announcements {
				free[announcement.Index] = announcement.Seats
			}

			// The tick itself carries nothing the host needs: the waiting list
			// arrives a moment later in the same minute and triggers the seating.
			if !this.InputByName(portWaiting).HasSignals() {
				return nil
			}
			waiting, err := payloads[Party](this.InputByName(portWaiting))
			if err != nil {
				return err
			}
			for _, seating := range choose(waiting, free) {
				delete(free, seating.TableIndex)
				if err := this.OutputByName(tablePortName(seating.TableIndex)).PutPayloads(seating); err != nil {
					return err
				}
				if err := this.OutputByName(portSeated).PutPayloads(seating.Party); err != nil {
					return err
				}
			}
			return nil
		}),
	)
}

// chooseFirstFit is the FIFO policy: walk the parties in arrival order and give
// each one the lowest-numbered free table where it fits.
func chooseFirstFit(waiting []Party, free freeTables) []Seating {
	remaining := maps.Clone(free)
	var seatings []Seating
	for _, party := range waiting {
		for _, index := range tablesByIndex(remaining) {
			if party.fits(remaining[index]) {
				seatings = append(seatings, Seating{Party: party, TableIndex: index})
				delete(remaining, index)
				break
			}
		}
	}
	return seatings
}

// tablesByIndex lists the free table indexes in numeric order.
func tablesByIndex(free freeTables) []int {
	return slices.Sorted(maps.Keys(free))
}
