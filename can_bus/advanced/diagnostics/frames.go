package diagnostics

import (
	"github.com/hovsep/fmesh-examples/can_bus/advanced/can/codec"
)

// Raw CAN frames used for diagnostics via OBD socket

var (
	FrameGetRPM = &codec.Frame{
		Id:  0x7DF,
		DLC: 8,
		Data: [8]byte{
			0x02,                         // Number of additional data bytes (service + PID)
			0x01,                         // Service ID: Show Current Data
			0x0C,                         // PID: Engine RPM
			0x00, 0x00, 0x00, 0x00, 0x00, // Padding (ISO-TP)
		},
	}

	FrameGetSpeed = &codec.Frame{
		Id:  0x7DF,
		DLC: 8,
		Data: [8]byte{
			0x02,
			0x01,
			0x0D,
			0x00, 0x00, 0x00, 0x00, 0x00,
		},
	}

	FrameGetCoolantTemperature = &codec.Frame{
		Id:  0x7DF,
		DLC: 8,
		Data: [8]byte{
			0x02,
			0x01,
			0x05,
			0x00, 0x00, 0x00, 0x00, 0x00,
		},
	}

	FrameGetEngineDTCs = &codec.Frame{
		Id:  0x7E0, // Physical address of the engine ECU (not functional broadcast)
		DLC: 8,
		Data: [8]byte{
			0x02,
			0x03,
			0x00,
			0x00, 0x00, 0x00, 0x00, 0x00,
		},
	}

	FrameGetVIN = &codec.Frame{
		Id:  0x7DF,
		DLC: 8,
		Data: [8]byte{
			0x02,
			0x09,
			0x02,
			0x00, 0x00, 0x00, 0x00, 0x00,
		},
	}

	FrameGetCalibrationID = &codec.Frame{
		Id:  0x7DF,
		DLC: 8,
		Data: [8]byte{
			0x02,
			0x09,
			0x04,
			0x00, 0x00, 0x00, 0x00, 0x00,
		},
	}

	FrameGetTransmissionFluidTemperature = &codec.Frame{
		Id:  0x7E1,
		DLC: 8,
		Data: [8]byte{
			0x02,
			0x01,
			0xA0,
			0x00, 0x00, 0x00, 0x00, 0x00,
		},
	}
)
