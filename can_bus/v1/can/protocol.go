package can

const (
	ProtocolDominantBit  = Bit(false)
	ProtocolRecessiveBit = Bit(true)

	ProtocolBitStuffingStep = 5

	ProtocolMaxDataLen = 8     // In bytes
	ProtocolMaxFrameID = 0x7FF // 11-bit max

	ProtocolIDBitsCount  = 11 // ID field size
	ProtocolDLCBitsCount = 4  // Data Length Code size
	ProtocolEOFBitsCount = 7  // End Of Frame marker (all recessive)
	ProtocolIFSBitsCount = 3  // Inter-Frame Space is the gap between two consecutive CAN frames (all recessive)
)
