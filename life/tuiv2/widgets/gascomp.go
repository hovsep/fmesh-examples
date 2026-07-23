package widgets

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/hovsep/fmesh-examples/life/tuiv2/styles"
)

type GasComponent struct {
	Name       string
	Percentage float64
	Color      lipgloss.Color
}

type GasComposition struct {
	Title      string
	Components []GasComponent
	Width      int
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

func (g *GasComposition) Render() string {
	if len(g.Components) == 0 {
		return ""
	}

	var output strings.Builder

	if g.Title != "" {
		output.WriteString(styles.LabelStyle.Render(g.Title))
		output.WriteString(":\n")
	}

	for _, comp := range g.Components {
		barWidth := int(comp.Percentage / 100.0 * float64(g.Width))
		if barWidth < 0 {
			barWidth = 0
		}
		if barWidth > g.Width {
			barWidth = g.Width
		}

		bar := strings.Repeat(styles.ProgressFullChar, barWidth) +
			strings.Repeat(styles.ProgressEmptyChar, g.Width-barWidth)

		barStyle := lipgloss.NewStyle().Foreground(comp.Color)
		name := styles.LabelStyle.Width(4).Render(comp.Name)
		percentage := styles.UnitStyle.Width(6).Render(fmt.Sprintf("%5.1f%%", comp.Percentage))
		barRendered := barStyle.Render(bar)

		output.WriteString(fmt.Sprintf(" %s %s %s\n", name, percentage, barRendered))
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
