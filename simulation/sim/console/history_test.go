package console

import (
	"path/filepath"
	"testing"
)

func TestHistory_ArrowUpWalksBackwards(t *testing.T) {
	h := &History{}
	h.Add("intake:water 500ml")
	h.Add("intake:food 200kcal")
	h.reset()

	// Newest first, which is what pressing up expects.
	if line, ok := h.Prev(""); !ok || line != "intake:food 200kcal" {
		t.Fatalf("first Prev = %q (ok=%v)", line, ok)
	}
	if line, ok := h.Prev(""); !ok || line != "intake:water 500ml" {
		t.Fatalf("second Prev = %q (ok=%v)", line, ok)
	}
	if _, ok := h.Prev(""); ok {
		t.Fatal("Prev walked past the oldest entry")
	}
}

func TestHistory_ArrowDownRestoresTheUnsentDraft(t *testing.T) {
	h := &History{}
	h.Add("intake:water 500ml")

	// Walking back from a half-typed line, then forward again, must give the
	// half-typed line back rather than losing it.
	if line, _ := h.Prev("intake:foo"); line != "intake:water 500ml" {
		t.Fatalf("Prev = %q", line)
	}

	line, ok := h.Next()
	if !ok {
		t.Fatal("Next returned nothing at the end of history")
	}
	if line != "intake:foo" {
		t.Fatalf("Next = %q, want the unsent draft %q", line, "intake:foo")
	}
}

func TestHistory_IgnoresBlanksAndRepeats(t *testing.T) {
	h := &History{}
	h.Add("jobs")
	h.Add("jobs")
	h.Add("   ")
	h.Add("")

	if got := h.Entries(); len(got) != 1 {
		t.Fatalf("history recorded %v, want a single entry", got)
	}
}

func TestHistory_AddingResetsBrowsing(t *testing.T) {
	h := &History{}
	h.Add("first")
	h.Add("second")

	h.Prev("") // browse back
	h.Add("third")

	// After entering a command, up should offer the newest entry again rather
	// than resuming wherever browsing had got to.
	if line, _ := h.Prev(""); line != "third" {
		t.Fatalf("Prev after Add = %q, want %q", line, "third")
	}
}

func TestHistory_SurvivesARestart(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history")

	h := LoadHistory(path)
	h.Add("intake:water 500ml")
	h.Add("every 1d excretion:defecate")
	if err := h.Save(); err != nil {
		t.Fatal(err)
	}

	reloaded := LoadHistory(path)
	entries := reloaded.Entries()
	if len(entries) != 2 || entries[1] != "every 1d excretion:defecate" {
		t.Fatalf("reloaded history = %v", entries)
	}

	// And it is immediately browsable, not left mid-list.
	if line, ok := reloaded.Prev(""); !ok || line != "every 1d excretion:defecate" {
		t.Fatalf("reloaded Prev = %q (ok=%v)", line, ok)
	}
}

func TestLoadHistory_MissingFileIsFine(t *testing.T) {
	h := LoadHistory(filepath.Join(t.TempDir(), "does-not-exist"))
	if len(h.Entries()) != 0 {
		t.Fatal("expected empty history")
	}
	if _, ok := h.Prev(""); ok {
		t.Fatal("empty history offered an entry")
	}
}

func TestHistory_IsBounded(t *testing.T) {
	h := &History{}
	for i := range maxHistory + 50 {
		h.Add(string(rune('a'+i%26)) + string(rune('0'+i%10)) + string(rune(i)))
	}

	if got := len(h.Entries()); got > maxHistory {
		t.Fatalf("history grew to %d entries, want at most %d", got, maxHistory)
	}
}

func TestCommonPrefix(t *testing.T) {
	tests := []struct {
		name string
		in   []string
		want string
	}{
		{"shared namespace", []string{"intake:water", "intake:food"}, "intake:"},
		{"nothing shared", []string{"jobs", "exit"}, ""},
		{"single value", []string{"help"}, "help"},
		{"empty", nil, ""},
		{"one is a prefix of the other", []string{"rate", "rate:sim", "rate:publish"}, "rate"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := commonPrefix(tt.in); got != tt.want {
				t.Fatalf("commonPrefix(%v) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}
