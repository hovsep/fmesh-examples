package main

import (
	"fmt"
	"os"
	"strings"

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

	fm.Components().ByName("concat").InputByName("i1").PutSignals(signal.New("hello "))
	fm.Components().ByName("concat").InputByName("i2").PutSignals(signal.New("world !"))

	_, err = fm.Run()
	if err != nil {
		fmt.Println("F-Mesh returned an error")
		os.Exit(1)
	}

	result := fm.Components().ByName("case").OutputByName("res").Signals().FirstPayloadOrNil()
	fmt.Printf("Result is : %v", result)
}

func getMesh() (*fmesh.FMesh, error) {
	concat, err := component.New("concat",
		component.WithInputs("i1", "i2"),
		component.WithOutputs("res"),
		component.WithActivationFunc(func(this *component.Component) error {
			word1 := this.InputByName("i1").Signals().FirstPayloadOrDefault("").(string)
			word2 := this.InputByName("i2").Signals().FirstPayloadOrDefault("").(string)
			this.OutputByName("res").PutSignals(signal.New(word1 + word2))
			return nil
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("concat component: %w", err)
	}

	caseComp, err := component.New("case",
		component.WithInputs("i1"),
		component.WithOutputs("res"),
		component.WithActivationFunc(func(this *component.Component) error {
			inputString := this.InputByName("i1").Signals().FirstPayloadOrDefault("").(string)
			this.OutputByName("res").PutSignals(signal.New(strings.ToTitle(inputString)))
			return nil
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("case component: %w", err)
	}

	fm, err := fmesh.New("hello world",
		fmesh.WithErrorHandlingStrategy(fmesh.StopOnFirstErrorOrPanic),
		fmesh.WithCyclesLimit(10),
	)
	if err != nil {
		return nil, fmt.Errorf("new mesh: %w", err)
	}

	if err := fm.AddComponents(concat, caseComp); err != nil {
		return nil, fmt.Errorf("add components: %w", err)
	}

	if err := fm.Components().ByName("concat").Outputs().ByName("res").PipeTo(
		fm.Components().ByName("case").Inputs().ByName("i1"),
	); err != nil {
		return nil, fmt.Errorf("pipe concat→case: %w", err)
	}

	return fm, nil
}
