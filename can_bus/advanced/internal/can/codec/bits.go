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

func (bits Bits) WithStuffing(count int) Bits {
	if count <= 0 {
		panic("count must be > 0")
	}

	if len(bits) == 0 {
		return bits
	}

	var result Bits
	consecutive := 1
	lastBit := bits[0]
	result = append(result, lastBit)

	for i := 1; i < len(bits); i++ {
		currentBit := bits[i]

		if currentBit == lastBit {
			consecutive++
			result = append(result, currentBit)

			if consecutive == count {
				// Insert stuff bit (opposite of current)
				stuffBit := !currentBit
				result = append(result, stuffBit)
				// The stuff bit breaks the sequence - use it as the new lastBit
				lastBit = stuffBit
				consecutive = 1
			}
		} else {
			// Different bit, reset counter
			result = append(result, currentBit)
			lastBit = currentBit
			consecutive = 1
		}
	}

	return result
}

func (bits Bits) WithoutStuffing(count int) Bits {

	if count <= 0 {
		panic("count must be > 0")
	}

	if len(bits) <= count {
		return bits // Too short to have any stuffing
	}

	// First pass: identify stuff bit positions
	stuffPositions := make(map[int]bool)

	i := 0
	for i < len(bits) {
		if i+count >= len(bits) {
			break // Not enough bits left for a stuffing sequence
		}

		// Check if we have 'count' consecutive identical bits starting at position i
		consecutive := 1
		for j := i + 1; j < len(bits) && bits[j] == bits[i] && consecutive < count; j++ {
			consecutive++
		}

		// If we found exactly 'count' consecutive bits and there's a next bit
		if consecutive == count && i+count < len(bits) {
			// Check if the next bit is a stuff bit (opposite polarity)
			if bits[i+count] == !bits[i] {
				stuffPositions[i+count] = true
				// Continue from next position to check for overlapping sequences
				// Don't skip past the entire sequence since stuff bit might start new sequence
			}
		}

		i++
	}

	// Second pass: build result by skipping stuff bit positions
	var result Bits
	for i := 0; i < len(bits); i++ {
		if !stuffPositions[i] {
			result = append(result, bits[i])
		}
	}

	return result
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

	for i := 0; i < bits.Len(); i++ {
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

// WithoutLastBit returns a new Bits slice with the last bit removed.
// If the slice is empty, returns an empty slice.
func (bits Bits) WithoutLastBit() Bits {
	if len(bits) == 0 {
		return NewBits(0)
	}

	result := make(Bits, len(bits)-1)
	copy(result, bits[:len(bits)-1])
	return result
}

// WithLastBitSwitched returns a new Bits slice with the last bit flipped.
// If the slice is empty, returns an empty slice.
func (bits Bits) WithLastBitSwitched() Bits {
	if len(bits) == 0 {
		return NewBits(0)
	}

	result := make(Bits, len(bits))
	copy(result, bits)
	result[len(result)-1] = !result[len(result)-1]
	return result
}

// WithLastBitReplaced returns a new Bits slice with the last bit replaced by the given bit.
// If the slice is empty, returns a slice with just the new bit.
func (bits Bits) WithLastBitReplaced(newBit Bit) Bits {
	if len(bits) == 0 {
		return Bits{newBit}
	}

	result := make(Bits, len(bits))
	copy(result, bits)
	result[len(result)-1] = newBit
	return result
}
