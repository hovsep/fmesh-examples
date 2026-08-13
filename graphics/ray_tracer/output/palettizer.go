// Package output provides the components delivering the rendered animation:
// palettizing frames for GIF, encoding the file and reporting progress.
package output

import (
	"context"
	"image"
	"image/color/palette"
	"image/draw"

	"github.com/hovsep/fmesh-examples/graphics/ray_tracer/common"
	"github.com/hovsep/fmesh/component"
)

// GetPalettizer returns the component converting RGBA frames to GIF's
// indexed color space
func GetPalettizer() *component.Component {
	return common.Must(component.New("palettizer",
		component.WithDescription("Converts frames to paletted colors with dithering"),
		component.WithInputs(common.PortIn),
		component.WithOutputs(common.PortOut),
		component.WithActivationFunc(func(_ context.Context, this *component.Component) error {
			return this.OutputByName(common.PortOut).PutSignalGroups(
				this.InputByName(common.PortIn).Signals().MapPayloads(func(p any) any {
					img := p.(*image.RGBA)

					// GIF frames are palette-based, dithering hides the banding
					paletted := image.NewPaletted(img.Bounds(), palette.Plan9)
					draw.FloydSteinberg.Draw(paletted, img.Bounds(), img, image.Point{})
					return paletted
				}))
		}),
	))
}
