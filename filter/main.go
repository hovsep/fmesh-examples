package main

import (
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

// This demo demonstrates F-Mesh's signal filtering and routing capabilities.
// It showcases how components can filter and route signals based on conditions
func main() {
	fm := getMesh()

	// Generate graphs if needed
	err := internal.HandleGraphFlag(fm, true)
	if err != nil {
		fmt.Println("Failed to generate graph: ", err)
		os.Exit(1)
	}

	// Init with data
	signalsToFilter := getSignals()
	fm.ComponentByName("pop-filter").InputByName(portIn).PutSignalGroups(signalsToFilter)

	_, err = fm.Run()
	if err != nil {
		fmt.Println("Pipeline finished with error:", err)
		os.Exit(1)
	}

	fmt.Println("Filtering finished successfully")
}

func getMesh() *fmesh.FMesh {
	filter := getFilter("pop-filter", meta.NewLabels().Set("genre", "pop"))
	printer1 := getPrinter("dropped-printer")
	printer2 := getPrinter("passed-printer")

	if err := filter.OutputByName("dropped").PipeTo(printer1.InputByName(portIn)); err != nil {
		panic(fmt.Sprintf("failed to pipe filter to printer1: %v", err))
	}
	if err := filter.OutputByName("passed").PipeTo(printer2.InputByName(portIn)); err != nil {
		panic(fmt.Sprintf("failed to pipe filter to printer2: %v", err))
	}

	fm, err := fmesh.New("demo-filter")
	if err != nil {
		panic(fmt.Sprintf("failed to create mesh: %v", err))
	}
	if err := fm.AddComponents(filter, printer1, printer2); err != nil {
		panic(fmt.Sprintf("failed to add components: %v", err))
	}

	return fm
}

func getPrinter(name string) *component.Component {
	c, err := component.New(name,
		component.WithDescription("Simple stdout printer"),
		component.WithInputs(portIn),
		component.WithActivationFunc(func(this *component.Component) error {
			return this.InputByName(portIn).Signals().ForEach(func(sig *signal.Signal) error {
				fmt.Printf("%s: %v \n", this.Name(), sig.PayloadOrDefault("no payload"))
				return nil
			})
		}),
	)
	if err != nil {
		panic(fmt.Sprintf("failed to create printer component: %v", err))
	}
	return c
}

func getFilter(name string, disallowedLabels *meta.Labels) *component.Component {
	c, err := component.New(name,
		component.WithDescription("Simple filter"),
		component.WithInputs(portIn),
		component.WithOutputs("dropped", "passed"),
		component.WithActivationFunc(func(this *component.Component) error {
			return this.InputByName(portIn).Signals().ForEach(func(sig *signal.Signal) error {
				if sig.Labels().HasAnyFrom(disallowedLabels) {
					return this.OutputByName("dropped").PutSignals(sig)
				}

				return this.OutputByName("passed").PutSignals(sig)
			})
		}),
	)
	if err != nil {
		panic(fmt.Sprintf("failed to create filter component: %v", err))
	}
	return c
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
