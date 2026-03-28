package internal

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/hovsep/fmesh"
	"github.com/hovsep/fmesh-graphviz/dot"
)

// HandleGraphFlag handles the -graph flag and generates a graph.dot and graph.svg file if requested
// @TODO: refactor, allow to export multiple meshes in 1 program
func HandleGraphFlag(fm *fmesh.FMesh) error {
	shouldGenerateGraph := os.Getenv("FMESH_GRAPH") == "1"

	if !shouldGenerateGraph {
		return nil
	}

	// Generate DOT format
	exporter := dot.NewDotExporter()
	dotBytes, err := exporter.Export(fm)
	if err != nil {
		return fmt.Errorf("failed to export mesh to DOT: %w", err)
	}

	dotFile := fm.Name() + "-graph.dot"
	svgFile := fm.Name() + "-graph.svg"

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
