package main

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"github.com/hovsep/fmesh"
	"github.com/hovsep/fmesh-graphviz/dot"
	"github.com/hovsep/fmesh/component"
	"github.com/hovsep/fmesh/port"
	"github.com/hovsep/fmesh/signal"
	"os"
)

// This example shows how to visualize your mesh as a DOT graph using fmesh-graphviz package
func main() {
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

	// Start the engine !
	fm.ComponentByName("engine").InputByName("start").PutSignals(signal.New("launch"))

	runtimeInfo, err := fm.Run()
	if err != nil {
		fmt.Println("Pipeline finished with error:", err)
		os.Exit(1)
	}

	// Visualise !
	exporter := dot.NewDotExporter()

	staticGraphBytes, err := exporter.Export(fm)
	if err != nil {
		panic("can not export static graph")
	}

	// Generate a random id, so user can run the example multiple times without filename collisions
	hash := make([]byte, 4)
	_, err = rand.Read(hash)
	if err != nil {
		panic(err)
	}
	runId := hex.EncodeToString(hash[:])

	writeGraphToFile(staticGraphBytes, fmt.Sprintf("static_graph-_%v.dot", runId))

	cyclesGraphs, err := exporter.ExportWithCycles(fm, runtimeInfo.Cycles.CyclesOrNil())
	if err != nil {
		panic("can not export graph with cycles")
	}

	for cycleNum, cycleGraph := range cyclesGraphs {
		writeGraphToFile(cycleGraph, fmt.Sprintf("cycle#%d-_%v.dot", cycleNum, runId))
	}

	fmt.Printf("Want to convert all .dot files to images? Run the following command: \nfor f in *.dot; do dot -Tpng \"$f\" -o \"${f%.dot}.png\"; done\n")

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
