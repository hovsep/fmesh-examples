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

// TODO: remove this
func (sid ServiceID) ToString() string {
	switch sid {
	case ServiceShowCurrentData, ResponseShowCurrentData:
		return "show current data"
	case ServiceReadStoredDiagnosticCodes, ResponseReadStoredDiagnosticCodes:
		return "read stored diagnostic codes"
	case ServiceVehicleInformation, ResponseVehicleInformation:
		return "vehicle information"
	default:
		return "unknown"

	}
}

// TODO: remove this
func (pid ParameterID) ToString() string {
	switch pid {
	case NoPID:
		return "no PID"
	case 0x0C:
		return "RPM"
	case 0x0D:
		return "Vehicle Speed"
	case 0x02:
		return "VIN"
	case 0x04:
		return "Calibration ID"
	case 0x05:
		return "Coolant Temperature"

	case 0x0F:
		return "Fluid temperature"
	case 0xA1:
		return "Gear position"
	default:
		return "unknown"
	}
}
