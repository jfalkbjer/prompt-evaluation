package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"prompt-evaluation/internal/bedrock"
	"prompt-evaluation/internal/decider"
	"prompt-evaluation/internal/gliner"
	"prompt-evaluation/internal/kev"
	"prompt-evaluation/internal/laya"
	"prompt-evaluation/internal/rubric"
	"prompt-evaluation/internal/score"
	"prompt-evaluation/internal/strands"
)

type choiceJudge interface {
	Name() string
	choose(ctx context.Context, text string) (label string, probability, confidence *float64, parseError string, err error)
}

// runChoice scores the PII-kind task. With combined set, each backend first answers the yes/no
// question and is asked the choice question only when the answer is yes, as in duckdb/setup.sql.
func runChoice(opts options, combined bool) error {
	cases, err := loadCases(opts)
	if err != nil {
		return err
	}
	urls := opts.urls

	ctx := context.Background()
	var judges []choiceJudge
	add := func(noul judge, c choiceJudge) {
		if !combined {
			judges = append(judges, c)
			return
		}
		if opts.rubric.Name != rubric.Original.Name {
			noul = renamed{noul, "+" + opts.rubric.Name}
		}
		judges = append(judges, combinedJudge{noul, c})
	}
	for _, name := range strings.Split(opts.backends, ",") {
		switch strings.TrimSpace(name) {
		case "kev":
			client := kev.New(urls.kev)
			client.Rubric = opts.rubric
			if err := client.Ping(ctx); err != nil {
				return err
			}
			add(kevJudge{client}, kevChoice{client})
		case "laya":
			client := laya.New(urls.laya)
			client.Rubric = opts.rubric
			if err := client.Ping(ctx); err != nil {
				return err
			}
			add(layaJudge{client}, layaChoice{client})
		case "strands":
			client := strands.New(urls.strands)
			client.Rubric = opts.rubric
			if err := client.Ping(ctx); err != nil {
				return err
			}
			add(strandsJudge{client}, strandsChoice{client})
		case "gliner":
			client := gliner.New(urls.gliner)
			client.Rubric = opts.rubric
			if err := client.Ping(ctx); err != nil {
				return err
			}
			add(glinerJudge{client}, glinerChoice{client})
		case "decider":
			client := decider.New(urls.decider)
			client.Rubric = opts.rubric
			if err := client.Ping(ctx); err != nil {
				return err
			}
			add(deciderJudge{client}, deciderChoice{client})
		case "bedrock":
			client, err := bedrock.New(ctx)
			if err != nil {
				return err
			}
			client.Rubric = opts.rubric
			add(bedrockJudge{client}, bedrockChoice{client})
		default:
			return fmt.Errorf("unknown backend %q", name)
		}
	}
	if len(judges) == 0 {
		return fmt.Errorf("no backends selected")
	}
	judges = withMask(judges, opts.mask, func(j choiceJudge, p prep) choiceJudge { return maskedChoice{j, p} })
	if opts.rule {
		for i, j := range judges {
			judges[i] = ruleChoice{j}
		}
	}

	classes := make([]string, len(rubric.TypeOptions))
	for i, opt := range rubric.TypeOptions {
		classes[i] = opt.Name
	}
	task := "choice"
	if combined {
		task = "combined"
	}
	report := struct {
		Task     string               `json:"task"`
		Prompts  int                  `json:"prompts"`
		Backends []score.ClassSummary `json:"backends"`
	}{Task: task, Prompts: len(cases)}

	for _, j := range judges {
		fmt.Printf("warmup %s\n", j.Name())
		if _, _, _, _, err := j.choose(ctx, "Vad är den rörliga räntan på ett privatlån?"); err != nil {
			return fmt.Errorf("%s warmup: %w", j.Name(), err)
		}
		outcomes := make([]score.ClassOutcome, 0, len(cases))
		for i, c := range cases {
			start := time.Now()
			label, probability, confidence, parseError, err := j.choose(ctx, c.Text)
			elapsed := time.Since(start)
			if err != nil {
				return fmt.Errorf("%s %s: %w", j.Name(), c.ID, err)
			}
			outcomes = append(outcomes, score.ClassOutcome{
				ID:          c.ID,
				Source:      c.Source,
				Style:       c.Style,
				Reason:      c.Reason,
				Gold:        c.PIIType,
				Predicted:   label,
				Probability: probability,
				Confidence:  confidence,
				ParseError:  parseError,
				Latency:     elapsed,
				LatencyMS:   float64(elapsed.Microseconds()) / 1000,
			})
			if (i+1)%10 == 0 {
				fmt.Printf("%s %d/%d\n", j.Name(), i+1, len(cases))
			}
		}
		summary := score.SummarizeClasses(j.Name(), outcomes, classes)
		report.Backends = append(report.Backends, summary)
		printClassSummary(summary)
	}
	if opts.mask == "both" {
		printClassMaskComparison(report.Backends)
	}
	return writeReport(opts.outPath, report)
}

// combinedJudge answers none when the yes/no question says not sensitive, and otherwise the choice
// question's answer. The choice question alone files a mention of a kind of data without its value
// under that kind; the yes/no question with the value-rule rubric does not.
type combinedJudge struct {
	noul   judge
	choice choiceJudge
}

func (j combinedJudge) Name() string { return j.noul.Name() + "-combined" }

func (j combinedJudge) choose(ctx context.Context, text string) (string, *float64, *float64, string, error) {
	positive, p, parseError, err := j.noul.decide(ctx, text)
	if err != nil || parseError != "" {
		return "", p, nil, parseError, err
	}
	if !positive {
		return noneType, p, nil, "", nil
	}
	label, _, conf, parseError, err := j.choice.choose(ctx, text)
	return label, p, conf, parseError, err
}

type kevChoice struct{ *kev.Client }

func (j kevChoice) choose(ctx context.Context, text string) (string, *float64, *float64, string, error) {
	got, err := j.Choose(ctx, text)
	if err != nil {
		return "", nil, nil, "", err
	}
	p, conf := got.Probability, got.Confidence
	return got.Name, &p, &conf, "", nil
}

type strandsChoice struct{ *strands.Client }

func (j strandsChoice) choose(ctx context.Context, text string) (string, *float64, *float64, string, error) {
	got, err := j.Choose(ctx, text)
	if err != nil {
		return "", nil, nil, "", err
	}
	p, conf := got.Probability, got.Confidence
	return got.Name, &p, &conf, "", nil
}

type glinerChoice struct{ *gliner.Client }

func (j glinerChoice) choose(ctx context.Context, text string) (string, *float64, *float64, string, error) {
	got, err := j.Choose(ctx, text)
	if err != nil {
		return "", nil, nil, "", err
	}
	p, conf := got.Probability, got.Confidence
	return got.Name, &p, &conf, "", nil
}

type deciderChoice struct{ *decider.Client }

func (j deciderChoice) choose(ctx context.Context, text string) (string, *float64, *float64, string, error) {
	got, err := j.Choose(ctx, text)
	if err != nil {
		return "", nil, nil, "", err
	}
	p, conf := got.Probability, got.Confidence
	return got.Name, &p, &conf, "", nil
}

type layaChoice struct{ *laya.Client }

func (j layaChoice) choose(ctx context.Context, text string) (string, *float64, *float64, string, error) {
	got, err := j.Choose(ctx, text)
	if err != nil {
		return "", nil, nil, "", err
	}
	p, conf := got.Probability, got.Confidence
	return got.Name, &p, &conf, "", nil
}

type bedrockChoice struct{ *bedrock.Client }

func (j bedrockChoice) choose(ctx context.Context, text string) (string, *float64, *float64, string, error) {
	label, err := j.Classify(ctx, text)
	if err != nil && strings.Contains(err.Error(), "expected one of") {
		return "", nil, nil, err.Error(), nil
	}
	return label, nil, nil, "", err
}

func printClassSummary(s score.ClassSummary) {
	fmt.Printf("\n%s\n", s.Name)
	fmt.Printf("  accuracy %.3f\n", s.Accuracy)
	fmt.Printf("  latency ms  mean %.0f  p50 %.0f  p95 %.0f\n", s.LatencyMS.Mean, s.LatencyMS.P50, s.LatencyMS.P95)
	for _, class := range s.Classes {
		fmt.Printf("  %-14s %2d/%-2d  recall %.3f\n", class.Type, class.Correct, class.Support, class.Recall)
	}
	printSets(s.Sets)
	if len(s.Disagreements) == 0 {
		fmt.Printf("  disagreements: none\n\n")
		return
	}
	fmt.Printf("  disagreements: %d\n", len(s.Disagreements))
	for _, d := range s.Disagreements {
		extra := ""
		if d.Probability != nil {
			extra += fmt.Sprintf(" p=%.3f", *d.Probability)
		}
		if d.Confidence != nil {
			extra += fmt.Sprintf(" conf=%.3f", *d.Confidence)
		}
		if d.ParseError != "" {
			extra += " parse_error"
		}
		fmt.Printf("  - %s gold=%s predicted=%s%s  %s\n", d.ID, d.Gold, d.Predicted, extra, d.Reason)
	}
	fmt.Println()
}
