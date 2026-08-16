package envfile

import (
	"os"
	"path/filepath"
	"testing"
)

// TestParseFile parses dotenv lines: comments, spacing, quotes, and ignores invalid lines.
func TestParseFile(t *testing.T) {
	tmp := t.TempDir()
	p := filepath.Join(tmp, ".env")
	content := `
# comment
FOO=bar
SPACED = value
QUOTED="x y"
SINGLE='z'
BADLINE
=novalue
#IGNORED=1
`
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	m, err := ParseFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if m["FOO"] != "bar" || m["SPACED"] != "value" || m["QUOTED"] != "x y" || m["SINGLE"] != "z" {
		t.Fatalf("unexpected parse result: %#v", m)
	}
	if _, ok := m["BADLINE"]; ok {
		t.Fatalf("BADLINE should be ignored: %#v", m)
	}
}

func TestParseBytes(t *testing.T) {
	m, err := ParseBytes([]byte("A=one\nB = 'two words'\nIGNORED\n"))
	if err != nil {
		t.Fatal(err)
	}
	if m["A"] != "one" || m["B"] != "two words" {
		t.Fatalf("unexpected parse result: %#v", m)
	}
}
