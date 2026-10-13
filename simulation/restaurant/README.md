# Restaurant

This example simulates one day in a restaurant as an F-Mesh and uses it to answer a business question: should the owner hire a smarter host, add two tables, or both? It was written for the ESM 2026 tutorial on F-Mesh, for an audience of engineers and non-programmers alike, so the code is meant to read like prose.

The scenario: parties of one to six guests arrive at the door, more of them around lunch and dinner. A host seats them at the tables that are free; a party that waits more than 20 minutes gives up and leaves. Every seated party eats for 45 to 90 minutes, pays 25 EUR per guest and goes. A bookkeeper counts served and lost parties, waiting time, table utilisation and revenue.

Time is discrete: **one mesh run is one simulated minute**. A run loop puts the current minute on the `tick` input of every component and calls `fm.Run(ctx)`, 720 times for a day from 11:00 to 23:00. Components keep their state between runs, so the day builds up minute by minute.

## How it works

| Component | Inputs | Outputs | State | Behaviour |
|-----------|--------|---------|-------|-----------|
| `arrivals` | `tick` | `party` | seeded random generator, next ID | Each minute decides how many parties walk in (two draws against a rate that peaks at lunch and dinner), gives each a size (couples are the most common) and a meal duration, and emits one `Party` per arrival. |
| `waiting-list` | `party`, `seated`, `tick` | `waiting`, `lost` | the queue, in arrival order | New parties join the queue, seated ones leave it. On every tick parties past their patience go out on `lost`; then the whole queue goes out on `waiting`, one signal per party, for the host to choose from. |
| `host` | `waiting`, `free`, `tick` | `seated`, `table0` ... `table<n-1>` | free tables (index -> seats) | Learns which tables are free from their announcements. When the waiting list arrives it seats parties according to its policy: a `Seating` goes to the matching indexed `table<i>` output and the party goes back on `seated`. Two hosts share this code and differ only in the policy function: **FIFO** (`newFIFOHost`) walks the parties in arrival order and gives each the first free table where it fits; **best fit** (`newBestFitHost`) walks the free tables from small to large and gives each the party that leaves the fewest empty seats, so big tables are kept for big parties. |
| `table-<i>` | `seat`, `tick` | `free`, `served` | seats, seated party, minutes left | One constructor for every size (`newTable(index, seats)`; the seats are also component metadata). Announces `TableFree` on the first tick and after every visit. On `seat` it stores the party; each tick counts the meal down and, when it is over, emits a `Served` record (party, seats, minutes waited, time left) and announces itself free again. |
| `report` | `served`, `lost`, `tick` | — | the `Report` so far | Adds up parties and guests served and lost, waiting minutes, busy table-minutes and minutes open. `readReport(fm)` takes the `Report` out after the day; its `String()` prints the summary. |

The wiring (`mesh.go`):

```
arrivals.party        -> waiting-list.party
waiting-list.waiting  -> host.waiting
waiting-list.lost     -> report.lost
host.seated           -> waiting-list.seated
host.table<i>         -> table-<i>.seat        (for every table)
table-<i>.free        -> host.free
table-<i>.served      -> report.served
```

![Mesh graph](./restaurant-graph.svg)

### How a minute works

One run is a few activation cycles. In the first cycle every component with a tick activates: a table whose party has finished emits `served` and `free`, the waiting list drops impatient parties and emits the queue, the arrivals may emit a new party. In the second cycle the host receives the queue and the free tables and seats whom it can; the report receives the served and lost parties; the waiting list receives the new arrivals. In the third cycle the tables receive their seatings and the waiting list forgets the seated parties. Nothing is left to deliver, so the run stops and the next minute begins.

### What-if and the decision

`main.go` runs the same day — same seed, so the same guests at the same minutes — four times: with the FIFO and the best-fit host, each with today's eight tables (four 2-tops, three 4-tops, one 6-top) and with two extra 4-tops. It prints one row per run and a closing sentence that reads the table as a decision: the option with the highest revenue after paying 60 EUR a day for each extra table (`extraTableDailyCost`).

```
policy    tables  served  lost  avg wait   util  revenue  lost rev
fifo           8      40     9     4.5 m    48% 2650 EUR  1000 EUR
fifo          10      44     5     3.8 m    43% 3050 EUR   600 EUR
best-fit       8      41     8     4.7 m    50% 2775 EUR   875 EUR
best-fit      10      46     3     3.7 m    44% 3300 EUR   350 EUR

Decision: a best-fit host with 10 tables earns the most, 3180 EUR net per day, 530 EUR more than today's fifo host with 8 tables; the 2 extra tables pay for themselves at 60 EUR a day each.
```

Change `defaultConfig()` in `mesh.go` (tables, patience, opening hours, revenue, seed) or the arrival rates in `arrivals.go` to ask your own questions.

Notable APIs: `component.WithIndexedOutputs` for the host's one-port-per-table, `component.WithInitialState` / `this.State()` for per-component state, `component.WithMeta` for the table size, `port.MultiPipe` for wiring, `sig.As[T]()` and `Signals().FirstAs[T]()` for typed payloads, and `fmesh.WithErrorHandlingStrategy(fmesh.StopOnFirstErrorOrPanic)` on the mesh.

## Run

```bash
go run .

# Regenerate restaurant-graph.dot / .svg
FMESH_GRAPH=1 go run .

# Tests, one file per component plus a whole-day test
go test -race .
```
