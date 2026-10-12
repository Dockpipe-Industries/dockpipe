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

const generalizedOneStageSharedContextResultPropagationSource = `public Record Token { public string Text; }
public Class OneStageSharedContextPipeline {
	public Result<List<Token>, string> Tokenize(string source, string phase, string scope) => phase == "fail" ? err<List<Token>, string>("tokenize failed:" + source + ":" + scope) : ok<List<Token>, string>(empty_list<Token>());
	public Result<string, string> Normalize(string source, string phase, string scope) => scope == "fail" ? err<string, string>("normalize failed:" + phase) : ok<string, string>(source + ":" + phase + ":" + scope);
	public Result<List<Token>, string> Compile(Result<string, string> scanned, string phase, string scope) {
		string source = propagate(scanned);
		return Tokenize(source, phase, scope);
	}
	public Result<string, string> Preserve(Result<string, string> scanned, string phase, string scope) {
		string source = propagate(scanned);
		return Normalize(source, phase, scope);
	}
}`

func generalizedOneStageSharedContextProgram(t *testing.T, method string) (*Analysis, SemanticIdentity, coreir.Program) {
	t.Helper()
	input := semanticTestModuleSet("compiler.selfhosting", []ModuleInput{testModule("compiler.selfhosting", "generalized-one-stage-shared-context-result.pipe", generalizedOneStageSharedContextResultPropagationSource)}, nil)
	input.LanguageContract = PipeLangLanguageContractV680
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

func TestV680GeneralizedOneStageSharedContextResultPropagationPipeline(t *testing.T) {
	analysis, identity, program := generalizedOneStageSharedContextProgram(t, "Compile")
	projection, err := BuildSemanticProjection(analysis)
	if err != nil {
		t.Fatal(err)
	}
	if projection.LanguageContract != PipeLangLanguageContractV680 || projection.Schema != PipeLangSemanticProjectionVersion || projection.CompilerContract != PipeLangCompilerContract {
		t.Fatalf("projection contract = %#v", projection)
	}

	typed, err := LowerSemanticMethodToHIR(analysis, identity)
	if err != nil {
		t.Fatal(err)
	}
	function := hirFunctionNamed(t, typed, "Compile")
	first := function.Body.ImmutableLocal
	if typed.LanguageContract != coreir.LanguageContractV680 || first == nil || first.Binding.Position != 3 || first.Initializer.Kind != hir.ExprPropagate || first.Return == nil || first.Return.Kind != hir.ExprCall || first.Return.Call == nil || len(first.Return.Call.Arguments) != 3 {
		t.Fatalf("v0.68.0 HIR = %#v", function.Body)
	}
	for position, want := range []int{3, 1, 2} {
		argument := first.Return.Call.Arguments[position]
		if argument.Reference == nil || argument.Reference.Position != want {
			t.Fatalf("v0.68.0 HIR helper argument %d = %#v", position, argument)
		}
	}

	coreFunction := coreFunctionNamed(t, program, "Compile")
	coreFirst := coreFunction.Body.ImmutableLocal
	if program.LanguageContract != coreir.LanguageContractV680 || coreFirst == nil || coreFirst.Position != 3 || coreFirst.Return == nil || coreFirst.Return.Kind != coreir.ExprCall || coreFirst.Return.Call == nil || len(coreFirst.Return.Call.Arguments) != 3 {
		t.Fatalf("v0.68.0 Core = %#v", coreFunction.Body)
	}
	for position, want := range []int{3, 1, 2} {
		argument := coreFirst.Return.Call.Arguments[position]
		if argument.Parameter == nil || *argument.Parameter != want {
			t.Fatalf("v0.68.0 Core helper argument %d = %#v", position, argument)
		}
	}
	if err := coreir.ValidateProgram(program); err != nil {
		t.Fatal(err)
	}

	entry := coreir.SemanticIdentity{PackageID: string(identity.PackageID), Path: string(identity.Path)}
	carrierType := coreFunction.Parameters[0].Type
	stringType := coreFunction.Parameters[1].Type
	successCarrier := coreeval.Outcome{OK: true, Value: coreeval.Value{Type: carrierType.Result.Success, String: "class Root {}"}}
	success, err := coreeval.EvaluateProgram(program, entry, []coreeval.Value{{Type: carrierType, Result: &successCarrier}, {Type: stringType, String: "release"}, {Type: stringType, String: "compiler"}})
	if err != nil || !success.OK || len(success.Value.List) != 0 {
		t.Fatalf("success = %#v, %v", success, err)
	}
	failureText := coreeval.Value{Type: carrierType.Result.Failure, String: "scan failed"}
	incomingCarrier := coreeval.Outcome{Value: coreeval.Value{Type: carrierType.Result.Success}, Failure: &failureText}
	incoming, err := coreeval.EvaluateProgram(program, entry, []coreeval.Value{{Type: carrierType, Result: &incomingCarrier}, {Type: stringType, String: "release"}, {Type: stringType, String: "compiler"}})
	if err != nil || incoming.OK || incoming.Failure == nil || incoming.Failure.String != "scan failed" || incoming.Value.List != nil {
		t.Fatalf("incoming failure = %#v, %v", incoming, err)
	}
	helperFailure, err := coreeval.EvaluateProgram(program, entry, []coreeval.Value{{Type: carrierType, Result: &successCarrier}, {Type: stringType, String: "fail"}, {Type: stringType, String: "compiler"}})
	if err != nil || helperFailure.OK || helperFailure.Failure == nil || helperFailure.Failure.String != "tokenize failed:class Root {}:compiler" || helperFailure.Value.List != nil {
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
	for _, fragment := range []string{"p3 := p0.Value", "PipeLangTokenize(p3, p1, p2)"} {
		if !strings.Contains(string(generated), fragment) {
			t.Fatalf("generated Go lacks v0.68.0 one-stage shared-context propagation %q:\n%s", fragment, generated)
		}
	}
	compileAndRunGeneratedGoFiles(t, generated, []byte(fmt.Sprintf(`package %s

import "testing"

func TestGeneratedGeneralizedOneStageSharedContext(t *testing.T) {
	success := PipeLangCompile(PipeLangResult[string, string]{OK: true, Value: "source"}, "release", "compiler")
	if !success.OK || success.Value == nil || len(success.Value) != 0 { t.Fatalf("success = %%#v", success) }
	incoming := PipeLangCompile(PipeLangResult[string, string]{Error: "scan failed"}, "release", "compiler")
	if incoming.OK || incoming.Error != "scan failed" || incoming.Value != nil { t.Fatalf("incoming = %%#v", incoming) }
	helper := PipeLangCompile(PipeLangResult[string, string]{OK: true, Value: "source"}, "fail", "compiler")
	if helper.OK || helper.Error != "tokenize failed:source:compiler" || helper.Value != nil { t.Fatalf("helper = %%#v", helper) }
}

func TestGeneratedGeneralizedOneStageSharedContextRejectsInvalidValues(t *testing.T) {
	for _, run := range []func(){
		func() { PipeLangCompile(PipeLangResult[string, string]{Value: "not zero", Error: "scan failed"}, "release", "compiler") },
		func() { PipeLangCompile(PipeLangResult[string, string]{Error: string([]byte{0xff})}, "release", "compiler") },
		func() { PipeLangCompile(PipeLangResult[string, string]{OK: true, Value: "source"}, string([]byte{0xff}), "compiler") },
		func() { PipeLangCompile(PipeLangResult[string, string]{OK: true, Value: "source"}, "release", string([]byte{0xff})) },
	} {
		func() {
			defer func() { if recover() == nil { t.Fatal("invalid v0.68.0 value did not panic") } }()
			run()
		}()
	}
}
`, gobackend.PackageName)))
}

func TestV680GeneralizedOneStageSharedContextMatrixAndGate(t *testing.T) {
	input := semanticTestModuleSet("compiler.selfhosting", []ModuleInput{testModule("compiler.selfhosting", "generalized-one-stage-shared-context-gate.pipe", generalizedOneStageSharedContextResultPropagationSource)}, nil)
	input.LanguageContract = PipeLangLanguageContractV670
	if err := AnalyzeSemanticModuleSet(input).Error(); err == nil {
		t.Fatal("v0.67.0 accepted the v0.68.0 one-stage multi-context form")
	}
	input.LanguageContract = PipeLangLanguageContractV680
	if err := AnalyzeSemanticModuleSet(input).Error(); err != nil {
		t.Fatal(err)
	}
	_, _, samePayload := generalizedOneStageSharedContextProgram(t, "Preserve")
	function := coreFunctionNamed(t, samePayload, "Preserve")
	if !coreir.TypeEqual(function.Parameters[0].Type.Result.Success, function.ReturnType.Result.Success) {
		t.Fatal("v0.68.0 same-payload form was not lowered")
	}

	arbitrary := `public Class Root { public Result<string, string> Next(string raw, string a, string b, string c, string d) => a == "fail" || b == "fail" || c == "fail" || d == "fail" ? err<string, string>("failed") : ok<string, string>(raw); public Result<string, string> Compile(Result<string, string> input, string a, string b, string c, string d) { string raw = propagate(input); return Next(raw, a, b, c, d); } }`
	arbitraryInput := semanticTestModuleSet("compiler.selfhosting", []ModuleInput{testModule("compiler.selfhosting", "arbitrary-one-stage-contexts.pipe", arbitrary)}, nil)
	arbitraryInput.LanguageContract = PipeLangLanguageContractV680
	if err := AnalyzeSemanticModuleSet(arbitraryInput).Error(); err != nil {
		t.Fatal(err)
	}

	priorSingle := semanticTestModuleSet("compiler.selfhosting", []ModuleInput{testModule("compiler.selfhosting", "single-context-v068.pipe", singleStageContextualSamePayloadSource)}, nil)
	priorSingle.LanguageContract = PipeLangLanguageContractV680
	if err := AnalyzeSemanticModuleSet(priorSingle).Error(); err != nil {
		t.Fatalf("v0.68.0 rejected inherited one-context form: %v", err)
	}
	priorChain := semanticTestModuleSet("compiler.selfhosting", []ModuleInput{testModule("compiler.selfhosting", "shared-context-chain-v068.pipe", generalizedSharedContextResultPropagationSource)}, nil)
	priorChain.LanguageContract = PipeLangLanguageContractV680
	if err := AnalyzeSemanticModuleSet(priorChain).Error(); err != nil {
		t.Fatalf("v0.68.0 rejected inherited multi-stage form: %v", err)
	}
}

func TestV680GeneralizedOneStageSharedContextRejectsExcludedSource(t *testing.T) {
	valid := `public Class Root { public Result<string, string> Next(string raw, string phase, string scope) => ok<string, string>(raw); public Result<string, string> Compile(Result<string, string> input, string phase, string scope) { string raw = propagate(input); return Next(raw, phase, scope); } }`
	tests := []struct {
		name   string
		source string
	}{
		{name: "non-string context", source: strings.Replace(valid, "string scope) {", "bool scope) {", 1)},
		{name: "missing helper context", source: strings.Replace(valid, "Next(raw, phase, scope)", "Next(raw, phase)", 1)},
		{name: "reordered helper contexts", source: strings.Replace(valid, "Next(raw, phase, scope)", "Next(raw, scope, phase)", 1)},
		{name: "repeated helper context", source: strings.Replace(valid, "Next(raw, phase, scope)", "Next(raw, phase, phase)", 1)},
		{name: "computed helper context", source: strings.Replace(valid, "Next(raw, phase, scope)", "Next(raw, phase, trim(scope))", 1)},
		{name: "private helper", source: strings.Replace(valid, "public Result<string, string> Next", "private Result<string, string> Next", 1)},
		{name: "extra local", source: strings.Replace(valid, "return Next(raw, phase, scope);", "string gap = scope; return Next(raw, phase, scope);", 1)},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			input := semanticTestModuleSet("compiler.selfhosting", []ModuleInput{testModule("compiler.selfhosting", test.name+".pipe", test.source)}, nil)
			input.LanguageContract = PipeLangLanguageContractV680
			if err := AnalyzeSemanticModuleSet(input).Error(); err == nil {
				t.Fatal("excluded v0.68.0 source was accepted")
			}
		})
	}
}

func TestV680GeneralizedOneStageSharedContextRejectsMalformedCoreAndBackend(t *testing.T) {
	_, _, program := generalizedOneStageSharedContextProgram(t, "Compile")
	for index := range program.Functions {
		if program.Functions[index].Name == "Compile" {
			repeatedContext := 1
			program.Functions[index].Body.ImmutableLocal.Return.Call.Arguments[2].Parameter = &repeatedContext
		}
	}
	if err := coreir.ValidateProgram(program); err == nil || !strings.Contains(err.Error(), "v0.68.0 generalized one-stage shared-context Result propagation") {
		t.Fatalf("Core validator accepted malformed v0.68.0 context: %v", err)
	}
	if _, err := gobackend.Generate(program); err == nil {
		t.Fatal("Go backend accepted malformed v0.68.0 Core")
	}
}
