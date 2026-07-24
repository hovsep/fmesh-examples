package views

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/hovsep/fmesh-examples/life/telemetry"
	"github.com/hovsep/fmesh-examples/life/tui/models"
	"github.com/hovsep/fmesh-examples/life/tui/styles"
	"github.com/hovsep/fmesh-examples/life/tui/widgets"
)

// FeelingsView shows what the body feels, alongside the numbers behind it.
//
// The point of the pairing is that a dashboard of readings does not tell you
// whether someone is having a bad time. "Thirsty 62%" does; the hydration
// percentage next to it explains why.
type FeelingsView struct {
	State    *models.AppState
	feelings *widgets.Feelings
	subject  string
}

func NewFeelingsView(state *models.AppState, subject string) *FeelingsView {
	return &FeelingsView{
		State:    state,
		feelings: widgets.NewFeelings(state, subject),
		subject:  subject,
	}
}

func (v *FeelingsView) Render(width, height int) string {
	panelWidth := (width - 6) / 2
	if panelWidth < 30 {
		// Too narrow to sit side by side; the feelings alone are the point.
		return styles.PanelStyle.Width(width - 4).Render(
			styles.PanelTitleStyle.Render("HOW LEON FEELS") + "\n" + v.feelings.Render(width-8))
	}

	left := styles.PanelStyle.Width(panelWidth).Height(height - 4).Render(
		styles.PanelTitleStyle.Render("HOW LEON FEELS") + "\n" + v.feelings.Render(panelWidth-4))

	right := styles.PanelStyle.Width(panelWidth).Height(height - 4).Render(
		styles.PanelTitleStyle.Render("WHY") + "\n" + v.renderDrivers(panelWidth-4))

	return lipgloss.JoinHorizontal(lipgloss.Top, left, right)
}

// renderDrivers shows the readings the feelings are derived from.
func (v *FeelingsView) renderDrivers(width int) string {
	drivers := []string{"energy", "glycemia", "hydration", "bladder_fill", "bowel_fill", "body_temperature"}

	lines := make([]string, 0, len(drivers))
	for _, port := range drivers {
		key := v.subject + telemetry.PathSeparator + port
		data, ok := v.State.GetSignal(key)
		if !ok {
			continue
		}
		lines = append(lines, widgets.NewVitalSign(models.SignalRegistry[key], data).Render(width))
	}
	return strings.Join(lines, "\n")
}
