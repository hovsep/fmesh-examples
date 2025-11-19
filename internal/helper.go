package internal

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/hovsep/fmesh"
	"github.com/hovsep/fmesh-graphviz/dot"
)

// HandleGraphFlag handles the -graph flag and generates a graph.dot and graph.svg file if requested
func HandleGraphFlag(fm *fmesh.FMesh) error {
	var shouldGenerateGraph bool

	flag.BoolVar(&shouldGenerateGraph, "graph", false, "Generate graph.dot and graph.svg files and exit")

	if !shouldGenerateGraph {
		return nil
	}

	// Generate DOT format
	exporter := dot.NewDotExporter()
	dotBytes, err := exporter.Export(fm)
	if err != nil {
		return fmt.Errorf("failed to export mesh to DOT: %w", err)
	}

	dotFile := "graph.dot"
	svgFile := "graph.svg"

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
