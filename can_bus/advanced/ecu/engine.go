package ecu

import (
	"errors"
	"github.com/hovsep/fmesh-examples/can_bus/advanced/internal/microcontroller"

	"github.com/hovsep/fmesh-examples/can_bus/advanced/internal/can"
	"github.com/hovsep/fmesh/component"
)

const (
	ECMUnitName        = "ecm"
	ECMPhysicalAddress = 0x7E0

	ecmPIDRPM                      microcontroller.ParameterID = 0x0C
	ecmPIDVehicleSpeed             microcontroller.ParameterID = 0x0D
	ecmPIDEngineCoolantTemperature microcontroller.ParameterID = 0x05
)

var (
	errPIDNotSupported = errors.New("PID not supported")
)

func NewECM() *can.Node {
	return can.NewNode(ECMUnitName, nil,
		microcontroller.LogicToActivationFunc(
			func(mode microcontroller.AddressingMode, request *microcontroller.ISOTPMessage, this *component.Component) (*microcontroller.ISOTPMessage, error) {
				switch mode {
				case microcontroller.Functional:
					return ecmHandleOBDFunctionalRequest(this, request)

				case microcontroller.Physical:
					return ecmHandleRequest(this, request)
				default:
					this.Logger().Printf("addressing mode not supported: %v", mode)
				}
				return nil, errors.New("something went wrong")
			}, ECMPhysicalAddress))
}

func ecmHandleOBDFunctionalRequest(this *component.Component, req *microcontroller.ISOTPMessage) (*microcontroller.ISOTPMessage, error) {
	switch req.ServiceID {
	case microcontroller.ServiceShowCurrentData:
		return handleServiceShowCurrentData(this, req)
	case microcontroller.ServiceReadStoredDiagnosticCodes:
	case microcontroller.ServiceVehicleInformation:
		switch req.PID {
		default:
			return nil, errPIDNotSupported
		}
	default:
		return nil, errors.New("ECM Service ID not supported")
	}

	return nil, errors.New("something went wrong")
}

func ecmHandleRequest(this *component.Component, req *microcontroller.ISOTPMessage) (*microcontroller.ISOTPMessage, error) {
	return nil, errors.New("physical addressing not supported")
}

func encodeRPM(rpm int) (byte, byte) {
	raw := rpm * 4
	hi := byte((raw >> 8) & 0xFF)
	lo := byte(raw & 0xFF)
	return hi, lo
}

func handleServiceShowCurrentData(this *component.Component, req *microcontroller.ISOTPMessage) (*microcontroller.ISOTPMessage, error) {
	switch req.PID {
	case ecmPIDRPM:
		return getRPM(), nil
	case ecmPIDVehicleSpeed:
		return getSpeed(), nil
	case ecmPIDEngineCoolantTemperature:
		return getCoolantTemp(), nil
	default:
		return nil, errPIDNotSupported
	}
	return nil, errPIDNotSupported
}

func getRPM() *microcontroller.ISOTPMessage {
	currentRPM := 3571 // todo get from somewhere
	rpmHi, rpmLow := encodeRPM(currentRPM)
	return &microcontroller.ISOTPMessage{
		Len:       0x04,
		ServiceID: microcontroller.ResponseShowCurrentData,
		PID:       ecmPIDRPM,
		Data:      []byte{rpmHi, rpmLow},
	}
}

func getSpeed() *microcontroller.ISOTPMessage {
	currentSpeed := byte(65) // todo
	return &microcontroller.ISOTPMessage{
		Len:       0x03,
		ServiceID: microcontroller.ResponseShowCurrentData,
		PID:       ecmPIDVehicleSpeed,
		Data:      []byte{currentSpeed},
	}
}

func getCoolantTemp() *microcontroller.ISOTPMessage {
	currentCoolantTemp := byte(92)
	return &microcontroller.ISOTPMessage{
		Len:       0x03,
		ServiceID: microcontroller.ResponseShowCurrentData,
		PID:       ecmPIDEngineCoolantTemperature,
		Data:      []byte{currentCoolantTemp},
	}
}
