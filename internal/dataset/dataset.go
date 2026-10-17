// Package dataset loads the labeled prompt cases.
package dataset

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"prompt-evaluation/internal/rubric"
)

// Case is one labeled prompt. Reason is for inspection and is not shown to the judges.
type Case struct {
	ID                   string `json:"id"`
	Style                string `json:"style"`
	Tricky               bool   `json:"tricky"`
	ContainsSensitivePII bool   `json:"contains_sensitive_pii"`
	PIIType              string `json:"pii_type"`
	Reason               string `json:"reason"`
	Text                 string `json:"text"`
	// Source is the set the case came from: main, or an extra set's file name without .json.
	Source string `json:"-"`
}

// MainSource is the Source of cases loaded with Load.
const MainSource = "main"

// Load reads the main set: a JSON array of cases that passes Validate.
func Load(path string) ([]Case, error) {
	cases, err := read(path, MainSource)
	if err != nil {
		return nil, err
	}
	if err := Validate(cases); err != nil {
		return nil, err
	}
	return cases, nil
}

// LoadExtra reads an extra labeled set, such as data/placeholders.json. Each case must be valid,
// but the set does not need the main set's size and balance.
func LoadExtra(path string) ([]Case, error) {
	cases, err := read(path, strings.TrimSuffix(filepath.Base(path), ".json"))
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	for _, c := range cases {
		if err := validateCase(c, seen); err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
	}
	return cases, nil
}

func read(path, source string) ([]Case, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cases []Case
	if err := json.Unmarshal(raw, &cases); err != nil {
		return nil, err
	}
	for i := range cases {
		cases[i].Source = source
	}
	return cases, nil
}

func validateCase(c Case, seen map[string]bool) error {
	if c.ID == "" || seen[c.ID] {
		return fmt.Errorf("missing or duplicate id %q", c.ID)
	}
	seen[c.ID] = true
	if c.Text == "" || c.Reason == "" || c.Style == "" || c.PIIType == "" {
		return fmt.Errorf("%s is missing text, reason, style, or pii_type", c.ID)
	}
	if !rubric.ValidType(c.PIIType) {
		return fmt.Errorf("%s has unknown pii_type %q", c.ID, c.PIIType)
	}
	if c.ContainsSensitivePII == (c.PIIType == "none") {
		return fmt.Errorf("%s label and pii_type disagree: sensitive=%t type=%s", c.ID, c.ContainsSensitivePII, c.PIIType)
	}
	return nil
}

// Validate checks the set the eval is built around: 100 cases, balanced labels,
// five styles, and a block of tricky items.
func Validate(cases []Case) error {
	if len(cases) != 100 {
		return fmt.Errorf("dataset has %d cases, want 100", len(cases))
	}
	seen := map[string]bool{}
	styles := map[string]int{}
	var positives, tricky int
	for _, c := range cases {
		if err := validateCase(c, seen); err != nil {
			return err
		}
		styles[c.Style]++
		if c.ContainsSensitivePII {
			positives++
		}
		if c.Tricky {
			tricky++
		}
	}
	if positives != 50 {
		return fmt.Errorf("got %d positive cases, want 50", positives)
	}
	if tricky < 40 {
		return fmt.Errorf("got %d tricky cases, want at least 40", tricky)
	}
	want := []string{"short_chat", "email", "json_tool", "case_note", "mixed"}
	for _, style := range want {
		if styles[style] != 20 {
			return fmt.Errorf("style %s has %d cases, want 20", style, styles[style])
		}
	}
	if len(styles) != len(want) {
		return fmt.Errorf("unexpected styles: %v", styles)
	}
	return nil
}
