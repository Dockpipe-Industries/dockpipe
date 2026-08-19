package pipelang

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"testing"

	"dockpipe/src/lib/pipelang/coreeval"
	"dockpipe/src/lib/pipelang/coreir"
	"dockpipe/src/lib/pipelang/gobackend"
	"dockpipe/src/lib/pipelang/hir"
)

func TestV230ContainsCaseFoldedHIRCoreEvaluatorAndGo(t *testing.T) {
	analysis, typed, core, generated := v230ContainsCaseFoldedPipeline(t)
	method := semanticMethodNamed(t, analysis, "ContainsCaseFolded")
	if method.Identity.Callable == nil || len(method.Identity.Callable.Parameters) != 2 || method.Identity.Callable.Parameters[0].Primitive != TypeString || method.Identity.Callable.Parameters[1].Primitive != TypeString || method.Identity.Callable.Returns.Primitive != TypeBool {
		t.Fatalf("contains_casefolded callable identity = %#v", method.Identity.Callable)
	}
	projection, err := BuildSemanticProjection(analysis)
	if err != nil {
		t.Fatal(err)
	}
	projected := projectedMemberNamed(t, projectedTypeNamed(t, projection, "Root"), "ContainsCaseFolded")
	if projection.Schema != PipeLangSemanticProjectionVersion || projection.LanguageContract != PipeLangLanguageContractV230 || len(projected.Parameters) != 2 || projected.Parameters[0].Type.Primitive != TypeString || projected.Parameters[1].Type.Primitive != TypeString || projected.Type.Primitive != TypeBool {
		t.Fatalf("contains_casefolded projection = %#v / %#v", projection, projected)
	}
	function := typed.Functions[0]
	if typed.LanguageContract != coreir.LanguageContractV230 || function.Body.Kind != hir.ExprTextContainsCaseFolded || function.Body.TextContains == nil || function.Body.TextContains.Value == nil || function.Body.TextContains.Value.Reference == nil || function.Body.TextContains.Value.Reference.Position != 0 || function.Body.TextContains.Query == nil || function.Body.TextContains.Query.Reference == nil || function.Body.TextContains.Query.Reference.Position != 1 {
		t.Fatalf("contains_casefolded HIR = %#v", typed)
	}
	coreFunction := core.Functions[0]
	if core.LanguageContract != coreir.LanguageContractV230 || coreFunction.Body.Kind != coreir.ExprTextContainsCaseFolded || coreFunction.Body.TextContains == nil || coreFunction.Body.TextContains.Value == nil || coreFunction.Body.TextContains.Value.Parameter == nil || *coreFunction.Body.TextContains.Value.Parameter != 0 || coreFunction.Body.TextContains.Query == nil || coreFunction.Body.TextContains.Query.Parameter == nil || *coreFunction.Body.TextContains.Query.Parameter != 1 {
		t.Fatalf("contains_casefolded Core = %#v", core)
	}

	textType := coreir.Type{Kind: coreir.TypePrimitive, Primitive: coreir.PrimitiveString}
	for _, test := range []struct {
		name, value, query string
		want               bool
	}{
		{name: "ASCII", value: "Alphabet", query: "PHA", want: true},
		{name: "full-fold-expansion", value: "Straße", query: "STRASSE", want: true},
		{name: "common-mapping", value: "\u212Aelvin", query: "kel", want: true},
		{name: "full-dotted-I", value: "\u0130", query: "i\u0307", want: true},
		{name: "no-normalization", value: "e\u0301", query: "\u00e9"},
		{name: "empty-query", value: "anything", query: "", want: true},
		{name: "not-found", value: "worker", query: "api"},
	} {
		t.Run(test.name, func(t *testing.T) {
			outcome, evaluateErr := coreeval.Evaluate(coreFunction, []coreeval.Value{{Type: textType, String: test.value}, {Type: textType, String: test.query}})
			if evaluateErr != nil || !outcome.OK || outcome.Value.Type.Primitive != coreir.PrimitiveBool || outcome.Value.Bool != test.want {
				t.Fatalf("contains_casefolded outcome = %#v (%v), want %t", outcome, evaluateErr, test.want)
			}
		})
	}
	invalid := string([]byte{0xff})
	for _, arguments := range [][]coreeval.Value{
		{{Type: textType, String: invalid}, {Type: textType, String: "valid"}},
		{{Type: textType, String: "valid"}, {Type: textType, String: invalid}},
	} {
		if _, err := coreeval.Evaluate(coreFunction, arguments); err == nil || !strings.Contains(err.Error(), "not valid UTF-8") {
			t.Fatalf("contains_casefolded invalid UTF-8 error = %v", err)
		}
	}

	second, err := gobackend.Generate(core)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(generated, second) {
		t.Fatal("contains_casefolded generated Go is nondeterministic")
	}
	compileAndRunGeneratedGoFiles(t, generated, []byte(containsCaseFoldedGeneratedGoTest(gobackend.FunctionName(coreFunction))))
	assertCompilerGolden(t, "text-contains-casefolded.hir.json", canonicalJSON(t, typed))
	assertCompilerGolden(t, "text-contains-casefolded.core.json", canonicalJSON(t, core))
	assertCompilerGolden(t, "text-contains-casefolded.go", generated)
}

func TestV230ContainsCaseFoldedPinnedUnicodeContract(t *testing.T) {
	if coreir.CaseFoldingUnicodeVersion != "17.0.0" || coreir.CaseFoldingDataSHA256 != "ff8d8fefbf123574205085d6714c36149eb946d717a0c585c27f0f4ef58c4183" || len(coreir.CaseFoldingMappings()) != 1585 {
		t.Fatalf("pinned case-fold contract = %s %s %d", coreir.CaseFoldingUnicodeVersion, coreir.CaseFoldingDataSHA256, len(coreir.CaseFoldingMappings()))
	}
	folded, err := coreir.FoldCaseText("Straße \u0130 \u212A")
	if err != nil || folded != "strasse i\u0307 k" {
		t.Fatalf("full default case fold = %q (%v)", folded, err)
	}
	composed, err := coreir.ContainsCaseFoldedText("e\u0301", "\u00e9")
	if err != nil || composed {
		t.Fatalf("case folding performed normalization: %t (%v)", composed, err)
	}
}

func TestV230ParserPreservesContainsCaseFoldedOperandsAndSpan(t *testing.T) {
	const source = `public Class Root { public bool Find(string value, string query) => contains_casefolded(value, query); }`
	sources, diagnostics := NewSourceSet([]SourceInput{{Path: "root.pipe", Data: []byte(source)}})
	if diagnostics.HasErrors() {
		t.Fatal(diagnosticError(sources, diagnostics))
	}
	program, err := parseSourceFileWithLanguageContract(sources, sources.Files()[0], PipeLangLanguageContractV230)
	if err != nil {
		t.Fatal(err)
	}
	contains, ok := program.Classes[0].Methods[0].Body.(*TextContainsCaseFoldedExpr)
	if !ok {
		t.Fatalf("contains_casefolded AST = %#v", program.Classes[0].Methods[0].Body)
	}
	value, valueOK := contains.Value.(*IdentExpr)
	query, queryOK := contains.Query.(*IdentExpr)
	if !contains.Span.IsValid() || !valueOK || value.Name != "value" || !value.Span.IsValid() || !queryOK || query.Name != "query" || !query.Span.IsValid() {
		t.Fatalf("contains_casefolded AST = %#v", contains)
	}
}

func TestV230ContainsCaseFoldedRejectsExcludedForms(t *testing.T) {
	for _, test := range []struct {
		name, source string
	}{
		{name: "extra-parameter", source: `public Class Root { public bool Find(string value, string query, string extra) => contains_casefolded(value, query); }`},
		{name: "reversed-operands", source: `public Class Root { public bool Find(string value, string query) => contains_casefolded(query, value); }`},
		{name: "literal-operand", source: `public Class Root { public bool Find(string value, string query) => contains_casefolded(value, "fixed"); }`},
		{name: "nested-expression", source: `public Class Root { public bool Find(string value, string query) => contains_casefolded(value, query) == true; }`},
		{name: "wrong-return", source: `public Class Root { public string Find(string value, string query) => contains_casefolded(value, query); }`},
		{name: "non-string-parameter", source: `public Class Root { public bool Find(string value, int query) => contains_casefolded(value, query); }`},
	} {
		t.Run(test.name, func(t *testing.T) {
			input := semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", "root.pipe", test.source)}, nil)
			input.LanguageContract = PipeLangLanguageContractV230
			if analysis := AnalyzeSemanticModuleSet(input); !analysis.Diagnostics.HasErrors() {
				t.Fatal("excluded contains_casefolded form was accepted")
			}
		})
	}
}

func TestContainsCaseFoldedRequiresExplicitV230Migration(t *testing.T) {
	const source = `public Class Root { public bool Find(string value, string query) => contains_casefolded(value, query); }`
	for _, contract := range []LanguageContract{PipeLangLanguageContractV010, PipeLangLanguageContractV020, PipeLangLanguageContractV030, PipeLangLanguageContractV040, PipeLangLanguageContractV050, PipeLangLanguageContractV060, PipeLangLanguageContractV070, PipeLangLanguageContractV080, PipeLangLanguageContractV090, PipeLangLanguageContractV100, PipeLangLanguageContractV110, PipeLangLanguageContractV120, PipeLangLanguageContractV130, PipeLangLanguageContractV140, PipeLangLanguageContractV150, PipeLangLanguageContractV160, PipeLangLanguageContractV170, PipeLangLanguageContractV180, PipeLangLanguageContractV190, PipeLangLanguageContractV200, PipeLangLanguageContractV210, PipeLangLanguageContractV220} {
		input := semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", "root.pipe", source)}, nil)
		input.LanguageContract = contract
		if analysis := AnalyzeSemanticModuleSet(input); !analysis.Diagnostics.HasErrors() {
			t.Fatalf("%s implicitly accepted contains_casefolded", contract)
		}
	}
}

func TestV230ContainsCaseFoldedRejectsMalformedHIRAndCore(t *testing.T) {
	_, typed, core, _ := v230ContainsCaseFoldedPipeline(t)
	malformedHIR := typed
	malformedHIR.Functions = append([]hir.Function(nil), typed.Functions...)
	malformedHIR.Functions[0].Body = typed.Functions[0].Body
	malformedText := *typed.Functions[0].Body.TextContains
	malformedText.Query = nil
	malformedHIR.Functions[0].Body.TextContains = &malformedText
	if _, err := LowerHIRToCore(malformedHIR); err == nil {
		t.Fatal("Core lowering accepted malformed contains_casefolded HIR")
	}
	priorHIR := typed
	priorHIR.LanguageContract = coreir.LanguageContractV220
	if _, err := LowerHIRToCore(priorHIR); err == nil {
		t.Fatal("v0.22.0 HIR implicitly accepted contains_casefolded")
	}

	malformedCore := core.Functions[0]
	malformedCore.Body = core.Functions[0].Body
	malformedCoreText := *core.Functions[0].Body.TextContains
	malformedCoreText.Value, malformedCoreText.Query = malformedCoreText.Query, malformedCoreText.Value
	malformedCore.Body.TextContains = &malformedCoreText
	if err := coreir.ValidateFunction(malformedCore); err == nil {
		t.Fatal("Core accepted reversed contains_casefolded parameters")
	}
	nestedCore := core.Functions[0]
	originalBody := nestedCore.Body
	nestedCore.Body = coreir.Expr{Kind: coreir.ExprBinary, Type: originalBody.Type, Binary: &coreir.Binary{Operator: coreir.OperatorEqual, Left: &originalBody, Right: &coreir.Expr{Kind: coreir.ExprLiteral, Type: originalBody.Type, Literal: &coreir.Literal{Bool: true}}}}
	if err := coreir.ValidateFunction(nestedCore); err == nil || !strings.Contains(err.Error(), "complete function body") {
		t.Fatalf("Core nested contains_casefolded error = %v", err)
	}
	priorCore := core
	priorCore.LanguageContract = coreir.LanguageContractV220
	if _, err := gobackend.Generate(priorCore); err == nil {
		t.Fatal("v0.22.0 Core implicitly accepted contains_casefolded")
	}
}

func TestV230PreservesV220RecordListFilterByText(t *testing.T) {
	source, err := os.ReadFile("testdata/record-list-filter-by-text.pipe")
	if err != nil {
		t.Fatal(err)
	}
	input := semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", "record-list-filter-by-text.pipe", string(source))}, nil)
	input.LanguageContract = PipeLangLanguageContractV230
	analysis := AnalyzeSemanticModuleSet(input)
	if err := analysis.Error(); err != nil {
		t.Fatal(err)
	}
	typed, err := LowerSemanticMethodToHIR(analysis, semanticMethodNamed(t, analysis, "FilterRows").Identity)
	if err != nil {
		t.Fatal(err)
	}
	core, err := LowerHIRToCore(typed)
	if err != nil {
		t.Fatal(err)
	}
	if typed.Functions[0].Body.Kind != hir.ExprListFilterByText || core.Functions[0].Body.Kind != coreir.ExprListFilterByText {
		t.Fatalf("v0.23.0 did not preserve v0.22.0 filter_by: %#v / %#v", typed.Functions[0].Body, core.Functions[0].Body)
	}
	if _, err := gobackend.Generate(core); err != nil {
		t.Fatal(err)
	}
}

func v230ContainsCaseFoldedPipeline(t *testing.T) (*Analysis, hir.Program, coreir.Program, []byte) {
	t.Helper()
	source, err := os.ReadFile("testdata/text-contains-casefolded.pipe")
	if err != nil {
		t.Fatal(err)
	}
	input := semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", "text-contains-casefolded.pipe", string(source))}, nil)
	input.LanguageContract = PipeLangLanguageContractV230
	analysis := AnalyzeSemanticModuleSet(input)
	if err := analysis.Error(); err != nil {
		t.Fatal(err)
	}
	typed, err := LowerSemanticMethodToHIR(analysis, semanticMethodNamed(t, analysis, "ContainsCaseFolded").Identity)
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
	return analysis, typed, core, generated
}

func containsCaseFoldedGeneratedGoTest(functionName string) string {
	return fmt.Sprintf(`package %s

import "testing"

func expectCaseFoldPanic(t *testing.T, invoke func()) {
	t.Helper()
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic")
		}
	}()
	invoke()
}

func TestGeneratedContainsCaseFolded(t *testing.T) {
	for _, test := range []struct {
		value, query string
		want bool
	}{
		{value: "Straße", query: "STRASSE", want: true},
		{value: "İ", query: "i̇", want: true},
		{value: "é", query: "é"},
		{value: "anything", query: "", want: true},
	} {
		if got := %s(test.value, test.query); got != test.want {
			t.Fatalf("%%q contains %%q = %%t, want %%t", test.value, test.query, got, test.want)
		}
	}
	expectCaseFoldPanic(t, func() { %s(string([]byte{0xff}), "valid") })
	expectCaseFoldPanic(t, func() { %s("valid", string([]byte{0xff})) })
}
`, gobackend.PackageName, functionName, functionName, functionName)
}
