package main

import (
	"github.com/hovsep/fmesh-examples/can_bus/advanced/can/codec"
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
)
