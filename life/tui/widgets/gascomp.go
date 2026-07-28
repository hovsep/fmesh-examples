package widgets

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/hovsep/fmesh-examples/life/tui/styles"
)

type GasComponent struct {
	Name       string
	Percentage float64
	Color      lipgloss.Color
}

type GasComposition struct {
	Title      string
	Components []GasComponent

	// Width is the width of a whole row, not of the bar: the bar takes what the
	// shared columns leave, exactly as it does on every other row.
	Width int
}

func NewGasComposition(title string, width int) *GasComposition {
	return &GasComposition{
		Title:      title,
		Components: []GasComponent{},
		Width:      width,
	}
}

func (g *GasComposition) AddComponent(name string, percentage float64, color lipgloss.Color) {
	g.Components = append(g.Components, GasComponent{
		Name:       name,
		Percentage: percentage,
		Color:      color,
	})
}

// Render draws each component on the shared grid (see layout.go), so the gas
// mixture lines up with the readings in the panels around it.
func (g *GasComposition) Render() string {
	if len(g.Components) == 0 {
		return ""
	}

	var output strings.Builder

	if g.Title != "" {
		output.WriteString(styles.LabelStyle.Render(g.Title))
		output.WriteString(":\n")
	}

	barWidth := BarWidth(g.Width)
	for _, comp := range g.Components {
		barStyle := lipgloss.NewStyle().Foreground(comp.Color)

		output.WriteString(fmt.Sprintf("%s %s %s %s\n",
			styles.LabelStyle.Render(Label(comp.Name)),
			barStyle.Render(Value(fmt.Sprintf("%.1f", comp.Percentage))),
			styles.UnitStyle.Render(Unit("%")),
			Bar(comp.Percentage/100.0, barWidth, barStyle),
		))
	}

	return strings.TrimSuffix(output.String(), "\n")
}

func PredefinedAtmosphericGas(n2, o2, ar, co2 float64) *GasComposition {
	gc := NewGasComposition("Atmospheric Gas", 20)
	gc.AddComponent("N₂", n2, lipgloss.Color("45"))    // Cyan
	gc.AddComponent("O₂", o2, lipgloss.Color("42"))    // Green
	gc.AddComponent("Ar", ar, lipgloss.Color("240"))   // Gray
	gc.AddComponent("CO₂", co2, lipgloss.Color("214")) // Orange/Yellow
	return gc
}
