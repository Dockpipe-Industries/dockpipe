package pipelang

import (
	"dockpipe/src/lib/pipelang/coreeval"
	"dockpipe/src/lib/pipelang/gobackend"
	"dockpipe/tests/containedexec"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// Orthogonal continuation count and nesting depth exercise the new control-flow
// model. Each fixture retains direct compiler measurements and fresh native proof.
func TestV1140BlocksMemory(t *testing.T) {
	command := exec.Command(filepath.Join(containedexec.GoRoot(t), "bin", "go"), "list", "-export", "-f", "packagefile {{.ImportPath}}={{.Export}}", "unicode/utf8")
	imports, err := containedexec.CombinedOutput(command)
	if err != nil {
		t.Fatal(err, string(imports))
	}
	for shape, depth := range []int{1, 4, 16} {
		t.Run(fmt.Sprintf("shape%d", shape), func(t *testing.T) {
			for _, count := range []int{1, 8, 32, 256} {
				t.Run(fmt.Sprint(count), func(t *testing.T) {
					var source strings.Builder
					source.WriteString(`public Class Choices{public string Observe(string value)=>value;public string Select(bool flag){`)
					for i := 0; i < count; i++ {
						fmt.Fprintf(&source, `string local%d=Observe("local");if(flag){string branch%d=Observe("branch");}else{string branch%d=Observe("other");}`, i, i, i)
					}
					for i := 0; i < depth; i++ {
						source.WriteString(`{if(flag){string inner=Observe("nested");}`)
					}
					source.WriteString(`if(flag){return Observe("early");}`)
					for i := 0; i < depth; i++ {
						source.WriteString(`}`)
					}
					source.WriteString(`return Observe("tail");}}`)
					_, program := conditionalLocalTreeProgramVersion(t, PipeLangLanguageContractV1140, source.String(), []string{"Select"})
					generated, err := gobackend.Generate(program)
					if err != nil {
						t.Fatal(err)
					}
					if strings.Count(string(generated), `PipeLangObserve("tail")`) != 1 {
						t.Fatal("continuation duplicated")
					}
					f := coreFunctionNamed(t, program, "Select")
					for _, flag := range []bool{false, true} {
						want := "tail"
						if flag {
							want = "early"
						}
						got, err := coreeval.EvaluateProgram(program, f.Identity, []coreeval.Value{{Type: f.Parameters[0].Type, Bool: flag}})
						if err != nil || !got.OK || got.Value.String != want {
							t.Fatal(got, err)
						}
					}
					tests := []byte(`package pipelanggenerated
 import "testing"
 func TestValues(t *testing.T){if PipeLangSelect(false)!="tail"||PipeLangSelect(true)!="early"{t.Fatal("wrong block result")}}`)
					dir := t.TempDir()
					for name, data := range map[string][]byte{"generated.go": generated, "importcfg": imports} {
						if err := os.WriteFile(filepath.Join(dir, name), data, 0600); err != nil {
							t.Fatal(err)
						}
					}
					compiler := filepath.Join(containedexec.GoRoot(t), "pkg", "tool", runtime.GOOS+"_"+runtime.GOARCH, "compile")
					command := exec.Command(compiler, "-c=4", "-p", "pipelanggenerated", "-importcfg", filepath.Join(dir, "importcfg"), "-o", filepath.Join(dir, "generated.a"), filepath.Join(dir, "generated.go"))
					output, measurement, err := containedexec.Measure(command)
					if err != nil {
						t.Fatal(err, string(output))
					}
					if measurement.MaxRSSKiB > 128*1024 || measurement.Elapsed > 5*time.Second {
						t.Fatal("compiler ceiling", measurement)
					}
					if root := os.Getenv("PIPELANG_MEMORY_FIXTURES"); root != "" {
						fixture := filepath.Join(root, fmt.Sprintf("v0.114.0-blocks-shape%d-%d", shape, count))
						if err := os.MkdirAll(fixture, 0700); err != nil {
							t.Fatal(err)
						}
						metadata, _ := json.MarshalIndent(map[string]any{"version": "v0.114.0", "shape": shape, "depth": depth, "joins": count, "locals": count, "choices": shape, "branch": true, "compiler_rss_kib": measurement.MaxRSSKiB, "compiler_elapsed_s": measurement.Elapsed.Seconds()}, "", "  ")
						for name, data := range map[string][]byte{"source.pipe": []byte(source.String()), "generated.go": generated, "generated_test.go": tests, "go.mod": []byte("module general-block-scale\n\ngo 1.25\n"), "importcfg": imports, "measurement.json": metadata} {
							if err := os.WriteFile(filepath.Join(fixture, name), data, 0600); err != nil {
								t.Fatal(err)
							}
						}
					}
					compileAndRunGeneratedGoFiles(t, generated, tests)
				})
			}
		})
	}
}
