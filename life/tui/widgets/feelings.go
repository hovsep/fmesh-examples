package widgets

import (
	"fmt"
	"slices"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/hovsep/fmesh-examples/life/common"
	"github.com/hovsep/fmesh-examples/life/telemetry"
	"github.com/hovsep/fmesh-examples/life/tui/models"
)

// feltThreshold is the intensity below which a feeling is not worth mentioning.
// Almost every feeling is very slightly present almost all the time; listing
// them all would bury the one that actually matters.
const feltThreshold = 0.05

// maxFeelingsShown keeps the panel to what a glance can take in.
const maxFeelingsShown = 6

// feelingIcons give each feeling a face, so the panel reads at a glance rather
// than needing to be parsed.
//
// Keep to single code points, optionally with the emoji-presentation selector:
// sequences built from zero-width joiners (the exhaling face, for one) are
// measured as one cell and drawn as two by most terminals. The selector on the
// plate is what makes it measure the two cells it is drawn in.
var feelingIcons = map[string]string{
	common.FeelingExhausted:      "🥵",
	common.FeelingBreathless:     "😧",
	common.FeelingHeadache:       "🤕",
	common.FeelingFeverish:       "🤒",
	common.FeelingThirsty:        "🥤",
	common.FeelingHungry:         "🍽️",
	common.FeelingNeedToUrinate:  "🚻",
	common.FeelingNeedToDefecate: "🚽",
	common.FeelingAnxious:        "😰",
	common.FeelingTired:          "😴",
	common.FeelingHappy:          "😀",
	common.FeelingContent:        "🙂",
}

// Feelings renders what the body currently feels, strongest first.
type Feelings struct {
	state   *models.AppState
	subject string
}

func NewFeelings(state *models.AppState, subject string) *Feelings {
	return &Feelings{state: state, subject: subject}
}

type feeling struct {
	name      string
	intensity float64
}

// Render draws the felt sensations as labelled bars, on the same grid as every
// other bar in the dashboard (see layout.go): the icon and name share the label
// column, so a feeling's bar starts where a vital sign's sparkline does.
func (f *Feelings) Render(width int) string {
	felt := f.current()
	if len(felt) == 0 {
		return dimStyle.Render("nothing in particular")
	}

	barWidth := BarWidth(width)

	// The icon is carved out of the label column rather than sitting outside it,
	// or every feeling would be shifted against every reading.
	nameWidth := LabelWidth - IconWidth - columnGap

	lines := make([]string, 0, len(felt))
	for _, item := range felt {
		name := padRight(truncate(common.FeelingLabels[item.name], nameWidth), nameWidth)

		lines = append(lines, fmt.Sprintf("%s %s %s %s %s",
			Icon(icon(item.name)),
			name,
			intensityStyle(item.intensity).Render(Value(fmt.Sprintf("%.0f", item.intensity*100))),
			dimStyle.Render(Unit("%")),
			Bar(item.intensity, barWidth, intensityStyle(item.intensity)),
		))
	}
	return strings.Join(lines, "\n")
}

// current returns the feelings worth showing, strongest first.
func (f *Feelings) current() []feeling {
	var felt []feeling
	for _, name := range common.Feelings {
		key := f.subject + telemetry.PathSeparator + "feelings" + telemetry.ScalarSeparator + name
		if intensity := f.state.GetLatestValue(key); intensity >= feltThreshold {
			felt = append(felt, feeling{name: name, intensity: intensity})
		}
	}

	// Strongest first. Ties keep common.Feelings order, which is severity, so
	// the list does not reshuffle between frames when several are equally weak.
	slices.SortStableFunc(felt, func(a, b feeling) int {
		switch {
		case a.intensity > b.intensity:
			return -1
		case a.intensity < b.intensity:
			return 1
		default:
			return 0
		}
	})

	return felt[:min(len(felt), maxFeelingsShown)]
}

func icon(name string) string {
	if i, ok := feelingIcons[name]; ok {
		return i
	}
	return " "
}

// intensityStyle colours by how insistent the feeling is: a passing note in
// green, something that needs attention in red.
func intensityStyle(intensity float64) lipgloss.Style {
	switch {
	case intensity >= 0.66:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#ff5f5f"))
	case intensity >= 0.33:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#ffd75f"))
	default:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#5fd75f"))
	}
}

var dimStyle = lipgloss.NewStyle().Faint(true)
