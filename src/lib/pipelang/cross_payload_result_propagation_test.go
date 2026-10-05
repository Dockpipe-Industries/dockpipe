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

const crossPayloadResultPropagationSource = `public Record Token { public string Text; }
public Record SyntaxNode { public string Kind; }
public Class CompilerPipeline {
	public Result<List<Token>, string> BuildTokens(string source) {
		List<Token> tokens = empty_list<Token>();
		return ok<List<Token>, string>(tokens);
	}
	public Result<string, string> SummarizeTokens(List<Token> tokens) {
		string summary = "tokens";
		return ok<string, string>(summary);
	}
	public Result<List<SyntaxNode>, string> BuildSyntax(List<Token> tokens) {
		List<SyntaxNode> nodes = empty_list<SyntaxNode>();
		return ok<List<SyntaxNode>, string>(nodes);
	}
	public Result<List<Token>, string> ScanToTokens(Result<string, string> scanned) {
		string source = propagate(scanned);
		return BuildTokens(source);
	}
	public Result<string, string> TokensToText(Result<List<Token>, string> tokenized) {
		List<Token> tokens = propagate(tokenized);
		return SummarizeTokens(tokens);
	}
	public Result<List<SyntaxNode>, string> TokensToSyntax(Result<List<Token>, string> tokenized) {
		List<Token> tokens = propagate(tokenized);
		return BuildSyntax(tokens);
	}
}`

func TestV590CrossPayloadResultPropagationPipeline(t *testing.T) {
	input := semanticTestModuleSet("compiler.selfhosting", []ModuleInput{testModule("compiler.selfhosting", "cross-payload-result.pipe", crossPayloadResultPropagationSource)}, nil)
	input.LanguageContract = PipeLangLanguageContractV590
	analysis := AnalyzeSemanticModuleSet(input)
	if err := analysis.Error(); err != nil {
		t.Fatal(err)
	}
	projection, err := BuildSemanticProjection(analysis)
	if err != nil {
		t.Fatal(err)
	}
	if projection.LanguageContract != PipeLangLanguageContractV590 || projection.Schema != PipeLangSemanticProjectionVersion || projection.CompilerContract != PipeLangCompilerContract {
		t.Fatalf("projection contract = %#v", projection)
	}

	identity := semanticMethodNamed(t, analysis, "ScanToTokens").Identity
	typed, err := LowerSemanticMethodToHIR(analysis, identity)
	if err != nil {
		t.Fatal(err)
	}
	function := hirFunctionNamed(t, typed, "ScanToTokens")
	local := function.Body.ImmutableLocal
	if typed.LanguageContract != coreir.LanguageContractV590 || len(function.Parameters) != 1 || local == nil || local.Binding.Position != 1 || local.Initializer.Kind != hir.ExprPropagate || local.Return == nil || local.Return.Kind != hir.ExprCall || local.Return.Call == nil || len(local.Return.Call.Arguments) != 1 || local.Return.Call.Arguments[0].Reference == nil || local.Return.Call.Arguments[0].Reference.Position != 1 {
		t.Fatalf("cross-payload propagation HIR = %#v", function.Body)
	}

	program, err := LowerHIRToCore(typed)
	if err != nil {
		t.Fatal(err)
	}
	coreFunction := coreFunctionNamed(t, program, "ScanToTokens")
	coreLocal := coreFunction.Body.ImmutableLocal
	if program.LanguageContract != coreir.LanguageContractV590 || coreLocal == nil || coreLocal.Position != 1 || coreLocal.Initializer == nil || coreLocal.Initializer.Propagate == nil || coreLocal.Initializer.Propagate.Value == nil || coreLocal.Initializer.Propagate.Value.Parameter == nil || *coreLocal.Initializer.Propagate.Value.Parameter != 0 || coreLocal.Return == nil || coreLocal.Return.Call == nil || coreLocal.Return.Call.Arguments[0].Parameter == nil || *coreLocal.Return.Call.Arguments[0].Parameter != 1 {
		t.Fatalf("cross-payload propagation Core = %#v", coreFunction.Body)
	}

	entry := coreir.SemanticIdentity{PackageID: string(identity.PackageID), Path: string(identity.Path)}
	carrierType := coreFunction.Parameters[0].Type
	successCarrier := coreeval.Outcome{OK: true, Value: coreeval.Value{Type: carrierType.Result.Success, String: "class Root {}"}}
	success, err := coreeval.EvaluateProgram(program, entry, []coreeval.Value{{Type: carrierType, Result: &successCarrier}})
	if err != nil || !success.OK || success.Value.Type.Kind != coreir.TypeList || success.Value.List == nil || len(success.Value.List) != 0 {
		t.Fatalf("success = %#v, %v", success, err)
	}
	failureValue := coreeval.Value{Type: carrierType.Result.Failure, String: "scan failed"}
	failureCarrier := coreeval.Outcome{Value: coreeval.Value{Type: carrierType.Result.Success}, Failure: &failureValue}
	failure, err := coreeval.EvaluateProgram(program, entry, []coreeval.Value{{Type: carrierType, Result: &failureCarrier}})
	if err != nil || failure.OK || failure.Failure == nil || failure.Failure.String != "scan failed" || !coreir.TypeEqual(failure.Value.Type, coreFunction.ReturnType.Result.Success) || failure.Value.List != nil {
		t.Fatalf("failure = %#v, %v", failure, err)
	}
	malformed := coreeval.Outcome{Value: coreeval.Value{Type: carrierType.Result.Success, String: "not zero"}, Failure: &failureValue}
	if _, err := coreeval.EvaluateProgram(program, entry, []coreeval.Value{{Type: carrierType, Result: &malformed}}); err == nil {
		t.Fatal("evaluator accepted a failed carrier with a non-canonical source payload")
	}
	invalidFailureValue := coreeval.Value{Type: carrierType.Result.Failure, String: string([]byte{0xff})}
	invalidFailure := coreeval.Outcome{Value: coreeval.Value{Type: carrierType.Result.Success}, Failure: &invalidFailureValue}
	if _, err := coreeval.EvaluateProgram(program, entry, []coreeval.Value{{Type: carrierType, Result: &invalidFailure}}); err == nil {
		t.Fatal("evaluator accepted a failed carrier with invalid error text")
	}

	generated, err := gobackend.Generate(program)
	if err != nil {
		t.Fatal(err)
	}
	generatedAgain, err := gobackend.Generate(program)
	if err != nil || string(generatedAgain) != string(generated) {
		t.Fatalf("generated Go is not deterministic: %v", err)
	}
	for _, fragment := range []string{"pipelangValidateTextResult(p0)", "return pipelangSnapshotResultErr", "p1 := p0.Value", "PipeLangBuildTokens(p1)"} {
		if !strings.Contains(string(generated), fragment) {
			t.Fatalf("generated Go lacks cross-payload propagation %q:\n%s", fragment, generated)
		}
	}
	compileAndRunGeneratedGoFiles(t, generated, []byte(fmt.Sprintf(`package %s

import "testing"

func TestGeneratedCrossPayloadResultPropagation(t *testing.T) {
	success := PipeLangScanToTokens(PipeLangResult[string, string]{OK: true, Value: "class Root {}"})
	if !success.OK || success.Value == nil || len(success.Value) != 0 { t.Fatalf("success = %%#v", success) }
	failure := PipeLangScanToTokens(PipeLangResult[string, string]{Error: "scan failed"})
	if failure.OK || failure.Error != "scan failed" || failure.Value != nil { t.Fatalf("failure = %%#v", failure) }
}

func TestGeneratedCrossPayloadRejectsMalformedCarrier(t *testing.T) {
	defer func() { if recover() == nil { t.Fatal("malformed carrier did not panic") } }()
	PipeLangScanToTokens(PipeLangResult[string, string]{Value: "not zero", Error: "scan failed"})
}

func TestGeneratedCrossPayloadRejectsInvalidErrorText(t *testing.T) {
	defer func() { if recover() == nil { t.Fatal("invalid error text did not panic") } }()
	PipeLangScanToTokens(PipeLangResult[string, string]{Error: string([]byte{0xff})})
}
`, gobackend.PackageName)))
}

func TestV590CrossPayloadResultPropagationAdmitsExactPayloadMatrix(t *testing.T) {
	input := semanticTestModuleSet("compiler.selfhosting", []ModuleInput{testModule("compiler.selfhosting", "cross-payload-matrix.pipe", crossPayloadResultPropagationSource)}, nil)
	input.LanguageContract = PipeLangLanguageContractV590
	analysis := AnalyzeSemanticModuleSet(input)
	if err := analysis.Error(); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"ScanToTokens", "TokensToText", "TokensToSyntax"} {
		identity := semanticMethodNamed(t, analysis, name).Identity
		typed, err := LowerSemanticMethodToHIR(analysis, identity)
		if err != nil {
			t.Fatalf("%s HIR: %v", name, err)
		}
		program, err := LowerHIRToCore(typed)
		if err != nil {
			t.Fatalf("%s Core: %v", name, err)
		}
		generated, err := gobackend.Generate(program)
		if err != nil {
			t.Fatalf("%s backend: %v", name, err)
		}
		compileAndRunGeneratedGoFiles(t, generated, []byte("package "+gobackend.PackageName+"\n"))
		function := coreFunctionNamed(t, program, name)
		carrierType := function.Parameters[0].Type
		sourceValue := coreeval.Value{Type: carrierType.Result.Success}
		if sourceValue.Type.Kind == coreir.TypePrimitive {
			sourceValue.String = "source"
		} else {
			sourceValue.List = []coreeval.Value{}
		}
		carrier := coreeval.Outcome{OK: true, Value: sourceValue}
		outcome, err := coreeval.EvaluateProgram(program, coreir.SemanticIdentity{PackageID: string(identity.PackageID), Path: string(identity.Path)}, []coreeval.Value{{Type: carrierType, Result: &carrier}})
		if err != nil || !outcome.OK {
			t.Fatalf("%s success = %#v, %v", name, outcome, err)
		}
		if name == "TokensToText" && outcome.Value.String != "tokens" {
			t.Fatalf("%s text = %#v", name, outcome)
		}
		if name != "TokensToText" && (outcome.Value.List == nil || len(outcome.Value.List) != 0) {
			t.Fatalf("%s list = %#v", name, outcome)
		}
		failureValue := coreeval.Value{Type: carrierType.Result.Failure, String: "blocked"}
		failureCarrier := coreeval.Outcome{Value: coreeval.Value{Type: carrierType.Result.Success}, Failure: &failureValue}
		failed, err := coreeval.EvaluateProgram(program, coreir.SemanticIdentity{PackageID: string(identity.PackageID), Path: string(identity.Path)}, []coreeval.Value{{Type: carrierType, Result: &failureCarrier}})
		if err != nil || failed.OK || failed.Failure == nil || failed.Failure.String != "blocked" || !coreir.TypeEqual(failed.Value.Type, function.ReturnType.Result.Success) {
			t.Fatalf("%s failure = %#v, %v", name, failed, err)
		}
	}
}

func TestV590CrossPayloadResultPropagationRejectsExcludedSource(t *testing.T) {
	valid := `public Record Token { public string Text; } public Class Root { public Result<List<Token>, string> Build(string raw) { List<Token> tokens = empty_list<Token>(); return ok<List<Token>, string>(tokens); } public Result<List<Token>, string> Parse(Result<string, string> scanned) { string raw = propagate(scanned); return Build(raw); } }`
	tests := []struct {
		name     string
		contract LanguageContract
		source   string
	}{
		{name: "prior contract", contract: PipeLangLanguageContractV580, source: valid},
		{name: "extra parameter", contract: PipeLangLanguageContractV590, source: strings.Replace(valid, "scanned)", "scanned, string fallback)", 1)},
		{name: "missing local", contract: PipeLangLanguageContractV590, source: strings.Replace(valid, "string raw = propagate(scanned); return Build(raw);", "return Build(propagate(scanned));", 1)},
		{name: "computed carrier", contract: PipeLangLanguageContractV590, source: strings.Replace(valid, "propagate(scanned)", "propagate(ok<string, string>(\"raw\"))", 1)},
		{name: "helper propagation", contract: PipeLangLanguageContractV590, source: strings.Replace(valid, "return Build(raw);", "return propagate(Build(raw));", 1)},
		{name: "computed helper argument", contract: PipeLangLanguageContractV590, source: strings.Replace(valid, "Build(raw)", "Build(trim(raw))", 1)},
		{name: "private helper", contract: PipeLangLanguageContractV590, source: strings.Replace(valid, "public Result<List<Token>, string> Build", "private Result<List<Token>, string> Build", 1)},
		{name: "wrong helper payload", contract: PipeLangLanguageContractV590, source: strings.Replace(valid, "Build(raw);", "Build(\"raw\");", 1)},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			input := semanticTestModuleSet("compiler.selfhosting", []ModuleInput{testModule("compiler.selfhosting", test.name+".pipe", test.source)}, nil)
			input.LanguageContract = test.contract
			if err := AnalyzeSemanticModuleSet(input).Error(); err == nil {
				t.Fatal("excluded cross-payload Result propagation source was accepted")
			}
		})
	}
}

func TestV590CrossPayloadResultPropagationRejectsMalformedCoreAndBackend(t *testing.T) {
	input := semanticTestModuleSet("compiler.selfhosting", []ModuleInput{testModule("compiler.selfhosting", "cross-payload-core.pipe", crossPayloadResultPropagationSource)}, nil)
	input.LanguageContract = PipeLangLanguageContractV590
	analysis := AnalyzeSemanticModuleSet(input)
	if err := analysis.Error(); err != nil {
		t.Fatal(err)
	}
	typed, err := LowerSemanticMethodToHIR(analysis, semanticMethodNamed(t, analysis, "ScanToTokens").Identity)
	if err != nil {
		t.Fatal(err)
	}
	program, err := LowerHIRToCore(typed)
	if err != nil {
		t.Fatal(err)
	}
	for index := range program.Functions {
		if program.Functions[index].Name != "ScanToTokens" {
			continue
		}
		badPosition := 0
		program.Functions[index].Body.ImmutableLocal.Return.Call.Arguments[0].Parameter = &badPosition
	}
	if err := coreir.ValidateProgram(program); err == nil {
		t.Fatal("Core validator accepted a helper argument that bypasses the propagated local")
	}
	if _, err := gobackend.Generate(program); err == nil {
		t.Fatal("Go backend accepted malformed cross-payload propagation Core")
	}
}
