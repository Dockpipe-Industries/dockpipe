package textvalue

import "testing"

func TestFirstNonEmpty(t *testing.T) {
	if got := FirstNonEmpty(" ", " value ", "later"); got != "value" {
		t.Fatalf("FirstNonEmpty() = %q", got)
	}
}
