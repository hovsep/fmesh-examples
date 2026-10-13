// This file holds the configuration of a restaurant and the wiring that turns
// the components into one mesh.
package main

import (
	"fmt"

	"github.com/hovsep/fmesh"
	"github.com/hovsep/fmesh/component"
	"github.com/hovsep/fmesh/port"
)

// The two seating policies a host can follow.
const (
	policyFIFO    = "fifo"
	policyBestFit = "best-fit"
)

// Defaults of a restaurant day.
const (
	defaultPatienceMinutes = 20      // a party gives up after waiting this long
	defaultOpeningMinutes  = 12 * 60 // open from 11:00 to 23:00
	defaultRevenuePerGuest = 25      // euros per served guest
	defaultSeed            = 1       // any fixed seed makes the day reproducible
)

// defaultTables is the seating plan: four 2-tops, three 4-tops and one 6-top.
var defaultTables = []int{2, 2, 2, 2, 4, 4, 4, 6}

// Config describes one restaurant and one day.
type Config struct {
	Tables          []int  // seats per table
	HostPolicy      string // policyFIFO or policyBestFit
	PatienceMinutes int
	OpeningMinutes  int
	RevenuePerGuest int
	Seed            int64
}

// defaultConfig is the restaurant as it is today, run by a first-come-first-served host.
func defaultConfig() Config {
	return Config{
		Tables:          defaultTables,
		HostPolicy:      policyFIFO,
		PatienceMinutes: defaultPatienceMinutes,
		OpeningMinutes:  defaultOpeningMinutes,
		RevenuePerGuest: defaultRevenuePerGuest,
		Seed:            defaultSeed,
	}
}

// newRestaurant builds the mesh: every component plus the pipes between them.
//
//	arrivals.party        -> waiting-list.party
//	waiting-list.waiting  -> host.waiting
//	waiting-list.lost     -> report.lost
//	host.seated           -> waiting-list.seated
//	host.table<i>         -> table-<i>.seat        (for every table)
//	table-<i>.free        -> host.free
//	table-<i>.served      -> report.served
func newRestaurant(cfg Config) (*fmesh.FMesh, error) {
	arrivals, err := newArrivals(cfg.Seed)
	if err != nil {
		return nil, err
	}
	waitingList, err := newWaitingList(cfg.PatienceMinutes)
	if err != nil {
		return nil, err
	}
	host, err := newHostForPolicy(cfg.HostPolicy, len(cfg.Tables))
	if err != nil {
		return nil, err
	}
	report, err := newReport(len(cfg.Tables), cfg.RevenuePerGuest)
	if err != nil {
		return nil, err
	}
	tables, err := newTables(cfg.Tables)
	if err != nil {
		return nil, err
	}

	fm, err := fmesh.New("restaurant",
		fmesh.WithDescription("a restaurant for one day: parties arrive, wait, eat and leave, one mesh run per minute"),
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
