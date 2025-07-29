package microcontroller

type AddressingMode uint8

type ServiceID uint8

type ParameterID uint8

type ParamsState map[ParameterID]any

const (
	FunctionalAddressing AddressingMode = iota // Broadcast to all nodes
	PhysicalAddressing                         // Node to node communication

	FunctionalRequestID   = 0x7DF
	ResponseAddressOffset = 0x08

	ServiceShowCurrentData           ServiceID = 0x01
	ServiceReadStoredDiagnosticCodes ServiceID = 0x03
	ServiceVehicleInformation        ServiceID = 0x09
	ResponseShowCurrentData          ServiceID = 0x41 // 0x40 + 0x01

)

func (mode AddressingMode) String() string {
	switch mode {
	case FunctionalAddressing:
		return "functional"
	case PhysicalAddressing:
		return "physical"
	default:
		return "unknown"
	}
}
