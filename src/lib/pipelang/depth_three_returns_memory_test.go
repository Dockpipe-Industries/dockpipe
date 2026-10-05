package pipelang

import (
	"encoding/json"
	"fmt"
	"go/ast"
	goparser "go/parser"
	gotoken "go/token"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"dockpipe/src/lib/pipelang/coreeval"
	"dockpipe/src/lib/pipelang/gobackend"
	"dockpipe/tests/containedexec"
)

// Scale inherited callers independently of a fixed depth-three block helper.
func TestV930DepthThreeReturnsMemory(t *testing.T) {
	command := exec.Command(filepath.Join(runtime.GOROOT(), "bin", "go"), "list", "-export", "-f", "packagefile {{.ImportPath}}={{.Export}}", "unicode/utf8")
	imports, err := containedexec.CombinedOutput(command)
	if err != nil {
		t.Fatalf("exports: %v %s", err, imports)
	}
	for shape := 0; shape < 4; shape++ {
		for _, unused := range []bool{false, true} {
			label := fmt.Sprintf("v0.93.0/choices%d/branchfalse/straighttrue/unused%t", shape, unused)
			t.Run(label, func(t *testing.T) {
				for _, count := range []int{0, 1, 8, 16, 32, 64, 128, 256} {
					if !t.Run(fmt.Sprint(count), func(t *testing.T) {
						left, right := `raw+"A"`, `raw+"C"`
						if shape&1 != 0 {
							left = `left ? raw+"A" : raw+"B"`
						}
						if shape&2 != 0 {
							right = `right ? raw+"C" : raw+"D"`
						}
						var source strings.Builder
						fmt.Fprintf(&source, `public Class Choices {public string Choose(string raw,bool outer,bool left,bool right){return outer ? (left ? (%s) : (%s)) : (right ? (%s) : (%s));}public string Select(string raw,bool outer,bool left,bool right){`, left, left, right, right)
						previous := "raw"
						for i := 0; i < count; i++ {
							name := fmt.Sprintf("q%d", i)
							fmt.Fprintf(&source, "string %s=Choose(%s,outer,left,right);", name, previous)
							if !unused || i < count-1 {
								previous = name
							}
						}
						// The zero-local case uses the inherited arrow-call spelling.
						if count == 0 {
							text := source.String()
							source.Reset()
							source.WriteString(strings.TrimSuffix(text, "{") + "=>Choose(raw,outer,left,right);}")
						} else {
							fmt.Fprintf(&source, "return %s;}}", previous)
						}
						_, program := conditionalLocalTreeProgramVersion(t, PipeLangLanguageContractV930, source.String(), []string{"Select"})
						f := coreFunctionNamed(t, program, "Select")
						generated, err := gobackend.Generate(program)
						if err != nil {
							t.Fatal(err)
						}
						tree, err := goparser.ParseFile(gotoken.NewFileSet(), "generated.go", generated, 0)
						if err != nil {
							t.Fatal(err)
						}
						depth, maximum := 0, 0
						stack := []bool{}
						ast.Inspect(tree, func(n ast.Node) bool {
							if n == nil {
								last := stack[len(stack)-1]
								stack = stack[:len(stack)-1]
								if last {
									depth--
								}
								return true
							}
							_, closure := n.(*ast.FuncLit)
							stack = append(stack, closure)
							if closure {
								depth++
								maximum = max(maximum, depth)
							}
							return true
						})
						if maximum > 3 {
							t.Fatalf("local-driven closure growth: %d", maximum)
						}
						var checks strings.Builder
						for mask := 0; mask < 8; mask++ {
							suffix := "C"
							if mask&1 != 0 {
								suffix = "A"
								if shape&1 != 0 && mask&2 == 0 {
									suffix = "B"
								}
							} else if shape&2 != 0 && mask&4 == 0 {
								suffix = "D"
							}
							selected := count
							if unused && count > 0 {
								selected--
							}
							if count == 0 {
								selected = 1
							}
							want := "raw" + strings.Repeat(suffix, selected)
							args := []coreeval.Value{{Type: f.Parameters[0].Type, String: "raw"}}
							for bit := 0; bit < 3; bit++ {
								args = append(args, coreeval.Value{Type: f.Parameters[bit+1].Type, Bool: mask&(1<<bit) != 0})
							}
							got, err := coreeval.EvaluateProgram(program, f.Identity, args)
							if err != nil || !got.OK || got.Value.String != want {
								t.Fatalf("%d: %#v %v want %q", mask, got, err, want)
							}
							fmt.Fprintf(&checks, "if got:=PipeLangSelect(\"raw\",%t,%t,%t);got!=%q{t.Fatal(got)}\n", mask&1 != 0, mask&2 != 0, mask&4 != 0, want)
						}
						tests := []byte(fmt.Sprintf("package %s\nimport \"testing\"\nfunc TestCalls(t *testing.T){%s}", gobackend.PackageName, checks.String()))
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
							t.Fatalf("compiler: %v %s", err, output)
						}
						if measurement.MaxRSSKiB > 128*1024 || measurement.Elapsed > 5*time.Second {
							t.Fatalf("compiler ceiling: %+v", measurement)
						}
						if root := os.Getenv("PIPELANG_MEMORY_FIXTURES"); root != "" {
							fixture := filepath.Join(root, strings.ReplaceAll(label, "/", "-")+fmt.Sprintf("-%d", count))
							if err := os.MkdirAll(fixture, 0700); err != nil {
								t.Fatal(err)
							}
							metadata, _ := json.MarshalIndent(map[string]any{"version": "v0.93.0", "locals": count, "choices": shape, "branch": false, "straight": true, "unused": unused, "closure_depth": maximum, "compiler_rss_kib": measurement.MaxRSSKiB, "compiler_elapsed_s": measurement.Elapsed.Seconds()}, "", "  ")
							for name, data := range map[string][]byte{"source.pipe": []byte(source.String()), "generated.go": generated, "generated_test.go": tests, "go.mod": []byte("module depth-three-scale\n\ngo 1.25\n"), "importcfg": imports, "measurement.json": metadata} {
								if err := os.WriteFile(filepath.Join(fixture, name), data, 0600); err != nil {
									t.Fatal(err)
								}
							}
						}
						compileAndRunGeneratedGoFiles(t, generated, tests)
					}) {
						break
					}
				}
			})
		}
	}
}
