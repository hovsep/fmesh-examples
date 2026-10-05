package main

import (
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/hovsep/fmesh/signal"
)

// Config is what rules.json holds: the conditions an email can be tested
// against, and the rules that combine them into actions.
type Config struct {
	Conditions []Condition `json:"conditions"`
	Rules      []Rule      `json:"rules"`
}

// Condition tests one field of an email.
type Condition struct {
	Name  string `json:"name"`
	Field string `json:"field"`
	Op    string `json:"op"`
	Value any    `json:"value"` // a string, or a number for numeric ops
}

// Rule fires its actions when all (or any) of its conditions hold.
// Exactly one of All and Any is set.
type Rule struct {
	Name    string   `json:"name"`
	All     []string `json:"all"`
	Any     []string `json:"any"`
	Actions []string `json:"actions"`
}

// conditionNames returns the conditions the rule combines, whichever list
// they are in.
func (r Rule) conditionNames() []string {
	if len(r.All) > 0 {
		return r.All
	}
	return r.Any
}

// The operators a condition can use. String ops compare case-insensitively.
var operators = map[string]func(field, want any) bool{
	"equals":       stringOp(strings.EqualFold),
	"contains":     stringOp(func(s, sub string) bool { return strings.Contains(strings.ToLower(s), strings.ToLower(sub)) }),
	"starts_with":  stringOp(func(s, p string) bool { return strings.HasPrefix(strings.ToLower(s), strings.ToLower(p)) }),
	"ends_with":    stringOp(func(s, p string) bool { return strings.HasSuffix(strings.ToLower(s), strings.ToLower(p)) }),
	"greater_than": numberOp(func(a, b float64) bool { return a > b }),
	"less_than":    numberOp(func(a, b float64) bool { return a < b }),
}

func stringOp(f func(field, want string) bool) func(field, want any) bool {
	return func(field, want any) bool {
		s, ok1 := field.(string)
		w, ok2 := want.(string)
		return ok1 && ok2 && f(s, w)
	}
}

func numberOp(f func(field, want float64) bool) func(field, want any) bool {
	return func(field, want any) bool {
		n, ok1 := field.(float64)
		w, ok2 := want.(float64)
		return ok1 && ok2 && f(n, w)
	}
}

// Matches reports whether an email (a signal whose metadata holds its fields)
// satisfies the condition.
func (c Condition) Matches(email *signal.Signal) bool {
	field, ok := email.Meta().All()[c.Field]
	return ok && operators[c.Op](field, c.Value)
}

// LoadConfig reads and validates a rules file. Every mistake a typo can make
// is caught here, before a mesh is built from it.
func LoadConfig(path string) (*Config, error) {
	raw, err := readFile(path, defaultRules)
	if err != nil {
		return nil, err
	}
	var cfg Config
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return &cfg, nil
}

// Validate checks that every name is unique and every reference resolves.
func (cfg *Config) Validate() error {
	var errs []error
	conditions := make(map[string]bool)
	for _, c := range cfg.Conditions {
		if conditions[c.Name] {
			errs = append(errs, fmt.Errorf("condition %q is defined twice", c.Name))
		}
		conditions[c.Name] = true
		if _, ok := operators[c.Op]; !ok {
			errs = append(errs, fmt.Errorf("condition %q: unknown op %q", c.Name, c.Op))
		}
	}

	rules := make(map[string]bool)
	for _, r := range cfg.Rules {
		if rules[r.Name] {
			errs = append(errs, fmt.Errorf("rule %q is defined twice", r.Name))
		}
		rules[r.Name] = true
		if (len(r.All) > 0) == (len(r.Any) > 0) {
			errs = append(errs, fmt.Errorf("rule %q: set exactly one of \"all\" and \"any\"", r.Name))
		}
		if len(r.Actions) == 0 {
			errs = append(errs, fmt.Errorf("rule %q has no actions", r.Name))
		}
		for _, name := range r.conditionNames() {
			if !conditions[name] {
				errs = append(errs, fmt.Errorf("rule %q: unknown condition %q", r.Name, name))
			}
		}
	}
	return errors.Join(errs...)
}

// Email is one message in inbox.json.
type Email struct {
	ID         string  `json:"id"`
	From       string  `json:"from"`
	Subject    string  `json:"subject"`
	Attachment string  `json:"attachment"`
	SizeKB     float64 `json:"size_kb"`
}

// Signal turns an email into what flows through the mesh: its id as the
// payload, its fields as metadata the conditions read.
func (e Email) Signal() *signal.Signal {
	return signal.New(e.ID).
		WithMetaMany(map[string]string{"from": e.From, "subject": e.Subject, "attachment": e.Attachment}).
		WithMeta("size_kb", e.SizeKB)
}

// LoadInbox reads the emails to run the rules on.
func LoadInbox(path string) ([]Email, error) {
	raw, err := readFile(path, defaultInbox)
	if err != nil {
		return nil, err
	}
	var emails []Email
	if err := json.Unmarshal(raw, &emails); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return emails, nil
}

// The sample files ship inside the binary, so the example runs from any
// directory; -rules and -inbox read your own files instead.
var (
	//go:embed rules.json
	defaultRules []byte
	//go:embed inbox.json
	defaultInbox []byte
)

func readFile(path string, fallback []byte) ([]byte, error) {
	if path == "" {
		return fallback, nil
	}
	return os.ReadFile(path)
}
