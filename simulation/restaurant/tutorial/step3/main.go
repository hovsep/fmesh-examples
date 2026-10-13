// Step 3 of the restaurant tutorial: run.
//
// What this step adds: the clock and the bookkeeper. A run loop (simulation.go)
// seeds the current minute into every component that has a "tick" input and
// runs the mesh once per minute, for a whole day. The new report component
// (report.go) adds up served and lost parties; at the end we read its state and
// print the report.
//
// What to look at: runDay and runMinute in simulation.go. One mesh run is one
// simulated minute; the components keep their state between runs, so the day
// builds up minute by minute. Everything else is the code of step 2. Three
// tables turn out to be far too few for this crowd; step 4 uses the real plan.
package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Println("=== Restaurant tutorial, step 3: a whole day ===")

	report, err := runDay(tutorialTables)
	if err != nil {
		fmt.Println("Simulation failed:", err)
		os.Exit(1)
	}
	fmt.Printf("A day with %d tables and a first-come-first-served host:\n", len(tutorialTables))
	fmt.Print(report)
}
