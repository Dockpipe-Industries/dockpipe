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

const checkedPropagationChainSource = `public Class CompilerCursor {
	public Result<int, ArithmeticError> AdvanceCompilerCursor(Result<int, ArithmeticError> carrier, int headerWidth, int payloadWidth, int trailerWidth) {
		int cursor = propagate(carrier);
		Result<int, ArithmeticError> afterHeader = cursor + headerWidth;
		int payloadStart = propagate(afterHeader);
		Result<int, ArithmeticError> afterPayload = payloadStart + payloadWidth;
		int trailerStart = propagate(afterPayload);
		return trailerStart + trailerWidth;
	}
	public Result<float, ArithmeticError> DivideCompilerScale(Result<float, ArithmeticError> carrier, float first, float second, float third) {
		float value = propagate(carrier);
		Result<float, ArithmeticError> afterFirst = value / first;
		float next = propagate(afterFirst);
		Result<float, ArithmeticError> afterSecond = next / second;
		float final = propagate(afterSecond);
		return final / third;
	}
}`

func TestV580CheckedPropagationChainPipeline(t *testing.T) {
	input := semanticTestModuleSet("compiler.cursor", []ModuleInput{testModule("compiler.cursor", "checked-propagation-chain.pipe", checkedPropagationChainSource)}, nil)
	input.LanguageContract = PipeLangLanguageContractV580
	analysis := AnalyzeSemanticModuleSet(input)
	if err := analysis.Error(); err != nil {
		t.Fatal(err)
	}
	projection, err := BuildSemanticProjection(analysis)
	if err != nil {
		t.Fatal(err)
	}
	if projection.LanguageContract != PipeLangLanguageContractV580 || projection.Schema != PipeLangSemanticProjectionVersion || projection.CompilerContract != PipeLangCompilerContract {
		t.Fatalf("projection contract = %#v", projection)
	}

	identity := semanticMethodNamed(t, analysis, "AdvanceCompilerCursor").Identity
	typed, err := LowerSemanticMethodToHIR(analysis, identity)
	if err != nil {
		t.Fatal(err)
	}
	function := hirFunctionNamed(t, typed, "AdvanceCompilerCursor")
	first := function.Body.ImmutableLocal
	if typed.LanguageContract != coreir.LanguageContractV580 || len(function.Parameters) != 4 || first == nil || first.Binding.Position != 4 || first.Initializer.Kind != hir.ExprPropagate {
		t.Fatalf("first propagation HIR = %#v", function.Body)
	}
	firstCarrier := first.Return.ImmutableLocal
	secondPayload := firstCarrier.Return.ImmutableLocal
	secondCarrier := secondPayload.Return.ImmutableLocal
	thirdPayload := secondCarrier.Return.ImmutableLocal
	if firstCarrier == nil || firstCarrier.Binding.Position != 5 || firstCarrier.Initializer.Kind != hir.ExprBinary || secondPayload == nil || secondPayload.Binding.Position != 6 || secondPayload.Initializer.Kind != hir.ExprPropagate || secondCarrier == nil || secondCarrier.Binding.Position != 7 || secondCarrier.Initializer.Kind != hir.ExprBinary || thirdPayload == nil || thirdPayload.Binding.Position != 8 || thirdPayload.Initializer.Kind != hir.ExprPropagate {
		t.Fatalf("checked propagation chain HIR = %#v", function.Body)
	}
	if thirdPayload.Return == nil || thirdPayload.Return.Kind != hir.ExprBinary || thirdPayload.Return.Binary == nil || thirdPayload.Return.Binary.Left == nil || thirdPayload.Return.Binary.Left.Reference == nil || thirdPayload.Return.Binary.Left.Reference.Position != 8 || thirdPayload.Return.Binary.Right == nil || thirdPayload.Return.Binary.Right.Reference == nil || thirdPayload.Return.Binary.Right.Reference.Position != 3 {
		t.Fatalf("terminal checked stage HIR = %#v", thirdPayload.Return)
	}

	program, err := LowerHIRToCore(typed)
	if err != nil {
		t.Fatal(err)
	}
	coreFunction := coreFunctionNamed(t, program, "AdvanceCompilerCursor")
	coreFirst := coreFunction.Body.ImmutableLocal
	coreFirstCarrier := coreFirst.Return.ImmutableLocal
	coreSecondPayload := coreFirstCarrier.Return.ImmutableLocal
	coreSecondCarrier := coreSecondPayload.Return.ImmutableLocal
	coreThirdPayload := coreSecondCarrier.Return.ImmutableLocal
	if program.LanguageContract != coreir.LanguageContractV580 || coreFirst.Position != 4 || coreFirstCarrier.Position != 5 || coreSecondPayload.Position != 6 || coreSecondCarrier.Position != 7 || coreThirdPayload.Position != 8 || coreThirdPayload.Initializer == nil || coreThirdPayload.Initializer.Propagate == nil || coreThirdPayload.Initializer.Propagate.Value == nil || coreThirdPayload.Initializer.Propagate.Value.Parameter == nil || *coreThirdPayload.Initializer.Propagate.Value.Parameter != 7 {
		t.Fatalf("checked propagation chain Core = %#v", coreFunction.Body)
	}

	entry := coreir.SemanticIdentity{PackageID: string(identity.PackageID), Path: string(identity.Path)}
	resultType := coreFunction.Parameters[0].Type
	for _, test := range []struct {
		name    string
		carrier coreeval.Outcome
		widths  [3]int64
		ok      bool
		value   int64
		failure coreir.ArithmeticError
	}{
		{name: "success", carrier: coreeval.Outcome{OK: true, Value: coreeval.Value{Type: resultType.Result.Success, Int: 39}}, widths: [3]int64{1, 1, 1}, ok: true, value: 42},
		{name: "incoming failure", carrier: coreeval.Outcome{Value: coreeval.Value{Type: resultType.Result.Success}, Error: coreir.ArithmeticOverflow}, widths: [3]int64{1, 1, 1}, failure: coreir.ArithmeticOverflow},
		{name: "first stage overflow", carrier: coreeval.Outcome{OK: true, Value: coreeval.Value{Type: resultType.Result.Success, Int: math.MaxInt64}}, widths: [3]int64{1, -1, -1}, failure: coreir.ArithmeticOverflow},
		{name: "second stage overflow", carrier: coreeval.Outcome{OK: true, Value: coreeval.Value{Type: resultType.Result.Success, Int: math.MaxInt64 - 1}}, widths: [3]int64{1, 1, -1}, failure: coreir.ArithmeticOverflow},
		{name: "terminal overflow", carrier: coreeval.Outcome{OK: true, Value: coreeval.Value{Type: resultType.Result.Success, Int: math.MaxInt64 - 2}}, widths: [3]int64{1, 1, 1}, failure: coreir.ArithmeticOverflow},
	} {
		arguments := []coreeval.Value{
			{Type: resultType, Result: &test.carrier},
			{Type: coreFunction.Parameters[1].Type, Int: test.widths[0]},
			{Type: coreFunction.Parameters[2].Type, Int: test.widths[1]},
			{Type: coreFunction.Parameters[3].Type, Int: test.widths[2]},
		}
		outcome, evalErr := coreeval.EvaluateProgram(program, entry, arguments)
		if evalErr != nil || outcome.OK != test.ok || outcome.Error != test.failure || (test.ok && outcome.Value.Int != test.value) {
			t.Fatalf("%s = %#v, %v", test.name, outcome, evalErr)
		}
	}
	malformed := coreeval.Outcome{OK: true, Value: coreeval.Value{Type: resultType.Result.Success, Int: 1}, Error: coreir.ArithmeticOverflow}
	if _, evalErr := coreeval.EvaluateProgram(program, entry, []coreeval.Value{{Type: resultType, Result: &malformed}, {Type: coreFunction.Parameters[1].Type, Int: 1}, {Type: coreFunction.Parameters[2].Type, Int: 1}, {Type: coreFunction.Parameters[3].Type, Int: 1}}); evalErr == nil {
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
	for _, fragment := range []string{"pipelangValidateArithmeticResult(p0)", "p4 := p0.Value", "p5 := pipelangCheckedAddInt64(p4, p1)", "pipelangValidateArithmeticResult(p5)", "p6 := p5.Value", "p7 := pipelangCheckedAddInt64(p6, p2)", "pipelangValidateArithmeticResult(p7)", "p8 := p7.Value", "pipelangCheckedAddInt64(p8, p3)"} {
		if !strings.Contains(string(generated), fragment) {
			t.Fatalf("generated Go lacks checked propagation chain %q:\n%s", fragment, generated)
		}
	}
	compileAndRunGeneratedGoFiles(t, generated, []byte(fmt.Sprintf(`package %s

import (
	"math"
	"testing"
)

func TestGeneratedCheckedPropagationChain(t *testing.T) {
	if got := PipeLangAdvanceCompilerCursor(PipeLangArithmeticResult[int64]{OK: true, Value: 39}, 1, 1, 1); !got.OK || got.Value != 42 { t.Fatalf("success = %%#v", got) }
	if got := PipeLangAdvanceCompilerCursor(PipeLangArithmeticResult[int64]{OK: true, Value: math.MaxInt64}, 1, -1, -1); got.OK || got.Error != PipeLangArithmeticOverflow { t.Fatalf("first overflow = %%#v", got) }
	if got := PipeLangAdvanceCompilerCursor(PipeLangArithmeticResult[int64]{OK: true, Value: math.MaxInt64 - 1}, 1, 1, -1); got.OK || got.Error != PipeLangArithmeticOverflow { t.Fatalf("second overflow = %%#v", got) }
	if got := PipeLangAdvanceCompilerCursor(PipeLangArithmeticResult[int64]{OK: true, Value: math.MaxInt64 - 2}, 1, 1, 1); got.OK || got.Error != PipeLangArithmeticOverflow { t.Fatalf("terminal overflow = %%#v", got) }
	if got := PipeLangAdvanceCompilerCursor(PipeLangArithmeticResult[int64]{Error: PipeLangArithmeticOverflow}, 1, 1, 1); got.OK || got.Error != PipeLangArithmeticOverflow { t.Fatalf("incoming failure = %%#v", got) }
}
`, gobackend.PackageName)))

	testV580CheckedPropagationFloatChain(t, analysis)
}

func testV580CheckedPropagationFloatChain(t *testing.T, analysis *Analysis) {
	t.Helper()
	identity := semanticMethodNamed(t, analysis, "DivideCompilerScale").Identity
	typed, err := LowerSemanticMethodToHIR(analysis, identity)
	if err != nil {
		t.Fatal(err)
	}
	program, err := LowerHIRToCore(typed)
	if err != nil {
		t.Fatal(err)
	}
	function := coreFunctionNamed(t, program, "DivideCompilerScale")
	resultType := function.Parameters[0].Type
	entry := coreir.SemanticIdentity{PackageID: string(identity.PackageID), Path: string(identity.Path)}
	for _, test := range []struct {
		divisors [3]float64
		ok       bool
		value    float64
		failure  coreir.ArithmeticError
	}{
		{divisors: [3]float64{2, 2, 2}, ok: true, value: 1},
		{divisors: [3]float64{0, 2, 2}, failure: coreir.ArithmeticDivisionByZero},
		{divisors: [3]float64{2, 0, 2}, failure: coreir.ArithmeticDivisionByZero},
		{divisors: [3]float64{2, 2, 0}, failure: coreir.ArithmeticDivisionByZero},
	} {
		carrier := coreeval.Outcome{OK: true, Value: coreeval.Value{Type: resultType.Result.Success, Float: 8}}
		outcome, evalErr := coreeval.EvaluateProgram(program, entry, []coreeval.Value{{Type: resultType, Result: &carrier}, {Type: function.Parameters[1].Type, Float: test.divisors[0]}, {Type: function.Parameters[2].Type, Float: test.divisors[1]}, {Type: function.Parameters[3].Type, Float: test.divisors[2]}})
		if evalErr != nil || outcome.OK != test.ok || outcome.Error != test.failure || (test.ok && outcome.Value.Float != test.value) {
			t.Fatalf("float outcome = %#v, %v", outcome, evalErr)
		}
	}
	incomingFailure := coreeval.Outcome{Value: coreeval.Value{Type: resultType.Result.Success}, Error: coreir.ArithmeticDivisionByZero}
	outcome, evalErr := coreeval.EvaluateProgram(program, entry, []coreeval.Value{{Type: resultType, Result: &incomingFailure}, {Type: function.Parameters[1].Type, Float: 2}, {Type: function.Parameters[2].Type, Float: 2}, {Type: function.Parameters[3].Type, Float: 2}})
	if evalErr != nil || outcome.OK || outcome.Error != coreir.ArithmeticDivisionByZero {
		t.Fatalf("float incoming failure = %#v, %v", outcome, evalErr)
	}
	generated, err := gobackend.Generate(program)
	if err != nil {
		t.Fatal(err)
	}
	compileAndRunGeneratedGoFiles(t, generated, []byte(fmt.Sprintf(`package %s

import "testing"

func TestGeneratedFloatCheckedPropagationChain(t *testing.T) {
	if got := PipeLangDivideCompilerScale(PipeLangArithmeticResult[float64]{OK: true, Value: 8}, 2, 2, 2); !got.OK || got.Value != 1 { t.Fatalf("success = %%#v", got) }
	if got := PipeLangDivideCompilerScale(PipeLangArithmeticResult[float64]{OK: true, Value: 8}, 0, 2, 2); got.OK || got.Error != PipeLangArithmeticDivisionByZero { t.Fatalf("first division = %%#v", got) }
	if got := PipeLangDivideCompilerScale(PipeLangArithmeticResult[float64]{OK: true, Value: 8}, 2, 0, 2); got.OK || got.Error != PipeLangArithmeticDivisionByZero { t.Fatalf("second division = %%#v", got) }
	if got := PipeLangDivideCompilerScale(PipeLangArithmeticResult[float64]{OK: true, Value: 8}, 2, 2, 0); got.OK || got.Error != PipeLangArithmeticDivisionByZero { t.Fatalf("terminal division = %%#v", got) }
	if got := PipeLangDivideCompilerScale(PipeLangArithmeticResult[float64]{Error: PipeLangArithmeticDivisionByZero}, 2, 2, 2); got.OK || got.Error != PipeLangArithmeticDivisionByZero { t.Fatalf("incoming failure = %%#v", got) }
}
`, gobackend.PackageName)))
}

func TestV580CheckedPropagationChainIntegerOperatorMatrix(t *testing.T) {
	for _, firstOperator := range []string{"+", "-", "*"} {
		for _, secondOperator := range []string{"+", "-", "*"} {
			for _, thirdOperator := range []string{"+", "-", "*"} {
				source := strings.Replace(checkedPropagationChainSource, "cursor + headerWidth", "cursor "+firstOperator+" headerWidth", 1)
				source = strings.Replace(source, "payloadStart + payloadWidth", "payloadStart "+secondOperator+" payloadWidth", 1)
				source = strings.Replace(source, "trailerStart + trailerWidth", "trailerStart "+thirdOperator+" trailerWidth", 1)
				input := semanticTestModuleSet("compiler.cursor", []ModuleInput{testModule("compiler.cursor", "operator-matrix.pipe", source)}, nil)
				input.LanguageContract = PipeLangLanguageContractV580
				analysis := AnalyzeSemanticModuleSet(input)
				if err := analysis.Error(); err != nil {
					t.Fatalf("operators %s, %s, %s: %v", firstOperator, secondOperator, thirdOperator, err)
				}
				identity := semanticMethodNamed(t, analysis, "AdvanceCompilerCursor").Identity
				typed, err := LowerSemanticMethodToHIR(analysis, identity)
				if err != nil {
					t.Fatal(err)
				}
				if _, err := LowerHIRToCore(typed); err != nil {
					t.Fatalf("operators %s, %s, %s Core: %v", firstOperator, secondOperator, thirdOperator, err)
				}
			}
		}
	}
}

func TestV580CheckedPropagationChainRejectsExcludedSource(t *testing.T) {
	valid := `public Class Root { public Result<int, ArithmeticError> Advance(Result<int, ArithmeticError> carrier, int first, int second, int third) { int value = propagate(carrier); Result<int, ArithmeticError> carrier1 = value + first; int next = propagate(carrier1); Result<int, ArithmeticError> carrier2 = next + second; int final = propagate(carrier2); return final + third; } }`
	tests := []struct {
		name     string
		contract LanguageContract
		source   string
		message  string
	}{
		{name: "prior contract", contract: PipeLangLanguageContractV570, source: valid, message: "exactly two propagations"},
		{name: "extra operand without stage", contract: PipeLangLanguageContractV580, source: strings.Replace(valid, "int third)", "int third, int fourth)", 1), message: "one propagation"},
		{name: "wrong operand type", contract: PipeLangLanguageContractV580, source: strings.Replace(valid, "int third)", "float third)", 1), message: "operand types"},
		{name: "first stage reversed", contract: PipeLangLanguageContractV580, source: strings.Replace(valid, "value + first", "first + value", 1), message: "checked stage 1"},
		{name: "second stage repeated", contract: PipeLangLanguageContractV580, source: strings.Replace(valid, "next + second", "next + next", 1), message: "checked stage 2"},
		{name: "computed propagation", contract: PipeLangLanguageContractV580, source: strings.Replace(valid, "Result<int, ArithmeticError> carrier2 = next + second; int final = propagate(carrier2);", "int final = propagate(next + second);", 1), message: "checked stage 2"},
		{name: "ordinary local gap", contract: PipeLangLanguageContractV580, source: strings.Replace(valid, "int final = propagate(carrier2);", "int gap = 0; int final = propagate(carrier2);", 1), message: "propagation"},
		{name: "terminal repeated", contract: PipeLangLanguageContractV580, source: strings.Replace(valid, "final + third", "final + final", 1), message: "terminal checked stage"},
		{name: "wrong float operator", contract: PipeLangLanguageContractV580, source: strings.Replace(checkedPropagationChainSource, "value / first", "value + first", 1), message: "checked stage 1"},
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

func TestV580CheckedPropagationChainPreservesTwoStageForm(t *testing.T) {
	input := semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", "two-stage-v058.pipe", twoStageCheckedPropagationSource)}, nil)
	input.LanguageContract = PipeLangLanguageContractV580
	analysis := AnalyzeSemanticModuleSet(input)
	if err := analysis.Error(); err != nil {
		t.Fatal(err)
	}
	identity := semanticMethodNamed(t, analysis, "AdvanceTwice").Identity
	typed, err := LowerSemanticMethodToHIR(analysis, identity)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := LowerHIRToCore(typed); err != nil {
		t.Fatal(err)
	}
}

func TestV580CheckedPropagationChainCoreRejectsMalformedComposition(t *testing.T) {
	input := semanticTestModuleSet("compiler.cursor", []ModuleInput{testModule("compiler.cursor", "malformed-chain.pipe", checkedPropagationChainSource)}, nil)
	input.LanguageContract = PipeLangLanguageContractV580
	analysis := AnalyzeSemanticModuleSet(input)
	if err := analysis.Error(); err != nil {
		t.Fatal(err)
	}
	identity := semanticMethodNamed(t, analysis, "AdvanceCompilerCursor").Identity
	typed, err := LowerSemanticMethodToHIR(analysis, identity)
	if err != nil {
		t.Fatal(err)
	}
	program, err := LowerHIRToCore(typed)
	if err != nil {
		t.Fatal(err)
	}
	bad := program
	bad.Functions = append([]coreir.Function(nil), program.Functions...)
	function := bad.Functions[len(bad.Functions)-1]
	firstCarrier := function.Body.ImmutableLocal.Return.ImmutableLocal
	firstCarrier.Position++
	bad.Functions[len(bad.Functions)-1] = function
	if err := coreir.ValidateProgram(bad); err == nil || !strings.Contains(err.Error(), "checked stage 1") {
		t.Fatalf("malformed Core error = %v", err)
	}
	if _, err := gobackend.Generate(bad); err == nil {
		t.Fatal("Go backend accepted malformed checked propagation chain")
	}
}
