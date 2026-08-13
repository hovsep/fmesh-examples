package main

import (
	"context"
	"fmt"
	"os"

	"github.com/hovsep/fmesh-examples/graphics/ray_tracer/common"
	"github.com/hovsep/fmesh/signal"
)

// Wavefront ray tracer rendering an animated GIF, with the whole renderer —
// including its control flow — expressed as an F-Mesh. See README.md for the
// architecture and the concepts demonstrated
func main() {
	fm, err := getMesh()
	if err != nil {
		fmt.Println("Failed to build mesh:", err)
		os.Exit(1)
	}

	// Generate graphs if needed
	if err := handleGraphFlag(fm); err != nil {
		fmt.Println("Failed to generate graph:", err)
		os.Exit(1)
	}

	// Request frame 0 from the scene source (the mesh's single entry point);
	// the assembler->orbit feedback keeps the mesh busy until the last frame
	if err := fm.ComponentByName("scene").InputByName(common.PortIn).PutSignals(signal.New(0)); err != nil {
		fmt.Println("Failed to put the kick-off signal:", err)
		os.Exit(1)
	}

	if _, err := fm.Run(context.Background()); err != nil {
		fmt.Println("Rendering finished with error:", err)
		os.Exit(1)
	}
}
