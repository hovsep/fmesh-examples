package microcontroller

// AddressingMode defines how a can message is addressed (to single ECU or to all ECUs connected to bus)
type AddressingMode uint32

type ServiceID uint8

type ParameterID uint8

// Logic is the app of the MCU, the main logic
type Logic func(mode AddressingMode, sid ServiceID, pid ParameterID, request *ISOTPMessage) *ISOTPMessage

const (
	Functional AddressingMode = 0x7DF // Broadcast to all nodes
	Physical   AddressingMode = 0x7E0 // Direct request-response

	ServiceShowCurrentData           ServiceID = 0x01
	ServiceReadStoredDiagnosticCodes ServiceID = 0x03
	ServiceVehicleInformation        ServiceID = 0x09
	ResponseShowCurrentData          ServiceID = 0x41 // 0x40 + 0x01

	Pid
)
