package examplehelper

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/hovsep/fmesh"
	"github.com/hovsep/fmesh-graphviz/dot"
)

// Flags holds parsed command-line flags for examples
type Flags struct {
	GenerateGraph bool
	GraphOutput   string
}

// ParseFlags parses common flags for examples
// Returns flags and whether to exit early (e.g., for --help)
func ParseFlags() (*Flags, bool) {
	var genGraph bool
	var graphOutput string
	var help bool

	flag.BoolVar(&genGraph, "graph", false, "Generate graph.dot and graph.svg files and exit")
	flag.StringVar(&graphOutput, "graph-output", "graph", "Base name for graph files (without extension)")
	flag.BoolVar(&help, "help", false, "Show help message")
	flag.Parse()

	if help {
		flag.Usage()
		return nil, true
	}

	return &Flags{
		GenerateGraph: genGraph,
		GraphOutput:   graphOutput,
	}, false
}

// GenerateGraph generates both graph.dot and graph.svg files from the given mesh
// Always generates DOT format, and SVG if graphviz is available
func GenerateGraph(fm *fmesh.FMesh, basePath string) error {
	if fm == nil {
		return fmt.Errorf("mesh is nil")
	}

	// Generate DOT format
	exporter := dot.NewDotExporter()
	dotBytes, err := exporter.Export(fm)
	if err != nil {
		return fmt.Errorf("failed to export mesh to DOT: %w", err)
	}

	dotFile := basePath + ".dot"
	svgFile := basePath + ".svg"

	// Always write DOT file
	if err := os.WriteFile(dotFile, dotBytes, 0644); err != nil {
		return fmt.Errorf("failed to write DOT file: %w", err)
	}
	absPath, _ := filepath.Abs(dotFile)
	fmt.Printf("DOT graph generated: %s\n", absPath)

	// Generate SVG if graphviz is available
	if _, err := exec.LookPath("dot"); err != nil {
		fmt.Printf("Warning: graphviz not installed, skipping SVG generation. DOT file available: %s\n", dotFile)
		return nil
	}

	// Convert DOT to SVG using graphviz
	cmd := exec.Command("dot", "-Tsvg", dotFile, "-o", svgFile)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to convert DOT to SVG: %w", err)
	}

	absPath, _ = filepath.Abs(svgFile)
	fmt.Printf("SVG graph generated: %s\n", absPath)
	return nil
}

// RunWithFlags is a convenience function that handles flags and runs the example
// If --graph flag is set, generates graph and returns true (caller should exit)
// Otherwise returns false and caller should continue with normal execution
func RunWithFlags(getMesh func() *fmesh.FMesh) bool {
	flags, shouldExit := ParseFlags()
	if shouldExit {
		return true
	}

	if flags.GenerateGraph {
		fm := getMesh()
		if err := GenerateGraph(fm, flags.GraphOutput); err != nil {
			fmt.Fprintf(os.Stderr, "Error generating graph: %v\n", err)
			os.Exit(1)
		}
		return true // Exit after generating graph
	}

	return false // Continue with normal execution
}
