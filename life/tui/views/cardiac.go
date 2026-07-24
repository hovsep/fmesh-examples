package views

import (
	"fmt"
	"strings"

	"github.com/guptarohit/asciigraph"
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

	// The ECG takes most of the height; the gauges sit under it. A fixed Y axis
	// (the activation waveform runs from about -0.2 at the S-wave to 1.0 at the
	// R-peak) keeps the baseline and the peaks steady instead of the plot
	// rescaling every frame.
	ecgHeight := max(height-6, 8)
	ecg := widgets.NewLineChart("ECG — cardiac activation (R-peaks)", "",
		widgets.ChartSeries{
			Signal: v.signal("heart_cardiac_activation"),
			Color:  asciigraph.Red,
			Legend: "cardiac",
		},
	).WithYBounds(-0.3, 1.1).Render(width, ecgHeight)

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
