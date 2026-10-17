package dataset

import (
	"strings"
	"testing"
)

func TestLoad(t *testing.T) {
	cases, err := Load("../../data/prompts.json")
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range cases {
		if c.ContainsSensitivePII {
			continue
		}
		lower := strings.ToLower(c.Text)
		for _, secret := range falseCaseForbidden {
			if strings.Contains(lower, strings.ToLower(secret)) {
				t.Errorf("%s is labeled false but contains %q", c.ID, secret)
			}
		}
	}
}

func TestLoadExtra(t *testing.T) {
	for _, name := range []string{"secret_mentions", "placeholders", "bare_numbers"} {
		cases, err := LoadExtra("../../data/" + name + ".json")
		if err != nil {
			t.Fatal(err)
		}
		if len(cases) != 24 || cases[0].Source != name {
			t.Errorf("%s: %d cases, source %q", name, len(cases), cases[0].Source)
		}
	}
}

// Values that appear only in positive cases. A hit in a negative case means the label is wrong.
var falseCaseForbidden = []string{
	"900101-1118",
	"9001011118",
	"8502151234",
	"850215-1234",
	"950712-9991",
	"670923-4321",
	"880630-7777",
	"198806307777",
	"900161-1118",
	"850275-1234",
	"010101-0101",
	"740418-2468",
	"030909-1357",
	"5555 4444 3333 2222",
	"4000000000000001",
	"6221260000000004",
	"1234-5678-9012-3456",
	"1234567890123456",
	"SE99",
	"SE12",
	"Hostlov2024",
	"Höstlöv!99",
	"Vinter2024",
	"918273",
	"004812",
	"K7M-22P-QQ9",
	"erik.lindqvist@example.com",
	"sara.ek@example.com",
	"leila.nordin@example.com",
	"070-123 45 67",
	"073-555 01 92",
	"Storgatan 12",
	"Strandvägen 8",
	"AB3901847",
	"Y12345678",
	"4821",
	"9055",
	"7712",
	"394857261",
	"220198347",
	"39 48 57 261",
}
