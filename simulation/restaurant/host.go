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
	"github.com/hovsep/fmesh/signal"
)

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
// tables are free from their announcements (table index -> seats) and, whenever
// it sees the waiting list, seats as many parties as the policy allows. The
// policy name is kept as metadata so the program can print which host ran the day.
//
// A policy gets the waiting parties in arrival order and the free tables, and
// returns who sits where: table index -> party. It must not use a table twice.
func newHost(policy string, description string, numberOfTables int,
	choose func(waiting []*signal.Signal, free map[int]int) map[int]*signal.Signal,
) (*component.Component, error) {
	return component.New("host",
		component.WithDescription("the host "+description),
		component.WithMeta("policy", policy),
		component.WithInputs("waiting", "free", "tick"),
		component.WithOutputs("seated"),
		component.WithIndexedOutputs("table", 0, numberOfTables-1),
		component.WithInitialState(func(state component.State) {
			state.Set("free", map[int]int{})
		}),
		component.WithActivationFunc(func(_ context.Context, this *component.Component) error {
			free, err := this.State().GetTyped[map[int]int]("free")
			if err != nil {
				return err
			}

			// Tables announce themselves when they become free: the payload is
			// the table index, the "seats" label says how big it is.
			for announcement := range this.InputByName("free").Signals().All() {
				index, err := announcement.As[int]()
				if err != nil {
					return err
				}
				free[index] = announcement.Meta().ValueOrDefault("seats", 0)
			}

			// The tick itself carries nothing the host needs: the waiting list
			// arrives a moment later in the same minute and triggers the seating.
			if !this.InputByName("waiting").HasSignals() {
				return nil
			}
			waiting := slices.Collect(this.InputByName("waiting").Signals().All())
			for index, party := range choose(waiting, free) {
				delete(free, index)
				// Indexed outputs are named "<prefix><index>", so table 3 is "table3".
				if err := this.OutputByName(fmt.Sprintf("table%d", index)).PutSignals(party); err != nil {
					return err
				}
				if err := this.OutputByName("seated").PutSignals(party); err != nil {
					return err
				}
			}
			return nil
		}),
	)
}

// chooseFirstFit is the FIFO policy: walk the parties in arrival order and give
// each one the lowest-numbered free table where it fits.
func chooseFirstFit(waiting []*signal.Signal, free map[int]int) map[int]*signal.Signal {
	seatings := map[int]*signal.Signal{}
	for _, party := range waiting {
		for _, index := range slices.Sorted(maps.Keys(free)) {
			if _, taken := seatings[index]; taken {
				continue
			}
			if party.PayloadOrDefault(0) <= free[index] {
				seatings[index] = party
				break
			}
		}
	}
	return seatings
}

// chooseBestFit is the best-fit policy: walk the free tables from small to
// large and give each one the waiting party that leaves the fewest empty
// seats. Among equally good parties the one that arrived first wins.
func chooseBestFit(waiting []*signal.Signal, free map[int]int) map[int]*signal.Signal {
	// Smallest table first; tables of the same size in index order, so that the
	// result does not depend on Go's random map order.
	bySize := slices.SortedFunc(maps.Keys(free), func(a, b int) int {
		if free[a] != free[b] {
			return free[a] - free[b]
		}
		return a - b
	})

	seatings := map[int]*signal.Signal{}
	remaining := slices.Clone(waiting)
	for _, index := range bySize {
		best := -1
		for i, party := range remaining {
			size := party.PayloadOrDefault(0)
			if size <= free[index] && (best < 0 || size > remaining[best].PayloadOrDefault(0)) {
				best = i
			}
		}
		if best < 0 {
			continue
		}
		seatings[index] = remaining[best]
		remaining = slices.Delete(remaining, best, best+1)
	}
	return seatings
}
