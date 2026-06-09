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

// This example is used in fmesh repo readme.md
func main() {
	fm := getMesh()

	// Generate graphs if needed
	err := internal.HandleGraphFlag(fm, true)
	if err != nil {
		fmt.Println("Failed to generate graph: ", err)
		os.Exit(1)
	}

	// Init inputs
	fm.Components().ByName("concat").InputByName("i1").PutSignals(signal.New("hello "))
	fm.Components().ByName("concat").InputByName("i2").PutSignals(signal.New("world !"))

	// Run the mesh
	_, err = fm.Run()

	// Check for errors
	if err != nil {
		fmt.Println("F-Mesh returned an error")
		os.Exit(1)
	}

	// Extract results
	result := fm.Components().ByName("case").OutputByName("res").Signals().FirstPayloadOrNil()
	fmt.Printf("Result is : %v", result)
}

func getMesh() *fmesh.FMesh {
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
		panic(fmt.Sprintf("failed to create concat component: %v", err))
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
		panic(fmt.Sprintf("failed to create case component: %v", err))
	}

	fm, err := fmesh.New("hello world",
		fmesh.WithErrorHandlingStrategy(fmesh.StopOnFirstErrorOrPanic),
		fmesh.WithCyclesLimit(10),
	)
	if err != nil {
		panic(fmt.Sprintf("failed to create mesh: %v", err))
	}

	if err := fm.AddComponents(concat, caseComp); err != nil {
		panic(fmt.Sprintf("failed to add components: %v", err))
	}

	if err := fm.Components().ByName("concat").Outputs().ByName("res").PipeTo(
		fm.Components().ByName("case").Inputs().ByName("i1"),
	); err != nil {
		panic(fmt.Sprintf("failed to pipe concat to case: %v", err))
	}

	return fm
}
