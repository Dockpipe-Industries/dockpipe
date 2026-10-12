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

const generalizedSharedContextResultPropagationSource = `public Record Token { public string Text; }
public Class SharedContextPipeline {
	public Result<string, string> Normalize(string source, string phase, string scope) => phase == "normalize" || scope == "normalize" ? err<string, string>("normalize failed") : ok<string, string>(source);
	public Result<List<Token>, string> Tokenize(string source, string phase, string scope) => phase == "tokenize" || scope == "tokenize" ? err<List<Token>, string>("tokenize failed") : ok<List<Token>, string>(empty_list<Token>());
	public Result<List<Token>, string> Preserve(List<Token> tokens, string phase, string scope) => phase == "preserve" || scope == "preserve" ? err<List<Token>, string>("preserve failed") : ok<List<Token>, string>(tokens);
	public Result<List<Token>, string> Compile(Result<string, string> scanned, string phase, string scope) {
		string source = propagate(scanned);
		Result<string, string> normalizedCarrier = Normalize(source, phase, scope);
		string normalized = propagate(normalizedCarrier);
		Result<List<Token>, string> tokenCarrier = Tokenize(normalized, phase, scope);
		List<Token> tokens = propagate(tokenCarrier);
		return Preserve(tokens, phase, scope);
	}
}`

func generalizedSharedContextProgram(t *testing.T) (*Analysis, SemanticIdentity, coreir.Program) {
	t.Helper()
	input := semanticTestModuleSet("compiler.selfhosting", []ModuleInput{testModule("compiler.selfhosting", "generalized-shared-context-result.pipe", generalizedSharedContextResultPropagationSource)}, nil)
	input.LanguageContract = PipeLangLanguageContractV670
	analysis := AnalyzeSemanticModuleSet(input)
	if err := analysis.Error(); err != nil {
		t.Fatal(err)
	}
	identity := semanticMethodNamed(t, analysis, "Compile").Identity
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

func TestV670GeneralizedSharedContextResultPropagationPipeline(t *testing.T) {
	analysis, identity, program := generalizedSharedContextProgram(t)
	projection, err := BuildSemanticProjection(analysis)
	if err != nil {
		t.Fatal(err)
	}
	if projection.LanguageContract != PipeLangLanguageContractV670 || projection.Schema != PipeLangSemanticProjectionVersion || projection.CompilerContract != PipeLangCompilerContract {
		t.Fatalf("projection contract = %#v", projection)
	}

	typed, err := LowerSemanticMethodToHIR(analysis, identity)
	if err != nil {
		t.Fatal(err)
	}
	function := hirFunctionNamed(t, typed, "Compile")
	local := function.Body.ImmutableLocal
	if typed.LanguageContract != coreir.LanguageContractV670 || local == nil || local.Binding.Position != 3 || local.Initializer.Kind != hir.ExprPropagate {
		t.Fatalf("first v0.67.0 HIR local = %#v", function.Body)
	}
	for local.Return != nil && local.Return.Kind == hir.ExprImmutableLocal {
		carrier := local.Return.ImmutableLocal
		if carrier == nil || carrier.Initializer.Kind != hir.ExprCall || carrier.Initializer.Call == nil || carrier.Return == nil || carrier.Return.Kind != hir.ExprImmutableLocal {
			t.Fatalf("v0.67.0 HIR chain = %#v", function.Body)
		}
		assertV670HIRContexts(t, carrier.Initializer.Call)
		local = carrier.Return.ImmutableLocal
	}
	if local.Return == nil || local.Return.Kind != hir.ExprCall || local.Return.Call == nil {
		t.Fatalf("v0.67.0 HIR terminal = %#v", function.Body)
	}
	assertV670HIRContexts(t, local.Return.Call)

	coreFunction := coreFunctionNamed(t, program, "Compile")
	if program.LanguageContract != coreir.LanguageContractV670 {
		t.Fatalf("Core contract = %q", program.LanguageContract)
	}
	coreLocal := coreFunction.Body.ImmutableLocal
	for coreLocal.Return != nil && coreLocal.Return.Kind == coreir.ExprImmutableLocal {
		carrier := coreLocal.Return.ImmutableLocal
		assertV670CoreContexts(t, carrier.Initializer.Call)
		coreLocal = carrier.Return.ImmutableLocal
	}
	assertV670CoreContexts(t, coreLocal.Return.Call)
	if err := coreir.ValidateProgram(program); err != nil {
		t.Fatal(err)
	}

	entry := coreir.SemanticIdentity{PackageID: string(identity.PackageID), Path: string(identity.Path)}
	carrierType := coreFunction.Parameters[0].Type
	successCarrier := coreeval.Outcome{OK: true, Value: coreeval.Value{Type: carrierType.Result.Success, String: "source"}}
	evaluate := func(phase, scope string) coreeval.Outcome {
		t.Helper()
		outcome, err := coreeval.EvaluateProgram(program, entry, []coreeval.Value{
			{Type: carrierType, Result: &successCarrier},
			{Type: coreFunction.Parameters[1].Type, String: phase},
			{Type: coreFunction.Parameters[2].Type, String: scope},
		})
		if err != nil {
			t.Fatal(err)
		}
		return outcome
	}
	if success := evaluate("ok", "ok"); !success.OK || success.Value.List == nil || len(success.Value.List) != 0 {
		t.Fatalf("success = %#v", success)
	}
	for _, context := range []string{"normalize", "tokenize", "preserve"} {
		for _, pair := range [][2]string{{context, "ok"}, {"ok", context}} {
			outcome := evaluate(pair[0], pair[1])
			if outcome.OK || outcome.Failure == nil || outcome.Failure.String != context+" failed" || outcome.Value.List != nil {
				t.Fatalf("%s contexts = %#v", context, outcome)
			}
		}
	}
	failureText := coreeval.Value{Type: carrierType.Result.Failure, String: "scan failed"}
	incomingCarrier := coreeval.Outcome{Value: coreeval.Value{Type: carrierType.Result.Success}, Failure: &failureText}
	incoming, err := coreeval.EvaluateProgram(program, entry, []coreeval.Value{
		{Type: carrierType, Result: &incomingCarrier},
		{Type: coreFunction.Parameters[1].Type, String: "normalize"},
		{Type: coreFunction.Parameters[2].Type, String: "tokenize"},
	})
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
	compileAndRunGeneratedGoFiles(t, generated, []byte(fmt.Sprintf(`package %s

import "testing"

func TestGeneratedGeneralizedSharedContextResultPropagation(t *testing.T) {
	success := PipeLangCompile(PipeLangResult[string, string]{OK: true, Value: "source"}, "ok", "ok")
	if !success.OK || success.Value == nil || len(success.Value) != 0 { t.Fatalf("success = %%#v", success) }
	for _, pair := range [][3]string{{"normalize", "ok", "normalize failed"}, {"ok", "tokenize", "tokenize failed"}, {"preserve", "ok", "preserve failed"}} {
		outcome := PipeLangCompile(PipeLangResult[string, string]{OK: true, Value: "source"}, pair[0], pair[1])
		if outcome.OK || outcome.Error != pair[2] || outcome.Value != nil { t.Fatalf("%%v = %%#v", pair, outcome) }
	}
}
`, gobackend.PackageName)))
}

func assertV670HIRContexts(t *testing.T, call *hir.Call) {
	t.Helper()
	if call == nil || len(call.Arguments) != 3 {
		t.Fatalf("v0.67.0 HIR helper arguments = %#v", call)
	}
	for position := 1; position < 3; position++ {
		argument := call.Arguments[position]
		if argument.Reference == nil || argument.Reference.Kind != hir.BindingParameter || argument.Reference.Position != position {
			t.Fatalf("v0.67.0 HIR helper context %d = %#v", position, call)
		}
	}
}

func assertV670CoreContexts(t *testing.T, call *coreir.Call) {
	t.Helper()
	if call == nil || len(call.Arguments) != 3 {
		t.Fatalf("v0.67.0 Core helper arguments = %#v", call)
	}
	for position := 1; position < 3; position++ {
		argument := call.Arguments[position]
		if argument == nil || argument.Parameter == nil || *argument.Parameter != position {
			t.Fatalf("v0.67.0 Core helper context %d = %#v", position, call)
		}
	}
}

func TestV670GeneralizedSharedContextResultPropagationGateAndExclusions(t *testing.T) {
	prior := semanticTestModuleSet("compiler.selfhosting", []ModuleInput{testModule("compiler.selfhosting", "generalized-shared-context-prior.pipe", generalizedSharedContextResultPropagationSource)}, nil)
	prior.LanguageContract = PipeLangLanguageContractV660
	if err := AnalyzeSemanticModuleSet(prior).Error(); err == nil {
		t.Fatal("v0.66.0 accepted more than one direct context parameter")
	}

	valid := `public Class Root { public Result<string, string> Step(string value, string first, string second) => ok<string, string>(value); public Result<string, string> Finish(string value, string first, string second) => ok<string, string>(value); public Result<string, string> Run(Result<string, string> input, string first, string second) { string value = propagate(input); Result<string, string> nextCarrier = Step(value, first, second); string next = propagate(nextCarrier); return Finish(next, first, second); } }`
	for name, source := range map[string]string{
		"missing context":    strings.Replace(valid, "Step(value, first, second)", "Step(value, first)", 1),
		"reordered contexts": strings.Replace(valid, "Step(value, first, second)", "Step(value, second, first)", 1),
		"repeated context":   strings.Replace(valid, "Step(value, first, second)", "Step(value, first, first)", 1),
		"computed context":   strings.Replace(valid, "Step(value, first, second)", "Step(value, trim(first), second)", 1),
		"non-string context": strings.ReplaceAll(valid, "string second", "int second"),
		"ordinary local gap": strings.Replace(valid, "string next = propagate(nextCarrier);", "string next = propagate(nextCarrier); string gap = first;", 1),
		"one-stage chain":    strings.Replace(valid, "Result<string, string> nextCarrier = Step(value, first, second); string next = propagate(nextCarrier); return Finish(next, first, second);", "return Step(value, first, second);", 1),
	} {
		t.Run(name, func(t *testing.T) {
			input := semanticTestModuleSet("compiler.selfhosting", []ModuleInput{testModule("compiler.selfhosting", name+".pipe", source)}, nil)
			input.LanguageContract = PipeLangLanguageContractV670
			if err := AnalyzeSemanticModuleSet(input).Error(); err == nil {
				t.Fatal("excluded v0.67.0 source was accepted")
			}
		})
	}
}

func TestV670GeneralizedSharedContextResultPropagationAdmitsArbitraryContextCount(t *testing.T) {
	source := `public Class Root {
	public Result<string, string> Step(string value, string first, string second, string third) => first == "" || second == "" || third == "" ? err<string, string>("empty context") : ok<string, string>(value);
	public Result<string, string> Finish(string value, string first, string second, string third) => first == "" || second == "" || third == "" ? err<string, string>("empty context") : ok<string, string>(value);
	public Result<string, string> Run(Result<string, string> input, string first, string second, string third) {
		string value = propagate(input);
		Result<string, string> nextCarrier = Step(value, first, second, third);
		string next = propagate(nextCarrier);
		return Finish(next, first, second, third);
	}
}`
	input := semanticTestModuleSet("compiler.selfhosting", []ModuleInput{testModule("compiler.selfhosting", "arbitrary-shared-contexts.pipe", source)}, nil)
	input.LanguageContract = PipeLangLanguageContractV670
	analysis := AnalyzeSemanticModuleSet(input)
	if err := analysis.Error(); err != nil {
		t.Fatal(err)
	}
	typed, err := LowerSemanticMethodToHIR(analysis, semanticMethodNamed(t, analysis, "Run").Identity)
	if err != nil {
		t.Fatal(err)
	}
	program, err := LowerHIRToCore(typed)
	if err != nil {
		t.Fatal(err)
	}
	if err := coreir.ValidateProgram(program); err != nil {
		t.Fatal(err)
	}
	function := coreFunctionNamed(t, program, "Run")
	call := function.Body.ImmutableLocal.Return.ImmutableLocal.Initializer.Call
	if call == nil || len(call.Arguments) != 4 {
		t.Fatalf("three-context helper arguments = %#v", call)
	}
	for position := 1; position < 4; position++ {
		if call.Arguments[position].Parameter == nil || *call.Arguments[position].Parameter != position {
			t.Fatalf("three-context helper argument %d = %#v", position, call.Arguments[position])
		}
	}
}

func TestV670GeneralizedSharedContextResultPropagationRejectsMalformedCoreAndBackend(t *testing.T) {
	_, _, program := generalizedSharedContextProgram(t)
	function := coreFunctionNamed(t, program, "Compile")
	wrongContext := 2
	function.Body.ImmutableLocal.Return.ImmutableLocal.Initializer.Call.Arguments[1].Parameter = &wrongContext
	for index := range program.Functions {
		if program.Functions[index].Name == "Compile" {
			program.Functions[index] = function
		}
	}
	if err := coreir.ValidateProgram(program); err == nil || !strings.Contains(err.Error(), "v0.67.0 generalized shared-context Result propagation") {
		t.Fatalf("Core validator accepted malformed v0.67.0 context: %v", err)
	}
	if _, err := gobackend.Generate(program); err == nil {
		t.Fatal("Go backend accepted malformed v0.67.0 Core")
	}
}

func TestV670PreservesV660GeneralizedContextualResultPropagation(t *testing.T) {
	input := semanticTestModuleSet("compiler.selfhosting", []ModuleInput{testModule("compiler.selfhosting", "v660-generalized-contextual.pipe", generalizedContextualBoundedResultPropagationSource)}, nil)
	input.LanguageContract = PipeLangLanguageContractV670
	if err := AnalyzeSemanticModuleSet(input).Error(); err != nil {
		t.Fatal(err)
	}
}
