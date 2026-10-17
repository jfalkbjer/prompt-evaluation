// Command eval scores the labeled prompts with local Kev and Claude Haiku 4.5.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"prompt-evaluation/internal/bedrock"
	"prompt-evaluation/internal/dataset"
	"prompt-evaluation/internal/decider"
	"prompt-evaluation/internal/gliner"
	"prompt-evaluation/internal/kev"
	"prompt-evaluation/internal/laya"
	"prompt-evaluation/internal/rubric"
	"prompt-evaluation/internal/score"
	"prompt-evaluation/internal/strands"
)

func main() {
	dataPath := flag.String("data", "data/prompts.json", "labeled prompts")
	outPath := flag.String("out", "results/latest.json", "report path")
	kevURL := flag.String("kev-url", "http://127.0.0.1:8009", "local Kev server")
	layaURL := flag.String("laya-url", "http://127.0.0.1:8010", "local Laya server")
	strandsURL := flag.String("strands-url", "http://127.0.0.1:8011", "local Strands Decider server")
	glinerURL := flag.String("gliner-url", "http://127.0.0.1:8012", "local GLiNER2.5 Decide server")
	deciderURL := flag.String("decider-url", "http://127.0.0.1:8013", "local Mapika decider server")
	backends := flag.String("backends", "kev,bedrock", "comma-separated: kev, laya, strands, gliner, decider, bedrock")
	task := flag.String("task", "noul", "noul (sensitive or not), choice (which kind of PII), gate (sensitive unless the choice is none), or combined (which kind of PII, none unless noul says sensitive)")
	limit := flag.Int("limit", 0, "score only the first N prompts; 0 means all")
	mask := flag.String("mask", "off", "rewrite the text before the model sees it: off, on (masked values), full (also placeholders and labels on valid numbers), or both (model alone, then model + masking)")
	extra := flag.String("extra", "", "comma-separated extra labeled sets to score after -data, such as data/placeholders.json")
	rubricName := flag.String("rubric", "original", "yes/no rubric: original, or value (naming a kind of data without its value is allowed)")
	rule := flag.String("rule", "off", "on: a valid personnummer, samordningsnummer, or card number counts as sensitive whatever the model says")
	flag.Parse()

	opts := options{
		dataPath: *dataPath,
		outPath:  *outPath,
		urls:     backendURLs{kev: *kevURL, laya: *layaURL, strands: *strandsURL, gliner: *glinerURL, decider: *deciderURL},
		backends: *backends,
		limit:    *limit,
		mask:     *mask,
		rule:     *rule == "on",
	}
	if *extra != "" {
		opts.extra = strings.Split(*extra, ",")
	}
	var err error
	opts.rubric, err = rubric.ByName(*rubricName)
	if err == nil {
		err = checkMask(*mask)
	}
	if err == nil && *rule != "on" && *rule != "off" {
		err = fmt.Errorf("unknown -rule %q: want on or off", *rule)
	}
	if err == nil {
		switch *task {
		case "noul":
			err = run(opts, false)
		case "gate":
			err = run(opts, true)
		case "choice":
			err = runChoice(opts, false)
		case "combined":
			err = runChoice(opts, true)
		default:
			err = fmt.Errorf("unknown task %q", *task)
		}
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "eval: %v\n", err)
		os.Exit(1)
	}
}

type backendURLs struct {
	kev, laya, strands, gliner, decider string
}

type options struct {
	dataPath, outPath string
	extra             []string
	urls              backendURLs
	backends          string
	limit             int
	mask              string
	rubric            rubric.Noul
	rule              bool
}

// loadCases reads the main set, then each extra set, and keeps the first limit cases.
func loadCases(opts options) ([]dataset.Case, error) {
	cases, err := dataset.Load(opts.dataPath)
	if err != nil {
		return nil, err
	}
	for _, path := range opts.extra {
		more, err := dataset.LoadExtra(strings.TrimSpace(path))
		if err != nil {
			return nil, err
		}
		cases = append(cases, more...)
	}
	if opts.limit > 0 && opts.limit < len(cases) {
		cases = cases[:opts.limit]
	}
	return cases, nil
}

type judge interface {
	Name() string
	decide(ctx context.Context, text string) (positive bool, probability *float64, parseError string, err error)
}

// run scores the yes/no task. With gate set, each backend answers the PII-kind
// choice question instead, and any answer other than none counts as sensitive.
func run(opts options, gate bool) error {
	cases, err := loadCases(opts)
	if err != nil {
		return err
	}
	urls := opts.urls

	ctx := context.Background()
	var judges []judge
	// The rubric is the yes/no question, so it changes nothing in the gate task's choice question.
	add := func(noul judge, c chooser) {
		if gate {
			judges = append(judges, gateJudge{c})
		} else if opts.rubric.Name != rubric.Original.Name {
			judges = append(judges, renamed{noul, "+" + opts.rubric.Name})
		} else {
			judges = append(judges, noul)
		}
	}
	for _, name := range strings.Split(opts.backends, ",") {
		switch strings.TrimSpace(name) {
		case "kev":
			client := kev.New(urls.kev)
			client.Rubric = opts.rubric
			if err := client.Ping(ctx); err != nil {
				return err
			}
			add(kevJudge{client}, client)
		case "laya":
			client := laya.New(urls.laya)
			client.Rubric = opts.rubric
			if err := client.Ping(ctx); err != nil {
				return err
			}
			add(layaJudge{client}, client)
		case "strands":
			client := strands.New(urls.strands)
			client.Rubric = opts.rubric
			if err := client.Ping(ctx); err != nil {
				return err
			}
			add(strandsJudge{client}, client)
		case "gliner":
			client := gliner.New(urls.gliner)
			client.Rubric = opts.rubric
			if err := client.Ping(ctx); err != nil {
				return err
			}
			add(glinerJudge{client}, client)
		case "decider":
			client := decider.New(urls.decider)
			client.Rubric = opts.rubric
			if err := client.Ping(ctx); err != nil {
				return err
			}
			add(deciderJudge{client}, client)
		case "bedrock":
			client, err := bedrock.New(ctx)
			if err != nil {
				return err
			}
			client.Rubric = opts.rubric
			if gate {
				judges = append(judges, bedrockGate{client})
			} else if opts.rubric.Name != rubric.Original.Name {
				judges = append(judges, renamed{bedrockJudge{client}, "+" + opts.rubric.Name})
			} else {
				judges = append(judges, bedrockJudge{client})
			}
		default:
			return fmt.Errorf("unknown backend %q", name)
		}
	}
	if len(judges) == 0 {
		return fmt.Errorf("no backends selected")
	}
	judges = withMask(judges, opts.mask, func(j judge, p prep) judge { return maskedJudge{j, p} })
	if opts.rule {
		for i, j := range judges {
			judges[i] = ruleJudge{j}
		}
	}

	report := struct {
		Prompts  int             `json:"prompts"`
		Backends []score.Summary `json:"backends"`
	}{Prompts: len(cases)}

	for _, j := range judges {
		fmt.Printf("warmup %s\n", j.Name())
		if _, _, _, err := j.decide(ctx, "Vad är den rörliga räntan på ett privatlån?"); err != nil {
			return fmt.Errorf("%s warmup: %w", j.Name(), err)
		}
		outcomes := make([]score.Outcome, 0, len(cases))
		for i, c := range cases {
			start := time.Now()
			positive, probability, parseError, err := j.decide(ctx, c.Text)
			elapsed := time.Since(start)
			if err != nil {
				return fmt.Errorf("%s %s: %w", j.Name(), c.ID, err)
			}
			outcomes = append(outcomes, score.Outcome{
				ID:          c.ID,
				Source:      c.Source,
				Style:       c.Style,
				Tricky:      c.Tricky,
				Reason:      c.Reason,
				Text:        c.Text,
				Gold:        c.ContainsSensitivePII,
				Predicted:   positive,
				Probability: probability,
				ParseError:  parseError,
				Latency:     elapsed,
				LatencyMS:   float64(elapsed.Microseconds()) / 1000,
			})
			if (i+1)%10 == 0 {
				fmt.Printf("%s %d/%d\n", j.Name(), i+1, len(cases))
			}
		}
		summary := score.Summarize(j.Name(), outcomes)
		report.Backends = append(report.Backends, summary)
		printSummary(summary)
	}
	if opts.mask == "both" {
		printMaskComparison(report.Backends)
	}
	return writeReport(opts.outPath, report)
}

func writeReport(outPath string, report any) error {
	if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	if err := os.WriteFile(outPath, raw, 0o644); err != nil {
		return err
	}
	fmt.Printf("wrote %s\n", outPath)
	return nil
}

func printSets(sets []score.SetScore) {
	if len(sets) < 2 {
		return
	}
	fmt.Printf("  by set:")
	for _, s := range sets {
		fmt.Printf("  %s %.3f (%d/%d)", s.Source, s.Accuracy, s.Correct, s.Prompts)
	}
	fmt.Println()
}

type kevJudge struct{ *kev.Client }

func (j kevJudge) decide(ctx context.Context, text string) (bool, *float64, string, error) {
	positive, p, err := j.Decide(ctx, text)
	return positive, p, "", err
}

type layaJudge struct{ *laya.Client }

func (j layaJudge) decide(ctx context.Context, text string) (bool, *float64, string, error) {
	positive, p, err := j.Decide(ctx, text)
	return positive, p, "", err
}

type strandsJudge struct{ *strands.Client }

func (j strandsJudge) decide(ctx context.Context, text string) (bool, *float64, string, error) {
	positive, p, err := j.Decide(ctx, text)
	return positive, p, "", err
}

type glinerJudge struct{ *gliner.Client }

func (j glinerJudge) decide(ctx context.Context, text string) (bool, *float64, string, error) {
	positive, p, err := j.Decide(ctx, text)
	return positive, p, "", err
}

type deciderJudge struct{ *decider.Client }

func (j deciderJudge) decide(ctx context.Context, text string) (bool, *float64, string, error) {
	positive, p, err := j.Decide(ctx, text)
	return positive, p, "", err
}

type bedrockJudge struct{ *bedrock.Client }

func (j bedrockJudge) decide(ctx context.Context, text string) (bool, *float64, string, error) {
	positive, err := j.Decide(ctx, text)
	if err != nil && strings.Contains(err.Error(), "expected true or false") {
		return false, nil, err.Error(), nil
	}
	return positive, nil, "", err
}

type chooser interface {
	Name() string
	Choose(ctx context.Context, text string) (kev.Choice, error)
}

// gateJudge answers the yes/no task with one choice question. The probability of
// sensitive data is everything the model did not put on none.
type gateJudge struct{ chooser }

func (j gateJudge) Name() string { return j.chooser.Name() + "-gate" }

func (j gateJudge) decide(ctx context.Context, text string) (bool, *float64, string, error) {
	got, err := j.Choose(ctx, text)
	if err != nil {
		return false, nil, "", err
	}
	none, ok := got.Probabilities[noneType]
	if !ok {
		return false, nil, "", fmt.Errorf("choice response has no %q probability", noneType)
	}
	p := 1 - none
	return got.Name != noneType, &p, "", nil
}

type bedrockGate struct{ *bedrock.Client }

func (j bedrockGate) Name() string { return j.Client.Name() + "-gate" }

func (j bedrockGate) decide(ctx context.Context, text string) (bool, *float64, string, error) {
	label, err := j.Classify(ctx, text)
	if err != nil && strings.Contains(err.Error(), "expected one of") {
		return false, nil, err.Error(), nil
	}
	if err != nil {
		return false, nil, "", err
	}
	return label != noneType, nil, "", nil
}

const noneType = "none"

func printSummary(s score.Summary) {
	brier := "n/a"
	if s.Brier != nil {
		brier = fmt.Sprintf("%.3f", *s.Brier)
	}
	fmt.Printf("\n%s\n", s.Name)
	fmt.Printf("  accuracy %.3f  precision %.3f  recall %.3f  f1 %.3f  brier %s\n",
		s.Accuracy, s.Precision, s.Recall, s.F1, brier)
	fmt.Printf("  tp %d  fp %d  tn %d  fn %d\n", s.TruePositives, s.FalsePositives, s.TrueNegatives, s.FalseNegatives)
	fmt.Printf("  latency ms  mean %.0f  p50 %.0f  p95 %.0f\n", s.LatencyMS.Mean, s.LatencyMS.P50, s.LatencyMS.P95)
	printSets(s.Sets)
	if len(s.Disagreements) == 0 {
		fmt.Printf("  disagreements: none\n\n")
		return
	}
	fmt.Printf("  disagreements: %d\n", len(s.Disagreements))
	for _, d := range s.Disagreements {
		prob := ""
		if d.Probability != nil {
			prob = fmt.Sprintf(" p=%.3f", *d.Probability)
		}
		extra := ""
		if d.ParseError != "" {
			extra = " parse_error"
		}
		fmt.Printf("  - %s gold=%t predicted=%t%s%s  %s\n", d.ID, d.Gold, d.Predicted, prob, extra, d.Reason)
	}
	fmt.Println()
}
