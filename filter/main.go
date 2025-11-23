package main

import (
	"fmt"
	"os"

	"github.com/hovsep/fmesh"
	"github.com/hovsep/fmesh-examples/internal"
	"github.com/hovsep/fmesh/component"
	"github.com/hovsep/fmesh/labels"
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
	err := internal.HandleGraphFlag(fm)
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
	filter := getFilter("pop-filter", labels.NewCollection().Add("genre", "pop"))
	printer1 := getPrinter("dropped-printer")
	printer2 := getPrinter("passed-printer")

	filter.OutputByName("dropped").PipeTo(printer1.InputByName(portIn))
	filter.OutputByName("passed").PipeTo(printer2.InputByName(portIn))

	return fmesh.New("demo-filter").
		AddComponents(filter, printer1, printer2)
}

func getPrinter(name string) *component.Component {
	return component.New(name).
		WithDescription("Simple stdout printer").
		AddInputs(portIn).
		WithActivationFunc(func(this *component.Component) error {
			return this.InputByName(portIn).Signals().ForEach(func(sig *signal.Signal) error {
				fmt.Printf("%s: %v \n", this.Name(), sig.PayloadOrDefault("no payload"))
				return nil
			}).ChainableErr()
		})
}

func getFilter(name string, disallowedLabels *labels.Collection) *component.Component {
	return component.New(name).
		WithDescription("Simple filter").
		AddInputs(portIn).
		AddOutputs("dropped", "passed").
		WithActivationFunc(func(this *component.Component) error {
			return this.InputByName(portIn).Signals().ForEach(func(sig *signal.Signal) error {
				if sig.Labels().HasAnyFrom(disallowedLabels) {
					return this.OutputByName("dropped").PutSignals(sig).ChainableErr()
				}

				return this.OutputByName("passed").PutSignals(sig).ChainableErr()
			}).ChainableErr()
		})
}

func getSignals() *signal.Group {
	return signal.NewGroup().Add(
		signal.New("Justice").AddLabels(labels.Map{
			"genre":  "pop",
			"artist": "Justin Bieber",
			"year":   "2021",
		}),
		signal.New("Dysania").AddLabels(labels.Map{
			"genre":  "rock",
			"artist": "Elita",
			"year":   "2023",
		}),
		signal.New("After Hours").AddLabels(labels.Map{
			"genre":  "pop",
			"artist": "The Weekend",
			"year":   "2020",
		}),
		signal.New("Random Access Memories").AddLabels(labels.Map{
			"genre":  "electronic",
			"artist": "Daft Punk",
			"year":   "2013",
		}),
		signal.New("Evermore").AddLabels(labels.Map{
			"genre":  "pop",
			"artist": "Taylor Swift",
			"year":   "2020",
		}),
		signal.New("1989").AddLabels(labels.Map{
			"genre":  "pop",
			"artist": "Taylor Swift",
			"year":   "2014",
		}),
		signal.New("To Pimp a Butterfly").AddLabels(labels.Map{
			"genre":  "hip-hop",
			"artist": "Kendrick Lamar",
			"year":   "2015",
		}),
		signal.New("Ghost Stories").AddLabels(labels.Map{
			"genre":  "alternative",
			"artist": "Coldplay",
			"year":   "2014",
		}),
		signal.New("Future Nostalgia").AddLabels(labels.Map{
			"genre":  "pop",
			"artist": "Dua Lipa",
			"year":   "2020",
		}))
}
