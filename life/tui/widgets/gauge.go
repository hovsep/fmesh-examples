package widgets

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/hovsep/fmesh-examples/life/tui/models"
	"github.com/hovsep/fmesh-examples/life/tui/styles"
)

type Gauge struct {
	Metadata models.SignalMetadata
	Signal   *models.SignalData
	Width    int
}

func NewGauge(metadata models.SignalMetadata, signal *models.SignalData, width int) *Gauge {
	return &Gauge{
		Metadata: metadata,
		Signal:   signal,
		Width:    width,
	}
}

func (g *Gauge) Render() string {
	if g.Signal == nil {
		return ""
	}

	value := g.Signal.Latest()
	health := g.Metadata.HealthStatus(value)

	percentage := 0.0
	if g.Metadata.MaxRange != g.Metadata.MinRange {
		percentage = (value - g.Metadata.MinRange) / (g.Metadata.MaxRange - g.Metadata.MinRange)
	}
	if percentage < 0 {
		percentage = 0
	}
	if percentage > 1 {
		percentage = 1
	}

	filledWidth := int(percentage * float64(g.Width))
	bar := strings.Repeat(styles.ProgressFullChar, filledWidth) +
		strings.Repeat(styles.ProgressEmptyChar, g.Width-filledWidth)

	var barStyle lipgloss.Style
	switch health {
	case models.HealthNormal:
		barStyle = styles.ValueNormalStyle
	case models.HealthWarning:
		barStyle = styles.ValueWarningStyle
	case models.HealthCritical:
		barStyle = styles.ValueCriticalStyle
	}

	var valueStr string
	if g.Metadata.DecimalPlaces == 0 {
		valueStr = fmt.Sprintf("%.0f", value)
	} else {
		format := fmt.Sprintf("%%.%df", g.Metadata.DecimalPlaces)
		valueStr = fmt.Sprintf(format, value)
	}

	label := styles.LabelStyle.Render(g.Metadata.Label)
	valueRendered := barStyle.Render(valueStr)
	unit := styles.UnitStyle.Render(g.Metadata.Unit)
	barRendered := barStyle.Render(bar)
	healthSymbol := health.String()

	return fmt.Sprintf("%s  %s%s %s %s",
		label,
		valueRendered,
		unit,
		barRendered,
		healthSymbol,
	)
}
