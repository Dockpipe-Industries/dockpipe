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

const twoStageCheckedPropagationSource = `public Class Root {
	public Result<int, ArithmeticError> AdvanceTwice(Result<int, ArithmeticError> carrier, int first, int second) {
		int value = propagate(carrier);
		Result<int, ArithmeticError> nextCarrier = value + first;
		int next = propagate(nextCarrier);
		return next + second;
	}
	public Result<float, ArithmeticError> DivideTwice(Result<float, ArithmeticError> carrier, float first, float second) {
		float value = propagate(carrier);
		Result<float, ArithmeticError> nextCarrier = value / first;
		float next = propagate(nextCarrier);
		return next / second;
	}
}`

func TestV570TwoStageCheckedPropagationPipeline(t *testing.T) {
	input := semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", "two-stage-checked-propagation.pipe", twoStageCheckedPropagationSource)}, nil)
	input.LanguageContract = PipeLangLanguageContractV570
	analysis := AnalyzeSemanticModuleSet(input)
	if err := analysis.Error(); err != nil {
		t.Fatal(err)
	}
	projection, err := BuildSemanticProjection(analysis)
	if err != nil {
		t.Fatal(err)
	}
	if projection.LanguageContract != PipeLangLanguageContractV570 || projection.Schema != PipeLangSemanticProjectionVersion || projection.CompilerContract != PipeLangCompilerContract {
		t.Fatalf("projection contract = %#v", projection)
	}

	identity := semanticMethodNamed(t, analysis, "AdvanceTwice").Identity
	typed, err := LowerSemanticMethodToHIR(analysis, identity)
	if err != nil {
		t.Fatal(err)
	}
	function := hirFunctionNamed(t, typed, "AdvanceTwice")
	first := function.Body.ImmutableLocal
	if typed.LanguageContract != coreir.LanguageContractV570 || len(function.Parameters) != 3 || first == nil || first.Binding.Position != 3 || first.Initializer.Kind != hir.ExprPropagate {
		t.Fatalf("first propagation HIR = %#v", function.Body)
	}
	second := first.Return.ImmutableLocal
	if second == nil || second.Binding.Position != 4 || second.Initializer.Kind != hir.ExprBinary || second.Initializer.Binary == nil || second.Initializer.Binary.Operator != hir.OperatorAdd {
		t.Fatalf("first checked stage HIR = %#v", first.Return)
	}
	third := second.Return.ImmutableLocal
	if third == nil || third.Binding.Position != 5 || third.Initializer.Kind != hir.ExprPropagate || third.Initializer.Propagate == nil || third.Initializer.Propagate.Value == nil || third.Initializer.Propagate.Value.Reference == nil || third.Initializer.Propagate.Value.Reference.Position != 4 {
		t.Fatalf("second propagation HIR = %#v", second.Return)
	}
	if third.Return == nil || third.Return.Kind != hir.ExprBinary || third.Return.Binary == nil || third.Return.Binary.Left == nil || third.Return.Binary.Left.Reference == nil || third.Return.Binary.Left.Reference.Position != 5 || third.Return.Binary.Right == nil || third.Return.Binary.Right.Reference == nil || third.Return.Binary.Right.Reference.Position != 2 {
		t.Fatalf("terminal checked stage HIR = %#v", third.Return)
	}

	program, err := LowerHIRToCore(typed)
	if err != nil {
		t.Fatal(err)
	}
	coreFunction := coreFunctionNamed(t, program, "AdvanceTwice")
	coreFirst := coreFunction.Body.ImmutableLocal
	coreSecond := coreFirst.Return.ImmutableLocal
	coreThird := coreSecond.Return.ImmutableLocal
	if program.LanguageContract != coreir.LanguageContractV570 || coreFirst.Position != 3 || coreSecond.Position != 4 || coreThird.Position != 5 || coreThird.Initializer == nil || coreThird.Initializer.Propagate == nil || coreThird.Initializer.Propagate.Value == nil || coreThird.Initializer.Propagate.Value.Parameter == nil || *coreThird.Initializer.Propagate.Value.Parameter != 4 {
		t.Fatalf("two-stage Core = %#v", coreFunction.Body)
	}

	entry := coreir.SemanticIdentity{PackageID: string(identity.PackageID), Path: string(identity.Path)}
	resultType := coreFunction.Parameters[0].Type
	for _, test := range []struct {
		name    string
		carrier coreeval.Outcome
		first   int64
		second  int64
		ok      bool
		value   int64
		failure coreir.ArithmeticError
	}{
		{name: "success", carrier: coreeval.Outcome{OK: true, Value: coreeval.Value{Type: resultType.Result.Success, Int: 40}}, first: 1, second: 1, ok: true, value: 42},
		{name: "incoming failure", carrier: coreeval.Outcome{Value: coreeval.Value{Type: resultType.Result.Success}, Error: coreir.ArithmeticOverflow}, first: 1, second: 1, failure: coreir.ArithmeticOverflow},
		{name: "first stage overflow", carrier: coreeval.Outcome{OK: true, Value: coreeval.Value{Type: resultType.Result.Success, Int: math.MaxInt64}}, first: 1, second: -1, failure: coreir.ArithmeticOverflow},
		{name: "second stage overflow", carrier: coreeval.Outcome{OK: true, Value: coreeval.Value{Type: resultType.Result.Success, Int: math.MaxInt64 - 1}}, first: 1, second: 1, failure: coreir.ArithmeticOverflow},
	} {
		outcome, evalErr := coreeval.EvaluateProgram(program, entry, []coreeval.Value{{Type: resultType, Result: &test.carrier}, {Type: coreFunction.Parameters[1].Type, Int: test.first}, {Type: coreFunction.Parameters[2].Type, Int: test.second}})
		if evalErr != nil || outcome.OK != test.ok || outcome.Error != test.failure || (test.ok && outcome.Value.Int != test.value) {
			t.Fatalf("%s = %#v, %v", test.name, outcome, evalErr)
		}
	}
	malformed := coreeval.Outcome{OK: true, Value: coreeval.Value{Type: resultType.Result.Success, Int: 1}, Error: coreir.ArithmeticOverflow}
	if _, evalErr := coreeval.EvaluateProgram(program, entry, []coreeval.Value{{Type: resultType, Result: &malformed}, {Type: coreFunction.Parameters[1].Type, Int: 1}, {Type: coreFunction.Parameters[2].Type, Int: 1}}); evalErr == nil {
		t.Fatal("evaluator accepted malformed incoming arithmetic carrier")
	}

	generated, err := gobackend.Generate(program)
	if err != nil {
		t.Fatal(err)
	}
	generatedAgain, err := gobackend.Generate(program)
	if err != nil || string(generatedAgain) != string(generated) {
		t.Fatalf("generated Go is not deterministic: %v", err)
	}
	for _, fragment := range []string{"pipelangValidateArithmeticResult(p0)", "p3 := p0.Value", "p4 := pipelangCheckedAddInt64(p3, p1)", "pipelangValidateArithmeticResult(p4)", "p5 := p4.Value", "pipelangCheckedAddInt64(p5, p2)"} {
		if !strings.Contains(string(generated), fragment) {
			t.Fatalf("generated Go lacks two-stage propagation %q:\n%s", fragment, generated)
		}
	}
	compileAndRunGeneratedGoFiles(t, generated, []byte(fmt.Sprintf(`package %s

import (
	"math"
	"testing"
)

func TestGeneratedTwoStageCheckedPropagation(t *testing.T) {
	if got := PipeLangAdvanceTwice(PipeLangArithmeticResult[int64]{OK: true, Value: 40}, 1, 1); !got.OK || got.Value != 42 { t.Fatalf("success = %%#v", got) }
	if got := PipeLangAdvanceTwice(PipeLangArithmeticResult[int64]{OK: true, Value: math.MaxInt64}, 1, -1); got.OK || got.Error != PipeLangArithmeticOverflow { t.Fatalf("first overflow = %%#v", got) }
	if got := PipeLangAdvanceTwice(PipeLangArithmeticResult[int64]{OK: true, Value: math.MaxInt64 - 1}, 1, 1); got.OK || got.Error != PipeLangArithmeticOverflow { t.Fatalf("second overflow = %%#v", got) }
	if got := PipeLangAdvanceTwice(PipeLangArithmeticResult[int64]{Error: PipeLangArithmeticOverflow}, 1, 1); got.OK || got.Error != PipeLangArithmeticOverflow { t.Fatalf("incoming failure = %%#v", got) }
}
`, gobackend.PackageName)))

	testV570TwoStageFloatPropagation(t, analysis)
}

func testV570TwoStageFloatPropagation(t *testing.T, analysis *Analysis) {
	t.Helper()
	identity := semanticMethodNamed(t, analysis, "DivideTwice").Identity
	typed, err := LowerSemanticMethodToHIR(analysis, identity)
	if err != nil {
		t.Fatal(err)
	}
	program, err := LowerHIRToCore(typed)
	if err != nil {
		t.Fatal(err)
	}
	function := coreFunctionNamed(t, program, "DivideTwice")
	resultType := function.Parameters[0].Type
	entry := coreir.SemanticIdentity{PackageID: string(identity.PackageID), Path: string(identity.Path)}
	for _, test := range []struct {
		first, second float64
		ok            bool
		value         float64
		failure       coreir.ArithmeticError
	}{
		{first: 2, second: 2, ok: true, value: 2.5},
		{first: 0, second: 2, failure: coreir.ArithmeticDivisionByZero},
		{first: 2, second: 0, failure: coreir.ArithmeticDivisionByZero},
	} {
		carrier := coreeval.Outcome{OK: true, Value: coreeval.Value{Type: resultType.Result.Success, Float: 10}}
		outcome, evalErr := coreeval.EvaluateProgram(program, entry, []coreeval.Value{{Type: resultType, Result: &carrier}, {Type: function.Parameters[1].Type, Float: test.first}, {Type: function.Parameters[2].Type, Float: test.second}})
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

func TestGeneratedTwoStageFloatPropagation(t *testing.T) {
	if got := PipeLangDivideTwice(PipeLangArithmeticResult[float64]{OK: true, Value: 10}, 2, 2); !got.OK || got.Value != 2.5 { t.Fatalf("success = %%#v", got) }
	if got := PipeLangDivideTwice(PipeLangArithmeticResult[float64]{OK: true, Value: 10}, 0, 2); got.OK || got.Error != PipeLangArithmeticDivisionByZero { t.Fatalf("first failure = %%#v", got) }
	if got := PipeLangDivideTwice(PipeLangArithmeticResult[float64]{OK: true, Value: 10}, 2, 0); got.OK || got.Error != PipeLangArithmeticDivisionByZero { t.Fatalf("second failure = %%#v", got) }
}
`, gobackend.PackageName)))
}

func TestV570TwoStageCheckedPropagationIntegerOperatorMatrix(t *testing.T) {
	for _, firstOperator := range []string{"+", "-", "*"} {
		for _, secondOperator := range []string{"+", "-", "*"} {
			source := strings.Replace(twoStageCheckedPropagationSource, "value + first", "value "+firstOperator+" first", 1)
			source = strings.Replace(source, "next + second", "next "+secondOperator+" second", 1)
			input := semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", "operator-matrix.pipe", source)}, nil)
			input.LanguageContract = PipeLangLanguageContractV570
			analysis := AnalyzeSemanticModuleSet(input)
			if err := analysis.Error(); err != nil {
				t.Fatalf("operators %s then %s: %v", firstOperator, secondOperator, err)
			}
			identity := semanticMethodNamed(t, analysis, "AdvanceTwice").Identity
			typed, err := LowerSemanticMethodToHIR(analysis, identity)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := LowerHIRToCore(typed); err != nil {
				t.Fatalf("operators %s then %s Core: %v", firstOperator, secondOperator, err)
			}
		}
	}
}

func TestV570TwoStageCheckedPropagationRejectsExcludedSource(t *testing.T) {
	valid := `public Class Root { public Result<int, ArithmeticError> Advance(Result<int, ArithmeticError> carrier, int first, int second) { int value = propagate(carrier); Result<int, ArithmeticError> nextCarrier = value + first; int next = propagate(nextCarrier); return next + second; } }`
	tests := []struct {
		name     string
		contract LanguageContract
		source   string
		message  string
	}{
		{name: "prior contract", contract: PipeLangLanguageContractV560, source: valid, message: "exactly one propagation"},
		{name: "fourth parameter", contract: PipeLangLanguageContractV570, source: strings.Replace(valid, "int second)", "int second, int extra)", 1), message: "exactly carrier and two payload operands"},
		{name: "wrong second operand type", contract: PipeLangLanguageContractV570, source: strings.Replace(valid, "int second)", "float second)", 1), message: "operand types"},
		{name: "first stage reversed", contract: PipeLangLanguageContractV570, source: strings.Replace(valid, "value + first", "first + value", 1), message: "first checked stage"},
		{name: "terminal repeated", contract: PipeLangLanguageContractV570, source: strings.Replace(valid, "next + second", "next + next", 1), message: "terminal checked stage"},
		{name: "computed propagation", contract: PipeLangLanguageContractV570, source: strings.Replace(valid, "Result<int, ArithmeticError> nextCarrier = value + first; int next = propagate(nextCarrier);", "int next = propagate(value + first);", 1), message: "first checked stage"},
		{name: "extra stage", contract: PipeLangLanguageContractV570, source: strings.Replace(valid, "return next + second;", "Result<int, ArithmeticError> finalCarrier = next + second; int final = propagate(finalCarrier); return final + second;", 1), message: "exactly two propagations"},
		{name: "wrong float first stage", contract: PipeLangLanguageContractV570, source: strings.Replace(twoStageCheckedPropagationSource, "value / first", "value + first", 1), message: "first checked stage"},
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

func TestV570TwoStageCheckedPropagationCoreRejectsMalformedComposition(t *testing.T) {
	input := semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", "two-stage-checked-propagation-core.pipe", twoStageCheckedPropagationSource)}, nil)
	input.LanguageContract = PipeLangLanguageContractV570
	analysis := AnalyzeSemanticModuleSet(input)
	if err := analysis.Error(); err != nil {
		t.Fatal(err)
	}
	typed, err := LowerSemanticMethodToHIR(analysis, semanticMethodNamed(t, analysis, "AdvanceTwice").Identity)
	if err != nil {
		t.Fatal(err)
	}
	program, err := LowerHIRToCore(typed)
	if err != nil {
		t.Fatal(err)
	}

	prior := program
	prior.LanguageContract = coreir.LanguageContractV560
	if err := coreir.ValidateProgram(prior); err == nil || !strings.Contains(err.Error(), "exactly one propagation") {
		t.Fatalf("prior contract error = %v", err)
	}
	if _, err := gobackend.Generate(prior); err == nil {
		t.Fatal("Go backend accepted v0.57 Core under v0.56")
	}

	malformed := program
	malformed.Functions = append([]coreir.Function(nil), program.Functions...)
	function := coreFunctionNamed(t, malformed, "AdvanceTwice")
	first := *function.Body.ImmutableLocal
	second := *first.Return.ImmutableLocal
	third := *second.Return.ImmutableLocal
	initializer := *third.Initializer
	propagation := *initializer.Propagate
	badReference := *propagation.Value
	badPosition := first.Position
	badReference.Parameter = &badPosition
	propagation.Value = &badReference
	initializer.Propagate = &propagation
	third.Initializer = &initializer
	second.Return.ImmutableLocal = &third
	first.Return.ImmutableLocal = &second
	function.Body.ImmutableLocal = &first
	for index := range malformed.Functions {
		if malformed.Functions[index].Name == "AdvanceTwice" {
			malformed.Functions[index] = function
		}
	}
	if err := coreir.ValidateProgram(malformed); err == nil || !strings.Contains(err.Error(), "second propagation") {
		t.Fatalf("malformed Core error = %v", err)
	}
	if _, err := gobackend.Generate(malformed); err == nil {
		t.Fatal("Go backend accepted malformed v0.57 propagation Core")
	}
}

func TestV570CompilerCursorAdvancesTwoCheckedWidths(t *testing.T) {
	const source = `public Class CompilerCursor {
		public Result<int, ArithmeticError> AdvanceTwice(Result<int, ArithmeticError> checkedCursor, int tokenWidth, int triviaWidth) {
			int cursor = propagate(checkedCursor);
			Result<int, ArithmeticError> tokenCursor = cursor + tokenWidth;
			int afterToken = propagate(tokenCursor);
			return afterToken + triviaWidth;
		}
	}`
	input := semanticTestModuleSet("compiler.cursor", []ModuleInput{testModule("compiler.cursor", "compiler-cursor-two-stage.pipe", source)}, nil)
	input.LanguageContract = PipeLangLanguageContractV570
	analysis := AnalyzeSemanticModuleSet(input)
	if err := analysis.Error(); err != nil {
		t.Fatal(err)
	}
	identity := semanticMethodNamed(t, analysis, "AdvanceTwice").Identity
	typed, err := LowerSemanticMethodToHIR(analysis, identity)
	if err != nil {
		t.Fatal(err)
	}
	program, err := LowerHIRToCore(typed)
	if err != nil {
		t.Fatal(err)
	}
	function := coreFunctionNamed(t, program, "AdvanceTwice")
	resultType := function.Parameters[0].Type
	entry := coreir.SemanticIdentity{PackageID: string(identity.PackageID), Path: string(identity.Path)}
	carrier := coreeval.Outcome{OK: true, Value: coreeval.Value{Type: resultType.Result.Success, Int: 72}}
	outcome, evalErr := coreeval.EvaluateProgram(program, entry, []coreeval.Value{{Type: resultType, Result: &carrier}, {Type: function.Parameters[1].Type, Int: 5}, {Type: function.Parameters[2].Type, Int: 2}})
	if evalErr != nil || !outcome.OK || outcome.Value.Int != 79 {
		t.Fatalf("cursor outcome = %#v, %v", outcome, evalErr)
	}
}

func TestV570PreservesV560V550AndV540CheckedPropagation(t *testing.T) {
	for _, test := range []struct {
		name   string
		source string
		method string
	}{
		{name: "v0.56 multi-parameter direct", source: multiParameterDirectCheckedPropagationSource, method: "Continue"},
		{name: "v0.55 sole-carrier direct", source: directCheckedArithmeticPropagationSource, method: "Continue"},
		{name: "v0.54 helper propagation", source: checkedArithmeticHelperPropagationSource, method: "Resolve"},
	} {
		t.Run(test.name, func(t *testing.T) {
			input := semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", test.name+".pipe", test.source)}, nil)
			input.LanguageContract = PipeLangLanguageContractV570
			analysis := AnalyzeSemanticModuleSet(input)
			if err := analysis.Error(); err != nil {
				t.Fatal(err)
			}
			typed, err := LowerSemanticMethodToHIR(analysis, semanticMethodNamed(t, analysis, test.method).Identity)
			if err != nil {
				t.Fatal(err)
			}
			program, err := LowerHIRToCore(typed)
			if err != nil || program.LanguageContract != coreir.LanguageContractV570 {
				t.Fatalf("inherited program = %#v, %v", program, err)
			}
		})
	}
}
