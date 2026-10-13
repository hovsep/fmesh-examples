// This file holds the things that travel through the mesh as signal payloads:
// parties, table announcements, seatings and service records. It also has two
// small helpers that read typed payloads off a port.
package main

import (
	"fmt"

	"github.com/hovsep/fmesh/port"
	"github.com/hovsep/fmesh/signal"
)

// Party is a group of guests that walks in together.
type Party struct {
	ID           int // running number, assigned on arrival
	Size         int // number of guests, 1..6
	ArrivedAt    int // minute of the day when the party arrived
	MealDuration int // how many minutes the party stays once seated
}

// TableFree is what a table announces when it is ready for the next party.
type TableFree struct {
	Index int
	Seats int
}

// Seating is the host's decision: this party sits at this table.
type Seating struct {
	Party      Party
	TableIndex int
}

// Served is written by a table when a party has eaten and left.
type Served struct {
	Party         Party
	Seats         int // size of the table the party sat at
	WaitedMinutes int // minutes between arrival and sitting down
	LeftAt        int // minute of the day when the party left
}

// fits says whether the party can sit at a table with that many seats.
func (p Party) fits(seats int) bool {
	return p.Size <= seats
}

// payloads returns every payload on the port as T, in arrival order.
// It fails loudly on a payload of another type instead of guessing.
func payloads[T any](p *port.Port) ([]T, error) {
	items := make([]T, 0, p.Signals().Len())
	err := p.Signals().ForEach(func(sig *signal.Signal) error {
		item, err := sig.As[T]()
		if err != nil {
			return fmt.Errorf("port %q: %w", p.Name(), err)
		}
		items = append(items, item)
		return nil
	})
	return items, err
}

// firstPayload returns the first payload on the port as T.
func firstPayload[T any](p *port.Port) (T, error) {
	item, err := p.Signals().FirstAs[T]()
	if err != nil {
		return item, fmt.Errorf("port %q: %w", p.Name(), err)
	}
	return item, nil
}
