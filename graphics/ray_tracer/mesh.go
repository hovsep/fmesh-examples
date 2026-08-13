package main

import (
	"github.com/hovsep/fmesh"
	"github.com/hovsep/fmesh-examples/graphics/ray_tracer/common"
	"github.com/hovsep/fmesh-examples/graphics/ray_tracer/imaging"
	"github.com/hovsep/fmesh-examples/graphics/ray_tracer/output"
	"github.com/hovsep/fmesh-examples/graphics/ray_tracer/setup"
	"github.com/hovsep/fmesh-examples/graphics/ray_tracer/tracing"
	"github.com/hovsep/fmesh/component"
)

// getMesh wires the wavefront rendering pipeline (see README.md for the topology)
func getMesh() (*fmesh.FMesh, error) {
	sceneSource := setup.GetSceneSource()
	orbit := setup.GetOrbit()
	cameraBuilder := setup.GetCameraBuilder()
	director := setup.GetDirector()
	raygens := tracing.GetRayGenerators()
	intersectors := tracing.GetIntersectors()
	shadowCasters := tracing.GetShadowCasters()
	shaders := tracing.GetShaders()
	accumulator := tracing.GetAccumulator()
	downsampler := imaging.GetDownsampler()
	gamma := imaging.GetGamma()
	packer := imaging.GetPacker()
	assembler := imaging.GetAssembler()
	palettizer := output.GetPalettizer()
	progress := output.GetProgress()
	encoder := output.GetEncoder()

	// The kick-off travels through the scene source, so the scene is
	// guaranteed to reach the chains before the first wave
	common.MustOK(sceneSource.OutputByName(common.PortStartOut).PipeTo(orbit.InputByName(common.PortNextFrame)))

	common.MustOK(orbit.OutputByName(common.PortOut).PipeTo(cameraBuilder.InputByName(common.PortIn)))
	common.MustOK(cameraBuilder.OutputByName(common.PortOut).PipeTo(director.InputByName(common.PortIn)))

	for i := range common.NumChains {
		raygen := raygens.ByName(common.WorkerName("raygen", i))
		intersector := intersectors.ByName(common.WorkerName("intersector", i))
		shadowCaster := shadowCasters.ByName(common.WorkerName("shadow-caster", i))
		shader := shaders.ByName(common.WorkerName("shader", i))

		common.MustOK(director.OutputByName(common.IndexedPortName(common.PortTileOut, i)).PipeTo(raygen.InputByName(common.PortIn)))
		common.MustOK(raygen.OutputByName(common.PortOut).PipeTo(intersector.InputByName(common.PortIn)))
		common.MustOK(intersector.OutputByName(common.PortOut).PipeTo(shadowCaster.InputByName(common.PortIn)))
		common.MustOK(shadowCaster.OutputByName(common.PortOut).PipeTo(shader.InputByName(common.PortIn)))
		common.MustOK(shader.OutputByName(common.PortSecondaryOut).PipeTo(intersector.InputByName(common.PortIn)))
		common.MustOK(shader.OutputByName(common.PortContribsOut).PipeTo(accumulator.InputByName(common.PortIn)))

		common.MustOK(sceneSource.OutputByName(common.PortGeometryOut).PipeTo(
			intersector.InputByName(common.PortGeometryIn),
			shadowCaster.InputByName(common.PortGeometryIn),
		))
		common.MustOK(sceneSource.OutputByName(common.PortLightOut).PipeTo(
			shadowCaster.InputByName(common.PortLightIn),
			shader.InputByName(common.PortLightIn),
		))
	}

	common.MustOK(accumulator.OutputByName(common.PortOut).PipeTo(downsampler.InputByName(common.PortIn)))
	common.MustOK(downsampler.OutputByName(common.PortOut).PipeTo(gamma.InputByName(common.PortIn)))
	common.MustOK(gamma.OutputByName(common.PortOut).PipeTo(packer.InputByName(common.PortIn)))
	common.MustOK(packer.OutputByName(common.PortOut).PipeTo(assembler.InputByName(common.PortIn)))

	common.MustOK(assembler.OutputByName(common.PortFrameOut).PipeTo(palettizer.InputByName(common.PortIn)))
	common.MustOK(palettizer.OutputByName(common.PortOut).PipeTo(encoder.InputByName(common.PortIn)))

	common.MustOK(assembler.OutputByName(common.PortProgressOut).PipeTo(progress.InputByName(common.PortIn)))

	// The assembler->orbit feedback keeps exactly one frame in flight, which
	// keeps the accumulator state and the wave interleaving trivial
	common.MustOK(assembler.OutputByName(common.PortNextOut).PipeTo(orbit.InputByName(common.PortNextFrame)))

	// The wavefront loop needs ~20 cycles per frame, far beyond the defaults
	fm, err := fmesh.New("ray tracer",
		fmesh.WithDescription("Wavefront ray tracer rendering an animated GIF"),
		fmesh.WithUnlimitedCycles(),
		fmesh.WithUnlimitedTime(),
	)
	if err != nil {
		return nil, err
	}

	if err := fm.AddComponents(sceneSource, orbit, cameraBuilder, director, accumulator,
		downsampler, gamma, packer, assembler, palettizer, progress, encoder); err != nil {
		return nil, err
	}
	for _, chain := range []*component.Collection{raygens, intersectors, shadowCasters, shaders} {
		if err := chain.ForEach(func(worker *component.Component) error {
			return fm.AddComponents(worker)
		}); err != nil {
			return nil, err
		}
	}
	return fm, nil
}
