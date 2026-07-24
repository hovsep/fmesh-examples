package widgets

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/hovsep/fmesh-examples/life/tui/models"
	"github.com/hovsep/fmesh-examples/life/tui/styles"
)

// Fixed column widths keep everything after the value from jumping as the value's
// digit count changes (e.g. "-24" vs "169"). The label and unit are padded to a
// column, the value is right-aligned in a column sized to its range, and the
// sparkline fills whatever width is left -- so it is also as large as the panel
// allows, which makes the trend far easier to read than the old fixed 10 cells.
const (
	vitalLabelWidth = 14
	vitalUnitWidth  = 6
	vitalSparkMin   = 12
)

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

func (v *VitalSign) Render(width int) string {
	if v.Signal == nil {
		return ""
	}

	value := v.Signal.Latest()
	health := v.Metadata.HealthStatus(value)

	valueWidth := v.valueColumnWidth()
	valueStr := padLeft(v.format(value), valueWidth) // right-aligned in its column

	var valueStyle lipgloss.Style
	switch health {
	case models.HealthNormal:
		valueStyle = styles.ValueNormalStyle
	case models.HealthWarning:
		valueStyle = styles.ValueWarningStyle
	case models.HealthCritical:
		valueStyle = styles.ValueCriticalStyle
	}

	label := styles.LabelStyle.Render(padRight(v.Metadata.Label, vitalLabelWidth))
	valueRendered := valueStyle.Render(valueStr)
	unit := styles.UnitStyle.Render(padRight(v.Metadata.Unit, vitalUnitWidth))
	healthSymbol := health.String()

	// The sparkline takes all the width the fixed columns leave it.
	overhead := vitalLabelWidth + 1 + valueWidth + 1 + vitalUnitWidth + 1 + 1 + 1
	sparkWidth := max(width-overhead, vitalSparkMin)
	sparkline := v.renderSparkline(sparkWidth)

	return fmt.Sprintf("%s %s %s %s %s", label, valueRendered, unit, sparkline, healthSymbol)
}

// format renders the value with the metric's decimal places.
func (v *VitalSign) format(value float64) string {
	return fmt.Sprintf("%.*f", v.Metadata.DecimalPlaces, value)
}

// valueColumnWidth sizes the value column to the widest value the metric's range
// can produce, so the column never has to grow (and shove the sparkline) at
// runtime.
func (v *VitalSign) valueColumnWidth() int {
	w := lipgloss.Width(v.format(v.Metadata.MinRange))
	w = max(w, lipgloss.Width(v.format(v.Metadata.MaxRange)))
	return max(w, 3)
}

func (v *VitalSign) renderSparkline(count int) string {
	values := v.Signal.GetLast(count)
	if len(values) == 0 {
		return strings.Repeat(" ", count)
	}

	min, max := values[0], values[0]
	for _, val := range values {
		if val < min {
			min = val
		}
		if val > max {
			max = val
		}
	}

	if v.Metadata.MinRange < v.Metadata.MaxRange {
		min = v.Metadata.MinRange
		max = v.Metadata.MaxRange
	}

	chars := []rune{'▁', '▂', '▃', '▄', '▅', '▆', '▇', '█'}

	var sparkline strings.Builder
	for _, val := range values {
		normalized := 0.0
		if max != min {
			normalized = (val - min) / (max - min)
		}

		idx := int(normalized * float64(len(chars)-1))
		if idx < 0 {
			idx = 0
		}
		if idx >= len(chars) {
			idx = len(chars) - 1
		}

		sparkline.WriteRune(chars[idx])
	}

	return styles.UnitStyle.Render(sparkline.String())
}

// padRight and padLeft pad to a display width (lipgloss.Width), so units with
// multi-byte glyphs like "cmH₂O" and "°C" still line up.
func padRight(s string, w int) string {
	if pad := w - lipgloss.Width(s); pad > 0 {
		return s + strings.Repeat(" ", pad)
	}
	return s
}

func padLeft(s string, w int) string {
	if pad := w - lipgloss.Width(s); pad > 0 {
		return strings.Repeat(" ", pad) + s
	}
	return s
}
