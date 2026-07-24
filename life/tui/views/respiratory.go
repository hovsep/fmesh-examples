package views

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
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

// Render draws the breathing waveforms. When split is true the left and right
// lungs are drawn as separate side-by-side charts instead of overlaid on one, so
// the two near-identical traces no longer collide.
func (v *RespiratoryView) Render(width, height int, split bool) string {
	if width < 80 {
		return "Terminal too narrow. Please resize to at least 80 columns wide."
	}

	rr := v.State.GetLatestValue("human-Leon::respiratory_rate")
	pp := v.State.GetLatestValue("human-Leon::pleural_pressure")
	mode := "together"
	if split {
		mode = "split"
	}
	header := styles.LabelStyle.Render("Respiratory rate: ") +
		styles.ValueNormalStyle.Render(fmt.Sprintf("%.0f", rr)) +
		styles.UnitStyle.Render(" /min") +
		styles.LabelStyle.Render("     Pleural pressure: ") +
		styles.ValueNormalStyle.Render(fmt.Sprintf("%.1f", pp)) +
		styles.UnitStyle.Render(" cmH₂O") +
		styles.LabelStyle.Render("     Lungs: ") + styles.ValueNormalStyle.Render(mode) +
		styles.LabelStyle.Render(" (press s)")

	// Three stacked rows share the remaining height.
	chartH := max(height-2, 9) / 3

	pleural := widgets.NewLineChart("Pleural Pressure", "cmH₂O",
		widgets.ChartSeries{Signal: v.signal("pleural_pressure"), Color: asciigraph.Orange, Legend: "pleural"},
	).Render(width, chartH)

	volume := v.lungRow("Lung Volume", "mL", "volume", chartH, width, split)
	flow := v.lungRow("Lung Airflow", "mL/s  (+ inspiration / − expiration)", "flow", chartH, width, split)

	return strings.Join([]string{header, "", pleural, volume, flow}, "\n")
}

// lungRow renders a left/right lung metric either as two side-by-side single-lung
// charts (split) or one overlaid two-series chart (together).
func (v *RespiratoryView) lungRow(title, unit, metric string, chartH, width int, split bool) string {
	if !split {
		return widgets.NewLineChart(title, unit,
			widgets.ChartSeries{Signal: v.signal("lung_left_" + metric), Color: asciigraph.DeepSkyBlue, Legend: "left"},
			widgets.ChartSeries{Signal: v.signal("lung_right_" + metric), Color: asciigraph.OrangeRed, Legend: "right"},
		).Render(width, chartH)
	}

	half := (width - 2) / 2
	left := widgets.NewLineChart(title+" — left", unit,
		widgets.ChartSeries{Signal: v.signal("lung_left_" + metric), Color: asciigraph.DeepSkyBlue, Legend: "left"},
	).Render(half, chartH)
	right := widgets.NewLineChart(title+" — right", unit,
		widgets.ChartSeries{Signal: v.signal("lung_right_" + metric), Color: asciigraph.OrangeRed, Legend: "right"},
	).Render(half, chartH)
	return lipgloss.JoinHorizontal(lipgloss.Top, left, "  ", right)
}

func (v *RespiratoryView) signal(name string) *models.SignalData {
	s, _ := v.State.GetSignal("human-Leon::" + name)
	return s
}
