package factor

//@TODO: let's drop the file for simplicity

import (
	"fmt"

	"github.com/hovsep/fmesh-examples/life/helper"
	"github.com/hovsep/fmesh/component"
	"github.com/hovsep/fmesh/signal"
)

// A barochamber: a sealed room with a pressure of its own.
//
// It is a drop-in replacement for the atmosphere, and the argument that a
// habitat factor is a contract rather than a place. It is built under the same
// name and publishes the same environmental_gas port, so env.Habitat wires it to
// the body by exactly the same naming convention, and the body cannot tell.
// Swapping the world out from under an organism costs one line in getHabitat.
//
// It is not the atmosphere with a different number in it. An atmosphere has an
// altitude and derives a pressure; a chamber has a pressure and no altitude at
// all. An atmosphere is 21% oxygen everywhere and always; a chamber's mixture is
// whatever was pumped into it, which is the entire point of having one. Those
// are different objects, and modelling them as one component with a flag would
// have hidden the only interesting thing about either.
//
// The two settings between them cover most of what chambers are used for:
//
//	pressure 2280, oxygen 100   hyperbaric oxygen therapy, three atmospheres
//	pressure 380,  oxygen 21    a hypobaric chamber, near the 5500 m equivalent
//	pressure 760,  oxygen 10    normobaric hypoxia: an altitude tent, which
//	                            fakes the mountain by thinning the mixture
//	                            rather than the air
//
// The last two rows are worth running one after the other in front of an
// audience. They deliver almost exactly the same inspired oxygen tension -- 0.21
// × (380 − 47) = 70 mmHg against 0.10 × (760 − 47) = 71 -- by opposite means,
// and the body cannot tell them apart, because the alveolar gas equation
// multiplies pressure by fraction and has no way to know which of the two moved.
// That is the whole principle an altitude tent is sold on.
const (
	// StateChamberPressure is the absolute pressure inside, in mmHg.
	StateChamberPressure = "chamber_pressure_mmhg"

	// StateChamberOxygen is the oxygen fraction of the mixture, as a percentage.
	StateChamberOxygen = "chamber_oxygen_pct"

	// StateChamberCOppm is carbon monoxide inside, parts per million. A sealed
	// room is exactly where it accumulates, and exactly where it is treated.
	StateChamberCOppm = "chamber_co_ppm"

	// Command verbs the chamber answers to.
	cmdSetPressure = "set_pressure"
	cmdSetOxygen   = "set_oxygen"

	// Chamber conditions before anyone touches the dials: ordinary room air at
	// one atmosphere, so a body put inside a chamber nobody has set is simply a
	// body indoors.
	defaultChamberPressure = helper.SeaLevelPressure
	defaultChamberOxygen   = 21.0

	// A chamber is climate-controlled, which is one more way it is not weather.
	chamberTemperature = 21.0
	chamberHumidity    = 40.0
)

// GetBarochamberComponent returns a barochamber standing in for the atmosphere.
//
// It is deliberately named "gas", not "barochamber": the name is the wiring
// contract that env.Habitat matches against, and a replacement that had to be
// wired differently would not be a replacement.
func GetBarochamberComponent() (*component.Component, error) {
	c, err := component.New("gas",
		component.WithDescription("Barochamber: a sealed volume at a set pressure and mixture"),
		component.WithInputs("time", "ctl"),
		component.WithOutputs("environmental_gas"),
		component.WithActivationFunc(
			helper.SequentialActivationFunc(
				handleChamberControls,
				emitChamberGas,
			),
		),
		component.WithInitialState(func(state component.State) {
			state.Set(StateChamberPressure, defaultChamberPressure)
			state.Set(StateChamberOxygen, defaultChamberOxygen)
			state.Set(StateChamberCOppm, 0.0)
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("barochamber component: %w", err)
	}
	return c, nil
}

func handleChamberControls(this *component.Component) error {
	return this.InputByName("ctl").
		Signals().
		Filter(func(s *signal.Signal) bool {
			return s.Labels().Has("cmd")
		}).ForEach(func(ctlSig *signal.Signal) error {
		switch ctlSig.Labels().ValueOrDefault("cmd", "") {
		case cmdSetPressure:
			// A chamber cannot be pumped down to nothing, and the floor is not a
			// safety feature: below the vapour pressure of water at body
			// temperature the alveolar gas equation has no positive term left,
			// and a body would not be short of oxygen so much as boiling.
			pressure := max(helper.AsF64OrDefault(ctlSig, defaultChamberPressure), 50.0)
			this.State().Set(StateChamberPressure, pressure)
			this.Logger().Printf("chamber at %.0f mmHg (%.2f atmospheres)",
				pressure, pressure/helper.SeaLevelPressure)
		case cmdSetCO:
			ppm := max(helper.AsF64OrDefault(ctlSig, 0.0), 0)
			this.State().Set(StateChamberCOppm, ppm)
			this.Logger().Printf("chamber air carrying %.0f ppm carbon monoxide", ppm)
		case cmdSetOxygen:
			oxygen := min(max(helper.AsF64OrDefault(ctlSig, defaultChamberOxygen), 1.0), 100.0)
			this.State().Set(StateChamberOxygen, oxygen)
			this.Logger().Printf("chamber mixture now %.0f%% oxygen", oxygen)
		}
		return nil
	})
}

func emitChamberGas(this *component.Component) error {
	// Same guard as the atmosphere: emit on a tick and not on a control signal,
	// or the body is left holding an unpaired breath of air and the mesh never
	// converges.
	if !this.InputByName("time").HasSignals() {
		return nil
	}

	oxygen := this.State().Get(StateChamberOxygen).(float64)
	pressure := this.State().Get(StateChamberPressure).(float64)

	// Whatever is not oxygen is nitrogen. Real chambers use helium in the deep
	// mixes, for reasons -- narcosis, density, the work of breathing -- that this
	// body has no way to feel, so pretending otherwise would be decoration.
	air, err := helper.PackAir(100.0-oxygen, oxygen, 0, 0, chamberTemperature, chamberHumidity)
	if err != nil {
		return fmt.Errorf("emit chamber gas: %w", err)
	}

	return this.OutputByName("environmental_gas").PutSignals(
		helper.WithCarbonMonoxide(
			helper.WithPressure(air, pressure),
			this.State().Get(StateChamberCOppm).(float64)))
}
