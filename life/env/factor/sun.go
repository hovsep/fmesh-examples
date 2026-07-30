package factor

import (
	"fmt"
	"math"
	"time"

	"github.com/hovsep/fmesh-examples/simulation/command"
	"github.com/hovsep/fmesh-examples/simulation/simtime"
	"github.com/hovsep/fmesh/component"
	"github.com/hovsep/fmesh/meta"
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

// StateHourOffset shifts the time of day without moving the simulation clock.
//
// A day is twenty-four hours long and a simulation usually is not, so waiting
// for noon is not a reasonable way to look at noon. The offset lets the sky be
// set directly, and time then goes on passing from there.
const StateHourOffset = "hour_offset"

// StatePendingHour holds an hour that has been asked for but not yet turned
// into an offset.
//
// The two cannot happen at once. A command arrives on its own mesh cycle, and
// the sun may well activate before the clock does on that cycle, so the elapsed
// time needed to work out the offset is not available yet -- and a port is
// drained at the end of a cycle, so a command that waits for the clock is a
// command that is simply lost. It is remembered instead, and converted on the
// next tick.
const StatePendingHour = "pending_hour"

// cmdSetHour sets the hour of the day.
const cmdSetHour = "set_hour"

// GetSunComponent returns the sun radiation exposure factor of the habitat.
func GetSunComponent() (*component.Component, error) {
	c, err := component.New("sun",
		component.WithDescription("Sun radiation exposure factor (day/night UV and illuminance cycle)"),
		component.WithInputs("time", "ctl"),
		component.WithOutputs("uvi", "lux"), // UV index 0..11, illuminance in lux
		component.WithActivationFunc(component.Sequential(setTimeOfDay, emitSunlight)),
		component.WithInitialState(func(state component.State) {
			state.Set(StateHourOffset, 0.0)
			state.Set(StatePendingHour, math.NaN()) // nothing asked for
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("sun component: %w", err)
	}
	return c, nil
}

// setTimeOfDay records an hour the sky has been asked to show.
func setTimeOfDay(this *component.Component) error {
	return command.ForEach(this, "ctl", func(name string, args *meta.Scalars) error {
		if command.Verb(name) != cmdSetHour {
			return nil
		}
		this.State().Set(StatePendingHour, args.ValueOrDefault("hour", 12))
		return nil
	})
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

	// A requested hour becomes an offset here, where the elapsed time is known.
	if pending := this.State().Get(StatePendingHour).(float64); !math.IsNaN(pending) {
		this.State().Set(StateHourOffset, pending-math.Mod(simDuration.Hours(), 24))
		this.State().Set(StatePendingHour, math.NaN())
		this.Logger().Printf("the sky is now at %.1f o'clock", pending)
	}

	offset := this.State().Get(StateHourOffset).(float64)
	uvi, lux := daylight(simDuration, offset)
	if err := this.OutputByName("uvi").PutPayloads(uvi); err != nil {
		return err
	}
	return this.OutputByName("lux").PutPayloads(lux)
}

// daylight returns the UV index and illuminance for the time of day, peaking at
// solar noon and zero at night.
func daylight(elapsed time.Duration, hourOffset float64) (uvi, lux float64) {
	//@TODO: shall we add some random clouds effects? If so let's have a weather widjet in TUI
	hour := math.Mod(math.Mod(elapsed.Hours()+hourOffset, 24)+24, 24)
	if hour < sunriseHour || hour > sunsetHour {
		return 0, 0
	}

	// A half-sine across the daylight window: 0 at sunrise/sunset, 1 at midday.
	daylen := sunsetHour - sunriseHour
	intensity := math.Sin((hour - sunriseHour) / daylen * math.Pi)
	return peakUVIndex * intensity, peakLux * intensity
}
