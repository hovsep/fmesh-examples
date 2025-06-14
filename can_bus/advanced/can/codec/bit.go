package codec

// Bit represents a single bit (true = 1, false = 0)
type Bit bool

func (bit Bit) String() string {
	if bit {
		return "1"
	}
	return "0"
}

func (bit Bit) IsDominant() bool {
	return bit == ProtocolDominantBit
}

func (bit Bit) IsRecessive() bool {
	return bit == ProtocolRecessiveBit
}
