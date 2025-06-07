package can

import (
	"strings"
)

// Bit represents a single bit (true = 1, false = 0)
type Bit bool

// Bits ...
type Bits []Bit

// BitBuffer represents a buffer of bits with an offset indicating how many bits have been read or written.
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

func (bit Bit) IsDominant() bool {
	return bit == ProtocolDominantBit
}

func (bit Bit) IsRecessive() bool {
	return bit == ProtocolRecessiveBit
}

func NewBits(len int) Bits {
	return make(Bits, len)
}

func (bits Bits) Len() int {
	return len(bits)
}

func (bits Bits) String() string {
	var sb strings.Builder
	for _, bit := range bits {
		sb.WriteString(bit.String())
	}
	return sb.String()
}

func (bits Bits) WithStuffing(afterEach int) Bits {
	if afterEach <= 0 {
		panic("afterEach must be > 0")
	}

	var stuffed Bits
	var count int
	var last Bit
	first := true

	for _, b := range bits {
		stuffed = append(stuffed, b)

		if first {
			last = b
			count = 1
			first = false
			continue
		}

		if b == last {
			count++
			if count == afterEach {
				// Insert the opposite bit
				stuffed = append(stuffed, !b)
				count = 0    // Reset count after stuffing
				first = true // Restart tracking from next bit
			}
		} else {
			last = b
			count = 1
		}
	}

	return stuffed
}

func (bits Bits) WithoutStuffing(afterEach int) Bits {
	if afterEach <= 0 {
		panic("afterEach must be > 0")
	}

	var unstuffed Bits
	var count int
	var last Bit
	first := true

	i := 0
	for i < len(bits) {
		b := bits[i]
		unstuffed = append(unstuffed, b)

		if first {
			last = b
			count = 1
			first = false
			i++
			continue
		}

		if b == last {
			count++
			if count == afterEach {
				// Skip the next bit (stuffed bit)
				i += 2 // skip current + stuffed
				if i <= len(bits) {
					first = true // restart tracking after stuffed bit
				}
				continue
			}
		} else {
			last = b
			count = 1
		}

		i++
	}

	return unstuffed
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
