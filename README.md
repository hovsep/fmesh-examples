<div align="center">
  <h1>F-Mesh Examples</h1>
  <p>Real-world examples of Flow-Based Programming with F-Mesh</p>

[![F-Mesh](https://img.shields.io/badge/F--Mesh-v1.13.0-blue)](https://github.com/hovsep/fmesh/releases/tag/v1.13.0)
[![Go Version](https://img.shields.io/badge/Go-1.27+-00ADD8?logo=go)](https://go.dev/)
[![Go Report Card](https://goreportcard.com/badge/github.com/hovsep/fmesh-examples)](https://goreportcard.com/report/github.com/hovsep/fmesh-examples)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

[![GitHub stars](https://img.shields.io/github/stars/hovsep/fmesh-examples?style=social)](https://github.com/hovsep/fmesh-examples/stargazers)
[![GitHub issues](https://img.shields.io/github/issues/hovsep/fmesh-examples)](https://github.com/hovsep/fmesh-examples/issues)
[![GitHub last commit](https://img.shields.io/github/last-commit/hovsep/fmesh-examples)](https://github.com/hovsep/fmesh-examples/commits/main)
[![GitHub contributors](https://img.shields.io/github/contributors/hovsep/fmesh-examples)](https://github.com/hovsep/fmesh-examples/graphs/contributors)

</div>

---

## About

This repository contains practical examples demonstrating [F-Mesh](https://github.com/hovsep/fmesh) - a Flow-Based Programming framework for Go. Each example shows how to model real-world systems as computational graphs with interconnected components.

**What is F-Mesh?**

F-Mesh is an FBP-inspired framework that lets you express your program as a mesh of components connected by pipes. Instead of writing imperative code, you describe how data flows through your system, making complex interactions more natural and maintainable. [Learn more in the wiki](https://github.com/hovsep/fmesh/wiki).

---

## Examples

Examples are grouped by what they teach, not by what they compute. Start at the top.

### `basics/` — everyday meshes

| Example | Description |
|---------|-------------|
| [String Processing](./basics/string_processing) | Two components and one pipe: the smallest useful mesh |
| [Filter](./basics/filter) | Routing signals to different outputs by label |
| [Pipeline](./basics/pipeline) | A multi-stage chain that reads stdin and files |

### `patterns/` — how a mesh is wired and driven

| Example | Description |
|---------|-------------|
| [Fibonacci](./patterns/fibonacci) | Cycles: a component's outputs piped back into its own inputs |
| [Nesting](./patterns/nesting) | Composition: a component whose activation runs a whole inner mesh |
| [Load Balancer](./patterns/load_balancer) | Indexed ports and component state: round-robin across N workers |
| [Async Input](./patterns/async_input) | Driving a mesh from outside: signals injected on a ticker, results drained into a channel |
| [State Machine](./patterns/state_machine) | An FSM out of nothing but fmesh: states are components, transitions are pipes, the current state is a mesh label — the mesh graph is the state diagram |

### `simulation/` — systems evolving over time

| Example | Description |
|---------|-------------|
| [Electric Circuit](./simulation/electric_circuit) | A supply/demand feedback loop with degrading state |
| [Basic CAN Bus](./simulation/can_bus/basic) | One bus fanning every frame out to all connected ECUs |
| [Advanced CAN Bus](./simulation/can_bus/advanced) | Full CAN protocol with ISO-TP, arbitration and diagnostics |
| [Life](./simulation/life) | A step simulation of human physiology inside a habitat |

These share [`simulation/sim`](./simulation/sim) — a small library for building simulations on an f-mesh (engines, simulated time, commands, scheduling, telemetry). It is a library, not an example.

### `graphics/` — pixels

| Example | Description |
|---------|-------------|
| [Graphviz](./graphics/graphviz) | Exporting mesh topology to DOT/SVG, with activated components highlighted |
| [Ray Tracer](./graphics/ray_tracer) | Wavefront 3D ray tracer: parallel tile bands and a reflection cycle |

---

## Quick Start

### Prerequisites

- Go 1.27 or later
- Git

### Running Examples

```bash
# Clone and setup
git clone https://github.com/hovsep/fmesh-examples.git
cd fmesh-examples
go mod tidy

# Run any example, from the repo root...
go run ./patterns/fibonacci
go run ./simulation/electric_circuit
go run ./graphics/ray_tracer

# ...or from the example's own directory
cd patterns/fibonacci && go run .

# Build all examples
make build

# Generate visualization graphs
make graph
```

---

## Project Structure

```
fmesh-examples/
├── basics/                  # everyday meshes
├── patterns/                # how a mesh is wired and driven
│   └── fibonacci/
│       ├── main.go          # example code
│       ├── *-graph.dot      # graphviz source (generated)
│       └── *-graph.svg      # visual diagram (generated)
├── simulation/              # systems evolving over time
│   ├── sim/                 # shared simulation library — not an example
│   ├── electric_circuit/
│   ├── can_bus/
│   │   ├── basic/main.go
│   │   └── advanced/
│   │       ├── main.go
│   │       └── can/         # reusable CAN components
│   └── life/
├── graphics/                # pixels
└── internal/                # FMESH_GRAPH helper, shared by every example
```

Each example is a standalone Go program, runnable either from the repo root (`go run ./patterns/fibonacci`) or from its own directory (`cd patterns/fibonacci && go run .`). Visualization files (`*-graph.dot` and `*-graph.svg`) are generated using `make graph`, and land in the example's own directory.

---

## Contributing

We welcome new examples from any domain: simulations, data processing, protocols, algorithms, or real-world systems.

### How to Contribute

1. **Fork** this repository

2. **Create** a new directory under the category that fits what your example teaches
   (`basics`, `patterns`, `simulation` or `graphics` — add a new category if none fit):
   ```bash
   mkdir patterns/my_example
   cd patterns/my_example
   ```

3. **Write** your example in `main.go`:
   - Follow existing patterns
   - Add comments explaining the scenario and concepts
   - Keep it focused on one concept

4. **Generate visualization** (optional):
   ```bash
   FMESH_GRAPH=1 go run .
   ```

5. **Test** your example:
   ```bash
   go run .
   ```

6. **Update README.md**:
   - Add your example to its category's table with a brief description

7. **Submit** a pull request

### Example Template

```go
package main

import (
    "fmt"
    "github.com/hovsep/fmesh"
    "github.com/hovsep/fmesh/component"
    "github.com/hovsep/fmesh/signal"
)

// Description of what this example demonstrates.
// Run: go run .

func main() {
    fm := fmesh.New("example").
        AddComponents(
            component.New("processor").
                AddInputs("in").
                AddOutputs("out").
                WithActivationFunc(func(c *component.Component) error {
                    // Your logic here
                    return nil
                }),
        )
    
    // Connect, initialize, run, and display results
}
```

### Guidelines

- One concept per example
- Well-commented code explaining the "why"
- Real-world scenarios preferred
- Self-contained and tested

---

## Resources

- **[F-Mesh Repository](https://github.com/hovsep/fmesh)** - Main framework
- **[F-Mesh Wiki](https://github.com/hovsep/fmesh/wiki)** - Complete documentation
- **[F-Mesh Graphviz](https://github.com/hovsep/fmesh-graphviz)** - Visualization tool
- **[Flow-Based Programming](https://jpaulm.github.io/fbp/)** - Learn about FBP (by J. Paul Morrison)

---

## License

MIT License - see [LICENSE](LICENSE) file for details.

---

<div align="center">
  <p>Built with <a href="https://github.com/hovsep/fmesh">F-Mesh</a> v1.13.0</p>
  <p>Questions? Open an <a href="https://github.com/hovsep/fmesh-examples/issues">issue</a> or check the <a href="https://github.com/hovsep/fmesh/wiki">wiki</a></p>
</div>
