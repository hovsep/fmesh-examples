package can

import "strings"

type Bit bool
type Bits []Bit

func (bits Bits) String() string {
	var sb strings.Builder
	for _, bit := range bits {
		sb.WriteString(bit.String())
	}
	return sb.String()
}

func (bit Bit) String() string {
	if bit {
		return "1"
	}
	return "0"
}
