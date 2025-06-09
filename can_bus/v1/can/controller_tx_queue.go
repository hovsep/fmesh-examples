package can

type TxQueueItem struct {
	Buf            *BitBuffer // Binary encoded frame, wih SOF, EOF, IFS and 1 extra bit
	LastIDBitIndex int        // Marks the end of stuffed ID field, so controller can check for arbitration winning
}

// TxQueue represents the queue of frames  to be transmitted
type TxQueue []*TxQueueItem

func (queueItem *TxQueueItem) IDIsTransmitted() bool {
	return queueItem.Buf.Offset >= queueItem.LastIDBitIndex+1
}
