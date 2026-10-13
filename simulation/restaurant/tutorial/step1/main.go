// Step 1 of the restaurant tutorial: one component.
//
// What this step adds: a single "table" component with two inputs (seat, tick)
// and two outputs (free, served), and a little state. We activate it by hand,
// minute by minute, and print what comes out.
//
// What to look at: the activation function at the heart of newTable. It reads
// its inputs, updates its state and writes to its outputs. That is all a
// component ever does. Also note that the table knows nothing about hosts or
// guests: it only speaks through its ports.
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/hovsep/fmesh"
	"github.com/hovsep/fmesh/component"
	"github.com/hovsep/fmesh/port"
	"github.com/hovsep/fmesh/signal"
)

// Port names.
const (
	portTick   = "tick"   // the current minute
	portSeat   = "seat"   // a Seating: "this party sits here"
	portFree   = "free"   // a TableFree announcement
	portServed = "served" // a Served record: the party has eaten and left
)

// Keys of a table's state.
const (
	stateIndex       = "index"
	stateSeats       = "seats"
	stateParty       = "party"        // the seated Party, or nil when the table is free
	stateMinutesLeft = "minutes-left" // minutes until the seated party leaves
	stateWaited      = "waited"       // how long the seated party waited for the table
	stateAnnounced   = "announced"    // has the host been told that this table is free?
	stateMinute      = "minute"       // the current minute, remembered from the last tick
)

// Party is a group of guests that walks in together.
type Party struct {
	ID           int
	Size         int
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
	Seats         int
	WaitedMinutes int
	LeftAt        int
}

func main() {
	fmt.Println("=== Restaurant tutorial, step 1: one table ===")

	table, err := newTable(0, 4)
	if err != nil {
		fmt.Println("Failed to build the table:", err)
		os.Exit(1)
	}

	// A component only runs inside a mesh, so the table gets a mesh of its own.
	fm, err := fmesh.New("one-table")
	if err != nil {
		fmt.Println("Failed to build mesh:", err)
		os.Exit(1)
	}
	if err := fm.AddComponents(table); err != nil {
		fmt.Println("Failed to add the table:", err)
		os.Exit(1)
	}

	party := Party{ID: 1, Size: 3, ArrivedAt: 0, MealDuration: 3}
	for minute := range 7 {
		// Every minute the table hears the clock. In minute 2 the host also seats a party.
		if err := table.InputByName(portTick).PutPayloads(minute); err != nil {
			fmt.Println("Failed to put the tick:", err)
			os.Exit(1)
		}
		if minute == 2 {
			if err := table.InputByName(portSeat).PutPayloads(Seating{Party: party, TableIndex: 0}); err != nil {
				fmt.Println("Failed to seat the party:", err)
				os.Exit(1)
			}
		}
		if _, err := fm.Run(context.Background()); err != nil {
			fmt.Println("Run failed:", err)
			os.Exit(1)
		}
		printOutputs(table, minute)
	}
}

// printOutputs shows what the table said this minute. Nothing is piped to the
// outputs, so the signals are still there after the run.
func printOutputs(table *component.Component, minute int) {
	fmt.Printf("minute %d:", minute)
	_ = table.Outputs().ForEach(func(out *port.Port) error {
		return out.Signals().ForEach(func(sig *signal.Signal) error {
			fmt.Printf("  %s -> %+v", out.Name(), sig.Payload())
			return nil
		})
	})
	fmt.Println()
}

// newTable creates the table with the given index and number of seats.
func newTable(index int, seats int) (*component.Component, error) {
	return component.New(fmt.Sprintf("table-%d", index),
		component.WithDescription(fmt.Sprintf("table for %d", seats)),
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
	minute, err := this.InputByName(portTick).Signals().FirstAs[int]()
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
	seating, err := this.InputByName(portSeat).Signals().FirstAs[Seating]()
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
