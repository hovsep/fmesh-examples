package can

import (
	"strings"
)

// Bit represents a single bit (true = 1, false = 0)
type Bit bool

// Bits ...
type Bits []Bit

// BitBuffer represents a buffer of bits with a position indicating how many bits have been read or written.
type BitBuffer struct {
	Bits   Bits // underlying bit slice
	Offset int  // how many bits are already processed?
}

func (bit Bit) String() string {
	if bit {
		return "1"
	}
	return "0"
}

func NewBits(len int) Bits {
	return make(Bits, len)
}

func (bits Bits) String() string {
	var sb strings.Builder
	for _, bit := range bits {
		sb.WriteString(bit.String())
	}
	return sb.String()
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
