package widgets

import (
	"github.com/guptarohit/asciigraph"
	"github.com/hovsep/fmesh-examples/life/tuiv2/models"
	"github.com/hovsep/fmesh-examples/life/tuiv2/styles"
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
}

func NewLineChart(title, unit string, series ...ChartSeries) *LineChart {
	return &LineChart{Title: title, Unit: unit, Series: series, Window: defaultWindow}
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

	return title + "\n" + asciigraph.PlotMany(data, opts...)
}
