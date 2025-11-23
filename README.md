<div align="center">
  <h1>F-Mesh Examples</h1>
  <p>Real-world examples of Flow-Based Programming with F-Mesh</p>

[![F-Mesh](https://img.shields.io/badge/F--Mesh-v1.4.0-blue)](https://github.com/hovsep/fmesh/releases/tag/v1.4.0-Vagharshapat)
[![Go Version](https://img.shields.io/badge/Go-1.24+-00ADD8?logo=go)](https://go.dev/)
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

| Example | Description |
|---------|-------------|
| [Async Input](./async_input/main.go) | Handling asynchronous data sources |
| [Electric Circuit](./electric_circuit/main.go) | Simulating electrical components |
| [Fibonacci](./fibonacci/main.go) | Recursive computation with cyclic pipes |
| [Filter](./filter/main.go) | Data filtering and routing |
| [Graphviz](./graphviz/main.go) | Visualizing mesh topology |
| [Load Balancer](./load_balancer/main.go) | Distributing work across components |
| [Nesting](./nesting/main.go) | Composing meshes within meshes |
| [Pipeline](./pipeline/main.go) | Sequential data processing |
| [String Processing](./string_processing/main.go) | Text transformation pipeline |
| [Basic CAN Bus](./can_bus/basic/main.go) | Simple automotive network simulation |
| [Advanced CAN Bus](./can_bus/advanced/main.go) | Full CAN protocol with ISO-TP |

---

## Quick Start

### Prerequisites

- Go 1.24 or later
- Git

### Running Examples

```bash
# Clone and setup
git clone https://github.com/hovsep/fmesh-examples.git
cd fmesh-examples
go mod tidy

# Run any example
go run ./fibonacci
go run ./electric_circuit

# Build all examples
make build

# Generate visualization graphs
make graph
```

---

## Project Structure

```
fmesh-examples/
├── fibonacci/
│   ├── main.go      # Example code
│   ├── graph.dot    # Graphviz source (generated)
│   └── graph.svg    # Visual diagram (generated)
├── electric_circuit/
│   └── main.go
└── can_bus/
    ├── basic/main.go
    └── advanced/
        ├── main.go
        └── can/     # Reusable CAN components
```

Each example is a standalone Go program. Visualization files (`graph.dot` and `graph.svg`) are generated using `make graph`.

---

## Contributing

We welcome new examples from any domain: simulations, data processing, protocols, algorithms, or real-world systems.

### How to Contribute

1. **Fork** this repository

2. **Create** a new directory for your example:
   ```bash
   mkdir my_example
   cd my_example
   ```

3. **Write** your example in `main.go`:
   - Follow existing patterns
   - Add comments explaining the scenario and concepts
   - Keep it focused on one concept

4. **Generate visualization** (optional):
   ```bash
   cd my_example
   go run . --graph
   ```

5. **Test** your example:
   ```bash
   go run .
   ```

6. **Update README.md**:
   - Add your example to the Examples table with a brief description

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
  <p>Built with <a href="https://github.com/hovsep/fmesh">F-Mesh</a> v1.4.0</p>
  <p>Questions? Open an <a href="https://github.com/hovsep/fmesh-examples/issues">issue</a> or check the <a href="https://github.com/hovsep/fmesh/wiki">wiki</a></p>
</div>
