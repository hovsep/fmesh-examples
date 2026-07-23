package imaging

import (
	"github.com/hovsep/fmesh-examples/ray_tracer/common"
	"github.com/hovsep/fmesh-examples/ray_tracer/render"
	"github.com/hovsep/fmesh/component"
)

// GetGamma returns the component applying gamma correction to linear colors
func GetGamma() *component.Component {
	return common.Must(component.New("gamma",
		component.WithDescription("Applies gamma correction to linear colors"),
		component.WithInputs(common.PortIn),
		component.WithOutputs(common.PortOut),
		component.WithActivationFunc(func(this *component.Component) error {
			return this.OutputByName(common.PortOut).PutSignalGroups(
				this.InputByName(common.PortIn).Signals().MapPayloads(func(p any) any {
					return render.GammaCorrect(p.([]render.Vec3))
				}))
		}),
	))
}
