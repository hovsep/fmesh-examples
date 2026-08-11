package dash

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/hovsep/fmesh-examples/simulation/life/tui/styles"
)

// The standard widgets. Between them they cover what this dashboard actually
// shows; anything genuinely bespoke -- a waveform, a body diagram -- is a widget
// too, and is referenced from the same table rather than special-cased in the
// renderer.

// Text is a fixed block of lines. For headings, legends and explanations.
type Text struct {
	Title string
	Lines []string
}

func (w Text) Render(_ Source, width, height int) string {
	return panel(w.Title, strings.Join(w.Lines, "\n"), width, height)
}

// Stat is one reading, drawn large: the value, its unit, and its label.
//
// Coloured by the health band the catalog gives it, so a number outside the
// range it should be in says so without anybody writing a threshold here.
type Stat struct {
	Metric string
	Label  string // overrides the catalog's label when set
}

func (w Stat) Render(src Source, width, height int) string {
	display, ok := src.Display(w.Metric)
	label := w.Label
	if label == "" && ok {
		label = display.Label
	}

	value := src.Value(w.Metric)
	text := fmt.Sprintf("%.*f", display.Decimals, value)

	style := styles.ValueNormalStyle
	if ok && (value < display.HealthMin || value > display.HealthMax) {
		style = styles.ValueWarningStyle
	}

	body := style.Render(text) + styles.UnitStyle.Render(" "+display.Unit) +
		"\n" + styles.LabelStyle.Render(label)
	return panel("", body, width, height)
}

// Stats is a row of readings drawn compactly, one per line.
type Stats struct {
	Title   string
	Metrics []string
}

func (w Stats) Render(src Source, width, height int) string {
	var b strings.Builder
	for _, metric := range w.Metrics {
		display, _ := src.Display(metric)
		value := src.Value(metric)

		style := styles.ValueNormalStyle
		if value < display.HealthMin || value > display.HealthMax {
			style = styles.ValueWarningStyle
		}

		fmt.Fprintf(&b, "%-22s %s %s\n",
			styles.LabelStyle.Render(display.Label),
			style.Render(fmt.Sprintf("%*.*f", 7, display.Decimals, value)),
			styles.UnitStyle.Render(display.Unit))
	}
	return panel(w.Title, strings.TrimRight(b.String(), "\n"), width, height)
}

// Bars draws each metric as a horizontal bar between the bounds the catalog
// gives it, which is the quickest way to see that something is off its range
// without reading any numbers.
type Bars struct {
	Title   string
	Metrics []string
}

func (w Bars) Render(src Source, width, height int) string {
	barWidth := max(width-40, 10)

	var b strings.Builder
	for _, metric := range w.Metrics {
		display, _ := src.Display(metric)
		value := src.Value(metric)

		span := display.Max - display.Min
		filled := 0
		if span > 0 {
			filled = int(float64(barWidth) * clamp01((value-display.Min)/span))
		}

		style := styles.ValueNormalStyle
		if value < display.HealthMin || value > display.HealthMax {
			style = styles.ValueWarningStyle
		}

		fmt.Fprintf(&b, "%-18s %s %s\n",
			styles.LabelStyle.Render(trim(display.Label, 18)),
			style.Render(strings.Repeat("█", filled)+
				styles.UnitStyle.Render(strings.Repeat("░", barWidth-filled))),
			style.Render(fmt.Sprintf("%.*f%s", display.Decimals, value, display.Unit)))
	}
	return panel(w.Title, strings.TrimRight(b.String(), "\n"), width, height)
}

// Func is an escape hatch: a widget that draws itself however it likes.
//
// It exists so that a waveform or a diagram can sit in the same table as
// everything else rather than being a reason for the renderer to know about it.
// Reach for it when a drawing is genuinely bespoke, not when a standard widget
// would do with slightly different spacing.
type Func struct {
	Draw func(src Source, width, height int) string
}

func (w Func) Render(src Source, width, height int) string { return w.Draw(src, width, height) }

// panel frames content to an exact height so rows stack predictably.
func panel(title, body string, width, height int) string {
	if title != "" {
		body = styles.PanelTitleStyle.Render(title) + "\n" + body
	}

	lines := strings.Split(body, "\n")
	if len(lines) > height {
		lines = lines[:max(height, 0)]
	}
	for len(lines) < height {
		lines = append(lines, "")
	}
	return lipgloss.NewStyle().Width(width).Render(strings.Join(lines, "\n"))
}

func clamp01(v float64) float64 { return max(0, min(1, v)) }

func trim(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
