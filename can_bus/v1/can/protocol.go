package can

const (
	ProtocolDominantBit  = Bit(false)
	ProtocolRecessiveBit = Bit(true)

	ProtocolBitStuffingStep = 5

	ProtocolMaxDataLen = 8     // In bytes
	ProtocolMaxFrameID = 0x7FF // 11-bit max

	ProtocolIDBitsCount  = 11
	ProtocolDLCBitsCount = 4
	ProtocolEOFBitsCount = 7
	ProtocolIFSBitsCount = 3 // Inter-Frame Space is the gap between two consecutive CAN frames
)
