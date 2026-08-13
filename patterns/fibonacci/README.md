# Fibonacci

This example demonstrates a self-feedback pipe: a component's own outputs are wired back into its own inputs. It teaches cycles — the one f-mesh idea a single-component, single-pipe mesh can't show, since here activation feeds itself.

The scenario is Fibonacci number generation: one generator component is seeded with `F(0) = 0` and `F(1) = 1`, then re-activates on each cycle, computing the next number from the previous two. The sequence emerges purely from re-activation, not from a loop in the Go code — each printed `Fibonacci: %d` line is one activation of the same component.

## How it works

- **`fibonacci number generator`** — inputs `i_cur`, `i_prev`; outputs `o_cur`, `o_prev`. Each activation reads `cur` and `prev` (defaulting to `0` via `FirstPayloadOrDefault(0)`), computes `next := cur + prev`, and either:
  - `next < 100`: prints `Fibonacci: %d`, puts `next` on `o_cur` and `cur` on `o_prev`, or
  - `next >= 100`: prints `%d >= 100, sequence complete` and puts nothing on either output.
- **Self-feedback pipes**: `o_cur → i_cur` and `o_prev → i_prev`, both on the same component, via `Outputs().ByName("o_cur").PipeTo(Inputs().ByName("i_cur"))`.
- **Termination**: once no signals are put on any output, there's nothing left to pipe forward, so the component has no reason to activate again and `fm.Run()` returns.

The mesh (`fibonacci example`) is built with `fmesh.New(..., fmesh.WithErrorHandlingStrategy(fmesh.StopOnFirstErrorOrPanic))`. Seeds are put directly on the component's inputs with `ComponentByName(...).Inputs().ByName("i_prev").PutSignals(signal.New(0))` before `fm.Run(context.Background())`.

![Mesh graph](./fibonacci%20example-graph.svg)

## Run

```bash
go run .

# Regenerate the graph files (fibonacci example-graph.dot / .svg)
FMESH_GRAPH=1 go run .
```
