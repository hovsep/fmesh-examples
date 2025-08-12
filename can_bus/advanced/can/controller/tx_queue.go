package controller

import "github.com/hovsep/fmesh-examples/can_bus/advanced/can/codec"

type TxQueueItem struct {
	Buf *codec.BitBuffer // Binary encoded frame, wih SOF, EOF, IFS and 1 extra bit
}

// TxQueue represents the queue of frames to be transmitted
type TxQueue []*TxQueueItem
