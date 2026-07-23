package setup

import (
	"github.com/hovsep/fmesh-examples/ray_tracer/common"
	"github.com/hovsep/fmesh-examples/ray_tracer/render"
	"github.com/hovsep/fmesh/component"
)

// GetCameraBuilder returns the component turning a viewpoint into a camera
// basis. A pure mapping stage: the frame scalar rides through untouched
func GetCameraBuilder() *component.Component {
	return common.Must(component.New("camera-builder",
		component.WithDescription("Builds the camera basis from the viewpoint"),
		component.WithInputs(common.PortIn),
		component.WithOutputs(common.PortOut),
		component.WithActivationFunc(func(this *component.Component) error {
			return this.OutputByName(common.PortOut).PutSignalGroups(
				this.InputByName(common.PortIn).Signals().MapPayloads(func(p any) any {
					viewpoint := p.(render.Viewpoint)
					return render.NewCamera(viewpoint.Position, viewpoint.LookAt)
				}))
		}),
	))
}
