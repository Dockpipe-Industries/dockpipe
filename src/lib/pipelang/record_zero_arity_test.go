package pipelang

import (
	"dockpipe/tests/containedexec"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"dockpipe/src/lib/pipelang/coreeval"
	"dockpipe/src/lib/pipelang/gobackend"
)

// Signature classification must not index a nonexistent first parameter. The
// named-predicate capability starts at v0.31; empty-list syntax predates it.
func TestRecordZeroArityTransport(t *testing.T) {
	for version := 15; version <= 113; version++ {
		contract := LanguageContract(fmt.Sprintf("v0.%d.0", version))
		t.Run(fmt.Sprint(version), func(t *testing.T) {
			input := semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", "zero.pipe", `
public Record Row { public string Name; }
public Class Root { public List<Row> Empty() => empty_list<Row>(); }
`)}, nil)
			input.LanguageContract = contract
			analysis := AnalyzeSemanticModuleSet(input)
			if err := analysis.Error(); err != nil {
				t.Fatal(err)
			}
			typed, err := LowerSemanticMethodToHIR(analysis, semanticMethodNamed(t, analysis, "Empty").Identity)
			if err != nil {
				t.Fatal(err)
			}
			core, err := LowerHIRToCore(typed)
			if err != nil {
				t.Fatal(err)
			}
			outcome, err := coreeval.Evaluate(core.Functions[0], nil)
			if err != nil || !outcome.OK || outcome.Value.List == nil || len(outcome.Value.List) != 0 {
				t.Fatalf("empty-list oracle: %#v (%v)", outcome, err)
			}
			if _, err := BuildSemanticProjection(analysis); err != nil {
				t.Fatal(err)
			}
			generated, err := gobackend.Generate(core)
			if err != nil {
				t.Fatal(err)
			}
			if version == 30 || version == 31 || version == 113 {
				// Independent native empty-list oracle across the regression boundary.
				compileAndRunGeneratedGoFiles(t, generated, []byte(`package pipelanggenerated
import "testing"
func TestEmpty(t *testing.T) { if got := PipeLangEmpty(); got == nil || len(got) != 0 { t.Fatal(got) } }
`))
				verifyZeroArityCompiler(t, version, generated)
			}
		})
	}
}

func TestRecordZeroArityRefusal(t *testing.T) {
	for _, contract := range []LanguageContract{PipeLangLanguageContractV300, PipeLangLanguageContractV310, PipeLangLanguageContractV1130} {
		input := semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", "zero.pipe", `
public Record Row { public string Name; }
public Class Root { public Row Invalid() => new Row { Name = "constant" }; }
`)}, nil)
		input.LanguageContract = contract
		analysis := AnalyzeSemanticModuleSet(input)
		if err := analysis.Error(); err == nil {
			t.Fatal("zero-argument construction bypassed declaration-ordered signature")
		}
	}
}

func verifyZeroArityCompiler(t *testing.T, version int, generated []byte) {
	t.Helper()
	command := exec.Command(filepath.Join(runtime.GOROOT(), "bin", "go"), "list", "-export", "-f", "packagefile {{.ImportPath}}={{.Export}}", "unicode/utf8")
	imports, err := containedexec.CombinedOutput(command)
	if err != nil {
		t.Fatalf("exports: %v %s", err, imports)
	}
	dir := t.TempDir()
	if root := os.Getenv("PIPELANG_ZERO_ARITY_OUTPUT"); root != "" {
		dir = filepath.Join(root, fmt.Sprint(version))
		if err := os.MkdirAll(dir, 0700); err != nil {
			t.Fatal(err)
		}
	}
	for name, data := range map[string][]byte{"generated.go": generated, "importcfg": imports} {
		if err := os.WriteFile(filepath.Join(dir, name), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	compiler := filepath.Join(runtime.GOROOT(), "pkg", "tool", runtime.GOOS+"_"+runtime.GOARCH, "compile")
	command = exec.Command(compiler, "-c=4", "-p", "pipelanggenerated", "-importcfg", filepath.Join(dir, "importcfg"), "-o", filepath.Join(dir, "generated.a"), filepath.Join(dir, "generated.go"))
	output, measurement, err := containedexec.Measure(command)
	if err != nil {
		t.Fatalf("compiler: %v %s", err, output)
	}
	if measurement.MaxRSSKiB > 128*1024 || measurement.Elapsed > 5*time.Second {
		t.Fatalf("compiler ceiling: %#v", measurement)
	}
	t.Logf("fresh compiler: %d KiB, %s", measurement.MaxRSSKiB, measurement.Elapsed)
}
