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

func main() {
	fm, err := getMesh()
	if err != nil {
		fmt.Println("Failed to build mesh:", err)
		os.Exit(1)
	}

	if err := internal.HandleGraphFlag(fm, true); err != nil {
		fmt.Println("Failed to generate graph:", err)
		os.Exit(1)
	}

	fm.ComponentByName("engine").InputByName("start").PutSignals(signal.New("launch"))

	runtimeInfo, err := fm.Run()
	if err != nil {
		fmt.Println("Pipeline finished with error:", err)
		os.Exit(1)
	}

	fmt.Println("The mesh successfully finished, so we can try to export it as DOT graph")
	fmt.Println("learn more about DOT at https://graphviz.org/")

	exporter := dot.NewDotExporter()

	staticGraphBytes, err := exporter.Export(fm)
	if err != nil {
		fmt.Println("can not export static graph:", err)
		os.Exit(1)
	}

	fmt.Println("The mesh static (without activation cycles info) DOT graph:")
	fmt.Println(string(staticGraphBytes))

	hash := make([]byte, 4)
	if _, err := rand.Read(hash); err != nil {
		fmt.Println("failed to generate random id:", err)
		os.Exit(1)
	}
	runId := hex.EncodeToString(hash[:])

	if err := writeGraphToFile(staticGraphBytes, fmt.Sprintf("static_graph-%v.dot", runId)); err != nil {
		fmt.Println("failed to write static graph:", err)
		os.Exit(1)
	}

	cyclesGraphs, err := exporter.ExportWithCycles(fm, runtimeInfo.Cycles)
	if err != nil {
		fmt.Println("can not export graph with cycles:", err)
		os.Exit(1)
	}

	fmt.Println("Also you can create a graph representation of each activation cycle ! (activated components will be highlighted with different color)")
	for cycleNum, cycleGraph := range cyclesGraphs {
		fmt.Printf("Cycle #%d graph:\n", cycleNum)
		fmt.Println(string(cycleGraph))
		if err := writeGraphToFile(cycleGraph, fmt.Sprintf("cycle#%d-%v.dot", cycleNum, runId)); err != nil {
			fmt.Println("failed to write cycle graph:", err)
		}
	}

	fmt.Println("You can inspect the graphs using online editors like https://edotor.net")
	fmt.Println("All generated graphs are also written as local files")
	fmt.Println("Want to convert all .dot files to images? Run the following command:")
	bashCmd := `for f in *.dot; do dot -Tpng "$f" -o "${f%.dot}.png"; done`
	fmt.Println(bashCmd)
}

func getMesh() (*fmesh.FMesh, error) {
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
		return nil, fmt.Errorf("engine component: %w", err)
	}

	clutch, err := component.New("clutch",
		component.WithDescription("Simple clutch"),
		component.WithInputs("rotation"),
		component.WithOutputs("rotation"),
		component.WithActivationFunc(func(this *component.Component) error {
			return port.ForwardSignals(this.InputByName("rotation"), this.OutputByName("rotation"))
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("clutch component: %w", err)
	}

	gearbox, err := component.New("gearbox",
		component.WithDescription("⚙️"),
		component.WithInputs("rotation"),
		component.WithOutputs("rotation"),
		component.WithActivationFunc(func(this *component.Component) error {
			return this.InputByName("rotation").Signals().ForEach(func(s *signal.Signal) error {
				rotationAfter := s.MapPayload(func(payload any) any {
					return payload.(int) / 2
				})
				return this.OutputByName("rotation").PutSignals(rotationAfter)
			})
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("gearbox component: %w", err)
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
		return nil, fmt.Errorf("wheels component: %w", err)
	}

	if err := engine.OutputByName("rotation").PipeTo(clutch.InputByName("rotation")); err != nil {
		return nil, fmt.Errorf("pipe engine→clutch: %w", err)
	}
	if err := clutch.OutputByName("rotation").PipeTo(gearbox.InputByName("rotation")); err != nil {
		return nil, fmt.Errorf("pipe clutch→gearbox: %w", err)
	}
	if err := gearbox.OutputByName("rotation").PipeTo(wheels.InputByName("rotation")); err != nil {
		return nil, fmt.Errorf("pipe gearbox→wheels: %w", err)
	}

	fm, err := fmesh.New("graph",
		fmesh.WithDescription("Simple car mechanics simulation"),
	)
	if err != nil {
		return nil, fmt.Errorf("new mesh: %w", err)
	}
	if err := fm.AddComponents(engine, clutch, gearbox, wheels); err != nil {
		return nil, fmt.Errorf("add components: %w", err)
	}

	return fm, nil
}

func writeGraphToFile(data []byte, fileName string) error {
	if len(data) == 0 {
		return fmt.Errorf("no data to write")
	}

	root, err := os.OpenRoot(".")
	if err != nil {
		return fmt.Errorf("open root: %w", err)
	}
	defer root.Close()

	file, err := root.Create(fileName)
	if err != nil {
		return fmt.Errorf("create file: %w", err)
	}
	defer file.Close()

	n, err := file.Write(data)
	if err != nil {
		return fmt.Errorf("write file: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("written 0 bytes")
	}
	return nil
}
