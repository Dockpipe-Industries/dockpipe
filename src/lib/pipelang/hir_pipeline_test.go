package pipelang

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	goparser "go/parser"
	gotoken "go/token"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"testing"

	"dockpipe/src/lib/pipelang/coreir"
	"dockpipe/src/lib/pipelang/gobackend"
)

var _ func(coreir.Program) ([]byte, error) = gobackend.Generate

const tinyPureFunctionSource = `public Class Root { public bool Ready(int count) => count > 0; }`

func TestTinyPureFunctionLowersThroughTypedHIRCoreAndGo(t *testing.T) {
	module := testModule("app.root", "root.pipe", tinyPureFunctionSource)
	analysis := AnalyzeSemanticModuleSet(semanticTestModuleSet("app.root", []ModuleInput{module}, nil))
	if err := analysis.Error(); err != nil {
		t.Fatal(err)
	}
	method := semanticMethod(t, analysis)
	typed, err := LowerSemanticMethodToHIR(analysis, method.Identity)
	if err != nil {
		t.Fatal(err)
	}
	core, err := LowerHIRToCore(typed)
	if err != nil {
		t.Fatal(err)
	}
	generated, err := gobackend.Generate(core)
	if err != nil {
		t.Fatal(err)
	}
	coreJSON := canonicalJSON(t, core)
	for _, forbidden := range [][]byte{[]byte("root.pipe"), []byte("symbol_id"), []byte("source_span"), []byte("go_type")} {
		if bytes.Contains(coreJSON, forbidden) {
			t.Fatalf("Core IR leaked source, analysis-local, or Go concept %q:\n%s", forbidden, coreJSON)
		}
	}
	assertCompilerGolden(t, "tiny-pure-function.hir.json", canonicalJSON(t, typed))
	assertCompilerGolden(t, "tiny-pure-function.core.json", coreJSON)
	assertCompilerGolden(t, "tiny-pure-function.go", generated)

	secondGenerated, err := gobackend.Generate(core)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(generated, secondGenerated) {
		t.Fatal("Go backend output changed for identical Core IR")
	}

	evaluated, err := InvokeFiles(map[string][]byte{"root.pipe": []byte(tinyPureFunctionSource)}, "Root", "Ready", []string{"2"})
	if err != nil {
		t.Fatal(err)
	}
	if evaluated.Value.Type != TypeBool {
		t.Fatalf("reference evaluator result = %#v", evaluated.Value)
	}
	compileAndRunGeneratedGo(t, generated, gobackend.FunctionName(core.Functions[0]), evaluated.Value.Bool)
}

func TestHIRLoweringFailureRetainsStructuredDiagnostic(t *testing.T) {
	module := testModule("app.root", "root.pipe", `public Class Root { public int Count; public bool Ready(int threshold) => Count > threshold; }`)
	analysis := AnalyzeSemanticModuleSet(semanticTestModuleSet("app.root", []ModuleInput{module}, nil))
	if err := analysis.Error(); err != nil {
		t.Fatal(err)
	}
	method := semanticMethod(t, analysis)
	_, err := LowerSemanticMethodToHIR(analysis, method.Identity)
	diagnostics, ok := AsDiagnostics(err)
	if !ok || len(diagnostics) != 1 {
		t.Fatalf("HIR error = %v, diagnostics = %#v", err, diagnostics)
	}
	diagnostic := diagnostics[0]
	if diagnostic.Code != CodeHIRLowering || diagnostic.Category != CategorySemantic || diagnostic.Primary.File != "root.pipe" || !reflectSemanticIdentities(diagnostic.SemanticIdentities, []SemanticIdentity{method.Identity}) {
		t.Fatalf("HIR diagnostic = %#v", diagnostic)
	}
	first, jsonErr := DiagnosticsJSON(analysis.Sources, diagnostics)
	if jsonErr != nil {
		t.Fatal(jsonErr)
	}
	_, err = LowerSemanticMethodToHIR(analysis, method.Identity)
	secondDiagnostics, _ := AsDiagnostics(err)
	second, jsonErr := DiagnosticsJSON(analysis.Sources, secondDiagnostics)
	if jsonErr != nil {
		t.Fatal(jsonErr)
	}
	if !bytes.Equal(first, second) {
		t.Fatalf("HIR diagnostic bytes are nondeterministic:\nfirst:\n%s\nsecond:\n%s", first, second)
	}
}

func TestHIRLoweringPreservesExistingAnalysisFailure(t *testing.T) {
	module := testModule("app.root", "root.pipe", `public Class Root { public bool Ready(int threshold) => threshold + true; }`)
	analysis := AnalyzeSemanticModuleSet(semanticTestModuleSet("app.root", []ModuleInput{module}, nil))
	if len(analysis.Diagnostics) != 1 || analysis.Diagnostics[0].Code != CodeExpressionType {
		t.Fatalf("analysis diagnostics = %#v", analysis.Diagnostics)
	}
	_, err := LowerSemanticMethodToHIR(analysis, SemanticIdentity{})
	diagnostics, ok := AsDiagnostics(err)
	if !ok || len(diagnostics) != 1 || diagnostics[0].Code != CodeExpressionType || diagnostics[0].Primary != analysis.Diagnostics[0].Primary {
		t.Fatalf("lowering replaced existing diagnostic: %#v (%v)", diagnostics, err)
	}
}

func TestGoBackendRejectsUnsupportedCoreCapability(t *testing.T) {
	integer := coreir.Type{Kind: coreir.TypePrimitive, Primitive: coreir.PrimitiveInt}
	floating := coreir.Type{Kind: coreir.TypePrimitive, Primitive: coreir.PrimitiveFloat}
	left := coreir.Expr{Kind: coreir.ExprLiteral, Type: integer, Literal: &coreir.Literal{Int: 4}}
	right := coreir.Expr{Kind: coreir.ExprLiteral, Type: integer, Literal: &coreir.Literal{Int: 2}}
	program := coreir.Program{LanguageContract: coreir.LanguageContractV010, CompilerContract: coreir.CompilerContractV1, Functions: []coreir.Function{{
		Identity: coreir.SemanticIdentity{PackageID: "test.package", Path: "app.root.divide"}, Name: "Divide", ReturnType: floating,
		Body: coreir.Expr{Kind: coreir.ExprBinary, Type: floating, Binary: &coreir.Binary{Operator: coreir.OperatorDivide, Left: &left, Right: &right}},
	}}}
	_, err := gobackend.Generate(program)
	var backendErr *gobackend.Error
	if !errors.As(err, &backendErr) || backendErr.Code != "PLGO0001" || !strings.Contains(backendErr.Message, "outside the step-6 Go capability slice") {
		t.Fatalf("backend capability error = %#v (%v)", backendErr, err)
	}
}

func TestGoBackendCannotImportParserASTOrHIR(t *testing.T) {
	entries, err := os.ReadDir("gobackend")
	if err != nil {
		t.Fatal(err)
	}
	var files []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".go") && !strings.HasSuffix(entry.Name(), "_test.go") {
			files = append(files, filepath.Join("gobackend", entry.Name()))
		}
	}
	sort.Strings(files)
	if len(files) == 0 {
		t.Fatal("Go backend has no source files")
	}
	for _, path := range files {
		parsed, err := goparser.ParseFile(gotoken.NewFileSet(), path, nil, goparser.ImportsOnly)
		if err != nil {
			t.Fatal(err)
		}
		for _, imported := range parsed.Imports {
			name, err := strconv.Unquote(imported.Path.Value)
			if err != nil {
				t.Fatal(err)
			}
			if name == "go/ast" || name == "go/parser" || (strings.HasPrefix(name, "dockpipe/src/lib/pipelang") && name != "dockpipe/src/lib/pipelang/coreir") {
				t.Fatalf("%s imports forbidden parser/AST/HIR dependency %q; Go generation must consume Core IR only", path, name)
			}
		}
	}
}

func canonicalJSON(t *testing.T, value any) []byte {
	t.Helper()
	payload, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	return append(payload, '\n')
}

func assertCompilerGolden(t *testing.T, name string, actual []byte) {
	t.Helper()
	path := filepath.Join("testdata", name)
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(actual, want) {
		t.Fatalf("%s drifted; actual bytes:\n%s", path, actual)
	}
}

func compileAndRunGeneratedGo(t *testing.T, generated []byte, functionName string, expected bool) {
	t.Helper()
	dir, err := os.MkdirTemp("/tmp", "pipelang-generated-go-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)
	files := map[string][]byte{
		"go.mod":            []byte("module pipelang-generated-check\n\ngo 1.25\n"),
		"generated.go":      generated,
		"generated_test.go": []byte(fmt.Sprintf("package %s\n\nimport \"testing\"\n\nfunc TestGenerated(t *testing.T) { if got := %s(2); got != %t { t.Fatalf(\"got %%v\", got) } }\n", gobackend.PackageName, functionName, expected)),
	}
	for name, payload := range files {
		if err := os.WriteFile(filepath.Join(dir, name), payload, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	goBinary := filepath.Join(runtime.GOROOT(), "bin", "go")
	command := exec.Command(goBinary, "test", ".")
	command.Dir = dir
	command.Env = append(os.Environ(), "GOTOOLCHAIN=local", "GOPROXY=off", "GOSUMDB=off", "GOWORK=off", "GOCACHE=/tmp/dockpipe-task021-generated-gocache", "GOTMPDIR=/tmp")
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("compile/run generated Go: %v\n%s", err, output)
	}
}

func reflectSemanticIdentities(left, right []SemanticIdentity) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if semanticIdentityKey(left[index]) != semanticIdentityKey(right[index]) {
			return false
		}
	}
	return true
}
