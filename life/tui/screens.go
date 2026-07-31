package tui

import (
	"github.com/hovsep/fmesh-examples/life/telemetry"
	"github.com/hovsep/fmesh-examples/life/tui/dash"
	"github.com/hovsep/fmesh-examples/life/tui/views"
)

// The dashboard, as a table.
//
// A screen is rows of panels; a row's weight is its share of the height. Adding
// a screen is an entry here, and adding a reading to an existing one is a metric
// name in a list -- neither needs a file, a renderer, or a case in a switch.
//
// The bespoke drawings are widgets like any other. A waveform and a body diagram
// genuinely need code, and wrapping them as dash.Func puts them in this table
// beside the standard panels rather than making the renderer know they exist.
// That is the difference between a dashboard that is configured and one that
// merely has configuration in it.
func screens(state *legacyViews) []dash.Screen {
	return []dash.Screen{
		{
			View: telemetry.ViewOverview, Title: "Overview",
			Rows: []dash.Row{{Widgets: []dash.Widget{dash.Func{
				Draw: func(_ dash.Source, w, h int) string { return state.overview.Render(w, h) },
			}}}},
		},
		{
			View: telemetry.ViewCardiovascular, Title: "Cardiovascular",
			Rows: []dash.Row{{Widgets: []dash.Widget{dash.Func{
				Draw: func(_ dash.Source, w, h int) string { return state.cardiac.Render(w, h) },
			}}}},
		},
		{
			View: telemetry.ViewRespiratory, Title: "Respiratory",
			Rows: []dash.Row{{Widgets: []dash.Widget{dash.Func{
				Draw: func(_ dash.Source, w, h int) string {
					return state.respiratory.Render(w, h, state.lungsSplit())
				},
			}}}},
		},
		{
			// Built entirely from the table, with no view behind it: three bands
			// of standard panels naming metrics from the catalog. This is what
			// the others become as their drawings are pulled apart into widgets.
			View: telemetry.ViewNervous, Title: "Nervous",
			Rows: []dash.Row{
				{Weight: 2, Widgets: []dash.Widget{
					dash.Stat{Metric: "brain_activity"},
					dash.Stat{Metric: "brain_activity_trend"},
					dash.Stat{Metric: "is_alive", Label: "Alive"},
				}},
				{Weight: 3, Widgets: []dash.Widget{dash.Bars{
					Title:   "AUTONOMIC",
					Metrics: []string{"heart_rate", "respiratory_rate", "mean_arterial_pressure"},
				}}},
				{Weight: 3, Widgets: []dash.Widget{dash.Text{
					Title: "ABOUT",
					Lines: []string{
						"The brain publishes one number: how hard it is driving the body.",
						"It never addresses an organ. The autonomic system downstream",
						"decides what a given drive means for a heart or a gut, which is",
						"both how a body works and what stops the brain becoming a",
						"switchboard.",
					},
				}}},
			},
		},
		{
			View: telemetry.ViewMetabolic, Title: "Metabolic",
			Rows: []dash.Row{
				{Weight: 2, Widgets: []dash.Widget{
					dash.Stat{Metric: "glycemia"},
					dash.Stat{Metric: "energy"},
					dash.Stat{Metric: "body_temperature"},
				}},
				{Weight: 3, Widgets: []dash.Widget{dash.Bars{
					Title:   "RESERVOIRS",
					Metrics: []string{"hydration", "stomach_fill", "bladder_fill", "bowel_fill"},
				}}},
				{Weight: 2, Widgets: []dash.Widget{dash.Stats{
					Title:   "EXCHANGE",
					Metrics: []string{"sweat_rate", "muscle_fatigue"},
				}}},
			},
		},
		{
			View: telemetry.ViewAffect, Title: "Feelings",
			Rows: []dash.Row{{Widgets: []dash.Widget{dash.Func{
				Draw: func(_ dash.Source, w, h int) string { return state.feelings.Render(w, h) },
			}}}},
		},
		{
			View: telemetry.ViewBody, Title: "Body",
			Rows: []dash.Row{{Widgets: []dash.Widget{dash.Func{
				Draw: func(_ dash.Source, w, h int) string { return state.body.Render(w, h) },
			}}}},
		},
	}
}

// legacyViews holds the drawings that still have a view behind them.
//
// It shrinks as each is taken apart into widgets; when it is empty this file is
// the whole dashboard and views/ is gone.
type legacyViews struct {
	overview    *views.OverviewView
	cardiac     *views.CardiacView
	respiratory *views.RespiratoryView
	feelings    *views.FeelingsView
	body        *views.BodyView
	lungsSplit  func() bool
}
