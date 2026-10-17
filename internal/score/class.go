package score

import "time"

// ClassOutcome is one PII-kind decision.
type ClassOutcome struct {
	ID          string        `json:"id"`
	Source      string        `json:"source,omitempty"`
	Style       string        `json:"style"`
	Reason      string        `json:"reason"`
	Gold        string        `json:"gold"`
	Predicted   string        `json:"predicted"`
	Probability *float64      `json:"probability,omitempty"`
	Confidence  *float64      `json:"confidence,omitempty"`
	ParseError  string        `json:"parse_error,omitempty"`
	Latency     time.Duration `json:"-"`
	LatencyMS   float64       `json:"latency_ms"`
}

// ClassCount is how often a kind was present and how often it was named correctly.
type ClassCount struct {
	Type    string  `json:"type"`
	Support int     `json:"support"`
	Correct int     `json:"correct"`
	Recall  float64 `json:"recall"`
}

// ClassSummary is one backend's PII-kind report.
type ClassSummary struct {
	Name          string         `json:"name"`
	Accuracy      float64        `json:"accuracy"`
	Classes       []ClassCount   `json:"classes"`
	LatencyMS     LatencyStats   `json:"latency_ms"`
	Sets          []SetScore     `json:"sets,omitempty"`
	Disagreements []ClassOutcome `json:"disagreements"`
}

// SummarizeClasses scores exact type matches. classes is the label order to report.
func SummarizeClasses(name string, outcomes []ClassOutcome, classes []string) ClassSummary {
	correct := 0
	support := map[string]int{}
	hits := map[string]int{}
	lat := make([]float64, 0, len(outcomes))
	var misses []ClassOutcome
	sources := make([]string, len(outcomes))
	hit := make([]bool, len(outcomes))
	for i, o := range outcomes {
		support[o.Gold]++
		sources[i] = o.Source
		hit[i] = o.Predicted == o.Gold && o.ParseError == ""
		if hit[i] {
			correct++
			hits[o.Gold]++
		} else {
			misses = append(misses, o)
		}
		lat = append(lat, o.LatencyMS)
	}
	counts := make([]ClassCount, 0, len(classes))
	for _, class := range classes {
		counts = append(counts, ClassCount{
			Type:    class,
			Support: support[class],
			Correct: hits[class],
			Recall:  ratio(hits[class], support[class]),
		})
	}
	return ClassSummary{
		Name:          name,
		Accuracy:      ratio(correct, len(outcomes)),
		Classes:       counts,
		LatencyMS:     latency(lat),
		Sets:          bySet(sources, hit),
		Disagreements: misses,
	}
}
