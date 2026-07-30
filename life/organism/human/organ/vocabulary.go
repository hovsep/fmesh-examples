package organ

// State keys shared by the organs that oscillate. A heart and a diaphragm both
// run at a rate and sit somewhere in a cycle; the words are the same even though
// the beats are not.
const (
	stateRate  = "rate"
	statePhase = "phase"
)

// Side names the paired organs. Only the lungs come in two here, but a side is a
// property of the body rather than of the lung, so it reads the same wherever it
// is used.
type Side = string

const (
	Left  Side = "left"
	Right Side = "right"
)
