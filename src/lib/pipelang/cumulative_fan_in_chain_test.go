package pipelang

import (
	"fmt"
	"strings"
	"testing"

	"dockpipe/src/lib/pipelang/coreeval"
	"dockpipe/src/lib/pipelang/coreir"
	"dockpipe/src/lib/pipelang/gobackend"
)

const cumulativeFanInChainSource = `public Class Root {
	public Result<string, string> First(string first, string second) => first == "" ? err<string, string>("first missing") : ok<string, string>(first);
	public Result<string, string> Second(string firstSelected, string first, string second) => second == "" ? err<string, string>(firstSelected) : ok<string, string>(second);
	public Result<string, string> Third(string firstSelected, string secondSelected, string first, string second) => firstSelected == "" ? err<string, string>(firstSelected) : ok<string, string>(secondSelected);
	public Result<string, string> Fourth(string firstSelected, string secondSelected, string thirdSelected, string first, string second) => firstSelected == "" || secondSelected == "" ? err<string, string>(thirdSelected) : ok<string, string>(firstSelected);
	public string Combine(string first, string second) {
		Result<string, string> firstCarrier = First(first, second);
		string left = match(firstCarrier){ ok(value) => value, err(problem) => problem };
		Result<string, string> secondCarrier = Second(left, first, second);
		string right = match(secondCarrier){ ok(value) => value, err(problem) => problem };
		Result<string, string> thirdCarrier = Third(left, right, first, second);
		string third = match(thirdCarrier){ ok(value) => value, err(problem) => problem };
		Result<string, string> fourthCarrier = Fourth(left, right, third, first, second);
		string fourth = match(fourthCarrier){ ok(value) => value, err(problem) => problem };
		return fourth;
	}
}`

func TestV520CumulativeFanInChainPipeline(t *testing.T) {
	input := semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", "cumulative-fan-in-chain.pipe", cumulativeFanInChainSource)}, nil)
	input.LanguageContract = PipeLangLanguageContractV520
	analysis := AnalyzeSemanticModuleSet(input)
	if err := analysis.Error(); err != nil {
		t.Fatal(err)
	}
	projection, err := BuildSemanticProjection(analysis)
	if err != nil {
		t.Fatal(err)
	}
	if projection.LanguageContract != PipeLangLanguageContractV520 || projection.Schema != PipeLangSemanticProjectionVersion || projection.CompilerContract != PipeLangCompilerContract {
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
	if typed.LanguageContract != coreir.LanguageContractV520 || program.LanguageContract != coreir.LanguageContractV520 || len(pairs) != 4 {
		t.Fatalf("cumulative contracts or pair count drifted: HIR=%q Core=%q pairs=%d", typed.LanguageContract, program.LanguageContract, len(pairs))
	}
	for position := 1; position < len(pairs); position++ {
		call := pairs[position].carrier.Initializer.Call
		if call == nil || len(call.Arguments) != position+2 {
			t.Fatalf("cumulative stage %d call arguments = %#v", position+1, call)
		}
		for prior := 0; prior < position; prior++ {
			selected := pairs[prior].carrier.Return.ImmutableLocal.Position
			if call.Arguments[prior].Parameter == nil || *call.Arguments[prior].Parameter != selected {
				t.Fatalf("cumulative stage %d prior argument %d = %#v, want binding %d", position+1, prior+1, call.Arguments[prior], selected)
			}
		}
		for parameter := 0; parameter < 2; parameter++ {
			argument := call.Arguments[position+parameter]
			if argument.Parameter == nil || *argument.Parameter != parameter {
				t.Fatalf("cumulative stage %d caller argument %d = %#v", position+1, parameter+1, argument)
			}
		}
	}

	entry := coreir.SemanticIdentity{PackageID: string(identity.PackageID), Path: string(identity.Path)}
	text := coreir.Type{Kind: coreir.TypePrimitive, Primitive: coreir.PrimitiveString}
	for _, test := range []struct{ first, second, want string }{
		{first: "alpha", second: "beta", want: "alpha"},
		{first: "", second: "beta", want: "first missing"},
		{first: "alpha", second: "", want: "alpha"},
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
		t.Fatalf("cumulative generated Go is not deterministic: %v", err)
	}
	for _, helper := range []string{"PipeLangFirst", "PipeLangSecond", "PipeLangThird", "PipeLangFourth"} {
		if strings.Count(string(generated), " := "+helper+"(") != 1 {
			t.Fatalf("generated cumulative chain does not call %s exactly once:\n%s", helper, generated)
		}
	}
	compileAndRunGeneratedGoFiles(t, generated, []byte(fmt.Sprintf(`package %s

import "testing"

func TestGeneratedCumulativeFanInChain(t *testing.T) {
	if got := PipeLangCombine("alpha", "beta"); got != "alpha" { t.Fatalf("success = %%q", got) }
	if got := PipeLangCombine("", "beta"); got != "first missing" { t.Fatalf("fallback = %%q", got) }
}
`, gobackend.PackageName)))
}

func TestV520CumulativeFanInChainRejectsExcludedSource(t *testing.T) {
	mixedImmediateStage := strings.Replace(cumulativeFanInChainSource, "Fourth(string firstSelected, string secondSelected, string thirdSelected, string first, string second)", "Fourth(string thirdSelected, string first, string second)", 1)
	mixedImmediateStage = strings.Replace(mixedImmediateStage, "firstSelected == \"\" || secondSelected == \"\" ? err<string, string>(thirdSelected) : ok<string, string>(firstSelected)", "thirdSelected == \"\" ? err<string, string>(thirdSelected) : ok<string, string>(thirdSelected)", 1)
	mixedImmediateStage = strings.Replace(mixedImmediateStage, "Fourth(left, right, third, first, second)", "Fourth(third, first, second)", 1)
	tests := []struct {
		name, source, message string
		contract              LanguageContract
	}{
		{name: "prior contract", source: cumulativeFanInChainSource, message: "immediately preceding selected local", contract: PipeLangLanguageContractV510},
		{name: "reordered fan in", source: strings.Replace(cumulativeFanInChainSource, "Third(left, right, first, second)", "Third(right, left, first, second)", 1), message: "chain order", contract: PipeLangLanguageContractV520},
		{name: "partial fan in", source: strings.Replace(cumulativeFanInChainSource, "Third(left, right, first, second)", "Third(right, first, second)", 1), message: "every prior selected local", contract: PipeLangLanguageContractV520},
		{name: "mixed immediate stage", source: mixedImmediateStage, message: "every prior selected local", contract: PipeLangLanguageContractV520},
		{name: "split cumulative chain", source: strings.Replace(cumulativeFanInChainSource, "Result<string, string> fourthCarrier", "string gap = third; Result<string, string> fourthCarrier", 1), message: "contiguous", contract: PipeLangLanguageContractV520},
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

func TestV520CumulativeFanInCoreRejectsMalformedArgumentOrder(t *testing.T) {
	input := semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", "cumulative-fan-in-core.pipe", cumulativeFanInChainSource)}, nil)
	input.LanguageContract = PipeLangLanguageContractV520
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
		t.Fatal("missing cumulative fan-in chain")
	}
	call := pairs[3].carrier.Initializer.Call
	call.Arguments[0], call.Arguments[1] = call.Arguments[1], call.Arguments[0]
	if err := coreir.ValidateProgram(program); err == nil || !strings.Contains(err.Error(), "chain order") {
		t.Fatalf("malformed cumulative Core error = %v", err)
	}
	if _, err := gobackend.Generate(program); err == nil {
		t.Fatal("Go backend accepted malformed cumulative fan-in Core")
	}
}

func TestV520PreservesV510ImmediateOnlyChain(t *testing.T) {
	input := semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", "v520-v510-compat.pipe", dependentCarrierChainSource)}, nil)
	input.LanguageContract = PipeLangLanguageContractV520
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
	if program.LanguageContract != coreir.LanguageContractV520 || len(coreCarrierMatchPairs(&combine.Body)) != 4 {
		t.Fatal("v0.52.0 changed inherited v0.51.0 chain")
	}
}
