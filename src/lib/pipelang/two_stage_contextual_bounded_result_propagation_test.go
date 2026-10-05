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

const twoStageContextualBoundedResultPropagationSource = `public Record Token { public string Text; }
public Class TwoStageContextualPipeline {
	public Result<string, string> Normalize(string source, string context) => context == "first" ? err<string, string>("normalize failed") : ok<string, string>(source + context);
	public Result<List<Token>, string> Finish(string source, string context) => context == "second" ? err<List<Token>, string>("finish failed") : ok<List<Token>, string>(empty_list<Token>());
	public Result<List<Token>, string> Tokenize(string source, string context) => context == "" ? err<List<Token>, string>("missing context") : ok<List<Token>, string>(empty_list<Token>());
	public Result<List<Token>, string> Preserve(List<Token> tokens, string context) => context == "" ? err<List<Token>, string>("missing context") : ok<List<Token>, string>(tokens);
	public Result<string, string> Again(string source, string context) => context == "" ? err<string, string>("missing context") : ok<string, string>(source);
	public Result<string, string> Finalize(string source, string context) => context == "" ? err<string, string>("missing context") : ok<string, string>(source);
	public Result<List<Token>, string> Compile(Result<string, string> scanned, string context) {
		string source = propagate(scanned);
		Result<string, string> normalizedCarrier = Normalize(source, context);
		string normalized = propagate(normalizedCarrier);
		return Finish(normalized, context);
	}
	public Result<List<Token>, string> Validate(Result<string, string> scanned, string context) {
		string source = propagate(scanned);
		Result<List<Token>, string> tokenCarrier = Tokenize(source, context);
		List<Token> tokens = propagate(tokenCarrier);
		return Preserve(tokens, context);
	}
	public Result<string, string> Repeat(Result<string, string> scanned, string context) {
		string source = propagate(scanned);
		Result<string, string> againCarrier = Again(source, context);
		string again = propagate(againCarrier);
		return Finalize(again, context);
	}
}`

func twoStageContextualBoundedProgram(t *testing.T, method string) (*Analysis, SemanticIdentity, coreir.Program) {
	t.Helper()
	input := semanticTestModuleSet("compiler.selfhosting", []ModuleInput{testModule("compiler.selfhosting", "two-stage-contextual-bounded-result.pipe", twoStageContextualBoundedResultPropagationSource)}, nil)
	input.LanguageContract = PipeLangLanguageContractV650
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

func TestV650TwoStageContextualBoundedResultPropagationPipeline(t *testing.T) {
	analysis, identity, program := twoStageContextualBoundedProgram(t, "Compile")
	projection, err := BuildSemanticProjection(analysis)
	if err != nil {
		t.Fatal(err)
	}
	if projection.LanguageContract != PipeLangLanguageContractV650 || projection.Schema != PipeLangSemanticProjectionVersion || projection.CompilerContract != PipeLangCompilerContract {
		t.Fatalf("projection contract = %#v", projection)
	}

	typed, err := LowerSemanticMethodToHIR(analysis, identity)
	if err != nil {
		t.Fatal(err)
	}
	function := hirFunctionNamed(t, typed, "Compile")
	first := function.Body.ImmutableLocal
	if typed.LanguageContract != coreir.LanguageContractV650 || first == nil || first.Binding.Position != 2 || first.Initializer.Kind != hir.ExprPropagate || first.Return == nil || first.Return.Kind != hir.ExprImmutableLocal {
		t.Fatalf("first v0.65.0 HIR local = %#v", function.Body)
	}
	intermediate := first.Return.ImmutableLocal
	if intermediate == nil || intermediate.Return == nil || intermediate.Return.Kind != hir.ExprImmutableLocal || intermediate.Return.ImmutableLocal == nil {
		t.Fatalf("v0.65.0 HIR intermediate chain = %#v", function.Body)
	}
	second := intermediate.Return.ImmutableLocal
	if intermediate.Binding.Position != 3 || intermediate.Initializer.Kind != hir.ExprCall || intermediate.Initializer.Call == nil || second.Binding.Position != 4 || second.Initializer.Kind != hir.ExprPropagate || second.Return == nil || second.Return.Kind != hir.ExprCall {
		t.Fatalf("v0.65.0 HIR chain = %#v", function.Body)
	}
	for _, call := range []*hir.Call{intermediate.Initializer.Call, second.Return.Call} {
		if call == nil || len(call.Arguments) != 2 || call.Arguments[1].Reference == nil || call.Arguments[1].Reference.Position != 1 {
			t.Fatalf("v0.65.0 HIR helper arguments = %#v", call)
		}
	}

	coreFunction := coreFunctionNamed(t, program, "Compile")
	coreFirst := coreFunction.Body.ImmutableLocal
	coreIntermediate := coreFirst.Return.ImmutableLocal
	coreSecond := coreIntermediate.Return.ImmutableLocal
	if program.LanguageContract != coreir.LanguageContractV650 || coreFirst.Position != 2 || coreIntermediate.Position != 3 || coreSecond.Position != 4 || !coreir.TypeEqual(coreFirst.Type, coreIntermediate.Type.Result.Success) {
		t.Fatalf("v0.65.0 Core chain = %#v", coreFunction.Body)
	}
	for _, call := range []*coreir.Call{coreIntermediate.Initializer.Call, coreSecond.Return.Call} {
		if call == nil || len(call.Arguments) != 2 || call.Arguments[1] == nil || call.Arguments[1].Parameter == nil || *call.Arguments[1].Parameter != 1 {
			t.Fatalf("v0.65.0 Core helper arguments = %#v", call)
		}
	}
	if err := coreir.ValidateProgram(program); err != nil {
		t.Fatal(err)
	}

	entry := coreir.SemanticIdentity{PackageID: string(identity.PackageID), Path: string(identity.Path)}
	carrierType := coreFunction.Parameters[0].Type
	contextType := coreFunction.Parameters[1].Type
	successCarrier := coreeval.Outcome{OK: true, Value: coreeval.Value{Type: carrierType.Result.Success, String: "source"}}
	success, err := coreeval.EvaluateProgram(program, entry, []coreeval.Value{{Type: carrierType, Result: &successCarrier}, {Type: contextType, String: "ok"}})
	if err != nil || !success.OK || success.Value.Type.Kind != coreir.TypeList || success.Value.List == nil || len(success.Value.List) != 0 {
		t.Fatalf("success = %#v, %v", success, err)
	}
	failureText := coreeval.Value{Type: carrierType.Result.Failure, String: "scan failed"}
	incomingCarrier := coreeval.Outcome{Value: coreeval.Value{Type: carrierType.Result.Success}, Failure: &failureText}
	incoming, err := coreeval.EvaluateProgram(program, entry, []coreeval.Value{{Type: carrierType, Result: &incomingCarrier}, {Type: contextType, String: "ok"}})
	if err != nil || incoming.OK || incoming.Failure == nil || incoming.Failure.String != "scan failed" || incoming.Value.List != nil {
		t.Fatalf("incoming failure = %#v, %v", incoming, err)
	}
	firstFailure, err := coreeval.EvaluateProgram(program, entry, []coreeval.Value{{Type: carrierType, Result: &successCarrier}, {Type: contextType, String: "first"}})
	if err != nil || firstFailure.OK || firstFailure.Failure == nil || firstFailure.Failure.String != "normalize failed" || firstFailure.Value.List != nil {
		t.Fatalf("first helper failure = %#v, %v", firstFailure, err)
	}
	secondFailure, err := coreeval.EvaluateProgram(program, entry, []coreeval.Value{{Type: carrierType, Result: &successCarrier}, {Type: contextType, String: "second"}})
	if err != nil || secondFailure.OK || secondFailure.Failure == nil || secondFailure.Failure.String != "finish failed" || secondFailure.Value.List != nil {
		t.Fatalf("second helper failure = %#v, %v", secondFailure, err)
	}

	generated, err := gobackend.Generate(program)
	if err != nil {
		t.Fatal(err)
	}
	generatedAgain, err := gobackend.Generate(program)
	if err != nil || string(generatedAgain) != string(generated) {
		t.Fatalf("generated Go is not deterministic: %v", err)
	}
	for _, fragment := range []string{"PipeLangNormalize(p2, p1)", "PipeLangFinish(p4, p1)"} {
		if !strings.Contains(string(generated), fragment) {
			t.Fatalf("generated Go lacks exact two-stage contextual propagation %q:\n%s", fragment, generated)
		}
	}
	compileAndRunGeneratedGoFiles(t, generated, []byte(fmt.Sprintf(`package %s

import "testing"

func TestGeneratedTwoStageContextualBoundedResultPropagation(t *testing.T) {
	success := PipeLangCompile(PipeLangResult[string, string]{OK: true, Value: "source"}, "ok")
	if !success.OK || success.Value == nil || len(success.Value) != 0 { t.Fatalf("success = %%#v", success) }
	incoming := PipeLangCompile(PipeLangResult[string, string]{Error: "scan failed"}, "ok")
	if incoming.OK || incoming.Error != "scan failed" || incoming.Value != nil { t.Fatalf("incoming = %%#v", incoming) }
	first := PipeLangCompile(PipeLangResult[string, string]{OK: true, Value: "source"}, "first")
	if first.OK || first.Error != "normalize failed" || first.Value != nil { t.Fatalf("first = %%#v", first) }
	second := PipeLangCompile(PipeLangResult[string, string]{OK: true, Value: "source"}, "second")
	if second.OK || second.Error != "finish failed" || second.Value != nil { t.Fatalf("second = %%#v", second) }
}
`, gobackend.PackageName)))
}

func TestV650TwoStageContextualBoundedResultPropagationMatrixAndGate(t *testing.T) {
	input := semanticTestModuleSet("compiler.selfhosting", []ModuleInput{testModule("compiler.selfhosting", "two-stage-contextual-bounded-gate.pipe", twoStageContextualBoundedResultPropagationSource)}, nil)
	input.LanguageContract = PipeLangLanguageContractV640
	if err := AnalyzeSemanticModuleSet(input).Error(); err == nil {
		t.Fatal("v0.64.0 accepted a two-stage contextual same-payload transition")
	}
	for _, method := range []string{"Compile", "Validate", "Repeat"} {
		t.Run(method, func(t *testing.T) {
			_, _, program := twoStageContextualBoundedProgram(t, method)
			if err := coreir.ValidateProgram(program); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestV650TwoStageContextualBoundedResultPropagationRejectsExcludedSource(t *testing.T) {
	valid := `public Record Token { public string Text; } public Class Root { public Result<string, string> First(string raw, string context) => ok<string, string>(raw); public Result<List<Token>, string> Second(string raw, string context) => ok<List<Token>, string>(empty_list<Token>()); public Result<List<Token>, string> Compile(Result<string, string> input, string context) { string raw = propagate(input); Result<string, string> firstCarrier = First(raw, context); string first = propagate(firstCarrier); return Second(first, context); } }`
	threeStages := `public Record Token { public string Text; } public Class Root { public Result<string, string> First(string raw, string context) => ok<string, string>(raw); public Result<List<Token>, string> Second(string raw, string context) => ok<List<Token>, string>(empty_list<Token>()); public Result<string, string> Third(List<Token> tokens, string context) => ok<string, string>(context); public Result<string, string> Compile(Result<string, string> input, string context) { string raw = propagate(input); Result<string, string> firstCarrier = First(raw, context); string first = propagate(firstCarrier); Result<List<Token>, string> secondCarrier = Second(first, context); List<Token> second = propagate(secondCarrier); return Third(second, context); } }`
	tests := []struct {
		name   string
		source string
	}{
		{name: "three-stage same-payload transition", source: threeStages},
		{name: "third caller parameter", source: strings.Replace(valid, "input, string context)", "input, string context, string extra)", 1)},
		{name: "computed context", source: strings.Replace(valid, "First(raw, context)", "First(raw, trim(context))", 1)},
		{name: "ordinary local gap", source: strings.Replace(valid, "string first = propagate(firstCarrier);", "string first = propagate(firstCarrier); string gap = context;", 1)},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			input := semanticTestModuleSet("compiler.selfhosting", []ModuleInput{testModule("compiler.selfhosting", test.name+".pipe", test.source)}, nil)
			input.LanguageContract = PipeLangLanguageContractV650
			if err := AnalyzeSemanticModuleSet(input).Error(); err == nil {
				t.Fatal("excluded v0.65.0 source was accepted")
			}
		})
	}
}

func TestV650TwoStageContextualBoundedResultPropagationRejectsMalformedCoreAndBackend(t *testing.T) {
	_, _, program := twoStageContextualBoundedProgram(t, "Compile")
	function := coreFunctionNamed(t, program, "Compile")
	wrongContext := 0
	function.Body.ImmutableLocal.Return.ImmutableLocal.Initializer.Call.Arguments[1].Parameter = &wrongContext
	for index := range program.Functions {
		if program.Functions[index].Name == "Compile" {
			program.Functions[index] = function
		}
	}
	if err := coreir.ValidateProgram(program); err == nil || !strings.Contains(err.Error(), "v0.65.0 exact two-stage contextual bounded Result propagation") {
		t.Fatalf("Core validator accepted malformed v0.65.0 context: %v", err)
	}
	if _, err := gobackend.Generate(program); err == nil {
		t.Fatal("Go backend accepted malformed v0.65.0 Core")
	}
}

func TestV650PreservesContextualResultPropagationContracts(t *testing.T) {
	for name, source := range map[string]string{
		"v0.62 chain":                  contextualCrossPayloadResultPropagationSource,
		"v0.64 one-stage same-payload": singleStageContextualSamePayloadSource,
	} {
		t.Run(name, func(t *testing.T) {
			input := semanticTestModuleSet("compiler.selfhosting", []ModuleInput{testModule("compiler.selfhosting", name+".pipe", source)}, nil)
			input.LanguageContract = PipeLangLanguageContractV650
			if err := AnalyzeSemanticModuleSet(input).Error(); err != nil {
				t.Fatal(err)
			}
		})
	}
}
