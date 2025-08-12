package engine

import (
	"github.com/hovsep/fmesh-example/can_bus/advanced/can"
	"github.com/hovsep/fmesh-example/can_bus/advanced/microcontroller"
	"github.com/hovsep/fmesh/component"
)

const (
	ECMUnitName        = "ecm"
	ECMPhysicalAddress = 0x7E0

	ecmPIDRPM                microcontroller.ParameterID = 0x0C
	ecmPIDVehicleSpeed       microcontroller.ParameterID = 0x0D
	ecmPIDVIN                microcontroller.ParameterID = 0x02
	ecmPIDCalibrationID      microcontroller.ParameterID = 0x04
	ecmPIDCoolantTemperature microcontroller.ParameterID = 0x05

	stateKeyParams = "params"
	stateKeyDTCs   = "dtcs"
)

var (
	// Supported trouble codes
	dtcP010C = microcontroller.DTC{0x01, 0x0C} // Mass or Volume Air Flow Circuit High Input
	dtcP0300 = microcontroller.DTC{0x03, 0x00} // Random/Multiple Cylinder Misfire

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
				microcontroller.ServiceReadStoredDiagnosticCodes: microcontroller.ParamsMap{
					microcontroller.NoPID: getStoredDTCs,
				},
				microcontroller.ServiceVehicleInformation: microcontroller.ParamsMap{
					ecmPIDVIN:           getVIN,
					ecmPIDCalibrationID: getCalibrationID,
				},
			},
			microcontroller.PhysicalAddressing: {
				microcontroller.ServiceShowCurrentData: microcontroller.ParamsMap{
					ecmPIDRPM:                getRPMParam,
					ecmPIDVehicleSpeed:       getSpeedParam,
					ecmPIDCoolantTemperature: getCoolantTempParam,
				},
				microcontroller.ServiceReadStoredDiagnosticCodes: microcontroller.ParamsMap{
					microcontroller.NoPID: getStoredDTCs,
				},
				microcontroller.ServiceVehicleInformation: microcontroller.ParamsMap{
					ecmPIDVIN:           getVIN,
					ecmPIDCalibrationID: getCalibrationID,
				},
			},
		},
	}
)

func NewNode() *can.Node {
	return can.NewNode(ECMUnitName, func(state component.State) {
		// Current state of params
		paramsState := microcontroller.ParamsState{
			ecmPIDRPM:                1984,
			ecmPIDVehicleSpeed:       byte(34),
			ecmPIDCoolantTemperature: byte(95),
			ecmPIDVIN:                []byte("VF1AB000123456789"),
			ecmPIDCalibrationID:      []byte("ECM-A1234-B5678"),
		}

		state.Set(stateKeyParams, paramsState)

		// Current state of DTCs.
		// Due to limitations of this example we support
		// only 2 DTC's maximum, so they can fit into a single frame
		DTCsState := []microcontroller.DTC{
			dtcP010C,
			dtcP0300,
		}

		state.Set(stateKeyDTCs, DTCsState)
	}, logicDescriptor.ToActivationFunc())
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

func encodeRPM(rpm int) (byte, byte) {
	raw := rpm * 4
	hi := byte((raw >> 8) & 0xFF)
	lo := byte(raw & 0xFF)
	return hi, lo
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

func getStoredDTCs(mode microcontroller.AddressingMode, request *microcontroller.ISOTPMessage, mcu *component.Component) (*microcontroller.ISOTPMessage, error) {
	dtcs := mcu.State().Get(stateKeyDTCs).([]microcontroller.DTC)

	var dtcData []byte
	for _, dtc := range dtcs {
		dtcData = append(dtcData, dtc[0], dtc[1])
	}

	return &microcontroller.ISOTPMessage{
		ServiceID: microcontroller.ResponseReadStoredDiagnosticCodes,
		Data:      dtcData,
	}, nil
}

func getVIN(mode microcontroller.AddressingMode, request *microcontroller.ISOTPMessage, mcu *component.Component) (*microcontroller.ISOTPMessage, error) {
	paramsState := mcu.State().Get(stateKeyParams).(microcontroller.ParamsState)
	vin := paramsState[ecmPIDVIN].([]byte)
	return &microcontroller.ISOTPMessage{
		ServiceID: microcontroller.ResponseVehicleInformation,
		PID:       ecmPIDVIN,
		Data:      microcontroller.FitDataIntoSingleFrame(vin),
	}, nil
}

func getCalibrationID(mode microcontroller.AddressingMode, request *microcontroller.ISOTPMessage, mcu *component.Component) (*microcontroller.ISOTPMessage, error) {
	paramsState := mcu.State().Get(stateKeyParams).(microcontroller.ParamsState)
	id := paramsState[ecmPIDCalibrationID].([]byte)
	return &microcontroller.ISOTPMessage{
		ServiceID: microcontroller.ResponseVehicleInformation,
		PID:       ecmPIDCalibrationID,
		Data:      microcontroller.FitDataIntoSingleFrame(id),
	}, nil
}
