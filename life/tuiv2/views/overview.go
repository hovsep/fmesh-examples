package views

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/hovsep/fmesh-examples/life/telemetry"
	"github.com/hovsep/fmesh-examples/life/tuiv2/models"
	"github.com/hovsep/fmesh-examples/life/tuiv2/styles"
	"github.com/hovsep/fmesh-examples/life/tuiv2/widgets"
)

type OverviewView struct {
	State    *models.AppState
	feelings *widgets.Feelings
}

func NewOverviewView(state *models.AppState) *OverviewView {
	return &OverviewView{
		State:    state,
		feelings: widgets.NewFeelings(state, telemetry.DefaultSubject),
	}
}

func (v *OverviewView) Render(width, height int) string {
	if width < 80 {
		return "Terminal too narrow. Please resize to at least 80 columns wide."
	}
	if height < 24 {
		return "Terminal too short. Please resize to at least 24 rows tall."
	}

	panelWidth := (width - 6) / 2
	panelHeight := (height - 10) / 2

	if panelWidth < 20 {
		panelWidth = 20
	}
	if panelHeight < 8 {
		panelHeight = 8
	}

	cardiovascular := v.renderCardiovascular(panelWidth, panelHeight)
	respiratory := v.renderRespiratory(panelWidth, panelHeight)
	nervous := v.renderNervous(panelWidth, panelHeight)
	gasExchange := v.renderGasExchange(panelWidth, panelHeight)

	topRow := lipgloss.JoinHorizontal(lipgloss.Top, cardiovascular, respiratory)
	bottomRow := lipgloss.JoinHorizontal(lipgloss.Top, nervous, gasExchange)
	return lipgloss.JoinVertical(lipgloss.Left, topRow, bottomRow)
}

// renderInspiredGas builds the composition panel from live telemetry.
//
// It used to be a hardcoded sea-level mixture, because composite gas signals
// carried their values as scalars that the wire protocol dropped. Now that
// scalars are published, the panel shows what Leon is actually breathing.
func (v *OverviewView) renderInspiredGas() *widgets.GasComposition {
	gas := widgets.NewGasComposition("", 20)
	for _, part := range []struct {
		scalar string
		label  string
		color  lipgloss.Color
	}{
		{"composition:nitrogen", "N₂", lipgloss.Color("#5f87d7")},
		{"composition:oxygen", "O₂", lipgloss.Color("#5fd75f")},
		{"composition:argon", "Ar", lipgloss.Color("#af87d7")},
		{"composition:pollution", "Poll", lipgloss.Color("#d75f5f")},
	} {
		key := telemetry.DefaultSubject + telemetry.PathSeparator + "inspired_gas" +
			telemetry.ScalarSeparator + part.scalar
		gas.AddComponent(part.label, v.State.GetLatestValue(key), part.color)
	}
	return gas
}

func (v *OverviewView) renderCardiovascular(width, height int) string {
	var content strings.Builder

	title := styles.PanelTitleStyle.Render("CARDIOVASCULAR")
	content.WriteString(title + "\n")
	content.WriteString(strings.Repeat("━", width-2) + "\n")

	if signal, exists := v.State.GetSignal("human-Leon::heart_rate"); exists {
		metadata := models.SignalRegistry["human-Leon::heart_rate"]
		vs := widgets.NewVitalSign(metadata, signal)
		content.WriteString(vs.Render(width-4) + "\n")
	}

	content.WriteString("\n")

	if signal, exists := v.State.GetSignal("human-Leon::blood_o2_level"); exists {
		metadata := models.SignalRegistry["human-Leon::blood_o2_level"]
		gauge := widgets.NewGauge(metadata, signal, 12)
		content.WriteString(gauge.Render() + "\n")
	}

	content.WriteString("\n")

	if signal, exists := v.State.GetSignal("human-Leon::blood_co2_level"); exists {
		metadata := models.SignalRegistry["human-Leon::blood_co2_level"]
		gauge := widgets.NewGauge(metadata, signal, 12)
		content.WriteString(gauge.Render() + "\n")
	}

	return styles.PanelStyle.
		Width(width).
		Height(height).
		Render(content.String())
}

func (v *OverviewView) renderRespiratory(width, height int) string {
	var content strings.Builder

	title := styles.PanelTitleStyle.Render("RESPIRATORY")
	content.WriteString(title + "\n")
	content.WriteString(strings.Repeat("━", width-2) + "\n")

	if signal, exists := v.State.GetSignal("human-Leon::respiratory_rate"); exists {
		metadata := models.SignalRegistry["human-Leon::respiratory_rate"]
		vs := widgets.NewVitalSign(metadata, signal)
		content.WriteString(vs.Render(width-4) + "\n")
	}

	content.WriteString("\n")

	if signal, exists := v.State.GetSignal("human-Leon::pleural_pressure"); exists {
		metadata := models.SignalRegistry["human-Leon::pleural_pressure"]
		vs := widgets.NewVitalSign(metadata, signal)
		content.WriteString(vs.Render(width-4) + "\n")
	}

	content.WriteString("\n")

	if signal, exists := v.State.GetSignal("human-Leon::lung_left_volume"); exists {
		metadata := models.SignalRegistry["human-Leon::lung_left_volume"]
		vs := widgets.NewVitalSign(metadata, signal)
		content.WriteString(vs.Render(width-4) + "\n")
	}

	content.WriteString("\n")

	if signal, exists := v.State.GetSignal("human-Leon::lung_right_volume"); exists {
		metadata := models.SignalRegistry["human-Leon::lung_right_volume"]
		vs := widgets.NewVitalSign(metadata, signal)
		content.WriteString(vs.Render(width-4) + "\n")
	}

	return styles.PanelStyle.
		Width(width).
		Height(height).
		Render(content.String())
}

func (v *OverviewView) renderNervous(width, height int) string {
	var content strings.Builder

	title := styles.PanelTitleStyle.Render("NERVOUS & MOOD")
	content.WriteString(title + "\n")
	content.WriteString(strings.Repeat("━", width-2) + "\n")

	if signal, exists := v.State.GetSignal("human-Leon::brain_activity"); exists {
		metadata := models.SignalRegistry["human-Leon::brain_activity"]
		vs := widgets.NewVitalSign(metadata, signal)
		content.WriteString(vs.Render(width-4) + "\n")
	}

	content.WriteString("\n")

	if signal, exists := v.State.GetSignal("human-Leon::brain_activity_trend"); exists {
		metadata := models.SignalRegistry["human-Leon::brain_activity_trend"]
		vs := widgets.NewVitalSign(metadata, signal)
		content.WriteString(vs.Render(width-4) + "\n")
	}

	content.WriteString("\n")

	if signal, exists := v.State.GetSignal("human-Leon::body_temperature"); exists {
		metadata := models.SignalRegistry["human-Leon::body_temperature"]
		vs := widgets.NewVitalSign(metadata, signal)
		content.WriteString(vs.Render(width-4) + "\n")
	}

	// What the body makes of all this. Numbers say what is happening; this says
	// whether it is a problem.
	content.WriteString("\n" + styles.LabelStyle.Render("Feeling:") + "\n")
	content.WriteString(v.feelings.Render(width-4) + "\n")

	return styles.PanelStyle.
		Width(width).
		Height(height).
		Render(content.String())
}

func (v *OverviewView) renderGasExchange(width, height int) string {
	var content strings.Builder

	title := styles.PanelTitleStyle.Render("GAS EXCHANGE")
	content.WriteString(title + "\n")
	content.WriteString(strings.Repeat("━", width-2) + "\n")

	if signal, exists := v.State.GetSignal("human-Leon::lung_left_flow"); exists {
		metadata := models.SignalRegistry["human-Leon::lung_left_flow"]
		vs := widgets.NewVitalSign(metadata, signal)
		content.WriteString(vs.Render(width-4) + "\n")
	}

	content.WriteString("\n")

	if signal, exists := v.State.GetSignal("human-Leon::lung_right_flow"); exists {
		metadata := models.SignalRegistry["human-Leon::lung_right_flow"]
		vs := widgets.NewVitalSign(metadata, signal)
		content.WriteString(vs.Render(width-4) + "\n")
	}

	content.WriteString("\n")

	content.WriteString(styles.LabelStyle.Render("Inspired Gas:") + "\n")
	content.WriteString(v.renderInspiredGas().Render() + "\n")

	return styles.PanelStyle.
		Width(width).
		Height(height).
		Render(content.String())
}
