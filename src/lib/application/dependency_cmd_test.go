package application

import "testing"

func TestDependencyCommandNeverAdmitsUpdateOrMisplacedFlags(t *testing.T) {
	for _, arguments := range [][]string{
		{"apply"}, {"plan"}, {"inventory", "--report", "other.json"},
		{"assess", "--pin", "1.0"}, {"inventory", "--timeout-seconds", "999"},
		{"plan", "--dry-run", "--resolver", "other"},
	} {
		if err := cmdDependency(arguments); err == nil {
			t.Fatalf("unsafe or ambiguous command accepted: %v", arguments)
		}
	}
}
