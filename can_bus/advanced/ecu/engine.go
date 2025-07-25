package ecu

import (
	"errors"
	"fmt"
	"github.com/hovsep/fmesh-examples/can_bus/advanced/internal/microcontroller"

	"github.com/hovsep/fmesh-examples/can_bus/advanced/internal/can"
	"github.com/hovsep/fmesh-examples/can_bus/advanced/internal/can/codec"
	"github.com/hovsep/fmesh-examples/can_bus/advanced/internal/can/common"
	"github.com/hovsep/fmesh/component"
	"github.com/hovsep/fmesh/signal"
)

const (
	ECMUnitName = "ecm"

	ecmPIDRPM                      microcontroller.ParameterID = 0x0C
	ecmPIDVehicleSpeed             microcontroller.ParameterID = 0x0D
	ecmPIDEngineCoolantTemperature microcontroller.ParameterID = 0x05
)

var (
	errPIDNotSupported = errors.New("PID not supported")
)

func NewECM() *can.Node {
	return can.NewNode(ECMUnitName, nil, ecmLogic)
}

func ecmLogic(this *component.Component) error {

	for _, sig := range this.InputByName(common.PortCANRx).AllSignalsOrNil() {
		frame, ok := sig.PayloadOrNil().(*codec.Frame)
		if !ok {
			return errors.New("got corrupted frame")
		}
		message, err := microcontroller.CANFrameToISOTP(frame)
		if err != nil {
			return fmt.Errorf("failed to parse ISO-TP message: %w", err)
		}

		this.Logger().Println("received ISO-TP message")

		addressingMode := microcontroller.AddressingMode(frame.Id)

		// TODO: move this to separate function which returns iso message, then just convert any results to can frames and send them
		switch addressingMode {
		case microcontroller.Functional:
			return ecmHandleOBDFunctionalRequest(this, message)

		case microcontroller.Physical:
			return ecmHandleRequest(this, message)
		default:
			this.Logger().Println("frame id is not supported:", frame.Id)
		}
	}

	return nil
}

func ecmHandleOBDFunctionalRequest(this *component.Component, msg *microcontroller.ISOTPMessage) error {

	sid := microcontroller.ServiceID((msg.ServiceID))

	switch sid {
	case microcontroller.ServiceShowCurrentData:
		return handleServiceShowCurrentData(this, msg)
	case microcontroller.ServiceReadStoredDiagnosticCodes:
	case microcontroller.ServiceVehicleInformation:
		pid := microcontroller.ParameterID((msg.PID))
		switch pid {
		default:
			return errPIDNotSupported
		}
	default:
		return errors.New("ECM Service ID not supported")
	}
	return nil
}

func ecmHandleRequest(this *component.Component, msg *microcontroller.ISOTPMessage) error {
	return nil
}

func encodeRPM(rpm int) (byte, byte) {
	raw := rpm * 4
	hi := byte((raw >> 8) & 0xFF)
	lo := byte(raw & 0xFF)
	return hi, lo
}

func handleServiceShowCurrentData(this *component.Component, msg *microcontroller.ISOTPMessage) error {
	switch microcontroller.ParameterID(msg.PID) {
	case ecmPIDRPM:
		currentRPMFrame, err := getRPM(this)
		if err != nil {
			return fmt.Errorf("failed to get current RPM: %w", err)
		}
		this.OutputByName(common.PortCANTx).PutSignals(signal.New(currentRPMFrame))
		return nil
	case ecmPIDVehicleSpeed:
		currentSpeedFrame, err := getSpeed(this)
		if err != nil {
			return fmt.Errorf("failed to get current speed: %w", err)
		}
		this.OutputByName(common.PortCANTx).PutSignals(signal.New(currentSpeedFrame))
		return nil
	case ecmPIDEngineCoolantTemperature:
		currentCoolantTemp, err := getCoolantTemp(this)
		if err != nil {
			return fmt.Errorf("failed to get coolant temperature: %w", err)
		}
		this.OutputByName(common.PortCANTx).PutSignals(signal.New(currentCoolantTemp))
	default:
		return errPIDNotSupported
	}
	return nil
}

func getRPM(this *component.Component) (*codec.Frame, error) {
	currentRPM := 3571 // todo get from somewhere
	rpmHi, rpmLow := encodeRPM(currentRPM)
	responseFrame, err := microcontroller.ISOTPToCANFrame(&microcontroller.ISOTPMessage{
		Len:       0x04,
		ServiceID: microcontroller.ResponseShowCurrentData,
		PID:       ecmPIDRPM,
		Data:      []byte{rpmHi, rpmLow},
	}, uint32(microcontroller.ResponseShowCurrentData))

	if err != nil {
		return nil, err
	}

	return responseFrame, nil
}

func getSpeed(this *component.Component) (*codec.Frame, error) {
	currentSpeed := byte(65) // todo
	responseFrame, err := microcontroller.ISOTPToCANFrame(&microcontroller.ISOTPMessage{
		Len:       0x03,
		ServiceID: microcontroller.ResponseShowCurrentData,
		PID:       ecmPIDVehicleSpeed,
		Data:      []byte{currentSpeed},
	}, uint32(microcontroller.ResponseShowCurrentData))

	if err != nil {
		return nil, err
	}

	return responseFrame, nil
}

func getCoolantTemp(this *component.Component) (*codec.Frame, error) {
	currentCoolantTemp := byte(92)
	responseFrame, err := microcontroller.ISOTPToCANFrame(&microcontroller.ISOTPMessage{
		Len:       0x03,
		ServiceID: microcontroller.ResponseShowCurrentData,
		PID:       ecmPIDEngineCoolantTemperature,
		Data:      []byte{currentCoolantTemp},
	}, uint32(microcontroller.ResponseShowCurrentData))

	if err != nil {
		return nil, err
	}

	return responseFrame, nil
}
