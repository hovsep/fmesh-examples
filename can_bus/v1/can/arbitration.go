package can

type ArbitrationState byte

const (
	arbitrationStateIn   ArbitrationState = iota // Controller is still competing with others
	arbitrationStateLost                         // Dominant bit took over, this controller needs to stop transmitting
	arbitrationStateWon                          // This controller won the arbitration, it is now the only writer, so no need to perform arbitration check (but still need to check for errors)
)
