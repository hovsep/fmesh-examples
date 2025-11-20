package microcontroller

import (
	"errors"
	"fmt"

	"github.com/hovsep/fmesh-examples/can_bus/advanced/can/codec"
	"github.com/hovsep/fmesh-examples/can_bus/advanced/can/common"
	"github.com/hovsep/fmesh/component"
	"github.com/hovsep/fmesh/signal"
)

// ReqHandler is the function that accepts request and provides response
type ReqHandler func(mode AddressingMode, request *ISOTPMessage, mcu *component.Component) (*ISOTPMessage, error)

// ParamsMap maps parameter IDs to respective handler
type ParamsMap map[ParameterID]ReqHandler

// ServiceMap maps services to supported parameters
type ServiceMap map[ServiceID]ParamsMap

// LogicMap maps addressing modes to allowed services
type LogicMap map[AddressingMode]ServiceMap

// LogicDescriptor contains the whole MCU behavior
type LogicDescriptor struct {
	PhysicalAddress uint32
	Table           LogicMap
}

func (ld LogicDescriptor) ToActivationFunc() component.ActivationFunc {
	af := func(this *component.Component) error {
		return this.InputByName(common.PortCANRx).Signals().ForEach(func(sig *signal.Signal) error {
			// Validate CAN frame
			frame, ok := sig.PayloadOrNil().(*codec.Frame)
			if !ok {
				return errors.New("failed to cast payload to CAN frame")
			}

			// Convert CAN frame to ISO-TP message (the request)
			isoReq, err := NewISOTPMessage().FromCANFrame(frame)
			if err != nil {
				return fmt.Errorf("failed to parse ISO-TP message: %w", err)
			}

			// Resolve request address
			addressingMode := PhysicalAddressing

			if frame.Id == FunctionalRequestID {
				addressingMode = FunctionalAddressing
			}
			this.Logger().Printf("received ISO-TP request: addressing mode: %s, req address: 0x%02X, sid: 0x%02X(%s), pid: 0x%02X(%s)", addressingMode, frame.Id, isoReq.ServiceID, isoReq.ServiceID.ToString(), isoReq.PID, isoReq.PID.ToString())

			if addressingMode == PhysicalAddressing && frame.Id != ld.PhysicalAddress {
				this.Logger().Printf(
					"skipping request: frame ID 0x%03X does not match physical address 0x%02X(AddressingMode: %v)",
					frame.Id, ld.PhysicalAddress, addressingMode,
				)
				return nil
			}

			// Check if addressing mode is supported
			services, ok := ld.Table[addressingMode]
			if !ok {
				return errors.New("addressing mode is not supported")
			}

			// Check if the service is supported
			params, ok := services[isoReq.ServiceID]
			if !ok {
				return errors.New("service is not supported")
			}

			// Check if parameter is supported
			reqHandler, ok := params[isoReq.PID]
			if !ok {
				this.Logger().Printf("skipping request: parameter 0x%02X (%s) is not supported", isoReq.PID, isoReq.PID.ToString())
				return nil
			}

			// Run logic and get the response
			isoResp, err := reqHandler(addressingMode, isoReq, this)
			if err != nil {
				return fmt.Errorf("failed to apply MCU logic: %w", err)
			}
			// For simplicity this demo supports only single frame responses, so any data that does not fit will be truncated:

			// Return response down to CAN controller
			respCANFrame, err := isoResp.ToCANFrame(ld.PhysicalAddress + ResponseAddressOffset)
			if err != nil {
				return fmt.Errorf("failed to convert ISOTP to CAN frame: %w", err)
			}
			this.OutputByName(common.PortCANTx).PutSignals(signal.New(respCANFrame))
			this.Logger().Printf("sending ISO-TP response: addressing mode: %s, req address: 0x%03X, sid: 0x%02X, pid: 0x%02X", addressingMode, respCANFrame.Id, isoResp.ServiceID, isoResp.PID)

			return nil
		}).ChainableErr()
	}

	return af
}
