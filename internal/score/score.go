// Package score turns judge decisions into accuracy and latency figures.
package score

import (
	"math"
	"sort"
	"time"
)

// Outcome is one scored prompt.
type Outcome struct {
	ID          string        `json:"id"`
	Source      string        `json:"source,omitempty"`
	Style       string        `json:"style"`
	Tricky      bool          `json:"tricky"`
	Reason      string        `json:"reason"`
	Text        string        `json:"text"`
	Gold        bool          `json:"gold"`
	Predicted   bool          `json:"predicted"`
	Probability *float64      `json:"probability,omitempty"`
	ParseError  string        `json:"parse_error,omitempty"`
	Latency     time.Duration `json:"-"`
	LatencyMS   float64       `json:"latency_ms"`
}

// LatencyStats is in milliseconds.
type LatencyStats struct {
	Mean float64 `json:"mean"`
	P50  float64 `json:"p50"`
	P95  float64 `json:"p95"`
}

// Summary is one backend's report.
type Summary struct {
	Name           string       `json:"name"`
	Accuracy       float64      `json:"accuracy"`
	Precision      float64      `json:"precision"`
	Recall         float64      `json:"recall"`
	F1             float64      `json:"f1"`
	Brier          *float64     `json:"brier,omitempty"`
	TruePositives  int          `json:"true_positives"`
	FalsePositives int          `json:"false_positives"`
	TrueNegatives  int          `json:"true_negatives"`
	FalseNegatives int          `json:"false_negatives"`
	LatencyMS      LatencyStats `json:"latency_ms"`
	Sets           []SetScore   `json:"sets,omitempty"`
	Disagreements  []Outcome    `json:"disagreements"`
}

// SetScore is the accuracy on one prompt set, such as main or placeholders.
type SetScore struct {
	Source   string  `json:"source"`
	Prompts  int     `json:"prompts"`
	Correct  int     `json:"correct"`
	Accuracy float64 `json:"accuracy"`
}

// bySet scores each Source in order of first appearance. It returns nil when no outcome has a Source.
func bySet(sources []string, correct []bool) []SetScore {
	var sets []SetScore
	index := map[string]int{}
	for i, src := range sources {
		if src == "" {
			continue
		}
		k, ok := index[src]
		if !ok {
			k = len(sets)
			index[src] = k
			sets = append(sets, SetScore{Source: src})
		}
		sets[k].Prompts++
		if correct[i] {
			sets[k].Correct++
		}
	}
	for i := range sets {
		sets[i].Accuracy = ratio(sets[i].Correct, sets[i].Prompts)
	}
	return sets
}

// Summarize scores a full pass. A parse error counts as an incorrect prediction.
// Brier is set only when every outcome has a probability.
func Summarize(name string, outcomes []Outcome) Summary {
	var tp, fp, tn, fn int
	var brierSum float64
	brierN := 0
	lat := make([]float64, 0, len(outcomes))
	var misses []Outcome
	sources := make([]string, len(outcomes))
	correct := make([]bool, len(outcomes))
	for i, o := range outcomes {
		sources[i] = o.Source
		correct[i] = o.Predicted == o.Gold && o.ParseError == ""
		switch {
		case o.Gold && o.Predicted:
			tp++
		case !o.Gold && o.Predicted:
			fp++
		case o.Gold && !o.Predicted:
			fn++
		default:
			tn++
		}
		if o.Probability != nil {
			y := 0.0
			if o.Gold {
				y = 1
			}
			diff := *o.Probability - y
			brierSum += diff * diff
			brierN++
		}
		lat = append(lat, o.LatencyMS)
		if o.Predicted != o.Gold || o.ParseError != "" {
			misses = append(misses, o)
		}
	}
	prec := ratio(tp, tp+fp)
	rec := ratio(tp, tp+fn)
	s := Summary{
		Name:           name,
		Accuracy:       ratio(tp+tn, len(outcomes)),
		Precision:      prec,
		Recall:         rec,
		F1:             f1(prec, rec),
		TruePositives:  tp,
		FalsePositives: fp,
		TrueNegatives:  tn,
		FalseNegatives: fn,
		LatencyMS:      latency(lat),
		Sets:           bySet(sources, correct),
		Disagreements:  misses,
	}
	if brierN == len(outcomes) && brierN > 0 {
		b := brierSum / float64(brierN)
		s.Brier = &b
	}
	return s
}

func ratio(num, den int) float64 {
	if den == 0 {
		return 0
	}
	return float64(num) / float64(den)
}

func f1(prec, rec float64) float64 {
	if prec+rec == 0 {
		return 0
	}
	return 2 * prec * rec / (prec + rec)
}

func latency(ms []float64) LatencyStats {
	if len(ms) == 0 {
		return LatencyStats{}
	}
	sorted := append([]float64(nil), ms...)
	sort.Float64s(sorted)
	var sum float64
	for _, v := range sorted {
		sum += v
	}
	return LatencyStats{
		Mean: sum / float64(len(sorted)),
		P50:  percentile(sorted, 0.50),
		P95:  percentile(sorted, 0.95),
	}
}

// percentile uses the nearest-rank method.
func percentile(sorted []float64, p float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	rank := int(math.Ceil(p*float64(len(sorted)))) - 1
	if rank < 0 {
		rank = 0
	}
	if rank >= len(sorted) {
		rank = len(sorted) - 1
	}
	return sorted[rank]
}
