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

const generalizedCrossPayloadResultPropagationSource = `public Record Token { public string Text; }
public Record SyntaxNode { public string Kind; }
public Record TypedNode { public string Type; }
public Class CompilerPipeline {
	public Result<List<Token>, string> Tokenize(string source) { List<Token> tokens = empty_list<Token>(); return ok<List<Token>, string>(tokens); }
	public Result<List<SyntaxNode>, string> Parse(List<Token> tokens) { List<SyntaxNode> nodes = empty_list<SyntaxNode>(); return ok<List<SyntaxNode>, string>(nodes); }
	public Result<List<TypedNode>, string> Bind(List<SyntaxNode> nodes) { List<TypedNode> typed = empty_list<TypedNode>(); return ok<List<TypedNode>, string>(typed); }
	public Result<string, string> Emit(List<TypedNode> typed) { string output = "compiled"; return ok<string, string>(output); }
	public Result<List<Token>, string> TokenizeFailure(string source) => true ? err<List<Token>, string>("tokenize failed") : ok<List<Token>, string>(empty_list<Token>());
	public Result<List<SyntaxNode>, string> ParseFailure(List<Token> tokens) => true ? err<List<SyntaxNode>, string>("parse failed") : ok<List<SyntaxNode>, string>(empty_list<SyntaxNode>());
	public Result<List<TypedNode>, string> BindFailure(List<SyntaxNode> nodes) => true ? err<List<TypedNode>, string>("bind failed") : ok<List<TypedNode>, string>(empty_list<TypedNode>());
	public Result<string, string> EmitFailure(List<TypedNode> typed) => true ? err<string, string>("emit failed") : ok<string, string>("");
	public Result<string, string> Compile(Result<string, string> scanned) {
		string source = propagate(scanned);
		Result<List<Token>, string> tokenized = Tokenize(source);
		List<Token> tokens = propagate(tokenized);
		Result<List<SyntaxNode>, string> parsed = Parse(tokens);
		List<SyntaxNode> syntax = propagate(parsed);
		Result<List<TypedNode>, string> bound = Bind(syntax);
		List<TypedNode> typed = propagate(bound);
		return Emit(typed);
	}
	public Result<string, string> FailTokenize(Result<string, string> scanned) {
		string source = propagate(scanned);
		Result<List<Token>, string> tokenized = TokenizeFailure(source);
		List<Token> tokens = propagate(tokenized);
		Result<List<SyntaxNode>, string> parsed = ParseFailure(tokens);
		List<SyntaxNode> syntax = propagate(parsed);
		Result<List<TypedNode>, string> bound = BindFailure(syntax);
		List<TypedNode> typed = propagate(bound);
		return EmitFailure(typed);
	}
	public Result<string, string> FailParse(Result<string, string> scanned) {
		string source = propagate(scanned);
		Result<List<Token>, string> tokenized = Tokenize(source);
		List<Token> tokens = propagate(tokenized);
		Result<List<SyntaxNode>, string> parsed = ParseFailure(tokens);
		List<SyntaxNode> syntax = propagate(parsed);
		Result<List<TypedNode>, string> bound = BindFailure(syntax);
		List<TypedNode> typed = propagate(bound);
		return EmitFailure(typed);
	}
	public Result<string, string> FailBind(Result<string, string> scanned) {
		string source = propagate(scanned);
		Result<List<Token>, string> tokenized = Tokenize(source);
		List<Token> tokens = propagate(tokenized);
		Result<List<SyntaxNode>, string> parsed = Parse(tokens);
		List<SyntaxNode> syntax = propagate(parsed);
		Result<List<TypedNode>, string> bound = BindFailure(syntax);
		List<TypedNode> typed = propagate(bound);
		return EmitFailure(typed);
	}
	public Result<string, string> FailEmit(Result<string, string> scanned) {
		string source = propagate(scanned);
		Result<List<Token>, string> tokenized = Tokenize(source);
		List<Token> tokens = propagate(tokenized);
		Result<List<SyntaxNode>, string> parsed = Parse(tokens);
		List<SyntaxNode> syntax = propagate(parsed);
		Result<List<TypedNode>, string> bound = Bind(syntax);
		List<TypedNode> typed = propagate(bound);
		return EmitFailure(typed);
	}
}`

func generalizedCrossPayloadProgram(t *testing.T, method string) (*Analysis, SemanticIdentity, coreir.Program) {
	t.Helper()
	input := semanticTestModuleSet("compiler.selfhosting", []ModuleInput{testModule("compiler.selfhosting", "generalized-cross-payload-result.pipe", generalizedCrossPayloadResultPropagationSource)}, nil)
	input.LanguageContract = PipeLangLanguageContractV610
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

func TestV610GeneralizedCrossPayloadResultPropagationPipeline(t *testing.T) {
	analysis, identity, program := generalizedCrossPayloadProgram(t, "Compile")
	projection, err := BuildSemanticProjection(analysis)
	if err != nil {
		t.Fatal(err)
	}
	if projection.LanguageContract != PipeLangLanguageContractV610 || projection.Schema != PipeLangSemanticProjectionVersion || projection.CompilerContract != PipeLangCompilerContract {
		t.Fatalf("projection contract = %#v", projection)
	}

	typed, err := LowerSemanticMethodToHIR(analysis, identity)
	if err != nil {
		t.Fatal(err)
	}
	function := hirFunctionNamed(t, typed, "Compile")
	first := function.Body.ImmutableLocal
	if typed.LanguageContract != coreir.LanguageContractV610 || first == nil || first.Binding.Position != 1 || first.Initializer.Kind != hir.ExprPropagate {
		t.Fatalf("first generalized HIR local = %#v", function.Body)
	}
	firstCarrier := first.Return.ImmutableLocal
	secondPayload := firstCarrier.Return.ImmutableLocal
	secondCarrier := secondPayload.Return.ImmutableLocal
	thirdPayload := secondCarrier.Return.ImmutableLocal
	thirdCarrier := thirdPayload.Return.ImmutableLocal
	fourthPayload := thirdCarrier.Return.ImmutableLocal
	if firstCarrier == nil || firstCarrier.Binding.Position != 2 || firstCarrier.Initializer.Kind != hir.ExprCall || secondPayload == nil || secondPayload.Binding.Position != 3 || secondPayload.Initializer.Kind != hir.ExprPropagate || secondCarrier == nil || secondCarrier.Binding.Position != 4 || secondCarrier.Initializer.Kind != hir.ExprCall || thirdPayload == nil || thirdPayload.Binding.Position != 5 || thirdPayload.Initializer.Kind != hir.ExprPropagate || thirdCarrier == nil || thirdCarrier.Binding.Position != 6 || thirdCarrier.Initializer.Kind != hir.ExprCall || fourthPayload == nil || fourthPayload.Binding.Position != 7 || fourthPayload.Initializer.Kind != hir.ExprPropagate || fourthPayload.Return.Kind != hir.ExprCall {
		t.Fatalf("generalized HIR chain = %#v", function.Body)
	}

	coreFunction := coreFunctionNamed(t, program, "Compile")
	coreFirst := coreFunction.Body.ImmutableLocal
	coreFirstCarrier := coreFirst.Return.ImmutableLocal
	coreSecondPayload := coreFirstCarrier.Return.ImmutableLocal
	coreSecondCarrier := coreSecondPayload.Return.ImmutableLocal
	coreThirdPayload := coreSecondCarrier.Return.ImmutableLocal
	coreThirdCarrier := coreThirdPayload.Return.ImmutableLocal
	coreFourthPayload := coreThirdCarrier.Return.ImmutableLocal
	if program.LanguageContract != coreir.LanguageContractV610 || coreFirst.Position != 1 || coreFirstCarrier.Position != 2 || coreSecondPayload.Position != 3 || coreSecondCarrier.Position != 4 || coreThirdPayload.Position != 5 || coreThirdCarrier.Position != 6 || coreFourthPayload.Position != 7 || coreFourthPayload.Initializer == nil || coreFourthPayload.Initializer.Propagate == nil || coreFourthPayload.Initializer.Propagate.Value == nil || coreFourthPayload.Initializer.Propagate.Value.Parameter == nil || *coreFourthPayload.Initializer.Propagate.Value.Parameter != 6 || coreFourthPayload.Return == nil || coreFourthPayload.Return.Call == nil || coreFourthPayload.Return.Call.Arguments[0].Parameter == nil || *coreFourthPayload.Return.Call.Arguments[0].Parameter != 7 {
		t.Fatalf("generalized Core chain = %#v", coreFunction.Body)
	}

	entry := coreir.SemanticIdentity{PackageID: string(identity.PackageID), Path: string(identity.Path)}
	carrierType := coreFunction.Parameters[0].Type
	successCarrier := coreeval.Outcome{OK: true, Value: coreeval.Value{Type: carrierType.Result.Success, String: "class Root {}"}}
	success, err := coreeval.EvaluateProgram(program, entry, []coreeval.Value{{Type: carrierType, Result: &successCarrier}})
	if err != nil || !success.OK || success.Value.String != "compiled" {
		t.Fatalf("success = %#v, %v", success, err)
	}
	failureText := coreeval.Value{Type: carrierType.Result.Failure, String: "scan failed"}
	incomingCarrier := coreeval.Outcome{Value: coreeval.Value{Type: carrierType.Result.Success}, Failure: &failureText}
	incoming, err := coreeval.EvaluateProgram(program, entry, []coreeval.Value{{Type: carrierType, Result: &incomingCarrier}})
	if err != nil || incoming.OK || incoming.Failure == nil || incoming.Failure.String != "scan failed" || incoming.Value.String != "" {
		t.Fatalf("incoming failure = %#v, %v", incoming, err)
	}
	malformedCarrier := coreeval.Outcome{Value: coreeval.Value{Type: carrierType.Result.Success, String: "not zero"}, Failure: &failureText}
	if _, err := coreeval.EvaluateProgram(program, entry, []coreeval.Value{{Type: carrierType, Result: &malformedCarrier}}); err == nil {
		t.Fatal("evaluator accepted a failed incoming carrier with a non-canonical source payload")
	}
	invalidFailure := coreeval.Value{Type: carrierType.Result.Failure, String: string([]byte{0xff})}
	invalidCarrier := coreeval.Outcome{Value: coreeval.Value{Type: carrierType.Result.Success}, Failure: &invalidFailure}
	if _, err := coreeval.EvaluateProgram(program, entry, []coreeval.Value{{Type: carrierType, Result: &invalidCarrier}}); err == nil {
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
	for _, fragment := range []string{"PipeLangTokenize(p1)", "if !p2.OK", "PipeLangParse(pipelangCloneListTestPackageCompilerSelfhostingToken(p3))", "if !p4.OK", "PipeLangBind(pipelangCloneListTestPackageCompilerSelfhostingSyntaxnode(p5))", "if !p6.OK", "PipeLangEmit(pipelangCloneListTestPackageCompilerSelfhostingTypednode(p7))"} {
		if !strings.Contains(string(generated), fragment) {
			t.Fatalf("generated Go lacks generalized cross-payload propagation %q:\n%s", fragment, generated)
		}
	}
	compileAndRunGeneratedGoFiles(t, generated, []byte(fmt.Sprintf(`package %s

import "testing"

func TestGeneratedGeneralizedCrossPayloadResultPropagation(t *testing.T) {
	success := PipeLangCompile(PipeLangResult[string, string]{OK: true, Value: "class Root {}"})
	if !success.OK || success.Value != "compiled" { t.Fatalf("success = %%#v", success) }
	incoming := PipeLangCompile(PipeLangResult[string, string]{Error: "scan failed"})
	if incoming.OK || incoming.Error != "scan failed" || incoming.Value != "" { t.Fatalf("incoming failure = %%#v", incoming) }
}

func TestGeneratedGeneralizedRejectsInvalidErrorText(t *testing.T) {
	defer func() { if recover() == nil { t.Fatal("invalid error text did not panic") } }()
	PipeLangCompile(PipeLangResult[string, string]{Error: string([]byte{0xff})})
}

func TestGeneratedGeneralizedRejectsMalformedCarrier(t *testing.T) {
	defer func() { if recover() == nil { t.Fatal("malformed carrier did not panic") } }()
	PipeLangCompile(PipeLangResult[string, string]{Value: "not zero", Error: "scan failed"})
}
`, gobackend.PackageName)))
}

func TestV610GeneralizedCrossPayloadResultPropagationShortCircuitsEveryIntermediateFailure(t *testing.T) {
	for _, test := range []struct {
		method string
		want   string
	}{
		{method: "FailTokenize", want: "tokenize failed"},
		{method: "FailParse", want: "parse failed"},
		{method: "FailBind", want: "bind failed"},
		{method: "FailEmit", want: "emit failed"},
	} {
		t.Run(test.method, func(t *testing.T) {
			_, identity, program := generalizedCrossPayloadProgram(t, test.method)
			function := coreFunctionNamed(t, program, test.method)
			carrierType := function.Parameters[0].Type
			carrier := coreeval.Outcome{OK: true, Value: coreeval.Value{Type: carrierType.Result.Success, String: "source"}}
			entry := coreir.SemanticIdentity{PackageID: string(identity.PackageID), Path: string(identity.Path)}
			outcome, err := coreeval.EvaluateProgram(program, entry, []coreeval.Value{{Type: carrierType, Result: &carrier}})
			if err != nil || outcome.OK || outcome.Failure == nil || outcome.Failure.String != test.want || outcome.Value.String != "" {
				t.Fatalf("outcome = %#v, %v", outcome, err)
			}
			generated, err := gobackend.Generate(program)
			if err != nil {
				t.Fatal(err)
			}
			compileAndRunGeneratedGoFiles(t, generated, []byte(fmt.Sprintf(`package %s

import "testing"

func TestGeneratedStageFailure(t *testing.T) {
	outcome := PipeLang%s(PipeLangResult[string, string]{OK: true, Value: "source"})
	if outcome.OK || outcome.Error != %q || outcome.Value != "" { t.Fatalf("outcome = %%#v", outcome) }
}
`, gobackend.PackageName, test.method, test.want)))
		})
	}
}

func TestV610GeneralizedCrossPayloadResultPropagationRejectsExcludedSource(t *testing.T) {
	valid := `public Record Token { public string Text; } public Record Node { public string Kind; } public Record Typed { public string Type; } public Class Root { public Result<List<Token>, string> First(string raw) { List<Token> values = empty_list<Token>(); return ok<List<Token>, string>(values); } public Result<List<Node>, string> Second(List<Token> values) { List<Node> nodes = empty_list<Node>(); return ok<List<Node>, string>(nodes); } public Result<List<Typed>, string> Third(List<Node> nodes) { List<Typed> typed = empty_list<Typed>(); return ok<List<Typed>, string>(typed); } public Result<List<Typed>, string> Compile(Result<string, string> input) { string raw = propagate(input); Result<List<Token>, string> firstCarrier = First(raw); List<Token> first = propagate(firstCarrier); Result<List<Node>, string> secondCarrier = Second(first); List<Node> second = propagate(secondCarrier); return Third(second); } }`
	tests := []struct {
		name     string
		contract LanguageContract
		source   string
	}{
		{name: "prior contract", contract: PipeLangLanguageContractV600, source: valid},
		{name: "extra parameter", contract: PipeLangLanguageContractV610, source: strings.Replace(valid, "input)", "input, string context)", 1)},
		{name: "same adjacent payload", contract: PipeLangLanguageContractV610, source: strings.Replace(valid, "public Result<List<Token>, string> First(string raw) { List<Token> values = empty_list<Token>(); return ok<List<Token>, string>(values); }", "public Result<string, string> First(string raw) => ok<string, string>(raw);", 1)},
		{name: "computed carrier", contract: PipeLangLanguageContractV610, source: strings.Replace(valid, "Result<List<Token>, string> firstCarrier = First(raw); List<Token> first = propagate(firstCarrier);", "List<Token> first = propagate(First(raw));", 1)},
		{name: "ordinary local gap", contract: PipeLangLanguageContractV610, source: strings.Replace(valid, "List<Token> first = propagate(firstCarrier);", "List<Token> first = propagate(firstCarrier); string gap = \"gap\";", 1)},
		{name: "computed helper argument", contract: PipeLangLanguageContractV610, source: strings.Replace(valid, "First(raw)", "First(trim(raw))", 1)},
		{name: "wrong preceding payload", contract: PipeLangLanguageContractV610, source: strings.Replace(valid, "Second(first)", "Second(firstCarrier)", 1)},
		{name: "private helper", contract: PipeLangLanguageContractV610, source: strings.Replace(valid, "public Result<List<Node>, string> Second", "private Result<List<Node>, string> Second", 1)},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			input := semanticTestModuleSet("compiler.selfhosting", []ModuleInput{testModule("compiler.selfhosting", test.name+".pipe", test.source)}, nil)
			input.LanguageContract = test.contract
			if err := AnalyzeSemanticModuleSet(input).Error(); err == nil {
				t.Fatal("excluded generalized cross-payload Result propagation source was accepted")
			}
		})
	}
}

func TestV610GeneralizedCrossPayloadResultPropagationRejectsMalformedCoreAndBackend(t *testing.T) {
	_, _, program := generalizedCrossPayloadProgram(t, "Compile")
	for index := range program.Functions {
		if program.Functions[index].Name != "Compile" {
			continue
		}
		badPosition := 2
		program.Functions[index].Body.ImmutableLocal.Return.ImmutableLocal.Return.ImmutableLocal.Return.ImmutableLocal.Return.ImmutableLocal.Initializer.Propagate.Value.Parameter = &badPosition
	}
	if err := coreir.ValidateProgram(program); err == nil || !strings.Contains(err.Error(), "generalized cross-payload") {
		t.Fatalf("Core validator accepted a later propagation that bypasses its explicit carrier local: %v", err)
	}
	if _, err := gobackend.Generate(program); err == nil {
		t.Fatal("Go backend accepted malformed generalized cross-payload propagation Core")
	}
}

func TestV610PreservesTwoStageCrossPayloadResultPropagation(t *testing.T) {
	input := semanticTestModuleSet("compiler.selfhosting", []ModuleInput{testModule("compiler.selfhosting", "two-stage-v061.pipe", twoStageCrossPayloadResultPropagationSource)}, nil)
	input.LanguageContract = PipeLangLanguageContractV610
	if err := AnalyzeSemanticModuleSet(input).Error(); err != nil {
		t.Fatal(err)
	}
}
