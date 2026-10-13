package output

import (
	"context"

	"github.com/hovsep/fmesh-examples/graphics/ray_tracer/common"
	"github.com/hovsep/fmesh/component"
)

// GetProgress returns the component reporting rendering progress.
// It sits aside of the main flow and is payload-agnostic:
// everything it needs rides on the signal's scalars
func GetProgress() *component.Component {
	return common.Must(component.New("progress",
		component.WithDescription("Reports rendering progress"),
		component.WithInputs(common.PortIn),
		component.WithActivationFunc(func(_ context.Context, this *component.Component) error {
			for sig := range this.InputByName(common.PortIn).Signals().All() {
				this.Logger().Printf("rendered frame %d/%d",
					common.ScalarInt(sig, common.ScalarFrame)+1, common.TotalFrames)
			}
			return nil
		}),
	))
}
