package main

import (
	"fmt"
	"os"

	"github.com/hovsep/fmesh"
	"github.com/hovsep/fmesh-examples/internal"
	"github.com/hovsep/fmesh/component"
	"github.com/hovsep/fmesh/signal"
)

func main() {
	fm, err := getMesh()
	if err != nil {
		fmt.Println("Failed to build mesh:", err)
		os.Exit(1)
	}

	if err := internal.HandleGraphFlag(fm, true); err != nil {
		fmt.Println("Failed to generate graph:", err)
		os.Exit(1)
	}

	f0, f1 := signal.New(0), signal.New(1)

	fm.ComponentByName("fibonacci number generator").Inputs().ByName("i_prev").PutSignals(f0)
	fm.ComponentByName("fibonacci number generator").Inputs().ByName("i_cur").PutSignals(f1)

	fmt.Println(f0.PayloadOrNil())
	fmt.Println(f1.PayloadOrNil())

	_, err = fm.Run()
	if err != nil {
		fmt.Println(err)
	}
}

func getMesh() (*fmesh.FMesh, error) {
	c1, err := component.New("fibonacci number generator",
		component.WithInputs("i_cur", "i_prev"),
		component.WithOutputs("o_cur", "o_prev"),
		component.WithActivationFunc(func(this *component.Component) error {
			cur := this.InputByName("i_cur").Signals().FirstPayloadOrDefault(0).(int)
			prev := this.InputByName("i_prev").Signals().FirstPayloadOrDefault(0).(int)

			next := cur + prev

			if next < 100 {
				fmt.Println(next)
				this.OutputByName("o_cur").PutSignals(signal.New(next))
				this.OutputByName("o_prev").PutSignals(signal.New(cur))
			}

			return nil
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("new component: %w", err)
	}

	if err := c1.Outputs().ByName("o_cur").PipeTo(c1.Inputs().ByName("i_cur")); err != nil {
		return nil, fmt.Errorf("pipe o_cur→i_cur: %w", err)
	}
	if err := c1.Outputs().ByName("o_prev").PipeTo(c1.Inputs().ByName("i_prev")); err != nil {
		return nil, fmt.Errorf("pipe o_prev→i_prev: %w", err)
	}

	fm, err := fmesh.New("fibonacci example")
	if err != nil {
		return nil, fmt.Errorf("new mesh: %w", err)
	}
	if err := fm.AddComponents(c1); err != nil {
		return nil, fmt.Errorf("add components: %w", err)
	}
	return fm, nil
}
