package views

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/hovsep/fmesh-examples/life/telemetry"
	"github.com/hovsep/fmesh-examples/life/tui/models"
	"github.com/hovsep/fmesh-examples/life/tui/styles"
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
	// The longest organ label sets the column so the bars line up.
	labelWidth := 0
	for _, organ := range telemetry.DamagedOrgans {
		labelWidth = max(labelWidth, lipgloss.Width(organ.Label))
	}
	barWidth := max(width-labelWidth-28, 10)

	lines := make([]string, 0, len(telemetry.DamagedOrgans))
	for _, organ := range telemetry.DamagedOrgans {
		key := v.subject + telemetry.PathSeparator + organ.Port + "_damage"
		damage := v.State.GetLatestValue(key)
		lines = append(lines, fmt.Sprintf("%-*s %s %3.0f%%  %s",
			labelWidth, organ.Label,
			renderDamageBar(damage, barWidth),
			damage*100,
			organStatus(damage),
		))
	}

	body := styles.PanelTitleStyle.Render("ORGAN DAMAGE") + "\n" +
		styles.LabelStyle.Render("healthy → damaged → failing → failed") + "\n\n" +
		strings.Join(lines, "\n")

	return styles.PanelStyle.Width(width - 4).Render(body)
}

// organStatus turns a damage level into the word the Body view shows.
func organStatus(damage float64) string {
	switch {
	case damage >= 1.0:
		return failedStyle.Render("FAILED")
	case damage >= 0.66:
		return alarmStyle.Render("failing")
	case damage >= 0.25:
		return warnStyle.Render("damaged")
	default:
		return okStyle.Render("healthy")
	}
}

// renderDamageBar fills green→yellow→red as damage rises: the opposite polarity
// of a health gauge, because here more filled is worse.
func renderDamageBar(damage float64, width int) string {
	filled := min(max(int(damage*float64(width)), 0), width)

	style := okStyle
	switch {
	case damage >= 0.66:
		style = alarmStyle
	case damage >= 0.25:
		style = warnStyle
	}
	return style.Render(strings.Repeat("█", filled)) +
		styles.LabelStyle.Render(strings.Repeat("░", width-filled))
}

var (
	okStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("#5fd75f"))
	warnStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#ffd75f"))
	alarmStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#ff5f5f"))
	failedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#ffffff")).Background(lipgloss.Color("#d70000")).Bold(true)
)
