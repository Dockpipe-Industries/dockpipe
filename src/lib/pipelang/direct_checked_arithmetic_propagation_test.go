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

const directCheckedArithmeticPropagationSource = `public Class Root {
	public Result<int, ArithmeticError> Continue(Result<int, ArithmeticError> carrier) {
		int value = propagate(carrier);
		return value + 0;
	}
	public Result<float, ArithmeticError> ContinueFloat(Result<float, ArithmeticError> carrier) {
		float value = propagate(carrier);
		return value / 1.0;
	}
}`

func TestV550DirectCheckedArithmeticPropagationPipeline(t *testing.T) {
	input := semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", "direct-checked-arithmetic-propagation.pipe", directCheckedArithmeticPropagationSource)}, nil)
	input.LanguageContract = PipeLangLanguageContractV550
	analysis := AnalyzeSemanticModuleSet(input)
	if err := analysis.Error(); err != nil {
		t.Fatal(err)
	}
	projection, err := BuildSemanticProjection(analysis)
	if err != nil {
		t.Fatal(err)
	}
	if projection.LanguageContract != PipeLangLanguageContractV550 || projection.Schema != PipeLangSemanticProjectionVersion || projection.CompilerContract != PipeLangCompilerContract {
		t.Fatalf("projection contract = %#v", projection)
	}

	identity := semanticMethodNamed(t, analysis, "Continue").Identity
	typed, err := LowerSemanticMethodToHIR(analysis, identity)
	if err != nil {
		t.Fatal(err)
	}
	function := hirFunctionNamed(t, typed, "Continue")
	local := function.Body.ImmutableLocal
	if typed.LanguageContract != coreir.LanguageContractV550 || local == nil || local.Initializer.Kind != hir.ExprPropagate || local.Initializer.Propagate == nil || local.Initializer.Propagate.Value == nil || local.Initializer.Propagate.Value.Kind != hir.ExprReference || local.Initializer.Propagate.Value.Reference == nil || local.Initializer.Propagate.Value.Reference.Kind != hir.BindingParameter || local.Initializer.Propagate.Value.Reference.Position != 0 {
		t.Fatalf("Continue HIR = %#v", function.Body)
	}
	if local.Return == nil || local.Return.Kind != hir.ExprBinary || local.Return.Binary == nil || local.Return.Binary.Operator != hir.OperatorAdd {
		t.Fatalf("continuation HIR = %#v", function.Body)
	}

	program, err := LowerHIRToCore(typed)
	if err != nil {
		t.Fatal(err)
	}
	coreFunction := coreFunctionNamed(t, program, "Continue")
	coreLocal := coreFunction.Body.ImmutableLocal
	if program.LanguageContract != coreir.LanguageContractV550 || coreLocal == nil || coreLocal.Initializer == nil || coreLocal.Initializer.Propagate == nil || coreLocal.Initializer.Propagate.Value == nil || coreLocal.Initializer.Propagate.Value.Parameter == nil || *coreLocal.Initializer.Propagate.Value.Parameter != 0 || coreLocal.Initializer.Propagate.Carrier.Result == nil || coreLocal.Initializer.Propagate.Carrier.Result.Failure.Kind != coreir.TypeArithmeticError {
		t.Fatalf("Continue Core = %#v", coreFunction.Body)
	}

	entry := coreir.SemanticIdentity{PackageID: string(identity.PackageID), Path: string(identity.Path)}
	resultType := coreFunction.Parameters[0].Type
	success := coreeval.Outcome{OK: true, Value: coreeval.Value{Type: resultType.Result.Success, Int: 42}}
	failure := coreeval.Outcome{Value: coreeval.Value{Type: resultType.Result.Success}, Error: coreir.ArithmeticOverflow}
	for _, test := range []struct {
		name    string
		carrier coreeval.Outcome
		ok      bool
		value   int64
		failure coreir.ArithmeticError
	}{
		{name: "success", carrier: success, ok: true, value: 42},
		{name: "propagated overflow", carrier: failure, failure: coreir.ArithmeticOverflow},
	} {
		argument := coreeval.Value{Type: resultType, Result: &test.carrier}
		outcome, evalErr := coreeval.EvaluateProgram(program, entry, []coreeval.Value{argument})
		if evalErr != nil || outcome.OK != test.ok || outcome.Error != test.failure || (test.ok && outcome.Value.Int != test.value) {
			t.Fatalf("%s = %#v, %v", test.name, outcome, evalErr)
		}
	}
	malformed := coreeval.Outcome{OK: true, Value: coreeval.Value{Type: resultType.Result.Success}, Error: coreir.ArithmeticOverflow}
	if _, evalErr := coreeval.EvaluateProgram(program, entry, []coreeval.Value{{Type: resultType, Result: &malformed}}); evalErr == nil {
		t.Fatal("evaluator accepted malformed direct arithmetic carrier")
	}

	generated, err := gobackend.Generate(program)
	if err != nil {
		t.Fatal(err)
	}
	generatedAgain, err := gobackend.Generate(program)
	if err != nil || string(generatedAgain) != string(generated) {
		t.Fatalf("generated Go is not deterministic: %v", err)
	}
	for _, fragment := range []string{"pipelangValidateArithmeticResult(p0)", "if !p0.OK {\n\t\treturn p0", "p1 := p0.Value"} {
		if !strings.Contains(string(generated), fragment) {
			t.Fatalf("generated Go lacks direct checked propagation %q:\n%s", fragment, generated)
		}
	}
	compileAndRunGeneratedGoFiles(t, generated, []byte(fmt.Sprintf(`package %s

import "testing"

func TestGeneratedDirectCheckedPropagation(t *testing.T) {
	success := PipeLangContinue(PipeLangArithmeticResult[int64]{OK: true, Value: 42})
	if !success.OK || success.Value != 42 || success.Error != "" { t.Fatalf("success = %%#v", success) }
	failure := PipeLangContinue(PipeLangArithmeticResult[int64]{Error: PipeLangArithmeticOverflow})
	if failure.OK || failure.Value != 0 || failure.Error != PipeLangArithmeticOverflow { t.Fatalf("failure = %%#v", failure) }
	defer func() { if recover() == nil { t.Fatal("malformed arithmetic carrier did not panic") } }()
	PipeLangContinue(PipeLangArithmeticResult[int64]{OK: true, Error: PipeLangArithmeticOverflow})
}
`, gobackend.PackageName)))

	testV550DirectFloatCheckedPropagation(t, analysis)
}

func testV550DirectFloatCheckedPropagation(t *testing.T, analysis *Analysis) {
	t.Helper()
	identity := semanticMethodNamed(t, analysis, "ContinueFloat").Identity
	typed, err := LowerSemanticMethodToHIR(analysis, identity)
	if err != nil {
		t.Fatal(err)
	}
	program, err := LowerHIRToCore(typed)
	if err != nil {
		t.Fatal(err)
	}
	function := coreFunctionNamed(t, program, "ContinueFloat")
	resultType := function.Parameters[0].Type
	entry := coreir.SemanticIdentity{PackageID: string(identity.PackageID), Path: string(identity.Path)}
	for _, test := range []struct {
		carrier coreeval.Outcome
		ok      bool
		value   float64
		failure coreir.ArithmeticError
	}{
		{carrier: coreeval.Outcome{OK: true, Value: coreeval.Value{Type: resultType.Result.Success, Float: 2.5}}, ok: true, value: 2.5},
		{carrier: coreeval.Outcome{Value: coreeval.Value{Type: resultType.Result.Success}, Error: coreir.ArithmeticDivisionByZero}, failure: coreir.ArithmeticDivisionByZero},
	} {
		outcome, evalErr := coreeval.EvaluateProgram(program, entry, []coreeval.Value{{Type: resultType, Result: &test.carrier}})
		if evalErr != nil || outcome.OK != test.ok || outcome.Error != test.failure || (test.ok && outcome.Value.Float != test.value) {
			t.Fatalf("float outcome = %#v, %v", outcome, evalErr)
		}
	}
	generated, err := gobackend.Generate(program)
	if err != nil {
		t.Fatal(err)
	}
	compileAndRunGeneratedGoFiles(t, generated, []byte(fmt.Sprintf(`package %s

import "testing"

func TestGeneratedDirectFloatCheckedPropagation(t *testing.T) {
	success := PipeLangContinueFloat(PipeLangArithmeticResult[float64]{OK: true, Value: 2.5})
	if !success.OK || success.Value != 2.5 || success.Error != "" { t.Fatalf("success = %%#v", success) }
	failure := PipeLangContinueFloat(PipeLangArithmeticResult[float64]{Error: PipeLangArithmeticDivisionByZero})
	if failure.OK || failure.Value != 0 || failure.Error != PipeLangArithmeticDivisionByZero { t.Fatalf("failure = %%#v", failure) }
}
`, gobackend.PackageName)))
}

func TestV550DirectCheckedArithmeticPropagationRejectsExcludedSource(t *testing.T) {
	tests := []struct {
		name     string
		contract LanguageContract
		source   string
		message  string
	}{
		{name: "prior contract", contract: PipeLangLanguageContractV540, source: directCheckedArithmeticPropagationSource, message: "admitted Optional or bounded Result"},
		{name: "extra parameter", contract: PipeLangLanguageContractV550, source: `public Class Root { public Result<int, ArithmeticError> Continue(Result<int, ArithmeticError> carrier, int extra) { int value = propagate(carrier); return value + 0; } }`, message: "sole direct carrier parameter"},
		{name: "not first local", contract: PipeLangLanguageContractV550, source: `public Class Root { public Result<int, ArithmeticError> Continue(Result<int, ArithmeticError> carrier) { int prefix = 0; int value = propagate(carrier); return value + prefix; } }`, message: "first immutable-local initializer"},
		{name: "computed operand", contract: PipeLangLanguageContractV550, source: `public Class Root { public Result<int, ArithmeticError> Identity(Result<int, ArithmeticError> carrier) => carrier; public Result<int, ArithmeticError> Continue(Result<int, ArithmeticError> carrier) { int value = propagate(Identity(carrier)); return value + 0; } }`, message: "admits inherited bounded matches"},
		{name: "mismatched return carrier", contract: PipeLangLanguageContractV550, source: `public Class Root { public Result<float, ArithmeticError> Continue(Result<int, ArithmeticError> carrier) { int value = propagate(carrier); return value / 1.0; } }`, message: "return type must be identical"},
		{name: "mismatched payload", contract: PipeLangLanguageContractV550, source: `public Class Root { public Result<int, ArithmeticError> Continue(Result<int, ArithmeticError> carrier) { float value = propagate(carrier); return value / 1.0; } }`, message: "payload"},
		{name: "additional propagation", contract: PipeLangLanguageContractV550, source: `public Class Root { public Result<int, ArithmeticError> Continue(Result<int, ArithmeticError> carrier) { int value = propagate(carrier); return propagate(carrier) + value; } }`, message: "exactly one propagation"},
		{name: "unchecked continuation", contract: PipeLangLanguageContractV550, source: `public Class Root { public Result<int, ArithmeticError> Continue(Result<int, ArithmeticError> carrier) { int value = propagate(carrier); return carrier; } }`, message: "checked arithmetic continuation"},
		{name: "wrong float continuation", contract: PipeLangLanguageContractV550, source: `public Class Root { public Result<float, ArithmeticError> Continue(Result<float, ArithmeticError> carrier) { float value = propagate(carrier); return value + 0.0; } }`, message: "checked arithmetic continuation"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			input := semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", test.name+".pipe", test.source)}, nil)
			input.LanguageContract = test.contract
			analysis := AnalyzeSemanticModuleSet(input)
			if len(analysis.Diagnostics) == 0 || !strings.Contains(analysis.Diagnostics[0].Message, test.message) {
				t.Fatalf("diagnostics = %#v", analysis.Diagnostics)
			}
		})
	}
}

func TestV550DirectCheckedArithmeticPropagationCoreRejectsMalformedComposition(t *testing.T) {
	input := semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", "direct-checked-arithmetic-propagation-core.pipe", directCheckedArithmeticPropagationSource)}, nil)
	input.LanguageContract = PipeLangLanguageContractV550
	analysis := AnalyzeSemanticModuleSet(input)
	if err := analysis.Error(); err != nil {
		t.Fatal(err)
	}
	typed, err := LowerSemanticMethodToHIR(analysis, semanticMethodNamed(t, analysis, "Continue").Identity)
	if err != nil {
		t.Fatal(err)
	}
	program, err := LowerHIRToCore(typed)
	if err != nil {
		t.Fatal(err)
	}

	prior := program
	prior.LanguageContract = coreir.LanguageContractV540
	if err := coreir.ValidateProgram(prior); err == nil || !strings.Contains(err.Error(), "admitted Optional or bounded Result") {
		t.Fatalf("prior contract error = %v", err)
	}
	if _, err := gobackend.Generate(prior); err == nil {
		t.Fatal("Go backend accepted v0.55 direct arithmetic propagation under v0.54")
	}

	badOperand := program
	badOperand.Functions = append([]coreir.Function(nil), program.Functions...)
	continued := coreFunctionNamed(t, badOperand, "Continue")
	local := *continued.Body.ImmutableLocal
	initializer := *local.Initializer
	propagation := *initializer.Propagate
	reference := *propagation.Value
	position := local.Position
	reference.Parameter = &position
	propagation.Value = &reference
	initializer.Propagate = &propagation
	local.Initializer = &initializer
	continued.Body.ImmutableLocal = &local
	for index := range badOperand.Functions {
		if badOperand.Functions[index].Name == "Continue" {
			badOperand.Functions[index] = continued
		}
	}
	if err := coreir.ValidateProgram(badOperand); err == nil || !strings.Contains(err.Error(), "sole direct carrier parameter") {
		t.Fatalf("bad operand Core error = %v", err)
	}
	if _, err := gobackend.Generate(badOperand); err == nil {
		t.Fatal("Go backend accepted malformed direct propagation Core")
	}

	badContinuation := program
	badContinuation.Functions = append([]coreir.Function(nil), program.Functions...)
	continued = coreFunctionNamed(t, badContinuation, "Continue")
	local = *continued.Body.ImmutableLocal
	carrierPosition := 0
	local.Return = &coreir.Expr{Kind: coreir.ExprReference, Type: continued.ReturnType, Parameter: &carrierPosition}
	continued.Body.ImmutableLocal = &local
	for index := range badContinuation.Functions {
		if badContinuation.Functions[index].Name == "Continue" {
			badContinuation.Functions[index] = continued
		}
	}
	if err := coreir.ValidateProgram(badContinuation); err == nil || !strings.Contains(err.Error(), "checked-arithmetic propagation") {
		t.Fatalf("bad continuation Core error = %v", err)
	}
	if _, err := gobackend.Generate(badContinuation); err == nil {
		t.Fatal("Go backend accepted direct propagation without a checked continuation")
	}
}

func TestV550PreservesV540HelperPropagationAndEarlierForms(t *testing.T) {
	for _, test := range []struct {
		name   string
		source string
		method string
	}{
		{name: "v0.54 helper propagation", source: checkedArithmeticHelperPropagationSource, method: "Resolve"},
		{name: "v0.53 bounded propagation", source: multiParameterHelperPropagationSource, method: "Normalize"},
		{name: "v0.52 cumulative matching", source: cumulativeFanInChainSource, method: "Combine"},
	} {
		t.Run(test.name, func(t *testing.T) {
			input := semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", test.name+".pipe", test.source)}, nil)
			input.LanguageContract = PipeLangLanguageContractV550
			analysis := AnalyzeSemanticModuleSet(input)
			if err := analysis.Error(); err != nil {
				t.Fatal(err)
			}
			typed, err := LowerSemanticMethodToHIR(analysis, semanticMethodNamed(t, analysis, test.method).Identity)
			if err != nil {
				t.Fatal(err)
			}
			program, err := LowerHIRToCore(typed)
			if err != nil {
				t.Fatal(err)
			}
			if program.LanguageContract != coreir.LanguageContractV550 {
				t.Fatalf("contract = %s", program.LanguageContract)
			}
		})
	}
}

func TestV550CompilerCursorConsumesCheckedResultParameter(t *testing.T) {
	const source = `public Class CompilerCursor {
		public Result<int, ArithmeticError> Continue(Result<int, ArithmeticError> checkedCursor) {
			int cursor = propagate(checkedCursor);
			return cursor + 0;
		}
	}`
	input := semanticTestModuleSet("compiler.cursor", []ModuleInput{testModule("compiler.cursor", "compiler-cursor-result.pipe", source)}, nil)
	input.LanguageContract = PipeLangLanguageContractV550
	analysis := AnalyzeSemanticModuleSet(input)
	if err := analysis.Error(); err != nil {
		t.Fatal(err)
	}
	identity := semanticMethodNamed(t, analysis, "Continue").Identity
	typed, err := LowerSemanticMethodToHIR(analysis, identity)
	if err != nil {
		t.Fatal(err)
	}
	program, err := LowerHIRToCore(typed)
	if err != nil {
		t.Fatal(err)
	}
	function := coreFunctionNamed(t, program, "Continue")
	resultType := function.Parameters[0].Type
	entry := coreir.SemanticIdentity{PackageID: string(identity.PackageID), Path: string(identity.Path)}
	for _, carrier := range []coreeval.Outcome{
		{OK: true, Value: coreeval.Value{Type: resultType.Result.Success, Int: 72}},
		{Value: coreeval.Value{Type: resultType.Result.Success}, Error: coreir.ArithmeticOverflow},
	} {
		outcome, evalErr := coreeval.EvaluateProgram(program, entry, []coreeval.Value{{Type: resultType, Result: &carrier}})
		if evalErr != nil || outcome.OK != carrier.OK || outcome.Error != carrier.Error || (carrier.OK && outcome.Value.Int != 72) {
			t.Fatalf("cursor outcome = %#v, %v", outcome, evalErr)
		}
	}
}
