package bedrock

import "testing"

func TestParseChoice(t *testing.T) {
	allowed := []string{"none", "government_id", "payment_card", "bank_account", "secret", "named_contact"}
	got, err := ParseChoice("  government_id. ", allowed)
	if err != nil || got != "government_id" {
		t.Fatalf("got %q %v", got, err)
	}
	got, err = ParseChoice("the kind is secret", allowed)
	if err != nil || got != "secret" {
		t.Fatalf("got %q %v", got, err)
	}
	if _, err := ParseChoice("secret and payment_card", allowed); err == nil {
		t.Fatal("expected two names to be rejected")
	}
}

func TestParseLabel(t *testing.T) {
	tests := []struct {
		in      string
		want    bool
		wantErr bool
	}{
		{in: "true", want: true},
		{in: "FALSE", want: false},
		{in: "  true. \n", want: true},
		{in: "yes", wantErr: true},
		{in: "the answer is true", wantErr: true},
	}
	for _, tt := range tests {
		got, err := ParseLabel(tt.in)
		if tt.wantErr {
			if err == nil {
				t.Fatalf("%q: expected error", tt.in)
			}
			continue
		}
		if err != nil {
			t.Fatalf("%q: %v", tt.in, err)
		}
		if got != tt.want {
			t.Fatalf("%q = %v, want %v", tt.in, got, tt.want)
		}
	}
}
