package controller

// TxState is a sub-DFSM determining which portion of data is being transmitted
type TxState byte

const (
	TxStateNone TxState = iota
	TxStateSOF
	TxStateID
	TxStateDLC
	TxStateData
	TxStateEOF
	TxStateIFS
)
