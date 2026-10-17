package main

import (
	"context"
	"testing"

	"prompt-evaluation/internal/kev"
)

type fakeChooser struct{ choice kev.Choice }

func (f fakeChooser) Name() string { return "fake" }

func (f fakeChooser) Choose(context.Context, string) (kev.Choice, error) { return f.choice, nil }

func TestGateJudge(t *testing.T) {
	tests := []struct {
		name     string
		choice   kev.Choice
		positive bool
		p        float64
	}{
		{"none is not sensitive", kev.Choice{Name: "none", Probabilities: map[string]float64{"none": 0.8, "secret": 0.2}}, false, 0.2},
		{"any other kind is sensitive", kev.Choice{Name: "secret", Probabilities: map[string]float64{"none": 0.3, "secret": 0.7}}, true, 0.7},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			j := gateJudge{fakeChooser{tt.choice}}
			positive, p, _, err := j.decide(context.Background(), "text")
			if err != nil {
				t.Fatal(err)
			}
			if positive != tt.positive || p == nil || *p < tt.p-1e-9 || *p > tt.p+1e-9 {
				t.Fatalf("positive=%v p=%v, want %v %v", positive, *p, tt.positive, tt.p)
			}
		})
	}
	if got := (gateJudge{fakeChooser{}}).Name(); got != "fake-gate" {
		t.Fatalf("name %q", got)
	}
}

type echoJudge struct{ seen *string }

func (e echoJudge) Name() string { return "echo" }

func (e echoJudge) decide(_ context.Context, text string) (bool, *float64, string, error) {
	*e.seen = text
	return false, nil, "", nil
}

func TestWithMask(t *testing.T) {
	var seen string
	base := []judge{echoJudge{&seen}}
	wrap := func(j judge, p prep) judge { return maskedJudge{j, p} }

	if got := withMask(base, "off", wrap); len(got) != 1 || got[0].Name() != "echo" {
		t.Fatalf("off: %v", got)
	}
	full := withMask(base, "full", wrap)
	if len(full) != 1 || full[0].Name() != "echo+mask-full" {
		t.Fatalf("full: %v", full)
	}
	_, _, _, _ = full[0].decide(context.Background(), "id xxxx, pnr 197306253977")
	if seen != "id masked, pnr [personnummer: 197306253977]" {
		t.Fatalf("full mask judge saw %q", seen)
	}
	both := withMask(base, "both", wrap)
	if len(both) != 2 || both[0].Name() != "echo" || both[1].Name() != "echo+mask" {
		t.Fatalf("both: %v", both)
	}
	_, _, _, _ = both[1].decide(context.Background(), "pnr 900101-****")
	if seen != "pnr masked" {
		t.Fatalf("masked judge saw %q", seen)
	}
	_, _, _, _ = both[0].decide(context.Background(), "pnr 900101-****")
	if seen != "pnr 900101-****" {
		t.Fatalf("plain judge saw %q", seen)
	}
}

type fixedJudge struct{ positive bool }

func (f fixedJudge) Name() string { return "fixed" }

func (f fixedJudge) decide(context.Context, string) (bool, *float64, string, error) {
	p := 0.2
	return f.positive, &p, "", nil
}

func TestRuleJudge(t *testing.T) {
	j := ruleJudge{fixedJudge{false}}
	if j.Name() != "fixed+rule" {
		t.Fatalf("name %q", j.Name())
	}
	positive, p, _, _ := j.decide(context.Background(), "if I add 197306253977 it will be")
	if !positive || *p != 1 {
		t.Fatalf("valid personnummer: positive=%v p=%v", positive, *p)
	}
	positive, p, _, _ = j.decide(context.Background(), "invoice 197306253978")
	if positive || *p != 0.2 {
		t.Fatalf("failing check digit: positive=%v p=%v, want the model's answer", positive, *p)
	}
}

type countingChoice struct {
	label string
	calls *int
}

func (c countingChoice) Name() string { return "fixed" }

func (c countingChoice) choose(context.Context, string) (string, *float64, *float64, string, error) {
	*c.calls++
	return c.label, nil, nil, "", nil
}

func TestCombinedJudge(t *testing.T) {
	calls := 0
	no := combinedJudge{fixedJudge{false}, countingChoice{"government_id", &calls}}
	if no.Name() != "fixed-combined" {
		t.Fatalf("name %q", no.Name())
	}
	if label, p, _, _, _ := no.choose(context.Background(), "My governmentid is xxxx"); label != "none" || *p != 0.2 || calls != 0 {
		t.Fatalf("not sensitive: label=%q p=%v choice calls=%d, want none without asking the choice question", label, *p, calls)
	}
	yes := combinedJudge{fixedJudge{true}, countingChoice{"government_id", &calls}}
	if label, _, _, _, _ := yes.choose(context.Background(), "My personnummer is 900101-1118"); label != "government_id" || calls != 1 {
		t.Fatalf("sensitive: label=%q choice calls=%d", label, calls)
	}
}

func TestGateJudgeMissingNone(t *testing.T) {
	j := gateJudge{fakeChooser{kev.Choice{Name: "secret", Probabilities: map[string]float64{"secret": 1}}}}
	if _, _, _, err := j.decide(context.Background(), "text"); err == nil {
		t.Fatal("expected an error without a none probability")
	}
}
