# Restaurant tutorial

The [restaurant example](../) rebuilt in five steps, each a complete program of its own. From step 2 on, a step directory mirrors the file layout of the main example (`ports.go`, `party.go`, `arrivals.go`, ... ) and every file holds only what that step needs, so you can diff two steps to see exactly what was added. Each `main.go` starts with a comment on what the step adds and what to look at. Written for the ESM 2026 tutorial on F-Mesh.

## Step 1 — Components

One `table` component, built with `newTable`, inside a mesh of its own. We activate it by hand for seven minutes and seat a party in minute 2; the program prints what comes out of `free` and `served`. Look at the activation function: it reads its inputs, updates its state and writes to its outputs, and that is all a component ever does. Run: `go run ./tutorial/step1`.

## Step 2 — Wiring

The arrivals, the waiting list and a first-come-first-served host, wired to three tables (`mesh.go`). We put a party at the door, run exactly one minute and print every signal, cycle by cycle, with an `AfterCycle` hook. Look at `newRestaurant`, where the pipes are drawn, and at how one minute is three cycles: announce and queue, seat, take the seat. Run: `go run ./tutorial/step2`; `FMESH_GRAPH=1 go run ./tutorial/step2` exports the diagram.

## Step 3 — Run

Adds the clock and the bookkeeper: `simulation.go` seeds the minute into every `tick` input and runs the mesh once per minute for a whole day; `report.go` adds up served and lost parties, waiting time, utilisation and revenue. Look at `runDay` and `runMinute`: components keep their state between runs, so the day builds up minute by minute. Three tables turn out to be far too few. Run: `go run ./tutorial/step3`.

## Step 4 — What-if

Adds the best-fit host (`host.go`). It has the same ports as the FIFO host and shares all of its code; only the seating rule differs. `newRestaurant` now takes the host constructor, so the same day runs twice, once per host, and the reports are printed side by side. Look at `newHost` and `seatingPolicy`: swapping a component changes nothing else. Run: `go run ./tutorial/step4`.

## Step 5 — Analyze

Adds a `Config` (`mesh.go`) and the what-if table (`main.go`): the same day for each host policy and each seating plan, and a closing sentence that reads the table as a business decision. This is the complete example, file for file. Look at `runWhatIf` and `decision`. Run: `go run ./tutorial/step5`.
