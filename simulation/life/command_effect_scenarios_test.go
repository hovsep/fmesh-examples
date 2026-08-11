package main

import (
	"time"

	"github.com/hovsep/fmesh-examples/simulation/sim/simtest"
)

// brimming warms long enough for the reservoirs to have something in them, since
// a bladder cannot be emptied before the kidney has filled it.
//
// Half an hour rather than the hour it began as: the kidney has the bladder a
// quarter full by then, which is as much as a test needs to watch it emptied,
// and the four scenarios that use this were between them a fifth of the whole
// package's running time.
var brimming = simtest.Scenario{
	Warm: 30 * time.Minute, Baseline: 60 * time.Second,
	Settle: 60 * time.Second, Window: 20 * time.Second,
}

// The claim this simulation makes about each of its commands.
//
// Read a group as a sentence: this command, in this world, moves these things
// this way, over this long. Everything not named is asserted to have stayed
// where it was, so the interesting entries are often the ones that say a value
// must NOT move -- those state that a mechanism is genuinely not connected, and
// they are the entries a wrong wiring trips over.
//
// Repeated chains are written once as fragments (ventilating, inspiring,
// failing, suffocating) and merged in, because each is one fact rather than a
// dozen coincidences: when the bellows moves more air, everything downstream of
// the bellows moves together, and restating that in every scenario would bury
// the line that is the actual claim.

// checks is a map literal, named for readability at the call sites below.
type checks = map[string]simtest.Check

// --- the air ---------------------------------------------------------------

func airScenarios() []simtest.Scenario {
	return []simtest.Scenario{
		// Thin air is the oxygen cascade end to end: less pressure, less oxygen
		// in the alveoli, less in the blood, a body that breathes harder about it
		// and blows off carbon dioxide doing so, and a body that says so.
		wholeBody(at(quick, "altitude/3000m", "altitude 3000", merge(
			suffocating(),
			ventilating(simtest.Up),
			checks{
				"air:pressure": down(),
				"blood_paco2":  down(),
				// Breathing off CO2 the body is not making any faster: the
				// exhaled mixture gets weaker, not stronger.
				"lung_left_exhaled_gas:composition:carbon_dioxide":  down(),
				"lung_right_exhaled_gas:composition:carbon_dioxide": down(),
				// Thin air is not a metabolic problem and must not read as one.
				"glycemia":              steady(0.02),
				"venous_blood:volume_l": frozen(),
			}))),

		// 5500 m, which this model does not let anyone survive.
		//
		// It should: people live year-round at 5100 m and Everest base camp is at
		// 5364. What the model does instead is starve the brain until the drive
		// collapses, within about six simulated minutes. This asserts what the
		// model actually does rather than what a physiologist would expect, so
		// the day that is fixed this test fails and says so.
		dying(at(paced, "altitude/5500m is fatal (it should not be)", "altitude 5500", merge(
			suffocating(), failing(),
			checks{
				"air:pressure":   down(),
				"is_alive":       down(),
				"brain_activity": down(),
			}))),

		dying(at(paced, "altitude/8848m is fatal", "altitude 8848", merge(
			suffocating(), failing(),
			checks{
				"air:pressure":   down(),
				"is_alive":       down(),
				"brain_activity": down(),
				"cardiac_output": down(),
			}))),

		// Issued from sea level, where it is a no-op: the command is accepted and
		// the world is already where it asks to be. Worth stating, because "the
		// command ran" and "something changed" are different claims.
		at(quick, "altitude/already at sea level", "altitude 0", checks{
			"air:pressure": frozen(),
		}),

		at(quick, "air:pressure/hyperbaric", "air:pressure 2280", checks{
			"air:pressure":      up(),
			"blood_pao2":        up(),
			"blood_spo2":        up(),
			"venous_blood:SpO2": up(),
			"venous_blood:CaO2": up(),
		}),

		// 380 mmHg is the pressure at about 5000 m, and is fatal here too.
		dying(at(paced, "air:pressure/half an atmosphere is fatal", "air:pressure 380", merge(
			suffocating(), failing(),
			checks{
				"air:pressure":   down(),
				"is_alive":       down(),
				"brain_activity": down(),
			}))),

		wholeBody(at(quick, "air:preset/hyperbaric", "air:preset hyperbaric", merge(
			inspiring(simtest.Up, "composition:oxygen"),
			inspiring(simtest.Down, "composition:nitrogen", "composition:argon"),
			checks{
				"air:pressure":           up(),
				"air:composition:oxygen": up(),
				"blood_pao2":             up(),
				"blood_spo2":             up(),
				"venous_blood:SpO2":      up(),
				"venous_blood:CaO2":      up(),
				// A preset replaces the whole atmosphere, not just its pressure.
				"air:humidity":                                down(),
				"air:composition:pollution":                   down(),
				"inspired_gas:humidity":                       down(),
				"inspired_gas:composition:pollution":          down(),
				"lung_left_alveolar_gas:O2_vol":               up(),
				"lung_right_alveolar_gas:O2_vol":              up(),
				"lung_left_exhaled_gas:composition:oxygen":    up(),
				"lung_right_exhaled_gas:composition:oxygen":   up(),
				"lung_left_exhaled_gas:composition:nitrogen":  down(),
				"lung_right_exhaled_gas:composition:nitrogen": down(),
				"lung_left_exhaled_gas:composition:argon":     down(),
				"lung_right_exhaled_gas:composition:argon":    down(),
			}))),

		dying(at(paced, "air:preset/everest", "air:preset everest", merge(
			suffocating(), failing(),
			checks{
				"air:pressure": down(),
				"is_alive":     down(),
			}))),

		wholeBody(at(quick, "air:oxygen/pure", "air:oxygen 100", merge(
			inspiring(simtest.Up, "composition:oxygen"),
			inspiring(simtest.Down, "composition:nitrogen", "composition:argon"),
			checks{
				"air:composition:oxygen": up(),
				"blood_pao2":             up(),
				"blood_spo2":             up(),
				"venous_blood:SpO2":      up(),
				"venous_blood:CaO2":      up(),
				"lung_left_exhaled_gas:composition:oxygen":    up(),
				"lung_right_exhaled_gas:composition:oxygen":   up(),
				"lung_left_exhaled_gas:composition:nitrogen":  down(),
				"lung_right_exhaled_gas:composition:nitrogen": down(),
				"lung_left_exhaled_gas:composition:argon":     down(),
				"lung_right_exhaled_gas:composition:argon":    down(),
				"lung_left_alveolar_gas:O2_vol":               up(),
				"lung_right_alveolar_gas:O2_vol":              up(),
				"air:composition:pollution":                   down(),
				"inspired_gas:composition:pollution":          down(),
			}))),

		dying(at(paced, "air:oxygen/thin", "air:oxygen 10", merge(
			suffocating(), failing(),
			checks{
				"air:composition:oxygen":            down(),
				"inspired_gas:composition:oxygen":   down(),
				"inspired_gas:composition:nitrogen": up(),
				"is_alive":                          down(),
			}))),

		// Carbon monoxide is the lesson the dashboard exists to teach: the
		// carrying capacity collapses while the saturation reads fine. Asserting
		// that SpO2 does NOT move is the whole point of the case.
		wholeBody(at(paced, "air:co/poisoning", "air:co 1000", checks{
			"air:carbon_monoxide_ppm": up(),
			"venous_blood:COHb":       up(),
			"venous_blood:CaO2":       down(),
			"blood_spo2":              steady(0.03),
			"blood_pao2":              steady(0.05),
			"venous_blood:SpO2":       steady(0.03),
		})),
		at(quick, "air:co/none", "air:co 0", checks{
			"venous_blood:COHb": frozen(),
		}),

		wholeBody(at(paced, "air:mixin/wood fire", "air:mixin wood_fire 30m", merge(
			inspiring(simtest.Up, "composition:pollution"),
			failing(),
			checks{
				"air:carbon_monoxide_ppm":   up(),
				"air:composition:pollution": up(),
				"venous_blood:COHb":         up(),
				"venous_blood:CaO2":         down(),
			}))),
		wholeBody(at(paced, "air:mixin/car exhaust", "air:mixin car_exhaust 30m", merge(
			inspiring(simtest.Up, "composition:pollution"),
			failing(),
			checks{
				"air:carbon_monoxide_ppm":   up(),
				"air:composition:pollution": up(),
				"venous_blood:COHb":         up(),
			}))),

		wholeBody(at(paced, "smoke:cigarette/one", "smoke:cigarette", merge(
			inspiring(simtest.Up, "composition:pollution"),
			failing(),
			checks{
				"air:carbon_monoxide_ppm":   up(),
				"air:composition:pollution": up(),
				"venous_blood:COHb":         up(),
			}))),
		wholeBody(at(paced, "smoke:cigarette/three", "smoke:cigarette 3", merge(
			inspiring(simtest.Up, "composition:pollution"),
			failing(),
			checks{
				"air:carbon_monoxide_ppm":   up(),
				"air:composition:pollution": up(),
				"venous_blood:COHb":         up(),
			}))),

		after(wholeBody(at(paced, "air:clear/after a fire", "air:clear", merge(
			inspiring(simtest.Down, "composition:pollution"),
			checks{
				"air:carbon_monoxide_ppm":   down(),
				"air:composition:pollution": down(),
			}))), "air:mixin wood_fire 2h"),
		after(wholeBody(at(paced, "air:clear/after cigarettes", "air:clear", merge(
			inspiring(simtest.Down, "composition:pollution"),
			checks{
				"air:carbon_monoxide_ppm":   down(),
				"air:composition:pollution": down(),
			}))), "smoke:cigarette 5"),
	}
}

// --- the weather and the sky ------------------------------------------------

func weatherScenarios() []simtest.Scenario {
	return []simtest.Scenario{
		// The chain the whole habitat exists to demonstrate, and the one that was
		// broken: hot air must reach the core, and a warm core must sweat and
		// cost water. Asserting only the air temperature is what let this sit
		// broken through fifty simulated minutes at 38 C.
		wholeBody(at(slow, "temp:hot/body answers the heat", "temp:hot", merge(
			inspiring(simtest.Up, "temperature"),
			checks{
				"air:temperature":  up(),
				"body_temperature": up(),
				"sweat_rate":       up(),
				"hydration":        down(),
				"shiver_rate":      steady(0.02),
			}))),
		// And the mirror: cold must reach the core and start the other effector.
		wholeBody(at(paced, "temp:cold/body answers the cold", "temp:cold", merge(
			inspiring(simtest.Down, "temperature"),
			failing(),
			checks{
				"air:temperature":  down(),
				"body_temperature": down(),
				"shiver_rate":      up(),
				"sweat_rate":       steady(0.02),
			}))),
		wholeBody(at(slow, "temp:zero/freezing", "temp:zero", merge(
			inspiring(simtest.Down, "temperature"),
			checks{
				"air:temperature":  down(),
				"body_temperature": down(),
				"shiver_rate":      up(),
			}))),

		// A single degree is a real change to the air and, correctly, nothing at
		// all to the body: it is well inside what a body compensates for free.
		at(quick, "temp:inc/one degree", "temp:inc", checks{
			"air:temperature":          up(),
			"inspired_gas:temperature": up(),
			"body_temperature":         steady(0.005),
			"sweat_rate":               frozen(),
			"shiver_rate":              frozen(),
		}),
		at(quick, "temp:dec/one degree", "temp:dec", checks{
			"air:temperature":          down(),
			"inspired_gas:temperature": down(),
			"body_temperature":         steady(0.005),
			"sweat_rate":               frozen(),
			"shiver_rate":              frozen(),
		}),

		wholeBody(at(slow, "sun:hour/midday", "sun:hour 12", merge(
			inspiring(simtest.Up, "temperature"),
			checks{
				"sun:uvi":         up(),
				"air:temperature": up(),
				// Nothing about the sky may reach the blood sugar.
				"glycemia": steady(0.02),
			}))),
		at(quick, "sun:hour/midnight", "sun:hour 0", checks{
			"sun:uvi": steady(0.02),
		}),

		// The second dose of each temperature command. One case proves the
		// command is connected; a second in a different world proves the body is
		// answering the temperature rather than the keystroke.
		after(wholeBody(at(slow, "temp:hot/rewarming a chilled body", "temp:hot", merge(
			inspiring(simtest.Up, "temperature"),
			checks{
				"air:temperature":  up(),
				"body_temperature": up(),
				"shiver_rate":      down(),
				"sweat_rate":       up(),
				"bladder_fill":     up(),
			}))), "temp:cold"),
		after(wholeBody(at(paced, "temp:cold/chilling a hot body", "temp:cold", merge(
			inspiring(simtest.Down, "temperature"),
			failing(),
			checks{
				"air:temperature":  down(),
				"body_temperature": down(),
				"shiver_rate":      up(),
			}))), "temp:hot"),
		after(wholeBody(at(quick, "temp:zero/from a hot room", "temp:zero", merge(
			inspiring(simtest.Down, "temperature"),
			checks{"air:temperature": down()},
		))), "temp:hot"),
		// A degree at a time, ten times over, out of the cold -- and ten degrees
		// of relief from thirty-five below is still twenty-five below. The air
		// answers the command and the body answers the air, and the air is still
		// far too cold: the core goes on falling and the shivering goes on
		// failing to stop it.
		after(wholeBody(at(slow, "temp:inc/ten degrees is not enough", "temp:inc", merge(
			inspiring(simtest.Up, "temperature"),
			failing(),
			checks{
				"air:temperature":  up(),
				"body_temperature": down(),
				"shiver_rate":      up(),
				"bladder_fill":     up(),
			}))),
			"temp:cold", "temp:inc", "temp:inc", "temp:inc", "temp:inc",
			"temp:inc", "temp:inc", "temp:inc", "temp:inc", "temp:inc"),
		after(wholeBody(at(quick, "temp:dec/from a hot room", "temp:dec", merge(
			inspiring(simtest.Down, "temperature"),
			checks{"air:temperature": down()},
		))), "temp:hot"),
	}
}

// --- effort -----------------------------------------------------------------

func activityScenarios() []simtest.Scenario {
	return []simtest.Scenario{
		// Hard work, and the whole cardiovascular and respiratory answer to it.
		// Stroke volume is named explicitly because it sat at exactly 70.00 mL
		// through a sprint for as long as nothing asked it to do otherwise.
		wholeBody(at(paced, "activity:start/hard effort", "activity:start 8", merge(
			ventilating(simtest.Up),
			checks{
				"heart_rate":             up(),
				"stroke_volume":          up(),
				"cardiac_output":         up(),
				"mean_arterial_pressure": up(),
				"vascular_resistance":    down(),
				"blood_paco2":            up(),
				"venous_blood:pH":        down(),
				// The runner's muscles make CO2 faster than the extra breathing
				// clears it, so what comes back out is a stronger mixture --
				// the opposite of the climber's, on the same rise in ventilation.
				"lung_left_exhaled_gas:composition:carbon_dioxide":  up(),
				"lung_right_exhaled_gas:composition:carbon_dioxide": up(),
				"muscle_fatigue":          up(),
				"venous_blood:adrenaline": up(),
				"venous_blood:cortisol":   up(),
				"venous_blood:glucagon":   up(),
				"body_temperature":        up(),
				"energy":                  down(),
				"glycemia":                down(),
				"feelings:tired":          up(),
				"feelings:content":        down(),
				"feelings:breathless":     up(),
				"sweat_rate":              up(),
				// Work this hard outruns what the circulation can deliver, and
				// the organs start taking damage for it.
				"brain_damage": up(), "heart_damage": up(), "kidney_damage": up(),
				"liver_damage": up(), "pancreas_damage": up(), "diaphragm_damage": up(),
				// Effort moves a great deal, but not a drop of blood in or out.
				"venous_blood:volume_l":   frozen(),
				"venous_blood:hemoglobin": frozen(),
			}))),

		// A gentle effort must move the same things less. That two intensities
		// give two answers is what makes it a response rather than a switch.
		wholeBody(at(paced, "activity:start/gentle effort", "activity:start 3", merge(
			ventilating(simtest.Up),
			checks{
				"heart_rate":              up(),
				"cardiac_output":          up(),
				"muscle_fatigue":          up(),
				"energy":                  down(),
				"sweat_rate":              up(),
				"feelings:tired":          up(),
				"vascular_resistance":     down(),
				"venous_blood:adrenaline": up(),
				"venous_blood:cortisol":   up(),
				"venous_blood:glucagon":   up(),
				"lung_left_exhaled_gas:composition:carbon_dioxide":  up(),
				"lung_right_exhaled_gas:composition:carbon_dioxide": up(),
				// Not enough work to need the pressure raised much.
				"mean_arterial_pressure": steady(0.15),
			}))),

		wholeBody(at(paced, "activity:start/near maximal", "activity:start 10", merge(
			ventilating(simtest.Up),
			checks{
				"heart_rate":              up(),
				"stroke_volume":           up(),
				"cardiac_output":          up(),
				"muscle_fatigue":          up(),
				"sweat_rate":              up(),
				"body_temperature":        up(),
				"venous_blood:adrenaline": up(),
				"venous_blood:cortisol":   up(),
				"venous_blood:glucagon":   up(),
				"vascular_resistance":     down(),
				"blood_paco2":             up(),
				"feelings:tired":          up(),
				"feelings:content":        down(),
				"feelings:breathless":     up(),
				"feelings:headache":       up(),
				"lung_left_exhaled_gas:composition:carbon_dioxide":  up(),
				"lung_right_exhaled_gas:composition:carbon_dioxide": up(),
				"brain_damage": up(), "heart_damage": up(), "kidney_damage": up(),
				"liver_damage": up(), "pancreas_damage": up(), "diaphragm_damage": up(),
			}))),

		// Stopping has to actually bring things back, which the old suite could
		// not say: it passed on a heart rate that fell from 155 to 154.
		after(wholeBody(at(paced, "activity:stop/pulse returns to rest", "activity:stop", checks{
			"heart_rate":              backTo(64, 0.25),
			"cardiac_output":          backTo(4.5, 0.35),
			"stroke_volume":           backTo(70, 0.15),
			"vascular_resistance":     up(),
			"venous_blood:adrenaline": down(),
			"venous_blood:insulin":    up(),
			"feelings:tired":          down(),
			"feelings:content":        up(),
		})), "activity:start 8"),
		after(wholeBody(at(paced, "activity:stop/breathing returns to rest", "activity:stop", checks{
			"respiratory_rate":        backTo(12, 0.35),
			"blood_paco2":             backTo(40, 0.15),
			"heart_rate":              down(),
			"cardiac_output":          down(),
			"vascular_resistance":     up(),
			"venous_blood:adrenaline": down(),
			"venous_blood:insulin":    up(),
			"feelings:tired":          down(),
			"feelings:content":        up(),
		})), "activity:start 8"),
	}
}

// --- eating, drinking, and getting rid of it --------------------------------

func intakeScenarios() []simtest.Scenario {
	return []simtest.Scenario{
		wholeBody(at(paced, "intake:water/half a litre", "intake:water 500ml", checks{
			"stomach_fill": up(),
			"hydration":    up(),
			// Drinking does not put water straight into the circulation.
			"venous_blood:volume_l": frozen(),
			"glycemia":              steady(0.02),
		})),
		wholeBody(at(paced, "intake:water/two litres", "intake:water 2l", checks{
			"stomach_fill": up(),
			"hydration":    up(),
		})),

		wholeBody(at(paced, "intake:food/small meal", "intake:food 200kcal", checks{
			"stomach_fill":         up(),
			"glycemia":             up(),
			"venous_blood:insulin": up(),
			"bowel_fill":           up(),
		})),
		wholeBody(at(paced, "intake:food/large meal", "intake:food 800kcal", checks{
			"stomach_fill":         up(),
			"glycemia":             up(),
			"venous_blood:insulin": up(),
			"bowel_fill":           up(),
		})),

		// The bladder fills on its own, so these warm for an hour before there is
		// anything worth emptying.
		wholeBody(at(brimming, "excretion:urinate/full bladder", "excretion:urinate", checks{
			"bladder_fill": down(),
		})),
		wholeBody(at(brimming, "excretion:urinate/again straight away", "excretion:urinate", checks{
			"bladder_fill": down(),
		})),
		after(wholeBody(at(brimming, "excretion:defecate/after a meal", "excretion:defecate", checks{
			"bowel_fill": down(),
		})), "intake:food 800kcal"),
		after(wholeBody(at(brimming, "excretion:defecate/after two meals", "excretion:defecate", checks{
			"bowel_fill": down(),
		})), "intake:food 800kcal", "intake:food 800kcal"),
	}
}

// --- feeling things ---------------------------------------------------------

func affectScenarios() []simtest.Scenario {
	return []simtest.Scenario{
		// A fright is the endocrine chain on its own, with no work being done:
		// the glands answer, the heart answers them, and the body reports it.
		wholeBody(at(paced, "emotion:stimulus/a fright", "emotion:stimulus 0.9 -0.8", checks{
			"venous_blood:adrenaline": up(),
			"heart_rate":              up(),
			"cardiac_output":          up(),
			"mean_arterial_pressure":  up(),
			"feelings:anxious":        up(),
			"feelings:content":        down(),
			"venous_blood:cortisol":   up(),
			// Being frightened is not being tired, and costs no muscle.
			"muscle_fatigue": frozen(),
			"energy":         steady(0.02),
		})),
		wholeBody(at(paced, "emotion:stimulus/good news", "emotion:stimulus 0.7 0.9", checks{
			"venous_blood:adrenaline": up(),
			"heart_rate":              up(),
			"feelings:happy":          up(),
			// Good news empties the same glands bad news does: arousal is
			// arousal, and it is the valence that differs, not the chemistry.
			"venous_blood:cortisol": up(),
			// Which is exactly why this line matters -- pleasant arousal must
			// not read as anxiety, even though the hormones cannot tell.
			"feelings:anxious": steady(0.02),
		})),
		wholeBody(at(paced, "emotion:stimulus/mild", "emotion:stimulus 0.2 -0.1", checks{
			"venous_blood:adrenaline": up(),
			"venous_blood:cortisol":   up(),
			"feelings:anxious":        up(),
		})),
	}
}

// --- injury -----------------------------------------------------------------

func traumaScenarios() []simtest.Scenario {
	return []simtest.Scenario{
		// A moderate bleed: preload falls, the heart cannot fill, and every
		// compensation the body has comes on at once.
		wholeBody(at(paced, "trauma:bleed/moderate", "trauma:bleed 1500ml", checks{
			"venous_blood:volume_l":   down(),
			"stroke_volume":           down(),
			"cardiac_output":          down(),
			"heart_rate":              up(),
			"vascular_resistance":     up(),
			"venous_blood:adrenaline": up(),
			"venous_blood:cortisol":   up(),
			"venous_blood:hemoglobin": down(),
			// The compensation is the point: pressure falls far less than flow.
			"mean_arterial_pressure": steady(0.15),
		})),
		wholeBody(at(quick, "trauma:bleed/small", "trauma:bleed 300ml", checks{
			"venous_blood:volume_l": down(),
			"stroke_volume":         down(),
			"cardiac_output":        down(),
			"heart_rate":            up(),
			"vascular_resistance":   up(),
		})),
		wholeBody(at(paced, "trauma:bleed/severe", "trauma:bleed 2500ml", merge(
			failing(),
			checks{
				"venous_blood:volume_l":   down(),
				"stroke_volume":           down(),
				"cardiac_output":          down(),
				"heart_rate":              up(),
				"mean_arterial_pressure":  down(),
				"vascular_resistance":     up(),
				"venous_blood:adrenaline": up(),
				"venous_blood:cortisol":   up(),
			}))),
	}
}
