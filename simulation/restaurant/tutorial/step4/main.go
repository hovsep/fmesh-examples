// Step 4 of the restaurant tutorial: what-if.
//
// What this step adds: a second host. newBestFitHost (host.go) has exactly the
// same ports as newFIFOHost and shares all of its code; only the seating rule,
// a function, differs. Because the two are interchangeable, we run the same day
// twice, once with each host, and print the reports side by side.
//
// What to look at: newHost takes the policy as a parameter (seatingPolicy), and
// newRestaurant in mesh.go takes the host constructor. Swapping a component is
// the whole point of building the restaurant as a mesh: nothing else changes.
package main

import (
	"fmt"
	"os"
	"strings"
)

func main() {
	fmt.Println("=== Restaurant tutorial, step 4: two hosts, same day ===")

	fifo, err := runDay(tutorialTables, newFIFOHost)
	if err != nil {
		fmt.Println("Simulation failed:", err)
		os.Exit(1)
	}
	bestFit, err := runDay(tutorialTables, newBestFitHost)
	if err != nil {
		fmt.Println("Simulation failed:", err)
		os.Exit(1)
	}
	printSideBySide("first come, first served", fifo, "best fit", bestFit)
}

// printSideBySide prints two reports in two columns.
func printSideBySide(leftTitle string, left Report, rightTitle string, right Report) {
	const column = 40
	fmt.Printf("%-*s%s\n", column, leftTitle, rightTitle)
	leftLines := strings.Split(strings.TrimRight(left.String(), "\n"), "\n")
	rightLines := strings.Split(strings.TrimRight(right.String(), "\n"), "\n")
	for i := range leftLines {
		fmt.Printf("%-*s%s\n", column, leftLines[i], rightLines[i])
	}
}
