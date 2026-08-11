package imaging

import (
	"context"

	"github.com/hovsep/fmesh-examples/graphics/ray_tracer/common"
	"github.com/hovsep/fmesh-examples/graphics/ray_tracer/render"
	"github.com/hovsep/fmesh/component"
)

// GetPacker returns the component packing colors into raw RGBA bytes
func GetPacker() *component.Component {
	return common.Must(component.New("packer",
		component.WithDescription("Packs colors into RGBA bytes"),
		component.WithInputs(common.PortIn),
		component.WithOutputs(common.PortOut),
		component.WithActivationFunc(func(_ context.Context, this *component.Component) error {
			return this.OutputByName(common.PortOut).PutSignalGroups(
				this.InputByName(common.PortIn).Signals().MapPayloads(func(p any) any {
					return render.PackRGBA(p.([]render.Vec3))
				}))
		}),
	))
}
