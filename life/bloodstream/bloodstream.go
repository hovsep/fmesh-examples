// Package bloodstream is the vocabulary of the blood: what it carries, how much
// of it, and how organs put things into it and take things out.
//
// It is deliberately separate from the component that circulates the blood
// (da:blood_system). The component integrates; this package says what the
// numbers mean. Keeping them apart is what lets a plugin attached to any organ
// speak the same language as the bloodstream without the two importing each
// other -- which is how every tissue in the body came to be perfused by one
// small plugin rather than by hand-written code in each organ.
package bloodstream

import (
	"math"

	"github.com/hovsep/fmesh/component"

	"github.com/hovsep/fmesh-examples/life/helper"
	. "github.com/hovsep/fmesh-examples/life/unit"
	"github.com/hovsep/fmesh/signal"
)

// Blood carries arterial blood gases in the units they are read in clinically:
// oxygen and carbon dioxide as partial pressures in mmHg, saturation as a
// percentage, and pH. Breathing pulls the gases toward what the alveoli offer;
// every organ draws oxygen out and returns carbon dioxide, so the values
// oscillate with the breath and drift away when breathing stops.
//
// Reference values for a resting adult breathing room air at sea level.
const (
	NormalPaO2  = 95.0 * MmHg
	NormalPaCO2 = 40.0 * MmHg
	NormalPH    = 7.40

	// AlveolarPO2 is the oxygen tension inside the alveoli on room air at sea
	// level: the pressure driving oxygen into the blood. It is a constant here
	// and becomes a function of barometric pressure and inspired fraction once
	// the airway carries them -- which is what makes altitude work.
	AlveolarPO2 = 104.0 * MmHg

	// AaGradient is the alveolar-arterial difference. Blood leaves the lungs a
	// little short of alveolar tension because some of it passes unventilated
	// alveoli, which is why a healthy PaO₂ is about 95 and not 104.
	AaGradient = 9.0 * MmHg

	// VentilatedPaCO2 is what ventilation pulls carbon dioxide down towards;
	// metabolism pushes it back up and the two settle near 40.
	VentilatedPaCO2 = 38.0 * MmHg

	// Survivable bounds. The oxygen ceiling is what hyperbaric therapy reaches.
	MinPaO2  = 5.0 * MmHg
	MaxPaO2  = 600.0 * MmHg
	MinPaCO2 = 10.0 * MmHg
	MaxPaCO2 = 150.0 * MmHg

	// CO2ExcretionFraction is kept for the lung component, which references it.
	CO2ExcretionFraction = 0.15 * Proportion

	// Saturation bounds. The floor is above zero because the arithmetic that
	// recovers tension from saturation cannot divide by a full or empty
	// hemoglobin.
	MinSaturation = 1.0
	MaxSaturation = 100.0

	// DLPerLiter converts blood volume to the decilitres hemoglobin is measured
	// against.
	DLPerLiter = 10.0

	// ReferenceVentilation is the air a quietly breathing adult moves, as the
	// average magnitude of its airflow in mL/s. Ventilation is measured against
	// it, so a body breathing normally is by definition ventilating normally.
	ReferenceVentilation = 222.0

	// CO2EliminationPerMmHg is how much carbon dioxide a normally ventilating
	// body blows off per second, per mmHg of arterial tension, in mL.
	//
	// Elimination is proportional to how much air is moved AND to how much CO₂
	// there is to carry away, which is what makes the textbook relationship fall
	// out: at equilibrium PaCO₂ is production divided by ventilation. Halve the
	// breathing and the CO₂ doubles.
	//
	// This replaced a formulation that pulled CO₂ toward a fixed target, which
	// pinned it near normal however hard or feebly the body breathed -- so
	// hypoventilation was harmless and the chemoreflex had nothing to do.
	CO2EliminationPerMmHg = 0.0717

	// MaxVentilationFactor caps how much good extra ventilation can do. Beyond a
	// point the blood leaving the lungs is already saturated and moving more air
	// past it achieves nothing, which is why hyperventilation can blow off carbon
	// dioxide without raising oxygen much at all.
	MaxVentilationFactor = 4.0

	// CO2StoragePerMmHg is how much carbon dioxide the body takes up for each
	// mmHg its arterial tension rises, in mL.
	//
	// It is enormous next to the oxygen store -- most of it is buffered as
	// bicarbonate in blood and tissue rather than carried as gas -- and that
	// difference is why a held breath costs tens of points of saturation but only
	// a few mmHg of CO₂. Calibrated so that a resting body, producing about
	// 200 mL/min, raises PaCO₂ by ~5 mmHg per apnoeic minute.
	CO2StoragePerMmHg = 37.0
)

// How much oxygen the blood actually carries.
//
// Partial pressure is only the reading on the gauge; what a tissue consumes is
// oxygen content, and almost all of that is bound to hemoglobin rather than
// dissolved. The distinction is the whole point of modelling content: bleeding,
// anemia and carbon monoxide all wreck the supply while leaving PaO₂ and SpO₂
// looking perfectly normal.
const (
	// NormalHemoglobin is a healthy adult concentration, in g/dL.
	NormalHemoglobin = 15.0

	// NormalBloodVolume is total blood volume for a 70 kg adult, in litres.
	NormalBloodVolume = 5.0 * Liter

	// HufnerConstant is how much oxygen a gram of fully saturated hemoglobin
	// carries, in mL. Hüfner's number.
	HufnerConstant = 1.34

	// DissolvedPerMmHg is the oxygen dissolved in plasma per mmHg of tension,
	// in mL/dL. It is tiny next to what hemoglobin carries -- 0.3 against 19.5 --
	// which is why breathing pure oxygen helps an anemic patient so little.
	DissolvedPerMmHg = 0.003

	// NormalSaO2 is arterial saturation in a healthy adult, as a percentage.
	NormalSaO2 = 97.0
)

// NormalOxygenContent is what a healthy arterial sample carries, ~20 mL/dL.
var NormalOxygenContent = OxygenContent(NormalHemoglobin, NormalSaO2, NormalPaO2)

// OxygenContent returns the oxygen carried per dL of blood: what hemoglobin
// holds plus the little that is dissolved.
func OxygenContent(hemoglobin, saturationPct, paO2 float64) float64 {
	return HufnerConstant*hemoglobin*saturationPct/100.0 + DissolvedPerMmHg*paO2
}

// OxygenDelivery returns how much oxygen reaches the tissues each minute, in mL:
// the blood's content multiplied by how fast it is being pumped.
//
//	DO₂ = cardiac output (L/min) × content (mL/dL) × 10
//
// A healthy adult delivers about 1000 mL/min and consumes about 250, an
// extraction of a quarter. Shock is what happens when delivery falls far enough
// that extraction cannot make up the difference.
func OxygenDelivery(cardiacOutputLPerMin, contentMlPerDl float64) float64 {
	return cardiacOutputLPerMin * contentMlPerDl * 10.0
}

// The oxyhemoglobin dissociation curve.
const (
	// P50 is the oxygen tension at which hemoglobin is half saturated.
	P50 = 26.6 * MmHg

	// hillCoefficient is the curve's cooperativity: binding one oxygen molecule
	// makes hemoglobin readier to bind the next, which is what gives the curve
	// its S shape.
	hillCoefficient = 2.7
)

// TensionAt is the dissociation curve read backwards: the oxygen tension at
// which hemoglobin would hold a given saturation.
//
// The blood tracks how much oxygen it is carrying, because that is what organs
// take out of it; the partial pressure a blood gas reports is recovered from
// that. Inverting the Hill equation is what lets the model keep both, and keep
// them consistent.
func TensionAt(saturationPct float64) float64 {
	s := helper.Clamp(saturationPct, 0.01, 99.99) / 100.0
	return P50 * math.Pow(s/(1-s), 1.0/hillCoefficient)
}

// SaturationAt returns hemoglobin saturation (%) at an arterial oxygen tension,
// from the Hill equation.
//
// This is the curve every physiology course draws. It is flat above about
// 80 mmHg, so a large fall in PaO₂ costs almost no saturation, and steep below
// about 60 mmHg, where saturation collapses. PaO₂ 60 → SpO₂ ≈ 90% is the corner
// it is known by, and the point at which supplemental oxygen is given.
func SaturationAt(paO2 float64) float64 {
	if paO2 <= 0 {
		return 0
	}
	bound := math.Pow(paO2, hillCoefficient)
	return 100.0 * bound / (bound + math.Pow(P50, hillCoefficient))
}

// PHAt returns blood pH at an arterial carbon dioxide tension.
//
// Carbon dioxide dissolves into carbonic acid, so ventilation sets pH:
// hypoventilation is an acidosis, hyperventilation an alkalosis. Acutely the
// shift is about 0.008 per mmHg -- the textbook "0.08 per 10 mmHg". Metabolic
// acid-base disturbance and renal compensation are not modelled.
func PHAt(paCO2 float64) float64 {
	return NormalPH - 0.008*(paCO2-NormalPaCO2)
}

// DefaultGlucoseLevel is the fasting blood sugar the blood assumes before the
// reservoir (physiology:physiological_state) reports otherwise. Blood is only the
// transport here; the glucose reservoir lives in physiology.
const DefaultGlucoseLevel = 90.0 // mg/dL

// The blood is the organism's shared bus: any organ can secrete a substance into it by
// emitting a signal on its "blood" output. Each signal is tagged with a substance label
// so the blood component knows how to handle it. Organs may send one, several, or none
// per tick (e.g. only a hormone), and unknown substances are simply ignored for now.
const SubstanceLabel = "substance"

const (
	// SubstanceO2Draw: payload is an oxygen demand in mL/s. What an organ takes
	// out of the blood, which is why it is a volume and not a pressure.
	SubstanceO2Draw = "o2_draw"
	// SubstanceCO2Load: payload is carbon dioxide returned, in mL/s.
	SubstanceCO2Load = "co2_load"
	// SubstanceHormone: a gland's secretion rate for the hormone named by the
	// HormoneLabel on the same signal, in units of saturation per second.
	SubstanceHormone = "hormone"
	// Future: toxins, drugs, nutrients, ... add a case in updateBloodLevels.
)

// SupplyPort is the input every organ receives blood on, and ReturnPort the
// output it puts things back into the blood on. They are named here because the
// plugins that use them have to agree, and only one of them may create each.
const (
	SupplyPort = "blood"
	ReturnPort = "blood"
)

// HormoneLabel names which hormone a secretion carries.
const HormoneLabel = "hormone"

// The hormones this body knows about, as levels between 0 (none circulating)
// and 1 (as much as this body ever makes).
//
// Two of them answer the same stressor on two timescales, which is the point of
// having both: adrenaline arrives in seconds and is gone in minutes, cortisol
// takes minutes to arrive and hours to leave. A fright and a siege are not the
// same problem and are not solved by the same chemistry.
//
// The pancreatic pair are arranged the other way round, and the contrast is
// worth seeing. Insulin and glucagon share a clock -- both are gone within
// minutes -- and differ in direction instead: one puts fuel away, the other
// fetches it back. An axis can be built out of two speeds or out of two signs,
// and the body uses both arrangements.
const (
	HormoneAdrenaline = "adrenaline"
	HormoneCortisol   = "cortisol"
	HormoneInsulin    = "insulin"
	HormoneGlucagon   = "glucagon"
)

// Hormones is every hormone the bloodstream carries, so the blood can clear them
// and telemetry can report them without either being told about each new one.
var Hormones = []string{HormoneAdrenaline, HormoneCortisol, HormoneInsulin, HormoneGlucagon}

// HormoneSecretion builds a signal for a gland to emit on its blood output.
// Rate is in level per second: how fast the gland is raising the circulating
// level, against the clearance that is always pulling it back down.
func HormoneSecretion(hormone string, rate float64) *signal.Signal {
	return signal.New(rate).
		WithLabel(SubstanceLabel, SubstanceHormone).
		WithLabel(HormoneLabel, hormone)
}

// Hormone half-lives: how long the blood takes to clear away half of what is
// circulating, in seconds. These are what make the two axes feel different.
const (
	AdrenalineHalfLifeSec = 120.0  // minutes: a fright passes
	CortisolHalfLifeSec   = 3600.0 // an hour: a siege does not

	// Both pancreatic hormones are cleared in about five minutes, which is why
	// blood sugar is corrected continuously rather than in one decision: the
	// pancreas must keep saying it, and stops being obeyed shortly after it
	// stops.
	InsulinHalfLifeSec  = 300.0
	GlucagonHalfLifeSec = 300.0
)

// Secretion builds a substance signal for an organ to emit on its "blood" output.
func Secretion(substance string, rate float64) *signal.Signal {
	return signal.New(rate).WithLabel(SubstanceLabel, substance)
}

// HormoneHalfLife returns how long the blood takes to clear half of a hormone.
func HormoneHalfLife(hormone string) float64 {
	switch hormone {
	case HormoneAdrenaline:
		return AdrenalineHalfLifeSec
	case HormoneCortisol:
		return CortisolHalfLifeSec
	case HormoneInsulin:
		return InsulinHalfLifeSec
	case HormoneGlucagon:
		return GlucagonHalfLifeSec
	default:
		return AdrenalineHalfLifeSec
	}
}

// EnsureSupplyPort gives a component the input it receives blood on, if it does
// not already have one.
//
// Two plugins need that port -- perfusion, to read what is being delivered, and
// receptor, to read what is being signalled -- and an organ may carry either or
// both. fmesh keeps a component's plugins in a map, so which of them initialises
// first is undefined: whichever arrives first must create the port and the other
// must find it. Adding it unconditionally works right up until the day the map
// hands them over in the other order, which is exactly how this was found.
func EnsureSupplyPort(c *component.Component) error {
	if c.InputByName(SupplyPort) != nil {
		return nil
	}
	return c.AddInputs(SupplyPort)
}

// EnsureReturnPort gives a component the output it puts things back into the
// blood on, if it does not already have one.
func EnsureReturnPort(c *component.Component) error {
	if c.OutputByName(ReturnPort) != nil {
		return nil
	}
	return c.AddOutputs(ReturnPort)
}
