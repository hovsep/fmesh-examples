package main

import (
	"context"
	"fmt"
	"os"

	"github.com/hovsep/fmesh"
	"github.com/hovsep/fmesh-examples/internal"
	"github.com/hovsep/fmesh/component"
	"github.com/hovsep/fmesh/signal"
)

// Automatic differentiation where the mesh is the computation graph: one
// component per operation of a tiny neural network, values flowing forward
// through one set of pipes and gradients flowing back through another. This
// is backpropagation the way the textbook draws it, except the drawing runs.
//
// The network learns XOR, the classic problem no single neuron can solve:
//
//	h1 = tanh(w11·x1 + w12·x2 + b1)
//	h2 = tanh(w21·x1 + w22·x2 + b2)
//	ŷ  = σ(v1·h1 + v2·h2 + c)
//	loss = mean((ŷ - y)²)
//
// Run: go run .

const (
	learningRate = 0.5
	epochs       = 3000
)

// XOR, all four cases as one batch.
var (
	x1 = []float64{0, 0, 1, 1}
	x2 = []float64{0, 1, 0, 1}
	y  = []float64{0, 1, 1, 0}
)

// The starting weights. Fixed, so every run learns the same way.
var initial = map[string]float64{
	"w11": 0.5, "w12": -0.6, "b1": 0.1,
	"w21": -0.7, "w22": 0.8, "b2": -0.2,
	"v1": 0.3, "v2": -0.4, "c": 0.05,
}

func main() {
	fmt.Println("=== Auto-diff ===")
	fmt.Println("A neural network as a mesh: values flow forward, gradients flow back.")
	fmt.Println()

	fm, err := getMesh()
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

	for epoch := 1; epoch <= epochs; epoch++ {
		loss, err := trainStep(fm)
		if err != nil {
			fmt.Println("Training failed:", err)
			os.Exit(1)
		}
		if epoch == 1 || epoch%500 == 0 {
			fmt.Printf("  epoch %4d  loss %.5f\n", epoch, loss)
		}
	}

	fmt.Println()
	fmt.Println("  x1 x2 │ XOR │ network")
	pred := fm.ComponentByName("loss").State().Get("pred").([]float64)
	for i := range y {
		fmt.Printf("   %v  %v │  %v  │ %.3f\n", x1[i], x2[i], y[i], pred[i])
	}
}

// trainStep is one epoch: start every leaf, and the mesh does the forward
// pass, the backward pass and the weight update on its own. It returns the
// loss before the update.
func trainStep(fm *fmesh.FMesh) (float64, error) {
	for _, c := range fm.Components().AllOrdered() {
		if c.InputByName(portGo) == nil {
			continue
		}
		if err := c.InputByName(portGo).PutSignals(signal.New("go")); err != nil {
			return 0, err
		}
	}
	if _, err := fm.Run(context.Background()); err != nil {
		return 0, err
	}
	return fm.ComponentByName("loss").OutputByName(portLoss).Signals().FirstAs[float64]()
}

// getMesh builds the network. node() adds one operation and draws both
// pipes for each of its inputs: the value forward, the gradient back.
func getMesh() (*fmesh.FMesh, error) {
	fm, err := fmesh.New("autodiff",
		fmesh.WithDescription("a 2-2-1 network learning XOR: one component per operation, gradients on reverse pipes"),
	)
	if err != nil {
		return nil, err
	}

	add := func(c *component.Component, err error) error {
		if err != nil {
			return err
		}
		return fm.AddComponents(c)
	}

	// The leaves: data, and the parameters the network learns.
	for name, values := range map[string][]float64{"x1": x1, "x2": x2, "y": y} {
		if err := add(newData(name, values)); err != nil {
			return nil, err
		}
	}
	for name, v := range initial {
		if err := add(newParam(name, v, learningRate, len(y))); err != nil {
			return nil, err
		}
	}

	node := func(name string, op Op, inputs ...string) error {
		if err := add(newOperation(name, op)); err != nil {
			return err
		}
		return connect(fm, name, inputs...)
	}

	steps := []error{
		// Hidden neuron 1.
		node("w11·x1", opMul, "w11", "x1"),
		node("w12·x2", opMul, "w12", "x2"),
		node("Σ1", opSum3, "w11·x1", "w12·x2", "b1"),
		node("h1", opTanh, "Σ1"),
		// Hidden neuron 2.
		node("w21·x1", opMul, "w21", "x1"),
		node("w22·x2", opMul, "w22", "x2"),
		node("Σ2", opSum3, "w21·x1", "w22·x2", "b2"),
		node("h2", opTanh, "Σ2"),
		// Output neuron.
		node("v1·h1", opMul, "v1", "h1"),
		node("v2·h2", opMul, "v2", "h2"),
		node("Σ3", opSum3, "v1·h1", "v2·h2", "c"),
		node("ŷ", opSigmoid, "Σ3"),
		// The loss closes the forward pass and opens the backward one.
		add(newLoss("loss")),
		connect(fm, "loss", "ŷ", "y"),
	}
	for _, err := range steps {
		if err != nil {
			return nil, err
		}
	}
	return fm, nil
}

// connect wires a node to the nodes it reads: the i-th one's value into
// input a, b, c..., and the gradient for that input back into its grad port.
// Data has no grad port: it is not learned, so its gradient goes nowhere.
func connect(fm *fmesh.FMesh, name string, inputs ...string) error {
	node := fm.ComponentByName(name)
	for i, from := range inputs {
		src := fm.ComponentByName(from)
		if err := src.OutputByName(portOut).PipeTo(node.InputByName(inputNames[i])); err != nil {
			return fmt.Errorf("%s → %s: %w", from, name, err)
		}
		if grad := src.InputByName(portGrad); grad != nil {
			if err := node.OutputByName(gradOf(inputNames[i])).PipeTo(grad); err != nil {
				return fmt.Errorf("%s ⇠ %s: %w", from, name, err)
			}
		}
	}
	return nil
}
