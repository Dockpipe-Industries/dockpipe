package pipelang

import (
	"fmt"
	"strings"
	"testing"

	"dockpipe/src/lib/pipelang/coreeval"
	"dockpipe/src/lib/pipelang/coreir"
	"dockpipe/src/lib/pipelang/gobackend"
	"dockpipe/src/lib/pipelang/hir"
)

const singleStageContextualSamePayloadSource = `public Record Token { public string Text; }
public Class SamePayloadCompilerPipeline {
	public Result<string, string> Normalize(string source, string context) => context == "fail" ? err<string, string>("normalize failed:" + source) : ok<string, string>(source);
	public Result<List<Token>, string> Preserve(List<Token> tokens, string context) => context == "fail" ? err<List<Token>, string>("preserve failed") : ok<List<Token>, string>(tokens);
	public Result<string, string> Compile(Result<string, string> scanned, string context) {
		string source = propagate(scanned);
		return Normalize(source, context);
	}
	public Result<List<Token>, string> Reuse(Result<List<Token>, string> parsed, string context) {
		List<Token> tokens = propagate(parsed);
		return Preserve(tokens, context);
	}
}`

func singleStageContextualSamePayloadProgram(t *testing.T, method string) (*Analysis, SemanticIdentity, coreir.Program) {
	t.Helper()
	input := semanticTestModuleSet("compiler.selfhosting", []ModuleInput{testModule("compiler.selfhosting", "single-stage-contextual-same-payload.pipe", singleStageContextualSamePayloadSource)}, nil)
	input.LanguageContract = PipeLangLanguageContractV640
	analysis := AnalyzeSemanticModuleSet(input)
	if err := analysis.Error(); err != nil {
		t.Fatal(err)
	}
	identity := semanticMethodNamed(t, analysis, method).Identity
	typed, err := LowerSemanticMethodToHIR(analysis, identity)
	if err != nil {
		t.Fatal(err)
	}
	program, err := LowerHIRToCore(typed)
	if err != nil {
		t.Fatal(err)
	}
	return analysis, identity, program
}

func TestV640SingleStageContextualSamePayloadResultPropagationPipeline(t *testing.T) {
	analysis, identity, program := singleStageContextualSamePayloadProgram(t, "Compile")
	projection, err := BuildSemanticProjection(analysis)
	if err != nil {
		t.Fatal(err)
	}
	if projection.LanguageContract != PipeLangLanguageContractV640 || projection.Schema != PipeLangSemanticProjectionVersion || projection.CompilerContract != PipeLangCompilerContract {
		t.Fatalf("projection contract = %#v", projection)
	}

	typed, err := LowerSemanticMethodToHIR(analysis, identity)
	if err != nil {
		t.Fatal(err)
	}
	function := hirFunctionNamed(t, typed, "Compile")
	first := function.Body.ImmutableLocal
	if typed.LanguageContract != coreir.LanguageContractV640 || first == nil || first.Binding.Position != 2 || first.Initializer.Kind != hir.ExprPropagate || first.Return == nil || first.Return.Kind != hir.ExprCall || first.Return.Call == nil || len(first.Return.Call.Arguments) != 2 {
		t.Fatalf("one-stage same-payload contextual HIR = %#v", function.Body)
	}
	if first.Return.Call.Arguments[0].Reference == nil || first.Return.Call.Arguments[0].Reference.Position != 2 || first.Return.Call.Arguments[1].Reference == nil || first.Return.Call.Arguments[1].Reference.Position != 1 {
		t.Fatalf("one-stage same-payload contextual HIR arguments = %#v", first.Return.Call.Arguments)
	}

	coreFunction := coreFunctionNamed(t, program, "Compile")
	coreFirst := coreFunction.Body.ImmutableLocal
	if program.LanguageContract != coreir.LanguageContractV640 || coreFirst == nil || coreFirst.Position != 2 || coreFirst.Return == nil || coreFirst.Return.Kind != coreir.ExprCall || coreFirst.Return.Call == nil || len(coreFirst.Return.Call.Arguments) != 2 {
		t.Fatalf("one-stage same-payload contextual Core = %#v", coreFunction.Body)
	}
	if !coreir.TypeEqual(coreFunction.Parameters[0].Type.Result.Success, coreFunction.ReturnType.Result.Success) {
		t.Fatal("same-payload Result identity was not preserved")
	}
	if coreFirst.Return.Call.Arguments[0].Parameter == nil || *coreFirst.Return.Call.Arguments[0].Parameter != 2 || coreFirst.Return.Call.Arguments[1].Parameter == nil || *coreFirst.Return.Call.Arguments[1].Parameter != 1 {
		t.Fatalf("one-stage same-payload contextual Core arguments = %#v", coreFirst.Return.Call.Arguments)
	}
	if err := coreir.ValidateProgram(program); err != nil {
		t.Fatal(err)
	}

	entry := coreir.SemanticIdentity{PackageID: string(identity.PackageID), Path: string(identity.Path)}
	carrierType := coreFunction.Parameters[0].Type
	contextType := coreFunction.Parameters[1].Type
	successCarrier := coreeval.Outcome{OK: true, Value: coreeval.Value{Type: carrierType.Result.Success, String: "  source  "}}
	success, err := coreeval.EvaluateProgram(program, entry, []coreeval.Value{{Type: carrierType, Result: &successCarrier}, {Type: contextType, String: "release"}})
	if err != nil || !success.OK || success.Value.String != "  source  " {
		t.Fatalf("success = %#v, %v", success, err)
	}
	failureText := coreeval.Value{Type: carrierType.Result.Failure, String: "scan failed"}
	incomingCarrier := coreeval.Outcome{Value: coreeval.Value{Type: carrierType.Result.Success}, Failure: &failureText}
	incoming, err := coreeval.EvaluateProgram(program, entry, []coreeval.Value{{Type: carrierType, Result: &incomingCarrier}, {Type: contextType, String: "release"}})
	if err != nil || incoming.OK || incoming.Failure == nil || incoming.Failure.String != "scan failed" || incoming.Value.String != "" {
		t.Fatalf("incoming failure = %#v, %v", incoming, err)
	}
	helperFailure, err := coreeval.EvaluateProgram(program, entry, []coreeval.Value{{Type: carrierType, Result: &successCarrier}, {Type: contextType, String: "fail"}})
	if err != nil || helperFailure.OK || helperFailure.Failure == nil || helperFailure.Failure.String != "normalize failed:  source  " || helperFailure.Value.String != "" {
		t.Fatalf("helper failure = %#v, %v", helperFailure, err)
	}

	generated, err := gobackend.Generate(program)
	if err != nil {
		t.Fatal(err)
	}
	generatedAgain, err := gobackend.Generate(program)
	if err != nil || string(generatedAgain) != string(generated) {
		t.Fatalf("generated Go is not deterministic: %v", err)
	}
	for _, fragment := range []string{"p2 := p0.Value", "PipeLangNormalize(p2, p1)"} {
		if !strings.Contains(string(generated), fragment) {
			t.Fatalf("generated Go lacks one-stage same-payload contextual propagation %q:\n%s", fragment, generated)
		}
	}
	compileAndRunGeneratedGoFiles(t, generated, []byte(fmt.Sprintf(`package %s

import "testing"

func TestGeneratedSingleStageContextualSamePayload(t *testing.T) {
	success := PipeLangCompile(PipeLangResult[string, string]{OK: true, Value: "  source  "}, "release")
	if !success.OK || success.Value != "  source  " { t.Fatalf("success = %%#v", success) }
	incoming := PipeLangCompile(PipeLangResult[string, string]{Error: "scan failed"}, "release")
	if incoming.OK || incoming.Error != "scan failed" || incoming.Value != "" { t.Fatalf("incoming = %%#v", incoming) }
	helper := PipeLangCompile(PipeLangResult[string, string]{OK: true, Value: "source"}, "fail")
	if helper.OK || helper.Error != "normalize failed:source" || helper.Value != "" { t.Fatalf("helper = %%#v", helper) }
}

func TestGeneratedSingleStageContextualSamePayloadRejectsInvalidValues(t *testing.T) {
	for _, run := range []func(){
		func() { PipeLangCompile(PipeLangResult[string, string]{Value: "not zero", Error: "scan failed"}, "release") },
		func() { PipeLangCompile(PipeLangResult[string, string]{Error: string([]byte{0xff})}, "release") },
		func() { PipeLangCompile(PipeLangResult[string, string]{OK: true, Value: "source"}, string([]byte{0xff})) },
	} {
		func() {
			defer func() { if recover() == nil { t.Fatal("invalid one-stage same-payload contextual value did not panic") } }()
			run()
		}()
	}
}
`, gobackend.PackageName)))
}

func TestV640SingleStageContextualSamePayloadResultPropagationMatrixAndGate(t *testing.T) {
	input := semanticTestModuleSet("compiler.selfhosting", []ModuleInput{testModule("compiler.selfhosting", "single-stage-contextual-same-payload-matrix.pipe", singleStageContextualSamePayloadSource)}, nil)
	input.LanguageContract = PipeLangLanguageContractV630
	if err := AnalyzeSemanticModuleSet(input).Error(); err == nil {
		t.Fatal("v0.63.0 accepted the v0.64.0 same-payload contextual form")
	}
	input.LanguageContract = PipeLangLanguageContractV640
	if err := AnalyzeSemanticModuleSet(input).Error(); err != nil {
		t.Fatal(err)
	}
	_, _, listProgram := singleStageContextualSamePayloadProgram(t, "Reuse")
	listFunction := coreFunctionNamed(t, listProgram, "Reuse")
	if listFunction.ReturnType.Result.Success.Kind != coreir.TypeList || !coreir.TypeEqual(listFunction.Parameters[0].Type.Result.Success, listFunction.ReturnType.Result.Success) {
		t.Fatal("same-payload Result-list matrix form was not lowered")
	}
	generated, err := gobackend.Generate(listProgram)
	if err != nil {
		t.Fatal(err)
	}
	compileAndRunGeneratedGoFiles(t, generated, []byte(fmt.Sprintf(`package %s

import "testing"

func TestGeneratedSamePayloadList(t *testing.T) {
	input := []PipeLangRecordTestPackageCompilerSelfhostingToken{{Text: "one"}}
	success := PipeLangReuse(PipeLangResult[[]PipeLangRecordTestPackageCompilerSelfhostingToken, string]{OK: true, Value: input}, "release")
	if !success.OK || len(success.Value) != 1 || success.Value[0].Text != "one" { t.Fatalf("success = %%#v", success) }
	failure := PipeLangReuse(PipeLangResult[[]PipeLangRecordTestPackageCompilerSelfhostingToken, string]{Error: "parse failed"}, "release")
	if failure.OK || failure.Error != "parse failed" || failure.Value != nil { t.Fatalf("failure = %%#v", failure) }
}
`, gobackend.PackageName)))
}

func TestV640SingleStageContextualSamePayloadRejectsExcludedSource(t *testing.T) {
	valid := `public Class Root { public Result<string, string> Next(string raw, string context) => ok<string, string>(raw); public Result<string, string> Compile(Result<string, string> input, string context) { string raw = propagate(input); return Next(raw, context); } }`
	samePayloadChain := `public Record Token { public string Text; } public Class Root { public Result<string, string> Again(string raw, string context) => ok<string, string>(raw); public Result<List<Token>, string> Finish(string raw, string context) => ok<List<Token>, string>(empty_list<Token>()); public Result<List<Token>, string> Compile(Result<string, string> input, string context) { string raw = propagate(input); Result<string, string> again = Again(raw, context); string normalized = propagate(again); return Finish(normalized, context); } }`
	tests := []struct {
		name   string
		source string
	}{
		{name: "third caller parameter", source: strings.Replace(valid, "input, string context)", "input, string context, string extra)", 1)},
		{name: "non-string context", source: strings.Replace(valid, "input, string context)", "input, bool context)", 1)},
		{name: "missing helper context", source: strings.Replace(valid, "Next(raw, context)", "Next(raw)", 1)},
		{name: "reversed helper arguments", source: strings.Replace(valid, "Next(raw, context)", "Next(context, raw)", 1)},
		{name: "computed helper context", source: strings.Replace(valid, "Next(raw, context)", "Next(raw, trim(context))", 1)},
		{name: "private helper", source: strings.Replace(valid, "public Result<string, string> Next", "private Result<string, string> Next", 1)},
		{name: "extra local", source: strings.Replace(valid, "return Next(raw, context);", "string gap = context; return Next(raw, context);", 1)},
		{name: "same-payload contextual chain", source: samePayloadChain},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			input := semanticTestModuleSet("compiler.selfhosting", []ModuleInput{testModule("compiler.selfhosting", test.name+".pipe", test.source)}, nil)
			input.LanguageContract = PipeLangLanguageContractV640
			if err := AnalyzeSemanticModuleSet(input).Error(); err == nil {
				t.Fatal("excluded v0.64.0 same-payload contextual source was accepted")
			}
		})
	}
}

func TestV640SingleStageContextualSamePayloadRejectsMalformedCoreAndBackend(t *testing.T) {
	_, _, program := singleStageContextualSamePayloadProgram(t, "Compile")
	for index := range program.Functions {
		if program.Functions[index].Name == "Compile" {
			wrongContext := 0
			program.Functions[index].Body.ImmutableLocal.Return.Call.Arguments[1].Parameter = &wrongContext
		}
	}
	if err := coreir.ValidateProgram(program); err == nil || !strings.Contains(err.Error(), "one-stage contextual bounded Result") {
		t.Fatalf("Core validator accepted a non-context second helper argument: %v", err)
	}
	if _, err := gobackend.Generate(program); err == nil {
		t.Fatal("Go backend accepted malformed v0.64.0 same-payload contextual Core")
	}
}

func TestV640PreservesV630SingleStageContextualCrossPayloadResultPropagation(t *testing.T) {
	input := semanticTestModuleSet("compiler.selfhosting", []ModuleInput{testModule("compiler.selfhosting", "single-stage-contextual-cross-payload-v064.pipe", singleStageContextualCrossPayloadSource)}, nil)
	input.LanguageContract = PipeLangLanguageContractV640
	if err := AnalyzeSemanticModuleSet(input).Error(); err != nil {
		t.Fatal(err)
	}
}
