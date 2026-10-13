// This file holds the program: it builds the restaurant, exports the diagram
// when asked, and otherwise lets a whole day pass minute by minute and prints
// the bookkeeper's report.
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/hovsep/fmesh-examples/internal"
	"github.com/hovsep/fmesh/component"
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

	// The clock: every minute, each component with a "tick" input hears the
	// minute, then the mesh runs until all signals have been handled.
	// Components keep their state between runs, so the day builds up.
	for minute := range openingMinutes {
		err := fm.Components().ForEach(func(c *component.Component) error {
			if tick := c.InputByName("tick"); tick != nil {
				return tick.PutPayloads(minute)
			}
			return nil
		})
		if err == nil {
			_, err = fm.Run(context.Background())
		}
		if err != nil {
			fmt.Printf("Simulation failed at minute %d: %v\n", minute, err)
			os.Exit(1)
		}
	}

	// The bookkeeper's state is the report.
	report := fm.ComponentByName("report").State()
	fmt.Printf("  Host policy    : %s\n", fm.ComponentByName("host").Meta().ValueOrDefault("policy", "?"))
	fmt.Printf("  Tables         : %4d\n", len(tableSeats))
	fmt.Printf("  Parties served : %4d  (%d guests)\n", report.Get("parties-served"), report.Get("guests-served"))
	fmt.Printf("  Parties lost   : %4d  (%d guests)\n", report.Get("parties-lost"), report.Get("guests-lost"))
	fmt.Printf("  Average wait   : %6.1f min\n", report.Get("average-wait"))
	fmt.Printf("  Utilisation    : %6.1f %%\n", report.Get("utilisation"))
	fmt.Printf("  Revenue        : %4d EUR\n", report.Get("revenue"))
	fmt.Printf("  Lost revenue   : %4d EUR\n", report.Get("lost-revenue"))
	fmt.Println("=== Simulation Complete ===")
}
