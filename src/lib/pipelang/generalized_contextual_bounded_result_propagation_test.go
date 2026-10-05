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

const generalizedContextualBoundedResultPropagationSource = `public Record Token { public string Text; }
public Record SyntaxNode { public string Kind; }
public Class GeneralizedContextualPipeline {
	public Result<string, string> Normalize(string source, string context) => context == "normalize" ? err<string, string>("normalize failed") : ok<string, string>(source);
	public Result<List<Token>, string> Tokenize(string source, string context) => context == "tokenize" ? err<List<Token>, string>("tokenize failed") : ok<List<Token>, string>(empty_list<Token>());
	public Result<List<Token>, string> PreserveTokens(List<Token> tokens, string context) => context == "preserve tokens" ? err<List<Token>, string>("preserve tokens failed") : ok<List<Token>, string>(tokens);
	public Result<List<SyntaxNode>, string> Parse(List<Token> tokens, string context) => context == "parse" ? err<List<SyntaxNode>, string>("parse failed") : ok<List<SyntaxNode>, string>(empty_list<SyntaxNode>());
	public Result<List<SyntaxNode>, string> PreserveSyntax(List<SyntaxNode> syntax, string context) => context == "preserve syntax" ? err<List<SyntaxNode>, string>("preserve syntax failed") : ok<List<SyntaxNode>, string>(syntax);
	public Result<string, string> RepeatText(string source, string context) => context == "repeat" ? err<string, string>("repeat failed") : ok<string, string>(source);
	public Result<string, string> FinalizeText(string source, string context) => context == "finalize" ? err<string, string>("finalize failed") : ok<string, string>(source);
	public Result<List<SyntaxNode>, string> Compile(Result<string, string> scanned, string context) {
		string source = propagate(scanned);
		Result<string, string> normalizedCarrier = Normalize(source, context);
		string normalized = propagate(normalizedCarrier);
		Result<List<Token>, string> tokenCarrier = Tokenize(normalized, context);
		List<Token> tokens = propagate(tokenCarrier);
		Result<List<Token>, string> preservedCarrier = PreserveTokens(tokens, context);
		List<Token> preserved = propagate(preservedCarrier);
		Result<List<SyntaxNode>, string> syntaxCarrier = Parse(preserved, context);
		List<SyntaxNode> syntax = propagate(syntaxCarrier);
		return PreserveSyntax(syntax, context);
	}
	public Result<string, string> Repeat(Result<string, string> scanned, string context) {
		string source = propagate(scanned);
		Result<string, string> normalizedCarrier = Normalize(source, context);
		string normalized = propagate(normalizedCarrier);
		Result<string, string> repeatedCarrier = RepeatText(normalized, context);
		string repeated = propagate(repeatedCarrier);
		return FinalizeText(repeated, context);
	}
}`

func generalizedContextualBoundedProgram(t *testing.T, method string) (*Analysis, SemanticIdentity, coreir.Program) {
	t.Helper()
	input := semanticTestModuleSet("compiler.selfhosting", []ModuleInput{testModule("compiler.selfhosting", "generalized-contextual-bounded-result.pipe", generalizedContextualBoundedResultPropagationSource)}, nil)
	input.LanguageContract = PipeLangLanguageContractV660
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

func TestV660GeneralizedContextualBoundedResultPropagationPipeline(t *testing.T) {
	analysis, identity, program := generalizedContextualBoundedProgram(t, "Compile")
	projection, err := BuildSemanticProjection(analysis)
	if err != nil {
		t.Fatal(err)
	}
	if projection.LanguageContract != PipeLangLanguageContractV660 || projection.Schema != PipeLangSemanticProjectionVersion || projection.CompilerContract != PipeLangCompilerContract {
		t.Fatalf("projection contract = %#v", projection)
	}

	typed, err := LowerSemanticMethodToHIR(analysis, identity)
	if err != nil {
		t.Fatal(err)
	}
	function := hirFunctionNamed(t, typed, "Compile")
	local := function.Body.ImmutableLocal
	if typed.LanguageContract != coreir.LanguageContractV660 || local == nil || local.Binding.Position != 2 || local.Initializer.Kind != hir.ExprPropagate {
		t.Fatalf("first v0.66.0 HIR local = %#v", function.Body)
	}
	positions := []int{2}
	calls := []*hir.Call{}
	for local.Return != nil && local.Return.Kind == hir.ExprImmutableLocal {
		carrier := local.Return.ImmutableLocal
		if carrier == nil || carrier.Initializer.Kind != hir.ExprCall || carrier.Initializer.Call == nil || carrier.Return == nil || carrier.Return.Kind != hir.ExprImmutableLocal {
			t.Fatalf("v0.66.0 HIR chain = %#v", function.Body)
		}
		calls = append(calls, carrier.Initializer.Call)
		positions = append(positions, carrier.Binding.Position)
		local = carrier.Return.ImmutableLocal
		positions = append(positions, local.Binding.Position)
	}
	if local.Return == nil || local.Return.Kind != hir.ExprCall || local.Return.Call == nil {
		t.Fatalf("v0.66.0 HIR terminal = %#v", function.Body)
	}
	calls = append(calls, local.Return.Call)
	if fmt.Sprint(positions) != "[2 3 4 5 6 7 8 9 10]" || len(calls) != 5 {
		t.Fatalf("v0.66.0 HIR positions/calls = %v/%d", positions, len(calls))
	}
	for _, call := range calls {
		if len(call.Arguments) != 2 || call.Arguments[1].Reference == nil || call.Arguments[1].Reference.Kind != hir.BindingParameter || call.Arguments[1].Reference.Position != 1 {
			t.Fatalf("v0.66.0 HIR helper context = %#v", call)
		}
	}

	coreFunction := coreFunctionNamed(t, program, "Compile")
	if program.LanguageContract != coreir.LanguageContractV660 {
		t.Fatalf("Core contract = %q", program.LanguageContract)
	}
	coreLocal := coreFunction.Body.ImmutableLocal
	coreCalls := []*coreir.Call{}
	for coreLocal.Return != nil && coreLocal.Return.Kind == coreir.ExprImmutableLocal {
		carrier := coreLocal.Return.ImmutableLocal
		coreCalls = append(coreCalls, carrier.Initializer.Call)
		coreLocal = carrier.Return.ImmutableLocal
	}
	coreCalls = append(coreCalls, coreLocal.Return.Call)
	for _, call := range coreCalls {
		if call == nil || len(call.Arguments) != 2 || call.Arguments[1] == nil || call.Arguments[1].Parameter == nil || *call.Arguments[1].Parameter != 1 {
			t.Fatalf("v0.66.0 Core helper context = %#v", call)
		}
	}
	if err := coreir.ValidateProgram(program); err != nil {
		t.Fatal(err)
	}

	entry := coreir.SemanticIdentity{PackageID: string(identity.PackageID), Path: string(identity.Path)}
	carrierType := coreFunction.Parameters[0].Type
	contextType := coreFunction.Parameters[1].Type
	successCarrier := coreeval.Outcome{OK: true, Value: coreeval.Value{Type: carrierType.Result.Success, String: "source"}}
	evaluate := func(context string) coreeval.Outcome {
		t.Helper()
		outcome, err := coreeval.EvaluateProgram(program, entry, []coreeval.Value{{Type: carrierType, Result: &successCarrier}, {Type: contextType, String: context}})
		if err != nil {
			t.Fatal(err)
		}
		return outcome
	}
	if success := evaluate("ok"); !success.OK || success.Value.List == nil || len(success.Value.List) != 0 {
		t.Fatalf("success = %#v", success)
	}
	for context, want := range map[string]string{
		"normalize":       "normalize failed",
		"tokenize":        "tokenize failed",
		"preserve tokens": "preserve tokens failed",
		"parse":           "parse failed",
		"preserve syntax": "preserve syntax failed",
	} {
		if outcome := evaluate(context); outcome.OK || outcome.Failure == nil || outcome.Failure.String != want || outcome.Value.List != nil {
			t.Fatalf("%s failure = %#v", context, outcome)
		}
	}
	failureText := coreeval.Value{Type: carrierType.Result.Failure, String: "scan failed"}
	incomingCarrier := coreeval.Outcome{Value: coreeval.Value{Type: carrierType.Result.Success}, Failure: &failureText}
	incoming, err := coreeval.EvaluateProgram(program, entry, []coreeval.Value{{Type: carrierType, Result: &incomingCarrier}, {Type: contextType, String: "ok"}})
	if err != nil || incoming.OK || incoming.Failure == nil || incoming.Failure.String != "scan failed" || incoming.Value.List != nil {
		t.Fatalf("incoming failure = %#v, %v", incoming, err)
	}

	generated, err := gobackend.Generate(program)
	if err != nil {
		t.Fatal(err)
	}
	generatedAgain, err := gobackend.Generate(program)
	if err != nil || string(generatedAgain) != string(generated) {
		t.Fatalf("generated Go is not deterministic: %v", err)
	}
	for _, helper := range []string{"PipeLangNormalize(", "PipeLangTokenize(", "PipeLangPreserveTokens(", "PipeLangParse(", "PipeLangPreserveSyntax("} {
		if !strings.Contains(string(generated), helper) {
			t.Fatalf("generated Go lacks v0.66.0 helper %q:\n%s", helper, generated)
		}
	}
	compileAndRunGeneratedGoFiles(t, generated, []byte(fmt.Sprintf(`package %s

import "testing"

func TestGeneratedGeneralizedContextualBoundedResultPropagation(t *testing.T) {
	success := PipeLangCompile(PipeLangResult[string, string]{OK: true, Value: "source"}, "ok")
	if !success.OK || success.Value == nil || len(success.Value) != 0 { t.Fatalf("success = %%#v", success) }
	for context, want := range map[string]string{"normalize": "normalize failed", "tokenize": "tokenize failed", "preserve tokens": "preserve tokens failed", "parse": "parse failed", "preserve syntax": "preserve syntax failed"} {
		outcome := PipeLangCompile(PipeLangResult[string, string]{OK: true, Value: "source"}, context)
		if outcome.OK || outcome.Error != want || outcome.Value != nil { t.Fatalf("%%s = %%#v", context, outcome) }
	}
	incoming := PipeLangCompile(PipeLangResult[string, string]{Error: "scan failed"}, "ok")
	if incoming.OK || incoming.Error != "scan failed" || incoming.Value != nil { t.Fatalf("incoming = %%#v", incoming) }
}
`, gobackend.PackageName)))
}

func TestV660GeneralizedContextualBoundedResultPropagationAdmitsAllEqualTransitions(t *testing.T) {
	_, identity, program := generalizedContextualBoundedProgram(t, "Repeat")
	if err := coreir.ValidateProgram(program); err != nil {
		t.Fatal(err)
	}
	function := coreFunctionNamed(t, program, "Repeat")
	carrierType := function.Parameters[0].Type
	carrier := coreeval.Outcome{OK: true, Value: coreeval.Value{Type: carrierType.Result.Success, String: "source"}}
	outcome, err := coreeval.EvaluateProgram(program, coreir.SemanticIdentity{PackageID: string(identity.PackageID), Path: string(identity.Path)}, []coreeval.Value{{Type: carrierType, Result: &carrier}, {Type: function.Parameters[1].Type, String: "ok"}})
	if err != nil || !outcome.OK || outcome.Value.String != "source" {
		t.Fatalf("all-equal outcome = %#v, %v", outcome, err)
	}
}

func TestV660GeneralizedContextualBoundedResultPropagationGateAndExclusions(t *testing.T) {
	prior := semanticTestModuleSet("compiler.selfhosting", []ModuleInput{testModule("compiler.selfhosting", "generalized-contextual-bounded-prior.pipe", generalizedContextualBoundedResultPropagationSource)}, nil)
	prior.LanguageContract = PipeLangLanguageContractV650
	if err := AnalyzeSemanticModuleSet(prior).Error(); err == nil {
		t.Fatal("v0.65.0 accepted a contextual chain with three or more stages and equal adjacent payloads")
	}
	valid := `public Class Root { public Result<string, string> Step(string value, string context) => ok<string, string>(value); public Result<string, string> Finish(string value, string context) => ok<string, string>(value); public Result<string, string> Run(Result<string, string> input, string context) { string first = propagate(input); Result<string, string> nextCarrier = Step(first, context); string second = propagate(nextCarrier); return Finish(second, context); } }`
	for name, source := range map[string]string{
		"third caller parameter": strings.Replace(valid, "input, string context)", "input, string context, string extra)", 1),
		"computed context":       strings.Replace(valid, "Step(first, context)", "Step(first, trim(context))", 1),
		"ordinary local gap":     strings.Replace(valid, "string second = propagate(nextCarrier);", "string second = propagate(nextCarrier); string gap = context;", 1),
		"computed carrier":       strings.Replace(valid, "Result<string, string> nextCarrier = Step(first, context); string second = propagate(nextCarrier);", "string second = propagate(Step(first, context));", 1),
	} {
		t.Run(name, func(t *testing.T) {
			input := semanticTestModuleSet("compiler.selfhosting", []ModuleInput{testModule("compiler.selfhosting", name+".pipe", source)}, nil)
			input.LanguageContract = PipeLangLanguageContractV660
			if err := AnalyzeSemanticModuleSet(input).Error(); err == nil {
				t.Fatal("excluded v0.66.0 source was accepted")
			}
		})
	}
}

func TestV660GeneralizedContextualBoundedResultPropagationRejectsMalformedCoreAndBackend(t *testing.T) {
	_, _, program := generalizedContextualBoundedProgram(t, "Compile")
	function := coreFunctionNamed(t, program, "Compile")
	wrongContext := 0
	function.Body.ImmutableLocal.Return.ImmutableLocal.Return.ImmutableLocal.Return.ImmutableLocal.Return.ImmutableLocal.Return.ImmutableLocal.Initializer.Call.Arguments[1].Parameter = &wrongContext
	for index := range program.Functions {
		if program.Functions[index].Name == "Compile" {
			program.Functions[index] = function
		}
	}
	if err := coreir.ValidateProgram(program); err == nil || !strings.Contains(err.Error(), "v0.66.0 generalized contextual bounded Result propagation") {
		t.Fatalf("Core validator accepted malformed v0.66.0 context: %v", err)
	}
	if _, err := gobackend.Generate(program); err == nil {
		t.Fatal("Go backend accepted malformed v0.66.0 Core")
	}
}

func TestV660PreservesPriorContextualResultPropagationContracts(t *testing.T) {
	for name, source := range map[string]string{
		"v0.62 distinct-payload chain": contextualCrossPayloadResultPropagationSource,
		"v0.65 exact two-stage chain":  twoStageContextualBoundedResultPropagationSource,
	} {
		t.Run(name, func(t *testing.T) {
			input := semanticTestModuleSet("compiler.selfhosting", []ModuleInput{testModule("compiler.selfhosting", name+".pipe", source)}, nil)
			input.LanguageContract = PipeLangLanguageContractV660
			if err := AnalyzeSemanticModuleSet(input).Error(); err != nil {
				t.Fatal(err)
			}
		})
	}
}
