# Internal Combustion Engine

A four-cylinder, four-stroke petrol engine, simplified until it fits on one screen but no further. Three things flow through it, and each is a flow through the mesh:

- **Fuel**: `fuel-tank → fuel-pump → cylinder-N`, into whichever cylinder is on its intake stroke.
- **Current**: `alternator → battery → starter / fuel-pump / ignition-coil`. The starter sags the battery while it cranks; once the engine runs, the alternator charges it back.
- **Rotation**: every cylinder and the starter put torque on the `crankshaft`, and the crankshaft's turn is what starts the next stroke.

That last pipe closes the loop. **One trip around the mesh is one stroke**, half a turn of the crankshaft, and nobody drives it from outside: you turn the key once, the starter turns the engine until it catches, the ECU follows the driver's foot, and the engine runs until the tank is dry. Then the crankshaft stops, nothing starts another stroke, and the mesh stops by itself.

```
Turning the key...
  stroke    0 │     0 rpm │ throttle   0% │ battery 12.6 V │ fuel 160.0 │ starter off
  stroke    2 │   342 rpm │ throttle   0% │ battery 10.5 V │ fuel 159.3 │ starter on
  stroke    4 │   689 rpm │ throttle   0% │ battery 11.9 V │ fuel 158.6 │ starter off
  ...
  stroke  120 │  2665 rpm │ throttle 100% │ battery 12.4 V │ fuel 105.0 │ starter off
  ...
  stroke  242 │     0 rpm │ throttle   0% │ battery 12.5 V │ fuel   0.0 │ starter off

The engine stopped after 242 strokes (1217 mesh cycles): the tank is dry.
```

## How it works

- **12 components, one per part**: crankshaft, ECU, alternator, battery, starter, fuel tank, fuel pump, ignition coil and four cylinders. Each keeps its own physics in its `State()`: the battery its charge, the tank its fuel, each cylinder its place in the four-stroke cycle and the fuel it took in.
- **A feedback cycle drives the simulation.** The crankshaft's `crank` output goes to the ECU and the alternator; everything downstream eventually comes back to the crankshaft as torque. No ticker, no loop in `main`: the mesh is the loop.
- **Flows of different lengths meet in the right stroke.** The control path (ECU → coil → cylinder) is shorter than the fuel path (ECU → tank → pump → cylinder) and the electric one (crankshaft → alternator → battery → coil). Every part waits for one signal on each of its inputs with `component.RequireInputs`, so each stroke's fuel, spark and voltage arrive together, and the crankshaft waits for the torque of all five sources.
- **Indexed ports** (`WithIndexedOutputs`) give the pump `fuel1..fuel4` and the coil `spark1..spark4`, one per cylinder.
- **The firing order is a property of the wiring, not of a loop.** Each cylinder starts at a different stroke so that power strokes come in the order 1-3-4-2; the ECU only has to tell the pump and the coil which cylinder is on intake and which on power.
- **The mesh stops itself.** When the crankshaft is still and the starter is off, it simply does not emit, so the run ends naturally (`Run` returns `nil`). Try `-battery 5`: the starter cannot turn the engine, it never catches, and the mesh stops after three strokes.
- A mesh-level `AfterCycle` hook reads the gauges from the components' state for the dashboard.

![Mesh graph](./combustion%20engine-graph.svg)

## Run

```bash
go run .

# A nearly flat battery
go run . -battery 5

# Regenerate the graph files (combustion engine-graph.dot / .svg)
FMESH_GRAPH=1 go run .
```
