package widgets

import (
	"fmt"
	"slices"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/hovsep/fmesh-examples/life/common"
	"github.com/hovsep/fmesh-examples/life/telemetry"
	"github.com/hovsep/fmesh-examples/life/tuiv2/models"
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
// Single code points only: emoji built from zero-width joiner sequences (the
// exhaling face, for one) are measured as one cell and drawn as two by most
// terminals, which shears the label off the line next to them.
var feelingIcons = map[string]string{
	common.FeelingExhausted:      "🥵",
	common.FeelingBreathless:     "😧",
	common.FeelingHeadache:       "🤕",
	common.FeelingFeverish:       "🤒",
	common.FeelingThirsty:        "🥤",
	common.FeelingHungry:         "🍽",
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

// Render draws the felt sensations as labelled bars.
func (f *Feelings) Render(width int) string {
	felt := f.current()
	if len(felt) == 0 {
		return dimStyle.Render("nothing in particular")
	}

	// The longest label sets the column, so the bars line up.
	labelWidth := 0
	for _, item := range felt {
		labelWidth = max(labelWidth, lipgloss.Width(common.FeelingLabels[item.name]))
	}

	barWidth := max(width-labelWidth-14, 6)

	lines := make([]string, 0, len(felt))
	for _, item := range felt {
		lines = append(lines, fmt.Sprintf("%s %-*s %s %3.0f%%",
			icon(item.name),
			labelWidth, common.FeelingLabels[item.name],
			renderIntensityBar(item.intensity, barWidth),
			item.intensity*100,
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

// renderIntensityBar colours by how insistent the feeling is: a passing note in
// green, something that needs attention in red.
func renderIntensityBar(intensity float64, width int) string {
	filled := int(intensity * float64(width))
	filled = min(max(filled, 0), width)

	style := lipgloss.NewStyle().Foreground(lipgloss.Color("#5fd75f"))
	switch {
	case intensity >= 0.66:
		style = lipgloss.NewStyle().Foreground(lipgloss.Color("#ff5f5f"))
	case intensity >= 0.33:
		style = lipgloss.NewStyle().Foreground(lipgloss.Color("#ffd75f"))
	}

	return style.Render(strings.Repeat("█", filled)) +
		dimStyle.Render(strings.Repeat("░", width-filled))
}

var dimStyle = lipgloss.NewStyle().Faint(true)
