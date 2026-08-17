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
	"dockpipe/src/lib/pipelang/hir"
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
	integer := coreInteger64()
	floating := coreBinary64()
	left := coreir.Expr{Kind: coreir.ExprLiteral, Type: integer, Literal: &coreir.Literal{Int: 4}}
	right := coreir.Expr{Kind: coreir.ExprLiteral, Type: integer, Literal: &coreir.Literal{Int: 2}}
	for _, operator := range []coreir.Operator{coreir.OperatorAdd, coreir.OperatorSubtract, coreir.OperatorMultiply, coreir.OperatorDivide} {
		t.Run(string(operator), func(t *testing.T) {
			resultType := integer
			if operator == coreir.OperatorDivide {
				resultType = floating
			}
			program := coreir.Program{LanguageContract: coreir.LanguageContractV010, CompilerContract: coreir.CompilerContractV1, Functions: []coreir.Function{{
				Identity: coreir.SemanticIdentity{PackageID: "test.package", Path: "app.root.calculate"}, Name: "Calculate", ReturnType: resultType,
				Body: coreir.Expr{Kind: coreir.ExprBinary, Type: resultType, Binary: &coreir.Binary{Operator: operator, Left: &left, Right: &right}},
			}}}
			_, err := gobackend.Generate(program)
			var backendErr *gobackend.Error
			if !errors.As(err, &backendErr) || backendErr.Code != "PLGO0001" || !strings.Contains(backendErr.Message, "checked Result failure semantics") {
				t.Fatalf("backend capability error = %#v (%v)", backendErr, err)
			}
		})
	}
}

func TestV010NumericPolicyRejectsImplicitConversionButLegacyRemainsFrozen(t *testing.T) {
	source := `public Class Root { public bool Compare(int left, float right) => left < right; }`
	legacy := AnalyzeFiles(map[string][]byte{"root.pipe": []byte(source)})
	if err := legacy.Error(); err != nil {
		t.Fatalf("legacy numeric policy drifted: %v", err)
	}
	analysis := AnalyzeSemanticModuleSet(semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", "root.pipe", source)}, nil))
	diagnostic := assertDiagnosticCode(t, analysis, CodeNumericSemantics)
	if diagnostic.Category != CategorySemantic || !strings.Contains(diagnostic.Message, "does not implicitly convert int and float") {
		t.Fatalf("numeric conversion diagnostic = %#v", diagnostic)
	}
}

func TestV010NumericPolicyRejectsUncheckedArithmetic(t *testing.T) {
	tests := []struct {
		name   string
		source string
	}{
		{name: "add", source: `public Class Root { public int Calculate(int left, int right) => left + right; }`},
		{name: "subtract", source: `public Class Root { public int Calculate(int left, int right) => left - right; }`},
		{name: "multiply", source: `public Class Root { public int Calculate(int left, int right) => left * right; }`},
		{name: "divide", source: `public Class Root { public float Calculate(int left, int right) => left / right; }`},
		{name: "negate", source: `public Class Root { public int Calculate(int value) => -value; }`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			analysis := AnalyzeSemanticModuleSet(semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", "root.pipe", test.source)}, nil))
			diagnostic := assertDiagnosticCode(t, analysis, CodeNumericSemantics)
			if !strings.Contains(diagnostic.Message, "checked Result failure semantics") {
				t.Fatalf("numeric arithmetic diagnostic = %#v", diagnostic)
			}
		})
	}
}

func TestGoBackendRejectsMismatchedNumericCoreOperands(t *testing.T) {
	integer := coreInteger64()
	floating := coreBinary64()
	left := coreir.Expr{Kind: coreir.ExprLiteral, Type: integer, Literal: &coreir.Literal{Int: 1}}
	right := coreir.Expr{Kind: coreir.ExprLiteral, Type: floating, Literal: &coreir.Literal{Float: 2}}
	program := coreir.Program{LanguageContract: coreir.LanguageContractV010, CompilerContract: coreir.CompilerContractV1, Functions: []coreir.Function{{
		Identity: coreir.SemanticIdentity{PackageID: "test.package", Path: "app.root.compare"}, Name: "Compare", ReturnType: coreir.Type{Kind: coreir.TypePrimitive, Primitive: coreir.PrimitiveBool},
		Body: coreir.Expr{Kind: coreir.ExprBinary, Type: coreir.Type{Kind: coreir.TypePrimitive, Primitive: coreir.PrimitiveBool}, Binary: &coreir.Binary{Operator: coreir.OperatorLessThan, Left: &left, Right: &right}},
	}}}
	_, err := gobackend.Generate(program)
	var backendErr *gobackend.Error
	if !errors.As(err, &backendErr) || backendErr.Code != "PLGO0001" || !strings.Contains(backendErr.Message, "mismatched Core operand types") {
		t.Fatalf("backend mismatch error = %#v (%v)", backendErr, err)
	}
}

func TestBinary64ComparisonAndEqualityMatchReferenceEvaluator(t *testing.T) {
	source := `public Class Root {
  public bool Less(float left, float right) => left < right;
  public bool LessEqual(float left, float right) => left <= right;
  public bool Greater(float left, float right) => left > right;
  public bool GreaterEqual(float left, float right) => left >= right;
  public bool Equal(float left, float right) => left == right;
  public bool NotEqual(float left, float right) => left != right;
}`
	analysis := AnalyzeSemanticModuleSet(semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", "root.pipe", source)}, nil))
	if err := analysis.Error(); err != nil {
		t.Fatal(err)
	}
	var typedFunctions []hir.Function
	functionNames := map[string]string{}
	for _, name := range []string{"Less", "LessEqual", "Greater", "GreaterEqual", "Equal", "NotEqual"} {
		method := semanticMethodNamed(t, analysis, name)
		typed, err := LowerSemanticMethodToHIR(analysis, method.Identity)
		if err != nil {
			t.Fatal(err)
		}
		typedFunctions = append(typedFunctions, typed.Functions[0])
	}
	core, err := LowerHIRToCore(hir.Program{LanguageContract: coreir.LanguageContractV010, CompilerContract: coreir.CompilerContractV1, Functions: typedFunctions})
	if err != nil {
		t.Fatal(err)
	}
	for _, function := range core.Functions {
		functionNames[function.Name] = gobackend.FunctionName(function)
	}
	generated, err := gobackend.Generate(core)
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		method   string
		left     string
		right    string
		expected bool
	}{
		{method: "Less", left: "NaN", right: "1", expected: false},
		{method: "LessEqual", left: "NaN", right: "NaN", expected: false},
		{method: "Greater", left: "1", right: "NaN", expected: false},
		{method: "GreaterEqual", left: "NaN", right: "1", expected: false},
		{method: "Equal", left: "NaN", right: "NaN", expected: false},
		{method: "NotEqual", left: "NaN", right: "NaN", expected: true},
		{method: "Equal", left: "0", right: "-0", expected: true},
		{method: "Less", left: "1", right: "2", expected: true},
	}
	for _, test := range cases {
		evaluated, err := InvokeFiles(map[string][]byte{"root.pipe": []byte(source)}, "Root", test.method, []string{test.left, test.right})
		if err != nil {
			t.Fatal(err)
		}
		if evaluated.Value.Type != TypeBool || evaluated.Value.Bool != test.expected {
			t.Fatalf("reference %s(%s, %s) = %#v", test.method, test.left, test.right, evaluated.Value)
		}
	}
	compileAndRunNumericConformanceGo(t, generated, functionNames)
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

func compileAndRunNumericConformanceGo(t *testing.T, generated []byte, names map[string]string) {
	t.Helper()
	testSource := fmt.Sprintf(`package %s

import (
	"math"
	"testing"
)

func TestGeneratedNumericConformance(t *testing.T) {
	nan := math.NaN()
	negativeZero := math.Copysign(0, -1)
	tests := []struct {
		name string
		got bool
		want bool
	}{
		{"nan-less", %s(nan, 1), false},
		{"nan-less-equal", %s(nan, nan), false},
		{"nan-greater", %s(1, nan), false},
		{"nan-greater-equal", %s(nan, 1), false},
		{"nan-equal", %s(nan, nan), false},
		{"nan-not-equal", %s(nan, nan), true},
		{"signed-zero-equal", %s(0, negativeZero), true},
		{"ordinary-less", %s(1, 2), true},
	}
	for _, test := range tests {
		if test.got != test.want {
			t.Fatalf("%%s: got %%v, want %%v", test.name, test.got, test.want)
		}
	}
}
`, gobackend.PackageName, names["Less"], names["LessEqual"], names["Greater"], names["GreaterEqual"], names["Equal"], names["NotEqual"], names["Equal"], names["Less"])
	compileAndRunGeneratedGoFiles(t, generated, []byte(testSource))
}

func compileAndRunGeneratedGoFiles(t *testing.T, generated, generatedTest []byte) {
	t.Helper()
	dir, err := os.MkdirTemp("/tmp", "pipelang-generated-go-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)
	files := map[string][]byte{
		"go.mod":            []byte("module pipelang-generated-check\n\ngo 1.25\n"),
		"generated.go":      generated,
		"generated_test.go": generatedTest,
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

func semanticMethodNamed(t *testing.T, analysis *Analysis, name string) SemanticDeclaration {
	t.Helper()
	for _, declaration := range analysis.SemanticIDs.Declarations() {
		if declaration.Kind == SemanticMethod && declaration.Name == name {
			return declaration
		}
	}
	t.Fatalf("missing semantic method %q", name)
	return SemanticDeclaration{}
}

func coreInteger64() coreir.Type {
	return coreir.Type{Kind: coreir.TypeNumeric, Numeric: &coreir.NumericType{Representation: coreir.NumericInteger, Bits: 64, Signed: true}}
}

func coreBinary64() coreir.Type {
	return coreir.Type{Kind: coreir.TypeNumeric, Numeric: &coreir.NumericType{Representation: coreir.NumericBinaryFloat, Bits: 64}}
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
