package widgets

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/hovsep/fmesh-examples/life/tui/models"
	"github.com/hovsep/fmesh-examples/life/tui/styles"
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

	var valueStr string
	if v.Metadata.DecimalPlaces == 0 {
		valueStr = fmt.Sprintf("%.0f", value)
	} else {
		format := fmt.Sprintf("%%.%df", v.Metadata.DecimalPlaces)
		valueStr = fmt.Sprintf(format, value)
	}

	var valueStyle lipgloss.Style
	switch health {
	case models.HealthNormal:
		valueStyle = styles.ValueNormalStyle
	case models.HealthWarning:
		valueStyle = styles.ValueWarningStyle
	case models.HealthCritical:
		valueStyle = styles.ValueCriticalStyle
	}

	label := styles.LabelStyle.Render(v.Metadata.Label)
	valueRendered := valueStyle.Render(valueStr)
	unit := styles.UnitStyle.Render(v.Metadata.Unit)
	healthSymbol := health.String()
	sparkline := v.renderSparkline(10)

	return fmt.Sprintf("%s  %s %s %s %s",
		label,
		valueRendered,
		unit,
		sparkline,
		healthSymbol,
	)
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
