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

const contextualCrossPayloadResultPropagationSource = `public Record Token { public string Text; }
public Record SyntaxNode { public string Kind; }
public Record TypedNode { public string Type; }
public Class ContextualCompilerPipeline {
	public Result<List<Token>, string> Tokenize(string source, string context) => context == "" ? err<List<Token>, string>("missing context") : ok<List<Token>, string>(empty_list<Token>());
	public Result<List<SyntaxNode>, string> Parse(List<Token> tokens, string context) { List<SyntaxNode> nodes = empty_list<SyntaxNode>(); return ok<List<SyntaxNode>, string>(nodes); }
	public Result<List<TypedNode>, string> Bind(List<SyntaxNode> nodes, string context) { List<TypedNode> typed = empty_list<TypedNode>(); return ok<List<TypedNode>, string>(typed); }
	public Result<string, string> Emit(List<TypedNode> typed, string context) { string output = "compiled:" + context; return ok<string, string>(output); }
	public Result<List<Token>, string> TokenizeFailure(string source, string context) => true ? err<List<Token>, string>("tokenize failed:" + context) : ok<List<Token>, string>(empty_list<Token>());
	public Result<List<SyntaxNode>, string> ParseFailure(List<Token> tokens, string context) => true ? err<List<SyntaxNode>, string>("parse failed:" + context) : ok<List<SyntaxNode>, string>(empty_list<SyntaxNode>());
	public Result<List<TypedNode>, string> BindFailure(List<SyntaxNode> nodes, string context) => true ? err<List<TypedNode>, string>("bind failed:" + context) : ok<List<TypedNode>, string>(empty_list<TypedNode>());
	public Result<string, string> EmitFailure(List<TypedNode> typed, string context) => true ? err<string, string>("emit failed:" + context) : ok<string, string>("");
	public Result<string, string> Compile(Result<string, string> scanned, string context) {
		string source = propagate(scanned);
		Result<List<Token>, string> tokenized = Tokenize(source, context);
		List<Token> tokens = propagate(tokenized);
		Result<List<SyntaxNode>, string> parsed = Parse(tokens, context);
		List<SyntaxNode> syntax = propagate(parsed);
		Result<List<TypedNode>, string> bound = Bind(syntax, context);
		List<TypedNode> typed = propagate(bound);
		return Emit(typed, context);
	}
	public Result<string, string> FailTokenize(Result<string, string> scanned, string context) {
		string source = propagate(scanned);
		Result<List<Token>, string> tokenized = TokenizeFailure(source, context);
		List<Token> tokens = propagate(tokenized);
		Result<List<SyntaxNode>, string> parsed = ParseFailure(tokens, context);
		List<SyntaxNode> syntax = propagate(parsed);
		Result<List<TypedNode>, string> bound = BindFailure(syntax, context);
		List<TypedNode> typed = propagate(bound);
		return EmitFailure(typed, context);
	}
	public Result<string, string> FailParse(Result<string, string> scanned, string context) {
		string source = propagate(scanned);
		Result<List<Token>, string> tokenized = Tokenize(source, context);
		List<Token> tokens = propagate(tokenized);
		Result<List<SyntaxNode>, string> parsed = ParseFailure(tokens, context);
		List<SyntaxNode> syntax = propagate(parsed);
		Result<List<TypedNode>, string> bound = BindFailure(syntax, context);
		List<TypedNode> typed = propagate(bound);
		return EmitFailure(typed, context);
	}
	public Result<string, string> FailBind(Result<string, string> scanned, string context) {
		string source = propagate(scanned);
		Result<List<Token>, string> tokenized = Tokenize(source, context);
		List<Token> tokens = propagate(tokenized);
		Result<List<SyntaxNode>, string> parsed = Parse(tokens, context);
		List<SyntaxNode> syntax = propagate(parsed);
		Result<List<TypedNode>, string> bound = BindFailure(syntax, context);
		List<TypedNode> typed = propagate(bound);
		return EmitFailure(typed, context);
	}
	public Result<string, string> FailEmit(Result<string, string> scanned, string context) {
		string source = propagate(scanned);
		Result<List<Token>, string> tokenized = Tokenize(source, context);
		List<Token> tokens = propagate(tokenized);
		Result<List<SyntaxNode>, string> parsed = Parse(tokens, context);
		List<SyntaxNode> syntax = propagate(parsed);
		Result<List<TypedNode>, string> bound = Bind(syntax, context);
		List<TypedNode> typed = propagate(bound);
		return EmitFailure(typed, context);
	}
}`

func contextualCrossPayloadProgram(t *testing.T, method string) (*Analysis, SemanticIdentity, coreir.Program) {
	t.Helper()
	input := semanticTestModuleSet("compiler.selfhosting", []ModuleInput{testModule("compiler.selfhosting", "contextual-cross-payload-result.pipe", contextualCrossPayloadResultPropagationSource)}, nil)
	input.LanguageContract = PipeLangLanguageContractV620
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

func TestV620ContextualCrossPayloadResultPropagationPipeline(t *testing.T) {
	analysis, identity, program := contextualCrossPayloadProgram(t, "Compile")
	projection, err := BuildSemanticProjection(analysis)
	if err != nil {
		t.Fatal(err)
	}
	if projection.LanguageContract != PipeLangLanguageContractV620 || projection.Schema != PipeLangSemanticProjectionVersion || projection.CompilerContract != PipeLangCompilerContract {
		t.Fatalf("projection contract = %#v", projection)
	}

	typed, err := LowerSemanticMethodToHIR(analysis, identity)
	if err != nil {
		t.Fatal(err)
	}
	function := hirFunctionNamed(t, typed, "Compile")
	first := function.Body.ImmutableLocal
	if typed.LanguageContract != coreir.LanguageContractV620 || first == nil || first.Binding.Position != 2 || first.Initializer.Kind != hir.ExprPropagate {
		t.Fatalf("first contextual HIR local = %#v", function.Body)
	}
	firstCarrier := first.Return.ImmutableLocal
	secondPayload := firstCarrier.Return.ImmutableLocal
	secondCarrier := secondPayload.Return.ImmutableLocal
	thirdPayload := secondCarrier.Return.ImmutableLocal
	thirdCarrier := thirdPayload.Return.ImmutableLocal
	fourthPayload := thirdCarrier.Return.ImmutableLocal
	if firstCarrier.Binding.Position != 3 || secondPayload.Binding.Position != 4 || secondCarrier.Binding.Position != 5 || thirdPayload.Binding.Position != 6 || thirdCarrier.Binding.Position != 7 || fourthPayload.Binding.Position != 8 || fourthPayload.Return.Kind != hir.ExprCall {
		t.Fatalf("contextual HIR chain = %#v", function.Body)
	}
	for _, call := range []*hir.Call{firstCarrier.Initializer.Call, secondCarrier.Initializer.Call, thirdCarrier.Initializer.Call, fourthPayload.Return.Call} {
		if call == nil || len(call.Arguments) != 2 || call.Arguments[1].Kind != hir.ExprReference || call.Arguments[1].Reference == nil || call.Arguments[1].Reference.Kind != hir.BindingParameter || call.Arguments[1].Reference.Position != 1 {
			t.Fatalf("contextual HIR helper call = %#v", call)
		}
	}

	coreFunction := coreFunctionNamed(t, program, "Compile")
	coreFirst := coreFunction.Body.ImmutableLocal
	coreFirstCarrier := coreFirst.Return.ImmutableLocal
	coreSecondPayload := coreFirstCarrier.Return.ImmutableLocal
	coreSecondCarrier := coreSecondPayload.Return.ImmutableLocal
	coreThirdPayload := coreSecondCarrier.Return.ImmutableLocal
	coreThirdCarrier := coreThirdPayload.Return.ImmutableLocal
	coreFourthPayload := coreThirdCarrier.Return.ImmutableLocal
	if program.LanguageContract != coreir.LanguageContractV620 || coreFirst.Position != 2 || coreFirstCarrier.Position != 3 || coreSecondPayload.Position != 4 || coreSecondCarrier.Position != 5 || coreThirdPayload.Position != 6 || coreThirdCarrier.Position != 7 || coreFourthPayload.Position != 8 {
		t.Fatalf("contextual Core chain = %#v", coreFunction.Body)
	}
	for _, call := range []*coreir.Call{coreFirstCarrier.Initializer.Call, coreSecondCarrier.Initializer.Call, coreThirdCarrier.Initializer.Call, coreFourthPayload.Return.Call} {
		if call == nil || len(call.Arguments) != 2 || call.Arguments[1] == nil || call.Arguments[1].Parameter == nil || *call.Arguments[1].Parameter != 1 {
			t.Fatalf("contextual Core helper call = %#v", call)
		}
	}

	entry := coreir.SemanticIdentity{PackageID: string(identity.PackageID), Path: string(identity.Path)}
	carrierType := coreFunction.Parameters[0].Type
	textType := coreFunction.Parameters[1].Type
	successCarrier := coreeval.Outcome{OK: true, Value: coreeval.Value{Type: carrierType.Result.Success, String: "class Root {}"}}
	success, err := coreeval.EvaluateProgram(program, entry, []coreeval.Value{{Type: carrierType, Result: &successCarrier}, {Type: textType, String: "debug"}})
	if err != nil || !success.OK || success.Value.String != "compiled:debug" {
		t.Fatalf("success = %#v, %v", success, err)
	}
	failureText := coreeval.Value{Type: carrierType.Result.Failure, String: "scan failed"}
	incomingCarrier := coreeval.Outcome{Value: coreeval.Value{Type: carrierType.Result.Success}, Failure: &failureText}
	incoming, err := coreeval.EvaluateProgram(program, entry, []coreeval.Value{{Type: carrierType, Result: &incomingCarrier}, {Type: textType, String: "debug"}})
	if err != nil || incoming.OK || incoming.Failure == nil || incoming.Failure.String != "scan failed" || incoming.Value.String != "" {
		t.Fatalf("incoming failure = %#v, %v", incoming, err)
	}
	malformedCarrier := coreeval.Outcome{Value: coreeval.Value{Type: carrierType.Result.Success, String: "not zero"}, Failure: &failureText}
	if _, err := coreeval.EvaluateProgram(program, entry, []coreeval.Value{{Type: carrierType, Result: &malformedCarrier}, {Type: textType, String: "debug"}}); err == nil {
		t.Fatal("evaluator accepted a failed incoming carrier with a non-canonical source payload")
	}
	invalidFailure := coreeval.Value{Type: carrierType.Result.Failure, String: string([]byte{0xff})}
	invalidCarrier := coreeval.Outcome{Value: coreeval.Value{Type: carrierType.Result.Success}, Failure: &invalidFailure}
	if _, err := coreeval.EvaluateProgram(program, entry, []coreeval.Value{{Type: carrierType, Result: &invalidCarrier}, {Type: textType, String: "debug"}}); err == nil {
		t.Fatal("evaluator accepted an incoming carrier with invalid error text")
	}
	if _, err := coreeval.EvaluateProgram(program, entry, []coreeval.Value{{Type: carrierType, Result: &successCarrier}, {Type: textType, String: string([]byte{0xff})}}); err == nil {
		t.Fatal("evaluator accepted invalid context text")
	}

	generated, err := gobackend.Generate(program)
	if err != nil {
		t.Fatal(err)
	}
	generatedAgain, err := gobackend.Generate(program)
	if err != nil || string(generatedAgain) != string(generated) {
		t.Fatalf("generated Go is not deterministic: %v", err)
	}
	for _, fragment := range []string{"PipeLangTokenize(p2, p1)", "PipeLangParse(pipelangCloneListTestPackageCompilerSelfhostingToken(p4), p1)", "PipeLangBind(pipelangCloneListTestPackageCompilerSelfhostingSyntaxnode(p6), p1)", "PipeLangEmit(pipelangCloneListTestPackageCompilerSelfhostingTypednode(p8), p1)"} {
		if !strings.Contains(string(generated), fragment) {
			t.Fatalf("generated Go lacks contextual propagation %q:\n%s", fragment, generated)
		}
	}
	compileAndRunGeneratedGoFiles(t, generated, []byte(fmt.Sprintf(`package %s

import "testing"

func TestGeneratedContextualCrossPayloadResultPropagation(t *testing.T) {
	success := PipeLangCompile(PipeLangResult[string, string]{OK: true, Value: "class Root {}"}, "release")
	if !success.OK || success.Value != "compiled:release" { t.Fatalf("success = %%#v", success) }
	incoming := PipeLangCompile(PipeLangResult[string, string]{Error: "scan failed"}, "release")
	if incoming.OK || incoming.Error != "scan failed" || incoming.Value != "" { t.Fatalf("incoming failure = %%#v", incoming) }
}

func TestGeneratedContextualCrossPayloadRejectsInvalidValues(t *testing.T) {
	for _, run := range []func(){
		func() { PipeLangCompile(PipeLangResult[string, string]{Value: "not zero", Error: "scan failed"}, "release") },
		func() { PipeLangCompile(PipeLangResult[string, string]{Error: string([]byte{0xff})}, "release") },
		func() { PipeLangCompile(PipeLangResult[string, string]{OK: true, Value: "class Root {}"}, string([]byte{0xff})) },
	} {
		func() {
			defer func() { if recover() == nil { t.Fatal("invalid contextual value did not panic") } }()
			run()
		}()
	}
}
`, gobackend.PackageName)))
}

func TestV620ContextualCrossPayloadResultPropagationAdmitsTwoStageLowerBound(t *testing.T) {
	source := `public Record Token { public string Text; } public Class Root {
		public Result<List<Token>, string> Tokenize(string raw, string context) { List<Token> tokens = empty_list<Token>(); return ok<List<Token>, string>(tokens); }
		public Result<string, string> Emit(List<Token> tokens, string context) { string output = context; return ok<string, string>(output); }
		public Result<string, string> Compile(Result<string, string> input, string context) {
			string raw = propagate(input);
			Result<List<Token>, string> tokenized = Tokenize(raw, context);
			List<Token> tokens = propagate(tokenized);
			return Emit(tokens, context);
		}
	}`
	input := semanticTestModuleSet("compiler.selfhosting", []ModuleInput{testModule("compiler.selfhosting", "contextual-two-stage.pipe", source)}, nil)
	input.LanguageContract = PipeLangLanguageContractV620
	if err := AnalyzeSemanticModuleSet(input).Error(); err != nil {
		t.Fatal(err)
	}
}

func TestV620ContextualCrossPayloadResultPropagationShortCircuitsEveryFailure(t *testing.T) {
	for _, test := range []struct {
		method string
		want   string
	}{
		{method: "FailTokenize", want: "tokenize failed:ctx"},
		{method: "FailParse", want: "parse failed:ctx"},
		{method: "FailBind", want: "bind failed:ctx"},
		{method: "FailEmit", want: "emit failed:ctx"},
	} {
		t.Run(test.method, func(t *testing.T) {
			_, identity, program := contextualCrossPayloadProgram(t, test.method)
			function := coreFunctionNamed(t, program, test.method)
			carrierType := function.Parameters[0].Type
			carrier := coreeval.Outcome{OK: true, Value: coreeval.Value{Type: carrierType.Result.Success, String: "source"}}
			entry := coreir.SemanticIdentity{PackageID: string(identity.PackageID), Path: string(identity.Path)}
			outcome, err := coreeval.EvaluateProgram(program, entry, []coreeval.Value{{Type: carrierType, Result: &carrier}, {Type: function.Parameters[1].Type, String: "ctx"}})
			if err != nil || outcome.OK || outcome.Failure == nil || outcome.Failure.String != test.want || outcome.Value.String != "" {
				t.Fatalf("outcome = %#v, %v", outcome, err)
			}
		})
	}
}

func TestV620ContextualCrossPayloadResultPropagationRejectsExcludedSource(t *testing.T) {
	valid := `public Record Token { public string Text; } public Record Node { public string Kind; } public Record Typed { public string Type; } public Class Root { public Result<List<Token>, string> First(string raw, string context) { List<Token> values = empty_list<Token>(); return ok<List<Token>, string>(values); } public Result<List<Node>, string> Second(List<Token> values, string context) { List<Node> nodes = empty_list<Node>(); return ok<List<Node>, string>(nodes); } public Result<List<Typed>, string> Third(List<Node> nodes, string context) { List<Typed> typed = empty_list<Typed>(); return ok<List<Typed>, string>(typed); } public Result<List<Typed>, string> Compile(Result<string, string> input, string context) { string raw = propagate(input); Result<List<Token>, string> firstCarrier = First(raw, context); List<Token> first = propagate(firstCarrier); Result<List<Node>, string> secondCarrier = Second(first, context); List<Node> second = propagate(secondCarrier); return Third(second, context); } }`
	tests := []struct {
		name     string
		contract LanguageContract
		source   string
	}{
		{name: "prior contract", contract: PipeLangLanguageContractV610, source: valid},
		{name: "third caller parameter", contract: PipeLangLanguageContractV620, source: strings.Replace(valid, "input, string context)", "input, string context, string extra)", 1)},
		{name: "non-string context", contract: PipeLangLanguageContractV620, source: strings.Replace(valid, "input, string context)", "input, bool context)", 1)},
		{name: "missing helper context", contract: PipeLangLanguageContractV620, source: strings.Replace(valid, "First(raw, context)", "First(raw)", 1)},
		{name: "reversed helper arguments", contract: PipeLangLanguageContractV620, source: strings.Replace(valid, "First(raw, context)", "First(context, raw)", 1)},
		{name: "computed helper context", contract: PipeLangLanguageContractV620, source: strings.Replace(valid, "First(raw, context)", "First(raw, trim(context))", 1)},
		{name: "wrong helper context", contract: PipeLangLanguageContractV620, source: strings.Replace(valid, "Second(first, context)", "Second(first, raw)", 1)},
		{name: "same adjacent payload", contract: PipeLangLanguageContractV620, source: strings.Replace(valid, "public Result<List<Token>, string> First(string raw, string context) { List<Token> values = empty_list<Token>(); return ok<List<Token>, string>(values); }", "public Result<string, string> First(string raw, string context) => ok<string, string>(raw);", 1)},
		{name: "computed carrier", contract: PipeLangLanguageContractV620, source: strings.Replace(valid, "Result<List<Token>, string> firstCarrier = First(raw, context); List<Token> first = propagate(firstCarrier);", "List<Token> first = propagate(First(raw, context));", 1)},
		{name: "ordinary local gap", contract: PipeLangLanguageContractV620, source: strings.Replace(valid, "List<Token> first = propagate(firstCarrier);", "List<Token> first = propagate(firstCarrier); string gap = context;", 1)},
		{name: "private helper", contract: PipeLangLanguageContractV620, source: strings.Replace(valid, "public Result<List<Node>, string> Second", "private Result<List<Node>, string> Second", 1)},
		{name: "one stage only", contract: PipeLangLanguageContractV620, source: `public Record Token { public string Text; } public Class Root { public Result<List<Token>, string> First(string raw, string context) => ok<List<Token>, string>(empty_list<Token>()); public Result<List<Token>, string> Compile(Result<string, string> input, string context) { string raw = propagate(input); return First(raw, context); } }`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			input := semanticTestModuleSet("compiler.selfhosting", []ModuleInput{testModule("compiler.selfhosting", test.name+".pipe", test.source)}, nil)
			input.LanguageContract = test.contract
			if err := AnalyzeSemanticModuleSet(input).Error(); err == nil {
				t.Fatal("excluded contextual cross-payload Result propagation source was accepted")
			}
		})
	}
}

func TestV620ContextualCrossPayloadResultPropagationRejectsMalformedCoreAndBackend(t *testing.T) {
	_, _, program := contextualCrossPayloadProgram(t, "Compile")
	for index := range program.Functions {
		if program.Functions[index].Name != "Compile" {
			continue
		}
		wrongContext := 2
		program.Functions[index].Body.ImmutableLocal.Return.ImmutableLocal.Initializer.Call.Arguments[1].Parameter = &wrongContext
	}
	if err := coreir.ValidateProgram(program); err == nil || !strings.Contains(err.Error(), "contextual cross-payload") {
		t.Fatalf("Core validator accepted a helper that bypasses the context parameter: %v", err)
	}
	if _, err := gobackend.Generate(program); err == nil {
		t.Fatal("Go backend accepted malformed contextual cross-payload propagation Core")
	}
}

func TestV620PreservesV610GeneralizedCrossPayloadResultPropagation(t *testing.T) {
	input := semanticTestModuleSet("compiler.selfhosting", []ModuleInput{testModule("compiler.selfhosting", "generalized-v062.pipe", generalizedCrossPayloadResultPropagationSource)}, nil)
	input.LanguageContract = PipeLangLanguageContractV620
	if err := AnalyzeSemanticModuleSet(input).Error(); err != nil {
		t.Fatal(err)
	}
}
