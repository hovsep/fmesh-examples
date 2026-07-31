package main

import (
	"context"
	"fmt"
	"os"

	"github.com/hovsep/fmesh"
	"github.com/hovsep/fmesh-examples/internal"
	"github.com/hovsep/fmesh/component"
	"github.com/hovsep/fmesh/meta"
	"github.com/hovsep/fmesh/signal"
)

const (
	portIn = "in"
)

func main() {
	fmt.Println("=== Song Filter Demo ===")
	fmt.Println("This example demonstrates signal filtering by labels.")
	fmt.Println("Signals matching a disallowed label set are dropped; all others pass through.")
	fmt.Println()

	fm, err := getMesh()
	if err != nil {
		fmt.Println("Failed to build mesh:", err)
		os.Exit(1)
	}

	if err := internal.HandleGraphFlag(fm, true); err != nil {
		fmt.Println("Failed to generate graph:", err)
		os.Exit(1)
	}

	signalsToFilter := getSignals()
	fm.ComponentByName("pop-filter").InputByName(portIn).PutSignalGroups(signalsToFilter)

	_, err = fm.Run(context.Background())
	if err != nil {
		fmt.Println("Pipeline finished with error:", err)
		os.Exit(1)
	}

	fmt.Println("=== Song Filter Demo Complete ===")
}

func getMesh() (*fmesh.FMesh, error) {
	filter, err := getFilter("pop-filter", meta.NewLabels().Set("genre", "pop"))
	if err != nil {
		return nil, fmt.Errorf("filter: %w", err)
	}

	printer1, err := getPrinter("dropped-printer")
	if err != nil {
		return nil, fmt.Errorf("printer1: %w", err)
	}

	printer2, err := getPrinter("passed-printer")
	if err != nil {
		return nil, fmt.Errorf("printer2: %w", err)
	}

	if err := filter.OutputByName("dropped").PipeTo(printer1.InputByName(portIn)); err != nil {
		return nil, fmt.Errorf("pipe filter→printer1: %w", err)
	}
	if err := filter.OutputByName("passed").PipeTo(printer2.InputByName(portIn)); err != nil {
		return nil, fmt.Errorf("pipe filter→printer2: %w", err)
	}

	fm, err := fmesh.New("demo-filter",
		fmesh.WithErrorHandlingStrategy(fmesh.StopOnFirstErrorOrPanic),
	)
	if err != nil {
		return nil, fmt.Errorf("new mesh: %w", err)
	}
	if err := fm.AddComponents(filter, printer1, printer2); err != nil {
		return nil, fmt.Errorf("add components: %w", err)
	}

	return fm, nil
}

func getPrinter(name string) (*component.Component, error) {
	return component.New(name,
		component.WithDescription("Simple stdout printer"),
		component.WithInputs(portIn),
		component.WithActivationFunc(func(_ context.Context, this *component.Component) error {
			return this.InputByName(portIn).Signals().ForEach(func(sig *signal.Signal) error {
				fmt.Printf("  [%s] %v\n", this.Name(), sig.Payload())
				return nil
			})
		}),
	)
}

func getFilter(name string, disallowedLabels *meta.Labels) (*component.Component, error) {
	return component.New(name,
		component.WithDescription("Simple filter"),
		component.WithInputs(portIn),
		component.WithOutputs("dropped", "passed"),
		component.WithActivationFunc(func(_ context.Context, this *component.Component) error {
			return this.InputByName(portIn).Signals().ForEach(func(sig *signal.Signal) error {
				var why string
				disallowedLabels.ForEach(func(k, v string) error {
					if sig.Labels().ValueIs(k, v) {
						why = fmt.Sprintf("%s=%s matches the filter rule", k, v)
					}
					return nil
				})
				if why != "" {
					dropped := sig.MapPayload(func(p any) any {
						return fmt.Sprintf("DROPPED: '%v' excluded because %s", p, why)
					})
					return this.OutputByName("dropped").PutSignals(dropped)
				}
				return this.OutputByName("passed").PutSignals(sig)
			})
		}),
	)
}

func getSignals() *signal.Group {
	return signal.NewGroup().With(
		signal.New("Justice").WithLabels(map[string]string{
			"genre":  "pop",
			"artist": "Justin Bieber",
			"year":   "2021",
		}),
		signal.New("Dysania").WithLabels(map[string]string{
			"genre":  "rock",
			"artist": "Elita",
			"year":   "2023",
		}),
		signal.New("After Hours").WithLabels(map[string]string{
			"genre":  "pop",
			"artist": "The Weekend",
			"year":   "2020",
		}),
		signal.New("Random Access Memories").WithLabels(map[string]string{
			"genre":  "electronic",
			"artist": "Daft Punk",
			"year":   "2013",
		}),
		signal.New("Evermore").WithLabels(map[string]string{
			"genre":  "pop",
			"artist": "Taylor Swift",
			"year":   "2020",
		}),
		signal.New("1989").WithLabels(map[string]string{
			"genre":  "pop",
			"artist": "Taylor Swift",
			"year":   "2014",
		}),
		signal.New("To Pimp a Butterfly").WithLabels(map[string]string{
			"genre":  "hip-hop",
			"artist": "Kendrick Lamar",
			"year":   "2015",
		}),
		signal.New("Ghost Stories").WithLabels(map[string]string{
			"genre":  "alternative",
			"artist": "Coldplay",
			"year":   "2014",
		}),
		signal.New("Future Nostalgia").WithLabels(map[string]string{
			"genre":  "pop",
			"artist": "Dua Lipa",
			"year":   "2020",
		}))
}
