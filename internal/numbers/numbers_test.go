package numbers

import "testing"

func TestKind(t *testing.T) {
	tests := map[string]string{
		"This is not sensitive data, but if I add 197306253977 it will be.": GovernmentID,
		"Här är numret: 850709-1232":                                        GovernmentID,
		"Kan ni kolla fakturan för 8907714565?":                             GovernmentID,
		`{"tool":"lookup","ref":"19920304-5670"}`:                           GovernmentID,
		"Kan ni spärra 4111 1111 1111 1111?":                                PaymentCard,
		"The charge came from 5500-0000-0000-0004.":                         PaymentCard,
		"Kunden läste upp 3782 822463 10005.":                               PaymentCard,
		"Please check the invoice for 197306253978":                         "",
		"OCR-numret är 9876543210":                                          "",
		`{"invoiceNumber":"1234567812345678"}`:                              "",
		"Order 4111 1111 1111 1112 was cancelled.":                          "",
		"Organisationsnummer 556677-8899":                                   "",
		"Reference 731325-1234 is on the reminder letter.":                  "",
		"Kortet visas som **** **** **** 1111 i appen.":                     "",
		"Call me on 0701234567 about the invoice.":                          "",
		"Fakturadatum 2026-10-12, belopp 1 875 kr, OCR 202":                 "",
	}
	for in, want := range tests {
		if got := Kind(in); got != want {
			t.Errorf("Kind(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestLabel(t *testing.T) {
	tests := map[string]string{
		"if I add 197306253977 it will be":   "if I add [personnummer: 197306253977] it will be",
		"kolla 8907714565 och 8907714565":    "kolla [samordningsnummer: 8907714565] och [samordningsnummer: 8907714565]",
		"spärra 4111 1111 1111 1111?":        "spärra [card number: 4111 1111 1111 1111]?",
		"check 197306253978 and 556677-8899": "check 197306253978 and 556677-8899",
	}
	for in, want := range tests {
		if got := Label(in); got != want {
			t.Errorf("Label(%q) = %q, want %q", in, got, want)
		}
	}
}
