package ecu

import (
	"errors"
	"fmt"
	"github.com/hovsep/fmesh-examples/can_bus/advanced/can"
	"github.com/hovsep/fmesh-examples/can_bus/advanced/can/codec"
	"github.com/hovsep/fmesh-examples/can_bus/advanced/can/common"
	"github.com/hovsep/fmesh/component"
	"github.com/hovsep/fmesh/signal"
)

const (
	ECMUnitName   = "ecm"
	ecmRequestID  = 0x7E0
	ecmResponseID = 0x7E8

	ecmServiceIDShowCurrentData              = 0x01
	ecmServiceIDStoredDiagnosticTroubleCodes = 0x03
	ecmServiceIDVehicleInformation           = 0x09

	ecmPIDRPM                      = 0x0C
	ecmPIDVehicleSpeed             = 0x0D
	ecmPIDEngineCoolantTemperature = 0x05
	ecmPIDECUName                  = 0x0A
)

var (
	errPIDNotSupported = errors.New("PID not supported")
)

func NewECM() *can.Node {
	return can.NewNode(ECMUnitName, func(state component.State) {
		state.Set(EcuMemLog, []string{})

		setParam(state, ecmPIDRPM, 3000)
		setParam(state, ecmPIDVehicleSpeed, 0)
		setParam(state, ecmPIDEngineCoolantTemperature, 28.5)
		setParam(state, ecmPIDECUName, "ENGINE_CONTROL_1")
	}, ecmLogic)
}

func ecmLogic(this *component.Component) error {
	for _, sig := range this.InputByName(common.PortCANRx).AllSignalsOrNil() {
		frame, ok := sig.PayloadOrNil().(*codec.Frame)
		if !ok {
			return errors.New("got corrupted frame")
		}
		message, err := frame.ToISOTPMessage()
		if err != nil {
			return fmt.Errorf("failed to parse ISO-TP message: %w", err)
		}

		// TODO: move this to separate function which returns iso message, then just convert any results to can frames and send them
		switch frame.Id {
		case ObdFunctionalRequestID:
			return ecmHandleOBDFR(this, message)

		case ecmRequestID:
			return ecmHandleRequest(this, message)
		default:
			this.Logger().Println("frame id is not supported:", frame.Id)
		}
	}

	return nil
}

func ecmHandleOBDFR(this *component.Component, msg *codec.ISOTPMessage) error {
	switch msg.ServiceID {
	case ecmServiceIDShowCurrentData:
		switch msg.PID {
		case ecmPIDRPM:
			currentRPM := getParam(this.State(), ecmPIDRPM).(int)
			rpmHi, rpmLow := encodeRPM(currentRPM)
			responseFrame, err := codec.FromISOTPMessage(&codec.ISOTPMessage{
				Len:       0x04,
				ServiceID: 0x41,
				PID:       0x0C,
				Data:      []byte{rpmHi, rpmLow},
			}, ecmResponseID)

			if err != nil {
				return fmt.Errorf("failed to encode RPM: %w", err)
			}

			this.OutputByName(common.PortCANTx).PutSignals(signal.New(responseFrame))
			return nil
		case ecmPIDVehicleSpeed:
		case ecmPIDEngineCoolantTemperature:
		default:
			return errPIDNotSupported
		}
	case ecmServiceIDStoredDiagnosticTroubleCodes:
	case ecmServiceIDVehicleInformation:
		switch msg.PID {
		case ecmPIDECUName:
		default:
			return errPIDNotSupported
		}
	default:
		return errors.New("ECM Service ID not supported")
	}
	return nil
}

func ecmHandleRequest(this *component.Component, msg *codec.ISOTPMessage) error {
	return nil
}

func encodeRPM(rpm int) (byte, byte) {
	raw := rpm * 4
	hi := byte((raw >> 8) & 0xFF)
	lo := byte(raw & 0xFF)
	return hi, lo
}
