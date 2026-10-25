package environment

import (
	"strings"
	"testing"
)

// TestMergeIfUnset copies keys from src into dst only when dst does not already have the key.
func TestMergeIfUnset(t *testing.T) {
	dst := map[string]string{"A": "1", "B": "2"}
	src := map[string]string{"B": "override", "C": "3"}
	MergeIfUnset(dst, src)
	if dst["A"] != "1" || dst["B"] != "2" || dst["C"] != "3" {
		t.Fatalf("unexpected merge result: %#v", dst)
	}
}

func TestMergeIfUnsetTreatsBlankAsUnset(t *testing.T) {
	dst := map[string]string{"A": "", "B": "   ", "C": "keep"}
	src := map[string]string{"A": "one", "B": "two", "C": "override", "D": "four"}
	MergeIfUnset(dst, src)
	if dst["A"] != "one" || dst["B"] != "two" || dst["C"] != "keep" || dst["D"] != "four" {
		t.Fatalf("unexpected merge result: %#v", dst)
	}
}

// TestEnvHelpers covers EnvSliceToMap, EnvironToMap, and EnvMapToSlice round-trips and edge cases.
func TestEnvHelpers(t *testing.T) {
	m := EnvSliceToMap([]string{" A = x ", "", "B=y", "BROKEN"})
	if m["A"] != "x" || m["B"] != "y" {
		t.Fatalf("EnvSliceToMap unexpected: %#v", m)
	}

	env := EnvironToMap([]string{"K=V", "X=Y=Z", "BAD"})
	if env["K"] != "V" || env["X"] != "Y=Z" {
		t.Fatalf("EnvironToMap unexpected: %#v", env)
	}

	lines := EnvMapToSlice(map[string]string{"ONE": "1", "TWO": "2"})
	joined := strings.Join(lines, "\n")
	if !strings.Contains(joined, "ONE=1") || !strings.Contains(joined, "TWO=2") {
		t.Fatalf("EnvMapToSlice unexpected: %#v", lines)
	}
}
