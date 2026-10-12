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

const twoStageCrossPayloadResultPropagationSource = `public Record Token { public string Text; }
public Record SyntaxNode { public string Kind; }
public Class CompilerPipeline {
	public Result<List<Token>, string> BuildTokens(string source) => source == "reject" ? err<List<Token>, string>("tokenize failed") : ok<List<Token>, string>(empty_list<Token>());
	public Result<List<SyntaxNode>, string> BuildSyntax(List<Token> tokens) { List<SyntaxNode> nodes = empty_list<SyntaxNode>(); return ok<List<SyntaxNode>, string>(nodes); }
	public Result<string, string> SummarizeTokens(List<Token> tokens) { string summary = "tokens"; return ok<string, string>(summary); }
	public Result<List<Token>, string> RebuildTokens(string summary) { List<Token> tokens = empty_list<Token>(); return ok<List<Token>, string>(tokens); }
	public Result<List<SyntaxNode>, string> ScanToSyntax(Result<string, string> scanned) {
		string source = propagate(scanned);
		Result<List<Token>, string> tokenized = BuildTokens(source);
		List<Token> tokens = propagate(tokenized);
		return BuildSyntax(tokens);
	}
	public Result<List<Token>, string> TokensRoundTrip(Result<List<Token>, string> tokenized) {
		List<Token> tokens = propagate(tokenized);
		Result<string, string> summarized = SummarizeTokens(tokens);
		string summary = propagate(summarized);
		return RebuildTokens(summary);
	}
}`

func TestV600TwoStageCrossPayloadResultPropagationPipeline(t *testing.T) {
	input := semanticTestModuleSet("compiler.selfhosting", []ModuleInput{testModule("compiler.selfhosting", "two-stage-cross-payload-result.pipe", twoStageCrossPayloadResultPropagationSource)}, nil)
	input.LanguageContract = PipeLangLanguageContractV600
	analysis := AnalyzeSemanticModuleSet(input)
	if err := analysis.Error(); err != nil {
		t.Fatal(err)
	}
	projection, err := BuildSemanticProjection(analysis)
	if err != nil {
		t.Fatal(err)
	}
	if projection.LanguageContract != PipeLangLanguageContractV600 || projection.Schema != PipeLangSemanticProjectionVersion || projection.CompilerContract != PipeLangCompilerContract {
		t.Fatalf("projection contract = %#v", projection)
	}

	identity := semanticMethodNamed(t, analysis, "ScanToSyntax").Identity
	typed, err := LowerSemanticMethodToHIR(analysis, identity)
	if err != nil {
		t.Fatal(err)
	}
	function := hirFunctionNamed(t, typed, "ScanToSyntax")
	first := function.Body.ImmutableLocal
	if typed.LanguageContract != coreir.LanguageContractV600 || first == nil || first.Binding.Position != 1 || first.Initializer.Kind != hir.ExprPropagate || first.Return == nil || first.Return.Kind != hir.ExprImmutableLocal {
		t.Fatalf("first two-stage HIR local = %#v", function.Body)
	}
	intermediate := first.Return.ImmutableLocal
	if intermediate == nil || intermediate.Binding.Position != 2 || intermediate.Initializer.Kind != hir.ExprCall || intermediate.Return == nil || intermediate.Return.Kind != hir.ExprImmutableLocal {
		t.Fatalf("intermediate two-stage HIR local = %#v", intermediate)
	}
	second := intermediate.Return.ImmutableLocal
	if second == nil || second.Binding.Position != 3 || second.Initializer.Kind != hir.ExprPropagate || second.Return == nil || second.Return.Kind != hir.ExprCall {
		t.Fatalf("second two-stage HIR local = %#v", second)
	}

	program, err := LowerHIRToCore(typed)
	if err != nil {
		t.Fatal(err)
	}
	coreFunction := coreFunctionNamed(t, program, "ScanToSyntax")
	coreFirst := coreFunction.Body.ImmutableLocal
	if program.LanguageContract != coreir.LanguageContractV600 || coreFirst == nil || coreFirst.Position != 1 || coreFirst.Initializer == nil || coreFirst.Initializer.Propagate == nil || coreFirst.Return == nil || coreFirst.Return.ImmutableLocal == nil {
		t.Fatalf("first two-stage Core local = %#v", coreFunction.Body)
	}
	coreIntermediate := coreFirst.Return.ImmutableLocal
	if coreIntermediate.Position != 2 || coreIntermediate.Initializer == nil || coreIntermediate.Initializer.Call == nil || coreIntermediate.Return == nil || coreIntermediate.Return.ImmutableLocal == nil {
		t.Fatalf("intermediate two-stage Core local = %#v", coreIntermediate)
	}
	coreSecond := coreIntermediate.Return.ImmutableLocal
	if coreSecond.Position != 3 || coreSecond.Initializer == nil || coreSecond.Initializer.Propagate == nil || coreSecond.Initializer.Propagate.Value == nil || coreSecond.Initializer.Propagate.Value.Parameter == nil || *coreSecond.Initializer.Propagate.Value.Parameter != 2 || coreSecond.Return == nil || coreSecond.Return.Call == nil || coreSecond.Return.Call.Arguments[0].Parameter == nil || *coreSecond.Return.Call.Arguments[0].Parameter != 3 {
		t.Fatalf("second two-stage Core local = %#v", coreSecond)
	}

	entry := coreir.SemanticIdentity{PackageID: string(identity.PackageID), Path: string(identity.Path)}
	carrierType := coreFunction.Parameters[0].Type
	successCarrier := coreeval.Outcome{OK: true, Value: coreeval.Value{Type: carrierType.Result.Success, String: "class Root {}"}}
	success, err := coreeval.EvaluateProgram(program, entry, []coreeval.Value{{Type: carrierType, Result: &successCarrier}})
	if err != nil || !success.OK || success.Value.Type.Kind != coreir.TypeList || success.Value.List == nil || len(success.Value.List) != 0 {
		t.Fatalf("success = %#v, %v", success, err)
	}
	incomingFailureValue := coreeval.Value{Type: carrierType.Result.Failure, String: "scan failed"}
	incomingFailureCarrier := coreeval.Outcome{Value: coreeval.Value{Type: carrierType.Result.Success}, Failure: &incomingFailureValue}
	incomingFailure, err := coreeval.EvaluateProgram(program, entry, []coreeval.Value{{Type: carrierType, Result: &incomingFailureCarrier}})
	if err != nil || incomingFailure.OK || incomingFailure.Failure == nil || incomingFailure.Failure.String != "scan failed" || !coreir.TypeEqual(incomingFailure.Value.Type, coreFunction.ReturnType.Result.Success) || incomingFailure.Value.List != nil {
		t.Fatalf("incoming failure = %#v, %v", incomingFailure, err)
	}
	intermediateFailureCarrier := coreeval.Outcome{OK: true, Value: coreeval.Value{Type: carrierType.Result.Success, String: "reject"}}
	intermediateFailure, err := coreeval.EvaluateProgram(program, entry, []coreeval.Value{{Type: carrierType, Result: &intermediateFailureCarrier}})
	if err != nil || intermediateFailure.OK || intermediateFailure.Failure == nil || intermediateFailure.Failure.String != "tokenize failed" || !coreir.TypeEqual(intermediateFailure.Value.Type, coreFunction.ReturnType.Result.Success) || intermediateFailure.Value.List != nil {
		t.Fatalf("intermediate failure = %#v, %v", intermediateFailure, err)
	}
	malformed := coreeval.Outcome{Value: coreeval.Value{Type: carrierType.Result.Success, String: "not zero"}, Failure: &incomingFailureValue}
	if _, err := coreeval.EvaluateProgram(program, entry, []coreeval.Value{{Type: carrierType, Result: &malformed}}); err == nil {
		t.Fatal("evaluator accepted a failed incoming carrier with a non-canonical source payload")
	}
	invalidFailureValue := coreeval.Value{Type: carrierType.Result.Failure, String: string([]byte{0xff})}
	invalidFailureCarrier := coreeval.Outcome{Value: coreeval.Value{Type: carrierType.Result.Success}, Failure: &invalidFailureValue}
	if _, err := coreeval.EvaluateProgram(program, entry, []coreeval.Value{{Type: carrierType, Result: &invalidFailureCarrier}}); err == nil {
		t.Fatal("evaluator accepted a failed incoming carrier with invalid error text")
	}

	generated, err := gobackend.Generate(program)
	if err != nil {
		t.Fatal(err)
	}
	generatedAgain, err := gobackend.Generate(program)
	if err != nil || string(generatedAgain) != string(generated) {
		t.Fatalf("generated Go is not deterministic: %v", err)
	}
	for _, fragment := range []string{"p1 := p0.Value", "PipeLangBuildTokens(p1)", "if !p2.OK", "PipeLangBuildSyntax(pipelangCloneListTestPackageCompilerSelfhostingToken(p3))"} {
		if !strings.Contains(string(generated), fragment) {
			t.Fatalf("generated Go lacks two-stage cross-payload propagation %q:\n%s", fragment, generated)
		}
	}
	compileAndRunGeneratedGoFiles(t, generated, []byte(fmt.Sprintf(`package %s

import "testing"

func TestGeneratedTwoStageCrossPayloadResultPropagation(t *testing.T) {
	success := PipeLangScanToSyntax(PipeLangResult[string, string]{OK: true, Value: "class Root {}"})
	if !success.OK || success.Value == nil || len(success.Value) != 0 { t.Fatalf("success = %%#v", success) }
	incoming := PipeLangScanToSyntax(PipeLangResult[string, string]{Error: "scan failed"})
	if incoming.OK || incoming.Error != "scan failed" || incoming.Value != nil { t.Fatalf("incoming failure = %%#v", incoming) }
	intermediate := PipeLangScanToSyntax(PipeLangResult[string, string]{OK: true, Value: "reject"})
	if intermediate.OK || intermediate.Error != "tokenize failed" || intermediate.Value != nil { t.Fatalf("intermediate failure = %%#v", intermediate) }
}

func TestGeneratedTwoStageRejectsInvalidErrorText(t *testing.T) {
	defer func() { if recover() == nil { t.Fatal("invalid error text did not panic") } }()
	PipeLangScanToSyntax(PipeLangResult[string, string]{Error: string([]byte{0xff})})
}

func TestGeneratedTwoStageRejectsMalformedCarrier(t *testing.T) {
	defer func() { if recover() == nil { t.Fatal("malformed carrier did not panic") } }()
	PipeLangScanToSyntax(PipeLangResult[string, string]{Value: "not zero", Error: "scan failed"})
}
`, gobackend.PackageName)))
}

func TestV600TwoStageCrossPayloadAllowsSourceAndTargetPayloadEquality(t *testing.T) {
	input := semanticTestModuleSet("compiler.selfhosting", []ModuleInput{testModule("compiler.selfhosting", "two-stage-round-trip.pipe", twoStageCrossPayloadResultPropagationSource)}, nil)
	input.LanguageContract = PipeLangLanguageContractV600
	analysis := AnalyzeSemanticModuleSet(input)
	if err := analysis.Error(); err != nil {
		t.Fatal(err)
	}
	identity := semanticMethodNamed(t, analysis, "TokensRoundTrip").Identity
	typed, err := LowerSemanticMethodToHIR(analysis, identity)
	if err != nil {
		t.Fatal(err)
	}
	program, err := LowerHIRToCore(typed)
	if err != nil {
		t.Fatal(err)
	}
	function := coreFunctionNamed(t, program, "TokensRoundTrip")
	entry := coreir.SemanticIdentity{PackageID: string(identity.PackageID), Path: string(identity.Path)}
	carrierType := function.Parameters[0].Type
	tokenType := carrierType.Result.Success.List.Element
	inputToken := coreeval.Value{Type: tokenType, Record: []coreeval.Value{{Type: tokenType.Record.Fields[0].Type, String: "input"}}}
	successCarrier := coreeval.Outcome{OK: true, Value: coreeval.Value{Type: carrierType.Result.Success, List: []coreeval.Value{inputToken}}}
	success, err := coreeval.EvaluateProgram(program, entry, []coreeval.Value{{Type: carrierType, Result: &successCarrier}})
	if err != nil || !success.OK || success.Value.List == nil || len(success.Value.List) != 0 {
		t.Fatalf("round-trip success = %#v, %v", success, err)
	}
	failureText := coreeval.Value{Type: carrierType.Result.Failure, String: "token failure"}
	failureCarrier := coreeval.Outcome{Value: coreeval.Value{Type: carrierType.Result.Success}, Failure: &failureText}
	failure, err := coreeval.EvaluateProgram(program, entry, []coreeval.Value{{Type: carrierType, Result: &failureCarrier}})
	if err != nil || failure.OK || failure.Failure == nil || failure.Failure.String != "token failure" || failure.Value.List != nil {
		t.Fatalf("round-trip failure = %#v, %v", failure, err)
	}
	generated, err := gobackend.Generate(program)
	if err != nil {
		t.Fatal(err)
	}
	compileAndRunGeneratedGoFiles(t, generated, []byte(fmt.Sprintf(`package %s

import "testing"

func TestGeneratedRoundTripPayloadEquality(t *testing.T) {
	input := []PipeLangRecordTestPackageCompilerSelfhostingToken{{Text: "input"}}
	success := PipeLangTokensRoundTrip(PipeLangResult[[]PipeLangRecordTestPackageCompilerSelfhostingToken, string]{OK: true, Value: input})
	if !success.OK || success.Value == nil || len(success.Value) != 0 { t.Fatalf("success = %%#v", success) }
	failure := PipeLangTokensRoundTrip(PipeLangResult[[]PipeLangRecordTestPackageCompilerSelfhostingToken, string]{Error: "token failure"})
	if failure.OK || failure.Error != "token failure" || failure.Value != nil { t.Fatalf("failure = %%#v", failure) }
}
`, gobackend.PackageName)))
}

func TestV600TwoStageCrossPayloadRejectsExcludedSource(t *testing.T) {
	valid := `public Record Token { public string Text; } public Record Node { public string Kind; } public Class Root { public Result<List<Token>, string> First(string raw) { List<Token> tokens = empty_list<Token>(); return ok<List<Token>, string>(tokens); } public Result<List<Node>, string> Second(List<Token> tokens) { List<Node> nodes = empty_list<Node>(); return ok<List<Node>, string>(nodes); } public Result<List<Node>, string> Compile(Result<string, string> input) { string raw = propagate(input); Result<List<Token>, string> tokensCarrier = First(raw); List<Token> tokens = propagate(tokensCarrier); return Second(tokens); } }`
	tests := []struct {
		name     string
		contract LanguageContract
		source   string
	}{
		{name: "prior contract", contract: PipeLangLanguageContractV590, source: valid},
		{name: "extra parameter", contract: PipeLangLanguageContractV600, source: strings.Replace(valid, "input)", "input, string context)", 1)},
		{name: "missing intermediate carrier", contract: PipeLangLanguageContractV600, source: strings.Replace(valid, "Result<List<Token>, string> tokensCarrier = First(raw); List<Token> tokens = propagate(tokensCarrier);", "List<Token> tokens = propagate(First(raw));", 1)},
		{name: "same first payload", contract: PipeLangLanguageContractV600, source: strings.Replace(valid, "public Result<List<Token>, string> First(string raw) { List<Token> tokens = empty_list<Token>(); return ok<List<Token>, string>(tokens); }", "public Result<string, string> First(string raw) => ok<string, string>(raw);", 1)},
		{name: "same second payload", contract: PipeLangLanguageContractV600, source: `public Record Token { public string Text; } public Class Root { public Result<List<Token>, string> First(string raw) { List<Token> tokens = empty_list<Token>(); return ok<List<Token>, string>(tokens); } public Result<List<Token>, string> Second(List<Token> tokens) => ok<List<Token>, string>(tokens); public Result<List<Token>, string> Compile(Result<string, string> input) { string raw = propagate(input); Result<List<Token>, string> tokensCarrier = First(raw); List<Token> tokens = propagate(tokensCarrier); return Second(tokens); } }`},
		{name: "extra local", contract: PipeLangLanguageContractV600, source: strings.Replace(valid, "return Second(tokens);", "string extra = \"extra\"; return Second(tokens);", 1)},
		{name: "computed first argument", contract: PipeLangLanguageContractV600, source: strings.Replace(valid, "First(raw)", "First(trim(raw))", 1)},
		{name: "private first helper", contract: PipeLangLanguageContractV600, source: strings.Replace(valid, "public Result<List<Token>, string> First", "private Result<List<Token>, string> First", 1)},
		{name: "wrong second argument", contract: PipeLangLanguageContractV600, source: strings.Replace(valid, "Second(tokens)", "Second(empty_list<Token>())", 1)},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			input := semanticTestModuleSet("compiler.selfhosting", []ModuleInput{testModule("compiler.selfhosting", test.name+".pipe", test.source)}, nil)
			input.LanguageContract = test.contract
			if err := AnalyzeSemanticModuleSet(input).Error(); err == nil {
				t.Fatal("excluded two-stage cross-payload Result propagation source was accepted")
			}
		})
	}
}

func TestV600TwoStageCrossPayloadRejectsMalformedCoreAndBackend(t *testing.T) {
	input := semanticTestModuleSet("compiler.selfhosting", []ModuleInput{testModule("compiler.selfhosting", "two-stage-cross-payload-core.pipe", twoStageCrossPayloadResultPropagationSource)}, nil)
	input.LanguageContract = PipeLangLanguageContractV600
	analysis := AnalyzeSemanticModuleSet(input)
	if err := analysis.Error(); err != nil {
		t.Fatal(err)
	}
	typed, err := LowerSemanticMethodToHIR(analysis, semanticMethodNamed(t, analysis, "ScanToSyntax").Identity)
	if err != nil {
		t.Fatal(err)
	}
	program, err := LowerHIRToCore(typed)
	if err != nil {
		t.Fatal(err)
	}
	for index := range program.Functions {
		if program.Functions[index].Name != "ScanToSyntax" {
			continue
		}
		badPosition := 1
		program.Functions[index].Body.ImmutableLocal.Return.ImmutableLocal.Return.ImmutableLocal.Initializer.Propagate.Value.Parameter = &badPosition
	}
	if err := coreir.ValidateProgram(program); err == nil {
		t.Fatal("Core validator accepted intermediate propagation that bypasses its explicit carrier local")
	}
	if _, err := gobackend.Generate(program); err == nil {
		t.Fatal("Go backend accepted malformed two-stage cross-payload propagation Core")
	}
}
