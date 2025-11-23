package codec

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func bitSeq(bools ...bool) Bits {
	var bits Bits
	for _, b := range bools {
		bits = append(bits, Bit(b))
	}
	return bits
}

func FuzzStuffingSymmetry(f *testing.F) {
	f.Add([]byte{0b10101010})
	f.Add([]byte{0b11111111})
	f.Add([]byte{0b00000000})
	f.Add([]byte{0b11001100})

	f.Fuzz(func(t *testing.T, data []byte) {
		var bits Bits
		for _, b := range data {
			for i := 7; i >= 0; i-- {
				bits = append(bits, (b>>i)&1 == 1)
			}
		}

		stuffed := bits.WithStuffing(5)
		unstuffed := stuffed.WithoutStuffing(5)

		if !bits.Equals(unstuffed) {
			t.Errorf("Mismatch:\noriginal = %s\nunstuffed = %s", bits.String(), unstuffed.String())
		}
	})
}

func TestWithStuffingAndWithoutStuffing_Symmetry(t *testing.T) {
	tests := []struct {
		name  string
		input Bits
		count int
	}{
		{"No stuffing needed", bitSeq(true, false, true, false), 5},
		{"Stuffing 5 dominant", bitSeq(false, false, false, false, false), 5},
		{"Stuffing 3 recessive", bitSeq(true, true, true, true), 3},
		{"Mixed pattern", bitSeq(true, true, true, false, false, false, false, false, true, true), 5},
		{"Empty input", bitSeq(), 5},
		{"Single bit", bitSeq(true), 5},
		{"Alternating bits", bitSeq(true, false, true, false, true, false), 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stuffed := tt.input.WithStuffing(tt.count)
			unstuffed := stuffed.WithoutStuffing(tt.count)

			assert.True(t, tt.input.Equals(unstuffed), "Original and unstuffed should match")
		})
	}
}

func TestStuffingActuallyAddsBit(t *testing.T) {
	bits := bitSeq(false, false, false, false, false, false) // 6 dominant
	stuffed := bits.WithStuffing(5)
	assert.Greater(t, stuffed.Len(), bits.Len(), "Stuffing should add bits")
}

func TestWithoutStuffingWithNoStuffBits(t *testing.T) {
	bits := bitSeq(true, false, true, false)
	unstuffed := bits.WithoutStuffing(5)
	assert.True(t, bits.Equals(unstuffed), "WithoutStuffing should not modify bits without stuffing")
}

func TestWithEOF(t *testing.T) {
	bits := bitSeq(true, false, false)
	withEOF := bits.WithEOF()

	assert.True(t, withEOF.Len() > bits.Len(), "EOF should extend the bit sequence")
	assert.True(t, withEOF[len(withEOF)-1] == ProtocolRecessiveBit, "EOF bits should be recessive")
}

func TestWithIFS(t *testing.T) {
	bits := bitSeq(true, true)
	withIFS := bits.WithIFS()

	assert.True(t, withIFS.Len() > bits.Len(), "IFS should extend the bit sequence")
	assert.True(t, withIFS[len(withIFS)-1] == ProtocolRecessiveBit, "IFS bits should be recessive")
}

func TestEquals(t *testing.T) {
	a := bitSeq(true, false, true, false)
	b := bitSeq(true, false, true, false)
	c := bitSeq(true, false, true, true)

	assert.True(t, a.Equals(b), "Equal sequences")
	assert.False(t, a.Equals(c), "Different sequences")
}

func TestToInt(t *testing.T) {
	bits := bitSeq(false, true, false, true) // 0101
	assert.Equal(t, 5, bits.ToInt())
}

func TestAllBitsAre(t *testing.T) {
	assert.True(t, bitSeq(true, true, true).AllBitsAre(true))
	assert.False(t, bitSeq(true, false, true).AllBitsAre(true))
}
