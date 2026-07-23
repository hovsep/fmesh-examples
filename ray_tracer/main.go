package main

import (
	"fmt"
	"os"

	"github.com/hovsep/fmesh-examples/ray_tracer/common"
	"github.com/hovsep/fmesh/signal"
)

// This example renders a 3D animation (an orbit around a small ray-traced
// scene) with the whole renderer — including its control flow — expressed
// as an F-Mesh. Every render function is a component:
//
//	scene ──(geometry)──> every intersector and shadow-caster    (one-off config signals,
//	  │ └───(light)─────> every shadow-caster and shader          derived from the scene)
//	  │
//	(start)                                 ┌>[raygen-0 ─> intersector-0 ─> shadow-caster-0 ─> shader-0]┐
//	  v                                     ├>[raygen-1 ─> ...                              ─> shader-1]┼─> accumulator
//	orbit ─> camera-builder ─> director ────┼>[raygen-2 ─> ...   ^                          ─> shader-2]┤        │
//	  ^                                     └>[raygen-3 ─> ...   └──(reflected wave, bounce+1)──────────┘        v
//	  │                                                                                                    downsampler
//	  │                                                                                                          v
//	  └──(next frame number)── assembler <── packer <── gamma <──────────────────────────────────────────────────┘
//	                              │  │
//	                              │  ├──(frame scalar)──> progress
//	                              │  └─> palettizer ─> encoder ─> out.gif
//
// This is wavefront ray tracing — the architecture real GPU path tracers use:
// instead of a recursive trace() call, rays travel in per-tile waves and the
// reflection recursion becomes a mesh cycle (shader -> intersector), with each
// ray carrying the weight of its remaining contribution. The accumulator sums
// the waves until the last bounce and releases the finished tile.
//
// F-Mesh concepts demonstrated:
//
//  1. Fan-out parallelism: four raygen->intersector->shadow-caster->shader
//     chains render tile bands concurrently (components of a cycle activate
//     in parallel goroutines).
//  2. Recursion as a feedback loop: the shader pipes the reflected wave back
//     into its intersector until MaxBounces is reached (wavefront tracing).
//  3. Fan-in: all shaders feed one accumulator port; all chains feed one
//     downsampler port.
//  4. Scalars: routing metadata (frame, y_from, bounce) rides on signals as
//     native numeric metadata; MapPayload(s) preserves it, so pure stages
//     never touch it. There are no custom "message" structs in this package.
//  5. Config as data flow: the scene is not shared state — the scene source
//     derives per-concern signals from it (geometry, light) and each tracing
//     component adopts a private copy of only the data it needs.
//  6. Cycle barrier: the assembler naively merges whatever tiles arrived,
//     because a cycle only ends when every component has finished.
//  7. Feedback-driven animation: the assembler requests the next frame from
//     the orbit — no rendering loop in main().
//  8. Observers: the progress component reports from scalars alone.
//
// The code is grouped by pipeline role: setup (scene, orbit, camera-builder,
// director), tracing (the chains and the accumulator), imaging (downsampler,
// gamma, packer, assembler), output (palettizer, encoder, progress) and
// common (shared port/scalar names and config). The pure 3D code (vectors,
// camera, scene, shading) lives in the render package and knows nothing
// about F-Mesh.
//
// This example is a standalone Go module using the latest fmesh release.
// Run: cd ray_tracer && go run . (writes out.gif to the current directory)
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

	// Kick off the animation by requesting the first frame from the scene
	// source (the mesh's single entry point), the mesh keeps itself busy
	// until the last frame is rendered
	if err := fm.ComponentByName("scene").InputByName(common.PortIn).PutSignals(signal.New(0)); err != nil {
		fmt.Println("Failed to put the kick-off signal:", err)
		os.Exit(1)
	}

	if _, err := fm.Run(); err != nil {
		fmt.Println("Rendering finished with error:", err)
		os.Exit(1)
	}
}
