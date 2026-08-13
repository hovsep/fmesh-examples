# String Processing

The smallest useful mesh: two components joined by one pipe. It teaches the basic shape of an f-mesh program — components with named inputs and outputs, a pipe connecting one component's output to another's input, and a single `fm.Run()` driving both activations.

The scenario is a two-stage string pipeline: `concat` joins two input words into one string, and `case` title-cases the result. Feeding `"hello "` and `"world !"` in produces `"HELLO WORLD !"` out.

## How it works

- **`concat`** — inputs `i1`, `i2`, output `res`. Reads both input payloads as strings, concatenates them, and puts the result on `res`.
- **`case`** — input `i1`, output `res`. Reads the incoming string and applies `strings.ToTitle`, putting the result on `res`.
- **Pipe**: `concat.res → case.i1`, wired with `Outputs().ByName("res").PipeTo(Inputs().ByName("i1"))`.

The mesh is built with `fmesh.New("hello world", fmesh.WithErrorHandlingStrategy(...), fmesh.WithCyclesLimit(10))`, components are added via `AddComponents`, inputs are seeded directly with `InputByName("i1").PutSignals(signal.New(...))`, and the final result is read back with `OutputByName("res").Signals().FirstPayloadOrNil()`.

![Mesh graph](./hello%20world-graph.svg)

## Run

```bash
go run .

# Regenerate the graph files (hello world-graph.dot / .svg)
FMESH_GRAPH=1 go run .
```
