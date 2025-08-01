package main

import (
	"github.com/hovsep/fmesh-examples/can_bus/advanced/internal/can/codec"
)

var (
	diagnosticFrameGetRPM = &codec.Frame{
		Id:  0x7DF,
		DLC: 8,
		Data: [8]byte{
			0x02,                         // Number of additional data bytes (service + PID)
			0x01,                         // Service ID: Show Current Data
			0x0C,                         // PID: Engine RPM
			0x00, 0x00, 0x00, 0x00, 0x00, // Padding (ISO-TP)
		},
	}

	diagnosticFrameGetSpeed = &codec.Frame{
		Id:  0x7DF,
		DLC: 8,
		Data: [8]byte{
			0x02,
			0x01,
			0x0D,
			0x00, 0x00, 0x00, 0x00, 0x00,
		},
	}

	diagnosticFrameGetCoolantTemp = &codec.Frame{
		Id:  0x7DF,
		DLC: 8,
		Data: [8]byte{
			0x02,
			0x01,
			0x05,
			0x00, 0x00, 0x00, 0x00, 0x00,
		},
	}

	diagnosticFrameGetEngineDTCs = &codec.Frame{
		Id:  0x7E0, // Physical address of the engine ECU (not functional broadcast)
		DLC: 8,
		Data: [8]byte{
			0x02,                         // Number of data bytes (1 for service, 0 for PID)
			0x03,                         // Service ID: Request Stored Diagnostic Trouble Codes
			0x00,                         // No PID (padding or dummy — optional)
			0x00, 0x00, 0x00, 0x00, 0x00, // Padding
		},
	}
)
