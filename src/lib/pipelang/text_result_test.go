package pipelang

import (
	"bytes"
	"fmt"
	"os"
	"testing"

	"dockpipe/src/lib/pipelang/coreeval"
	"dockpipe/src/lib/pipelang/coreir"
	"dockpipe/src/lib/pipelang/gobackend"
	"dockpipe/src/lib/pipelang/hir"
)

func TestV250TextResultHIRCoreEvaluatorAndGo(t *testing.T) {
	source, err := os.ReadFile("testdata/text-result.pipe")
	if err != nil {
		t.Fatal(err)
	}
	input := semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", "text-result.pipe", string(source))}, nil)
	input.LanguageContract = PipeLangLanguageContractV250
	analysis := AnalyzeSemanticModuleSet(input)
	if err := analysis.Error(); err != nil {
		t.Fatal(err)
	}

	projection, err := BuildSemanticProjection(analysis)
	if err != nil {
		t.Fatal(err)
	}
	textOK := projectedMemberNamed(t, projectedTypeNamed(t, projection, "Root"), "TextOk")
	if projection.LanguageContract != PipeLangLanguageContractV250 || projection.Schema != PipeLangSemanticProjectionVersion || textOK.Type.Identity == nil || textOK.Type.Identity.PackageID != PipeLangBuiltinPackageID || textOK.Type.Identity.Path != PipeLangResultSemanticPath || len(textOK.Type.Arguments) != 2 || textOK.Type.Arguments[0].Primitive != TypeString || textOK.Type.Arguments[1].Primitive != TypeString {
		t.Fatalf("text Result semantic projection = %#v / %#v", projection, textOK)
	}

	names := []string{"TextOk", "TextFailed", "ForwardText", "TextSucceeded", "TextOr", "ErrorOr"}
	var typed hir.Program
	for index, name := range names {
		lowered, err := LowerSemanticMethodToHIR(analysis, semanticMethodNamed(t, analysis, name).Identity)
		if err != nil {
			t.Fatal(err)
		}
		if index == 0 {
			typed = lowered
		} else {
			typed.Functions = append(typed.Functions, lowered.Functions...)
		}
	}
	wantHIRKinds := []hir.ExprKind{hir.ExprResultOK, hir.ExprResultErr, hir.ExprReference, hir.ExprResultIsOK, hir.ExprResultSuccessOr, hir.ExprResultFailureOr}
	if typed.LanguageContract != coreir.LanguageContractV250 || len(typed.Functions) != len(wantHIRKinds) {
		t.Fatalf("text Result HIR = %#v", typed)
	}
	for index, kind := range wantHIRKinds {
		if typed.Functions[index].Body.Kind != kind {
			t.Fatalf("text Result HIR function %d = %#v", index, typed.Functions[index])
		}
	}
	priorHIR := typed
	priorHIR.LanguageContract = coreir.LanguageContractV240
	if _, err := LowerHIRToCore(priorHIR); err == nil {
		t.Fatal("v0.24.0 HIR implicitly accepted text Result")
	}
	malformedHIR := typed
	malformedHIR.Functions = append([]hir.Function(nil), typed.Functions...)
	malformedHIR.Functions[0].Body = typed.Functions[0].Body
	malformedOK := *typed.Functions[0].Body.ResultOK
	malformedOK.Value = nil
	malformedHIR.Functions[0].Body.ResultOK = &malformedOK
	if _, err := LowerHIRToCore(malformedHIR); err == nil {
		t.Fatal("Core lowering accepted malformed text Result HIR")
	}

	core, err := LowerHIRToCore(typed)
	if err != nil {
		t.Fatal(err)
	}
	wantCoreKinds := []coreir.ExprKind{coreir.ExprResultOK, coreir.ExprResultErr, coreir.ExprReference, coreir.ExprResultIsOK, coreir.ExprResultSuccessOr, coreir.ExprResultFailureOr}
	for index, kind := range wantCoreKinds {
		if core.Functions[index].Body.Kind != kind {
			t.Fatalf("text Result Core function %d = %#v", index, core.Functions[index])
		}
	}
	malformedCore := core.Functions[0]
	malformedCore.Body = core.Functions[0].Body
	malformedCoreOK := *core.Functions[0].Body.ResultOK
	malformedCoreOK.Value = nil
	malformedCore.Body.ResultOK = &malformedCoreOK
	if err := coreir.ValidateFunction(malformedCore); err == nil {
		t.Fatal("Core accepted malformed text Result construction")
	}
	priorCore := core
	priorCore.LanguageContract = coreir.LanguageContractV240
	if _, err := gobackend.Generate(priorCore); err == nil {
		t.Fatal("v0.24.0 Core implicitly accepted text Result")
	}

	stringType := core.Functions[0].Parameters[0].Type
	ok, err := coreeval.Evaluate(core.Functions[0], []coreeval.Value{{Type: stringType, String: "running"}})
	if err != nil || !ok.OK || ok.Value.String != "running" {
		t.Fatalf("text Result ok = %#v (%v)", ok, err)
	}
	failed, err := coreeval.Evaluate(core.Functions[1], []coreeval.Value{{Type: stringType, String: "daemon unavailable"}})
	if err != nil || failed.OK || failed.Failure == nil || failed.Failure.String != "daemon unavailable" {
		t.Fatalf("text Result err = %#v (%v)", failed, err)
	}
	resultType := core.Functions[0].ReturnType
	okValue := coreeval.Value{Type: resultType, Result: &ok}
	failedValue := coreeval.Value{Type: resultType, Result: &failed}
	forwarded, err := coreeval.Evaluate(core.Functions[2], []coreeval.Value{okValue})
	if err != nil || !forwarded.OK || forwarded.Value.String != "running" {
		t.Fatalf("text Result identity = %#v (%v)", forwarded, err)
	}
	for _, test := range []struct {
		value coreeval.Value
		want  bool
	}{{okValue, true}, {failedValue, false}} {
		outcome, err := coreeval.Evaluate(core.Functions[3], []coreeval.Value{test.value})
		if err != nil || outcome.Value.Bool != test.want {
			t.Fatalf("text Result is_ok = %#v (%v)", outcome, err)
		}
	}
	selected, err := coreeval.Evaluate(core.Functions[4], []coreeval.Value{okValue, {Type: stringType, String: "cached"}})
	if err != nil || selected.Value.String != "running" {
		t.Fatalf("text Result success_or success = %#v (%v)", selected, err)
	}
	defaulted, err := coreeval.Evaluate(core.Functions[4], []coreeval.Value{failedValue, {Type: stringType, String: "cached"}})
	if err != nil || defaulted.Value.String != "cached" {
		t.Fatalf("text Result success_or failure = %#v (%v)", defaulted, err)
	}
	errorOutcome, err := coreeval.Evaluate(core.Functions[5], []coreeval.Value{failedValue, {Type: stringType, String: "none"}})
	if err != nil || errorOutcome.Value.String != "daemon unavailable" {
		t.Fatalf("text Result failure_or failure = %#v (%v)", errorOutcome, err)
	}
	noError, err := coreeval.Evaluate(core.Functions[5], []coreeval.Value{okValue, {Type: stringType, String: "none"}})
	if err != nil || noError.Value.String != "none" {
		t.Fatalf("text Result failure_or success = %#v (%v)", noError, err)
	}
	invalidText := coreeval.Value{Type: stringType, String: string([]byte{0xff})}
	if _, err := coreeval.Evaluate(core.Functions[4], []coreeval.Value{okValue, invalidText}); err == nil {
		t.Fatal("text Result success_or accepted invalid unselected fallback")
	}
	if _, err := coreeval.Evaluate(core.Functions[5], []coreeval.Value{failedValue, invalidText}); err == nil {
		t.Fatal("text Result failure_or accepted invalid unselected fallback")
	}
	if _, err := coreeval.Evaluate(core.Functions[0], []coreeval.Value{invalidText}); err == nil {
		t.Fatal("text Result ok accepted invalid UTF-8")
	}
	malformedResult := coreeval.Value{Type: resultType, Result: &coreeval.Outcome{OK: false, Value: coreeval.Value{Type: stringType, String: "not-zero"}, Failure: &coreeval.Value{Type: stringType, String: "failed"}}}
	if _, err := coreeval.Evaluate(core.Functions[2], []coreeval.Value{malformedResult}); err == nil {
		t.Fatal("text Result identity accepted non-canonical failure payload")
	}

	generated, err := gobackend.Generate(core)
	if err != nil {
		t.Fatal(err)
	}
	second, err := gobackend.Generate(core)
	if err != nil || !bytes.Equal(generated, second) {
		t.Fatalf("text Result generated Go is nondeterministic: %v", err)
	}
	compileAndRunGeneratedGoFiles(t, generated, []byte(textResultGeneratedGoTest()))
}

func TestV250TextResultParserPreservesTypeArgumentsOperandsAndSpans(t *testing.T) {
	const source = `public Class Root { public Result<string, string> Value(string value) => ok<string, string>(value); }`
	sources, diagnostics := NewSourceSet([]SourceInput{{Path: "root.pipe", Data: []byte(source)}})
	if diagnostics.HasErrors() {
		t.Fatal(diagnosticError(sources, diagnostics))
	}
	program, err := parseSourceFileWithLanguageContract(sources, sources.Files()[0], PipeLangLanguageContractV250)
	if err != nil {
		t.Fatal(err)
	}
	result, ok := program.Classes[0].Methods[0].Body.(*ResultOKExpr)
	if !ok {
		t.Fatalf("text Result AST = %#v", program.Classes[0].Methods[0].Body)
	}
	operand, operandOK := result.Value.(*IdentExpr)
	if !result.Span.IsValid() || !result.SuccessType.Span.IsValid() || !result.FailureType.Span.IsValid() || !operandOK || operand.Name != "value" || !operand.Span.IsValid() {
		t.Fatalf("text Result AST = %#v", program.Classes[0].Methods[0].Body)
	}
}

func TestV250TextResultRejectsExcludedForms(t *testing.T) {
	tests := []string{
		`public Class Root { public Result<int, string> Value(int value) => ok<int, string>(value); }`,
		`public Class Root { public Result<string, bool> Value(string value) => ok<string, bool>(value); }`,
		`public Class Root { public Result<string, string> Value(string value) => ok<string, string>("literal"); }`,
		`public Class Root { public string Value(Result<string, string> value) => success_or(value, "literal"); }`,
		`public Record Holder { public Result<string, string> Value; }`,
		`public Interface Root { public Result<string, string> Value(string value); }`,
	}
	for index, source := range tests {
		input := semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", "root.pipe", source)}, nil)
		input.LanguageContract = PipeLangLanguageContractV250
		if analysis := AnalyzeSemanticModuleSet(input); !analysis.Diagnostics.HasErrors() {
			t.Fatalf("excluded text Result form %d was accepted", index)
		}
	}
}

func TestTextResultRequiresExplicitV250Migration(t *testing.T) {
	source := `public Class Root { public Result<string, string> Value(string value) => ok<string, string>(value); }`
	for _, contract := range []LanguageContract{PipeLangLanguageContractV010, PipeLangLanguageContractV020, PipeLangLanguageContractV030, PipeLangLanguageContractV040, PipeLangLanguageContractV050, PipeLangLanguageContractV060, PipeLangLanguageContractV070, PipeLangLanguageContractV080, PipeLangLanguageContractV090, PipeLangLanguageContractV100, PipeLangLanguageContractV110, PipeLangLanguageContractV120, PipeLangLanguageContractV130, PipeLangLanguageContractV140, PipeLangLanguageContractV150, PipeLangLanguageContractV160, PipeLangLanguageContractV170, PipeLangLanguageContractV180, PipeLangLanguageContractV190, PipeLangLanguageContractV200, PipeLangLanguageContractV210, PipeLangLanguageContractV220, PipeLangLanguageContractV230, PipeLangLanguageContractV240} {
		input := semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", "root.pipe", source)}, nil)
		input.LanguageContract = contract
		if analysis := AnalyzeSemanticModuleSet(input); !analysis.Diagnostics.HasErrors() {
			t.Fatalf("%s implicitly accepted text Result", contract)
		}
	}
}

func textResultGeneratedGoTest() string {
	return fmt.Sprintf(`package %s

import "testing"

func expectTextResultPanic(t *testing.T, invoke func()) {
	t.Helper()
	defer func() { if recover() == nil { t.Fatal("expected panic") } }()
	invoke()
}

func TestGeneratedTextResult(t *testing.T) {
	ok := PipeLangTextOk("running")
	if !PipeLangTextSucceeded(ok) || PipeLangTextOr(ok, "cached") != "running" { t.Fatal("ok Result mismatch") }
	forwarded := PipeLangForwardText(ok)
	if !forwarded.OK || forwarded.Value != "running" { t.Fatal("identity Result mismatch") }
	failed := PipeLangTextFailed("daemon unavailable")
	if PipeLangTextSucceeded(failed) || PipeLangTextOr(failed, "cached") != "cached" || PipeLangErrorOr(failed, "none") != "daemon unavailable" || PipeLangErrorOr(forwarded, "none") != "none" { t.Fatal("Result default mismatch") }
	expectTextResultPanic(t, func() { PipeLangTextOk(string([]byte{0xff})) })
	expectTextResultPanic(t, func() { PipeLangTextFailed(string([]byte{0xff})) })
	expectTextResultPanic(t, func() { PipeLangTextOr(ok, string([]byte{0xff})) })
	expectTextResultPanic(t, func() { PipeLangErrorOr(failed, string([]byte{0xff})) })
	expectTextResultPanic(t, func() { PipeLangForwardText(PipeLangResult[string, string]{OK: true, Error: "bad"}) })
	expectTextResultPanic(t, func() { PipeLangForwardText(PipeLangResult[string, string]{OK: false, Value: "not-zero", Error: "failed"}) })
}
`, gobackend.PackageName)
}
