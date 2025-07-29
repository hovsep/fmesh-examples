package ecu

import (
	"github.com/hovsep/fmesh-examples/can_bus/advanced/diagnostics"
	"github.com/hovsep/fmesh-examples/can_bus/advanced/internal/can"
	"github.com/hovsep/fmesh-examples/can_bus/advanced/internal/microcontroller"
	"github.com/hovsep/fmesh/component"
)

const (
	ECMUnitName        = "ecm"
	ECMPhysicalAddress = 0x7E0

	ecmPIDRPM                microcontroller.ParameterID = 0x0C
	ecmPIDVehicleSpeed       microcontroller.ParameterID = 0x0D
	ecmPIDCoolantTemperature microcontroller.ParameterID = 0x05

	stateKeyParams = "params"
	stateKeyDTCs   = "dtcs"
)

var (
	// Supported trouble codes
	dtcP010C = diagnostics.DTC{0x01, 0x0C} //Mass or Volume Air Flow Circuit High Input
	dtcP0011 = diagnostics.DTC{0x00, 0x11} //Camshaft Position Timing Over-Advanced
	dtcP0300 = diagnostics.DTC{0x03, 0x00} //Random/Multiple Cylinder Misfire

	// The "brain" of this unit
	logicDescriptor = &microcontroller.LogicDescriptor{
		PhysicalAddress: ECMPhysicalAddress,
		Table: microcontroller.LogicMap{
			microcontroller.FunctionalAddressing: {
				microcontroller.ServiceShowCurrentData: microcontroller.ParamsMap{
					ecmPIDRPM:                getRPMParam,
					ecmPIDVehicleSpeed:       getSpeedParam,
					ecmPIDCoolantTemperature: getCoolantTempParam,
				},
				microcontroller.ServiceReadStoredDiagnosticCodes: microcontroller.ParamsMap{},
				microcontroller.ServiceVehicleInformation:        microcontroller.ParamsMap{},
			},
			microcontroller.PhysicalAddressing: {
				microcontroller.ServiceShowCurrentData: microcontroller.ParamsMap{
					ecmPIDRPM:                getRPMParam,
					ecmPIDVehicleSpeed:       getSpeedParam,
					ecmPIDCoolantTemperature: getCoolantTempParam,
				},
			},
		},
	}
)

func NewECM() *can.Node {
	return can.NewNode(ECMUnitName, func(state component.State) {
		// Current state of params
		paramsState := microcontroller.ParamsState{
			ecmPIDRPM:                1984, //1 works, 3000 works, but 1984 brakes
			ecmPIDVehicleSpeed:       byte(34),
			ecmPIDCoolantTemperature: 95,
		}

		state.Set(stateKeyParams, paramsState)

		// Current state of DTCs
		DTCsState := []diagnostics.DTC{
			dtcP010C,
			dtcP0011,
			dtcP0300,
		}

		state.Set(stateKeyDTCs, DTCsState)
	}, logicDescriptor.ToActivationFunc())
}

func encodeRPM(rpm int) (byte, byte) {
	raw := rpm * 4
	hi := byte((raw >> 8) & 0xFF)
	lo := byte(raw & 0xFF)
	return hi, lo
}

func getSpeedParam(mode microcontroller.AddressingMode, request *microcontroller.ISOTPMessage, mcu *component.Component) (*microcontroller.ISOTPMessage, error) {
	paramsState := mcu.State().Get(stateKeyParams).(microcontroller.ParamsState)
	currentSpeed := paramsState[ecmPIDVehicleSpeed].(byte)
	return &microcontroller.ISOTPMessage{
		ServiceID: microcontroller.ResponseShowCurrentData,
		PID:       ecmPIDVehicleSpeed,
		Data:      []byte{currentSpeed},
	}, nil
}

func getRPMParam(mode microcontroller.AddressingMode, request *microcontroller.ISOTPMessage, mcu *component.Component) (*microcontroller.ISOTPMessage, error) {
	paramsState := mcu.State().Get(stateKeyParams).(microcontroller.ParamsState)
	currentRPM := paramsState[ecmPIDRPM].(int)
	rpmHi, rpmLow := encodeRPM(currentRPM)
	return &microcontroller.ISOTPMessage{
		ServiceID: microcontroller.ResponseShowCurrentData,
		PID:       ecmPIDRPM,
		Data:      []byte{rpmHi, rpmLow},
	}, nil
}

func getCoolantTempParam(mode microcontroller.AddressingMode, request *microcontroller.ISOTPMessage, mcu *component.Component) (*microcontroller.ISOTPMessage, error) {
	paramsState := mcu.State().Get(stateKeyParams).(microcontroller.ParamsState)
	currentCoolantTemp := paramsState[ecmPIDCoolantTemperature].(byte)
	return &microcontroller.ISOTPMessage{
		ServiceID: microcontroller.ResponseShowCurrentData,
		PID:       ecmPIDCoolantTemperature,
		Data:      []byte{currentCoolantTemp},
	}, nil
}
