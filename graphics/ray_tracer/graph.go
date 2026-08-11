package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/hovsep/fmesh"
	"github.com/hovsep/fmesh-graphviz/dot"
)

// handleGraphFlag handles the FMESH_GRAPH env flag: when set, it exports the
// mesh topology as DOT (and SVG if graphviz is installed) and exits.
// This is a local copy of the repo's internal helper: the example lives in
// its own module and cannot import it
func handleGraphFlag(fm *fmesh.FMesh) error {
	if os.Getenv("FMESH_GRAPH") != "1" {
		return nil
	}
	defer os.Exit(0)

	exporter := dot.NewDotExporter()
	dotBytes, err := exporter.Export(fm)
	if err != nil {
		return fmt.Errorf("failed to export mesh to DOT: %w", err)
	}

	dotFile := fm.Name() + "-graph.dot"
	svgFile := fm.Name() + "-graph.svg"

	if err := os.WriteFile(dotFile, dotBytes, 0644); err != nil {
		return fmt.Errorf("failed to write DOT file: %w", err)
	}
	absPath, _ := filepath.Abs(dotFile)
	fmt.Printf("DOT graph generated: %s\n", absPath)

	if _, err := exec.LookPath("dot"); err != nil {
		fmt.Printf("Warning: graphviz not installed, skipping SVG generation. DOT file available: %s\n", dotFile)
		return nil
	}

	cmd := exec.Command("dot", "-Tsvg", dotFile, "-o", svgFile)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to convert DOT to SVG: %w", err)
	}

	absPath, _ = filepath.Abs(svgFile)
	fmt.Printf("SVG graph generated: %s\n", absPath)
	return nil
}
