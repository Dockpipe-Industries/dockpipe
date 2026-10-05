package remote

import (
	"fmt"
	"strings"
	"testing"
)

func TestResultValidationPreservesWireBounds(t *testing.T) {
	for _, status := range []string{"success", "failure", "cancelled", "unknown"} {
		result := Result{Status: status}
		if status != "success" {
			result.ExitCode = 1
		}
		if err := result.Validate(); err != nil {
			t.Fatalf("valid %s result: %v", status, err)
		}
	}
	maximum := Result{Status: "success", Log: strings.Repeat("x", MaxLog), Artifacts: map[string][]byte{"results/data.bin": make([]byte, MaxArtifacts)}}
	if err := maximum.Validate(); err != nil {
		t.Fatalf("maximum permitted result: %v", err)
	}
	tooMany := map[string][]byte{}
	for index := 0; index < 33; index++ {
		tooMany[fmt.Sprintf("result-%d", index)] = nil
	}
	for _, result := range []Result{
		{Status: "queued"},
		{Status: "success", ExitCode: 1},
		{Status: "success", Log: maximum.Log + "x"},
		{Status: "success", Artifacts: tooMany},
		{Status: "success", Artifacts: map[string][]byte{"a": maximum.Artifacts["results/data.bin"], "b": {1}}},
	} {
		if err := result.Validate(); err == nil {
			t.Fatal("accepted result outside wire bounds")
		}
	}
}

func TestArtifactPathsAreCanonicalAndRelative(t *testing.T) {
	for _, name := range []string{"", ".", "../secret", "/absolute", "a/../b", "a//b", "a/", "a\\b", "C:secret", "a\x00b", "a\nb"} {
		result := Result{Status: "success", Artifacts: map[string][]byte{name: nil}}
		if err := result.Validate(); err == nil {
			t.Fatalf("unsafe artifact path %q", name)
		}
	}
	for _, name := range []string{"result.json", "results/benchmark.bin", "results with spaces/data.bin"} {
		if !SafeRelative(name) {
			t.Fatalf("valid artifact path %q rejected", name)
		}
	}
}
