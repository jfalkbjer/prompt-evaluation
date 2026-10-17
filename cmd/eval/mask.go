package main

import (
	"context"
	"fmt"

	"prompt-evaluation/internal/masking"
	"prompt-evaluation/internal/numbers"
	"prompt-evaluation/internal/score"
)

// prep rewrites the text before the model sees it.
type prep struct {
	suffix string
	apply  func(string) string
}

var (
	// maskOn replaces already-masked values such as 900101-****.
	maskOn = prep{"+mask", masking.Replace}
	// maskFull also replaces placeholders such as xxxx and [REDACTED], and labels valid personnummer,
	// samordningsnummer, and card numbers. It is the text duckdb/setup.sql sends to Kev.
	maskFull = prep{"+mask-full", func(s string) string { return numbers.Label(masking.ReplacePlaceholders(s)) }}
)

type maskedJudge struct {
	judge
	prep prep
}

func (j maskedJudge) Name() string { return j.judge.Name() + j.prep.suffix }

func (j maskedJudge) decide(ctx context.Context, text string) (bool, *float64, string, error) {
	return j.judge.decide(ctx, j.prep.apply(text))
}

type maskedChoice struct {
	choiceJudge
	prep prep
}

func (j maskedChoice) Name() string { return j.choiceJudge.Name() + j.prep.suffix }

func (j maskedChoice) choose(ctx context.Context, text string) (string, *float64, *float64, string, error) {
	return j.choiceJudge.choose(ctx, j.prep.apply(text))
}

func checkMask(mode string) error {
	switch mode {
	case "off", "on", "full", "both":
		return nil
	}
	return fmt.Errorf("unknown -mask %q: want off, on, full or both", mode)
}

// withMask applies the -mask mode. With both, each judge is followed by its masked twin.
func withMask[J any](judges []J, mode string, wrap func(J, prep) J) []J {
	switch mode {
	case "on", "full":
		p := maskOn
		if mode == "full" {
			p = maskFull
		}
		out := make([]J, len(judges))
		for i, j := range judges {
			out[i] = wrap(j, p)
		}
		return out
	case "both":
		out := make([]J, 0, 2*len(judges))
		for _, j := range judges {
			out = append(out, j, wrap(j, maskOn))
		}
		return out
	}
	return judges
}

// printMaskComparison prints model alone against model plus masking, for -mask both.
func printMaskComparison(s []score.Summary) {
	fmt.Println("model alone -> model + masking")
	for i := 0; i+1 < len(s); i += 2 {
		a, b := s[i], s[i+1]
		fmt.Printf("  %-28s accuracy %.3f -> %.3f (%+.3f)  precision %.3f -> %.3f  recall %.3f -> %.3f  fp %d -> %d  fn %d -> %d\n",
			a.Name, a.Accuracy, b.Accuracy, b.Accuracy-a.Accuracy, a.Precision, b.Precision, a.Recall, b.Recall,
			a.FalsePositives, b.FalsePositives, a.FalseNegatives, b.FalseNegatives)
	}
	fmt.Println()
}

func printClassMaskComparison(s []score.ClassSummary) {
	fmt.Println("model alone -> model + masking")
	for i := 0; i+1 < len(s); i += 2 {
		a, b := s[i], s[i+1]
		fmt.Printf("  %-28s accuracy %.3f -> %.3f (%+.3f)\n", a.Name, a.Accuracy, b.Accuracy, b.Accuracy-a.Accuracy)
	}
	fmt.Println()
}
