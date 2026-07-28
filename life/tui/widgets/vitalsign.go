package widgets

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/hovsep/fmesh-examples/life/tui/models"
	"github.com/hovsep/fmesh-examples/life/tui/styles"
)

// vitalSparkChars draw the trend, lightest to heaviest.
var vitalSparkChars = []rune{'▁', '▂', '▃', '▄', '▅', '▆', '▇', '█'}

type VitalSign struct {
	Metadata models.SignalMetadata
	Signal   *models.SignalData
}

func NewVitalSign(metadata models.SignalMetadata, signal *models.SignalData) *VitalSign {
	return &VitalSign{
		Metadata: metadata,
		Signal:   signal,
	}
}

// Render draws one row of the shared grid (see layout.go): the metric, its
// current reading, and its recent trend, ending in a health mark.
func (v *VitalSign) Render(width int) string {
	if v.Signal == nil {
		return ""
	}

	value := v.Signal.Latest()
	health := v.Metadata.HealthStatus(value)

	var valueStyle lipgloss.Style
	switch health {
	case models.HealthNormal:
		valueStyle = styles.ValueNormalStyle
	case models.HealthWarning:
		valueStyle = styles.ValueWarningStyle
	case models.HealthCritical:
		valueStyle = styles.ValueCriticalStyle
	}

	return fmt.Sprintf("%s %s %s %s %s",
		styles.LabelStyle.Render(Label(v.Metadata.Label)),
		valueStyle.Render(Value(v.format(value))),
		styles.UnitStyle.Render(Unit(v.Metadata.Unit)),
		v.renderSparkline(BarWidth(width)),
		Marker(health.String()),
	)
}

// format renders the value with the metric's decimal places.
func (v *VitalSign) format(value float64) string {
	return fmt.Sprintf("%.*f", v.Metadata.DecimalPlaces, value)
}

// renderSparkline draws the last count samples, scaled to the metric's range so
// the trend reads against what the value can be rather than against itself.
func (v *VitalSign) renderSparkline(count int) string {
	values := v.Signal.GetLast(count)
	if len(values) == 0 {
		return strings.Repeat(" ", count)
	}

	low, high := values[0], values[0]
	for _, val := range values {
		low = min(low, val)
		high = max(high, val)
	}
	if v.Metadata.MinRange < v.Metadata.MaxRange {
		low, high = v.Metadata.MinRange, v.Metadata.MaxRange
	}

	var sparkline strings.Builder
	for _, val := range values {
		normalized := 0.0
		if high != low {
			normalized = (val - low) / (high - low)
		}

		idx := min(max(int(normalized*float64(len(vitalSparkChars)-1)), 0), len(vitalSparkChars)-1)
		sparkline.WriteRune(vitalSparkChars[idx])
	}

	// A history shorter than the column pads out rather than stretching, so the
	// row still ends where every other row ends while it is filling up.
	return styles.UnitStyle.Render(sparkline.String()) +
		strings.Repeat(" ", max(count-len(values), 0))
}
