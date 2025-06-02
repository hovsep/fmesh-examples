package main

import "strings"

type Bits []bool

func (b Bits) String() string {
	var sb strings.Builder
	for _, bit := range b {
		if bit {
			sb.WriteByte('1')
		} else {
			sb.WriteByte('0')
		}
	}
	return sb.String()
}
