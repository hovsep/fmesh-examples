package views

import (
	"fmt"
	"strings"

	"github.com/guptarohit/asciigraph"
	"github.com/hovsep/fmesh-examples/life/tui/models"
	"github.com/hovsep/fmesh-examples/life/tui/styles"
	"github.com/hovsep/fmesh-examples/life/tui/widgets"
)

// RespiratoryView is a full-screen detail view of the breathing waveforms.
type RespiratoryView struct {
	State *models.AppState
}

func NewRespiratoryView(state *models.AppState) *RespiratoryView {
	return &RespiratoryView{State: state}
}

func (v *RespiratoryView) Render(width, height int) string {
	if width < 80 {
		return "Terminal too narrow. Please resize to at least 80 columns wide."
	}

	rr := v.State.GetLatestValue("human-Leon::respiratory_rate")
	pp := v.State.GetLatestValue("human-Leon::pleural_pressure")
	header := styles.LabelStyle.Render("Respiratory rate: ") +
		styles.ValueNormalStyle.Render(fmt.Sprintf("%.0f", rr)) +
		styles.UnitStyle.Render(" /min") +
		styles.LabelStyle.Render("     Pleural pressure: ") +
		styles.ValueNormalStyle.Render(fmt.Sprintf("%.1f", pp)) +
		styles.UnitStyle.Render(" cmH₂O")

	// Three stacked charts share the remaining height.
	chartH := max(height-2, 9) / 3

	pleural := widgets.NewLineChart("Pleural Pressure", "cmH₂O",
		widgets.ChartSeries{Signal: v.signal("pleural_pressure"), Color: asciigraph.Orange, Legend: "pleural"},
	).Render(width, chartH)

	volume := widgets.NewLineChart("Lung Volume", "mL",
		widgets.ChartSeries{Signal: v.signal("lung_left_volume"), Color: asciigraph.DeepSkyBlue, Legend: "left"},
		widgets.ChartSeries{Signal: v.signal("lung_right_volume"), Color: asciigraph.OrangeRed, Legend: "right"},
	).Render(width, chartH)

	flow := widgets.NewLineChart("Lung Airflow", "mL/s  (+ inspiration / − expiration)",
		widgets.ChartSeries{Signal: v.signal("lung_left_flow"), Color: asciigraph.DeepSkyBlue, Legend: "left"},
		widgets.ChartSeries{Signal: v.signal("lung_right_flow"), Color: asciigraph.OrangeRed, Legend: "right"},
	).Render(width, chartH)

	return strings.Join([]string{header, "", pleural, volume, flow}, "\n")
}

func (v *RespiratoryView) signal(name string) *models.SignalData {
	s, _ := v.State.GetSignal("human-Leon::" + name)
	return s
}
