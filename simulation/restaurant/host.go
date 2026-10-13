// This file holds the "host" component: the person who decides which waiting
// party gets which free table. Two hosts exist, with the same ports and the
// same code; only the seating policy differs.
package main

import (
	"context"
	"fmt"
	"maps"
	"slices"

	"github.com/hovsep/fmesh/component"
)

// Key of the host's state: the free tables, table index -> number of seats.
const stateFreeTables = "free-tables"

// tablePort is the host's output port that leads to the table with the given
// index, e.g. "table3": F-Mesh names indexed ports "<prefix><index>".
func tablePort(index int) string {
	return fmt.Sprintf("table%d", index)
}

// freeTables maps the index of every free table to its number of seats.
type freeTables map[int]int

// seatingPolicy picks seatings for the waiting parties (in arrival order) given
// the free tables. It must not use a table twice.
type seatingPolicy func(waiting []Party, free freeTables) []Seating

// newFIFOHost seats parties first come, first served: each party takes the
// first free table where it fits, in table order. A couple can take the 6-top.
func newFIFOHost(numberOfTables int) (*component.Component, error) {
	return newHost("fifo", "seats parties first come, first served at the first table where they fit", numberOfTables, chooseFirstFit)
}

// newBestFitHost gives every free table to the waiting party that wastes the
// fewest seats, so big tables are kept for big parties.
func newBestFitHost(numberOfTables int) (*component.Component, error) {
	return newHost("best-fit", "seats the party that fits each free table best, keeping big tables for big parties", numberOfTables, chooseBestFit)
}

// newHost creates a host with the given seating policy. The host learns which
// tables are free from their announcements and, whenever it sees the waiting
// list, seats as many parties as the policy allows. The policy name is kept
// as metadata so the program can print which host ran the day.
func newHost(policy string, description string, numberOfTables int, choose seatingPolicy) (*component.Component, error) {
	return component.New("host",
		component.WithDescription("the host "+description),
		component.WithMeta("policy", policy),
		component.WithInputs("waiting", "free", "tick"),
		component.WithOutputs("seated"),
		component.WithIndexedOutputs("table", 0, numberOfTables-1),
		component.WithInitialState(func(state component.State) {
			state.Set(stateFreeTables, freeTables{})
		}),
		component.WithActivationFunc(func(_ context.Context, this *component.Component) error {
			free := this.State().Get(stateFreeTables).(freeTables)

			// Tables announce themselves when they become free.
			announcements, err := payloads[TableFree](this.InputByName("free"))
			if err != nil {
				return err
			}
			for _, announcement := range announcements {
				free[announcement.Index] = announcement.Seats
			}

			// The tick itself carries nothing the host needs: the waiting list
			// arrives a moment later in the same minute and triggers the seating.
			if !this.InputByName("waiting").HasSignals() {
				return nil
			}
			waiting, err := payloads[Party](this.InputByName("waiting"))
			if err != nil {
				return err
			}
			for _, seating := range choose(waiting, free) {
				delete(free, seating.TableIndex)
				if err := this.OutputByName(tablePort(seating.TableIndex)).PutPayloads(seating); err != nil {
					return err
				}
				if err := this.OutputByName("seated").PutPayloads(seating.Party); err != nil {
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

// chooseBestFit is the best-fit policy: walk the free tables from small to
// large and give each one the waiting party that leaves the fewest empty
// seats. Among equally good parties the one that arrived first wins.
func chooseBestFit(waiting []Party, free freeTables) []Seating {
	remaining := slices.Clone(waiting)
	var seatings []Seating
	for _, index := range tablesBySize(free) {
		seats := free[index]
		best := bestFittingParty(remaining, seats)
		if best < 0 {
			continue
		}
		seatings = append(seatings, Seating{Party: remaining[best], TableIndex: index})
		remaining = slices.Delete(remaining, best, best+1)
	}
	return seatings
}

// bestFittingParty returns the position of the party that wastes the fewest
// seats at a table of that size, or -1 when nobody fits.
func bestFittingParty(waiting []Party, seats int) int {
	best := -1
	for i, party := range waiting {
		if !party.fits(seats) {
			continue
		}
		if best < 0 || party.Size > waiting[best].Size {
			best = i
		}
	}
	return best
}

// tablesBySize lists the free table indexes from the smallest table to the
// largest; tables of the same size keep their index order. A fixed order keeps
// the simulation deterministic (Go maps iterate in random order).
func tablesBySize(free freeTables) []int {
	indexes := make([]int, 0, len(free))
	for index := range free {
		indexes = append(indexes, index)
	}
	slices.SortFunc(indexes, func(a, b int) int {
		if free[a] != free[b] {
			return free[a] - free[b]
		}
		return a - b
	})
	return indexes
}

// tablesByIndex lists the free table indexes in numeric order.
func tablesByIndex(free freeTables) []int {
	return slices.Sorted(maps.Keys(free))
}
