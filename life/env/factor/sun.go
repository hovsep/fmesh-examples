package factor

import (
	"fmt"
	"math"
	"time"

	"github.com/hovsep/fmesh-examples/simulation/simtime"
	"github.com/hovsep/fmesh/component"
)

const (
	// peakUVIndex is the UV index at solar noon on a clear day; peakLux the
	// illuminance. Both follow a simple day/night cycle so a long exposure or a
	// night scenario differs, without pretending to real astronomy.
	peakUVIndex = 8.0
	peakLux     = 100000.0

	// sunriseHour and sunsetHour bound daylight; the sun is dark outside them.
	sunriseHour = 6.0
	sunsetHour  = 20.0
)

// @TODO: make sun to affect air temp
// @TODO:
// GetSunComponent returns the sun radiation exposure factor of the habitat.
func GetSunComponent() (*component.Component, error) {
	c, err := component.New("sun",
		component.WithDescription("Sun radiation exposure factor (day/night UV and illuminance cycle)"),
		component.WithInputs("time", "ctl"),
		component.WithOutputs("uvi", "lux"), // UV index 0..11, illuminance in lux
		component.WithActivationFunc(emitSunlight),
	)
	if err != nil {
		return nil, fmt.Errorf("sun component: %w", err)
	}
	return c, nil
}

func emitSunlight(this *component.Component) error {
	tick := this.InputByName("time").Signals().First()
	if tick == nil {
		return nil
	}

	_, simDuration, _, _, err := simtime.UnpackTick(tick)
	if err != nil {
		return fmt.Errorf("sun tick: %w", err)
	}

	uvi, lux := daylight(simDuration)
	if err := this.OutputByName("uvi").PutPayloads(uvi); err != nil {
		return err
	}
	return this.OutputByName("lux").PutPayloads(lux)
}

// daylight returns the UV index and illuminance for the time of day, peaking at
// solar noon and zero at night.
func daylight(elapsed time.Duration) (uvi, lux float64) {
	//@TODO: shall we add some random clouds effects? If so let's have a weather widjet in TUI
	hour := math.Mod(elapsed.Hours(), 24)
	if hour < sunriseHour || hour > sunsetHour {
		return 0, 0
	}

	// A half-sine across the daylight window: 0 at sunrise/sunset, 1 at midday.
	daylen := sunsetHour - sunriseHour
	intensity := math.Sin((hour - sunriseHour) / daylen * math.Pi)
	return peakUVIndex * intensity, peakLux * intensity
}
