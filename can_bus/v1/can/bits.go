package can

import "strings"

// Bit represent a single bit (true = 1, false = 0)
type Bit bool

// Bits ...
type Bits []Bit

// BitBuffer represents a buffer of bits with a position indicating how many bits have been read or written.
type BitBuffer struct {
	Bits Bits // underlying bit slice
	Pos  int  // current bit offset (read or write position)
}

func (bit Bit) String() string {
	if bit {
		return "1"
	}
	return "0"
}

func (bits Bits) String() string {
	var sb strings.Builder
	for _, bit := range bits {
		sb.WriteString(bit.String())
	}
	return sb.String()
}
