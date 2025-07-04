package codec

import (
	"strings"
)

// Bits ...
type Bits []Bit

func NewBits(len int) Bits {
	return make(Bits, len)
}

func RepeatBit(bit Bit, n int) Bits {
	bits := NewBits(0)
	for i := 0; i < n; i++ {
		bits = append(bits, bit)
	}
	return bits
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
				stuffedBit := !b
				stuffed = append(stuffed, stuffedBit)
				// Treat the stuffed bit as a breaker, not part of the run
				last = stuffedBit
				count = 1
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

func (bits Bits) WithEOF() Bits {
	return append(bits, RepeatBit(ProtocolRecessiveBit, ProtocolEOFSize)...)
}

func (bits Bits) WithIFS() Bits {
	return append(bits, RepeatBit(ProtocolRecessiveBit, ProtocolIFSSize)...)
}

func (bits Bits) WithBits(extraBits ...Bit) Bits {
	for _, b := range extraBits {
		bits = append(bits, b)
	}
	return bits
}

func (bits Bits) ToInt() int {
	var result = 0
	for _, bit := range bits {
		result <<= 1
		if bit {
			result |= 1
		}
	}
	return result
}

func (bits Bits) Equals(b Bits) bool {
	if bits.Len() != b.Len() {
		return false
	}

	for i := 0; i < bits.Len()-1; i++ {
		if bits[i] != b[i] {
			return false
		}
	}
	return true
}

func (bits Bits) AllBitsAre(b Bit) bool {
	for _, bb := range bits {
		if bb != b {
			return false
		}
	}

	return true
}
