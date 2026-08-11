// Package imaging provides the components turning traced color samples into
// finished RGBA frames: downsampling, gamma correction, byte packing and
// the assembler gluing tiles together.
package imaging

import (
	"context"

	"github.com/hovsep/fmesh-examples/graphics/ray_tracer/common"
	"github.com/hovsep/fmesh-examples/graphics/ray_tracer/render"
	"github.com/hovsep/fmesh/component"
)

// GetDownsampler returns the component averaging the supersampled colors
// down to one color per pixel. All chains fan into its single input port
func GetDownsampler() *component.Component {
	return common.Must(component.New("downsampler",
		component.WithDescription("Averages color samples down to one color per pixel"),
		component.WithInputs(common.PortIn),
		component.WithOutputs(common.PortOut),
		component.WithActivationFunc(func(_ context.Context, this *component.Component) error {
			return this.OutputByName(common.PortOut).PutSignalGroups(
				this.InputByName(common.PortIn).Signals().MapPayloads(func(p any) any {
					return render.AveragePixels(p.([]render.Vec3))
				}))
		}),
	))
}
