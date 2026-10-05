# Graphviz

This example teaches how to export an f-mesh's topology to DOT/SVG using [`github.com/hovsep/fmesh-export`](https://github.com/hovsep/fmesh-export), and how to render each activation cycle as its own graph with the components that fired highlighted.

It runs a small car drivetrain mesh, then exports it twice: once as a static graph (topology only) and once per activation cycle (topology plus which components activated).

## How it works

The mesh models a drivetrain chain: `engine -> clutch -> gearbox -> wheels`. Starting the engine sends a rotation signal down the chain each cycle; the gearbox halves the rotation value before forwarding it to the wheels.

The example creates one exporter with `dot.New()`. After `fm.Run(...)` completes, it:

- calls `exporter.Export(fm)` for a static DOT graph of the topology, written to `static_graph.dot`
- calls `exporter.ExportCycle(fm, c)` for every cycle in `runtimeInfo.Cycles`: one DOT graph per activation cycle, each with the activated components highlighted, written to `cycle-001.dot`, `cycle-002.dot`, ... (each run overwrites the previous one's files)

Separately, `internal.HandleGraphFlag` (shared by every example via `FMESH_GRAPH=1`) exports the mesh's static topology and, if the `dot` binary is on `PATH`, converts it to `graph-graph.svg` — the image below.

![Mesh graph](./graph-graph.svg)

## Run

```bash
go run .

# Only write graph-graph.dot / graph-graph.svg via internal.HandleGraphFlag, then exit without running the mesh
FMESH_GRAPH=1 go run .
```

A plain `go run .` also writes the static and per-cycle `.dot` files described above into the current directory. To convert all of them to images:

```bash
for f in *.dot; do dot -Tpng "$f" -o "${f%.dot}.png"; done
```

## Animate the cycles as a GIF

The per-cycle graphs are frames of the run. With [graphviz](https://graphviz.org/) and [ImageMagick](https://imagemagick.org/) installed, render each frame to PNG and stitch them into a looping GIF, one second per cycle:

```bash
for f in cycle-*.dot; do dot -Tpng "$f" -o "${f%.dot}.png"; done
magick -delay 100 -loop 0 cycle-*.png mesh.gif
```

The zero-padded frame names keep the glob in cycle order. ImageMagick 6 names the command `convert` instead of `magick`.
