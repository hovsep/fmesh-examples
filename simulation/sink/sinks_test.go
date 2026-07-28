package sink

import "testing"

func TestChannel_DeliversLines(t *testing.T) {
	s := NewChannel(4)

	for _, line := range []string{"a 1", "b 2"} {
		if err := s.Publish(line); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}

	var got []string
	for line := range s.Lines() {
		got = append(got, line)
	}
	if len(got) != 2 || got[0] != "a 1" || got[1] != "b 2" {
		t.Fatalf("drained %v, want [a 1, b 2]", got)
	}
}

func TestChannel_DropsRatherThanBlocks(t *testing.T) {
	// Telemetry must never apply back-pressure: with nobody draining, a full
	// buffer costs lines, not a stalled simulation.
	s := NewChannel(2)

	for range 100 {
		if err := s.Publish("line"); err != nil {
			t.Fatal(err)
		}
	}

	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	if got := len(s.Lines()); got != 2 {
		t.Fatalf("buffered %d lines, want the buffer size (2)", got)
	}
}

func TestNoopAndStdoutAreSafeToCloseTwice(t *testing.T) {
	for _, s := range []Sink{NewNoop(), NewStdout()} {
		if err := s.Publish("x"); err != nil {
			t.Fatal(err)
		}
		if err := s.Close(); err != nil {
			t.Fatal(err)
		}
		if err := s.Close(); err != nil {
			t.Fatalf("second Close: %v", err)
		}
	}
}
