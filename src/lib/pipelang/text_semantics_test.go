package pipelang

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"dockpipe/src/lib/pipelang/coreeval"
	"dockpipe/src/lib/pipelang/coreir"
	"dockpipe/src/lib/pipelang/gobackend"
	"dockpipe/src/lib/pipelang/hir"
)

func TestV080OrdinalTextOrderingHIRCoreGoAndProjection(t *testing.T) {
	source, err := os.ReadFile(filepath.Join("testdata", "text-order.pipe"))
	if err != nil {
		t.Fatal(err)
	}
	input := semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", "root.pipe", string(source))}, nil)
	input.LanguageContract = PipeLangLanguageContractV080
	analysis := AnalyzeSemanticModuleSet(input)
	if err := analysis.Error(); err != nil {
		t.Fatal(err)
	}
	method := semanticMethodNamed(t, analysis, "Before")
	if method.Identity.Callable == nil || len(method.Identity.Callable.Parameters) != 2 || method.Identity.Callable.Parameters[0].Primitive != TypeString || method.Identity.Callable.Parameters[1].Primitive != TypeString || method.Identity.Callable.Returns.Primitive != TypeBool {
		t.Fatalf("text ordering callable identity = %#v", method.Identity.Callable)
	}
	projection, err := BuildSemanticProjection(analysis)
	if err != nil {
		t.Fatal(err)
	}
	projected := projectedMemberNamed(t, projection.Modules[0].Types[0], "Before")
	if projection.LanguageContract != PipeLangLanguageContractV080 || len(projected.Parameters) != 2 || projected.Parameters[0].Type.Primitive != TypeString || projected.Parameters[1].Type.Primitive != TypeString || projected.Type.Primitive != TypeBool {
		t.Fatalf("text ordering projection = %#v", projected)
	}
	typed, err := LowerSemanticMethodToHIR(analysis, method.Identity)
	if err != nil {
		t.Fatal(err)
	}
	if typed.LanguageContract != coreir.LanguageContractV080 || len(typed.Functions) != 1 || typed.Functions[0].Body.Kind != hir.ExprBinary || typed.Functions[0].Body.Binary == nil || typed.Functions[0].Body.Binary.Operator != hir.OperatorLessThan {
		t.Fatalf("text ordering HIR = %#v", typed)
	}
	core, err := LowerHIRToCore(typed)
	if err != nil {
		t.Fatal(err)
	}
	if core.LanguageContract != coreir.LanguageContractV080 || len(core.Functions) != 1 || core.Functions[0].Body.Binary == nil || core.Functions[0].Body.Binary.Operator != coreir.OperatorLessThan {
		t.Fatalf("text ordering Core = %#v", core)
	}
	generated, err := gobackend.Generate(core)
	if err != nil {
		t.Fatal(err)
	}
	second, err := gobackend.Generate(core)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(generated, second) {
		t.Fatal("ordinal text Go output is nondeterministic")
	}
	textType := coreir.Type{Kind: coreir.TypePrimitive, Primitive: coreir.PrimitiveString}
	for _, test := range []struct {
		name        string
		left, right string
		want        bool
	}{
		{name: "ASCII", left: "alpha", right: "beta", want: true},
		{name: "prefix", left: "a", right: "aa", want: true},
		{name: "equal", left: "same", right: "same"},
		{name: "preserved normalization", left: "e\u0301", right: "\u00e9", want: true},
		{name: "scalar beyond BMP", left: "\ue000", right: "\U00010000", want: true},
	} {
		outcome, evaluateErr := coreeval.Evaluate(core.Functions[0], []coreeval.Value{{Type: textType, String: test.left}, {Type: textType, String: test.right}})
		if evaluateErr != nil || !outcome.OK || outcome.Value.Type.Kind != coreir.TypePrimitive || outcome.Value.Type.Primitive != coreir.PrimitiveBool || outcome.Value.Bool != test.want {
			t.Fatalf("%s Core outcome = %#v (%v)", test.name, outcome, evaluateErr)
		}
	}
	invalidUTF8 := string([]byte{0xff})
	if _, err := coreeval.Evaluate(core.Functions[0], []coreeval.Value{{Type: textType, String: invalidUTF8}, {Type: textType, String: "valid"}}); err == nil || !strings.Contains(err.Error(), "not valid UTF-8") {
		t.Fatalf("invalid Core text argument error = %v", err)
	}
	compileAndRunTextOrderingGo(t, generated, gobackend.FunctionName(core.Functions[0]))
	assertCompilerGolden(t, "text-order.hir.json", canonicalJSON(t, typed))
	assertCompilerGolden(t, "text-order.core.json", canonicalJSON(t, core))
	assertCompilerGolden(t, "text-order.go", generated)
}

func TestV080AdmitsExactlyFourDirectOrdinalTextOperators(t *testing.T) {
	operators := []struct {
		spelling string
		want     hir.Operator
	}{
		{spelling: "<", want: hir.OperatorLessThan},
		{spelling: "<=", want: hir.OperatorLessOrEqual},
		{spelling: ">", want: hir.OperatorGreaterThan},
		{spelling: ">=", want: hir.OperatorGreaterOrEqual},
	}
	for _, operator := range operators {
		t.Run(operator.spelling, func(t *testing.T) {
			source := fmt.Sprintf(`public Class Root { public bool Compare(string left, string right) => left %s right; }`, operator.spelling)
			input := semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", "root.pipe", source)}, nil)
			input.LanguageContract = PipeLangLanguageContractV080
			analysis := AnalyzeSemanticModuleSet(input)
			if err := analysis.Error(); err != nil {
				t.Fatal(err)
			}
			typed, err := LowerSemanticMethodToHIR(analysis, semanticMethodNamed(t, analysis, "Compare").Identity)
			if err != nil {
				t.Fatal(err)
			}
			if typed.Functions[0].Body.Binary == nil || typed.Functions[0].Body.Binary.Operator != operator.want {
				t.Fatalf("operator %q HIR = %#v", operator.spelling, typed.Functions[0].Body)
			}
			core, err := LowerHIRToCore(typed)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := gobackend.Generate(core); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestOrdinalTextOrderingRequiresExplicitV080Migration(t *testing.T) {
	for _, contract := range []LanguageContract{PipeLangLanguageContractV010, PipeLangLanguageContractV020, PipeLangLanguageContractV030, PipeLangLanguageContractV040, PipeLangLanguageContractV050, PipeLangLanguageContractV060, PipeLangLanguageContractV070} {
		t.Run(string(contract), func(t *testing.T) {
			input := semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", "root.pipe", `public Class Root { public bool Before(string left, string right) => left < right; }`)}, nil)
			input.LanguageContract = contract
			diagnostic := assertDiagnosticCode(t, AnalyzeSemanticModuleSet(input), CodeExpressionType)
			if !strings.Contains(diagnostic.Message, `invalid operand types for "<": string and string`) {
				t.Fatalf("%s text ordering diagnostic = %#v", contract, diagnostic)
			}
		})
	}
}

func TestV080OrdinalTextOrderingRejectsExcludedForms(t *testing.T) {
	tests := []struct {
		name   string
		source string
	}{
		{name: "extra parameter", source: `public Class Root { public bool Compare(string left, string right, string extra) => left < right; }`},
		{name: "reversed operands", source: `public Class Root { public bool Compare(string left, string right) => right < left; }`},
		{name: "literal operand", source: `public Class Root { public bool Compare(string left, string right) => left < "z"; }`},
		{name: "nested ordering", source: `public Class Root { public bool Compare(string left, string right) => (left < right) == true; }`},
		{name: "wrong return", source: `public Class Root { public string Compare(string left, string right) => left < right; }`},
		{name: "mixed operands", source: `public Class Root { public bool Compare(string left, int right) => left < right; }`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			input := semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", "root.pipe", test.source)}, nil)
			input.LanguageContract = PipeLangLanguageContractV080
			diagnostic := assertDiagnosticCode(t, AnalyzeSemanticModuleSet(input), CodeExpressionType)
			if diagnostic.Category != CategorySemantic {
				t.Fatalf("excluded text ordering diagnostic = %#v", diagnostic)
			}
		})
	}
}

func TestExistingTextOperationsGainCoreEvaluatorConformance(t *testing.T) {
	tests := []struct {
		name       string
		source     string
		wantString string
		wantBool   bool
	}{
		{name: "Join", source: `public Class Root { public string Join(string left, string right) => left + right; }`, wantString: "e\u0301\u00e9"},
		{name: "Equal", source: `public Class Root { public bool Equal(string left, string right) => left == right; }`},
		{name: "Different", source: `public Class Root { public bool Different(string left, string right) => left != right; }`, wantBool: true},
	}
	textType := coreir.Type{Kind: coreir.TypePrimitive, Primitive: coreir.PrimitiveString}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			analysis := AnalyzeSemanticModuleSet(semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", "root.pipe", test.source)}, nil))
			if err := analysis.Error(); err != nil {
				t.Fatal(err)
			}
			typed, err := LowerSemanticMethodToHIR(analysis, semanticMethodNamed(t, analysis, test.name).Identity)
			if err != nil {
				t.Fatal(err)
			}
			core, err := LowerHIRToCore(typed)
			if err != nil {
				t.Fatal(err)
			}
			outcome, err := coreeval.Evaluate(core.Functions[0], []coreeval.Value{{Type: textType, String: "e\u0301"}, {Type: textType, String: "\u00e9"}})
			if err != nil || !outcome.OK || outcome.Value.String != test.wantString || outcome.Value.Bool != test.wantBool {
				t.Fatalf("existing %s Core outcome = %#v (%v)", test.name, outcome, err)
			}
			generated, err := gobackend.Generate(core)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Contains(generated, []byte("pipelangCompareOrdinalText")) && test.name != "Join" {
				t.Fatalf("existing %s did not use deterministic text support:\n%s", test.name, generated)
			}
			compileAndRunExistingTextGo(t, generated, gobackend.FunctionName(core.Functions[0]), test.wantString, test.wantBool)
		})
	}
}

func TestV080PreservesFrozenArithmeticResultSemantics(t *testing.T) {
	tests := []struct {
		name, method, source string
	}{
		{name: "Add", method: "Add", source: checkedAddSource},
		{name: "Subtract", method: "Subtract", source: checkedSubtractSource},
		{name: "Multiply", method: "Multiply", source: checkedMultiplySource},
		{name: "Negate", method: "Negate", source: checkedNegateSource},
		{name: "Divide", method: "Divide", source: checkedDivideSource},
		{name: "ForwardInt", method: "Forward", source: resultTransportIntSource},
		{name: "ForwardFloat", method: "Forward", source: resultTransportFloatSource},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			input := semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", "root.pipe", test.source)}, nil)
			input.LanguageContract = PipeLangLanguageContractV080
			analysis := AnalyzeSemanticModuleSet(input)
			if err := analysis.Error(); err != nil {
				t.Fatal(err)
			}
			typed, err := LowerSemanticMethodToHIR(analysis, semanticMethodNamed(t, analysis, test.method).Identity)
			if err != nil {
				t.Fatal(err)
			}
			core, err := LowerHIRToCore(typed)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := gobackend.Generate(core); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestOrdinalTextComparisonPinsScalarOrder(t *testing.T) {
	for _, test := range []struct {
		left, right string
		want        int
	}{
		{left: "same", right: "same"},
		{left: "a", right: "aa", want: -1},
		{left: "e\u0301", right: "\u00e9", want: -1},
		{left: "\ue000", right: "\U00010000", want: -1},
		{left: "z", right: "a", want: 1},
	} {
		got, err := coreir.CompareOrdinalText(test.left, test.right)
		if err != nil || got != test.want {
			t.Fatalf("CompareOrdinalText(%q, %q) = %d, %v; want %d", test.left, test.right, got, err, test.want)
		}
	}
	if _, err := coreir.CompareOrdinalText(string([]byte{0xff}), "valid"); err == nil {
		t.Fatal("invalid UTF-8 text comparison was accepted")
	}
}

func compileAndRunTextOrderingGo(t *testing.T, generated []byte, functionName string) {
	t.Helper()
	testSource := fmt.Sprintf(`package %s

import "testing"

func TestGeneratedOrdinalText(t *testing.T) {
	for _, test := range []struct { left, right string; want bool }{
		{left: "alpha", right: "beta", want: true},
		{left: "a", right: "aa", want: true},
		{left: "e\u0301", right: "\u00e9", want: true},
		{left: "\ue000", right: "\U00010000", want: true},
		{left: "same", right: "same"},
	} {
		if got := %s(test.left, test.right); got != test.want {
			t.Fatalf("%%q < %%q = %%v, want %%v", test.left, test.right, got, test.want)
		}
	}
	defer func() {
		if recover() == nil { t.Fatal("invalid UTF-8 did not fail the generated Go boundary") }
	}()
	%s(string([]byte{0xff}), "valid")
}
`, gobackend.PackageName, functionName, functionName)
	compileAndRunGeneratedGoFiles(t, generated, []byte(testSource))
}

func compileAndRunExistingTextGo(t *testing.T, generated []byte, functionName, wantString string, wantBool bool) {
	t.Helper()
	var assertion string
	if wantString != "" {
		assertion = fmt.Sprintf(`if got := %s("e\u0301", "\u00e9"); got != %q {
		t.Fatalf("generated text result = %%q, want %%q", got, %q)
	}`, functionName, wantString, wantString)
	} else {
		assertion = fmt.Sprintf(`if got := %s("e\u0301", "\u00e9"); got != %t {
		t.Fatalf("generated text result = %%v, want %%v", got, %t)
	}`, functionName, wantBool, wantBool)
	}
	testSource := fmt.Sprintf(`package %s

import "testing"

func TestGeneratedExistingText(t *testing.T) {
	%s
}
`, gobackend.PackageName, assertion)
	compileAndRunGeneratedGoFiles(t, generated, []byte(testSource))
}
