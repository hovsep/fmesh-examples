package views

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/hovsep/fmesh-examples/simulation/life/telemetry"
	"github.com/hovsep/fmesh-examples/simulation/life/tui/models"
	"github.com/hovsep/fmesh-examples/simulation/life/tui/styles"
	"github.com/hovsep/fmesh-examples/simulation/life/tui/widgets"
)

// BodyView shows each organ's damage as a bar and a status word, so a smoking or
// aging or death scenario can be watched organ by organ.
type BodyView struct {
	State   *models.AppState
	subject string
}

func NewBodyView(state *models.AppState, subject string) *BodyView {
	return &BodyView{State: state, subject: subject}
}

func (v *BodyView) Render(width, height int) string {
	// The same grid every other bar sits on (see widgets/layout.go), with the
	// status word as the trailing marker.
	rows := rowWidth(panelWidth(width))
	barWidth := widgets.BarWidth(rows)

	lines := make([]string, 0, len(telemetry.DamagedOrgans))
	for _, organ := range telemetry.DamagedOrgans {
		key := v.subject + telemetry.PathSeparator + organ.Port + "_damage"
		damage := v.State.GetLatestValue(key)

		lines = append(lines, fmt.Sprintf("%s %s %s %s %s",
			styles.LabelStyle.Render(widgets.Label(organ.Label)),
			damageStyle(damage).Render(widgets.Value(fmt.Sprintf("%.0f", damage*100))),
			styles.UnitStyle.Render(widgets.Unit("%")),
			widgets.Bar(damage, barWidth, damageStyle(damage)),
			widgets.Marker(organStatus(damage)),
		))
	}

	body := styles.PanelTitleStyle.Render("ORGAN DAMAGE") + "\n" +
		styles.LabelStyle.Render("healthy → damaged → failing → failed") + "\n\n" +
		strings.Join(lines, "\n")

	return styles.PanelStyle.Width(panelWidth(width)).Render(body)
}

// organStatus turns a damage level into the word the Body view shows.
func organStatus(damage float64) string {
	switch {
	case damage >= 1.0:
		return failedStyle.Render("FAILED ")
	case damage >= 0.66:
		return alarmStyle.Render("failing")
	case damage >= 0.25:
		return warnStyle.Render("damaged")
	default:
		return okStyle.Render("healthy")
	}
}

// damageStyle runs green→yellow→red as damage rises: the opposite polarity of a
// health gauge, because here a fuller bar is worse.
func damageStyle(damage float64) lipgloss.Style {
	switch {
	case damage >= 0.66:
		return alarmStyle
	case damage >= 0.25:
		return warnStyle
	default:
		return okStyle
	}
}

var (
	okStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("#5fd75f"))
	warnStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#ffd75f"))
	alarmStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#ff5f5f"))
	failedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#ffffff")).Background(lipgloss.Color("#d70000")).Bold(true)
)
