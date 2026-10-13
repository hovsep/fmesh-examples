// This file holds the configuration of a restaurant and the wiring that turns
// the components into one mesh.
package main

import (
	"fmt"

	"github.com/hovsep/fmesh"
	"github.com/hovsep/fmesh/component"
	"github.com/hovsep/fmesh/port"
)

// A small restaurant: a 2-top, a 4-top and a 6-top.
var tutorialTables = []int{2, 4, 6}

// Settings of the day.
const (
	patienceMinutes = 20      // a party gives up after waiting this long
	openingMinutes  = 12 * 60 // open from 11:00 to 23:00
	revenuePerGuest = 25      // euros per served guest
	seed            = 1       // any fixed seed makes the day reproducible
)

// newRestaurant builds the mesh: every component plus the pipes between them.
//
//	arrivals.party        -> waiting-list.party
//	waiting-list.waiting  -> host.waiting
//	waiting-list.lost     -> report.lost
//	host.seated           -> waiting-list.seated
//	host.table<i>         -> table-<i>.seat        (for every table)
//	table-<i>.free        -> host.free
//	table-<i>.served      -> report.served
func newRestaurant(seats []int) (*fmesh.FMesh, error) {
	arrivals, err := newArrivals(seed)
	if err != nil {
		return nil, err
	}
	waitingList, err := newWaitingList(patienceMinutes)
	if err != nil {
		return nil, err
	}
	host, err := newFIFOHost(len(seats))
	if err != nil {
		return nil, err
	}
	report, err := newReport(len(seats), revenuePerGuest)
	if err != nil {
		return nil, err
	}
	tables, err := newTables(seats)
	if err != nil {
		return nil, err
	}

	fm, err := fmesh.New("restaurant-step3",
		fmesh.WithDescription("a small restaurant run for a whole day"),
		fmesh.WithErrorHandlingStrategy(fmesh.StopOnFirstErrorOrPanic),
	)
	if err != nil {
		return nil, err
	}
	if err := fm.AddComponents(arrivals, waitingList, host, report); err != nil {
		return nil, err
	}
	if err := fm.AddComponents(tables...); err != nil {
		return nil, err
	}

	err = port.MultiPipe(
		port.Pipe{From: arrivals.OutputByName(portParty), To: waitingList.InputByName(portParty)},
		port.Pipe{From: waitingList.OutputByName(portWaiting), To: host.InputByName(portWaiting)},
		port.Pipe{From: waitingList.OutputByName(portLost), To: report.InputByName(portLost)},
		port.Pipe{From: host.OutputByName(portSeated), To: waitingList.InputByName(portSeated)},
	)
	if err != nil {
		return nil, err
	}
	for i, table := range tables {
		err = port.MultiPipe(
			port.Pipe{From: host.OutputByName(tablePortName(i)), To: table.InputByName(portSeat)},
			port.Pipe{From: table.OutputByName(portFree), To: host.InputByName(portFree)},
			port.Pipe{From: table.OutputByName(portServed), To: report.InputByName(portServed)},
		)
		if err != nil {
			return nil, err
		}
	}
	return fm, nil
}

// newTables creates one table component per entry of seats.
func newTables(seats []int) ([]*component.Component, error) {
	tables := make([]*component.Component, len(seats))
	for i, s := range seats {
		table, err := newTable(i, s)
		if err != nil {
			return nil, fmt.Errorf("table %d: %w", i, err)
		}
		tables[i] = table
	}
	return tables, nil
}

// seedTick puts the current minute on the "tick" input of every component that has one.
func seedTick(fm *fmesh.FMesh, minute int) error {
	return fm.Components().ForEach(func(c *component.Component) error {
		tick := c.InputByName(portTick)
		if tick == nil {
			return nil
		}
		return tick.PutPayloads(minute)
	})
}
