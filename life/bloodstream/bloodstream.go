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

	"github.com/hovsep/fmesh-examples/simulation/mathx"
	"github.com/hovsep/fmesh/signal"
)

// Blood carries arterial blood gases in the units they are read in clinically:
// oxygen and carbon dioxide as partial pressures in mmHg, saturation as a
// percentage, and pH. Breathing pulls the gases toward what the alveoli offer;
// every organ draws oxygen out and returns carbon dioxide, so the values
// oscillate with the breath and drift away when breathing stops.
//
// Reference values for a resting adult breathing room air at sea level. Partial
// pressures are in mmHg throughout, which is the unit blood gases are read in
// clinically.
const (
	// These are reference values, not settings: what a healthy resting adult
	// reads at sea level. Nothing here should become dynamic, because the whole
	// point of them is to be the fixed thing a changing body is compared against
	// -- the alveolar gas equation already takes pressure and mixture from the
	// air, so altitude and a chamber move the body's numbers without moving what
	// normal means.
	NormalPaO2  = 95.0
	NormalPaCO2 = 40.0
	NormalPH    = 7.40

	// WaterVaporPressure is what the airway's own moisture contributes, in mmHg.
	//
	// It is a constant 47 whatever the weather, because inspired air arrives at
	// the alveoli fully saturated at body temperature -- and it is subtracted
	// before anything else, which is why it matters more the higher you go. At
	// sea level it costs 6% of the available pressure; on the summit of Everest,
	// where the barometer reads 253, it costs nearly a fifth of it before a
	// single molecule of oxygen is accounted for.
	WaterVaporPressure = 47.0

	// RespiratoryQuotient is carbon dioxide produced per oxygen consumed. About
	// 0.8 on a mixed diet: eight molecules out for every ten in.
	RespiratoryQuotient = 0.8

	// AaGradient is the alveolar-arterial difference: blood leaves the lungs a
	// little short of alveolar tension, because some of it passes alveoli that
	// are perfused but not ventilated and arrives having exchanged nothing.
	//
	// Five mmHg is a healthy young adult. It widens with age, and it widens
	// sharply with anything that spoils the match between air and blood --
	// which is what makes it the number a clinician reaches for to tell a lung
	// problem from a breathing problem. A body that is simply not breathing
	// enough has a normal gradient and a high PaCO₂; a body with a damaged lung
	// has a wide one. This model has no such damage term yet, so the gradient is
	// a constant, and the honest way to read it is as a healthy pair of lungs.
	//
	// Taken with the alveolar gas equation it puts a resting arterial PaO₂ at
	// 99.7 − 5 ≈ 95, which is NormalPaO2. The two are meant to agree, and if one
	// is changed the other has to be.
	AaGradient = 5.0

	// VentilatedPaCO2 is what ventilation pulls carbon dioxide down towards;
	// metabolism pushes it back up and the two settle near 40.
	VentilatedPaCO2 = 38.0

	// Survivable bounds.
	//
	// The oxygen ceiling is what hyperbaric therapy actually reaches, and it is
	// far higher than most people expect. Pure oxygen at three atmospheres puts
	// alveolar PO₂ over 2000 mmHg -- enough that the oxygen merely *dissolved* in
	// plasma, normally a rounding error against what hemoglobin carries, can by
	// itself supply a resting body. That is why a hyperbaric chamber can keep
	// someone alive whose hemoglobin has been taken out of service by carbon
	// monoxide. The ceiling used to be 600, which quietly made that impossible.
	MinPaO2  = 5.0
	MaxPaO2  = 2200.0
	MinPaCO2 = 10.0
	MaxPaCO2 = 150.0

	// CO2ExcretionFraction is kept for the lung component, which references it.
	CO2ExcretionFraction = 0.15

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
	NormalBloodVolume = 5.0

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
	P50 = 26.6

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
	s := mathx.Clamp(saturationPct, 0.01, 99.99) / 100.0
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

// AlveolarPO2At is the alveolar gas equation: the oxygen tension inside the
// alveoli, which is the pressure driving oxygen into the blood.
//
//	PAO₂ = FiO₂ × (Pb − PH₂O) − PaCO₂ / R
//
// It is the single most useful equation in respiratory physiology, and it says
// that alveolar oxygen depends on three things a body does not control -- how
// much air there is, how much of it is oxygen, and how wet the airway makes it --
// and one thing it does: how hard it breathes.
//
// That last term is why the equation is worth having rather than a constant.
// Breathing harder lowers PaCO₂, and every mmHg of carbon dioxide removed buys
// back 1.25 mmHg of alveolar oxygen. At sea level that is a refinement. At 8848
// metres it is the whole margin: 21% of (253 − 47) is 43 mmHg of oxygen, and a
// body at a normal PaCO₂ of 40 would be subtracting 50 of them and arriving at
// less than nothing. Climbers survive by hyperventilating their PaCO₂ down to
// near 10, which turns an impossible sum into a survivable one -- and the
// chemoreflex in physiology:autonomic_coordination does exactly that, without
// having been told anything about mountains.
func AlveolarPO2At(barometric, inspiredO2Fraction, paCO2 float64) float64 {
	return inspiredO2Fraction*(barometric-WaterVaporPressure) - paCO2/RespiratoryQuotient
}

// RoomAirO2Fraction is the oxygen fraction of the atmosphere, as a proportion.
// It is the same on a mountain as at sea level; only the pressure changes.
const RoomAirO2Fraction = 0.21

// SeaLevelAlveolarPO2 is what the equation gives for a resting adult breathing
// room air at sea level: about 100 mmHg, which is where the textbook figure
// comes from.
var SeaLevelAlveolarPO2 = AlveolarPO2At(760.0, RoomAirO2Fraction, NormalPaCO2)

// TensionForContent is the oxygen content equation solved backwards: the
// arterial tension at which blood of a given carrying capacity would hold a
// given amount of oxygen.
//
// It exists because content is the quantity worth storing and tension is the
// quantity worth reporting. Organs consume millilitres of oxygen, so millilitres
// are what the blood must keep track of; a clinician reads a partial pressure and
// a saturation, so those are what it must publish. One of the two has to be
// derived from the other, and deriving in this direction is the one that stays
// honest when the carrier is damaged.
//
// The blood used to store saturation and derive tension, which worked until
// something needed oxygen that hemoglobin was not carrying. Saturation cannot go
// past 100, so neither could the tension derived from it, and pure oxygen at
// three atmospheres -- whose entire therapeutic point is the oxygen dissolved in
// plasma at a tension hemoglobin has nothing to do with -- came out at a PaO₂ of
// 145 instead of 2180.
//
// There is no closed form: the Hill equation is not invertible in company with
// the linear dissolved term. Content rises strictly with tension, though, so
// bisection finds it in a fixed number of steps and cannot fail to converge.
func TensionForContent(hemoglobin, content float64) float64 {
	if content <= 0 {
		return MinPaO2
	}

	low, high := MinPaO2, MaxPaO2
	if OxygenContentAt(hemoglobin, high) <= content {
		return high
	}

	// Fifty halvings of the range resolve it far below the precision anything
	// downstream reports.
	for range 50 {
		mid := (low + high) / 2
		if OxygenContentAt(hemoglobin, mid) < content {
			low = mid
		} else {
			high = mid
		}
	}
	return (low + high) / 2
}

// OxygenContentAt is the content of blood in equilibrium at a given tension:
// what hemoglobin holds at the saturation that tension produces, plus what is
// dissolved.
func OxygenContentAt(hemoglobin, paO2 float64) float64 {
	return OxygenContent(hemoglobin, SaturationAt(paO2), paO2)
}

// Carbon monoxide.
//
// It is the most instructive poison a blood model can carry, because everything
// that makes it dangerous is invisible to everything that normally measures
// danger. It binds hemoglobin some two hundred times more readily than oxygen
// does and does not let go, so a fraction of the carrier is simply withdrawn from
// service -- while the oxygen that is still dissolved, and therefore the arterial
// PO₂, is untouched. A blood gas reads normal. A pulse oximeter reads normal, and
// worse than normal: it cannot tell oxyhemoglobin from carboxyhemoglobin and
// reports the sum of the two, so it reads *high* in a patient who is suffocating.
//
// The body has no receptor for it either. The chemoreceptors watch carbon
// dioxide and, faintly, oxygen tension; both are normal, so there is no
// breathlessness, which is why carbon monoxide kills people in their sleep and
// why its victims so often do not get up and leave.
const (
	// COHalfLifeSec is how long the body takes to clear half its
	// carboxyhemoglobin while breathing room air: about five hours.
	//
	// The clearance is the treatment, and the treatment is oxygen, because carbon
	// monoxide and oxygen compete for the same site. Raise the oxygen tension and
	// the competition shifts, which is why the half-life is not one number:
	//
	//	room air              ~300 min
	//	100% oxygen at 1 atm   ~90 min
	//	100% oxygen at 3 atm   ~23 min
	//
	// COClearanceOxygenFactor turns those into one rule, below.
	COHalfLifeSec = 300.0 * 60.0

	// COClearanceOxygenFactor is how much faster clearance runs per mmHg of
	// arterial oxygen tension above normal.
	//
	// Real clearance is not linear in tension, and one straight line cannot hit
	// all three clinical figures exactly. This one gets 300 minutes on room air,
	// 85 against a reported ~90 on a mask, and 28 against a reported ~23 in a
	// chamber -- the right order, the right magnitudes, and the hyperbaric end
	// about a fifth slower than life. That is close enough to make the point and
	// not close enough to quote.
	//
	// Nothing about the chamber is special in the model, and nothing is in the
	// clinic either: it simply puts arterial oxygen at a tension a mask cannot
	// reach, and the competition for the binding site shifts accordingly.
	COClearanceOxygenFactor = 0.0050

	// COFractionPerCigarette is how much of the hemoglobin one cigarette takes
	// out of service, as a fraction.
	//
	// A smoker runs at 4-8% carboxyhemoglobin against a non-smoker's 1-2%, and
	// reaches it a few percent at a time. It is a small number that never quite
	// clears between cigarettes, which is the point: the harm is the standing
	// level, not any single one.
	COFractionPerCigarette = 0.015

	// MaxCarboxyhemoglobin is the ceiling on the arithmetic. Above about 60% the
	// question stops being physiological.
	MaxCarboxyhemoglobin = 0.75

	// COUptakePerPpmPerMl is how much hemoglobin one millilitre of air carrying
	// one part per million of carbon monoxide takes out of service.
	//
	// Uptake scales with how much air is moved as well as how foul it is, which
	// is not a detail: it is why someone working in a contaminated space is
	// poisoned far faster than someone asleep in it, and why the advice is to
	// leave rather than to hurry.
	//
	// Calibrated between the two ends of the clinical range and exactly at
	// neither. Real uptake follows the Coburn-Forster-Kane equation, which is not
	// a straight line, and clearance is not a single exponential; one constant
	// cannot honour both. This one settles near 9% at the 35 ppm of an
	// occupational exposure limit, against a reported 5%, and takes about 1.4
	// hours to reach 50% at 1000 ppm, against a reported 1. The right magnitudes
	// and the right ordering, and not a number to quote.
	COUptakePerPpmPerMl = 4.5e-10
)

// SubstanceCOLoad is a signal putting carbon monoxide onto the blood bus: the
// payload is the fraction of hemoglobin taken out of service.
const SubstanceCOLoad = "co_load"

// EffectiveHemoglobin is the hemoglobin still available to carry oxygen, once
// carbon monoxide has taken its share out of service.
//
// This one line is the whole of what carbon monoxide does here, and it is enough,
// because the rest of the model was already built to care about the difference
// between how much carrier there is and how saturated it looks. Every consequence
// -- the normal blood gas, the falling delivery, the oximeter reading high --
// follows from the carrier shrinking while the tension stays put.
func EffectiveHemoglobin(hemoglobin, carboxyFraction float64) float64 {
	return hemoglobin * (1 - mathx.Clamp(carboxyFraction, 0, MaxCarboxyhemoglobin))
}

// COHalfLifeAt is how fast carbon monoxide is cleared at a given arterial oxygen
// tension, in seconds.
func COHalfLifeAt(paO2 float64) float64 {
	speedup := 1 + COClearanceOxygenFactor*max(paO2-NormalPaO2, 0)
	return COHalfLifeSec / speedup
}
