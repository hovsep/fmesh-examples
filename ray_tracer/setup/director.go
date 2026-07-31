package setup

import (
	"context"

	"github.com/hovsep/fmesh-examples/ray_tracer/common"
	"github.com/hovsep/fmesh-examples/ray_tracer/render"
	"github.com/hovsep/fmesh/component"
	"github.com/hovsep/fmesh/signal"
)

// GetDirector returns the component which splits a frame into horizontal
// tiles, one per parallel tracing chain
func GetDirector() *component.Component {
	return common.Must(component.New("director",
		component.WithDescription("Splits each frame into tiles and distributes them to the chains"),
		component.WithInputs(common.PortIn),
		component.WithIndexedOutputs(common.PortTileOut, 0, common.NumChains-1),
		component.WithActivationFunc(func(_ context.Context, this *component.Component) error {
			return this.InputByName(common.PortIn).Signals().ForEach(func(sig *signal.Signal) error {
				cam := sig.Payload().(render.Camera)

				rowsPerTile := common.FrameHeight / common.NumChains
				for i := range common.NumChains {
					yFrom := i * rowsPerTile
					yTo := yFrom + rowsPerTile
					if i == common.NumChains-1 {
						yTo = common.FrameHeight
					}

					tile := signal.New(cam).WithScalars(map[string]float64{
						common.ScalarFrame: sig.Scalars().ValueOrDefault(common.ScalarFrame, 0),
						common.ScalarYFrom: float64(yFrom),
						common.ScalarYTo:   float64(yTo),
					})
					if err := this.OutputByName(common.IndexedPortName(common.PortTileOut, i)).PutSignals(tile); err != nil {
						return err
					}
				}
				return nil
			})
		}),
	))
}
