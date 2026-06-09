package main

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"

	"github.com/hovsep/fmesh"
	"github.com/hovsep/fmesh-examples/internal"
	"github.com/hovsep/fmesh-graphviz/dot"
	"github.com/hovsep/fmesh/component"
	"github.com/hovsep/fmesh/port"
	"github.com/hovsep/fmesh/signal"
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
	err := internal.HandleGraphFlag(fm, true)
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

	cyclesGraphs, err := exporter.ExportWithCycles(fm, runtimeInfo.Cycles)
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
	bashCmd := `for f in *.dot; do dot -Tpng "$f" -o "${f%.dot}.png"; done`
	fmt.Println(bashCmd)

}

func getMesh() *fmesh.FMesh {
	engine, err := component.New("engine",
		component.WithDescription("Sends out rotation signal once started"),
		component.WithInputs("start"),
		component.WithOutputs("rotation"),
		component.WithActivationFunc(func(this *component.Component) error {
			revolution := signal.New(10).WithLabel("direction", "clockwise")

			this.OutputByName("rotation").PutSignals(revolution)
			return nil
		}),
	)
	if err != nil {
		panic(fmt.Sprintf("failed to create engine component: %v", err))
	}

	clutch, err := component.New("clutch",
		component.WithDescription("Simple clutch"),
		component.WithInputs("rotation"),
		component.WithOutputs("rotation"),
		component.WithActivationFunc(func(this *component.Component) error {
			// Assume clutch is always engaged
			return port.ForwardSignals(this.InputByName("rotation"), this.OutputByName("rotation"))
		}),
	)
	if err != nil {
		panic(fmt.Sprintf("failed to create clutch component: %v", err))
	}

	gearbox, err := component.New("gearbox",
		component.WithDescription("⚙️"),
		component.WithInputs("rotation"),
		component.WithOutputs("rotation"),
		component.WithActivationFunc(func(this *component.Component) error {
			return this.InputByName("rotation").Signals().ForEach(func(s *signal.Signal) error {
				rotationAfter := s.MapPayload(func(payload any) any {
					// Simulate gear ratio
					return payload.(int) / 2
				})

				return this.OutputByName("rotation").PutSignals(rotationAfter)

			})
		}),
	)
	if err != nil {
		panic(fmt.Sprintf("failed to create gearbox component: %v", err))
	}

	wheels, err := component.New("wheels",
		component.WithDescription("🚗"),
		component.WithInputs("rotation"),
		component.WithOutputs("rotation"),
		component.WithActivationFunc(func(this *component.Component) error {
			return port.ForwardSignals(this.InputByName("rotation"), this.OutputByName("rotation"))
		}),
	)
	if err != nil {
		panic(fmt.Sprintf("failed to create wheels component: %v", err))
	}

	// Piping
	if err := engine.OutputByName("rotation").PipeTo(clutch.InputByName("rotation")); err != nil {
		panic(fmt.Sprintf("failed to pipe engine to clutch: %v", err))
	}
	if err := clutch.OutputByName("rotation").PipeTo(gearbox.InputByName("rotation")); err != nil {
		panic(fmt.Sprintf("failed to pipe clutch to gearbox: %v", err))
	}
	if err := gearbox.OutputByName("rotation").PipeTo(wheels.InputByName("rotation")); err != nil {
		panic(fmt.Sprintf("failed to pipe gearbox to wheels: %v", err))
	}

	fm, err := fmesh.New("graph",
		fmesh.WithDescription("Simple car mechanics simulation"),
	)
	if err != nil {
		panic(fmt.Sprintf("failed to create mesh: %v", err))
	}
	if err := fm.AddComponents(engine, clutch, gearbox, wheels); err != nil {
		panic(fmt.Sprintf("failed to add components: %v", err))
	}

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
