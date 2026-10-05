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
func TestV1150AssignmentsMemory(t *testing.T) {
	command := exec.Command(filepath.Join(runtime.GOROOT(), "bin", "go"), "list", "-export", "-f", "packagefile {{.ImportPath}}={{.Export}}", "unicode/utf8")
	imports, err := containedexec.CombinedOutput(command)
	if err != nil {
		t.Fatal(err, string(imports))
	}
	for shape, depth := range []int{1, 4, 16} {
		t.Run(fmt.Sprintf("shape%d", shape), func(t *testing.T) {
			for _, count := range []int{1, 8, 32, 256} {
				t.Run(fmt.Sprint(count), func(t *testing.T) {
					var source strings.Builder
					source.WriteString(`public Class Choices{public string Observe(string value)=>value;public string Select(bool flag){mutable string result="";`)
					for i := 0; i < count; i++ {
						switch shape {
						case 0:
							fmt.Fprintf(&source, `mutable string local%d="initial";`, i)
						case 1:
							fmt.Fprintf(&source, `mutable string local%d;`, i)
						case 2:
							fmt.Fprintf(&source, `string local%d;`, i)
						}
						fmt.Fprintf(&source, `if(flag){local%d=Observe("T");}else{local%d=Observe("F");} result=Observe(result+local%d);`, i, i, i)
					}
					for i := 0; i < depth; i++ {
						source.WriteString(`{result=Observe(result+"N");`)
					}
					for i := 0; i < depth; i++ {
						source.WriteString(`}`)
					}
					source.WriteString(`return result;}}`)
					_, program := conditionalLocalTreeProgramVersion(t, PipeLangLanguageContractV1150, source.String(), []string{"Select"})
					generated, err := gobackend.Generate(program)
					if err != nil {
						t.Fatal(err)
					}
					f := coreFunctionNamed(t, program, "Select")
					for _, flag := range []bool{false, true} {
						want := strings.Repeat("F", count) + strings.Repeat("N", depth)
						if flag {
							want = strings.Repeat("T", count) + strings.Repeat("N", depth)
						}
						got, err := coreeval.EvaluateProgram(program, f.Identity, []coreeval.Value{{Type: f.Parameters[0].Type, Bool: flag}})
						if err != nil || !got.OK || got.Value.String != want {
							t.Fatal(got, err)
						}
					}
					tests := []byte(fmt.Sprintf(`package pipelanggenerated
 import "testing"
 func TestValues(t *testing.T){if PipeLangSelect(false)!=%q||PipeLangSelect(true)!=%q{t.Fatal("wrong assignment result")}}`, strings.Repeat("F", count)+strings.Repeat("N", depth), strings.Repeat("T", count)+strings.Repeat("N", depth)))
					dir := t.TempDir()
					for name, data := range map[string][]byte{"generated.go": generated, "importcfg": imports} {
						if err := os.WriteFile(filepath.Join(dir, name), data, 0600); err != nil {
							t.Fatal(err)
						}
					}
					compiler := filepath.Join(runtime.GOROOT(), "pkg", "tool", runtime.GOOS+"_"+runtime.GOARCH, "compile")
					command := exec.Command(compiler, "-c=4", "-p", "pipelanggenerated", "-importcfg", filepath.Join(dir, "importcfg"), "-o", filepath.Join(dir, "generated.a"), filepath.Join(dir, "generated.go"))
					output, measurement, err := containedexec.Measure(command)
					if err != nil {
						t.Fatal(err, string(output))
					}
					if measurement.MaxRSSKiB > 128*1024 || measurement.Elapsed > 5*time.Second {
						t.Fatal("compiler ceiling", measurement)
					}
					if root := os.Getenv("PIPELANG_MEMORY_FIXTURES"); root != "" {
						fixture := filepath.Join(root, fmt.Sprintf("v0.115.0-assignments-shape%d-%d", shape, count))
						if err := os.MkdirAll(fixture, 0700); err != nil {
							t.Fatal(err)
						}
						metadata, _ := json.MarshalIndent(map[string]any{"version": "v0.115.0", "shape": shape, "depth": depth, "joins": count, "locals": count, "choices": shape, "branch": true, "compiler_rss_kib": measurement.MaxRSSKiB, "compiler_elapsed_s": measurement.Elapsed.Seconds()}, "", "  ")
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
