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

// Scale preceding eager locals at each new conditional statement-test depth.
// All cases contain the new condition, including zero-local controls.
func TestV1080TerminalInnerSelectorArmsMemory(t *testing.T) {
	command := exec.Command(filepath.Join(containedexec.GoRoot(t), "bin", "go"), "list", "-export", "-f", "packagefile {{.ImportPath}}={{.Export}}", "unicode/utf8")
	imports, err := containedexec.CombinedOutput(command)
	if err != nil {
		t.Fatalf("exports: %v %s", err, imports)
	}
	for shape := 0; shape < 36; shape++ {
		for _, unused := range []bool{false, true} {
			label := fmt.Sprintf("v0.108.0/choices%d/branchtrue/straightfalse/unused%t", shape, unused)
			t.Run(label, func(t *testing.T) {
				baselineDepth := -1
				for _, count := range []int{0, 1, 8, 16, 24, 32, 64, 128, 256} {
					if !t.Run(fmt.Sprint(count), func(t *testing.T) {
						left, right := `raw+"A"`, `raw+"C"`
						if (shape%4)&1 != 0 {
							left = `trim(raw+"B")`
						}
						if (shape%4)&2 != 0 {
							right = `trim(raw+"D")`
						}
						var source strings.Builder
						fmt.Fprintf(&source, `public Class Choices {public string Select(string raw,bool outer,bool left,bool right){`)
						previous := "raw"
						for i := 0; i < count; i++ {
							name := fmt.Sprintf("q%d", i)
							l := strings.ReplaceAll(left, "raw", previous)
							r := strings.ReplaceAll(right, "raw", previous)
							fmt.Fprintf(&source, "string %s=(outer ? left : right) ? %s : %s;", name, l, r)
							if !unused || i < count-1 {
								previous = name
							}
						}
						fmt.Fprintf(&source, "return %s;}}", previous)

						body := source.String()
						start := strings.Index(body, "{") + 1
						start += strings.Index(body[start:], "{") + 1
						locals := body[start:strings.LastIndex(body, "return ")]
						trueArm, falseArm := "left", "right"
						if (shape/12+1)&1 != 0 {
							trueArm = "(left ? true : false)"
						}
						if (shape/12+1)&2 != 0 {
							falseArm = "(right ? true : false)"
						}
						test := "(outer ? " + trueArm + " : " + falseArm + ") ? true : false"
						returned := "if(" + test + "){return " + previous + ";}else{return raw;}"
						placed := body[:start] + locals + returned + "}}"
						if (shape%12)/4 == 1 {
							placed = body[:start] + "if(left){" + locals + returned + "}else{return raw;}}}"
						}
						if (shape%12)/4 == 2 {
							placed = body[:start] + "if(left){if(right){" + locals + "if(" + strings.ReplaceAll(test, "right", "!right") + "){return " + previous + ";}else{return raw;}}else{return raw;}}else{return raw;}}}"
						}
						_, program := conditionalLocalTreeProgramVersion(t, PipeLangLanguageContractV1080, placed, []string{"Select"})
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
						if count == 1 {
							baselineDepth = maximum
						}
						// Terminal placement adds zero, one or three fixed statement closures.
						// Local count must never grow the one-local depth; compiler ceilings stay fixed.
						placementCeiling := []int{6, 7, 8}[(shape%12)/4]
						if maximum > placementCeiling || (count > 0 && maximum > baselineDepth) {
							t.Fatalf("local-driven closure growth: %d", maximum)
						}
						var checks strings.Builder
						for mask := 0; mask < 8; mask++ {
							suffix := "C"
							picked := mask&4 != 0
							if mask&1 != 0 {
								picked = mask&2 != 0
							}
							if picked {
								suffix = "A"
								if (shape%4)&1 != 0 {
									suffix = "B"
								}
							} else if (shape%4)&2 != 0 {
								suffix = "D"
							}
							selected := count
							if unused && count > 0 {
								selected--
							}

							want := "raw" + strings.Repeat(suffix, selected)
							if ((shape%12)/4 == 0 && !picked) || ((shape%12)/4 == 1 && (mask&2 == 0 || !picked)) || ((shape%12)/4 == 2 && mask != 7) {
								want = "raw"
							}
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
						compiler := filepath.Join(containedexec.GoRoot(t), "pkg", "tool", runtime.GOOS+"_"+runtime.GOARCH, "compile")
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
							metadata, _ := json.MarshalIndent(map[string]any{"version": "v0.108.0", "locals": count, "choices": shape, "branch": true, "straight": false, "unused": unused, "closure_depth": maximum, "compiler_rss_kib": measurement.MaxRSSKiB, "compiler_elapsed_s": measurement.Elapsed.Seconds()}, "", "  ")
							for name, data := range map[string][]byte{"source.pipe": []byte(placed), "generated.go": generated, "generated_test.go": tests, "go.mod": []byte("module nested-terminal-test-scale\n\ngo 1.25\n"), "importcfg": imports, "measurement.json": metadata} {
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
