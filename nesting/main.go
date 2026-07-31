package main

import (
	"context"
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

func main() {
	outerMesh, err := getMesh()
	if err != nil {
		fmt.Println("Failed to build mesh:", err)
		os.Exit(1)
	}

	outerMesh.Components().ByName("starter").InputByName("in").PutSignals(signal.New(315))

	if _, err := outerMesh.Run(context.Background()); err != nil {
		fmt.Println("outer mesh failed with error:", err)
		os.Exit(1)
	}

	outerMesh.Components().ByName("factorizer").OutputByName("out").Signals().ForEach(func(sig *signal.Signal) error {
		result := sig.Payload().(factorizedNumber)
		fmt.Printf("Factors of number %d : %v \n", result.Num, result.Factors)
		return nil
	})
}

func getMesh() (*fmesh.FMesh, error) {
	starter, err := component.New("starter",
		component.WithDescription("This component just holds numbers we want to factorize"),
		component.WithInputs("in"),
		component.WithOutputs("out"),
		component.WithActivationFunc(func(ctx context.Context, this *component.Component) error {
			return port.ForwardSignals(ctx, this.InputByName("in"), this.OutputByName("out"))
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("starter: %w", err)
	}

	filter, err := component.New("filter",
		component.WithDescription("In this component we can do some optional filtering"),
		component.WithInputs("in"),
		component.WithOutputs("out", "log"),
		component.WithActivationFunc(func(_ context.Context, this *component.Component) error {
			isValid := func(num int) bool { return num < 1000 }
			return this.InputByName("in").Signals().ForEach(func(sig *signal.Signal) error {
				if isValid(sig.Payload().(int)) {
					return this.OutputByName("out").PutSignals(sig)
				}
				return this.OutputByName("log").PutSignals(sig)
			})
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("filter: %w", err)
	}

	logger, err := component.New("logger",
		component.WithDescription("Simple logger"),
		component.WithInputs("in"),
		component.WithActivationFunc(func(_ context.Context, this *component.Component) error {
			return this.InputByName("in").Signals().ForEach(func(sig *signal.Signal) error {
				this.Logger().Println(sig.Payload())
				return nil
			})
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("logger: %w", err)
	}

	factorizer, err := component.New("factorizer",
		component.WithDescription("Prime factorization implemented as separate f-mesh"),
		component.WithInputs("in"),
		component.WithOutputs("out"),
		component.WithActivationFunc(func(_ context.Context, this *component.Component) error {
			factorization, err := getPrimeFactorizationMesh()
			if err != nil {
				return fmt.Errorf("build sub-mesh: %w", err)
			}

			return this.InputByName("in").Signals().ForEach(func(sig *signal.Signal) error {
				factorization.Components().ByName("starter").InputByName("in").PutSignals(sig)
				_, err := factorization.Run(context.Background())
				if err != nil {
					return fmt.Errorf("inner mesh failed: %w", err)
				}
				factors := factorization.Components().ByName("results").OutputByName("factors").Signals().AllPayloads()
				number := sig.Payload().(int)
				return this.OutputByName("out").PutSignals(signal.New(factorizedNumber{
					Num:     number,
					Factors: factors,
				}))
			})
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("factorizer: %w", err)
	}

	if err := starter.OutputByName("out").PipeTo(filter.InputByName("in")); err != nil {
		return nil, fmt.Errorf("pipe starter→filter: %w", err)
	}
	if err := filter.OutputByName("log").PipeTo(logger.InputByName("in")); err != nil {
		return nil, fmt.Errorf("pipe filter→logger: %w", err)
	}
	if err := filter.OutputByName("out").PipeTo(factorizer.InputByName("in")); err != nil {
		return nil, fmt.Errorf("pipe filter→factorizer: %w", err)
	}

	outerMesh, err := fmesh.New("outer",
		fmesh.WithErrorHandlingStrategy(fmesh.StopOnFirstErrorOrPanic),
	)
	if err != nil {
		return nil, fmt.Errorf("new outer mesh: %w", err)
	}
	if err := outerMesh.AddComponents(starter, filter, logger, factorizer); err != nil {
		return nil, fmt.Errorf("add components: %w", err)
	}

	if err := internal.HandleGraphFlag(outerMesh, true); err != nil {
		return nil, fmt.Errorf("handle graph flag: %w", err)
	}

	return outerMesh, nil
}

func getPrimeFactorizationMesh() (*fmesh.FMesh, error) {
	starter, err := component.New("starter",
		component.WithDescription("Load the number to be factorized"),
		component.WithInputs("in"),
		component.WithOutputs("out"),
		component.WithActivationFunc(func(_ context.Context, this *component.Component) error {
			return this.OutputByName("out").PutSignals(this.InputByName("in").Signals().First())
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("starter: %w", err)
	}

	d2, err := component.New("d2",
		component.WithDescription("Divide by smallest prime (2) to handle even factors"),
		component.WithInputs("in"),
		component.WithOutputs("out", "factor"),
		component.WithActivationFunc(func(_ context.Context, this *component.Component) error {
			number := this.InputByName("in").Signals().FirstPayloadOrNil().(int)
			for number%2 == 0 {
				this.OutputByName("factor").PutSignals(signal.New(2))
				number /= 2
			}
			return this.OutputByName("out").PutSignals(signal.New(number))
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("d2: %w", err)
	}

	dodd, err := component.New("dodd",
		component.WithDescription("Divide by odd primes starting from 3"),
		component.WithInputs("in"),
		component.WithOutputs("out", "factor"),
		component.WithActivationFunc(func(_ context.Context, this *component.Component) error {
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
		return nil, fmt.Errorf("dodd: %w", err)
	}

	finalPrime, err := component.New("final_prime",
		component.WithDescription("Store the last remaining prime factor, if any"),
		component.WithInputs("in"),
		component.WithOutputs("factor"),
		component.WithActivationFunc(func(_ context.Context, this *component.Component) error {
			number := this.InputByName("in").Signals().FirstPayloadOrNil().(int)
			if number > 1 {
				return this.OutputByName("factor").PutSignals(signal.New(number))
			}
			return nil
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("finalPrime: %w", err)
	}

	results, err := component.New("results",
		component.WithDescription("factors holder"),
		component.WithInputs("factor"),
		component.WithOutputs("factors"),
		component.WithActivationFunc(func(ctx context.Context, this *component.Component) error {
			return port.ForwardSignals(ctx, this.InputByName("factor"), this.OutputByName("factors"))
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("results: %w", err)
	}

	if err := starter.OutputByName("out").PipeTo(d2.InputByName("in")); err != nil {
		return nil, fmt.Errorf("pipe starter→d2: %w", err)
	}
	if err := d2.OutputByName("out").PipeTo(dodd.InputByName("in")); err != nil {
		return nil, fmt.Errorf("pipe d2→dodd: %w", err)
	}
	if err := dodd.OutputByName("out").PipeTo(finalPrime.InputByName("in")); err != nil {
		return nil, fmt.Errorf("pipe dodd→finalPrime: %w", err)
	}
	if err := d2.OutputByName("factor").PipeTo(results.InputByName("factor")); err != nil {
		return nil, fmt.Errorf("pipe d2→results: %w", err)
	}
	if err := dodd.OutputByName("factor").PipeTo(results.InputByName("factor")); err != nil {
		return nil, fmt.Errorf("pipe dodd→results: %w", err)
	}
	if err := finalPrime.OutputByName("factor").PipeTo(results.InputByName("factor")); err != nil {
		return nil, fmt.Errorf("pipe finalPrime→results: %w", err)
	}

	algoMesh, err := fmesh.New("prime factors algo",
		fmesh.WithDescription("Pass single signal to starter"),
		fmesh.WithErrorHandlingStrategy(fmesh.StopOnFirstErrorOrPanic),
	)
	if err != nil {
		return nil, fmt.Errorf("new algo mesh: %w", err)
	}
	if err := algoMesh.AddComponents(starter, d2, dodd, finalPrime, results); err != nil {
		return nil, fmt.Errorf("add algo components: %w", err)
	}

	if err := internal.HandleGraphFlag(algoMesh, false); err != nil {
		return nil, fmt.Errorf("handle graph flag: %w", err)
	}

	return algoMesh, nil
}
