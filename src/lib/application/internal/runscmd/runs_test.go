package runscmd

import (
	"reflect"
	"strings"
	"testing"
)

func TestRunRejectsUnknownSubcommand(t *testing.T) {
	err := Run([]string{"nope"})
	if err == nil || !strings.Contains(err.Error(), "list, policy, or events") {
		t.Fatalf("unexpected guidance: %v", err)
	}
}

func TestSortedRunEventIDKeys(t *testing.T) {
	if got := sortedRunEventIDKeys(map[string]string{"z": "1", "a": "2"}); !reflect.DeepEqual(got, []string{"a", "z"}) {
		t.Fatalf("keys = %#v", got)
	}
}
