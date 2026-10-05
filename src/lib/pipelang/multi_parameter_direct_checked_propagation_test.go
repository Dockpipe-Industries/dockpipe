package pipelang

import (
	"fmt"
	"math"
	"strings"
	"testing"

	"dockpipe/src/lib/pipelang/coreeval"
	"dockpipe/src/lib/pipelang/coreir"
	"dockpipe/src/lib/pipelang/gobackend"
	"dockpipe/src/lib/pipelang/hir"
)

const multiParameterDirectCheckedPropagationSource = `public Class Root {
	public Result<int, ArithmeticError> Continue(Result<int, ArithmeticError> carrier, int operand) {
		int value = propagate(carrier);
		return value + operand;
	}
	public Result<float, ArithmeticError> ContinueFloat(Result<float, ArithmeticError> carrier, float operand) {
		float value = propagate(carrier);
		return value / operand;
	}
}`

func TestV560MultiParameterDirectCheckedPropagationPipeline(t *testing.T) {
	input := semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", "multi-parameter-direct-checked-propagation.pipe", multiParameterDirectCheckedPropagationSource)}, nil)
	input.LanguageContract = PipeLangLanguageContractV560
	analysis := AnalyzeSemanticModuleSet(input)
	if err := analysis.Error(); err != nil {
		t.Fatal(err)
	}
	projection, err := BuildSemanticProjection(analysis)
	if err != nil {
		t.Fatal(err)
	}
	if projection.LanguageContract != PipeLangLanguageContractV560 || projection.Schema != PipeLangSemanticProjectionVersion || projection.CompilerContract != PipeLangCompilerContract {
		t.Fatalf("projection contract = %#v", projection)
	}

	identity := semanticMethodNamed(t, analysis, "Continue").Identity
	typed, err := LowerSemanticMethodToHIR(analysis, identity)
	if err != nil {
		t.Fatal(err)
	}
	function := hirFunctionNamed(t, typed, "Continue")
	local := function.Body.ImmutableLocal
	if typed.LanguageContract != coreir.LanguageContractV560 || len(function.Parameters) != 2 || local == nil || local.Binding.Position != 2 || local.Initializer.Kind != hir.ExprPropagate || local.Initializer.Propagate == nil || local.Initializer.Propagate.Value == nil || local.Initializer.Propagate.Value.Reference == nil || local.Initializer.Propagate.Value.Reference.Kind != hir.BindingParameter || local.Initializer.Propagate.Value.Reference.Position != 0 {
		t.Fatalf("Continue HIR = %#v", function)
	}
	if local.Return == nil || local.Return.Kind != hir.ExprBinary || local.Return.Binary == nil || local.Return.Binary.Operator != hir.OperatorAdd || local.Return.Binary.Left == nil || local.Return.Binary.Left.Reference == nil || local.Return.Binary.Left.Reference.Position != 2 || local.Return.Binary.Right == nil || local.Return.Binary.Right.Reference == nil || local.Return.Binary.Right.Reference.Position != 1 {
		t.Fatalf("continuation HIR = %#v", local.Return)
	}

	program, err := LowerHIRToCore(typed)
	if err != nil {
		t.Fatal(err)
	}
	coreFunction := coreFunctionNamed(t, program, "Continue")
	coreLocal := coreFunction.Body.ImmutableLocal
	if program.LanguageContract != coreir.LanguageContractV560 || coreLocal == nil || coreLocal.Position != 2 || coreLocal.Initializer == nil || coreLocal.Initializer.Propagate == nil || coreLocal.Initializer.Propagate.Value == nil || coreLocal.Initializer.Propagate.Value.Parameter == nil || *coreLocal.Initializer.Propagate.Value.Parameter != 0 {
		t.Fatalf("Continue Core = %#v", coreFunction.Body)
	}
	if coreLocal.Return == nil || coreLocal.Return.Binary == nil || coreLocal.Return.Binary.Left == nil || coreLocal.Return.Binary.Left.Parameter == nil || *coreLocal.Return.Binary.Left.Parameter != 2 || coreLocal.Return.Binary.Right == nil || coreLocal.Return.Binary.Right.Parameter == nil || *coreLocal.Return.Binary.Right.Parameter != 1 {
		t.Fatalf("continuation Core = %#v", coreLocal.Return)
	}

	entry := coreir.SemanticIdentity{PackageID: string(identity.PackageID), Path: string(identity.Path)}
	resultType := coreFunction.Parameters[0].Type
	for _, test := range []struct {
		name    string
		carrier coreeval.Outcome
		operand int64
		ok      bool
		value   int64
		failure coreir.ArithmeticError
	}{
		{name: "success", carrier: coreeval.Outcome{OK: true, Value: coreeval.Value{Type: resultType.Result.Success, Int: 40}}, operand: 2, ok: true, value: 42},
		{name: "continuation overflow", carrier: coreeval.Outcome{OK: true, Value: coreeval.Value{Type: resultType.Result.Success, Int: math.MaxInt64}}, operand: 1, failure: coreir.ArithmeticOverflow},
		{name: "propagated overflow", carrier: coreeval.Outcome{Value: coreeval.Value{Type: resultType.Result.Success}, Error: coreir.ArithmeticOverflow}, operand: 2, failure: coreir.ArithmeticOverflow},
	} {
		outcome, evalErr := coreeval.EvaluateProgram(program, entry, []coreeval.Value{{Type: resultType, Result: &test.carrier}, {Type: coreFunction.Parameters[1].Type, Int: test.operand}})
		if evalErr != nil || outcome.OK != test.ok || outcome.Error != test.failure || (test.ok && outcome.Value.Int != test.value) {
			t.Fatalf("%s = %#v, %v", test.name, outcome, evalErr)
		}
	}
	malformed := coreeval.Outcome{OK: true, Value: coreeval.Value{Type: resultType.Result.Success}, Error: coreir.ArithmeticOverflow}
	if _, evalErr := coreeval.EvaluateProgram(program, entry, []coreeval.Value{{Type: resultType, Result: &malformed}, {Type: coreFunction.Parameters[1].Type, Int: 1}}); evalErr == nil {
		t.Fatal("evaluator accepted malformed arithmetic carrier")
	}

	generated, err := gobackend.Generate(program)
	if err != nil {
		t.Fatal(err)
	}
	generatedAgain, err := gobackend.Generate(program)
	if err != nil || string(generatedAgain) != string(generated) {
		t.Fatalf("generated Go is not deterministic: %v", err)
	}
	for _, fragment := range []string{"pipelangValidateArithmeticResult(p0)", "if !p0.OK {", "return p0", "p2 := p0.Value", "p1"} {
		if !strings.Contains(string(generated), fragment) {
			t.Fatalf("generated Go lacks multi-parameter direct propagation %q:\n%s", fragment, generated)
		}
	}
	compileAndRunGeneratedGoFiles(t, generated, []byte(fmt.Sprintf(`package %s

import (
	"math"
	"testing"
)

func TestGeneratedMultiParameterDirectCheckedPropagation(t *testing.T) {
	success := PipeLangContinue(PipeLangArithmeticResult[int64]{OK: true, Value: 40}, 2)
	if !success.OK || success.Value != 42 || success.Error != "" { t.Fatalf("success = %%#v", success) }
	overflow := PipeLangContinue(PipeLangArithmeticResult[int64]{OK: true, Value: math.MaxInt64}, 1)
	if overflow.OK || overflow.Error != PipeLangArithmeticOverflow { t.Fatalf("overflow = %%#v", overflow) }
	failure := PipeLangContinue(PipeLangArithmeticResult[int64]{Error: PipeLangArithmeticOverflow}, 2)
	if failure.OK || failure.Value != 0 || failure.Error != PipeLangArithmeticOverflow { t.Fatalf("failure = %%#v", failure) }
	defer func() { if recover() == nil { t.Fatal("malformed arithmetic carrier did not panic") } }()
	PipeLangContinue(PipeLangArithmeticResult[int64]{OK: true, Error: PipeLangArithmeticOverflow}, 1)
}
`, gobackend.PackageName)))

	testV560MultiParameterDirectFloatPropagation(t, analysis)
}

func testV560MultiParameterDirectFloatPropagation(t *testing.T, analysis *Analysis) {
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
		operand float64
		ok      bool
		value   float64
		failure coreir.ArithmeticError
	}{
		{carrier: coreeval.Outcome{OK: true, Value: coreeval.Value{Type: resultType.Result.Success, Float: 5}}, operand: 2, ok: true, value: 2.5},
		{carrier: coreeval.Outcome{OK: true, Value: coreeval.Value{Type: resultType.Result.Success, Float: 5}}, operand: 0, failure: coreir.ArithmeticDivisionByZero},
		{carrier: coreeval.Outcome{Value: coreeval.Value{Type: resultType.Result.Success}, Error: coreir.ArithmeticDivisionByZero}, operand: 2, failure: coreir.ArithmeticDivisionByZero},
	} {
		outcome, evalErr := coreeval.EvaluateProgram(program, entry, []coreeval.Value{{Type: resultType, Result: &test.carrier}, {Type: function.Parameters[1].Type, Float: test.operand}})
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

func TestGeneratedMultiParameterDirectFloatPropagation(t *testing.T) {
	success := PipeLangContinueFloat(PipeLangArithmeticResult[float64]{OK: true, Value: 5}, 2)
	if !success.OK || success.Value != 2.5 || success.Error != "" { t.Fatalf("success = %%#v", success) }
	failure := PipeLangContinueFloat(PipeLangArithmeticResult[float64]{OK: true, Value: 5}, 0)
	if failure.OK || failure.Error != PipeLangArithmeticDivisionByZero { t.Fatalf("failure = %%#v", failure) }
}
`, gobackend.PackageName)))
}

func TestV560MultiParameterDirectCheckedPropagationRejectsExcludedSource(t *testing.T) {
	valid := `public Class Root { public Result<int, ArithmeticError> Continue(Result<int, ArithmeticError> carrier, int operand) { int value = propagate(carrier); return value + operand; } }`
	tests := []struct {
		name     string
		contract LanguageContract
		source   string
		message  string
	}{
		{name: "prior contract", contract: PipeLangLanguageContractV550, source: valid, message: "sole direct carrier parameter"},
		{name: "third parameter", contract: PipeLangLanguageContractV560, source: strings.Replace(valid, "int operand)", "int operand, int extra)", 1), message: "exactly carrier and payload operand"},
		{name: "carrier second", contract: PipeLangLanguageContractV560, source: `public Class Root { public Result<int, ArithmeticError> Continue(int operand, Result<int, ArithmeticError> carrier) { int value = propagate(carrier); return value + operand; } }`, message: "sole direct carrier parameter"},
		{name: "wrong operand type", contract: PipeLangLanguageContractV560, source: strings.Replace(valid, "int operand)", "float operand)", 1), message: "operand type"},
		{name: "reversed continuation", contract: PipeLangLanguageContractV560, source: strings.Replace(valid, "value + operand", "operand + value", 1), message: "left operand"},
		{name: "repeated local", contract: PipeLangLanguageContractV560, source: strings.Replace(valid, "value + operand", "value + value", 1), message: "right operand"},
		{name: "literal operand", contract: PipeLangLanguageContractV560, source: strings.Replace(valid, "value + operand", "value + 1", 1), message: "right operand"},
		{name: "extra propagation", contract: PipeLangLanguageContractV560, source: strings.Replace(valid, "value + operand", "propagate(carrier) + operand", 1), message: "exactly one propagation"},
		{name: "helper operand", contract: PipeLangLanguageContractV560, source: `public Class Root { public Result<int, ArithmeticError> Identity(Result<int, ArithmeticError> carrier) => carrier; public Result<int, ArithmeticError> Continue(Result<int, ArithmeticError> carrier, int operand) { int value = propagate(Identity(carrier)); return value + operand; } }`, message: "inherited bounded matches"},
		{name: "wrong float continuation", contract: PipeLangLanguageContractV560, source: `public Class Root { public Result<float, ArithmeticError> Continue(Result<float, ArithmeticError> carrier, float operand) { float value = propagate(carrier); return value + operand; } }`, message: "left operand"},
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

func TestV560MultiParameterDirectCheckedPropagationCoreRejectsMalformedComposition(t *testing.T) {
	input := semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", "multi-parameter-direct-checked-propagation-core.pipe", multiParameterDirectCheckedPropagationSource)}, nil)
	input.LanguageContract = PipeLangLanguageContractV560
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
	prior.LanguageContract = coreir.LanguageContractV550
	if err := coreir.ValidateProgram(prior); err == nil || !strings.Contains(err.Error(), "sole direct carrier parameter") {
		t.Fatalf("prior contract error = %v", err)
	}
	if _, err := gobackend.Generate(prior); err == nil {
		t.Fatal("Go backend accepted v0.56 Core under v0.55")
	}

	badOperand := program
	badOperand.Functions = append([]coreir.Function(nil), program.Functions...)
	continued := coreFunctionNamed(t, badOperand, "Continue")
	local := *continued.Body.ImmutableLocal
	returned := *local.Return
	binary := *returned.Binary
	right := *binary.Right
	position := local.Position
	right.Parameter = &position
	binary.Right = &right
	returned.Binary = &binary
	local.Return = &returned
	continued.Body.ImmutableLocal = &local
	for index := range badOperand.Functions {
		if badOperand.Functions[index].Name == "Continue" {
			badOperand.Functions[index] = continued
		}
	}
	if err := coreir.ValidateProgram(badOperand); err == nil || !strings.Contains(err.Error(), "parameter 1 as right operand") {
		t.Fatalf("bad operand Core error = %v", err)
	}
	if _, err := gobackend.Generate(badOperand); err == nil {
		t.Fatal("Go backend accepted malformed v0.56 operand Core")
	}
}

func TestV560PreservesV550DirectAndV540HelperPropagation(t *testing.T) {
	for _, test := range []struct {
		name   string
		source string
		method string
	}{
		{name: "v0.55 direct propagation", source: directCheckedArithmeticPropagationSource, method: "Continue"},
		{name: "v0.54 helper propagation", source: checkedArithmeticHelperPropagationSource, method: "Resolve"},
	} {
		t.Run(test.name, func(t *testing.T) {
			input := semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", test.name+".pipe", test.source)}, nil)
			input.LanguageContract = PipeLangLanguageContractV560
			analysis := AnalyzeSemanticModuleSet(input)
			if err := analysis.Error(); err != nil {
				t.Fatal(err)
			}
			typed, err := LowerSemanticMethodToHIR(analysis, semanticMethodNamed(t, analysis, test.method).Identity)
			if err != nil {
				t.Fatal(err)
			}
			program, err := LowerHIRToCore(typed)
			if err != nil || program.LanguageContract != coreir.LanguageContractV560 {
				t.Fatalf("inherited program = %#v, %v", program, err)
			}
		})
	}
}

func TestV560CompilerCursorAdvancesCheckedResultWithWidth(t *testing.T) {
	const source = `public Class CompilerCursor {
		public Result<int, ArithmeticError> Advance(Result<int, ArithmeticError> checkedCursor, int width) {
			int cursor = propagate(checkedCursor);
			return cursor + width;
		}
	}`
	input := semanticTestModuleSet("compiler.cursor", []ModuleInput{testModule("compiler.cursor", "compiler-cursor-advance.pipe", source)}, nil)
	input.LanguageContract = PipeLangLanguageContractV560
	analysis := AnalyzeSemanticModuleSet(input)
	if err := analysis.Error(); err != nil {
		t.Fatal(err)
	}
	identity := semanticMethodNamed(t, analysis, "Advance").Identity
	typed, err := LowerSemanticMethodToHIR(analysis, identity)
	if err != nil {
		t.Fatal(err)
	}
	program, err := LowerHIRToCore(typed)
	if err != nil {
		t.Fatal(err)
	}
	function := coreFunctionNamed(t, program, "Advance")
	resultType := function.Parameters[0].Type
	entry := coreir.SemanticIdentity{PackageID: string(identity.PackageID), Path: string(identity.Path)}
	for _, test := range []struct {
		carrier coreeval.Outcome
		ok      bool
		value   int64
		failure coreir.ArithmeticError
	}{
		{carrier: coreeval.Outcome{OK: true, Value: coreeval.Value{Type: resultType.Result.Success, Int: 72}}, ok: true, value: 77},
		{carrier: coreeval.Outcome{Value: coreeval.Value{Type: resultType.Result.Success}, Error: coreir.ArithmeticOverflow}, failure: coreir.ArithmeticOverflow},
	} {
		outcome, evalErr := coreeval.EvaluateProgram(program, entry, []coreeval.Value{{Type: resultType, Result: &test.carrier}, {Type: function.Parameters[1].Type, Int: 5}})
		if evalErr != nil || outcome.OK != test.ok || outcome.Error != test.failure || (test.ok && outcome.Value.Int != test.value) {
			t.Fatalf("cursor outcome = %#v, %v", outcome, evalErr)
		}
	}
}
