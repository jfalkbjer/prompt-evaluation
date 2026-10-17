package masking

import "testing"

func TestReplaceMaskedValues(t *testing.T) {
	tests := map[string]string{
		"Personnumret är maskerat som 900101-****.":     "Personnumret är maskerat som masked.",
		`{"personnummer":"900101-****"}`:                `{"personnummer":"masked"}`,
		"Long form 19900101-XXXX on file.":              "Long form masked on file.",
		"Kortet visas som **** **** **** 4242 i appen.": "Kortet visas som masked i appen.",
		"Card ****4242 was declined.":                   "Card masked was declined.",
		"Account XXXX-XXXX-1234 is closed.":             "Account masked is closed.",
		"Masked 900101-**** and full 850505-2222.":      "Masked masked and full 850505-2222.",
	}
	for in, want := range tests {
		if got := Replace(in); got != want {
			t.Errorf("Replace(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestReplacePlaceholders(t *testing.T) {
	tests := map[string]string{
		"My governmentid is xxxx":                     "My governmentid is masked",
		"Personnummer: XXXXXX-XXXX":                   "Personnummer: masked",
		`{"iban":"[REDACTED]"}`:                       `{"iban":"masked"}`,
		"Mitt personnummer är <personnummer>.":        "Mitt personnummer är masked.",
		"Kortnummer **** **** **** **** saknas.":      "Kortnummer masked saknas.",
		"Masked 900101-**** and full 850505-2222.":    "Masked masked and full 850505-2222.",
		"Mitt personnummer är 900101-1118, tack Max.": "Mitt personnummer är 900101-1118, tack Max.",
	}
	for in, want := range tests {
		if got := ReplacePlaceholders(in); got != want {
			t.Errorf("ReplacePlaceholders(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestReplaceKeepsFullValues(t *testing.T) {
	for _, in := range []string{
		"Mitt personnummer är 900101-1118.",
		"Kortnumret är 5555 4444 3333 2222.",
		"IBAN SE45 5000 0000 0583 9825 7466.",
		"Clearing 8327-9, konto 123 456 789-0.",
		"Ticket INC-4471, amount 195 kr, ending 4242.",
	} {
		if got := Replace(in); got != in {
			t.Errorf("Replace(%q) changed a full value: %q", in, got)
		}
	}
}
