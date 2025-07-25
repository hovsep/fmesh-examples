package microcontroller

import (
	"errors"
	"fmt"
	"github.com/hovsep/fmesh-examples/can_bus/advanced/internal/can/codec"
	"github.com/hovsep/fmesh-examples/can_bus/advanced/internal/can/common"
	"github.com/hovsep/fmesh/component"
	"github.com/hovsep/fmesh/signal"
)

// LogicFunc is the high level app of the MCU which works on top of CAN
type LogicFunc func(mode AddressingMode, request *ISOTPMessage, this *component.Component) (*ISOTPMessage, error)

// LogicToActivationFunc embeds the given MCU logic into FMesh activation func
func LogicToActivationFunc(logic LogicFunc, mcuPhysicalAddress uint32) component.ActivationFunc {
	af := func(this *component.Component) error {

		for _, sig := range this.InputByName(common.PortCANRx).AllSignalsOrNil() {
			// Validate CAN frame
			frame, ok := sig.PayloadOrNil().(*codec.Frame)
			if !ok {
				return errors.New("got corrupted frame")
			}

			// Convert CAN frame to ISO-TP message (the request)
			ISOTPRequest, err := CANFrameToISOTP(frame)
			if err != nil {
				return fmt.Errorf("failed to parse ISO-TP message: %w", err)
			}

			// Resolve request address
			addressingMode := Physical

			if frame.Id == FunctionalRequestID {
				addressingMode = Functional
			}
			this.Logger().Printf("received ISO-TP request: address: %s to %d, sid: %d, pid: %d", addressingMode, frame.Id, ISOTPRequest.ServiceID, ISOTPRequest.PID)

			// Run logic and get response
			ISOTPResponse, err := logic(addressingMode, ISOTPRequest, this)
			if err != nil {
				return fmt.Errorf("failed to apply MCU logic: %w", err)
			}

			// Return response down to CAN controller
			canFrame, err := ISOTPToCANFrame(ISOTPResponse, mcuPhysicalAddress)
			if err != nil {
				return fmt.Errorf("failed to convert ISOTP to CAN frame: %w", err)
			}
			this.OutputByName(common.PortCANTx).PutSignals(signal.New(canFrame))
			return nil
		}

		return nil
	}

	return af
}
