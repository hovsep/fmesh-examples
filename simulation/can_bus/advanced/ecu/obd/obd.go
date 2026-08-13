package obd

import (
	"context"
	"errors"
	"fmt"

	"github.com/hovsep/fmesh-examples/simulation/can_bus/advanced/can"
	"github.com/hovsep/fmesh-examples/simulation/can_bus/advanced/can/common"
	"github.com/hovsep/fmesh/component"
	"github.com/hovsep/fmesh/port"
)

const (
	PortOBDIn   = "obd_in"
	PortOBDOut  = "obd_out"
	OBDUnitName = "obd"
)

// NewNode creates an OBD can node
// in real life OBD socket is not a can node, but for simplicity
// we simulate OBD socket with plugged-in OBD adapter as a single CAN node
func NewNode() (*can.Node, error) {
	obdDevice, err := can.NewNode(OBDUnitName, func(state component.State) {
	},
		func(ctx context.Context, this *component.Component) error {

			errRx := port.ForwardSignals(ctx, this.InputByName(common.PortCANRx), this.OutputByName(PortOBDOut))

			// Everything received by OBD interface goes to can bus (todo: make it realistic, process only first signal, as OBD can not receive multiple frames at the same time)
			errTx := port.ForwardSignals(ctx, this.InputByName(PortOBDIn), this.OutputByName(common.PortCANTx))

			return errors.Join(errRx, errTx)
		})
	if err != nil {
		return nil, fmt.Errorf("obd node: %w", err)
	}

	if err := obdDevice.MCU.AddInputs(PortOBDIn); err != nil {
		return nil, fmt.Errorf("obd add inputs: %w", err)
	}
	if err := obdDevice.MCU.AddOutputs(PortOBDOut); err != nil {
		return nil, fmt.Errorf("obd add outputs: %w", err)
	}

	return obdDevice, nil
}
