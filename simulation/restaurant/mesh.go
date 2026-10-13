// This file holds the restaurant as it is today and the wiring that turns the
// components into one mesh. To ask a what-if question, change a number here or
// swap the host in buildMesh and run the day again.
package main

import (
	"context"
	"fmt"

	"github.com/hovsep/fmesh"
	"github.com/hovsep/fmesh/component"
	"github.com/hovsep/fmesh/port"
)

// The restaurant and its day.
const (
	patienceMinutes = 20      // a party gives up after waiting this long
	openingHour     = 11      // minute 0 of the day is 11:00
	openingMinutes  = 12 * 60 // open until 23:00
	revenuePerGuest = 25      // euros per served guest
	seed            = 1       // any fixed seed makes the day reproducible
)

// tableSeats is the seating plan: four 2-tops, three 4-tops and one 6-top.
var tableSeats = []int{2, 2, 2, 2, 4, 4, 4, 6}

// buildMesh builds the mesh: every component plus the pipes between them.
//
//	clock.tick            -> <every component>.tick
//	arrivals.party        -> waiting-list.party
//	waiting-list.waiting  -> host.waiting
//	waiting-list.lost     -> report.lost
//	host.seated           -> waiting-list.seated
//	host.table<i>         -> table-<i>.seat        (for every table)
//	table-<i>.free        -> host.free
//	table-<i>.served      -> report.served
//
// Every component is created by its own constructor in its own file; this
// function only puts them together.
func buildMesh() (*fmesh.FMesh, error) {
	clock, err := newClock()
	if err != nil {
		return nil, err
	}
	arrivals, err := newArrivals(seed)
	if err != nil {
		return nil, err
	}
	waitingList, err := newWaitingList(patienceMinutes)
	if err != nil {
		return nil, err
	}
	// The host is the one component with an alternative: newBestFitHost has
	// the same ports, so swapping it here changes nothing else.
	host, err := newFIFOHost(len(tableSeats))
	if err != nil {
		return nil, err
	}
	report, err := newReport(len(tableSeats), revenuePerGuest)
	if err != nil {
		return nil, err
	}
	tables := make([]*component.Component, len(tableSeats))
	for i, seats := range tableSeats {
		if tables[i], err = newTable(i, seats); err != nil {
			return nil, err
		}
	}

	// A mesh is created with options, like a component. StopOnFirstErrorOrPanic
	// ends the run at the first error, so a bug cannot hide in a half-done minute.
	fm, err := fmesh.New("restaurant",
		fmesh.WithDescription("a restaurant for one day: parties arrive, wait, eat and leave, one mesh run per minute"),
		fmesh.WithErrorHandlingStrategy(fmesh.StopOnFirstErrorOrPanic),
	)
	if err != nil {
		return nil, err
	}
	if err := fm.AddComponents(clock, arrivals, waitingList, host, report); err != nil {
		return nil, err
	}
	if err := fm.AddComponents(tables...); err != nil {
		return nil, err
	}

	// A pipe joins an output port to an input port: whatever a component puts
	// on the output is delivered to the input in the next round.
	err = port.MultiPipe(
		port.Pipe{From: arrivals.OutputByName("party"), To: waitingList.InputByName("party")},
		port.Pipe{From: waitingList.OutputByName("waiting"), To: host.InputByName("waiting")},
		port.Pipe{From: waitingList.OutputByName("lost"), To: report.InputByName("lost")},
		port.Pipe{From: host.OutputByName("seated"), To: waitingList.InputByName("seated")},
	)
	if err != nil {
		return nil, err
	}
	for i, table := range tables {
		err = port.MultiPipe(
			port.Pipe{From: host.OutputByName(fmt.Sprintf("table%d", i)), To: table.InputByName("seat")},
			port.Pipe{From: table.OutputByName("free"), To: host.InputByName("free")},
			port.Pipe{From: table.OutputByName("served"), To: report.InputByName("served")},
		)
		if err != nil {
			return nil, err
		}
	}

	// One output can feed many pipes: every component with a "tick" input
	// hears the same minute from the clock.
	for c := range fm.Components().All() {
		if tick := c.InputByName("tick"); tick != nil {
			if err := clock.OutputByName("tick").PipeTo(tick); err != nil {
				return nil, err
			}
		}
	}
	// A hook runs code at a moment in the mesh's life. Nobody inside the mesh
	// can start the clock, so before every run this hook nudges it, and the
	// minute goes out. That is what makes one run one minute.
	fm.SetupHooks(func(hooks *fmesh.Hooks) {
		hooks.BeforeRun(func(_ context.Context, _ *fmesh.FMesh) error {
			return clock.InputByName("advance").PutPayloads(true)
		})
	})
	return fm, nil
}
