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

// Independent enum/member and consecutive-local dimensions retain direct
// compiler measurements and fresh native value oracles for every fixture.
func TestV1130EnumsMemory(t *testing.T) {
	command := exec.Command(filepath.Join(containedexec.GoRoot(t), "bin", "go"), "list", "-export", "-f", "packagefile {{.ImportPath}}={{.Export}}", "unicode/utf8")
	imports, err := containedexec.CombinedOutput(command)
	if err != nil {
		t.Fatalf("exports: %v %s", err, imports)
	}
	for shape, members := range []int{2, 8, 32} {
		label := fmt.Sprintf("v0.113.0/choices%d", shape)
		t.Run(label, func(t *testing.T) {
			baselineDepth := -1
			for _, count := range []int{0, 1, 8, 32, 128, 256} {
				if !t.Run(fmt.Sprint(count), func(t *testing.T) {
					var source strings.Builder
					source.WriteString("public Enum Mode{")
					for i := 0; i < members; i++ {
						fmt.Fprintf(&source, "M%d=\"tag-%03d\";", i, i)
					}
					source.WriteString("} public Class Choices{public Mode Echo(Mode value)=>value;public string Select(Mode value,bool flag){")
					previous := "value"
					for i := 0; i < count; i++ {
						fmt.Fprintf(&source, "Mode q%d=flag ? Echo(%s) : Mode.M%d;", i, previous, i%members)
						previous = fmt.Sprintf("q%d", i)
					}
					fmt.Fprintf(&source, "return match(Echo(%s)){", previous)
					for i := 0; i < members; i++ {
						if i != 0 {
							source.WriteString(",")
						}
						fmt.Fprintf(&source, "Mode.M%d=>\"result-%d\"", i, i)
					}
					source.WriteString("};}}")
					if count == 0 {
						raw := source.String()
						raw = strings.Replace(raw, "Select(Mode value,bool flag){return ", "Select(Mode value,bool flag)=>", 1)
						raw = strings.TrimSuffix(raw, "};}}") + "};}"
						source.Reset()
						source.WriteString(raw)
					}
					_, program := conditionalLocalTreeProgramVersion(t, PipeLangLanguageContractV1130, source.String(), []string{"Select"})
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
					if count > 1 && maximum > baselineDepth {
						t.Fatalf("local-driven closure growth: %d", maximum)
					}
					prepared, err := coreeval.PrepareProgram(program)
					if err != nil {
						t.Fatal(err)
					}
					var checks strings.Builder
					for input := 0; input < members; input++ {
						for _, flag := range []bool{false, true} {
							selected := input
							if !flag && count > 0 {
								selected = (count - 1) % members
							}
							want := fmt.Sprintf("result-%d", selected)
							tag := fmt.Sprintf("tag-%03d", input)
							got, err := prepared.Evaluate(f.Identity, []coreeval.Value{{Type: f.Parameters[0].Type, String: tag}, {Type: f.Parameters[1].Type, Bool: flag}})
							if err != nil || !got.OK || got.Value.String != want {
								t.Fatalf("%s %t: %#v %v want %q", tag, flag, got, err, want)
							}
							fmt.Fprintf(&checks, "if got:=PipeLangSelect(%q,%t);got!=%q{t.Fatal(got)}\n", tag, flag, want)
						}
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
						metadata, _ := json.MarshalIndent(map[string]any{"version": "v0.113.0", "locals": count, "choices": shape, "members": members, "branch": false, "straight": true, "unused": false, "closure_depth": maximum, "compiler_rss_kib": measurement.MaxRSSKiB, "compiler_elapsed_s": measurement.Elapsed.Seconds()}, "", "  ")
						for name, data := range map[string][]byte{"source.pipe": []byte(source.String()), "generated.go": generated, "generated_test.go": tests, "go.mod": []byte("module nominal-enum-scale\n\ngo 1.25\n"), "importcfg": imports, "measurement.json": metadata} {
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
