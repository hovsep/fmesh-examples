package ecu

import (
	"github.com/hovsep/fmesh-examples/can_bus/advanced/internal/can"
	"github.com/hovsep/fmesh-examples/can_bus/advanced/internal/microcontroller"
	"github.com/hovsep/fmesh/component"
)

// AI gen, needs review

const (
	TCMUnitName        = "tcm"
	TCMPhysicalAddress = 0x7E1

	tcmPIDFluidTemp     microcontroller.ParameterID = 0x0F // example PID
	tcmPIDGearPosition  microcontroller.ParameterID = 0xA1 // custom example PID
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
			microcontroller.FunctionalAddressing: serviceMap(),
			microcontroller.PhysicalAddressing:   serviceMap(),
		},
	}
)

func NewTCM() *can.Node {
	return can.NewNode(TCMUnitName, func(state component.State) {
		// Set parameter values
		paramState := microcontroller.ParamsState{
			tcmPIDFluidTemp:     byte(85),                    // e.g. 85°C
			tcmPIDGearPosition:  byte(3),                     // e.g. Drive
			tcmPIDVIN:           []byte("VF1TC000987654321"), // Different VIN
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

func serviceMap() microcontroller.ServiceMap {
	return microcontroller.ServiceMap{
		microcontroller.ServiceShowCurrentData: microcontroller.ParamsMap{
			tcmPIDFluidTemp:    getFluidTempParam,
			tcmPIDGearPosition: getGearPosParam,
		},
		microcontroller.ServiceReadStoredDiagnosticCodes: microcontroller.ParamsMap{
			microcontroller.NoPID: getTCMDTCs,
		},
		microcontroller.ServiceVehicleInformation: microcontroller.ParamsMap{
			tcmPIDVIN:           getTCMVIN,
			tcmPIDCalibrationID: getTCMCalibrationID,
		},
	}
}

func getFluidTempParam(mode microcontroller.AddressingMode, req *microcontroller.ISOTPMessage, mcu *component.Component) (*microcontroller.ISOTPMessage, error) {
	state := mcu.State().Get(tcmStateKeyParams).(microcontroller.ParamsState)
	temp := state[tcmPIDFluidTemp].(byte)
	return &microcontroller.ISOTPMessage{
		ServiceID: microcontroller.ResponseShowCurrentData,
		PID:       tcmPIDFluidTemp,
		Data:      []byte{temp},
	}, nil
}

func getGearPosParam(mode microcontroller.AddressingMode, req *microcontroller.ISOTPMessage, mcu *component.Component) (*microcontroller.ISOTPMessage, error) {
	state := mcu.State().Get(tcmStateKeyParams).(microcontroller.ParamsState)
	pos := state[tcmPIDGearPosition].(byte)
	return &microcontroller.ISOTPMessage{
		ServiceID: microcontroller.ResponseShowCurrentData,
		PID:       tcmPIDGearPosition,
		Data:      []byte{pos},
	}, nil
}

func getTCMDTCs(mode microcontroller.AddressingMode, req *microcontroller.ISOTPMessage, mcu *component.Component) (*microcontroller.ISOTPMessage, error) {
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

func getTCMVIN(mode microcontroller.AddressingMode, req *microcontroller.ISOTPMessage, mcu *component.Component) (*microcontroller.ISOTPMessage, error) {
	state := mcu.State().Get(tcmStateKeyParams).(microcontroller.ParamsState)
	vin := state[tcmPIDVIN].([]byte)
	return &microcontroller.ISOTPMessage{
		ServiceID: microcontroller.ResponseVehicleInformation,
		PID:       tcmPIDVIN,
		Data:      microcontroller.FitDataIntoSingleFrame(vin),
	}, nil
}

func getTCMCalibrationID(mode microcontroller.AddressingMode, req *microcontroller.ISOTPMessage, mcu *component.Component) (*microcontroller.ISOTPMessage, error) {
	state := mcu.State().Get(tcmStateKeyParams).(microcontroller.ParamsState)
	id := state[tcmPIDCalibrationID].([]byte)
	return &microcontroller.ISOTPMessage{
		ServiceID: microcontroller.ResponseVehicleInformation,
		PID:       tcmPIDCalibrationID,
		Data:      microcontroller.FitDataIntoSingleFrame(id),
	}, nil
}
