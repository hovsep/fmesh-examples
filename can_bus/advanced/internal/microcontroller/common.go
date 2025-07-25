package microcontroller

type AddressingMode uint8

type ServiceID uint8

type ParameterID uint8

const (
	Functional AddressingMode = iota // Broadcast to all nodes
	Physical                         // Node to node communication

	FunctionalRequestID = 0x7DF

	ServiceShowCurrentData           ServiceID = 0x01
	ServiceReadStoredDiagnosticCodes ServiceID = 0x03
	ServiceVehicleInformation        ServiceID = 0x09
	ResponseShowCurrentData          ServiceID = 0x41 // 0x40 + 0x01

	Pid
)

func (mode AddressingMode) String() string {
	switch mode {
	case Functional:
		return "Functional"
	case Physical:
		return "Physical"
	default:
		return "Unknown"
	}
}
