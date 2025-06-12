package controller

import "github.com/hovsep/fmesh-examples/can_bus/advanced/can/codec"

type TxQueueItem struct {
	Buf            *codec.BitBuffer // Binary encoded frame, wih SOF, EOF, IFS and 1 extra bit
	LastIDBitIndex int              // Marks the end of stuffed ID field, so the controller can check for arbitration winning TODO: get rid of this if possible
}

// TxQueue represents the queue of frames to be transmitted
type TxQueue []*TxQueueItem

func (queueItem *TxQueueItem) IDIsTransmitted() bool {
	return queueItem.Buf.Offset >= queueItem.LastIDBitIndex+1
}
