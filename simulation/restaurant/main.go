// This file holds the program: it prints a banner, exports the diagram when
// asked, and then answers the owner's question with a what-if table: which
// host policy and which seating plan earn the most?
package main

import (
	"fmt"
	"os"
	"slices"

	"github.com/hovsep/fmesh-examples/internal"
)

// extraTableDailyCost is what one added table costs per day (rent, staff, cleaning), in euros.
const extraTableDailyCost = 60

// extraTables is the seating plan change the owner is considering: two more 4-tops.
var extraTables = []int{4, 4}

// option is one row of the what-if table: a policy, a seating plan and the day they produced.
type option struct {
	Policy string
	Tables []int
	Report Report
}

func main() {
	fmt.Println("=== Restaurant Simulation ===")
	fmt.Println("One mesh run is one minute. Parties arrive, wait for a table, eat and leave;")
	fmt.Println("a host decides who sits where and a bookkeeper counts the day.")
	fmt.Println("Written for the ESM 2026 tutorial on F-Mesh.")
	fmt.Println()

	fm, err := newRestaurant(defaultConfig())
	if err != nil {
		fmt.Println("Failed to build mesh:", err)
		os.Exit(1)
	}
	handled, err := internal.HandleGraphFlag(fm)
	if err != nil {
		fmt.Println("Failed to generate graph:", err)
		os.Exit(1)
	}
	if handled {
		return
	}

	options, err := runWhatIf(defaultConfig())
	if err != nil {
		fmt.Println("Simulation failed:", err)
		os.Exit(1)
	}
	printWhatIf(options)
	fmt.Println()
	fmt.Println(decision(options))
	fmt.Println("=== Simulation Complete ===")
}

// runWhatIf runs the same day (same seed, same guests) for every combination of
// host policy and seating plan.
func runWhatIf(base Config) ([]option, error) {
	layouts := [][]int{base.Tables, slices.Concat(base.Tables, extraTables)}
	var options []option
	for _, policy := range []string{policyFIFO, policyBestFit} {
		for _, tables := range layouts {
			cfg := base
			cfg.HostPolicy = policy
			cfg.Tables = tables
			report, err := runDay(cfg)
			if err != nil {
				return nil, fmt.Errorf("%s with %d tables: %w", policy, len(tables), err)
			}
			options = append(options, option{Policy: policy, Tables: tables, Report: report})
		}
	}
	return options, nil
}

// printWhatIf prints one row per option.
func printWhatIf(options []option) {
	fmt.Printf("%-9s %6s %7s %5s %9s %6s %8s %9s\n",
		"policy", "tables", "served", "lost", "avg wait", "util", "revenue", "lost rev")
	for _, o := range options {
		r := o.Report
		fmt.Printf("%-9s %6d %7d %5d %7.1f m %5.0f%% %4d EUR %5d EUR\n",
			o.Policy, len(o.Tables), r.PartiesServed, r.PartiesLost,
			r.AverageWait(), 100*r.Utilisation(), r.Revenue(), r.LostRevenue())
	}
}

// netRevenue is the day's revenue minus the cost of the tables added to the base plan.
func (o option) netRevenue(baseTables int) int {
	added := len(o.Tables) - baseTables
	return o.Report.Revenue() - added*extraTableDailyCost
}

// decision reads the table for the owner: which option earns the most after
// paying for any extra tables, and how it compares with today.
func decision(options []option) string {
	today := options[0]
	baseTables := len(today.Tables)
	best := slices.MaxFunc(options, func(a, b option) int {
		return a.netRevenue(baseTables) - b.netRevenue(baseTables)
	})

	gain := best.netRevenue(baseTables) - today.netRevenue(baseTables)
	sentence := fmt.Sprintf("Decision: a %s host with %d tables earns the most, %d EUR net per day, %d EUR more than today's %s host with %d tables",
		best.Policy, len(best.Tables), best.netRevenue(baseTables), gain, today.Policy, baseTables)
	if len(best.Tables) > baseTables {
		return sentence + fmt.Sprintf("; the %d extra tables pay for themselves at %d EUR a day each.",
			len(best.Tables)-baseTables, extraTableDailyCost)
	}
	return sentence + fmt.Sprintf("; extra tables do not pay for themselves at %d EUR a day each.", extraTableDailyCost)
}
