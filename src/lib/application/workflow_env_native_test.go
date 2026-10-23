package application

import (
	"runtime"
	"strings"
	"testing"

	"dockpipe/src/lib/domain"
)

func TestAppendUniqueEnvironmentPreservesNativeOverride(t *testing.T) {
	environment := appendUniqueEnv([]string{"Path=inherited", "PATH=duplicate", "KEEP=private exact value"}, "PATH=explicit")
	paths := 0
	for _, entry := range environment {
		key, value, _ := strings.Cut(entry, "=")
		matches := key == "PATH"
		if runtime.GOOS == "windows" {
			matches = strings.EqualFold(key, "PATH")
		}
		if matches {
			paths++
			if value != "explicit" {
				t.Fatal("native explicit override was discarded")
			}
		}
	}
	if paths != 1 {
		t.Fatal("duplicate native environment key retained")
	}
	if runtime.GOOS != "windows" && environment[0] != "Path=inherited" {
		t.Fatal("Unix key case was conflated")
	}
}

// The workflow map also feeds container steps. Authored names must retain their
// case even when the host itself has case-insensitive native environment keys.
func TestCLIEnvironmentNamesKeepAuthoredCase(t *testing.T) {
	environment := map[string]string{"PATH": "inherited", "MixedCase": "variable"}
	MergeExtraEnvCLIIntoSteps(environment, []string{"Path=explicit", "lowerName=value", "MixedCase=environment"}, []string{"MixedCase=variable"})
	if environment["PATH"] != "inherited" || environment["Path"] != "explicit" || environment["lowerName"] != "value" || environment["MixedCase"] != "variable" {
		t.Fatal("authored container environment name or explicit precedence changed")
	}
	if _, found := environment["LOWERNAME"]; found {
		t.Fatal("authored environment key was uppercased")
	}
	directory := t.TempDir()
	environment = make(map[string]string)
	options := &CliOpts{VarOverrides: []string{"MixedCase=variable"}}
	if err := buildWorkflowEnvInto(environment, &domain.Workflow{}, "", directory, directory, options); err != nil {
		t.Fatal(err)
	}
	if environment["MixedCase"] != "variable" {
		t.Fatal("authored variable key case changed")
	}
	if _, found := environment["MIXEDCASE"]; found {
		t.Fatal("authored variable name was uppercased")
	}
}
