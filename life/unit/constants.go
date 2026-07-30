package unit

// @TODO: check if my approach is even helpful (when I multiply a number by constant to get units)
// maybe it is overkill. If we justify the approach - let's check is somewhere we are missing units and use them, if no - let's drop units and just use comments
const (
	Percent = 1

	// PerMinute is a value in per minute
	PerMinute = 1

	// PerSecond is a rate expressed per second (1/s)
	PerSecond = 1.0

	// PercentPerSecond is a rate of change of a percentage level, per second (%/s)
	PercentPerSecond = 1.0

	// Milliliter is a unit of volume in milliliters
	Milliliter = 1.0

	// DNCS is Dimensionless Normalized Control Signal ∈ [0,1]
	DNCS = 1.0

	// Proportion is a percentage converted to the interval [0,1]
	Proportion = 1.0

	// CmH2O is pressure in centimeters of water column (respiratory standard unit)
	CmH2O = 1.0

	// MlPerCmH2O is a lung compliance (mechanical property defined as a pressure–volume relationship) unit
	MlPerCmH2O = 1.0

	// CmH2OPerMlPerSecond is a unit of air resistance
	CmH2OPerMlPerSecond = 1.0

	// Celsius is a temperature unit
	Celsius = 1.0

	// MmHg is pressure in millimetres of mercury, the unit blood gases and blood
	// pressure are read in clinically (PaO₂, PaCO₂, mean arterial pressure).
	MmHg = 1.0

	// MmHgPerSecond is a rate of change of a partial pressure, per second.
	MmHgPerSecond = 1.0

	// Liter is a unit of volume in litres, used for blood volume.
	Liter = 1.0
)
