package main

import (
	"context"

	"prompt-evaluation/internal/numbers"
)

const ruleSuffix = "+rule"

// ruleJudge counts a text as sensitive, with probability 1, when it has a valid personnummer,
// samordningsnummer, or card number. Otherwise the model decides. The check sees the original text.
type ruleJudge struct{ judge }

func (j ruleJudge) Name() string { return j.judge.Name() + ruleSuffix }

func (j ruleJudge) decide(ctx context.Context, text string) (bool, *float64, string, error) {
	positive, p, parseError, err := j.judge.decide(ctx, text)
	if err == nil && numbers.Kind(text) != "" {
		one := 1.0
		return true, &one, "", nil
	}
	return positive, p, parseError, err
}

// ruleChoice answers government_id or payment_card when the check finds a valid number of that kind.
type ruleChoice struct{ choiceJudge }

func (j ruleChoice) Name() string { return j.choiceJudge.Name() + ruleSuffix }

func (j ruleChoice) choose(ctx context.Context, text string) (string, *float64, *float64, string, error) {
	label, p, conf, parseError, err := j.choiceJudge.choose(ctx, text)
	if kind := numbers.Kind(text); err == nil && kind != "" {
		return kind, nil, nil, "", nil
	}
	return label, p, conf, parseError, err
}

// renamed adds a suffix to a judge's report name, such as +value for the value-rule rubric.
type renamed struct {
	judge
	suffix string
}

func (j renamed) Name() string { return j.judge.Name() + j.suffix }
