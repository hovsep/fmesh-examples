package microcontroller

import (
	"github.com/hovsep/fmesh-examples/can_bus/advanced/can/common"
	"github.com/hovsep/fmesh/component"
)

// New creates a microcontroller unit component
func New(name string, initState func(state component.State), af component.ActivationFunc) *component.Component {
	return component.New("mcu-" + name).
		AddInputs(common.PortCANRx).  // Frame in
		AddOutputs(common.PortCANTx). // Frame out
		WithInitialState(initState).
		WithActivationFunc(af)
}
