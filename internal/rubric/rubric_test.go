package rubric

import (
	"strings"
	"testing"
)

func TestValueRuleAddsOneLine(t *testing.T) {
	if !strings.Contains(ValueRule.Instructions, valueRuleLine+"\n\nIgnore how") {
		t.Fatalf("value rule line not inserted after the allowed list:\n%s", ValueRule.Instructions)
	}
	if got := strings.Replace(ValueRule.Instructions, valueRuleLine, "", 1); got != Instructions {
		t.Fatal("value rule changes more than the one line")
	}
}

func TestByName(t *testing.T) {
	for _, name := range []string{"original", "value"} {
		if n, err := ByName(name); err != nil || n.Name != name {
			t.Errorf("ByName(%q) = %q, %v", name, n.Name, err)
		}
	}
	if _, err := ByName("other"); err == nil {
		t.Error("expected an error for an unknown rubric")
	}
}
