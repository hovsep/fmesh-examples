package main

import "github.com/hovsep/fmesh-examples/can_bus/v1/can"

var (
	frameStartEngine = &can.Frame{
		Id:   0x100,
		DLC:  1,
		Data: [8]byte{0x01},
	}
)
