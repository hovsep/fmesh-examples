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

	// Generate graphs if needed
	err := internal.HandleGraphFlag(outerMesh)
	if err != nil {
		fmt.Println("Failed to generate graph: ", err)
		os.Exit(1)
	}

	// Set init data
	outerMesh.Components().
		ByName("starter").
		InputByName("in").
		PutSignals(signal.New(315))

	// Run outer mesh
	_, err = outerMesh.Run()

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
	starter := component.New("starter").
		WithDescription("This component just holds numbers we want to factorize").
		AddInputs("in"). // A single port is enough, as it can hold any number of signals (as long as they fit into1 memory)
		AddOutputs("out").
		WithActivationFunc(func(this *component.Component) error {
			// Pure bypass
			return port.ForwardSignals(this.InputByName("in"), this.OutputByName("out"))
		})

	filter := component.New("filter").
		WithDescription("In this component we can do some optional filtering").
		AddInputs("in").
		AddOutputs("out", "log").
		WithActivationFunc(func(this *component.Component) error {
			isValid := func(num int) bool {
				return num < 1000
			}

			return this.InputByName("in").Signals().ForEach(func(sig *signal.Signal) error {
				if isValid(sig.PayloadOrNil().(int)) {
					return this.OutputByName("out").PutSignals(sig).ChainableErr()
				}
				return this.OutputByName("log").PutSignals(sig).ChainableErr()
			}).ChainableErr()
		})

	logger := component.New("logger").
		WithDescription("Simple logger").
		AddInputs("in").
		WithActivationFunc(func(this *component.Component) error {
			return this.InputByName("in").Signals().ForEach(func(sig *signal.Signal) error {
				this.Logger().Println(sig.PayloadOrNil())
				return nil
			}).ChainableErr()
		})

	factorizer := component.New("factorizer").
		WithDescription("Prime factorization implemented as separate f-mesh").
		AddInputs("in").
		AddOutputs("out").
		WithActivationFunc(func(this *component.Component) error {
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
				})).ChainableErr()
			}).ChainableErr()
		})

	// Setup pipes
	starter.OutputByName("out").PipeTo(filter.InputByName("in"))
	filter.OutputByName("log").PipeTo(logger.InputByName("in"))
	filter.OutputByName("out").PipeTo(factorizer.InputByName("in"))

	// Build the mesh
	return fmesh.New("outer").AddComponents(starter, filter, logger, factorizer)
}

func getPrimeFactorizationMesh() *fmesh.FMesh {
	starter := component.New("starter").
		WithDescription("Load the number to be factorized").
		AddInputs("in").
		AddOutputs("out").
		WithActivationFunc(func(this *component.Component) error {
			// For simplicity this f-mesh processes only one signal per run, so ignore all except the first
			return this.OutputByName("out").PutSignals(this.InputByName("in").Signals().First()).ChainableErr()
		})

	d2 := component.New("d2").
		WithDescription("Divide by smallest prime (2) to handle even factors").
		AddInputs("in").
		AddOutputs("out", "factor").
		WithActivationFunc(func(this *component.Component) error {
			number := this.InputByName("in").Signals().FirstPayloadOrNil().(int)

			for number%2 == 0 {
				this.OutputByName("factor").PutSignals(signal.New(2))
				number /= 2
			}

			return this.OutputByName("out").PutSignals(signal.New(number)).ChainableErr()
		})

	dodd := component.New("dodd").
		WithDescription("Divide by odd primes starting from 3").
		AddInputs("in").
		AddOutputs("out", "factor").
		WithActivationFunc(func(this *component.Component) error {
			number := this.InputByName("in").Signals().FirstPayloadOrNil().(int)
			divisor := 3
			for number > 1 && divisor*divisor <= number {
				for number%divisor == 0 {
					this.OutputByName("factor").PutSignals(signal.New(divisor))
					number /= divisor
				}
				divisor += 2
			}
			return this.OutputByName("out").PutSignals(signal.New(number)).ChainableErr()
		})

	finalPrime := component.New("final_prime").
		WithDescription("Store the last remaining prime factor, if any").
		AddInputs("in").
		AddOutputs("factor").
		WithActivationFunc(func(this *component.Component) error {
			number := this.InputByName("in").Signals().FirstPayloadOrNil().(int)
			if number > 1 {
				return this.OutputByName("factor").PutSignals(signal.New(number)).ChainableErr()
			}
			return nil
		})

	results := component.New("results").
		WithDescription("factors holder").
		AddInputs("factor").
		AddOutputs("factors").
		WithActivationFunc(func(this *component.Component) error {
			return port.ForwardSignals(this.InputByName("factor"), this.OutputByName("factors"))
		})

	// Main pipeline starter->d2->dodd->finalPrime
	starter.OutputByName("out").PipeTo(d2.InputByName("in"))
	d2.OutputByName("out").PipeTo(dodd.InputByName("in"))
	dodd.OutputByName("out").PipeTo(finalPrime.InputByName("in"))

	// All found factors are accumulated in results
	d2.OutputByName("factor").PipeTo(results.InputByName("factor"))
	dodd.OutputByName("factor").PipeTo(results.InputByName("factor"))
	finalPrime.OutputByName("factor").PipeTo(results.InputByName("factor"))

	return fmesh.New("prime factors algo").
		WithDescription("Pass single signal to starter").
		AddComponents(starter, d2, dodd, finalPrime, results)
}
