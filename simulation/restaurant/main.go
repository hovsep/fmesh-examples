// This file holds the program: it builds the restaurant, exports the diagram
// when asked, and otherwise lets a whole day pass minute by minute and prints
// the bookkeeper's report.
//
// If F-Mesh is new to you: a mesh is a set of components joined by pipes.
// Signals travel the pipes. A component wakes up when a signal reaches one of
// its inputs, does its work and may put signals on its outputs, which the pipes
// deliver in the next round. fm.Run repeats rounds until no signal is left,
// then returns. Components keep their state from one run to the next, which
// is why calling Run once per minute adds up to a day.
package main

import (
	"context"
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

	fm, err := buildMesh()
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

	// One run is one minute: the clock ticks, every component that keeps time
	// hears the minute, and the mesh runs until all signals have been handled.
	for minute := range openingMinutes {
		if _, err := fm.Run(context.Background()); err != nil {
			fmt.Printf("Simulation failed at minute %d: %v\n", minute, err)
			os.Exit(1)
		}
	}

	// A component's state is plain data the program can read: the bookkeeper's
	// state is the report.
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
