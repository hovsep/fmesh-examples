package views

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/hovsep/fmesh-examples/life/telemetry"
	"github.com/hovsep/fmesh-examples/life/tuiv2/models"
	"github.com/hovsep/fmesh-examples/life/tuiv2/styles"
	"github.com/hovsep/fmesh-examples/life/tuiv2/widgets"
)

// MetricsView renders every metric belonging to one part of the body.
//
// It is built from telemetry.Catalog rather than from a hand-written list of
// panels, so a value added to the catalog appears here on its own. That is what
// keeps "all important metrics are shown" true as the body grows, instead of
// true only on the day someone last updated the dashboard.
type MetricsView struct {
	State   *models.AppState
	Title   string
	View    telemetry.View
	subject string
}

func NewMetricsView(state *models.AppState, title string, view telemetry.View, subject string) *MetricsView {
	return &MetricsView{State: state, Title: title, View: view, subject: subject}
}

func (v *MetricsView) Render(width, height int) string {
	signals := v.signals()
	if len(signals) == 0 {
		return styles.PanelStyle.Render("No metrics are published for this view yet.")
	}

	// Two columns while there is room, so a long list stays readable rather than
	// scrolling off the bottom.
	columns := 1
	if width >= 100 && len(signals) > height-6 {
		columns = 2
	}
	columnWidth := (width - 6) / columns

	rendered := make([]string, 0, len(signals))
	for _, s := range signals {
		data, ok := v.State.GetSignal(s.Key)
		if !ok {
			continue
		}
		metadata := models.SignalRegistry[s.Key]
		rendered = append(rendered, widgets.NewVitalSign(metadata, data).Render(columnWidth-4))
	}

	if columns == 1 {
		return styles.PanelStyle.Width(width - 4).Render(
			styles.PanelTitleStyle.Render(v.Title) + "\n" + strings.Join(rendered, "\n"))
	}

	split := (len(rendered) + 1) / 2
	left := styles.PanelStyle.Width(columnWidth).Render(
		styles.PanelTitleStyle.Render(v.Title) + "\n" + strings.Join(rendered[:split], "\n"))
	right := styles.PanelStyle.Width(columnWidth).Render(
		styles.PanelTitleStyle.Render(" ") + "\n" + strings.Join(rendered[split:], "\n"))

	return lipgloss.JoinHorizontal(lipgloss.Top, left, right)
}

// signals returns the catalog entries belonging to this view.
func (v *MetricsView) signals() []telemetry.Signal {
	var mine []telemetry.Signal
	for _, s := range telemetry.Signals(v.subject) {
		if s.View == v.View {
			mine = append(mine, s)
		}
	}
	return mine
}
