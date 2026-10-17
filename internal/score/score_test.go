package score

import (
	"math"
	"testing"
	"time"
)

func TestSummarize(t *testing.T) {
	p := func(v float64) *float64 { return &v }
	outcomes := []Outcome{
		{ID: "a", Gold: true, Predicted: true, Probability: p(1), Latency: time.Millisecond, LatencyMS: 10},
		{ID: "b", Gold: true, Predicted: false, Probability: p(0), Latency: time.Millisecond, LatencyMS: 30},
		{ID: "c", Gold: false, Predicted: true, Probability: p(1), Latency: time.Millisecond, LatencyMS: 20},
		{ID: "d", Gold: false, Predicted: false, Probability: p(0), Latency: time.Millisecond, LatencyMS: 40},
	}
	got := Summarize("test", outcomes)
	if got.TruePositives != 1 || got.FalseNegatives != 1 || got.FalsePositives != 1 || got.TrueNegatives != 1 {
		t.Fatalf("counts = %+v", got)
	}
	if math.Abs(got.Accuracy-0.5) > 1e-9 {
		t.Fatalf("accuracy = %v", got.Accuracy)
	}
	if math.Abs(got.Precision-0.5) > 1e-9 || math.Abs(got.Recall-0.5) > 1e-9 {
		t.Fatalf("precision/recall = %v/%v", got.Precision, got.Recall)
	}
	if got.Brier == nil || math.Abs(*got.Brier-0.5) > 1e-9 {
		t.Fatalf("brier = %v", got.Brier)
	}
	if got.LatencyMS.P50 != 20 || got.LatencyMS.P95 != 40 {
		t.Fatalf("latency = %+v", got.LatencyMS)
	}
	if len(got.Disagreements) != 2 {
		t.Fatalf("disagreements = %d", len(got.Disagreements))
	}
}

func TestSummarizeBySet(t *testing.T) {
	got := Summarize("test", []Outcome{
		{Source: "main", Gold: true, Predicted: true},
		{Source: "extra", Gold: true, Predicted: false},
		{Source: "main", Gold: false, Predicted: true},
		{Source: "extra", Gold: false, Predicted: false},
		{Source: "extra", Gold: true, Predicted: true},
	})
	want := []SetScore{{"main", 2, 1, 0.5}, {"extra", 3, 2, 2.0 / 3}}
	if len(got.Sets) != len(want) {
		t.Fatalf("sets = %+v", got.Sets)
	}
	for i := range want {
		if got.Sets[i] != want[i] {
			t.Errorf("set %d = %+v, want %+v", i, got.Sets[i], want[i])
		}
	}
	if Summarize("no sources", []Outcome{{Gold: true, Predicted: true}}).Sets != nil {
		t.Error("expected no sets without sources")
	}
}

func TestSummarizeOmitsBrierWithoutProbabilities(t *testing.T) {
	got := Summarize("llm", []Outcome{
		{Gold: true, Predicted: true, LatencyMS: 5},
	})
	if got.Brier != nil {
		t.Fatal("expected no brier score")
	}
	if got.Accuracy != 1 {
		t.Fatalf("accuracy = %v", got.Accuracy)
	}
}
