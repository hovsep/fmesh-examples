package ecu

import (
	"github.com/hovsep/fmesh-examples/can_bus/advanced/internal/can"
	"github.com/hovsep/fmesh-examples/can_bus/advanced/internal/microcontroller"
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
	logicDescriptor = &microcontroller.LogicDescriptor{
		PhysicalAddress: ECMPhysicalAddress,
		Table: microcontroller.LogicMap{
			microcontroller.FunctionalAddressing: {
				microcontroller.ServiceShowCurrentData: microcontroller.ParamsMap{
					ecmPIDRPM:                      getRPMParam,
					ecmPIDVehicleSpeed:             getSpeedParam,
					ecmPIDEngineCoolantTemperature: getCoolantTempParam,
				},
				microcontroller.ServiceReadStoredDiagnosticCodes: microcontroller.ParamsMap{},
				microcontroller.ServiceVehicleInformation:        microcontroller.ParamsMap{},
			},
			microcontroller.PhysicalAddressing: {},
		},
	}
)

func NewECM() *can.Node {
	return can.NewNode(ECMUnitName, func(state component.State) {

	}, logicDescriptor.ToActivationFunc())
}

func encodeRPM(rpm int) (byte, byte) {
	raw := rpm * 4
	hi := byte((raw >> 8) & 0xFF)
	lo := byte(raw & 0xFF)
	return hi, lo
}

func getSpeedParam(mode microcontroller.AddressingMode, request *microcontroller.ISOTPMessage, mcu *component.Component) (*microcontroller.ISOTPMessage, error) {
	currentSpeed := byte(65) // todo
	return &microcontroller.ISOTPMessage{
		Len:       0x03,
		ServiceID: microcontroller.ResponseShowCurrentData,
		PID:       ecmPIDVehicleSpeed,
		Data:      []byte{currentSpeed},
	}, nil
}

func getRPMParam(mode microcontroller.AddressingMode, request *microcontroller.ISOTPMessage, mcu *component.Component) (*microcontroller.ISOTPMessage, error) {
	currentRPM := 3571 // todo get from somewhere
	rpmHi, rpmLow := encodeRPM(currentRPM)
	return &microcontroller.ISOTPMessage{
		Len:       0x04,
		ServiceID: microcontroller.ResponseShowCurrentData,
		PID:       ecmPIDRPM,
		Data:      []byte{rpmHi, rpmLow},
	}, nil
}

func getCoolantTempParam(mode microcontroller.AddressingMode, request *microcontroller.ISOTPMessage, mcu *component.Component) (*microcontroller.ISOTPMessage, error) {
	currentCoolantTemp := byte(92)
	return &microcontroller.ISOTPMessage{
		Len:       0x03,
		ServiceID: microcontroller.ResponseShowCurrentData,
		PID:       ecmPIDEngineCoolantTemperature,
		Data:      []byte{currentCoolantTemp},
	}, nil
}
