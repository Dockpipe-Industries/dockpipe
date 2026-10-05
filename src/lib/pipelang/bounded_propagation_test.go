package pipelang

import (
	"strings"
	"testing"

	"dockpipe/src/lib/pipelang/coreeval"
	"dockpipe/src/lib/pipelang/coreir"
	"dockpipe/src/lib/pipelang/gobackend"
	"dockpipe/src/lib/pipelang/hir"
)

func TestV340BoundedOptionalPropagation(t *testing.T) {
	source := `public Class Root { public Optional<string> Forward(Optional<string> value) => some(propagate(value)); }`
	input := semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", "propagate.pipe", source)}, nil)
	input.LanguageContract = PipeLangLanguageContractV340
	analysis := AnalyzeSemanticModuleSet(input)
	if err := analysis.Error(); err != nil {
		t.Fatal(err)
	}
	typed, err := LowerSemanticMethodToHIR(analysis, semanticMethodNamed(t, analysis, "Forward").Identity)
	if err != nil {
		t.Fatal(err)
	}
	if typed.LanguageContract != coreir.LanguageContractV340 || typed.Functions[0].Body.Kind != hir.ExprOptionalSome || typed.Functions[0].Body.Some.Value.Kind != hir.ExprPropagate {
		t.Fatalf("propagation HIR = %#v", typed)
	}
	core, err := LowerHIRToCore(typed)
	if err != nil {
		t.Fatal(err)
	}
	if core.Functions[0].Body.Some.Value.Kind != coreir.ExprPropagate {
		t.Fatalf("propagation Core = %#v", core)
	}
	inner := core.Functions[0].ReturnType.Optional.Value
	present := coreeval.Value{Type: core.Functions[0].ReturnType, Optional: &coreeval.OptionalValue{Present: true, Value: &coreeval.Value{Type: inner, String: "row"}}}
	outcome, err := coreeval.Evaluate(core.Functions[0], []coreeval.Value{present})
	if err != nil || !outcome.OK || outcome.Value.Optional == nil || !outcome.Value.Optional.Present || outcome.Value.Optional.Value.String != "row" {
		t.Fatalf("present = %#v (%v)", outcome, err)
	}
	absent := coreeval.Value{Type: core.Functions[0].ReturnType, Optional: &coreeval.OptionalValue{}}
	outcome, err = coreeval.Evaluate(core.Functions[0], []coreeval.Value{absent})
	if err != nil || !outcome.OK || outcome.Value.Optional == nil || outcome.Value.Optional.Present {
		t.Fatalf("absent = %#v (%v)", outcome, err)
	}
	generated, err := gobackend.Generate(core)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(generated), "value, present := pipelangPropagateOptional(p0)") || !strings.Contains(string(generated), "if !present") {
		t.Fatalf("generated Go lacks explicit propagation:\n%s", generated)
	}
}

func TestV340PropagationMisuseDiagnosticAndCompatibility(t *testing.T) {
	bad := semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", "bad.pipe", `public Class Root { public Optional<string> Bad(string value) => some(propagate(value)); }`)}, nil)
	bad.LanguageContract = PipeLangLanguageContractV340
	analysis := AnalyzeSemanticModuleSet(bad)
	if len(analysis.Diagnostics) != 1 || analysis.Diagnostics[0].Code != CodePropagation || analysis.Diagnostics[0].Primary.File != "bad.pipe" {
		t.Fatalf("misuse diagnostics = %#v", analysis.Diagnostics)
	}
	legacy := semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", "legacy.pipe", `public Class Root { public Optional<string> Forward(Optional<string> value) => value; }`)}, nil)
	legacy.LanguageContract = PipeLangLanguageContractV330
	if err := AnalyzeSemanticModuleSet(legacy).Error(); err != nil {
		t.Fatalf("v0.33.0 compatibility: %v", err)
	}
}

func TestV340BoundedTextResultPropagation(t *testing.T) {
	source := `public Class Root { public Result<string, string> Forward(Result<string, string> value) => ok<string, string>(propagate(value)); }`
	input := semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", "result-propagate.pipe", source)}, nil)
	input.LanguageContract = PipeLangLanguageContractV340
	analysis := AnalyzeSemanticModuleSet(input)
	if err := analysis.Error(); err != nil {
		t.Fatal(err)
	}
	typed, err := LowerSemanticMethodToHIR(analysis, semanticMethodNamed(t, analysis, "Forward").Identity)
	if err != nil {
		t.Fatal(err)
	}
	core, err := LowerHIRToCore(typed)
	if err != nil {
		t.Fatal(err)
	}
	if core.Functions[0].Body.ResultOK.Value.Kind != coreir.ExprPropagate {
		t.Fatalf("Result propagation Core = %#v", core)
	}
	resultType := core.Functions[0].ReturnType
	success := coreeval.Value{Type: resultType, Result: &coreeval.Outcome{OK: true, Value: coreeval.Value{Type: resultType.Result.Success, String: "ok"}}}
	outcome, err := coreeval.Evaluate(core.Functions[0], []coreeval.Value{success})
	if err != nil || !outcome.OK || outcome.Value.String != "ok" {
		t.Fatalf("Result success = %#v (%v)", outcome, err)
	}
	failureValue := coreeval.Value{Type: resultType.Result.Failure, String: "failed"}
	failure := coreeval.Value{Type: resultType, Result: &coreeval.Outcome{Value: coreeval.Value{Type: resultType.Result.Success}, Failure: &failureValue}}
	outcome, err = coreeval.Evaluate(core.Functions[0], []coreeval.Value{failure})
	if err != nil || outcome.OK || outcome.Failure == nil || outcome.Failure.String != "failed" {
		t.Fatalf("Result failure = %#v (%v)", outcome, err)
	}
	generated, err := gobackend.Generate(core)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(generated), "if !p0.OK") {
		t.Fatalf("generated Go lacks Result propagation:\n%s", generated)
	}
}
