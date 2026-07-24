package widgets

import (
	"github.com/guptarohit/asciigraph"
	"github.com/hovsep/fmesh-examples/life/tui/models"
	"github.com/hovsep/fmesh-examples/life/tui/styles"
)

// defaultWindow is how many recent samples a chart considers. asciigraph
// resamples this down to the plot width, so a larger window shows more history.
const defaultWindow = 300

// ChartSeries is a single line drawn on a LineChart.
type ChartSeries struct {
	Signal *models.SignalData
	Color  asciigraph.AnsiColor
	Legend string
}

// LineChart renders one or more time series as an ASCII line diagram, with a
// styled title above the plot.
type LineChart struct {
	Title  string
	Unit   string
	Series []ChartSeries
	Window int

	// When fixedY is set the plot uses [lowerBound, upperBound] for its Y axis
	// instead of auto-scaling to the current window. A steady axis is what makes
	// a waveform like the ECG readable: the baseline and peaks stay put instead
	// of the whole plot rescaling every frame.
	fixedY                 bool
	lowerBound, upperBound float64
}

func NewLineChart(title, unit string, series ...ChartSeries) *LineChart {
	return &LineChart{Title: title, Unit: unit, Series: series, Window: defaultWindow}
}

// WithYBounds fixes the Y axis to [lower, upper].
func (c *LineChart) WithYBounds(lower, upper float64) *LineChart {
	c.fixedY = true
	c.lowerBound = lower
	c.upperBound = upper
	return c
}

// WithWindow overrides how many recent samples the chart considers.
func (c *LineChart) WithWindow(window int) *LineChart {
	c.Window = window
	return c
}

// Render draws the chart within the given width and height (in cells). height
// is the total budget including the title line.
func (c *LineChart) Render(width, height int) string {
	title := styles.PanelTitleStyle.Render(c.Title)
	if c.Unit != "" {
		title += " " + styles.UnitStyle.Render("("+c.Unit+")")
	}

	plotWidth := max(width-12, 10) // leave room for the y-axis labels

	var (
		data    [][]float64
		colors  []asciigraph.AnsiColor
		legends []string
	)
	for _, s := range c.Series {
		if s.Signal == nil {
			continue
		}
		vals := s.Signal.GetLast(c.Window)
		if len(vals) == 0 {
			continue
		}
		data = append(data, vals)
		colors = append(colors, s.Color)
		legends = append(legends, s.Legend)
	}

	if len(data) == 0 {
		return title + "\n" + styles.UnitStyle.Render("  collecting data…")
	}

	multiSeries := len(data) > 1

	// Fit within the height budget: title (1) + plot's own top row (1), plus the
	// blank+legend block (2) that asciigraph adds for multi-series charts.
	reserved := 2
	if multiSeries {
		reserved += 2
	}
	plotHeight := max(height-reserved, 3)

	opts := []asciigraph.Option{
		asciigraph.Width(plotWidth),
		asciigraph.Height(plotHeight),
		asciigraph.SeriesColors(colors...),
		asciigraph.Precision(0),
	}
	if multiSeries {
		opts = append(opts, asciigraph.SeriesLegends(legends...))
	}
	if c.fixedY {
		opts = append(opts, asciigraph.LowerBound(c.lowerBound), asciigraph.UpperBound(c.upperBound))
	}

	return title + "\n" + asciigraph.PlotMany(data, opts...)
}
