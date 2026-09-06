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

// Vary choice count independently of local count: the old all-choice scale
// fixture selected the optimized path and missed zero/one/two-choice blowups.
func TestCompilerMemoryLocalSequences(t *testing.T) {
	type family struct {
		version                  LanguageContract
		choices                  int
		branch, unused, straight bool
	}
	families := []family{
		{PipeLangLanguageContractV850, 0, false, false, true},
		{PipeLangLanguageContractV850, 1, false, false, true},
		{PipeLangLanguageContractV850, 2, false, false, true},
		{PipeLangLanguageContractV850, 3, false, false, true},
		{PipeLangLanguageContractV850, -1, false, false, true},
		{PipeLangLanguageContractV850, -1, false, true, true},
		{PipeLangLanguageContractV840, 0, false, false, false},
		{PipeLangLanguageContractV840, 1, false, false, false},
		{PipeLangLanguageContractV840, 2, false, false, false},
		{PipeLangLanguageContractV840, 3, false, false, false},
		{PipeLangLanguageContractV840, -1, false, false, false},
		{PipeLangLanguageContractV830, 0, false, false, false},
		{PipeLangLanguageContractV830, 1, false, false, false},
		{PipeLangLanguageContractV830, 2, false, false, false},
		{PipeLangLanguageContractV400, 0, false, false, true},
		{PipeLangLanguageContractV720, 0, true, true, false},
		{PipeLangLanguageContractV840, 2, true, true, false},
	}
	// Bootstrap export data separately; target code always compiles freshly.
	goBinary := filepath.Join(runtime.GOROOT(), "bin", "go")
	command := exec.Command(goBinary, "list", "-export", "-f", "packagefile {{.ImportPath}}={{.Export}}", "unicode/utf8")
	imports, err := containedexec.CombinedOutput(command)
	if err != nil {
		t.Fatalf("contained export bootstrap: %v\n%s", err, imports)
	}
	for _, f := range families {
		label := fmt.Sprintf("%s/choices%d/branch%t/straight%t/unused%t", f.version, f.choices, f.branch, f.straight, f.unused)
		// A failed family stops here; never continue increasing its source size.
		t.Run(label, func(t *testing.T) {
			for _, count := range []int{8, 16, 24, 32, 64, 128, 256} {
				ok := t.Run(fmt.Sprint(count), func(t *testing.T) {
					var source strings.Builder
					source.WriteString("public Class Choices {public string Select(string raw,bool pick,bool enabled){")
					if f.branch {
						if f.version == PipeLangLanguageContractV720 {
							source.WriteString("string root=raw;")
						}
						source.WriteString("if(enabled){")
					}
					previous := "raw"
					if f.branch && f.version == PipeLangLanguageContractV720 {
						previous = "root"
					}
					for i := 0; i < count; i++ {
						name := fmt.Sprintf("q%d", i)
						if f.choices < 0 || i < f.choices {
							fmt.Fprintf(&source, "string %s=pick ? %s+\"T\" : %s+\"F\";", name, previous, previous)
						} else {
							fmt.Fprintf(&source, "string %s=%s+\"T\";", name, previous)
						}
						if !f.unused || i < count-1 {
							previous = name
						}
					}
					if f.straight {
						fmt.Fprintf(&source, "return %s;}}", previous)
					} else if f.branch {
						fmt.Fprintf(&source, "return %s;}else{return raw;}}}", previous)
					} else {
						fmt.Fprintf(&source, "if(enabled){return %s;}else{return raw;}}}", previous)
					}
					_, program := conditionalLocalTreeProgramVersion(t, f.version, source.String(), []string{"Select"})
					function := program.Functions[0]
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
					ast.Inspect(tree, func(node ast.Node) bool {
						if node == nil {
							last := stack[len(stack)-1]
							stack = stack[:len(stack)-1]
							if last {
								depth--
							}
							return true
						}
						_, closure := node.(*ast.FuncLit)
						stack = append(stack, closure)
						if closure {
							depth++
							if depth > maximum {
								maximum = depth
							}
						}
						return true
					})
					if maximum > 3 {
						t.Fatalf("local-driven closure nesting: depth %d", maximum)
					}
					var checks strings.Builder
					for _, pick := range []bool{false, true} {
						for _, enabled := range []bool{false, true} {
							want := "raw"
							if enabled || f.straight {
								for i := 0; i < count; i++ {
									if f.unused && i == count-1 {
										continue
									}
									if !pick && (f.choices < 0 || i < f.choices) {
										want += "F"
									} else {
										want += "T"
									}
								}
							}
							args := []coreeval.Value{{Type: function.Parameters[0].Type, String: "raw"}, {Type: function.Parameters[1].Type, Bool: pick}, {Type: function.Parameters[2].Type, Bool: enabled}}
							got, err := coreeval.EvaluateProgram(program, function.Identity, args)
							if err != nil || !got.OK || got.Value.String != want {
								t.Fatalf("eval: %#v %v want %q", got, err, want)
							}
							fmt.Fprintf(&checks, "if got:=PipeLangSelect(\"raw\",%t,%t);got!=%q{t.Fatal(got)}\n", pick, enabled, want)
						}
					}
					testSource := []byte(fmt.Sprintf("package %s\nimport \"testing\"\nfunc TestSequence(t *testing.T){%s}\n", gobackend.PackageName, checks.String()))
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
						t.Fatalf("contained compiler: %v\n%s", err, output)
					}
					t.Logf("warm compiler-only locals=%d choices=%d bytes=%d depth=%d rss_KiB=%d elapsed=%s", count, f.choices, len(generated), maximum, measurement.MaxRSSKiB, measurement.Elapsed)
					if measurement.MaxRSSKiB > 128*1024 || measurement.Elapsed > 5*time.Second {
						t.Fatalf("compiler budget exceeded: %+v", measurement)
					}
					if root := os.Getenv("PIPELANG_MEMORY_FIXTURES"); root != "" {
						fixture := filepath.Join(root, strings.ReplaceAll(label, "/", "-")+fmt.Sprintf("-%d", count))
						if err := os.MkdirAll(fixture, 0700); err != nil {
							t.Fatal(err)
						}
						metadata, _ := json.MarshalIndent(map[string]any{"locals": count, "choices": f.choices, "version": f.version, "branch": f.branch, "unused": f.unused, "straight": f.straight, "source_bytes": source.Len(), "go_bytes": len(generated), "closure_depth": maximum, "compiler_rss_kib": measurement.MaxRSSKiB, "compiler_elapsed_s": measurement.Elapsed.Seconds()}, "", "  ")
						for name, data := range map[string][]byte{"source.pipe": []byte(source.String()), "generated.go": generated, "generated_test.go": testSource, "go.mod": []byte("module memory-regression\n\ngo 1.25\n"), "measurement.json": metadata, "importcfg": imports} {
							if err := os.WriteFile(filepath.Join(fixture, name), data, 0600); err != nil {
								t.Fatal(err)
							}
						}
					}
					compileAndRunGeneratedGoFiles(t, generated, testSource)
				})
				if !ok {
					break
				}
			}
		})
	}
}
