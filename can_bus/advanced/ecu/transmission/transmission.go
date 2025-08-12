package transmission

import (
	"github.com/hovsep/fmesh-example/can_bus/advanced/can"
	"github.com/hovsep/fmesh-example/can_bus/advanced/microcontroller"
	"github.com/hovsep/fmesh/component"
)

// AI gen, needs review

const (
	TCMUnitName        = "tcm"
	TCMPhysicalAddress = 0x7E1

	tcmPIDFluidTemp     microcontroller.ParameterID = 0xA0
	tcmPIDGearPosition  microcontroller.ParameterID = 0xA1
	tcmPIDVIN           microcontroller.ParameterID = 0x02
	tcmPIDCalibrationID microcontroller.ParameterID = 0x04

	tcmStateKeyParams = "params"
	tcmStateKeyDTCs   = "dtcs"
)

var (
	P0710 = microcontroller.DTC{0x07, 0x10} // Fluid Temp Sensor Circuit High
	P0705 = microcontroller.DTC{0x07, 0x05} // Range Sensor Circuit Malfunction

	tcmLogic = &microcontroller.LogicDescriptor{
		PhysicalAddress: TCMPhysicalAddress,
		Table: microcontroller.LogicMap{
			microcontroller.FunctionalAddressing: {
				microcontroller.ServiceShowCurrentData: microcontroller.ParamsMap{
					tcmPIDFluidTemp:    getFluidTemperature,
					tcmPIDGearPosition: getGearPosition,
				},
				microcontroller.ServiceReadStoredDiagnosticCodes: microcontroller.ParamsMap{
					microcontroller.NoPID: getDTCs,
				},
				microcontroller.ServiceVehicleInformation: microcontroller.ParamsMap{
					tcmPIDVIN:           getVIN,
					tcmPIDCalibrationID: getCalibrationID,
				},
			},
			microcontroller.PhysicalAddressing: {
				microcontroller.ServiceShowCurrentData: microcontroller.ParamsMap{
					tcmPIDFluidTemp:    getFluidTemperature,
					tcmPIDGearPosition: getGearPosition,
				},
				microcontroller.ServiceReadStoredDiagnosticCodes: microcontroller.ParamsMap{
					microcontroller.NoPID: getDTCs,
				},
				microcontroller.ServiceVehicleInformation: microcontroller.ParamsMap{
					tcmPIDVIN:           getVIN,
					tcmPIDCalibrationID: getCalibrationID,
				},
			},
		},
	}
)

func NewNode() *can.Node {
	return can.NewNode(TCMUnitName, func(state component.State) {
		// Set parameter values
		paramState := microcontroller.ParamsState{
			tcmPIDFluidTemp:     byte(88),
			tcmPIDGearPosition:  byte(3),
			tcmPIDVIN:           []byte("VF1TC000987654321"),
			tcmPIDCalibrationID: []byte("TCM-C9999-Z8888"),
		}
		state.Set(tcmStateKeyParams, paramState)

		// Set DTCs
		dtcs := []microcontroller.DTC{
			P0710,
			P0705,
		}
		state.Set(tcmStateKeyDTCs, dtcs)
	}, tcmLogic.ToActivationFunc())
}

func getFluidTemperature(mode microcontroller.AddressingMode, req *microcontroller.ISOTPMessage, mcu *component.Component) (*microcontroller.ISOTPMessage, error) {
	state := mcu.State().Get(tcmStateKeyParams).(microcontroller.ParamsState)
	temp := state[tcmPIDFluidTemp].(byte)
	return &microcontroller.ISOTPMessage{
		ServiceID: microcontroller.ResponseShowCurrentData,
		PID:       tcmPIDFluidTemp,
		Data:      []byte{temp},
	}, nil
}

func getGearPosition(mode microcontroller.AddressingMode, req *microcontroller.ISOTPMessage, mcu *component.Component) (*microcontroller.ISOTPMessage, error) {
	state := mcu.State().Get(tcmStateKeyParams).(microcontroller.ParamsState)
	pos := state[tcmPIDGearPosition].(byte)
	return &microcontroller.ISOTPMessage{
		ServiceID: microcontroller.ResponseShowCurrentData,
		PID:       tcmPIDGearPosition,
		Data:      []byte{pos},
	}, nil
}

func getDTCs(mode microcontroller.AddressingMode, req *microcontroller.ISOTPMessage, mcu *component.Component) (*microcontroller.ISOTPMessage, error) {
	dtcs := mcu.State().Get(tcmStateKeyDTCs).([]microcontroller.DTC)

	var data []byte
	for _, dtc := range dtcs {
		data = append(data, dtc[0], dtc[1])
	}

	return &microcontroller.ISOTPMessage{
		ServiceID: microcontroller.ResponseReadStoredDiagnosticCodes,
		Data:      data,
	}, nil
}

func getVIN(mode microcontroller.AddressingMode, req *microcontroller.ISOTPMessage, mcu *component.Component) (*microcontroller.ISOTPMessage, error) {
	state := mcu.State().Get(tcmStateKeyParams).(microcontroller.ParamsState)
	vin := state[tcmPIDVIN].([]byte)
	return &microcontroller.ISOTPMessage{
		ServiceID: microcontroller.ResponseVehicleInformation,
		PID:       tcmPIDVIN,
		Data:      microcontroller.FitDataIntoSingleFrame(vin),
	}, nil
}

func getCalibrationID(mode microcontroller.AddressingMode, req *microcontroller.ISOTPMessage, mcu *component.Component) (*microcontroller.ISOTPMessage, error) {
	state := mcu.State().Get(tcmStateKeyParams).(microcontroller.ParamsState)
	id := state[tcmPIDCalibrationID].([]byte)
	return &microcontroller.ISOTPMessage{
		ServiceID: microcontroller.ResponseVehicleInformation,
		PID:       tcmPIDCalibrationID,
		Data:      microcontroller.FitDataIntoSingleFrame(id),
	}, nil
}
