// Step 2 of the restaurant tutorial: wiring.
//
// What this step adds: the arrivals, the waiting list and a first-come-first-
// served host, wired to three tables with pipes (see mesh.go). Together they
// form one mesh. We run exactly ONE minute by hand and print every signal that
// moves, cycle by cycle.
//
// What to look at: newRestaurant in mesh.go, where the pipes are drawn, and the
// printout. One minute is several cycles: the tables announce themselves and
// the waiting list shows the queue (cycle 1), the host seats the party
// (cycle 2), the table takes it and the queue forgets it (cycle 3, which
// produces no new signals, so the run ends). Nobody called anybody: signals
// simply flowed along the pipes.
//
// Run with FMESH_GRAPH=1 to export the diagram of the mesh instead.
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/hovsep/fmesh"
	"github.com/hovsep/fmesh-examples/internal"
	"github.com/hovsep/fmesh/component"
	"github.com/hovsep/fmesh/port"
	"github.com/hovsep/fmesh/signal"
)

func main() {
	fmt.Println("=== Restaurant tutorial, step 2: wiring ===")

	fm, err := newRestaurant(tutorialTables)
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

	// After every cycle, before the signals move on, print what each component produced.
	fm.SetupHooks(func(h *fmesh.Hooks) {
		h.AfterCycle(func(_ context.Context, cc *fmesh.CycleContext) error {
			printOutputs(cc)
			return nil
		})
	})

	// A party of four is already at the door when we open; then minute 0 starts.
	party := Party{ID: 1, Size: 4, ArrivedAt: 0, MealDuration: 60}
	if err := fm.ComponentByName(waitingListName).InputByName(portParty).PutPayloads(party); err != nil {
		fmt.Println("Failed to put the party:", err)
		os.Exit(1)
	}
	if err := seedTick(fm, 0); err != nil {
		fmt.Println("Failed to seed the tick:", err)
		os.Exit(1)
	}
	if _, err := fm.Run(context.Background()); err != nil {
		fmt.Println("Run failed:", err)
		os.Exit(1)
	}
}

// printOutputs lists every signal sitting on an output port at the end of a cycle.
func printOutputs(cc *fmesh.CycleContext) {
	fmt.Printf("cycle %d:\n", cc.Cycle.Number())
	_ = cc.FMesh.Components().ForEach(func(c *component.Component) error {
		return c.Outputs().ForEach(func(out *port.Port) error {
			return out.Signals().ForEach(func(sig *signal.Signal) error {
				fmt.Printf("  %s.%s -> %+v\n", c.Name(), out.Name(), sig.Payload())
				return nil
			})
		})
	})
}
