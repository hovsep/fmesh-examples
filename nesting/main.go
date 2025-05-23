package main

import (
	"fmt"
	"github.com/hovsep/fmesh"
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
	starter := component.New("starter").
		WithDescription("This component just holds numbers we want to factorize").
		WithInputs("in"). // Single port is enough, as it can hold any number of signals (as long as they fit into1 memory)
		WithOutputs("out").
		WithActivationFunc(func(this *component.Component) error {
			// Pure bypass
			return port.ForwardSignals(this.InputByName("in"), this.OutputByName("out"))
		})

	filter := component.New("filter").
		WithDescription("In this component we can do some optional filtering").
		WithInputs("in").
		WithOutputs("out", "log").
		WithActivationFunc(func(this *component.Component) error {
			isValid := func(num int) bool {
				return num < 1000
			}

			for _, sig := range this.InputByName("in").AllSignalsOrNil() {
				if isValid(sig.PayloadOrNil().(int)) {
					this.OutputByName("out").PutSignals(sig)
				} else {
					this.OutputByName("log").PutSignals(sig)
				}
			}
			return nil
		})

	logger := component.New("logger").
		WithDescription("Simple logger").
		WithInputs("in").
		WithActivationFunc(func(this *component.Component) error {
			for _, sig := range this.InputByName("in").AllSignalsOrNil() {
				this.Logger().Println(sig.PayloadOrNil())
			}
			return nil
		})

	factorizer := component.New("factorizer").
		WithDescription("Prime factorization implemented as separate f-mesh").
		WithInputs("in").
		WithOutputs("out").
		WithActivationFunc(func(this *component.Component) error {
			// This activation function has no implementation of factorization algorithm,
			// it only runs another f-mesh to get results

			// Get nested mesh or meshes
			factorization := getPrimeFactorizationMesh()

			// As nested f-mesh processes 1 signal per run we run it in the loop per each number
			for _, numSig := range this.InputByName("in").AllSignalsOrNil() {
				// Set init data to nested mesh (pass signals from outer mesh to inner one)
				factorization.Components().ByName("starter").InputByName("in").PutSignals(numSig)

				// Run nested mesh
				_, err := factorization.Run()

				if err != nil {
					return fmt.Errorf("inner mesh failed: %w", err)
				}

				// Get results from nested mesh
				factors, err := factorization.Components().ByName("results").OutputByName("factors").AllSignalsPayloads()
				if err != nil {
					return fmt.Errorf("failed to get factors: %w", err)
				}

				// Pass results to outer mesh
				number := numSig.PayloadOrNil().(int)
				this.OutputByName("out").PutSignals(signal.New(factorizedNumber{
					Num:     number,
					Factors: factors,
				}))
			}

			return nil
		})

	// Setup pipes
	starter.OutputByName("out").PipeTo(filter.InputByName("in"))
	filter.OutputByName("log").PipeTo(logger.InputByName("in"))
	filter.OutputByName("out").PipeTo(factorizer.InputByName("in"))

	// Build the mesh
	outerMesh := fmesh.New("outer").WithComponents(starter, filter, logger, factorizer)

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
	for _, resSig := range outerMesh.Components().ByName("factorizer").OutputByName("out").AllSignalsOrNil() {
		result := resSig.PayloadOrNil().(factorizedNumber)
		fmt.Printf("Factors of number %d : %v \n", result.Num, result.Factors)
	}
}

func getPrimeFactorizationMesh() *fmesh.FMesh {
	starter := component.New("starter").
		WithDescription("Load the number to be factorized").
		WithInputs("in").
		WithOutputs("out").
		WithActivationFunc(func(this *component.Component) error {
			// For simplicity this f-mesh processes only one signal per run, so ignore all except first
			this.OutputByName("out").PutSignals(this.InputByName("in").Buffer().First())
			return nil
		})

	d2 := component.New("d2").
		WithDescription("Divide by smallest prime (2) to handle even factors").
		WithInputs("in").
		WithOutputs("out", "factor").
		WithActivationFunc(func(this *component.Component) error {
			number := this.InputByName("in").FirstSignalPayloadOrNil().(int)

			for number%2 == 0 {
				this.OutputByName("factor").PutSignals(signal.New(2))
				number /= 2
			}

			this.OutputByName("out").PutSignals(signal.New(number))
			return nil
		})

	dodd := component.New("dodd").
		WithDescription("Divide by odd primes starting from 3").
		WithInputs("in").
		WithOutputs("out", "factor").
		WithActivationFunc(func(this *component.Component) error {
			number := this.InputByName("in").FirstSignalPayloadOrNil().(int)
			divisor := 3
			for number > 1 && divisor*divisor <= number {
				for number%divisor == 0 {
					this.OutputByName("factor").PutSignals(signal.New(divisor))
					number /= divisor
				}
				divisor += 2
			}
			this.OutputByName("out").PutSignals(signal.New(number))
			return nil
		})

	finalPrime := component.New("final_prime").
		WithDescription("Store the last remaining prime factor, if any").
		WithInputs("in").
		WithOutputs("factor").
		WithActivationFunc(func(this *component.Component) error {
			number := this.InputByName("in").FirstSignalPayloadOrNil().(int)
			if number > 1 {
				this.OutputByName("factor").PutSignals(signal.New(number))
			}
			return nil
		})

	results := component.New("results").
		WithDescription("factors holder").
		WithInputs("factor").
		WithOutputs("factors").
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
		WithComponents(starter, d2, dodd, finalPrime, results)
}
