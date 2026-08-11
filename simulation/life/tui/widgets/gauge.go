package widgets

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
	"github.com/hovsep/fmesh-examples/simulation/life/tui/models"
	"github.com/hovsep/fmesh-examples/simulation/life/tui/styles"
)

// Gauge shows where a reading sits in its range, as a filled bar.
//
// It is a vital sign with the trend swapped for a level, and sits on the same
// grid (see layout.go), so a gauge and a sparkline in one panel line up.
type Gauge struct {
	Metadata models.SignalMetadata
	Signal   *models.SignalData
}

func NewGauge(metadata models.SignalMetadata, signal *models.SignalData) *Gauge {
	return &Gauge{
		Metadata: metadata,
		Signal:   signal,
	}
}

// Render draws one row of width display cells.
func (g *Gauge) Render(width int) string {
	if g.Signal == nil {
		return ""
	}

	value := g.Signal.Latest()
	health := g.Metadata.HealthStatus(value)

	fraction := 0.0
	if g.Metadata.MaxRange != g.Metadata.MinRange {
		fraction = (value - g.Metadata.MinRange) / (g.Metadata.MaxRange - g.Metadata.MinRange)
	}

	var barStyle lipgloss.Style
	switch health {
	case models.HealthNormal:
		barStyle = styles.ValueNormalStyle
	case models.HealthWarning:
		barStyle = styles.ValueWarningStyle
	case models.HealthCritical:
		barStyle = styles.ValueCriticalStyle
	}

	return fmt.Sprintf("%s %s %s %s %s",
		styles.LabelStyle.Render(Label(g.Metadata.Label)),
		barStyle.Render(Value(fmt.Sprintf("%.*f", g.Metadata.DecimalPlaces, value))),
		styles.UnitStyle.Render(Unit(g.Metadata.Unit)),
		Bar(fraction, BarWidth(width), barStyle),
		Marker(health.String()),
	)
}
