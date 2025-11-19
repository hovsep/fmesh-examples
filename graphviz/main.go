package main

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"github.com/hovsep/fmesh"
	"github.com/hovsep/fmesh-examples/internal"
	"github.com/hovsep/fmesh-graphviz/dot"
	"github.com/hovsep/fmesh/component"
	"github.com/hovsep/fmesh/port"
	"github.com/hovsep/fmesh/signal"
	"os"
)

// This example demonstrates how to visualize an fmesh network using the fmesh-graphviz package.
// It builds a simple mesh representing a car drivetrain (engine → clutch → gearbox → wheels),
// then exports the mesh structure and its activation cycles as DOT files.
//
// These DOT files can be rendered into images using Graphviz,
// allowing you to inspect both the static topology and runtime behavior of the mesh.
func main() {
	fm := getMesh()

	// Generate graphs if needed
	err := internal.HandleGraphFlag(fm)
	if err != nil {
		fmt.Println("Failed to generate graph: ", err)
		os.Exit(1)
	}

	// Start the engine!
	fm.ComponentByName("engine").InputByName("start").PutSignals(signal.New("launch"))

	runtimeInfo, err := fm.Run()
	if err != nil {
		panic("Pipeline finished with error:" + err.Error())
	}

	fmt.Println("The mesh successfully finished, so we can try to export it as DOT graph")
	fmt.Println("learn more about DOT at https://graphviz.org/")

	// Visualise !
	exporter := dot.NewDotExporter()

	staticGraphBytes, err := exporter.Export(fm)
	if err != nil {
		panic("can not export static graph")
	}

	fmt.Println("The mesh static (without activation cycles info) DOT graph:")
	fmt.Println(string(staticGraphBytes))

	// Generate a random id, so user can run the example multiple times without filename collisions
	hash := make([]byte, 4)
	_, err = rand.Read(hash)
	if err != nil {
		panic(err)
	}
	runId := hex.EncodeToString(hash[:])

	writeGraphToFile(staticGraphBytes, fmt.Sprintf("static_graph-%v.dot", runId))

	cyclesGraphs, err := exporter.ExportWithCycles(fm, runtimeInfo.Cycles.CyclesOrNil())
	if err != nil {
		panic("can not export graph with cycles")
	}

	fmt.Println("Also you can create a graph representation of each activation cycle ! (activated components will be highlighted with different color)")
	for cycleNum, cycleGraph := range cyclesGraphs {
		fmt.Printf("Cycle #%d graph:\n", cycleNum)
		fmt.Println(string(cycleGraph))
		writeGraphToFile(cycleGraph, fmt.Sprintf("cycle#%d-%v.dot", cycleNum, runId))
	}

	fmt.Println("You can inspect the graphs using online editors like https://edotor.net")
	fmt.Println("All generated graphs are also written as local files")
	fmt.Println("Want to convert all .dot files to images? Run the following command:")
	// ignore go vet
	fmt.Println(`for f in *.dot; do dot -Tpng "$f" -o "${f%.dot}.png"; done`)

}

func getMesh() *fmesh.FMesh {
	fm := fmesh.New("graph").
		WithDescription("Simple car mechanics simulation").
		WithComponents(
			component.New("engine").
				WithDescription("Sends out rotation signal once started").
				WithInputs("start").
				WithOutputs("rotation").
				WithActivationFunc(func(this *component.Component) error {
					revolution := signal.New(10)
					revolution.AddLabel("direction", "clockwise")

					this.OutputByName("rotation").PutSignals(revolution)
					return nil
				}),

			component.New("clutch").
				WithDescription("Simple clutch").
				WithInputs("rotation").
				WithOutputs("rotation").
				WithActivationFunc(func(this *component.Component) error {
					// Assume clutch is always engaged
					return port.ForwardSignals(this.InputByName("rotation"), this.OutputByName("rotation"))
				}),

			component.New("gearbox").
				WithDescription("⚙️").
				WithInputs("rotation").
				WithOutputs("rotation").
				WithActivationFunc(func(this *component.Component) error {
					for _, s := range this.InputByName("rotation").AllSignalsOrNil() {
						// Simulate gear ratio
						rotationAfter := signal.New(s.PayloadOrDefault(0).(int) / 2).WithLabels(s.Labels())
						this.OutputByName("rotation").PutSignals(rotationAfter)
					}

					return nil
				}),

			component.New("wheels").
				WithDescription("🚗").
				WithInputs("rotation").
				WithOutputs("rotation").
				WithActivationFunc(func(this *component.Component) error {
					return port.ForwardSignals(this.InputByName("rotation"), this.OutputByName("rotation"))
				}),
		)

	// Piping
	fm.ComponentByName("engine").OutputByName("rotation").PipeTo(fm.ComponentByName("clutch").InputByName("rotation"))
	fm.ComponentByName("clutch").OutputByName("rotation").PipeTo(fm.ComponentByName("gearbox").InputByName("rotation"))
	fm.ComponentByName("gearbox").OutputByName("rotation").PipeTo(fm.ComponentByName("wheels").InputByName("rotation"))

	return fm
}

func writeGraphToFile(data []byte, fileName string) {
	if len(data) == 0 {
		panic("something is wrong: got no data")
	}

	root, err := os.OpenRoot(".")
	if err != nil {
		panic("can not open root")
	}
	file, err := root.Create(fileName)
	if err != nil {
		panic("can not open root")
	}

	n, err := file.Write(data)
	if err != nil {
		panic("can not write to file")
	}

	if n == 0 {
		panic("something is wrong: written 0 bytes")
	}
}
