package internal

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/hovsep/fmesh"
	"github.com/hovsep/fmesh-export/dot"
)

// HandleGraphFlag handles the FMESH_GRAPH env flag and generates graph files.
// It reports handled=true when the flag is set, so a caller that only wants
// the graph can stop there; it never exits the process itself.
func HandleGraphFlag(fm *fmesh.FMesh) (handled bool, err error) {
	if os.Getenv("FMESH_GRAPH") != "1" {
		return false, nil
	}

	// Generate DOT format
	dotBytes, err := dot.New().Export(fm)
	if err != nil {
		return true, fmt.Errorf("failed to export mesh to DOT: %w", err)
	}

	dotFile := fm.Name() + "-graph.dot"
	svgFile := fm.Name() + "-graph.svg"

	// Always write DOT file
	if err := os.WriteFile(dotFile, dotBytes, 0644); err != nil {
		return true, fmt.Errorf("failed to write DOT file: %w", err)
	}
	absPath, _ := filepath.Abs(dotFile)
	fmt.Printf("DOT graph generated: %s\n", absPath)

	// Generate SVG if graphviz is available
	if _, err := exec.LookPath("dot"); err != nil {
		fmt.Printf("Warning: graphviz not installed, skipping SVG generation. DOT file available: %s\n", dotFile)
		return true, nil
	}

	// Convert DOT to SVG using graphviz
	cmd := exec.Command("dot", "-Tsvg", dotFile, "-o", svgFile)
	if err := cmd.Run(); err != nil {
		return true, fmt.Errorf("failed to convert DOT to SVG: %w", err)
	}

	absPath, _ = filepath.Abs(svgFile)
	fmt.Printf("SVG graph generated: %s\n", absPath)
	return true, nil
}
