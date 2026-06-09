package main

import (
	"fmt"
	"os"

	"github.com/hovsep/fmesh"
	"github.com/hovsep/fmesh-examples/internal"
	"github.com/hovsep/fmesh/component"
	"github.com/hovsep/fmesh/port"
	"github.com/hovsep/fmesh/signal"
)

type factorizedNumber struct {
	Num     int
	Factors []any
}

// This example demonstrates the ability to nest meshes, where a component within a mesh
// can itself be another mesh. This nesting is recursive, allowing for an unlimited depth
// of nested meshes. Each nested mesh behaves as an individual component within the larger
// mesh, enabling complex and hierarchical workflows.
// In this example we implement prime factorization (which is core part of RSA encryption algorithm) as a sub-mesh
func main() {
	outerMesh := getMesh()

	// Set init data
	outerMesh.Components().
		ByName("starter").
		InputByName("in").
		PutSignals(signal.New(315))

	// Run outer mesh
	_, err := outerMesh.Run()

	if err != nil {
		fmt.Println(fmt.Errorf("outer mesh failed with error: %w", err))
	}

	// Read results
	outerMesh.Components().ByName("factorizer").OutputByName("out").Signals().ForEach(func(sig *signal.Signal) error {
		result := sig.PayloadOrNil().(factorizedNumber)
		fmt.Printf("Factors of number %d : %v \n", result.Num, result.Factors)
		return nil
	})
}

func getMesh() *fmesh.FMesh {
	starter, err := component.New("starter",
		component.WithDescription("This component just holds numbers we want to factorize"),
		component.WithInputs("in"),
		component.WithOutputs("out"),
		component.WithActivationFunc(func(this *component.Component) error {
			// Pure bypass
			return port.ForwardSignals(this.InputByName("in"), this.OutputByName("out"))
		}),
	)
	if err != nil {
		panic(fmt.Sprintf("failed to create starter: %v", err))
	}

	filter, err := component.New("filter",
		component.WithDescription("In this component we can do some optional filtering"),
		component.WithInputs("in"),
		component.WithOutputs("out", "log"),
		component.WithActivationFunc(func(this *component.Component) error {
			isValid := func(num int) bool {
				return num < 1000
			}

			return this.InputByName("in").Signals().ForEach(func(sig *signal.Signal) error {
				if isValid(sig.PayloadOrNil().(int)) {
					return this.OutputByName("out").PutSignals(sig)
				}
				return this.OutputByName("log").PutSignals(sig)
			})
		}),
	)
	if err != nil {
		panic(fmt.Sprintf("failed to create filter: %v", err))
	}

	logger, err := component.New("logger",
		component.WithDescription("Simple logger"),
		component.WithInputs("in"),
		component.WithActivationFunc(func(this *component.Component) error {
			return this.InputByName("in").Signals().ForEach(func(sig *signal.Signal) error {
				this.Logger().Println(sig.PayloadOrNil())
				return nil
			})
		}),
	)
	if err != nil {
		panic(fmt.Sprintf("failed to create logger: %v", err))
	}

	factorizer, err := component.New("factorizer",
		component.WithDescription("Prime factorization implemented as separate f-mesh"),
		component.WithInputs("in"),
		component.WithOutputs("out"),
		component.WithActivationFunc(func(this *component.Component) error {
			// This activation function has no implementation of the factorization algorithm,
			// it only runs another f-mesh to get results

			// Get nested mesh or meshes
			factorization := getPrimeFactorizationMesh()

			// As nested f-mesh processes 1 signal per run we run it in the loop per each number
			return this.InputByName("in").Signals().ForEach(func(sig *signal.Signal) error {

				// Set init data to nested mesh (pass signals from outer mesh to inner one)
				factorization.Components().ByName("starter").InputByName("in").PutSignals(sig)

				// Run nested mesh
				_, err := factorization.Run()

				if err != nil {
					return fmt.Errorf("inner mesh failed: %w", err)
				}

				// Get results from nested mesh
				factors, err := factorization.Components().ByName("results").OutputByName("factors").Signals().AllPayloads()
				if err != nil {
					return fmt.Errorf("failed to get factors: %w", err)
				}

				// Pass results to outer mesh
				number := sig.PayloadOrNil().(int)
				return this.OutputByName("out").PutSignals(signal.New(factorizedNumber{
					Num:     number,
					Factors: factors,
				}))
			})
		}),
	)
	if err != nil {
		panic(fmt.Sprintf("failed to create factorizer: %v", err))
	}

	// Setup pipes
	if err := starter.OutputByName("out").PipeTo(filter.InputByName("in")); err != nil {
		panic(fmt.Sprintf("failed to pipe starter to filter: %v", err))
	}
	if err := filter.OutputByName("log").PipeTo(logger.InputByName("in")); err != nil {
		panic(fmt.Sprintf("failed to pipe filter to logger: %v", err))
	}
	if err := filter.OutputByName("out").PipeTo(factorizer.InputByName("in")); err != nil {
		panic(fmt.Sprintf("failed to pipe filter to factorizer: %v", err))
	}

	// Build the mesh
	outerMesh, err := fmesh.New("outer")
	if err != nil {
		panic(fmt.Sprintf("failed to create outer mesh: %v", err))
	}
	if err := outerMesh.AddComponents(starter, filter, logger, factorizer); err != nil {
		panic(fmt.Sprintf("failed to add components: %v", err))
	}

	// Generate graphs if needed
	err = internal.HandleGraphFlag(outerMesh, true)
	if err != nil {
		fmt.Println("Failed to generate graph: ", err)
		os.Exit(1)
	}

	return outerMesh
}

func getPrimeFactorizationMesh() *fmesh.FMesh {
	starter, err := component.New("starter",
		component.WithDescription("Load the number to be factorized"),
		component.WithInputs("in"),
		component.WithOutputs("out"),
		component.WithActivationFunc(func(this *component.Component) error {
			// For simplicity this f-mesh processes only one signal per run, so ignore all except the first
			return this.OutputByName("out").PutSignals(this.InputByName("in").Signals().First())
		}),
	)
	if err != nil {
		panic(fmt.Sprintf("failed to create starter: %v", err))
	}

	d2, err := component.New("d2",
		component.WithDescription("Divide by smallest prime (2) to handle even factors"),
		component.WithInputs("in"),
		component.WithOutputs("out", "factor"),
		component.WithActivationFunc(func(this *component.Component) error {
			number := this.InputByName("in").Signals().FirstPayloadOrNil().(int)

			for number%2 == 0 {
				this.OutputByName("factor").PutSignals(signal.New(2))
				number /= 2
			}

			return this.OutputByName("out").PutSignals(signal.New(number))
		}),
	)
	if err != nil {
		panic(fmt.Sprintf("failed to create d2: %v", err))
	}

	dodd, err := component.New("dodd",
		component.WithDescription("Divide by odd primes starting from 3"),
		component.WithInputs("in"),
		component.WithOutputs("out", "factor"),
		component.WithActivationFunc(func(this *component.Component) error {
			number := this.InputByName("in").Signals().FirstPayloadOrNil().(int)
			divisor := 3
			for number > 1 && divisor*divisor <= number {
				for number%divisor == 0 {
					this.OutputByName("factor").PutSignals(signal.New(divisor))
					number /= divisor
				}
				divisor += 2
			}
			return this.OutputByName("out").PutSignals(signal.New(number))
		}),
	)
	if err != nil {
		panic(fmt.Sprintf("failed to create dodd: %v", err))
	}

	finalPrime, err := component.New("final_prime",
		component.WithDescription("Store the last remaining prime factor, if any"),
		component.WithInputs("in"),
		component.WithOutputs("factor"),
		component.WithActivationFunc(func(this *component.Component) error {
			number := this.InputByName("in").Signals().FirstPayloadOrNil().(int)
			if number > 1 {
				return this.OutputByName("factor").PutSignals(signal.New(number))
			}
			return nil
		}),
	)
	if err != nil {
		panic(fmt.Sprintf("failed to create finalPrime: %v", err))
	}

	results, err := component.New("results",
		component.WithDescription("factors holder"),
		component.WithInputs("factor"),
		component.WithOutputs("factors"),
		component.WithActivationFunc(func(this *component.Component) error {
			return port.ForwardSignals(this.InputByName("factor"), this.OutputByName("factors"))
		}),
	)
	if err != nil {
		panic(fmt.Sprintf("failed to create results: %v", err))
	}

	// Main pipeline starter->d2->dodd->finalPrime
	if err := starter.OutputByName("out").PipeTo(d2.InputByName("in")); err != nil {
		panic(fmt.Sprintf("failed to pipe starter to d2: %v", err))
	}
	if err := d2.OutputByName("out").PipeTo(dodd.InputByName("in")); err != nil {
		panic(fmt.Sprintf("failed to pipe d2 to dodd: %v", err))
	}
	if err := dodd.OutputByName("out").PipeTo(finalPrime.InputByName("in")); err != nil {
		panic(fmt.Sprintf("failed to pipe dodd to finalPrime: %v", err))
	}

	// All found factors are accumulated in results
	if err := d2.OutputByName("factor").PipeTo(results.InputByName("factor")); err != nil {
		panic(fmt.Sprintf("failed to pipe d2 factor: %v", err))
	}
	if err := dodd.OutputByName("factor").PipeTo(results.InputByName("factor")); err != nil {
		panic(fmt.Sprintf("failed to pipe dodd factor: %v", err))
	}
	if err := finalPrime.OutputByName("factor").PipeTo(results.InputByName("factor")); err != nil {
		panic(fmt.Sprintf("failed to pipe finalPrime factor: %v", err))
	}

	algoMesh, err := fmesh.New("prime factors algo",
		fmesh.WithDescription("Pass single signal to starter"),
	)
	if err != nil {
		panic(fmt.Sprintf("failed to create algo mesh: %v", err))
	}
	if err := algoMesh.AddComponents(starter, d2, dodd, finalPrime, results); err != nil {
		panic(fmt.Sprintf("failed to add components: %v", err))
	}

	// Generate graphs if needed
	err = internal.HandleGraphFlag(algoMesh, false)
	if err != nil {
		fmt.Println("Failed to generate graph: ", err)
		os.Exit(1)
	}

	return algoMesh
}
