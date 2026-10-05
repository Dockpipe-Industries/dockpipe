package pipelang

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"testing"

	"dockpipe/src/lib/pipelang/coreeval"
	"dockpipe/src/lib/pipelang/coreir"
	"dockpipe/src/lib/pipelang/gobackend"
	"dockpipe/src/lib/pipelang/hir"
)

const boundedTwoCarrierMatchesSource = `public Record Item { public string Value; }
public Class Root {
	public Result<string, string> First(string first, string second) => first == "" ? err<string, string>("first missing") : ok<string, string>(first);
	public Result<string, string> Second(string first, string second) => second == "" ? err<string, string>("second missing") : ok<string, string>(second);
	public string Combine(string first, string second) {
		Result<string, string> firstCarrier = First(first, second);
		string left = match(firstCarrier){ ok(value) => value, err(problem) => problem };
		string separator = left == "" ? "" : "/";
		Result<string, string> secondCarrier = Second(first, second);
		string right = match(secondCarrier){ ok(value) => value, err(problem) => left };
		return left + separator + right;
	}
	public Result<int, ArithmeticError> Add(int left, int right) => left + right;
	public Result<int, ArithmeticError> Multiply(int left, int right) => left * right;
	public int Arithmetic(int left, int right) {
		Result<int, ArithmeticError> sumCarrier = Add(left, right);
		int sum = match(sumCarrier){ ok(value) => value, err(problem) => 0 };
		Result<int, ArithmeticError> productCarrier = Multiply(left, right);
		int product = match(productCarrier){ ok(value) => value, err(problem) => sum };
		return product;
	}
	public Result<List<Item>, string> Snapshot(Result<List<Item>, string> snapshot) => snapshot;
	public Result<List<Item>, string> ConfirmSnapshot(Result<List<Item>, string> snapshot) => snapshot;
	public int CountSnapshots(Result<List<Item>, string> snapshot) {
		Result<List<Item>, string> firstCarrier = Snapshot(snapshot);
		int first = match(firstCarrier){ ok(items) => count(items), err(problem) => 0 };
		Result<List<Item>, string> secondCarrier = ConfirmSnapshot(snapshot);
		int second = match(secondCarrier){ ok(items) => count(items), err(problem) => first };
		return second;
	}
	public Result<float, ArithmeticError> Divide(float left, float right) => left / right;
	public Result<float, ArithmeticError> ConfirmDivide(float left, float right) => left / right;
	public float ArithmeticFloat(float left, float right) {
		Result<float, ArithmeticError> firstCarrier = Divide(left, right);
		float first = match(firstCarrier){ ok(value) => value, err(problem) => 0.0 };
		Result<float, ArithmeticError> secondCarrier = ConfirmDivide(left, right);
		float second = match(secondCarrier){ ok(value) => value, err(problem) => first };
		return second;
	}
}`

func TestV490BoundedTwoCarrierMatchesPipeline(t *testing.T) {
	input := semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", "bounded-two-carrier-matches.pipe", boundedTwoCarrierMatchesSource)}, nil)
	input.LanguageContract = PipeLangLanguageContractV490
	analysis := AnalyzeSemanticModuleSet(input)
	if err := analysis.Error(); err != nil {
		t.Fatal(err)
	}
	projection, err := BuildSemanticProjection(analysis)
	if err != nil {
		t.Fatal(err)
	}
	if projection.LanguageContract != PipeLangLanguageContractV490 || projection.Schema != PipeLangSemanticProjectionVersion || projection.CompilerContract != PipeLangCompilerContract {
		t.Fatalf("projection contract = %#v", projection)
	}

	identity := semanticMethodNamed(t, analysis, "Combine").Identity
	typed, err := LowerSemanticMethodToHIR(analysis, identity)
	if err != nil {
		t.Fatal(err)
	}
	combineHIR := hirFunctionNamed(t, typed, "Combine")
	if typed.LanguageContract != coreir.LanguageContractV490 || countHIRCarrierMatchPairs(combineHIR.Body) != 2 {
		t.Fatalf("Combine HIR does not contain two bounded carrier/match pairs: %#v", combineHIR.Body)
	}

	program, err := LowerHIRToCore(typed)
	if err != nil {
		t.Fatal(err)
	}
	combine := coreFunctionNamed(t, program, "Combine")
	if program.LanguageContract != coreir.LanguageContractV490 || len(coreCarrierMatchPairs(&combine.Body)) != 2 {
		t.Fatalf("Combine Core does not contain two bounded carrier/match pairs: %#v", combine.Body)
	}

	entry := coreir.SemanticIdentity{PackageID: string(identity.PackageID), Path: string(identity.Path)}
	text := coreir.Type{Kind: coreir.TypePrimitive, Primitive: coreir.PrimitiveString}
	for _, test := range []struct {
		first  string
		second string
		want   string
	}{
		{first: "alpha", second: "beta", want: "alpha/beta"},
		{first: "", second: "beta", want: "first missing/beta"},
		{first: "alpha", second: "", want: "alpha/alpha"},
		{first: "", second: "", want: "first missing/first missing"},
	} {
		outcome, evalErr := coreeval.EvaluateProgram(program, entry, []coreeval.Value{{Type: text, String: test.first}, {Type: text, String: test.second}})
		if evalErr != nil || !outcome.OK || outcome.Value.String != test.want {
			t.Fatalf("Combine(%q, %q) = %#v, %v, want %q", test.first, test.second, outcome, evalErr, test.want)
		}
	}

	generated, err := gobackend.Generate(program)
	if err != nil {
		t.Fatal(err)
	}
	generatedAgain, err := gobackend.Generate(program)
	if err != nil || string(generatedAgain) != string(generated) {
		t.Fatalf("two-match Go is nondeterministic: %v", err)
	}
	if strings.Count(string(generated), " := PipeLangFirst(") != 1 || strings.Count(string(generated), " := PipeLangSecond(") != 1 {
		t.Fatalf("generated Go does not call each helper exactly once:\n%s", generated)
	}
	compileAndRunGeneratedGoFiles(t, generated, []byte(fmt.Sprintf(`package %s

import "testing"

func TestGeneratedBoundedTwoCarrierMatches(t *testing.T) {
	if got := PipeLangCombine("alpha", "beta"); got != "alpha/beta" { t.Fatalf("both success = %%q", got) }
	if got := PipeLangCombine("", "beta"); got != "first missing/beta" { t.Fatalf("first failure = %%q", got) }
	if got := PipeLangCombine("alpha", ""); got != "alpha/alpha" { t.Fatalf("second failure = %%q", got) }
}
`, gobackend.PackageName)))

	testV490ArithmeticPairs(t, analysis)
	testV490ResultListPairs(t, analysis)
	testV490FloatArithmeticPairs(t, analysis)
}

func TestV490PreservesV480SingleCarrierMatch(t *testing.T) {
	input := semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", "v490-single-carrier-match.pipe", priorLocalCarrierMatchSource)}, nil)
	input.LanguageContract = PipeLangLanguageContractV490
	analysis := AnalyzeSemanticModuleSet(input)
	if err := analysis.Error(); err != nil {
		t.Fatal(err)
	}
	identity := semanticMethodNamed(t, analysis, "Resolve").Identity
	typed, err := LowerSemanticMethodToHIR(analysis, identity)
	if err != nil {
		t.Fatal(err)
	}
	program, err := LowerHIRToCore(typed)
	if err != nil {
		t.Fatal(err)
	}
	if typed.LanguageContract != coreir.LanguageContractV490 || program.LanguageContract != coreir.LanguageContractV490 {
		t.Fatalf("single-match contracts = HIR %q, Core %q", typed.LanguageContract, program.LanguageContract)
	}
	resolve := coreFunctionNamed(t, program, "Resolve")
	if len(coreCarrierMatchPairs(&resolve.Body)) != 1 {
		t.Fatal("v0.49.0 changed the admitted v0.48.0 single carrier/match shape")
	}
	if _, err := gobackend.Generate(program); err != nil {
		t.Fatal(err)
	}
}

func testV490ArithmeticPairs(t *testing.T, analysis *Analysis) {
	t.Helper()
	identity := semanticMethodNamed(t, analysis, "Arithmetic").Identity
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
		left  int64
		right int64
		want  int64
	}{
		{left: 6, right: 7, want: 42},
		{left: math.MaxInt64, right: 1, want: math.MaxInt64},
		{left: math.MaxInt64, right: 2, want: 0},
	} {
		outcome, evalErr := coreeval.EvaluateProgram(program, entry, coreIntArguments(test.left, test.right))
		if evalErr != nil || !outcome.OK || outcome.Value.Int != test.want {
			t.Fatalf("Arithmetic(%d, %d) = %#v, %v, want %d", test.left, test.right, outcome, evalErr, test.want)
		}
	}
	generated, err := gobackend.Generate(program)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(generated), " := PipeLangAdd(") != 1 || strings.Count(string(generated), " := PipeLangMultiply(") != 1 {
		t.Fatalf("generated arithmetic Go does not call each helper exactly once:\n%s", generated)
	}
	compileAndRunGeneratedGoFiles(t, generated, []byte(fmt.Sprintf(`package %s

import (
	"math"
	"testing"
)

func TestGeneratedBoundedIntegerCarrierMatches(t *testing.T) {
	if got := PipeLangArithmetic(6, 7); got != 42 { t.Fatalf("success = %%d", got) }
	if got := PipeLangArithmetic(math.MaxInt64, 1); got != math.MaxInt64 { t.Fatalf("multiply overflow = %%d", got) }
	if got := PipeLangArithmetic(math.MaxInt64, 2); got != 0 { t.Fatalf("both overflow = %%d", got) }
}
`, gobackend.PackageName)))
}

func testV490ResultListPairs(t *testing.T, analysis *Analysis) {
	t.Helper()
	identity := semanticMethodNamed(t, analysis, "CountSnapshots").Identity
	typed, err := LowerSemanticMethodToHIR(analysis, identity)
	if err != nil {
		t.Fatal(err)
	}
	program, err := LowerHIRToCore(typed)
	if err != nil {
		t.Fatal(err)
	}
	function := coreFunctionNamed(t, program, "CountSnapshots")
	if len(coreCarrierMatchPairs(&function.Body)) != 2 {
		t.Fatalf("CountSnapshots Core does not contain two bounded pairs: %#v", function.Body)
	}
	carrier := function.Parameters[0].Type
	itemType := carrier.Result.Success.List.Element
	items := coreeval.Value{Type: carrier.Result.Success, List: []coreeval.Value{{Type: itemType, Record: []coreeval.Value{{Type: itemType.Record.Fields[0].Type, String: "one"}}}}}
	success := coreeval.Value{Type: carrier, Result: &coreeval.Outcome{OK: true, Value: items}}
	failureText := coreeval.Value{Type: carrier.Result.Failure, String: "offline"}
	failure := coreeval.Value{Type: carrier, Result: &coreeval.Outcome{Value: coreeval.Value{Type: carrier.Result.Success}, Failure: &failureText}}
	entry := coreir.SemanticIdentity{PackageID: string(identity.PackageID), Path: string(identity.Path)}
	for _, test := range []struct {
		name  string
		value coreeval.Value
		want  int64
	}{{name: "success", value: success, want: 1}, {name: "failure", value: failure, want: 0}} {
		outcome, evalErr := coreeval.EvaluateProgram(program, entry, []coreeval.Value{test.value})
		if evalErr != nil || !outcome.OK || outcome.Value.Int != test.want {
			t.Fatalf("CountSnapshots %s = %#v, %v", test.name, outcome, evalErr)
		}
	}
	generated, err := gobackend.Generate(program)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(generated), " := PipeLangSnapshot(") != 1 || strings.Count(string(generated), " := PipeLangConfirmSnapshot(") != 1 {
		t.Fatalf("generated Result-list Go does not call each helper exactly once:\n%s", generated)
	}
	compileAndRunGeneratedGoFiles(t, generated, []byte(fmt.Sprintf(`package %s

import "testing"

func TestGeneratedBoundedResultListCarrierMatches(t *testing.T) {
	items := []PipeLangRecordTestPackageAppRootItem{{Value: "one"}}
	success := PipeLangResult[[]PipeLangRecordTestPackageAppRootItem, string]{OK: true, Value: items}
	if got := PipeLangCountSnapshots(success); got != 1 { t.Fatalf("success = %%d", got) }
	failure := PipeLangResult[[]PipeLangRecordTestPackageAppRootItem, string]{Error: "offline"}
	if got := PipeLangCountSnapshots(failure); got != 0 { t.Fatalf("failure = %%d", got) }
}
`, gobackend.PackageName)))
}

func testV490FloatArithmeticPairs(t *testing.T, analysis *Analysis) {
	t.Helper()
	identity := semanticMethodNamed(t, analysis, "ArithmeticFloat").Identity
	typed, err := LowerSemanticMethodToHIR(analysis, identity)
	if err != nil {
		t.Fatal(err)
	}
	program, err := LowerHIRToCore(typed)
	if err != nil {
		t.Fatal(err)
	}
	function := coreFunctionNamed(t, program, "ArithmeticFloat")
	if len(coreCarrierMatchPairs(&function.Body)) != 2 {
		t.Fatalf("ArithmeticFloat Core does not contain two bounded pairs: %#v", function.Body)
	}
	entry := coreir.SemanticIdentity{PackageID: string(identity.PackageID), Path: string(identity.Path)}
	for _, test := range []struct {
		left  float64
		right float64
		want  float64
	}{{left: 5, right: 2, want: 2.5}, {left: 1, right: 0, want: 0}} {
		outcome, evalErr := coreeval.EvaluateProgram(program, entry, coreFloatArguments(test.left, test.right))
		if evalErr != nil || !outcome.OK || outcome.Value.Float != test.want {
			t.Fatalf("ArithmeticFloat(%v, %v) = %#v, %v", test.left, test.right, outcome, evalErr)
		}
	}
	generated, err := gobackend.Generate(program)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(generated), " := PipeLangDivide(") != 1 || strings.Count(string(generated), " := PipeLangConfirmDivide(") != 1 {
		t.Fatalf("generated checked-float Go does not call each helper exactly once:\n%s", generated)
	}
	compileAndRunGeneratedGoFiles(t, generated, []byte(fmt.Sprintf(`package %s

import "testing"

func TestGeneratedBoundedFloatCarrierMatches(t *testing.T) {
	if got := PipeLangArithmeticFloat(5, 2); got != 2.5 { t.Fatalf("success = %%v", got) }
	if got := PipeLangArithmeticFloat(1, 0); got != 0 { t.Fatalf("division by zero = %%v", got) }
}
`, gobackend.PackageName)))
}

func TestV490BoundedTwoCarrierMatchesRejectsExcludedSource(t *testing.T) {
	tests := []struct {
		name     string
		contract LanguageContract
		source   string
		message  string
	}{
		{name: "prior contract", contract: PipeLangLanguageContractV480, source: boundedTwoCarrierMatchesSource, message: "exactly one match"},
		{name: "three pairs", contract: PipeLangLanguageContractV490, source: `public Class Root { public Optional<string> Read(string value) => some(value); public string Resolve(string value) { Optional<string> one = Read(value); string first = match(one){ some(item) => item, none => "" }; Optional<string> two = Read(value); string second = match(two){ some(item) => item, none => first }; Optional<string> three = Read(value); string third = match(three){ some(item) => item, none => second }; return third; } }`, message: "exactly two matches"},
		{name: "intervening local", contract: PipeLangLanguageContractV490, source: `public Class Root { public Optional<string> Read(string value) => some(value); public string Resolve(string value) { Optional<string> one = Read(value); Optional<string> copy = one; string first = match(one){ some(item) => item, none => "" }; Optional<string> two = Read(value); string second = match(two){ some(item) => item, none => first }; return second; } }`, message: "two non-overlapping adjacent"},
		{name: "direct helper matches", contract: PipeLangLanguageContractV490, source: `public Class Root { public Optional<string> Read(string value) => some(value); public string Resolve(string value) { string first = match(Read(value)){ some(item) => item, none => "" }; string second = match(Read(value)){ some(item) => item, none => first }; return second; } }`, message: "two non-overlapping adjacent"},
		{name: "computed second argument", contract: PipeLangLanguageContractV490, source: `public Class Root { public Optional<string> Read(string value) => some(value); public string Resolve(string value) { Optional<string> one = Read(value); string first = match(one){ some(item) => item, none => "" }; Optional<string> two = Read(trim(value)); string second = match(two){ some(item) => item, none => first }; return second; } }`, message: "every caller parameter"},
		{name: "wrong second carrier", contract: PipeLangLanguageContractV490, source: `public Class Root { public Optional<string> Read(string value) => some(value); public string Resolve(string value) { Optional<string> one = Read(value); string first = match(one){ some(item) => item, none => "" }; Optional<string> two = Read(value); string second = match(one){ some(item) => item, none => first }; return second; } }`, message: "two non-overlapping adjacent"},
		{name: "reversed second arms", contract: PipeLangLanguageContractV490, source: `public Class Root { public Optional<string> Read(string value) => some(value); public string Resolve(string value) { Optional<string> one = Read(value); string first = match(one){ some(item) => item, none => "" }; Optional<string> two = Read(value); string second = match(two){ none => first, some(item) => item }; return second; } }`, message: "source-ordered"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			input := semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", test.name+".pipe", test.source)}, nil)
			input.LanguageContract = test.contract
			analysis := AnalyzeSemanticModuleSet(input)
			if len(analysis.Diagnostics) == 0 || !strings.Contains(analysis.Diagnostics[0].Message, test.message) || !analysis.Diagnostics[0].Primary.IsValid() {
				t.Fatalf("diagnostics = %#v", analysis.Diagnostics)
			}
		})
	}
}

func TestV490BoundedTwoCarrierMatchesCoreRejectsMalformedComposition(t *testing.T) {
	input := semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", "bounded-two-carrier-matches-core.pipe", boundedTwoCarrierMatchesSource)}, nil)
	input.LanguageContract = PipeLangLanguageContractV490
	analysis := AnalyzeSemanticModuleSet(input)
	if err := analysis.Error(); err != nil {
		t.Fatal(err)
	}
	typed, err := LowerSemanticMethodToHIR(analysis, semanticMethodNamed(t, analysis, "Combine").Identity)
	if err != nil {
		t.Fatal(err)
	}
	program, err := LowerHIRToCore(typed)
	if err != nil {
		t.Fatal(err)
	}

	downgraded := program
	downgraded.LanguageContract = coreir.LanguageContractV480
	if err := coreir.ValidateProgram(downgraded); err == nil || !strings.Contains(err.Error(), "exactly one match") {
		t.Fatalf("downgraded Core error = %v", err)
	}

	encoded, err := json.Marshal(program)
	if err != nil {
		t.Fatal(err)
	}
	var badReference coreir.Program
	if err := json.Unmarshal(encoded, &badReference); err != nil {
		t.Fatal(err)
	}
	for index := range badReference.Functions {
		if badReference.Functions[index].Name != "Combine" {
			continue
		}
		pairs := coreCarrierMatchPairs(&badReference.Functions[index].Body)
		if len(pairs) != 2 {
			t.Fatalf("bad-reference setup pairs = %d", len(pairs))
		}
		wrong := pairs[0].carrier.Position
		pairs[1].match.Value.Parameter = &wrong
	}
	if err := coreir.ValidateProgram(badReference); err == nil || !strings.Contains(err.Error(), "two non-overlapping adjacent") {
		t.Fatalf("bad-reference Core error = %v", err)
	}
	if _, err := gobackend.Generate(badReference); err == nil {
		t.Fatal("Go backend accepted malformed v0.49.0 two-match Core")
	}
}

func countHIRCarrierMatchPairs(expression hir.Expr) int {
	count := 0
	for expression.Kind == hir.ExprImmutableLocal && expression.ImmutableLocal != nil && expression.ImmutableLocal.Initializer != nil && expression.ImmutableLocal.Return != nil {
		carrier := expression.ImmutableLocal
		next := carrier.Return
		if carrier.Initializer.Kind == hir.ExprCall && next.Kind == hir.ExprImmutableLocal && next.ImmutableLocal != nil && next.ImmutableLocal.Initializer != nil {
			matched := next.ImmutableLocal.Initializer
			if matched.Kind == hir.ExprMatch && matched.Match != nil && matched.Match.Value != nil && matched.Match.Value.Kind == hir.ExprReference && matched.Match.Value.Reference != nil && matched.Match.Value.Reference.Kind == hir.BindingLocal && matched.Match.Value.Reference.Position == carrier.Binding.Position {
				count++
				expression = *next.ImmutableLocal.Return
				continue
			}
		}
		expression = *carrier.Return
	}
	return count
}

type coreCarrierMatchPair struct {
	carrier *coreir.ImmutableLocal
	match   *coreir.Match
}

func coreCarrierMatchPairs(expression *coreir.Expr) []coreCarrierMatchPair {
	pairs := make([]coreCarrierMatchPair, 0, 2)
	for expression != nil && expression.Kind == coreir.ExprImmutableLocal && expression.ImmutableLocal != nil && expression.ImmutableLocal.Initializer != nil && expression.ImmutableLocal.Return != nil {
		carrier := expression.ImmutableLocal
		next := carrier.Return
		if carrier.Initializer.Kind == coreir.ExprCall && next.Kind == coreir.ExprImmutableLocal && next.ImmutableLocal != nil && next.ImmutableLocal.Initializer != nil && next.ImmutableLocal.Return != nil {
			matched := next.ImmutableLocal.Initializer
			if matched.Kind == coreir.ExprMatch && matched.Match != nil && matched.Match.Value != nil && matched.Match.Value.Kind == coreir.ExprReference && matched.Match.Value.Parameter != nil && *matched.Match.Value.Parameter == carrier.Position {
				pairs = append(pairs, coreCarrierMatchPair{carrier: carrier, match: matched.Match})
				expression = next.ImmutableLocal.Return
				continue
			}
		}
		expression = carrier.Return
	}
	return pairs
}
