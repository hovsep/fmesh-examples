package main

import (
	"github.com/hovsep/fmesh-examples/can_bus/advanced/can/codec"
)

var (
	frameDiagnosticRequest = &codec.Frame{
		Id:  0x7DF,
		DLC: 1,
		Data: [8]byte{
			0x01,                         // Number of additional data bytes (service + PID)
			0x00,                         // Service ID: Show Current Data
			0x00,                         // PID: Engine RPM
			0x00, 0x00, 0x00, 0x00, 0x00, // Padding (ISO 15765-4)
		},
	}
)
