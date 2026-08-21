# State Machine

A state diagram and a mesh are the same picture: circles connected by arrows. This example takes that literally and builds a finite-state machine out of nothing but fmesh — no library, no new types — so the exported graph below is the state diagram, and nobody drew it by hand.

The scenario is the life of an order: `created → paid → shipped → delivered`, with `cancel` escapes along the way, an `update` self-loop while the order is still editable, and a `pay` transition guarded by a payment check. The demo walks the whole arc — wrong events bounce off, a cheap payment is refused, a real one goes through — then rebuilds the machine from its saved state in a fresh mesh and carries the order to the end of the road.

## How it works

- **Every state is a component**, named after it, with two inputs (`enter`, `event`) and one output per event that leads out of it — so every arrow on the graph is labeled with the event that fires it. A state with no outputs is final by construction: after entering it the mesh has nothing left to do, and the run simply ends. Nobody counts cycles, nobody pulls a brake.
- **Every transition is a pipe** from the source state's event-named output to the target state's `enter` input. `getMesh` is the diagram in code: `addState` declares a circle and the arrows leaving it, `port.MultiPipe` draws each arrow to its target.
- **Firing an event is dropping a signal into the mesh.** The event rides as a label on the signal; `fire` puts it on the current state's `event` port and runs the mesh, which settles after one transition at most. The state routes the signal out of the output of the same name — or, if it has none, refuses it with an activation error that comes straight back from `Run` and satisfies `errors.Is`.
- **The current state is a mesh label.** Whichever state a transition lands in writes its own name into `fm.Labels()` — the state component is the only one who knows for sure. The machine's whole state is that one string, which is why `getMesh(startAt)` builds it with `fmesh.WithLabel`: a fresh mesh started from a saved state resumes mid-journey.
- **A guard is a component on its pipe.** `pay?` reads the whole signal — scalars and labels, not just the payload — and either forwards it to `paid` or returns an error. Nothing moved, so the current state is untouched.
- **A refusal leaves no residue.** fmesh stops the run on the error before draining that cycle's inputs, so the refused signal would stay on the port and replay on the next fire. A `component.WithHooks` `OnError` hook on every state and guard clears its inputs instead.
- Notable APIs: `component.Sequential` + `component.When(component.HasSignalsOn(...))` to branch an activation on which port has signals, `port.MultiPipe` / `port.Pipe`, `fmesh.WithLabel` and `fm.Labels()` for mesh-level metadata, `signal.And` / `signal.HasLabel` predicates, `component.WithHooks` with `OnError`, and `fmesh.StopOnFirstErrorOrPanic` to turn an activation error into the run's result.

![Mesh graph](./order%20lifecycle-graph.svg)

## Run

```bash
go run .

# Regenerate the graph files (order lifecycle-graph.dot / .svg)
FMESH_GRAPH=1 go run .
```
