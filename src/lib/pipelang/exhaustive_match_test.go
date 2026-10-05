package pipelang

import (
	"dockpipe/src/lib/pipelang/coreeval"
	"dockpipe/src/lib/pipelang/coreir"
	"dockpipe/src/lib/pipelang/gobackend"
	"dockpipe/src/lib/pipelang/hir"
	"strings"
	"testing"
)

func TestV350ExhaustiveOptionalMatch(t *testing.T) {
	source := `public Class Root { public string Read(Optional<string> value) => match(value){ some(item) => item, none => "missing" }; }`
	input := semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", "match.pipe", source)}, nil)
	input.LanguageContract = PipeLangLanguageContractV350
	analysis := AnalyzeSemanticModuleSet(input)
	if err := analysis.Error(); err != nil {
		t.Fatal(err)
	}
	typed, err := LowerSemanticMethodToHIR(analysis, semanticMethodNamed(t, analysis, "Read").Identity)
	if err != nil {
		t.Fatal(err)
	}
	if typed.LanguageContract != coreir.LanguageContractV350 || typed.Functions[0].Body.Kind != hir.ExprMatch || typed.Functions[0].Body.Match.Arms[0].Binding.Kind != hir.BindingMatchArm {
		t.Fatalf("HIR = %#v", typed)
	}
	core, err := LowerHIRToCore(typed)
	if err != nil {
		t.Fatal(err)
	}
	typ := core.Functions[0].Parameters[0].Type
	payload := coreeval.Value{Type: typ.Optional.Value, String: "row"}
	present := coreeval.Value{Type: typ, Optional: &coreeval.OptionalValue{Present: true, Value: &payload}}
	outcome, err := coreeval.Evaluate(core.Functions[0], []coreeval.Value{present})
	if err != nil || !outcome.OK || outcome.Value.String != "row" {
		t.Fatalf("present=%#v %v", outcome, err)
	}
	absent := coreeval.Value{Type: typ, Optional: &coreeval.OptionalValue{}}
	outcome, err = coreeval.Evaluate(core.Functions[0], []coreeval.Value{absent})
	if err != nil || outcome.Value.String != "missing" {
		t.Fatalf("absent=%#v %v", outcome, err)
	}
	generated, err := gobackend.Generate(core)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(generated), "func() string") {
		t.Fatalf("Go lacks match IIFE:\n%s", generated)
	}
}
func TestV350MatchDiagnostics(t *testing.T) {
	cases := []struct {
		body string
		code DiagnosticCode
	}{{`match(value){some(item)=>item}`, CodeMatchNonExhaustive}, {`match(value){some(a)=>a,some(b)=>b,none=>""}`, CodeMatchDuplicate}, {`match(value){_=>"",none=>""}`, CodeMatchUnreachable}}
	for _, tc := range cases {
		input := semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", "bad.pipe", `public Class Root { public string Read(Optional<string> value) => `+tc.body+`; }`)}, nil)
		input.LanguageContract = PipeLangLanguageContractV350
		a := AnalyzeSemanticModuleSet(input)
		if len(a.Diagnostics) != 1 || a.Diagnostics[0].Code != tc.code {
			t.Fatalf("%s diagnostics=%#v", tc.body, a.Diagnostics)
		}
	}
}

func TestV350ExhaustiveResultMatch(t *testing.T) {
	source := `public Class Root { public string Read(Result<string, string> value) => match(value){ ok(item) => item, err(problem) => problem }; }`
	input := semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", "result-match.pipe", source)}, nil)
	input.LanguageContract = PipeLangLanguageContractV350
	analysis := AnalyzeSemanticModuleSet(input)
	if err := analysis.Error(); err != nil {
		t.Fatal(err)
	}
	typed, err := LowerSemanticMethodToHIR(analysis, semanticMethodNamed(t, analysis, "Read").Identity)
	if err != nil {
		t.Fatal(err)
	}
	core, err := LowerHIRToCore(typed)
	if err != nil {
		t.Fatal(err)
	}
	typ := core.Functions[0].Parameters[0].Type
	success := coreeval.Value{Type: typ, Result: &coreeval.Outcome{OK: true, Value: coreeval.Value{Type: typ.Result.Success, String: "yes"}}}
	outcome, err := coreeval.Evaluate(core.Functions[0], []coreeval.Value{success})
	if err != nil || outcome.Value.String != "yes" {
		t.Fatalf("success=%#v %v", outcome, err)
	}
	failureValue := coreeval.Value{Type: typ.Result.Failure, String: "no"}
	failure := coreeval.Value{Type: typ, Result: &coreeval.Outcome{Value: coreeval.Value{Type: typ.Result.Success}, Failure: &failureValue}}
	outcome, err = coreeval.Evaluate(core.Functions[0], []coreeval.Value{failure})
	if err != nil || outcome.Value.String != "no" {
		t.Fatalf("failure=%#v %v", outcome, err)
	}
}
func TestV350ArithmeticResultMatch(t *testing.T) {
	source := `public Class Root { public string Read(Result<int, ArithmeticError> value) => match(value){ ok(item) => "ok", err(problem) => "error" }; }`
	input := semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", "arithmetic-match.pipe", source)}, nil)
	input.LanguageContract = PipeLangLanguageContractV350
	analysis := AnalyzeSemanticModuleSet(input)
	if err := analysis.Error(); err != nil {
		t.Fatal(err)
	}
	typed, err := LowerSemanticMethodToHIR(analysis, semanticMethodNamed(t, analysis, "Read").Identity)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = LowerHIRToCore(typed); err != nil {
		t.Fatal(err)
	}
}
