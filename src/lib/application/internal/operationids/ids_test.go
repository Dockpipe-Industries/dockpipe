package operationids

import (
	"path/filepath"
	"testing"
)

func TestBuildAndMerge(t *testing.T) {
	ids := Merge(Build(filepath.Join("tmp", "project"), " workflow "), map[string]string{
		"result": " compiled ",
		"empty":  " ",
	})
	if ids["project"] != "project" || ids["workflow"] != "workflow" || ids["result"] != "compiled" {
		t.Fatalf("ids = %#v", ids)
	}
	if _, ok := ids["empty"]; ok {
		t.Fatalf("empty id retained: %#v", ids)
	}
}

func TestMergeReturnsNilForEmptyIdentifiers(t *testing.T) {
	if got := Merge(map[string]string{" ": "value"}, map[string]string{"key": " "}); got != nil {
		t.Fatalf("Merge() = %#v, want nil", got)
	}
}
