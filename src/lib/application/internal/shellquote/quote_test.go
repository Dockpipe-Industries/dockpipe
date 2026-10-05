package shellquote

import "testing"

func TestPOSIX(t *testing.T) {
	if got := POSIX("a'b c"); got != `'a'"'"'b c'` {
		t.Fatalf("POSIX() = %q", got)
	}
}
