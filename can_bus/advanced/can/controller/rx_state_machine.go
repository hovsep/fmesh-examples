package controller

// RxState is a sub-DFSM determining which portion of data is being received
type RxState byte

const (
	RxStateNone RxState = iota
	RxStateSOF
	RxStateID
	RxStateDLC
	RxStateData
	RxStateEOF
	RxStateIFS
)
