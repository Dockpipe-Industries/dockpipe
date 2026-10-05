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

const singleStageContextualCrossPayloadSource = `public Record Token { public string Text; }
public Class SingleStageCompilerPipeline {
	public Result<List<Token>, string> Tokenize(string source, string context) => context == "fail" ? err<List<Token>, string>("tokenize failed:" + source) : ok<List<Token>, string>(empty_list<Token>());
	public Result<string, string> Emit(List<Token> tokens, string context) { string output = "emitted:" + context; return ok<string, string>(output); }
	public Result<List<Token>, string> Compile(Result<string, string> scanned, string context) {
		string source = propagate(scanned);
		return Tokenize(source, context);
	}
	public Result<string, string> Render(Result<List<Token>, string> parsed, string context) {
		List<Token> tokens = propagate(parsed);
		return Emit(tokens, context);
	}
}`

func singleStageContextualProgram(t *testing.T, method string) (*Analysis, SemanticIdentity, coreir.Program) {
	t.Helper()
	input := semanticTestModuleSet("compiler.selfhosting", []ModuleInput{testModule("compiler.selfhosting", "single-stage-contextual-cross-payload.pipe", singleStageContextualCrossPayloadSource)}, nil)
	input.LanguageContract = PipeLangLanguageContractV630
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

func TestV630SingleStageContextualCrossPayloadResultPropagationPipeline(t *testing.T) {
	analysis, identity, program := singleStageContextualProgram(t, "Compile")
	projection, err := BuildSemanticProjection(analysis)
	if err != nil {
		t.Fatal(err)
	}
	if projection.LanguageContract != PipeLangLanguageContractV630 || projection.Schema != PipeLangSemanticProjectionVersion || projection.CompilerContract != PipeLangCompilerContract {
		t.Fatalf("projection contract = %#v", projection)
	}

	typed, err := LowerSemanticMethodToHIR(analysis, identity)
	if err != nil {
		t.Fatal(err)
	}
	function := hirFunctionNamed(t, typed, "Compile")
	first := function.Body.ImmutableLocal
	if typed.LanguageContract != coreir.LanguageContractV630 || first == nil || first.Binding.Position != 2 || first.Initializer.Kind != hir.ExprPropagate || first.Return == nil || first.Return.Kind != hir.ExprCall || first.Return.Call == nil || len(first.Return.Call.Arguments) != 2 {
		t.Fatalf("one-stage contextual HIR = %#v", function.Body)
	}
	if first.Return.Call.Arguments[0].Reference == nil || first.Return.Call.Arguments[0].Reference.Position != 2 || first.Return.Call.Arguments[1].Reference == nil || first.Return.Call.Arguments[1].Reference.Position != 1 {
		t.Fatalf("one-stage contextual HIR arguments = %#v", first.Return.Call.Arguments)
	}

	coreFunction := coreFunctionNamed(t, program, "Compile")
	coreFirst := coreFunction.Body.ImmutableLocal
	if program.LanguageContract != coreir.LanguageContractV630 || coreFirst == nil || coreFirst.Position != 2 || coreFirst.Return == nil || coreFirst.Return.Kind != coreir.ExprCall || coreFirst.Return.Call == nil || len(coreFirst.Return.Call.Arguments) != 2 {
		t.Fatalf("one-stage contextual Core = %#v", coreFunction.Body)
	}
	if coreFirst.Return.Call.Arguments[0].Parameter == nil || *coreFirst.Return.Call.Arguments[0].Parameter != 2 || coreFirst.Return.Call.Arguments[1].Parameter == nil || *coreFirst.Return.Call.Arguments[1].Parameter != 1 {
		t.Fatalf("one-stage contextual Core arguments = %#v", coreFirst.Return.Call.Arguments)
	}
	if err := coreir.ValidateProgram(program); err != nil {
		t.Fatal(err)
	}

	entry := coreir.SemanticIdentity{PackageID: string(identity.PackageID), Path: string(identity.Path)}
	carrierType := coreFunction.Parameters[0].Type
	contextType := coreFunction.Parameters[1].Type
	successCarrier := coreeval.Outcome{OK: true, Value: coreeval.Value{Type: carrierType.Result.Success, String: "class Root {}"}}
	success, err := coreeval.EvaluateProgram(program, entry, []coreeval.Value{{Type: carrierType, Result: &successCarrier}, {Type: contextType, String: "release"}})
	if err != nil || !success.OK || len(success.Value.List) != 0 {
		t.Fatalf("success = %#v, %v", success, err)
	}
	failureText := coreeval.Value{Type: carrierType.Result.Failure, String: "scan failed"}
	incomingCarrier := coreeval.Outcome{Value: coreeval.Value{Type: carrierType.Result.Success}, Failure: &failureText}
	incoming, err := coreeval.EvaluateProgram(program, entry, []coreeval.Value{{Type: carrierType, Result: &incomingCarrier}, {Type: contextType, String: "release"}})
	if err != nil || incoming.OK || incoming.Failure == nil || incoming.Failure.String != "scan failed" || incoming.Value.List != nil {
		t.Fatalf("incoming failure = %#v, %v", incoming, err)
	}
	helperFailure, err := coreeval.EvaluateProgram(program, entry, []coreeval.Value{{Type: carrierType, Result: &successCarrier}, {Type: contextType, String: "fail"}})
	if err != nil || helperFailure.OK || helperFailure.Failure == nil || helperFailure.Failure.String != "tokenize failed:class Root {}" || helperFailure.Value.List != nil {
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
	for _, fragment := range []string{"p2 := p0.Value", "PipeLangTokenize(p2, p1)"} {
		if !strings.Contains(string(generated), fragment) {
			t.Fatalf("generated Go lacks one-stage contextual propagation %q:\n%s", fragment, generated)
		}
	}
	compileAndRunGeneratedGoFiles(t, generated, []byte(fmt.Sprintf(`package %s

import "testing"

func TestGeneratedSingleStageContextualCrossPayload(t *testing.T) {
	success := PipeLangCompile(PipeLangResult[string, string]{OK: true, Value: "class Root {}"}, "release")
	if !success.OK || len(success.Value) != 0 { t.Fatalf("success = %%#v", success) }
	incoming := PipeLangCompile(PipeLangResult[string, string]{Error: "scan failed"}, "release")
	if incoming.OK || incoming.Error != "scan failed" || incoming.Value != nil { t.Fatalf("incoming = %%#v", incoming) }
	helper := PipeLangCompile(PipeLangResult[string, string]{OK: true, Value: "source"}, "fail")
	if helper.OK || helper.Error != "tokenize failed:source" || helper.Value != nil { t.Fatalf("helper = %%#v", helper) }
}

func TestGeneratedSingleStageContextualRejectsInvalidValues(t *testing.T) {
	for _, run := range []func(){
		func() { PipeLangCompile(PipeLangResult[string, string]{Value: "not zero", Error: "scan failed"}, "release") },
		func() { PipeLangCompile(PipeLangResult[string, string]{Error: string([]byte{0xff})}, "release") },
		func() { PipeLangCompile(PipeLangResult[string, string]{OK: true, Value: "source"}, string([]byte{0xff})) },
	} {
		func() {
			defer func() { if recover() == nil { t.Fatal("invalid one-stage contextual value did not panic") } }()
			run()
		}()
	}
}
`, gobackend.PackageName)))
}

func TestV630SingleStageContextualCrossPayloadResultPropagationMatrixAndGate(t *testing.T) {
	input := semanticTestModuleSet("compiler.selfhosting", []ModuleInput{testModule("compiler.selfhosting", "single-stage-contextual-matrix.pipe", singleStageContextualCrossPayloadSource)}, nil)
	input.LanguageContract = PipeLangLanguageContractV620
	if err := AnalyzeSemanticModuleSet(input).Error(); err == nil {
		t.Fatal("v0.62.0 accepted the v0.63.0 one-stage contextual form")
	}
	input.LanguageContract = PipeLangLanguageContractV630
	if err := AnalyzeSemanticModuleSet(input).Error(); err != nil {
		t.Fatal(err)
	}
	if _, _, program := singleStageContextualProgram(t, "Render"); coreFunctionNamed(t, program, "Render").ReturnType.Result.Success.Kind != coreir.TypePrimitive {
		t.Fatal("list-to-text matrix form was not lowered")
	}
}

func TestV630SingleStageContextualCrossPayloadRejectsExcludedSource(t *testing.T) {
	valid := `public Record Token { public string Text; } public Class Root { public Result<List<Token>, string> Next(string raw, string context) => ok<List<Token>, string>(empty_list<Token>()); public Result<List<Token>, string> Compile(Result<string, string> input, string context) { string raw = propagate(input); return Next(raw, context); } }`
	tests := []struct {
		name   string
		source string
	}{
		{name: "third caller parameter", source: strings.Replace(valid, "input, string context)", "input, string context, string extra)", 1)},
		{name: "non-string context", source: strings.Replace(valid, "input, string context)", "input, bool context)", 1)},
		{name: "missing helper context", source: strings.Replace(valid, "Next(raw, context)", "Next(raw)", 1)},
		{name: "reversed helper arguments", source: strings.Replace(valid, "Next(raw, context)", "Next(context, raw)", 1)},
		{name: "computed helper context", source: strings.Replace(valid, "Next(raw, context)", "Next(raw, trim(context))", 1)},
		{name: "same payload", source: strings.Replace(valid, "Result<List<Token>, string> Next(string raw, string context) => ok<List<Token>, string>(empty_list<Token>())", "Result<string, string> Next(string raw, string context) => ok<string, string>(raw)", 1)},
		{name: "private helper", source: strings.Replace(valid, "public Result<List<Token>, string> Next", "private Result<List<Token>, string> Next", 1)},
		{name: "extra local", source: strings.Replace(valid, "return Next(raw, context);", "string gap = context; return Next(raw, context);", 1)},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			input := semanticTestModuleSet("compiler.selfhosting", []ModuleInput{testModule("compiler.selfhosting", test.name+".pipe", test.source)}, nil)
			input.LanguageContract = PipeLangLanguageContractV630
			if err := AnalyzeSemanticModuleSet(input).Error(); err == nil {
				t.Fatal("excluded one-stage contextual source was accepted")
			}
		})
	}
}

func TestV630SingleStageContextualCrossPayloadRejectsMalformedCoreAndBackend(t *testing.T) {
	_, _, program := singleStageContextualProgram(t, "Compile")
	for index := range program.Functions {
		if program.Functions[index].Name == "Compile" {
			wrongContext := 0
			program.Functions[index].Body.ImmutableLocal.Return.Call.Arguments[1].Parameter = &wrongContext
		}
	}
	if err := coreir.ValidateProgram(program); err == nil || !strings.Contains(err.Error(), "one-stage contextual cross-payload") {
		t.Fatalf("Core validator accepted a non-context second helper argument: %v", err)
	}
	if _, err := gobackend.Generate(program); err == nil {
		t.Fatal("Go backend accepted malformed one-stage contextual Core")
	}
}

func TestV630PreservesV620ContextualCrossPayloadResultPropagationChains(t *testing.T) {
	input := semanticTestModuleSet("compiler.selfhosting", []ModuleInput{testModule("compiler.selfhosting", "contextual-chain-v063.pipe", contextualCrossPayloadResultPropagationSource)}, nil)
	input.LanguageContract = PipeLangLanguageContractV630
	if err := AnalyzeSemanticModuleSet(input).Error(); err != nil {
		t.Fatal(err)
	}
}
