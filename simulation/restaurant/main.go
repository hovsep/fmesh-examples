// This file holds the program: it prints a banner, exports the diagram when
// asked, and then runs one day in the restaurant and prints the bookkeeper's
// report.
package main

import (
	"fmt"
	"os"

	"github.com/hovsep/fmesh-examples/internal"
)

func main() {
	fmt.Println("=== Restaurant Simulation ===")
	fmt.Println("One mesh run is one minute. Parties arrive, wait for a table, eat and leave;")
	fmt.Println("a host decides who sits where and a bookkeeper counts the day.")
	fmt.Println("Written for the ESM 2026 tutorial on F-Mesh.")
	fmt.Println()

	fm, err := newRestaurant()
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

	report, err := runDay()
	if err != nil {
		fmt.Println("Simulation failed:", err)
		os.Exit(1)
	}
	fmt.Printf("The day with %d tables, where %s:\n", len(tableSeats), fm.ComponentByName("host").Description())
	fmt.Print(report)
	fmt.Println("=== Simulation Complete ===")
}
