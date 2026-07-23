package main

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/hovsep/fmesh-examples/life/tuiv2/models"
	"github.com/hovsep/fmesh-examples/life/tuiv2/protocol"
	"github.com/hovsep/fmesh-examples/life/tuiv2/styles"
	"github.com/hovsep/fmesh-examples/life/tuiv2/views"
)

// Render cadence bounds. The render interval controls how often the TUI
// repaints from the latest state; it is fully independent of how fast the
// simulation runs or how fast updates arrive over the socket.
const (
	defaultRenderInterval = 100 * time.Millisecond // 10 FPS
	minRenderInterval     = 16 * time.Millisecond  // ~60 FPS
	maxRenderInterval     = time.Second            // 1 FPS
)

// Model represents the Bubble Tea application model
type Model struct {
	state           *models.AppState
	reader          *protocol.Reader
	overviewView    *views.OverviewView
	respiratoryView *views.RespiratoryView
	width           int
	height          int
	renderInterval  time.Duration
}

// NewModel creates a new application model
func NewModel(socketPath string) (*Model, error) {
	// Create app state
	state := models.NewAppState(2000)

	// Create protocol reader
	reader := protocol.NewReader(socketPath)
	err := reader.Connect()
	if err != nil {
		return nil, fmt.Errorf("failed to connect to socket: %w", err)
	}

	// Start reading
	reader.Start()

	// Ingestion is decoupled from rendering: continuously drain updates into
	// the mutex-guarded state from a background goroutine. The Bubble Tea loop
	// only renders (at renderInterval), so a fast simulation never floods or
	// stalls the UI.
	go ingest(reader, state)

	// Create views
	overviewView := views.NewOverviewView(state)
	respiratoryView := views.NewRespiratoryView(state)

	return &Model{
		state:           state,
		reader:          reader,
		overviewView:    overviewView,
		respiratoryView: respiratoryView,
		width:           120,
		height:          40,
		renderInterval:  defaultRenderInterval,
	}, nil
}

// ingest continuously drains reader updates into the application state.
func ingest(reader *protocol.Reader, state *models.AppState) {
	for {
		select {
		case update, ok := <-reader.Updates:
			if !ok {
				return
			}
			state.UpdateSignal(update.Key, update.Value)
			state.IncrementTick()
		case err, ok := <-reader.Errors:
			if !ok {
				return
			}
			state.SetLastError(err)
		}
	}
}

// Init initializes the application
func (m Model) Init() tea.Cmd {
	return tickEvery(m.renderInterval)
}

// Update handles messages and updates the model
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit

		case "tab":
			m.state.NextView()
			return m, nil

		case "shift+tab":
			m.state.PrevView()
			return m, nil

		case "1":
			m.state.SetView(models.ViewOverview)
			return m, nil

		case "2":
			m.state.SetView(models.ViewCardiovascular)
			return m, nil

		case "3":
			m.state.SetView(models.ViewRespiratory)
			return m, nil

		case "4":
			m.state.SetView(models.ViewNervous)
			return m, nil

		case "+", "=":
			// Faster refresh (shorter interval)
			m.renderInterval = max(m.renderInterval/2, minRenderInterval)
			return m, nil

		case "-", "_":
			// Slower refresh (longer interval)
			m.renderInterval = min(m.renderInterval*2, maxRenderInterval)
			return m, nil
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tickMsg:
		// Render tick: Bubble Tea repaints via View() after this returns. The
		// interval is re-read each tick so FPS changes take effect immediately.
		return m, tickEvery(m.renderInterval)
	}

	return m, nil
}

// View renders the application
func (m Model) View() string {
	if err := m.state.GetLastError(); err != nil {
		return fmt.Sprintf("Error: %v\n\nPress q to quit.", err)
	}

	// Ensure valid dimensions
	if m.width < 80 || m.height < 24 {
		return fmt.Sprintf("Terminal too small!\n\nMinimum size: 80 columns x 24 rows\nCurrent size: %d columns x %d rows\n\nPlease resize your terminal and press any key.", m.width, m.height)
	}

	// Render header
	header := m.renderHeader()

	// Render tabs
	tabs := m.renderTabs()

	// Render current view
	contentHeight := m.height - 6 // Account for header, tabs, help
	var content string

	switch m.state.GetView() {
	case models.ViewOverview:
		content = m.overviewView.Render(m.width, contentHeight)
	case models.ViewCardiovascular:
		content = "Cardiovascular Detail View (Coming Soon)"
	case models.ViewRespiratory:
		content = m.respiratoryView.Render(m.width, contentHeight)
	case models.ViewNervous:
		content = "Nervous Detail View (Coming Soon)"
	default:
		content = m.overviewView.Render(m.width, contentHeight)
	}

	// Render help
	help := m.renderHelp()

	// Combine all sections
	return lipgloss.JoinVertical(
		lipgloss.Left,
		header,
		tabs,
		content,
		help,
	)
}

// renderHeader renders the top header bar
func (m Model) renderHeader() string {
	// Status
	status := "Alive ●"
	statusStyle := styles.StatusAliveStyle
	if !m.state.GetIsAlive() {
		status = "Dead ✗"
		statusStyle = styles.StatusDeadStyle
	}

	// Time
	elapsed := m.state.GetElapsedTime()
	hours := int(elapsed.Hours())
	minutes := int(elapsed.Minutes()) % 60
	seconds := int(elapsed.Seconds()) % 60
	timeStr := fmt.Sprintf("Time: %02d:%02d:%02d", hours, minutes, seconds)

	// Tick count
	tickStr := fmt.Sprintf("Tick: %d", m.state.GetTickCount())

	// Build header
	left := styles.HeaderStyle.Render("Human Leon")
	middle := statusStyle.Render(status)
	right := styles.HeaderStyle.Render(fmt.Sprintf("%s | %s", timeStr, tickStr))

	// Calculate spacing
	usedWidth := lipgloss.Width(left) + lipgloss.Width(middle) + lipgloss.Width(right)
	spacingLeft := (m.width - usedWidth) / 2
	spacingRight := m.width - usedWidth - spacingLeft

	if spacingLeft < 0 {
		spacingLeft = 0
	}
	if spacingRight < 0 {
		spacingRight = 0
	}

	headerLine := left +
		lipgloss.NewStyle().Width(spacingLeft).Render("") +
		middle +
		lipgloss.NewStyle().Width(spacingRight).Render("") +
		right

	// Add bottom border
	borderWidth := m.width
	if borderWidth < 1 {
		borderWidth = 1
	}
	border := lipgloss.NewStyle().
		Foreground(styles.ColorBorder).
		Render(strings.Repeat("═", borderWidth))

	return headerLine + "\n" + border
}

// renderTabs renders the tab bar
func (m Model) renderTabs() string {
	currentView := m.state.GetView()

	views := []models.ViewType{
		models.ViewOverview,
		models.ViewCardiovascular,
		models.ViewRespiratory,
		models.ViewNervous,
	}

	var tabs []string
	for _, view := range views {
		if view == currentView {
			tabs = append(tabs, styles.TabActiveStyle.Render(view.String()))
		} else {
			tabs = append(tabs, styles.TabInactiveStyle.Render(view.String()))
		}
	}

	tabBar := lipgloss.JoinHorizontal(lipgloss.Top, tabs...)
	return tabBar + "\n"
}

// renderHelp renders the help bar
func (m Model) renderHelp() string {
	fps := int(time.Second / m.renderInterval)
	helpText := fmt.Sprintf("Tab: Next View | Shift+Tab: Prev View | 1-4: Jump to View | +/-: FPS (%d) | q: Quit", fps)
	return styles.HelpStyle.Render(helpText)
}

// Messages

type tickMsg time.Time

// Commands

// tickEvery returns a command that sends a tick message at regular intervals
func tickEvery(d time.Duration) tea.Cmd {
	return tea.Tick(d, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}
