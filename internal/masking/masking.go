// Package masking replaces values that are already masked before a judge sees them.
//
// The rubric allows masked values such as 900101-**** and **** **** **** 4242,
// but models tend to read the visible digits as a real identifier. A full ID,
// card or account number elsewhere in the text is left as it is.
package masking

import "regexp"

// Placeholder is what a masked value becomes.
const Placeholder = "masked"

var (
	// A personnummer or samordningsnummer whose last four digits are hidden: 900101-****, 19900101-XXXX.
	maskedNationalID = regexp.MustCompile(`\b(?:\d{2})?\d{6}[-+]?[*xX•]{4}`)
	// A card or account number shown only by its last digits: **** **** **** 4242, ****4242, XXXX-XXXX-1234.
	maskedNumber = regexp.MustCompile(`(?:[*xX•]{4}[ -]?)+\d{2,4}\b`)

	// Placeholders that stand in for a whole value. Models otherwise read the label in front of them
	// ("personnummer", "IBAN") as the value.
	placeholders = []*regexp.Regexp{
		regexp.MustCompile(`\[(?i:redacted|removed|masked|hidden)\]`),
		regexp.MustCompile(`<[A-Za-zåäöÅÄÖ_ ]+>`),
		regexp.MustCompile(`\b[xX]{3,}(?:[- ][xX]{2,})*\b`),
		regexp.MustCompile(`\*{3,}(?:[ -]\*{2,})*`),
	}
)

// Replace returns text with every masked value swapped for Placeholder.
func Replace(text string) string {
	text = maskedNationalID.ReplaceAllString(text, Placeholder)
	return maskedNumber.ReplaceAllString(text, Placeholder)
}

// ReplacePlaceholders is Replace plus placeholders such as [REDACTED], <personnummer>, xxxx, XXXXXX-XXXX,
// and **** **** **** ****.
func ReplacePlaceholders(text string) string {
	text = Replace(text)
	for _, re := range placeholders {
		text = re.ReplaceAllString(text, Placeholder)
	}
	return text
}
