// This file holds the "report" component, the bookkeeper of the day: it adds
// up every served and every lost party, and the Report type that prints the
// result.
package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/hovsep/fmesh"
	"github.com/hovsep/fmesh/component"
)

// Key of the report's state: the Report so far.
const stateReport = "report"

// Report is the day's bookkeeping. Totals grow during the day; the methods
// turn them into the figures a restaurant owner asks for.
type Report struct {
	Tables          int // number of tables in the restaurant
	RevenuePerGuest int // euros one served guest brings in
	Minutes         int // minutes the restaurant has been open

	PartiesServed    int
	GuestsServed     int
	PartiesLost      int
	GuestsLost       int
	TotalWaitMinutes int // summed over served parties
	BusyTableMinutes int // table-minutes with a party at the table
}

// newReport creates the bookkeeper for a restaurant with that many tables.
func newReport(numberOfTables int, revenuePerGuest int) (*component.Component, error) {
	return component.New(reportName,
		component.WithDescription("the bookkeeper: counts served and lost parties, waiting time and revenue"),
		component.WithInputs(portServed, portLost, portTick),
		component.WithInitialState(func(state component.State) {
			state.Set(stateReport, Report{Tables: numberOfTables, RevenuePerGuest: revenuePerGuest})
		}),
		component.WithActivationFunc(func(_ context.Context, this *component.Component) error {
			report := this.State().Get(stateReport).(Report)
			defer func() { this.State().Set(stateReport, report) }()

			report.Minutes += this.InputByName(portTick).Signals().Len()

			served, err := payloads[Served](this.InputByName(portServed))
			if err != nil {
				return err
			}
			for _, visit := range served {
				report.PartiesServed++
				report.GuestsServed += visit.Party.Size
				report.TotalWaitMinutes += visit.WaitedMinutes
				report.BusyTableMinutes += visit.Party.MealDuration
			}

			lost, err := payloads[Party](this.InputByName(portLost))
			if err != nil {
				return err
			}
			for _, party := range lost {
				report.PartiesLost++
				report.GuestsLost += party.Size
			}
			return nil
		}),
	)
}

// readReport takes the Report out of the mesh after the day.
func readReport(fm *fmesh.FMesh) Report {
	return fm.ComponentByName(reportName).State().Get(stateReport).(Report)
}

// AverageWait is the mean waiting time of a served party, in minutes.
func (r Report) AverageWait() float64 {
	if r.PartiesServed == 0 {
		return 0
	}
	return float64(r.TotalWaitMinutes) / float64(r.PartiesServed)
}

// Utilisation is the share of table-minutes that had a party at the table, 0..1.
func (r Report) Utilisation() float64 {
	available := r.Tables * r.Minutes
	if available == 0 {
		return 0
	}
	return float64(r.BusyTableMinutes) / float64(available)
}

// Revenue is what the served guests paid, in euros.
func (r Report) Revenue() int {
	return r.GuestsServed * r.RevenuePerGuest
}

// LostRevenue is what the guests who left without eating would have paid.
func (r Report) LostRevenue() int {
	return r.GuestsLost * r.RevenuePerGuest
}

// String prints the report as a short, aligned summary.
func (r Report) String() string {
	var b strings.Builder
	fmt.Fprintf(&b, "  Parties served : %4d  (%d guests)\n", r.PartiesServed, r.GuestsServed)
	fmt.Fprintf(&b, "  Parties lost   : %4d  (%d guests)\n", r.PartiesLost, r.GuestsLost)
	fmt.Fprintf(&b, "  Average wait   : %6.1f min\n", r.AverageWait())
	fmt.Fprintf(&b, "  Utilisation    : %6.1f %%\n", 100*r.Utilisation())
	fmt.Fprintf(&b, "  Revenue        : %4d EUR\n", r.Revenue())
	fmt.Fprintf(&b, "  Lost revenue   : %4d EUR\n", r.LostRevenue())
	return b.String()
}
