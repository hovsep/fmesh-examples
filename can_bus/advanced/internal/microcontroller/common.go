package microcontroller

type AddressingMode uint8

type ServiceID uint8

type ParameterID uint8

type ParamsState map[ParameterID]any

// DTC represents On-Board Diagnostic Trouble Code
type DTC [2]byte

const (
	FunctionalAddressing AddressingMode = iota // Broadcast to all nodes
	PhysicalAddressing                         // Node to node communication

	FunctionalRequestID   = 0x7DF
	ResponseAddressOffset = 0x08

	ServiceShowCurrentData           ServiceID = 0x01
	ServiceReadStoredDiagnosticCodes ServiceID = 0x03
	ServiceVehicleInformation        ServiceID = 0x09

	ResponseServiceIDOffset           ServiceID = 0x40
	ResponseShowCurrentData                     = ServiceShowCurrentData + ResponseServiceIDOffset
	ResponseReadStoredDiagnosticCodes           = ServiceReadStoredDiagnosticCodes + ResponseServiceIDOffset
	ResponseVehicleInformation                  = ServiceVehicleInformation + ResponseServiceIDOffset

	// NoPID is a dummy ParameterID used for services that don't require a real PID.
	NoPID ParameterID = 0x00
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
