package pipelang

import (
	"fmt"
	"strings"
	"testing"

	"dockpipe/src/lib/pipelang/coreeval"
	"dockpipe/src/lib/pipelang/coreir"
	"dockpipe/src/lib/pipelang/gobackend"
)

const dependentCarrierChainSource = `public Class Root {
	public Result<string, string> First(string first, string second) => first == "" ? err<string, string>("first missing") : ok<string, string>(first);
	public Result<string, string> Second(string selected, string first, string second) => second == "" ? err<string, string>(selected) : ok<string, string>(second);
	public Result<string, string> Third(string selected, string first, string second) => selected == "" ? err<string, string>(first) : ok<string, string>(selected);
	public Result<string, string> Fourth(string selected, string first, string second) => selected == "" ? err<string, string>(second) : ok<string, string>(selected);
	public string Combine(string first, string second) {
		string prefix = "";
		Result<string, string> firstCarrier = First(first, second);
		string left = match(firstCarrier){ ok(value) => value, err(problem) => problem };
		Result<string, string> secondCarrier = Second(left, first, second);
		string right = match(secondCarrier){ ok(value) => value, err(problem) => problem };
		Result<string, string> thirdCarrier = Third(right, first, second);
		string third = match(thirdCarrier){ ok(value) => value, err(problem) => problem };
		Result<string, string> fourthCarrier = Fourth(third, first, second);
		string fourth = match(fourthCarrier){ ok(value) => value, err(problem) => problem };
		string joined = prefix + left + "/" + right + "/" + third + "/" + fourth;
		return joined;
	}
	public Result<int, ArithmeticError> Add(int left, int right) => left + right;
	public Result<int, ArithmeticError> Multiply(int left, int right) => left * right;
	public Result<int, ArithmeticError> MultiplyAfter(int selected, int left, int right) => Multiply(selected, right);
	public Result<int, ArithmeticError> AddAfter(int selected, int left, int right) => Add(selected, left);
	public int Arithmetic(int left, int right) {
		Result<int, ArithmeticError> firstCarrier = Add(left, right);
		int first = match(firstCarrier){ ok(value) => value, err(problem) => 0 };
		Result<int, ArithmeticError> secondCarrier = MultiplyAfter(first, left, right);
		int second = match(secondCarrier){ ok(value) => value, err(problem) => 0 };
		Result<int, ArithmeticError> thirdCarrier = AddAfter(second, left, right);
		int third = match(thirdCarrier){ ok(value) => value, err(problem) => 0 };
		return third;
	}
}`

func TestV510DependentCarrierChainPipeline(t *testing.T) {
	input := semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", "dependent-carrier-chain.pipe", dependentCarrierChainSource)}, nil)
	input.LanguageContract = PipeLangLanguageContractV510
	analysis := AnalyzeSemanticModuleSet(input)
	if err := analysis.Error(); err != nil {
		t.Fatal(err)
	}
	projection, err := BuildSemanticProjection(analysis)
	if err != nil {
		t.Fatal(err)
	}
	if projection.LanguageContract != PipeLangLanguageContractV510 || projection.Schema != PipeLangSemanticProjectionVersion || projection.CompilerContract != PipeLangCompilerContract {
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
	if typed.LanguageContract != coreir.LanguageContractV510 || program.LanguageContract != coreir.LanguageContractV510 || len(pairs) != 4 {
		t.Fatalf("dependent contracts or pair count drifted: HIR=%q Core=%q pairs=%d", typed.LanguageContract, program.LanguageContract, len(pairs))
	}
	for position := 1; position < len(pairs); position++ {
		call := pairs[position].carrier.Initializer.Call
		previousSelected := pairs[position-1].carrier.Return.ImmutableLocal
		if call == nil || previousSelected == nil || len(call.Arguments) != 3 || call.Arguments[0].Parameter == nil || *call.Arguments[0].Parameter != previousSelected.Position || call.Arguments[1].Parameter == nil || *call.Arguments[1].Parameter != 0 || call.Arguments[2].Parameter == nil || *call.Arguments[2].Parameter != 1 {
			t.Fatalf("dependent stage %d call arguments = %#v", position+1, call)
		}
	}

	entry := coreir.SemanticIdentity{PackageID: string(identity.PackageID), Path: string(identity.Path)}
	text := coreir.Type{Kind: coreir.TypePrimitive, Primitive: coreir.PrimitiveString}
	for _, test := range []struct{ first, second, want string }{
		{first: "alpha", second: "beta", want: "alpha/beta/beta/beta"},
		{first: "", second: "beta", want: "first missing/beta/beta/beta"},
		{first: "alpha", second: "", want: "alpha/alpha/alpha/alpha"},
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
		t.Fatalf("dependent chain generated Go is not deterministic: %v", err)
	}
	for _, helper := range []string{"PipeLangFirst", "PipeLangSecond", "PipeLangThird", "PipeLangFourth"} {
		if strings.Count(string(generated), " := "+helper+"(") != 1 {
			t.Fatalf("generated dependent chain does not call %s exactly once:\n%s", helper, generated)
		}
	}
	compileAndRunGeneratedGoFiles(t, generated, []byte(fmt.Sprintf(`package %s

import "testing"

func TestGeneratedDependentCarrierChain(t *testing.T) {
	if got := PipeLangCombine("alpha", "beta"); got != "alpha/beta/beta/beta" { t.Fatalf("success = %%q", got) }
	if got := PipeLangCombine("alpha", ""); got != "alpha/alpha/alpha/alpha" { t.Fatalf("fallback = %%q", got) }
}
`, gobackend.PackageName)))

	testV510DependentArithmeticChain(t, analysis)
}

func testV510DependentArithmeticChain(t *testing.T, analysis *Analysis) {
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
	outcome, evalErr := coreeval.EvaluateProgram(program, entry, coreIntArguments(6, 7))
	if evalErr != nil || !outcome.OK || outcome.Value.Int != 97 {
		t.Fatalf("Arithmetic = %#v, %v", outcome, evalErr)
	}
	generated, err := gobackend.Generate(program)
	if err != nil {
		t.Fatal(err)
	}
	for _, helper := range []string{"PipeLangAdd", "PipeLangMultiplyAfter", "PipeLangAddAfter"} {
		if strings.Count(string(generated), " := "+helper+"(") != 1 {
			t.Fatalf("generated arithmetic chain does not call %s exactly once:\n%s", helper, generated)
		}
	}
	compileAndRunGeneratedGoFiles(t, generated, []byte(fmt.Sprintf(`package %s

import "testing"

func TestGeneratedDependentArithmeticChain(t *testing.T) {
	if got := PipeLangArithmetic(6, 7); got != 97 { t.Fatalf("success = %%d", got) }
}
`, gobackend.PackageName)))
}

func TestV510DependentCarrierChainRejectsExcludedSource(t *testing.T) {
	tests := []struct {
		name, source, message string
		contract              LanguageContract
	}{
		{name: "prior contract", source: dependentCarrierChainSource, message: "exactly two matches", contract: PipeLangLanguageContractV500},
		{name: "split chain", source: `public Class Root { public Optional<string> First(string value) => some(value); public Optional<string> Next(string selected, string value) => First(selected); public string Resolve(string value) { Optional<string> firstCarrier = First(value); string first = match(firstCarrier){ some(item) => item, none => "" }; Optional<string> secondCarrier = Next(first, value); string second = match(secondCarrier){ some(item) => item, none => first }; string gap = second; Optional<string> thirdCarrier = Next(second, value); string third = match(thirdCarrier){ some(item) => item, none => second }; return third; } }`, message: "contiguous", contract: PipeLangLanguageContractV510},
		{name: "non-immediate dependency", source: `public Class Root { public Optional<string> First(string value) => some(value); public Optional<string> Next(string selected, string value) => First(selected); public string Resolve(string value) { Optional<string> firstCarrier = First(value); string first = match(firstCarrier){ some(item) => item, none => "" }; Optional<string> secondCarrier = Next(first, value); string second = match(secondCarrier){ some(item) => item, none => first }; Optional<string> thirdCarrier = Next(first, value); string third = match(thirdCarrier){ some(item) => item, none => second }; return third; } }`, message: "immediately preceding selected local", contract: PipeLangLanguageContractV510},
		{name: "independent third helper", source: `public Class Root { public Optional<string> Read(string value) => some(value); public Optional<string> Next(string selected, string value) => Read(selected); public string Resolve(string value) { Optional<string> firstCarrier = Read(value); string first = match(firstCarrier){ some(item) => item, none => "" }; Optional<string> secondCarrier = Next(first, value); string second = match(secondCarrier){ some(item) => item, none => first }; Optional<string> thirdCarrier = Read(value); string third = match(thirdCarrier){ some(item) => item, none => second }; return third; } }`, message: "immediately preceding selected local", contract: PipeLangLanguageContractV510},
		{name: "reordered third-stage caller", source: strings.Replace(dependentCarrierChainSource, "Third(right, first, second)", "Third(right, second, first)", 1), message: "declaration order", contract: PipeLangLanguageContractV510},
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

func TestV510DependentCarrierChainCoreRejectsMalformedDependency(t *testing.T) {
	input := semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", "dependent-chain-core.pipe", dependentCarrierChainSource)}, nil)
	input.LanguageContract = PipeLangLanguageContractV510
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
	if len(pairs) != 4 || pairs[3].carrier.Initializer.Call == nil {
		t.Fatal("missing dependent chain")
	}
	firstSelected := pairs[0].carrier.Return.ImmutableLocal.Position
	pairs[3].carrier.Initializer.Call.Arguments[0].Parameter = &firstSelected
	if err := coreir.ValidateProgram(program); err == nil || !strings.Contains(err.Error(), "immediately preceding selected local") {
		t.Fatalf("malformed dependent Core error = %v", err)
	}
	if _, err := gobackend.Generate(program); err == nil {
		t.Fatal("Go backend accepted malformed dependent chain Core")
	}
}

func TestV510PreservesV500DependentSecondCarrier(t *testing.T) {
	input := semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", "v510-v500-compat.pipe", dependentSecondCarrierMatchSource)}, nil)
	input.LanguageContract = PipeLangLanguageContractV510
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
	if program.LanguageContract != coreir.LanguageContractV510 || len(coreCarrierMatchPairs(&combine.Body)) != 2 {
		t.Fatal("v0.51.0 changed inherited v0.50.0 pair")
	}
}
