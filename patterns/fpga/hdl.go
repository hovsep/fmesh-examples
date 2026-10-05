package main

import (
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"
)

// A tiny hardware description language, a strict subset of Verilog:
//
//	module name(input a, b, output s);
//	  assign s = a ^ b;                    // combinational: follows its inputs
//	  always @(posedge clk) q <= q ^ a;    // a register: changes on the clock
//	endmodule
//
// Expressions use ~ & ^ | (in that order of precedence), parentheses, 0 and 1.
// Synthesis turns every assign/always into one look-up table: the truth
// table of its expression, which is what an FPGA cell is programmed with.

// LUTInputs is how many inputs one logic cell has.
const LUTInputs = 4

// Design is a synthesised module: what gets flashed onto the board.
type Design struct {
	Name    string
	Inputs  []string // pins driven from outside
	Outputs []string // pins read from outside
	Cells   []Cell   // one per assign/always, in source order
}

// Cell is one configured logic cell.
type Cell struct {
	Signal     string   // the signal it drives
	Source     string   // the expression, as written
	Inputs     []string // the signals it reads, in LUT input order
	Table      []bool   // Table[i] is the output for input bits i (input 0 is bit 0)
	Registered bool     // true for always @(posedge clk): a flip-flop after the LUT
}

// TableString renders the truth table, highest index first, like a bitstream.
func (c Cell) TableString() string {
	var b strings.Builder
	for i := len(c.Table) - 1; i >= 0; i-- {
		if c.Table[i] {
			b.WriteByte('1')
		} else {
			b.WriteByte('0')
		}
	}
	return b.String()
}

var (
	reModule = regexp.MustCompile(`^module\s+(\w+)\s*\((.*)\)$`)
	reAssign = regexp.MustCompile(`^assign\s+(\w+)\s*=\s*(.+)$`)
	reAlways = regexp.MustCompile(`^always\s*@\(\s*posedge\s+clk\s*\)\s*(\w+)\s*<=\s*(.+)$`)
	reWire   = regexp.MustCompile(`^(wire|reg)\s+[\w\s,]+$`)
)

// Synthesize parses a module and compiles every statement into a cell.
func Synthesize(source string) (*Design, error) {
	var lines []string
	for _, line := range strings.Split(source, "\n") {
		line, _, _ = strings.Cut(line, "//")
		lines = append(lines, line)
	}

	d := &Design{}
	ended := false
	for _, stmt := range strings.Split(strings.Join(lines, " "), ";") {
		stmt = strings.Join(strings.Fields(stmt), " ")
		if rest, ok := strings.CutPrefix(stmt, "endmodule"); ok {
			ended, stmt = true, strings.TrimSpace(rest)
		}
		switch m := matchAny(stmt); {
		case stmt == "":
		case m.module != nil:
			d.Name = m.module[1]
			if err := d.parsePorts(m.module[2]); err != nil {
				return nil, err
			}
		case m.assign != nil:
			d.Cells = append(d.Cells, Cell{Signal: m.assign[1], Source: m.assign[2]})
		case m.always != nil:
			d.Cells = append(d.Cells, Cell{Signal: m.always[1], Source: m.always[2], Registered: true})
		case reWire.MatchString(stmt):
		default:
			return nil, fmt.Errorf("cannot parse %q", stmt)
		}
	}
	if d.Name == "" || !ended {
		return nil, errors.New("expected module ... endmodule")
	}
	return d, d.compile()
}

type match struct{ module, assign, always []string }

func matchAny(stmt string) match {
	return match{reModule.FindStringSubmatch(stmt), reAssign.FindStringSubmatch(stmt), reAlways.FindStringSubmatch(stmt)}
}

// parsePorts reads "input a, b, output s, t".
func (d *Design) parsePorts(list string) error {
	var dir *[]string
	for _, item := range strings.Split(list, ",") {
		words := strings.Fields(item)
		if len(words) == 2 && (words[0] == "input" || words[0] == "output") {
			dir, words = map[string]*[]string{"input": &d.Inputs, "output": &d.Outputs}[words[0]], words[1:]
		}
		if dir == nil || len(words) != 1 {
			return fmt.Errorf("bad port list %q", list)
		}
		*dir = append(*dir, words[0])
	}
	return nil
}

// compile works out each cell's inputs and truth table, and checks that
// every signal is driven exactly once and every cell fits in a LUT.
func (d *Design) compile() error {
	driven := map[string]bool{}
	for _, in := range d.Inputs {
		driven[in] = true
	}
	for _, c := range d.Cells {
		if driven[c.Signal] {
			return fmt.Errorf("%s is driven twice", c.Signal)
		}
		driven[c.Signal] = true
	}

	for i := range d.Cells {
		c := &d.Cells[i]
		expr, err := parseExpr(c.Source)
		if err != nil {
			return fmt.Errorf("%s: %w", c.Signal, err)
		}
		c.Inputs = expr.signals(nil)
		for _, s := range c.Inputs {
			if !driven[s] {
				return fmt.Errorf("%s reads %s, which nothing drives", c.Signal, s)
			}
		}
		if len(c.Inputs) == 0 {
			return fmt.Errorf("%s reads no signal: a constant needs no cell", c.Signal)
		}
		if len(c.Inputs) > LUTInputs {
			return fmt.Errorf("%s reads %d signals, but a cell has %d inputs: split it with an assign",
				c.Signal, len(c.Inputs), LUTInputs)
		}

		// The truth table: evaluate the expression for every input combination.
		c.Table = make([]bool, 1<<len(c.Inputs))
		for bits := range c.Table {
			env := map[string]bool{}
			for j, s := range c.Inputs {
				env[s] = bits&(1<<j) != 0
			}
			c.Table[bits] = expr.eval(env)
		}
	}

	for _, out := range d.Outputs {
		if !slices.ContainsFunc(d.Cells, func(c Cell) bool { return c.Signal == out }) {
			return fmt.Errorf("output %s is not driven by any assign or always", out)
		}
	}
	return nil
}

// expr is a parsed expression tree.
type expr struct {
	op       byte // 0 for a leaf; '~', '&', '^', '|' otherwise
	name     string
	constant bool
	isConst  bool
	args     []*expr
}

func (e *expr) eval(env map[string]bool) bool {
	switch e.op {
	case 0:
		if e.isConst {
			return e.constant
		}
		return env[e.name]
	case '~':
		return !e.args[0].eval(env)
	case '&':
		return e.args[0].eval(env) && e.args[1].eval(env)
	case '^':
		return e.args[0].eval(env) != e.args[1].eval(env)
	default: // '|'
		return e.args[0].eval(env) || e.args[1].eval(env)
	}
}

// signals lists the signal names the expression reads, in order of first use.
func (e *expr) signals(seen []string) []string {
	if e.op == 0 && !e.isConst && !slices.Contains(seen, e.name) {
		return append(seen, e.name)
	}
	for _, a := range e.args {
		seen = a.signals(seen)
	}
	return seen
}

var reToken = regexp.MustCompile(`\s*(\w+|[~&^|()])`)

// parseExpr is a recursive-descent parser: | binds loosest, then ^, then &, then ~.
func parseExpr(src string) (*expr, error) {
	var tokens []string
	rest := src
	for strings.TrimSpace(rest) != "" {
		m := reToken.FindStringSubmatchIndex(rest)
		if m == nil || m[0] != 0 {
			return nil, fmt.Errorf("unexpected %q", strings.TrimSpace(rest))
		}
		tokens = append(tokens, rest[m[2]:m[3]])
		rest = rest[m[1]:]
	}

	pos := 0
	peek := func() string {
		if pos < len(tokens) {
			return tokens[pos]
		}
		return ""
	}
	var binary func(level int) (*expr, error)
	var unary func() (*expr, error)
	ops := []byte{'|', '^', '&'}

	binary = func(level int) (*expr, error) {
		if level == len(ops) {
			return unary()
		}
		left, err := binary(level + 1)
		if err != nil {
			return nil, err
		}
		for peek() == string(ops[level]) {
			pos++
			right, err := binary(level + 1)
			if err != nil {
				return nil, err
			}
			left = &expr{op: ops[level], args: []*expr{left, right}}
		}
		return left, nil
	}
	unary = func() (*expr, error) {
		tok := peek()
		pos++
		switch {
		case tok == "~":
			arg, err := unary()
			return &expr{op: '~', args: []*expr{arg}}, err
		case tok == "(":
			inner, err := binary(0)
			if err != nil {
				return nil, err
			}
			if peek() != ")" {
				return nil, fmt.Errorf("missing ) in %q", src)
			}
			pos++
			return inner, nil
		case tok == "0" || tok == "1":
			return &expr{isConst: true, constant: tok == "1"}, nil
		case regexp.MustCompile(`^[A-Za-z_]\w*$`).MatchString(tok):
			return &expr{name: tok}, nil
		}
		return nil, fmt.Errorf("unexpected %q in %q", tok, src)
	}

	e, err := binary(0)
	if err != nil {
		return nil, err
	}
	if pos != len(tokens) {
		return nil, fmt.Errorf("unexpected %q in %q", tokens[pos], src)
	}
	return e, nil
}
