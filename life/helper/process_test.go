package helper

import "testing"

func TestProcess_DeliversWholeQuantityOverTime(t *testing.T) {
	p := &Process{Kind: "water_ml", Remaining: 500, RatePerSec: 100}

	var total float64
	for range 100 { // 100 ticks of 0.1s = 10s, enough for 500 at 100/s (5s)
		total += p.Advance(0.1)
	}

	if !p.Done() {
		t.Fatalf("process should be finished, %v remaining", p.Remaining)
	}
	if total < 499.9 || total > 500.1 {
		t.Fatalf("delivered %v, want the full 500", total)
	}
}

func TestProcess_LargerQuantityTakesProportionallyLonger(t *testing.T) {
	const rate = 100
	ticksTo := func(quantity float64) int {
		p := &Process{Kind: "water_ml", Remaining: quantity, RatePerSec: rate}
		n := 0
		for !p.Done() {
			p.Advance(0.01)
			n++
		}
		return n
	}

	small := ticksTo(50)
	large := ticksTo(500)

	// 500 mL should take about ten times as long as 50 mL.
	ratio := float64(large) / float64(small)
	if ratio < 9.5 || ratio > 10.5 {
		t.Fatalf("500mL took %dx as long as 50mL, want ~10x", int(ratio))
	}
}

func TestProcess_NeverOverdelivers(t *testing.T) {
	p := &Process{Kind: "food_kcal", Remaining: 10, RatePerSec: 1000}
	// One big tick would deliver 100 at the raw rate; it must cap at what is left.
	if got := p.Advance(0.1); got != 10 {
		t.Fatalf("delivered %v, want exactly the 10 that remained", got)
	}
	if !p.Done() {
		t.Fatal("process should be finished")
	}
}

func TestProcessSet_RunsSeveralAtOnceAndDropsFinished(t *testing.T) {
	var set ProcessSet
	set.Start(&Process{Kind: "water_ml", Remaining: 10, RatePerSec: 100}) // finishes fast
	set.Start(&Process{Kind: "food_kcal", Remaining: 100, RatePerSec: 5}) // lingers

	first := set.Advance(0.1) // 0.1s: water gets 10 (done), food gets 0.5
	if first["water_ml"] != 10 {
		t.Fatalf("water delivered %v, want 10", first["water_ml"])
	}
	if first["food_kcal"] < 0.49 || first["food_kcal"] > 0.51 {
		t.Fatalf("food delivered %v, want ~0.5", first["food_kcal"])
	}
	if set.Active() != 1 {
		t.Fatalf("expected only the food process left, got %d active", set.Active())
	}
}

func TestProcessSet_IgnoresEmptyStarts(t *testing.T) {
	var set ProcessSet
	set.Start(&Process{Kind: "x", Remaining: 0, RatePerSec: 100})
	set.Start(&Process{Kind: "x", Remaining: 100, RatePerSec: 0})
	set.Start(nil)
	if set.Active() != 0 {
		t.Fatalf("empty processes should not be started, got %d active", set.Active())
	}
}
