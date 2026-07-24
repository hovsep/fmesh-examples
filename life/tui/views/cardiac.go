package views

import (
	"fmt"
	"strings"
	"time"

	"github.com/hovsep/fmesh-examples/life/tui/models"
	"github.com/hovsep/fmesh-examples/life/tui/styles"
	"github.com/hovsep/fmesh-examples/life/tui/widgets"
)

// CardiacView is a full-screen ECG: a large trace of the heart's contraction
// waveform, where the R-peaks stand out, above the cardiovascular vitals.
//
// The generic metrics list showed cardiac activation as a tiny sparkline; a beat
// is much more legible as a wide, tall waveform over time.
type CardiacView struct {
	State *models.AppState
}

func NewCardiacView(state *models.AppState) *CardiacView {
	return &CardiacView{State: state}
}

func (v *CardiacView) Render(width, height int) string {
	if width < 80 {
		return "Terminal too narrow. Please resize to at least 80 columns wide."
	}

	hr := v.State.GetLatestValue("human-Leon::heart_rate")
	o2 := v.State.GetLatestValue("human-Leon::blood_o2_level")
	co2 := v.State.GetLatestValue("human-Leon::blood_co2_level")
	header := styles.LabelStyle.Render("Heart rate: ") +
		styles.ValueNormalStyle.Render(fmt.Sprintf("%.0f", hr)) + styles.UnitStyle.Render(" BPM") +
		styles.LabelStyle.Render("     Blood O₂: ") +
		styles.ValueNormalStyle.Render(fmt.Sprintf("%.1f", o2)) + styles.UnitStyle.Render(" %") +
		styles.LabelStyle.Render("     Blood CO₂: ") +
		styles.ValueNormalStyle.Render(fmt.Sprintf("%.1f", co2)) + styles.UnitStyle.Render(" %")

	// The ECG takes most of the height; the gauges sit under it. It is an
	// idealised trace drawn at the real heart rate and scrolled by simulated
	// time -- see widgets.ECG for why the raw activation samples are not plotted.
	ecgHeight := max(height-6, 8)
	elapsed := time.Duration(v.State.GetLatestValue(models.SimDurationKey)) * time.Millisecond
	alive := v.State.GetLatestValue("human-Leon::is_alive") != 0
	bpm := hr
	if !alive {
		bpm = 0 // a dead heart flatlines
	}
	ecg := widgets.NewECG("ECG — heartbeat", bpm, elapsed.Seconds()).Render(width, ecgHeight)

	var gauges strings.Builder
	if s, ok := v.State.GetSignal("human-Leon::blood_o2_level"); ok {
		gauges.WriteString(widgets.NewGauge(models.SignalRegistry["human-Leon::blood_o2_level"], s, 20).Render() + "\n")
	}
	if s, ok := v.State.GetSignal("human-Leon::blood_co2_level"); ok {
		gauges.WriteString(widgets.NewGauge(models.SignalRegistry["human-Leon::blood_co2_level"], s, 20).Render())
	}

	return strings.Join([]string{header, "", ecg, "", gauges.String()}, "\n")
}

func (v *CardiacView) signal(name string) *models.SignalData {
	s, _ := v.State.GetSignal("human-Leon::" + name)
	return s
}
