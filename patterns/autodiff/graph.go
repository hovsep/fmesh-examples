package main

import (
	"context"
	"fmt"
	"math"

	"github.com/hovsep/fmesh/component"
	"github.com/hovsep/fmesh/signal"
)

// The building blocks of the computation graph. Every value is a vector,
// one entry per training example, so one run of the mesh pushes the whole
// batch forward and its gradients back.

// Port names. An operation's inputs are a, b, c...; for each input x it has
// a gradient output dx, piped back to whoever produced x.
const (
	portGo   = "go"   // a leaf: start a forward pass
	portOut  = "out"  // forward: this node's value
	portGrad = "grad" // backward: d(loss)/d(this node's value), from every consumer
	portLoss = "loss" // the loss node's report
)

var inputNames = []string{"a", "b", "c"}

func gradOf(input string) string { return "d" + input }

// Op is the math of one operation: how to compute its value from its inputs,
// and how to turn the gradient of its value into gradients of its inputs.
type Op struct {
	Name     string
	Arity    int
	Forward  func(in [][]float64) []float64
	Backward func(in [][]float64, out, grad []float64) [][]float64
}

var (
	opMul = Op{"×", 2,
		func(in [][]float64) []float64 { return zip(in[0], in[1], func(a, b float64) float64 { return a * b }) },
		func(in [][]float64, _, g []float64) [][]float64 {
			return [][]float64{
				zip(g, in[1], func(g, b float64) float64 { return g * b }), // d(a·b)/da = b
				zip(g, in[0], func(g, a float64) float64 { return g * a }), // d(a·b)/db = a
			}
		},
	}
	opSum3 = Op{"+", 3,
		func(in [][]float64) []float64 {
			return zip(zip(in[0], in[1], add), in[2], add)
		},
		func(_ [][]float64, _, g []float64) [][]float64 { return [][]float64{g, g, g} }, // passes the gradient through
	}
	opTanh = Op{"tanh", 1,
		func(in [][]float64) []float64 { return mapf(in[0], math.Tanh) },
		func(_ [][]float64, out, g []float64) [][]float64 {
			return [][]float64{zip(g, out, func(g, y float64) float64 { return g * (1 - y*y) })}
		},
	}
	opSigmoid = Op{"σ", 1,
		func(in [][]float64) []float64 {
			return mapf(in[0], func(x float64) float64 { return 1 / (1 + math.Exp(-x)) })
		},
		func(_ [][]float64, out, g []float64) [][]float64 {
			return [][]float64{zip(g, out, func(g, y float64) float64 { return g * y * (1 - y) })}
		},
	}
)

// newOperation builds one node of the graph.
//
// Forward: it waits until every input has a value, computes its own, keeps
// both in its state, and passes its value on.
//
// Backward: its value may feed several nodes, and each sends back a
// gradient. It waits for one from every consumer (it counts the pipes on its
// own output), adds them up, and sends each of its inputs its share.
func newOperation(name string, op Op) (*component.Component, error) {
	inputs := inputNames[:op.Arity]
	gradOutputs := make([]string, op.Arity)
	for i, in := range inputs {
		gradOutputs[i] = gradOf(in)
	}

	return component.New(name,
		component.WithDescription(op.Name),
		component.WithInputs(append([]string{portGrad}, inputs...)...),
		component.WithOutputs(append([]string{portOut}, gradOutputs...)...),
		component.WithActivationFunc(func(ctx context.Context, this *component.Component) error {
			if this.InputByName(portGrad).HasSignals() {
				grad, ready, err := collectGrad(this)
				if err != nil || !ready {
					return err
				}
				in := this.State().Get("in").([][]float64)
				out := this.State().Get("out").([]float64)
				for i, g := range op.Backward(in, out, grad) {
					if err := this.OutputByName(gradOutputs[i]).PutSignals(signal.New(g)); err != nil {
						return err
					}
				}
				return nil
			}

			if err := component.RequireInputs(inputs...)(ctx, this); err != nil {
				return err
			}
			in := make([][]float64, op.Arity)
			for i, name := range inputs {
				v, err := this.InputByName(name).Signals().FirstAs[[]float64]()
				if err != nil {
					return err
				}
				in[i] = v
			}
			out := op.Forward(in)
			this.State().Set("in", in)
			this.State().Set("out", out)
			return this.OutputByName(portOut).PutSignals(signal.New(out))
		}),
	)
}

// collectGrad waits until every consumer of this node's value has sent its
// gradient back, and returns their sum.
func collectGrad(this *component.Component) ([]float64, bool, error) {
	signals := this.InputByName(portGrad).Signals()
	if signals.Len() < this.OutputByName(portOut).Pipes().Len() {
		return nil, false, component.ErrWaitKeepingInputs
	}
	var total []float64
	for _, sig := range signals.All() {
		g, err := sig.As[[]float64]()
		if err != nil {
			return nil, false, err
		}
		if total == nil {
			total = make([]float64, len(g))
		}
		total = zip(total, g, add)
	}
	return total, true, nil
}

// newData is a leaf holding training data: inputs or targets. Data is not
// learned, so no gradient comes back to it.
func newData(name string, values []float64) (*component.Component, error) {
	return component.New(name,
		component.WithDescription(fmt.Sprint(values)),
		component.WithInputs(portGo),
		component.WithOutputs(portOut),
		component.WithActivationFunc(func(_ context.Context, this *component.Component) error {
			return this.OutputByName(portOut).PutSignals(signal.New(values))
		}),
	)
}

// newParam is a leaf the network learns: one number, sent forward as a
// vector the size of the batch. When its gradient comes back it takes a
// gradient-descent step, so the optimiser lives in the leaves.
func newParam(name string, initial, learningRate float64, batch int) (*component.Component, error) {
	return component.New(name,
		component.WithDescription("learned weight"),
		component.WithInputs(portGo, portGrad),
		component.WithOutputs(portOut),
		component.WithInitialState(func(s component.State) { s.Set("value", initial) }),
		component.WithActivationFunc(func(_ context.Context, this *component.Component) error {
			value := this.State().Get("value").(float64)
			if this.InputByName(portGrad).HasSignals() {
				grad, ready, err := collectGrad(this)
				if err != nil || !ready {
					return err
				}
				total := 0.0
				for _, g := range grad {
					total += g // one number, used by every example: their gradients add up
				}
				this.State().Set("grad", total)
				this.State().Set("value", value-learningRate*total)
				return nil
			}

			out := make([]float64, batch)
			for i := range out {
				out[i] = value
			}
			return this.OutputByName(portOut).PutSignals(signal.New(out))
		}),
	)
}

// newLoss ends the forward pass and starts the backward one: mean squared
// error against the targets, and its gradient sent straight back.
func newLoss(name string) (*component.Component, error) {
	return component.New(name,
		component.WithDescription("mean squared error"),
		component.WithInputs("a", "b"), // a: prediction, b: target
		component.WithOutputs(portLoss, gradOf("a")),
		component.WithActivationFunc(component.Sequential(
			component.RequireInputs("a", "b"),
			func(_ context.Context, this *component.Component) error {
				pred, err := this.InputByName("a").Signals().FirstAs[[]float64]()
				if err != nil {
					return err
				}
				target, err := this.InputByName("b").Signals().FirstAs[[]float64]()
				if err != nil {
					return err
				}
				n := float64(len(pred))
				loss, grad := 0.0, make([]float64, len(pred))
				for i := range pred {
					diff := pred[i] - target[i]
					loss += diff * diff / n
					grad[i] = 2 * diff / n
				}
				this.State().Set("pred", pred)
				if err := this.OutputByName(portLoss).PutSignals(signal.New(loss)); err != nil {
					return err
				}
				return this.OutputByName(gradOf("a")).PutSignals(signal.New(grad))
			},
		)),
	)
}

func add(a, b float64) float64 { return a + b }

func zip(a, b []float64, f func(a, b float64) float64) []float64 {
	out := make([]float64, len(a))
	for i := range a {
		out[i] = f(a[i], b[i])
	}
	return out
}

func mapf(a []float64, f func(float64) float64) []float64 {
	out := make([]float64, len(a))
	for i := range a {
		out[i] = f(a[i])
	}
	return out
}
