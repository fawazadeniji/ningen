package main

import (
	"math"
	"testing"
)

func TestNDCGAtK_PerfectRanking(t *testing.T) {
	gt := map[string]float64{"a": 0.1, "b": 0.2}
	ranked := []string{"a", "b", "c"}
	got := ndcgAtK(ranked, gt, 10)
	if got < 0.999 {
		t.Errorf("perfect ranking should give NDCG~1.0, got %.4f", got)
	}
}

func TestNDCGAtK_NoHits(t *testing.T) {
	gt := map[string]float64{"x": 0.1, "y": 0.2}
	ranked := []string{"a", "b", "c"}
	got := ndcgAtK(ranked, gt, 10)
	if got != 0.0 {
		t.Errorf("no hits should give NDCG=0.0, got %.4f", got)
	}
}

func TestNDCGAtK_LaterHitLowerScore(t *testing.T) {
	gt := map[string]float64{"b": 0.1}
	// b at position 1 (first) vs position 2 (second)
	first := ndcgAtK([]string{"b", "a"}, gt, 10)
	second := ndcgAtK([]string{"a", "b"}, gt, 10)
	if first <= second {
		t.Errorf("earlier hit should score higher: first=%.4f second=%.4f", first, second)
	}
}

func TestNDCGAtK_CutoffAtK(t *testing.T) {
	gt := map[string]float64{"z": 0.1}
	ranked := []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "z"} // z at position 11
	got := ndcgAtK(ranked, gt, 10)
	if got != 0.0 {
		t.Errorf("item beyond k should not count, got %.4f", got)
	}
}

func TestNDCGAtK_EmptyGT(t *testing.T) {
	got := ndcgAtK([]string{"a", "b"}, map[string]float64{}, 10)
	if got != 0.0 {
		t.Errorf("empty GT should give NDCG=0, got %.4f", got)
	}
}

func TestHitAtK_Hit(t *testing.T) {
	gt := map[string]float64{"b": 0.2}
	got := hitAtK([]string{"a", "b", "c"}, gt, 10)
	if got != 1.0 {
		t.Errorf("expected hit=1.0, got %.1f", got)
	}
}

func TestHitAtK_Miss(t *testing.T) {
	gt := map[string]float64{"z": 0.2}
	got := hitAtK([]string{"a", "b", "c"}, gt, 10)
	if got != 0.0 {
		t.Errorf("expected hit=0.0, got %.1f", got)
	}
}

func TestHitAtK_BeyondK(t *testing.T) {
	gt := map[string]float64{"d": 0.2}
	got := hitAtK([]string{"a", "b", "c", "d"}, gt, 3) // d is at index 3, k=3
	if got != 0.0 {
		t.Errorf("item beyond k should not count as hit, got %.1f", got)
	}
}

func TestMRR_FirstPosition(t *testing.T) {
	gt := map[string]float64{"a": 0.1}
	got := mrrScore([]string{"a", "b"}, gt)
	if math.Abs(got-1.0) > 1e-9 {
		t.Errorf("first position: expected MRR=1.0, got %.4f", got)
	}
}

func TestMRR_SecondPosition(t *testing.T) {
	gt := map[string]float64{"b": 0.1}
	got := mrrScore([]string{"a", "b"}, gt)
	if math.Abs(got-0.5) > 1e-9 {
		t.Errorf("second position: expected MRR=0.5, got %.4f", got)
	}
}

func TestMRR_NoHit(t *testing.T) {
	gt := map[string]float64{"z": 0.1}
	got := mrrScore([]string{"a", "b"}, gt)
	if got != 0.0 {
		t.Errorf("no hit: expected MRR=0.0, got %.4f", got)
	}
}

func TestMean_Empty(t *testing.T) {
	if mean(nil) != 0 {
		t.Error("mean of empty slice should be 0")
	}
}

func TestMean_Values(t *testing.T) {
	got := mean([]float64{1.0, 2.0, 3.0})
	if math.Abs(got-2.0) > 1e-9 {
		t.Errorf("mean([1,2,3]) = %.4f, want 2.0", got)
	}
}
