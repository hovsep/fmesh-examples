package console

import (
	"bufio"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// maxHistory bounds the file so a long-lived session does not grow it forever.
const maxHistory = 500

// History is the list of commands entered, browsable with the arrow keys and
// kept across sessions.
//
// The cursor sits one past the end when the user is typing something new, which
// is what makes "up, up, down, down" land back on the unsent line rather than on
// the newest history entry.
type History struct {
	path    string
	entries []string
	cursor  int
	draft   string // what was being typed before browsing started
}

// LoadHistory reads history from path, tolerating its absence.
func LoadHistory(path string) *History {
	h := &History{path: path}

	file, err := os.Open(path)
	if err != nil {
		return h // no history yet, which is not a problem worth reporting
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		if line := strings.TrimSpace(scanner.Text()); line != "" {
			h.entries = append(h.entries, line)
		}
	}
	h.trim()
	h.cursor = len(h.entries)
	return h
}

// DefaultHistoryPath is where history lives when the caller has no preference.
func DefaultHistoryPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ".life_history"
	}
	return filepath.Join(home, ".life_history")
}

// Add records a command, ignoring blanks and immediate repeats so holding a key
// or re-running the same thing does not flood the list.
func (h *History) Add(line string) {
	line = strings.TrimSpace(line)
	if line == "" {
		return
	}
	if len(h.entries) > 0 && h.entries[len(h.entries)-1] == line {
		h.reset()
		return
	}

	h.entries = append(h.entries, line)
	h.trim()
	h.reset()
}

// Entries returns the recorded commands, oldest first.
func (h *History) Entries() []string { return slices.Clone(h.entries) }

// Prev walks back through history, returning the line to show.
func (h *History) Prev(current string) (string, bool) {
	if len(h.entries) == 0 || h.cursor == 0 {
		return "", false
	}

	// Stepping off the end of history means the current line is an unsent draft;
	// remember it so walking forward again restores it.
	if h.cursor == len(h.entries) {
		h.draft = current
	}

	h.cursor--
	return h.entries[h.cursor], true
}

// Next walks forward through history, returning the line to show. Past the
// newest entry it returns the draft the user was typing.
func (h *History) Next() (string, bool) {
	if h.cursor >= len(h.entries) {
		return "", false
	}

	h.cursor++
	if h.cursor == len(h.entries) {
		return h.draft, true
	}
	return h.entries[h.cursor], true
}

// Save writes history back to disk. Failure is reported so the caller can
// mention it, but it is never fatal: losing history should not lose the session.
func (h *History) Save() error {
	if h.path == "" {
		return nil
	}

	file, err := os.OpenFile(h.path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := bufio.NewWriter(file)
	for _, entry := range h.entries {
		if _, err := writer.WriteString(entry + "\n"); err != nil {
			return err
		}
	}
	return writer.Flush()
}

func (h *History) reset() {
	h.cursor = len(h.entries)
	h.draft = ""
}

func (h *History) trim() {
	if len(h.entries) > maxHistory {
		h.entries = h.entries[len(h.entries)-maxHistory:]
	}
}
