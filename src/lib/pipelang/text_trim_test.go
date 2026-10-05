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

func TestV260TextTrimHIRCoreEvaluatorAndGo(t *testing.T) {
	analysis, typed, core, generated := v260TextTrimPipeline(t)
	method := semanticMethodNamed(t, analysis, "Trim")
	if method.Identity.Callable == nil || len(method.Identity.Callable.Parameters) != 1 || method.Identity.Callable.Parameters[0].Primitive != TypeString || method.Identity.Callable.Returns.Primitive != TypeString {
		t.Fatalf("trim callable identity = %#v", method.Identity.Callable)
	}
	projection, err := BuildSemanticProjection(analysis)
	if err != nil {
		t.Fatal(err)
	}
	projected := projectedMemberNamed(t, projectedTypeNamed(t, projection, "Root"), "Trim")
	if projection.Schema != PipeLangSemanticProjectionVersion || projection.LanguageContract != PipeLangLanguageContractV260 || len(projected.Parameters) != 1 || projected.Parameters[0].Type.Primitive != TypeString || projected.Type.Primitive != TypeString {
		t.Fatalf("trim projection = %#v / %#v", projection, projected)
	}
	function := typed.Functions[0]
	if typed.LanguageContract != coreir.LanguageContractV260 || function.Body.Kind != hir.ExprTextTrim || function.Body.TextTrim == nil || function.Body.TextTrim.Value == nil || function.Body.TextTrim.Value.Reference == nil || function.Body.TextTrim.Value.Reference.Position != 0 {
		t.Fatalf("trim HIR = %#v", typed)
	}
	coreFunction := core.Functions[0]
	if core.LanguageContract != coreir.LanguageContractV260 || coreFunction.Body.Kind != coreir.ExprTextTrim || coreFunction.Body.TextTrim == nil || coreFunction.Body.TextTrim.Value == nil || coreFunction.Body.TextTrim.Value.Parameter == nil || *coreFunction.Body.TextTrim.Value.Parameter != 0 {
		t.Fatalf("trim Core = %#v", core)
	}

	textType := coreir.Type{Kind: coreir.TypePrimitive, Primitive: coreir.PrimitiveString}
	for _, test := range []struct {
		name, value, want string
	}{
		{name: "ASCII", value: "\t  launcher \r\n", want: "launcher"},
		{name: "Unicode", value: "\u1680\u2000docker\u205f\u3000", want: "docker"},
		{name: "interior-preserved", value: " \talpha\u00a0beta\n ", want: "alpha\u00a0beta"},
		{name: "all-whitespace", value: "\u0085\u2028\u2029", want: ""},
		{name: "empty", value: "", want: ""},
		{name: "zero-width-space-is-not-whitespace", value: "\u200bvalue\u200b", want: "\u200bvalue\u200b"},
		{name: "bom-is-not-whitespace", value: "\ufeffvalue\ufeff", want: "\ufeffvalue\ufeff"},
	} {
		t.Run(test.name, func(t *testing.T) {
			outcome, evaluateErr := coreeval.Evaluate(coreFunction, []coreeval.Value{{Type: textType, String: test.value}})
			if evaluateErr != nil || !outcome.OK || outcome.Value.Type.Primitive != coreir.PrimitiveString || outcome.Value.String != test.want {
				t.Fatalf("trim outcome = %#v (%v), want %q", outcome, evaluateErr, test.want)
			}
		})
	}
	invalid := string([]byte{0xff})
	if _, err := coreeval.Evaluate(coreFunction, []coreeval.Value{{Type: textType, String: invalid}}); err == nil || !strings.Contains(err.Error(), "not valid UTF-8") {
		t.Fatalf("trim invalid UTF-8 error = %v", err)
	}

	second, err := gobackend.Generate(core)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(generated, second) {
		t.Fatal("trim generated Go is nondeterministic")
	}
	compileAndRunGeneratedGoFiles(t, generated, []byte(textTrimGeneratedGoTest(gobackend.FunctionName(coreFunction))))
	assertCompilerGolden(t, "text-trim.hir.json", canonicalJSON(t, typed))
	assertCompilerGolden(t, "text-trim.core.json", canonicalJSON(t, core))
	assertCompilerGolden(t, "text-trim.go", generated)
}

func TestV260TextTrimPinnedUnicodeContract(t *testing.T) {
	ranges := coreir.TextWhitespaceRanges()
	if coreir.TextWhitespaceUnicodeVersion != "17.0.0" || len(ranges) != 10 {
		t.Fatalf("pinned whitespace contract = %s %#v", coreir.TextWhitespaceUnicodeVersion, ranges)
	}
	count := 0
	for _, scalarRange := range ranges {
		if scalarRange.First > scalarRange.Last {
			t.Fatalf("invalid whitespace range %#v", scalarRange)
		}
		count += int(scalarRange.Last-scalarRange.First) + 1
		for scalar := scalarRange.First; scalar <= scalarRange.Last; scalar++ {
			trimmed, err := coreir.TrimText(string(scalar) + "x" + string(scalar))
			if err != nil || trimmed != "x" {
				t.Fatalf("U+%04X was not trimmed: %q (%v)", scalar, trimmed, err)
			}
		}
	}
	if count != 25 {
		t.Fatalf("Unicode White_Space scalar count = %d, want 25", count)
	}
	ranges[0].First = 0
	if pinned := coreir.TextWhitespaceRanges(); pinned[0].First != 0x0009 {
		t.Fatalf("caller mutated pinned whitespace table: %#v", pinned[0])
	}
	for _, scalar := range []rune{0x0008, 0x000E, 0x180E, 0x200B, 0xFEFF} {
		value := string(scalar) + "x" + string(scalar)
		trimmed, err := coreir.TrimText(value)
		if err != nil || trimmed != value {
			t.Fatalf("non-White_Space U+%04X changed: %q (%v)", scalar, trimmed, err)
		}
	}
}

func TestV260ParserPreservesTextTrimOperandAndSpan(t *testing.T) {
	const source = `public Class Root { public string Clean(string value) => trim(value); }`
	sources, diagnostics := NewSourceSet([]SourceInput{{Path: "root.pipe", Data: []byte(source)}})
	if diagnostics.HasErrors() {
		t.Fatal(diagnosticError(sources, diagnostics))
	}
	program, err := parseSourceFileWithLanguageContract(sources, sources.Files()[0], PipeLangLanguageContractV260)
	if err != nil {
		t.Fatal(err)
	}
	trim, ok := program.Classes[0].Methods[0].Body.(*TextTrimExpr)
	if !ok {
		t.Fatalf("trim AST = %#v", program.Classes[0].Methods[0].Body)
	}
	value, valueOK := trim.Value.(*IdentExpr)
	if !trim.Span.IsValid() || !valueOK || value.Name != "value" || !value.Span.IsValid() {
		t.Fatalf("trim AST = %#v", trim)
	}
}

func TestV260TextTrimRejectsExcludedForms(t *testing.T) {
	for _, test := range []struct {
		name, source string
	}{
		{name: "extra-parameter", source: `public Class Root { public string Clean(string value, string extra) => trim(value); }`},
		{name: "literal-operand", source: `public Class Root { public string Clean(string value) => trim("fixed"); }`},
		{name: "nested-trim", source: `public Class Root { public string Clean(string value) => trim(trim(value)); }`},
		{name: "nested-expression", source: `public Class Root { public string Clean(string value) => trim(value) + "x"; }`},
		{name: "wrong-return", source: `public Class Root { public bool Clean(string value) => trim(value); }`},
		{name: "non-string-parameter", source: `public Class Root { public string Clean(int value) => trim(value); }`},
		{name: "field-default", source: `public Class Root { public string Value = trim("fixed"); }`},
	} {
		t.Run(test.name, func(t *testing.T) {
			input := semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", "root.pipe", test.source)}, nil)
			input.LanguageContract = PipeLangLanguageContractV260
			if analysis := AnalyzeSemanticModuleSet(input); !analysis.Diagnostics.HasErrors() {
				t.Fatal("excluded trim form was accepted")
			}
		})
	}
}

func TestTextTrimRequiresExplicitV260Migration(t *testing.T) {
	const source = `public Class Root { public string Clean(string value) => trim(value); }`
	for _, contract := range []LanguageContract{PipeLangLanguageContractV010, PipeLangLanguageContractV020, PipeLangLanguageContractV030, PipeLangLanguageContractV040, PipeLangLanguageContractV050, PipeLangLanguageContractV060, PipeLangLanguageContractV070, PipeLangLanguageContractV080, PipeLangLanguageContractV090, PipeLangLanguageContractV100, PipeLangLanguageContractV110, PipeLangLanguageContractV120, PipeLangLanguageContractV130, PipeLangLanguageContractV140, PipeLangLanguageContractV150, PipeLangLanguageContractV160, PipeLangLanguageContractV170, PipeLangLanguageContractV180, PipeLangLanguageContractV190, PipeLangLanguageContractV200, PipeLangLanguageContractV210, PipeLangLanguageContractV220, PipeLangLanguageContractV230, PipeLangLanguageContractV240, PipeLangLanguageContractV250} {
		input := semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", "root.pipe", source)}, nil)
		input.LanguageContract = contract
		if analysis := AnalyzeSemanticModuleSet(input); !analysis.Diagnostics.HasErrors() {
			t.Fatalf("%s implicitly accepted trim", contract)
		}
	}
}

func TestV260TextTrimRejectsMalformedHIRAndCore(t *testing.T) {
	_, typed, core, _ := v260TextTrimPipeline(t)
	malformedHIR := typed
	malformedHIR.Functions = append([]hir.Function(nil), typed.Functions...)
	malformedHIR.Functions[0].Body = typed.Functions[0].Body
	malformedTrim := *typed.Functions[0].Body.TextTrim
	malformedTrim.Value = nil
	malformedHIR.Functions[0].Body.TextTrim = &malformedTrim
	if _, err := LowerHIRToCore(malformedHIR); err == nil {
		t.Fatal("Core lowering accepted malformed trim HIR")
	}
	priorHIR := typed
	priorHIR.LanguageContract = coreir.LanguageContractV250
	if _, err := LowerHIRToCore(priorHIR); err == nil {
		t.Fatal("v0.25.0 HIR implicitly accepted trim")
	}

	malformedCore := core.Functions[0]
	malformedCore.Body = core.Functions[0].Body
	malformedCoreTrim := *core.Functions[0].Body.TextTrim
	malformedCoreTrim.Value = &coreir.Expr{Kind: coreir.ExprLiteral, Type: malformedCore.ReturnType, Literal: &coreir.Literal{String: "fixed"}}
	malformedCore.Body.TextTrim = &malformedCoreTrim
	if err := coreir.ValidateFunction(malformedCore); err == nil {
		t.Fatal("Core accepted a non-parameter trim operand")
	}
	priorCore := core
	priorCore.LanguageContract = coreir.LanguageContractV250
	if _, err := gobackend.Generate(priorCore); err == nil {
		t.Fatal("v0.25.0 Core implicitly accepted trim")
	}
}

func TestV260PreservesV250TextResult(t *testing.T) {
	source, err := os.ReadFile("testdata/text-result.pipe")
	if err != nil {
		t.Fatal(err)
	}
	input := semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", "text-result.pipe", string(source))}, nil)
	input.LanguageContract = PipeLangLanguageContractV260
	analysis := AnalyzeSemanticModuleSet(input)
	if err := analysis.Error(); err != nil {
		t.Fatal(err)
	}
	typed, err := LowerSemanticMethodToHIR(analysis, semanticMethodNamed(t, analysis, "TextOk").Identity)
	if err != nil {
		t.Fatal(err)
	}
	core, err := LowerHIRToCore(typed)
	if err != nil {
		t.Fatal(err)
	}
	if typed.Functions[0].Body.Kind != hir.ExprResultOK || core.Functions[0].Body.Kind != coreir.ExprResultOK {
		t.Fatalf("v0.26.0 did not preserve v0.25.0 text Result: %#v / %#v", typed.Functions[0].Body, core.Functions[0].Body)
	}
	if _, err := gobackend.Generate(core); err != nil {
		t.Fatal(err)
	}
}

func v260TextTrimPipeline(t *testing.T) (*Analysis, hir.Program, coreir.Program, []byte) {
	t.Helper()
	source, err := os.ReadFile("testdata/text-trim.pipe")
	if err != nil {
		t.Fatal(err)
	}
	input := semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", "text-trim.pipe", string(source))}, nil)
	input.LanguageContract = PipeLangLanguageContractV260
	analysis := AnalyzeSemanticModuleSet(input)
	if err := analysis.Error(); err != nil {
		t.Fatal(err)
	}
	typed, err := LowerSemanticMethodToHIR(analysis, semanticMethodNamed(t, analysis, "Trim").Identity)
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

func textTrimGeneratedGoTest(functionName string) string {
	return fmt.Sprintf(`package %s

import "testing"

func expectTrimPanic(t *testing.T, invoke func()) {
	t.Helper()
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic")
		}
	}()
	invoke()
}

func TestGeneratedTextTrim(t *testing.T) {
	for _, test := range []struct {
		value, want string
	}{
		{value: "\t launcher \r\n", want: "launcher"},
		{value: "\u1680\u2000docker\u205f\u3000", want: "docker"},
		{value: " \talpha\u00a0beta\n ", want: "alpha\u00a0beta"},
		{value: "\u200bvalue\u200b", want: "\u200bvalue\u200b"},
	} {
		if got := %s(test.value); got != test.want {
			t.Fatalf("trim(%%q) = %%q, want %%q", test.value, got, test.want)
		}
	}
	expectTrimPanic(t, func() { %s(string([]byte{0xff})) })
}
`, gobackend.PackageName, functionName, functionName)
}
