package pipelang

import (
	"fmt"
	"math"
	"strings"
	"testing"

	"dockpipe/src/lib/pipelang/coreeval"
	"dockpipe/src/lib/pipelang/coreir"
	"dockpipe/src/lib/pipelang/gobackend"
)

const dependentSecondCarrierMatchSource = `public Record Item { public string Value; }
public Class Root {
	public Result<string, string> First(string first, string second) => first == "" ? err<string, string>("first missing") : ok<string, string>(first);
	public Result<string, string> Second(string selected, string first, string second) => second == "" ? err<string, string>(selected) : ok<string, string>(second);
	public string Combine(string first, string second) {
		Result<string, string> firstCarrier = First(first, second);
		string left = match(firstCarrier){ ok(value) => value, err(problem) => problem };
		Result<string, string> secondCarrier = Second(left, first, second);
		string right = match(secondCarrier){ ok(value) => value, err(problem) => problem };
		string joined = left + "/" + right;
		return joined;
	}
	public Result<int, ArithmeticError> Add(int left, int right) => left + right;
	public Result<int, ArithmeticError> Multiply(int left, int right) => left * right;
	public Result<int, ArithmeticError> MultiplyAfter(int sum, int left, int right) => Multiply(sum, right);
	public int Arithmetic(int left, int right) {
		Result<int, ArithmeticError> sumCarrier = Add(left, right);
		int sum = match(sumCarrier){ ok(value) => value, err(problem) => 0 };
		Result<int, ArithmeticError> productCarrier = MultiplyAfter(sum, left, right);
		int product = match(productCarrier){ ok(value) => value, err(problem) => sum };
		return product;
	}
	public Result<List<Item>, string> Snapshot(Result<List<Item>, string> snapshot) => snapshot;
	public Result<List<Item>, string> ConfirmSnapshot(int countBefore, Result<List<Item>, string> snapshot) => Snapshot(snapshot);
	public int CountSnapshots(Result<List<Item>, string> snapshot) {
		Result<List<Item>, string> firstCarrier = Snapshot(snapshot);
		int first = match(firstCarrier){ ok(items) => count(items), err(problem) => 0 };
		Result<List<Item>, string> secondCarrier = ConfirmSnapshot(first, snapshot);
		int second = match(secondCarrier){ ok(items) => count(items), err(problem) => first };
		return second;
	}
	public Result<float, ArithmeticError> Divide(float left, float right) => left / right;
	public Result<float, ArithmeticError> DivideAfter(float quotient, float left, float right) => Divide(quotient, right);
	public float ArithmeticFloat(float left, float right) {
		Result<float, ArithmeticError> firstCarrier = Divide(left, right);
		float first = match(firstCarrier){ ok(value) => value, err(problem) => 0.0 };
		Result<float, ArithmeticError> secondCarrier = DivideAfter(first, left, right);
		float second = match(secondCarrier){ ok(value) => value, err(problem) => first };
		return second;
	}
	public Optional<string> Read(string value) => some(value);
	public Optional<string> Confirm(string selected, string value) => Read(selected);
	public string Resolve(string value) {
		Optional<string> firstCarrier = Read(value);
		string first = match(firstCarrier){ some(item) => item, none => "" };
		Optional<string> secondCarrier = Confirm(first, value);
		string second = match(secondCarrier){ some(item) => item, none => first };
		return second;
	}
}`

func TestV500DependentSecondCarrierPipeline(t *testing.T) {
	input := semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", "dependent-second-carrier.pipe", dependentSecondCarrierMatchSource)}, nil)
	input.LanguageContract = PipeLangLanguageContractV500
	analysis := AnalyzeSemanticModuleSet(input)
	if err := analysis.Error(); err != nil {
		t.Fatal(err)
	}
	projection, err := BuildSemanticProjection(analysis)
	if err != nil {
		t.Fatal(err)
	}
	if projection.LanguageContract != PipeLangLanguageContractV500 || projection.Schema != PipeLangSemanticProjectionVersion || projection.CompilerContract != PipeLangCompilerContract {
		t.Fatalf("projection contract = %#v", projection)
	}

	identity := semanticMethodNamed(t, analysis, "Combine").Identity
	typed, err := LowerSemanticMethodToHIR(analysis, identity)
	if err != nil {
		t.Fatal(err)
	}
	program, err := LowerHIRToCore(typed)
	if err != nil {
		t.Fatal(err)
	}
	combine := coreFunctionNamed(t, program, "Combine")
	pairs := coreCarrierMatchPairs(&combine.Body)
	if typed.LanguageContract != coreir.LanguageContractV500 || program.LanguageContract != coreir.LanguageContractV500 || len(pairs) != 2 {
		t.Fatalf("dependent contracts or pair count drifted: HIR=%q Core=%q pairs=%d", typed.LanguageContract, program.LanguageContract, len(pairs))
	}
	secondCall := pairs[1].carrier.Initializer.Call
	firstSelected := pairs[0].carrier.Return.ImmutableLocal
	if secondCall == nil || firstSelected == nil || len(secondCall.Arguments) != 3 || secondCall.Arguments[0].Parameter == nil || *secondCall.Arguments[0].Parameter != firstSelected.Position || secondCall.Arguments[1].Parameter == nil || *secondCall.Arguments[1].Parameter != 0 || secondCall.Arguments[2].Parameter == nil || *secondCall.Arguments[2].Parameter != 1 {
		t.Fatalf("dependent second call arguments = %#v", secondCall)
	}

	entry := coreir.SemanticIdentity{PackageID: string(identity.PackageID), Path: string(identity.Path)}
	text := coreir.Type{Kind: coreir.TypePrimitive, Primitive: coreir.PrimitiveString}
	for _, test := range []struct{ first, second, want string }{
		{first: "alpha", second: "beta", want: "alpha/beta"},
		{first: "", second: "beta", want: "first missing/beta"},
		{first: "alpha", second: "", want: "alpha/alpha"},
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
	if err != nil || string(generatedAgain) != string(generated) || strings.Count(string(generated), " := PipeLangFirst(") != 1 || strings.Count(string(generated), " := PipeLangSecond(") != 1 {
		t.Fatalf("dependent generated Go is not deterministic and once-only: %v\n%s", err, generated)
	}
	compileAndRunGeneratedGoFiles(t, generated, []byte(fmt.Sprintf(`package %s

import "testing"

func TestGeneratedDependentSecondCarrier(t *testing.T) {
	if got := PipeLangCombine("alpha", "beta"); got != "alpha/beta" { t.Fatalf("success = %%q", got) }
	if got := PipeLangCombine("alpha", ""); got != "alpha/alpha" { t.Fatalf("dependent failure = %%q", got) }
}
`, gobackend.PackageName)))

	testV500DependentArithmetic(t, analysis)
	testV500DependentResultList(t, analysis)
	testV500DependentFloat(t, analysis)
	testV500DependentOptional(t, analysis)
}

func testV500DependentArithmetic(t *testing.T, analysis *Analysis) {
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
	for _, test := range []struct{ left, right, want int64 }{{6, 7, 91}, {math.MaxInt64, 1, 0}, {math.MaxInt64, 2, 0}} {
		outcome, evalErr := coreeval.EvaluateProgram(program, entry, coreIntArguments(test.left, test.right))
		if evalErr != nil || !outcome.OK || outcome.Value.Int != test.want {
			t.Fatalf("Arithmetic = %#v, %v", outcome, evalErr)
		}
	}
	generated, err := gobackend.Generate(program)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(generated), " := PipeLangAdd(") != 1 || strings.Count(string(generated), " := PipeLangMultiplyAfter(") != 1 {
		t.Fatalf("generated dependent integer Go does not call each helper once:\n%s", generated)
	}
	compileAndRunGeneratedGoFiles(t, generated, []byte(fmt.Sprintf(`package %s

import "testing"

func TestGeneratedDependentIntegerCarrierMatches(t *testing.T) {
	if got := PipeLangArithmetic(6, 7); got != 91 { t.Fatalf("success = %%d", got) }
	if got := PipeLangArithmetic(9223372036854775807, 1); got != 0 { t.Fatalf("overflow = %%d", got) }
}
`, gobackend.PackageName)))
}

func testV500DependentResultList(t *testing.T, analysis *Analysis) {
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
	carrier := function.Parameters[0].Type
	itemType := carrier.Result.Success.List.Element
	items := coreeval.Value{Type: carrier.Result.Success, List: []coreeval.Value{{Type: itemType, Record: []coreeval.Value{{Type: itemType.Record.Fields[0].Type, String: "one"}}}}}
	success := coreeval.Value{Type: carrier, Result: &coreeval.Outcome{OK: true, Value: items}}
	failureText := coreeval.Value{Type: carrier.Result.Failure, String: "offline"}
	failure := coreeval.Value{Type: carrier, Result: &coreeval.Outcome{Value: coreeval.Value{Type: carrier.Result.Success}, Failure: &failureText}}
	entry := coreir.SemanticIdentity{PackageID: string(identity.PackageID), Path: string(identity.Path)}
	for _, test := range []struct {
		value coreeval.Value
		want  int64
	}{{success, 1}, {failure, 0}} {
		outcome, evalErr := coreeval.EvaluateProgram(program, entry, []coreeval.Value{test.value})
		if evalErr != nil || !outcome.OK || outcome.Value.Int != test.want {
			t.Fatalf("CountSnapshots = %#v, %v", outcome, evalErr)
		}
	}
	generated, err := gobackend.Generate(program)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(generated), " := PipeLangSnapshot(") != 1 || strings.Count(string(generated), " := PipeLangConfirmSnapshot(") != 1 {
		t.Fatalf("generated dependent Result-list Go does not call each helper once:\n%s", generated)
	}
	compileAndRunGeneratedGoFiles(t, generated, []byte(fmt.Sprintf(`package %s

import "testing"

func TestGeneratedDependentResultListCarrierMatches(t *testing.T) {
	items := []PipeLangRecordTestPackageAppRootItem{{Value: "one"}}
	success := PipeLangResult[[]PipeLangRecordTestPackageAppRootItem, string]{OK: true, Value: items}
	if got := PipeLangCountSnapshots(success); got != 1 { t.Fatalf("success = %%d", got) }
	failure := PipeLangResult[[]PipeLangRecordTestPackageAppRootItem, string]{Error: "offline"}
	if got := PipeLangCountSnapshots(failure); got != 0 { t.Fatalf("failure = %%d", got) }
}
`, gobackend.PackageName)))
}

func testV500DependentFloat(t *testing.T, analysis *Analysis) {
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
	entry := coreir.SemanticIdentity{PackageID: string(identity.PackageID), Path: string(identity.Path)}
	for _, test := range []struct{ left, right, want float64 }{{8, 2, 2}, {1, 0, 0}} {
		outcome, evalErr := coreeval.EvaluateProgram(program, entry, coreFloatArguments(test.left, test.right))
		if evalErr != nil || !outcome.OK || outcome.Value.Float != test.want {
			t.Fatalf("ArithmeticFloat = %#v, %v", outcome, evalErr)
		}
	}
	generated, err := gobackend.Generate(program)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(generated), " := PipeLangDivide(") != 1 || strings.Count(string(generated), " := PipeLangDivideAfter(") != 1 {
		t.Fatalf("generated dependent float Go does not call each helper once:\n%s", generated)
	}
	compileAndRunGeneratedGoFiles(t, generated, []byte(fmt.Sprintf(`package %s

import "testing"

func TestGeneratedDependentFloatCarrierMatches(t *testing.T) {
	if got := PipeLangArithmeticFloat(8, 2); got != 2 { t.Fatalf("success = %%v", got) }
	if got := PipeLangArithmeticFloat(1, 0); got != 0 { t.Fatalf("division by zero = %%v", got) }
}
`, gobackend.PackageName)))
}

func testV500DependentOptional(t *testing.T, analysis *Analysis) {
	t.Helper()
	identity := semanticMethodNamed(t, analysis, "Resolve").Identity
	typed, err := LowerSemanticMethodToHIR(analysis, identity)
	if err != nil {
		t.Fatal(err)
	}
	program, err := LowerHIRToCore(typed)
	if err != nil {
		t.Fatal(err)
	}
	entry := coreir.SemanticIdentity{PackageID: string(identity.PackageID), Path: string(identity.Path)}
	text := coreir.Type{Kind: coreir.TypePrimitive, Primitive: coreir.PrimitiveString}
	outcome, evalErr := coreeval.EvaluateProgram(program, entry, []coreeval.Value{{Type: text, String: "kept"}})
	if evalErr != nil || !outcome.OK || outcome.Value.String != "kept" {
		t.Fatalf("Resolve = %#v, %v", outcome, evalErr)
	}
	generated, err := gobackend.Generate(program)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(generated), " := PipeLangRead(") != 1 || strings.Count(string(generated), " := PipeLangConfirm(") != 1 {
		t.Fatalf("generated dependent Optional Go does not call each helper once:\n%s", generated)
	}
	compileAndRunGeneratedGoFiles(t, generated, []byte(fmt.Sprintf(`package %s

import "testing"

func TestGeneratedDependentOptionalCarrierMatches(t *testing.T) {
	if got := PipeLangResolve("kept"); got != "kept" { t.Fatalf("success = %%q", got) }
}
`, gobackend.PackageName)))
}

func TestV500DependentSecondCarrierRejectsExcludedSource(t *testing.T) {
	tests := []struct{ name, source, message string }{
		{name: "prior contract", source: dependentSecondCarrierMatchSource, message: "the v0.49.0 Result contract"},
		{name: "split stage", source: `public Class Root { public Optional<string> First(string value) => some(value); public Optional<string> Second(string selected, string value) => First(selected); public string Resolve(string value) { Optional<string> firstCarrier = First(value); string first = match(firstCarrier){ some(item) => item, none => "" }; string gap = first; Optional<string> secondCarrier = Second(first, value); string second = match(secondCarrier){ some(item) => item, none => first }; return second; } }`, message: "four contiguous"},
		{name: "computed dependency", source: `public Class Root { public Optional<string> First(string value) => some(value); public Optional<string> Second(string selected, string value) => First(selected); public string Resolve(string value) { Optional<string> firstCarrier = First(value); string first = match(firstCarrier){ some(item) => item, none => "" }; Optional<string> secondCarrier = Second(trim(first), value); string second = match(secondCarrier){ some(item) => item, none => first }; return second; } }`, message: "first selected local"},
		{name: "other prior local", source: `public Class Root { public Optional<string> First(string value) => some(value); public Optional<string> Second(string selected, string value) => First(selected); public string Resolve(string value) { string other = ""; Optional<string> firstCarrier = First(value); string first = match(firstCarrier){ some(item) => item, none => "" }; Optional<string> secondCarrier = Second(other, value); string second = match(secondCarrier){ some(item) => item, none => first }; return second; } }`, message: "first selected local"},
		{name: "reordered caller", source: `public Class Root { public Result<string, string> First(string left, string right) => left == "" ? err<string, string>(right) : ok<string, string>(left); public Result<string, string> Second(string selected, string left, string right) => First(left, right); public string Resolve(string left, string right) { Result<string, string> firstCarrier = First(left, right); string first = match(firstCarrier){ ok(item) => item, err(problem) => problem }; Result<string, string> secondCarrier = Second(first, right, left); string second = match(secondCarrier){ ok(item) => item, err(problem) => problem }; return second; } }`, message: "declaration order"},
		{name: "repeated caller", source: `public Class Root { public Result<string, string> First(string left, string right) => left == "" ? err<string, string>(right) : ok<string, string>(left); public Result<string, string> Second(string selected, string left, string right) => First(left, right); public string Resolve(string left, string right) { Result<string, string> firstCarrier = First(left, right); string first = match(firstCarrier){ ok(item) => item, err(problem) => problem }; Result<string, string> secondCarrier = Second(first, left, left); string second = match(secondCarrier){ ok(item) => item, err(problem) => problem }; return second; } }`, message: "declaration order"},
		{name: "third match", source: `public Class Root { public Optional<string> Read(string value) => some(value); public string Resolve(string value) { Optional<string> oneCarrier = Read(value); string one = match(oneCarrier){ some(item) => item, none => "" }; Optional<string> twoCarrier = Read(value); string two = match(twoCarrier){ some(item) => item, none => one }; Optional<string> threeCarrier = Read(value); string three = match(threeCarrier){ some(item) => item, none => two }; return three; } }`, message: "exactly two matches"},
		{name: "unreferenced widened helper", source: `public Class Root { public Optional<string> First(string value) => some(value); public Optional<string> Extra(string selected, string value) => First(selected); public string Keep(string value) => value; }`, message: "primitive Optional is admitted only"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			contract := PipeLangLanguageContractV500
			if test.name == "prior contract" {
				contract = PipeLangLanguageContractV490
			}
			input := semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", test.name+".pipe", test.source)}, nil)
			input.LanguageContract = contract
			analysis := AnalyzeSemanticModuleSet(input)
			if len(analysis.Diagnostics) == 0 || !strings.Contains(analysis.Diagnostics[0].Message, test.message) || !analysis.Diagnostics[0].Primary.IsValid() {
				t.Fatalf("diagnostics = %#v", analysis.Diagnostics)
			}
		})
	}
}

func TestV500DependentSecondCarrierCoreRejectsMalformedDependency(t *testing.T) {
	input := semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", "dependent-second-core.pipe", dependentSecondCarrierMatchSource)}, nil)
	input.LanguageContract = PipeLangLanguageContractV500
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
	function := coreFunctionNamed(t, program, "Combine")
	pairs := coreCarrierMatchPairs(&function.Body)
	if len(pairs) != 2 || pairs[1].carrier.Initializer.Call == nil {
		t.Fatal("missing dependent pair")
	}
	wrong := 0
	pairs[1].carrier.Initializer.Call.Arguments[0].Parameter = &wrong
	if err := coreir.ValidateProgram(program); err == nil || !strings.Contains(err.Error(), "first selected local") {
		t.Fatalf("malformed dependent Core error = %v", err)
	}
	if _, err := gobackend.Generate(program); err == nil {
		t.Fatal("Go backend accepted malformed dependent Core")
	}
}

func TestV500PreservesV490IndependentPairs(t *testing.T) {
	input := semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", "v500-v490-compat.pipe", boundedTwoCarrierMatchesSource)}, nil)
	input.LanguageContract = PipeLangLanguageContractV500
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
	combine := coreFunctionNamed(t, program, "Combine")
	if program.LanguageContract != coreir.LanguageContractV500 || len(coreCarrierMatchPairs(&combine.Body)) != 2 {
		t.Fatal("v0.50.0 changed inherited v0.49.0 pairs")
	}
}
