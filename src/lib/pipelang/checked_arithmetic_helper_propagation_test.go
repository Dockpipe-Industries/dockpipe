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

const checkedArithmeticHelperPropagationSource = `public Class Root {
	public Result<int, ArithmeticError> Add(int left, int right) => left + right;
	public Result<int, ArithmeticError> Resolve(int left, int right) {
		Result<int, ArithmeticError> carrier = Add(left, right);
		int value = propagate(carrier);
		return value + 0;
	}
	public Result<float, ArithmeticError> Divide(float left, float right) => left / right;
	public Result<float, ArithmeticError> ResolveFloat(float left, float right) {
		Result<float, ArithmeticError> carrier = Divide(left, right);
		float value = propagate(carrier);
		return value / 1.0;
	}
}`

func TestV540CheckedArithmeticHelperPropagationPipeline(t *testing.T) {
	input := semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", "checked-arithmetic-helper-propagation.pipe", checkedArithmeticHelperPropagationSource)}, nil)
	input.LanguageContract = PipeLangLanguageContractV540
	analysis := AnalyzeSemanticModuleSet(input)
	if err := analysis.Error(); err != nil {
		t.Fatal(err)
	}
	projection, err := BuildSemanticProjection(analysis)
	if err != nil {
		t.Fatal(err)
	}
	if projection.LanguageContract != PipeLangLanguageContractV540 || projection.Schema != PipeLangSemanticProjectionVersion || projection.CompilerContract != PipeLangCompilerContract {
		t.Fatalf("projection contract = %#v", projection)
	}

	identity := semanticMethodNamed(t, analysis, "Resolve").Identity
	typed, err := LowerSemanticMethodToHIR(analysis, identity)
	if err != nil {
		t.Fatal(err)
	}
	function := hirFunctionNamed(t, typed, "Resolve")
	first := function.Body.ImmutableLocal
	if typed.LanguageContract != coreir.LanguageContractV540 || first == nil || first.Initializer.Kind != hir.ExprCall || first.Initializer.Call == nil || len(first.Initializer.Call.Arguments) != 2 || first.Return == nil || first.Return.Kind != hir.ExprImmutableLocal {
		t.Fatalf("Resolve HIR = %#v", function.Body)
	}
	for position, argument := range first.Initializer.Call.Arguments {
		if argument.Kind != hir.ExprReference || argument.Reference == nil || argument.Reference.Kind != hir.BindingParameter || argument.Reference.Position != position {
			t.Fatalf("helper argument %d = %#v", position, argument)
		}
	}
	second := first.Return.ImmutableLocal
	if second == nil || second.Initializer.Kind != hir.ExprPropagate || second.Initializer.Propagate == nil || second.Initializer.Propagate.Value == nil || second.Initializer.Propagate.Value.Kind != hir.ExprReference || second.Initializer.Propagate.Value.Reference == nil || second.Initializer.Propagate.Value.Reference.Kind != hir.BindingLocal || second.Initializer.Propagate.Value.Reference.Position != first.Binding.Position {
		t.Fatalf("propagation HIR = %#v", function.Body)
	}

	program, err := LowerHIRToCore(typed)
	if err != nil {
		t.Fatal(err)
	}
	coreFunction := coreFunctionNamed(t, program, "Resolve")
	coreFirst := coreFunction.Body.ImmutableLocal
	if program.LanguageContract != coreir.LanguageContractV540 || coreFirst == nil || coreFirst.Initializer == nil || coreFirst.Initializer.Call == nil || len(coreFirst.Initializer.Call.Arguments) != 2 || coreFirst.Return == nil || coreFirst.Return.ImmutableLocal == nil {
		t.Fatalf("Resolve Core = %#v", coreFunction.Body)
	}
	coreSecond := coreFirst.Return.ImmutableLocal
	if coreSecond.Initializer == nil || coreSecond.Initializer.Propagate == nil || coreSecond.Initializer.Propagate.Value == nil || coreSecond.Initializer.Propagate.Value.Parameter == nil || *coreSecond.Initializer.Propagate.Value.Parameter != coreFirst.Position || coreSecond.Initializer.Propagate.Carrier.Result == nil || coreSecond.Initializer.Propagate.Carrier.Result.Failure.Kind != coreir.TypeArithmeticError {
		t.Fatalf("propagation Core = %#v", coreFunction.Body)
	}

	entry := coreir.SemanticIdentity{PackageID: string(identity.PackageID), Path: string(identity.Path)}
	for _, test := range []struct {
		name      string
		arguments []coreeval.Value
		ok        bool
		value     int64
		failure   coreir.ArithmeticError
	}{
		{name: "success", arguments: coreIntArguments(20, 22), ok: true, value: 42},
		{name: "helper overflow", arguments: coreIntArguments(math.MaxInt64, 1), failure: coreir.ArithmeticOverflow},
	} {
		outcome, evalErr := coreeval.EvaluateProgram(program, entry, test.arguments)
		if evalErr != nil || outcome.OK != test.ok || outcome.Error != test.failure || (test.ok && outcome.Value.Int != test.value) {
			t.Fatalf("%s = %#v, %v", test.name, outcome, evalErr)
		}
	}

	generated, err := gobackend.Generate(program)
	if err != nil {
		t.Fatal(err)
	}
	generatedAgain, err := gobackend.Generate(program)
	if err != nil || string(generatedAgain) != string(generated) {
		t.Fatalf("generated Go is not deterministic: %v", err)
	}
	for _, fragment := range []string{"p2 := PipeLangAdd(p0, p1)", "pipelangValidateArithmeticResult(p2)", "if !p2.OK {\n\t\treturn p2\n\t}", "p3 := p2.Value"} {
		if !strings.Contains(string(generated), fragment) {
			t.Fatalf("generated Go lacks checked-arithmetic propagation %q:\n%s", fragment, generated)
		}
	}
	if strings.Count(string(generated), "p2 := PipeLangAdd(p0, p1)") != 1 {
		t.Fatalf("generated Go does not evaluate the helper exactly once:\n%s", generated)
	}
	compileAndRunGeneratedGoFiles(t, generated, []byte(fmt.Sprintf(`package %s

import (
	"math"
	"testing"
)

func TestGeneratedCheckedArithmeticHelperPropagation(t *testing.T) {
	assert := func(got PipeLangArithmeticResult[int64], ok bool, value int64, arithmeticError PipeLangArithmeticError) {
		t.Helper()
		if got.OK != ok || got.Value != value || got.Error != arithmeticError { t.Fatalf("got %%#v", got) }
	}
	assert(PipeLangResolve(20, 22), true, 42, "")
	assert(PipeLangResolve(math.MaxInt64, 1), false, 0, PipeLangArithmeticOverflow)
	for _, malformed := range []PipeLangArithmeticResult[int64]{
		{OK: true, Error: PipeLangArithmeticOverflow},
		{Value: 1, Error: PipeLangArithmeticOverflow},
		{Error: PipeLangArithmeticError("unknown")},
	} {
		func() {
			defer func() { if recover() == nil { t.Fatal("malformed arithmetic carrier did not panic") } }()
			pipelangValidateArithmeticResult(malformed)
		}()
	}
}
`, gobackend.PackageName)))

	testV540FloatCheckedArithmeticHelperPropagation(t, analysis)
}

func testV540FloatCheckedArithmeticHelperPropagation(t *testing.T, analysis *Analysis) {
	t.Helper()
	identity := semanticMethodNamed(t, analysis, "ResolveFloat").Identity
	typed, err := LowerSemanticMethodToHIR(analysis, identity)
	if err != nil {
		t.Fatal(err)
	}
	program, err := LowerHIRToCore(typed)
	if err != nil {
		t.Fatal(err)
	}
	entry := coreir.SemanticIdentity{PackageID: string(identity.PackageID), Path: string(identity.Path)}
	for _, test := range []struct {
		arguments []coreeval.Value
		ok        bool
		value     float64
		failure   coreir.ArithmeticError
	}{
		{arguments: coreFloatArguments(5, 2), ok: true, value: 2.5},
		{arguments: coreFloatArguments(1, 0), failure: coreir.ArithmeticDivisionByZero},
	} {
		outcome, evalErr := coreeval.EvaluateProgram(program, entry, test.arguments)
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

func TestGeneratedFloatCheckedArithmeticHelperPropagation(t *testing.T) {
	success := PipeLangResolveFloat(5, 2)
	if !success.OK || success.Value != 2.5 || success.Error != "" { t.Fatalf("success = %%#v", success) }
	failure := PipeLangResolveFloat(1, 0)
	if failure.OK || failure.Value != 0 || failure.Error != PipeLangArithmeticDivisionByZero { t.Fatalf("failure = %%#v", failure) }
}
`, gobackend.PackageName)))
}

func TestV540CheckedArithmeticHelperPropagationRejectsExcludedSource(t *testing.T) {
	valid := checkedArithmeticHelperPropagationSource
	tests := []struct {
		name     string
		contract LanguageContract
		source   string
		message  string
	}{
		{name: "prior contract", contract: PipeLangLanguageContractV530, source: valid, message: "admitted Optional or bounded Result"},
		{name: "reordered", contract: PipeLangLanguageContractV540, source: strings.Replace(valid, "Add(left, right)", "Add(right, left)", 1), message: "declaration order"},
		{name: "repeated", contract: PipeLangLanguageContractV540, source: strings.Replace(valid, "Add(left, right)", "Add(left, left)", 1), message: "declaration order"},
		{name: "omitted", contract: PipeLangLanguageContractV540, source: strings.Replace(valid, "Add(left, right)", "Add(left)", 1), message: "declaration order"},
		{name: "extra", contract: PipeLangLanguageContractV540, source: strings.Replace(valid, "Add(left, right)", "Add(left, right, left)", 1), message: "declaration order"},
		{name: "computed", contract: PipeLangLanguageContractV540, source: strings.Replace(valid, "Add(left, right)", "Add(left + 0, right)", 1), message: "declaration order"},
		{name: "carrier not first", contract: PipeLangLanguageContractV540, source: strings.Replace(valid, "Result<int, ArithmeticError> carrier = Add(left, right);", "int prefix = 0; Result<int, ArithmeticError> carrier = Add(left, right);", 1), message: "one helper-call carrier local"},
		{name: "intervening local", contract: PipeLangLanguageContractV540, source: strings.Replace(valid, "int value = propagate(carrier);", "Result<int, ArithmeticError> copy = carrier; int value = propagate(copy);", 1), message: "second immutable-local initializer"},
		{name: "wrong carrier", contract: PipeLangLanguageContractV540, source: strings.Replace(valid, "propagate(carrier)", "propagate(left)", 1), message: "immediately preceding"},
		{name: "additional propagation", contract: PipeLangLanguageContractV540, source: strings.Replace(valid, "return value + 0;", "return propagate(carrier) + value;", 1), message: "one helper-call carrier local"},
		{name: "direct helper propagation", contract: PipeLangLanguageContractV540, source: strings.Replace(valid, "Result<int, ArithmeticError> carrier = Add(left, right);\n\t\tint value = propagate(carrier);", "int value = propagate(Add(left, right));", 1), message: "v0.52.0 admits"},
		{name: "direct parameter propagation", contract: PipeLangLanguageContractV540, source: `public Class Root { public Result<int, ArithmeticError> Resolve(Result<int, ArithmeticError> carrier) { int value = propagate(carrier); return value + 0; } }`, message: "admitted Optional or bounded Result"},
		{name: "wrong carrier family", contract: PipeLangLanguageContractV540, source: `public Class Root { public Result<int, string> Validate(int left, int right) => ok<int, string>(left); public Result<int, string> Resolve(int left, int right) { Result<int, string> carrier = Validate(left, right); int value = propagate(carrier); return ok<int, string>(value); } }`, message: "checked-arithmetic Result"},
		{name: "private helper", contract: PipeLangLanguageContractV540, source: strings.Replace(valid, "public Result<int, ArithmeticError> Add", "private Result<int, ArithmeticError> Add", 1), message: "must be public"},
		{name: "cross owner", contract: PipeLangLanguageContractV540, source: `public Class Other { public Result<int, ArithmeticError> Add(int left, int right) => left + right; } public Class Root { public Result<int, ArithmeticError> Resolve(int left, int right) { Result<int, ArithmeticError> carrier = Add(left, right); int value = propagate(carrier); return value + 0; } }`, message: "has no method"},
		{name: "overloaded helper", contract: PipeLangLanguageContractV540, source: `public Class Root { public Result<int, ArithmeticError> Add(int left, int right) => left + right; public Result<int, ArithmeticError> Add(int left) => left + 0; public Result<int, ArithmeticError> Resolve(int left, int right) { Result<int, ArithmeticError> carrier = Add(left, right); int value = propagate(carrier); return value + 0; } }`, message: "duplicate member"},
		{name: "generic helper", contract: PipeLangLanguageContractV540, source: `public Class Root { public Result<int, ArithmeticError> Add<T>(int left, int right) => left + right; public Result<int, ArithmeticError> Resolve(int left, int right) { Result<int, ArithmeticError> carrier = Add(left, right); int value = propagate(carrier); return value + 0; } }`, message: ""},
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

func TestV540CheckedArithmeticHelperPropagationCoreRejectsMalformedComposition(t *testing.T) {
	input := semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", "checked-arithmetic-helper-propagation-core.pipe", checkedArithmeticHelperPropagationSource)}, nil)
	input.LanguageContract = PipeLangLanguageContractV540
	analysis := AnalyzeSemanticModuleSet(input)
	if err := analysis.Error(); err != nil {
		t.Fatal(err)
	}
	typed, err := LowerSemanticMethodToHIR(analysis, semanticMethodNamed(t, analysis, "Resolve").Identity)
	if err != nil {
		t.Fatal(err)
	}
	program, err := LowerHIRToCore(typed)
	if err != nil {
		t.Fatal(err)
	}

	prior := program
	prior.LanguageContract = coreir.LanguageContractV530
	if err := coreir.ValidateProgram(prior); err == nil || !strings.Contains(err.Error(), "admitted Optional or bounded Result") {
		t.Fatalf("prior contract error = %v", err)
	}
	if _, err := gobackend.Generate(prior); err == nil {
		t.Fatal("Go backend accepted v0.54 arithmetic propagation under v0.53")
	}

	malformed := program
	malformed.Functions = append([]coreir.Function(nil), program.Functions...)
	resolve := coreFunctionNamed(t, malformed, "Resolve")
	first := *resolve.Body.ImmutableLocal
	initializer := *first.Initializer
	call := *initializer.Call
	call.Arguments = append([]*coreir.Expr(nil), call.Arguments...)
	call.Arguments[0], call.Arguments[1] = call.Arguments[1], call.Arguments[0]
	initializer.Call = &call
	first.Initializer = &initializer
	resolve.Body.ImmutableLocal = &first
	for index := range malformed.Functions {
		if malformed.Functions[index].Name == "Resolve" {
			malformed.Functions[index] = resolve
		}
	}
	if err := coreir.ValidateProgram(malformed); err == nil || !strings.Contains(err.Error(), "declaration order") {
		t.Fatalf("malformed Core error = %v", err)
	}
	if _, err := gobackend.Generate(malformed); err == nil {
		t.Fatal("Go backend accepted malformed arithmetic propagation Core")
	}

	badContinuation := program
	badContinuation.Functions = append([]coreir.Function(nil), program.Functions...)
	resolve = coreFunctionNamed(t, badContinuation, "Resolve")
	first = *resolve.Body.ImmutableLocal
	second := *first.Return.ImmutableLocal
	carrierPosition := first.Position
	second.Return = &coreir.Expr{Kind: coreir.ExprReference, Type: resolve.ReturnType, Parameter: &carrierPosition}
	first.Return = &coreir.Expr{Kind: coreir.ExprImmutableLocal, Type: resolve.ReturnType, ImmutableLocal: &second}
	resolve.Body.ImmutableLocal = &first
	for index := range badContinuation.Functions {
		if badContinuation.Functions[index].Name == "Resolve" {
			badContinuation.Functions[index] = resolve
		}
	}
	if err := coreir.ValidateProgram(badContinuation); err == nil || !strings.Contains(err.Error(), "checked arithmetic continuation") {
		t.Fatalf("bad continuation Core error = %v", err)
	}
	if _, err := gobackend.Generate(badContinuation); err == nil {
		t.Fatal("Go backend accepted arithmetic propagation without a checked continuation")
	}
}

func TestV540PreservesInheritedPropagationAndMatchChains(t *testing.T) {
	tests := []struct {
		name   string
		source string
		method string
	}{
		{name: "v0.53 multi-parameter propagation", source: multiParameterHelperPropagationSource, method: "Normalize"},
		{name: "v0.52 cumulative fan-in", source: cumulativeFanInChainSource, method: "Combine"},
		{name: "v0.51 immediate-only chain", source: dependentCarrierChainSource, method: "Combine"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			input := semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", test.name+".pipe", test.source)}, nil)
			input.LanguageContract = PipeLangLanguageContractV540
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
			if program.LanguageContract != coreir.LanguageContractV540 {
				t.Fatalf("contract = %s", program.LanguageContract)
			}
		})
	}
}

func TestV540CheckedArithmeticHelperPropagationPreservesOneParameterForm(t *testing.T) {
	const source = `public Class Root {
		public Result<int, ArithmeticError> Negate(int value) => -value;
		public Result<int, ArithmeticError> Resolve(int value) {
			Result<int, ArithmeticError> carrier = Negate(value);
			int selected = propagate(carrier);
			return selected + 0;
		}
	}`
	input := semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", "checked-arithmetic-one-parameter-propagation.pipe", source)}, nil)
	input.LanguageContract = PipeLangLanguageContractV540
	analysis := AnalyzeSemanticModuleSet(input)
	if err := analysis.Error(); err != nil {
		t.Fatal(err)
	}
	typed, err := LowerSemanticMethodToHIR(analysis, semanticMethodNamed(t, analysis, "Resolve").Identity)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := LowerHIRToCore(typed); err != nil {
		t.Fatal(err)
	}
}

func TestV540CompilerCursorConsumerPropagatesCheckedOffset(t *testing.T) {
	const source = `public Class CompilerCursor {
		public Result<int, ArithmeticError> AddOffset(int cursor, int width) => cursor + width;
		public Result<int, ArithmeticError> Advance(int cursor, int width) {
			Result<int, ArithmeticError> carrier = AddOffset(cursor, width);
			int next = propagate(carrier);
			return next + 0;
		}
	}`
	input := semanticTestModuleSet("compiler.cursor", []ModuleInput{testModule("compiler.cursor", "compiler-cursor.pipe", source)}, nil)
	input.LanguageContract = PipeLangLanguageContractV540
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
	entry := coreir.SemanticIdentity{PackageID: string(identity.PackageID), Path: string(identity.Path)}
	if outcome, evalErr := coreeval.EvaluateProgram(program, entry, coreIntArguments(64, 8)); evalErr != nil || !outcome.OK || outcome.Value.Int != 72 {
		t.Fatalf("cursor success = %#v, %v", outcome, evalErr)
	}
	if outcome, evalErr := coreeval.EvaluateProgram(program, entry, coreIntArguments(math.MaxInt64, 1)); evalErr != nil || outcome.OK || outcome.Error != coreir.ArithmeticOverflow {
		t.Fatalf("cursor overflow = %#v, %v", outcome, evalErr)
	}
	generated, err := gobackend.Generate(program)
	if err != nil {
		t.Fatal(err)
	}
	compileAndRunGeneratedGoFiles(t, generated, []byte(fmt.Sprintf(`package %s

import (
	"math"
	"testing"
)

func TestGeneratedCompilerCursor(t *testing.T) {
	if got := PipeLangAdvance(64, 8); !got.OK || got.Value != 72 { t.Fatalf("success = %%#v", got) }
	if got := PipeLangAdvance(math.MaxInt64, 1); got.OK || got.Error != PipeLangArithmeticOverflow { t.Fatalf("overflow = %%#v", got) }
}
`, gobackend.PackageName)))
}
