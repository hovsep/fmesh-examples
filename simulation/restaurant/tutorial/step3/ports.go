// This file holds the names of components and ports. The wiring, the
// components and the tests all share them, so a typo cannot slip through.
package main

import (
	"fmt"
)

// Component names.
const (
	arrivalsName    = "arrivals"
	waitingListName = "waiting-list"
	hostName        = "host"
	reportName      = "report"
	tableNamePrefix = "table-"
)

// Port names. The same name is used on both ends of a pipe where it makes
// sense (the arrivals' "party" output feeds the waiting list's "party" input).
const (
	portTick    = "tick"    // the current minute, fed by the run loop
	portParty   = "party"   // a Party that has just arrived
	portWaiting = "waiting" // the whole waiting list, one Party per signal
	portLost    = "lost"    // a Party that gave up waiting
	portSeated  = "seated"  // a Party that got a table
	portFree    = "free"    // a TableFree announcement
	portSeat    = "seat"    // a Seating: "this party sits here"
	portServed  = "served"  // a Served record: the party has eaten and left

	// The host has one output per table: table0, table1, ... (indexed ports).
	tablePortPrefix = "table"
)

// Metadata keys.
const metaSeats = "seats"

// tableName is the component name of the table with the given index, e.g. "table-3".
func tableName(index int) string {
	return fmt.Sprintf("%s%d", tableNamePrefix, index)
}

// tablePortName is the host's output port that leads to the table with the
// given index, e.g. "table3". F-Mesh names indexed ports "<prefix><index>".
func tablePortName(index int) string {
	return fmt.Sprintf("%s%d", tablePortPrefix, index)
}
