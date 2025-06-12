package codec

// BitBuffer represents a buffer of bits with an offset indicating how many bits have been read or written.
type BitBuffer struct {
	Bits   Bits // underlying bit slice
	Offset int  // how many bits are already processed?
}

func NewBitBuffer(bits Bits) *BitBuffer {
	return &BitBuffer{
		Bits:   bits,
		Offset: 0,
	}
}

func NewEmptyBitBuffer() *BitBuffer {
	return &BitBuffer{
		Bits:   NewBits(0),
		Offset: 0,
	}
}

func (bb *BitBuffer) Len() int {
	return bb.Bits.Len()
}

func (bb *BitBuffer) NextBit() Bit {
	return bb.Bits[bb.Offset]
}

func (bb *BitBuffer) PreviousBit() Bit {
	return bb.Bits[bb.Offset-1]
}

func (bb *BitBuffer) IncreaseOffset() {
	bb.Offset++
}

func (bb *BitBuffer) ResetOffset() {
	bb.Offset = 0
}

func (bb *BitBuffer) Available() int {
	return len(bb.Bits) - bb.Offset
}

func (bb *BitBuffer) AppendBit(bit Bit) {
	bb.Bits = append(bb.Bits, bit)
}
