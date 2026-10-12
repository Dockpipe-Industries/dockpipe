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

const multiParameterHelperPropagationSource = `public Class Root {
	public Result<string, string> Validate(string raw, string suffix) => raw == "" ? err<string, string>("missing") : ok<string, string>(raw + suffix);
	public Result<string, string> Normalize(string raw, string suffix) {
		Result<string, string> carrier = Validate(raw, suffix);
		string value = propagate(carrier);
		string cleaned = trim(value);
		return ok<string, string>(cleaned);
	}
}`

func TestV530MultiParameterHelperPropagationPipeline(t *testing.T) {
	input := semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", "multi-parameter-helper-propagation.pipe", multiParameterHelperPropagationSource)}, nil)
	input.LanguageContract = PipeLangLanguageContractV530
	analysis := AnalyzeSemanticModuleSet(input)
	if err := analysis.Error(); err != nil {
		t.Fatal(err)
	}
	projection, err := BuildSemanticProjection(analysis)
	if err != nil {
		t.Fatal(err)
	}
	if projection.LanguageContract != PipeLangLanguageContractV530 || projection.Schema != PipeLangSemanticProjectionVersion || projection.CompilerContract != PipeLangCompilerContract {
		t.Fatalf("projection contract = %#v", projection)
	}

	identity := semanticMethodNamed(t, analysis, "Normalize").Identity
	typed, err := LowerSemanticMethodToHIR(analysis, identity)
	if err != nil {
		t.Fatal(err)
	}
	function := hirFunctionNamed(t, typed, "Normalize")
	first := function.Body.ImmutableLocal
	if typed.LanguageContract != coreir.LanguageContractV530 || first == nil || first.Initializer.Kind != hir.ExprCall || first.Initializer.Call == nil || len(first.Initializer.Call.Arguments) != 2 || first.Return == nil || first.Return.Kind != hir.ExprImmutableLocal {
		t.Fatalf("Normalize HIR = %#v", function.Body)
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
	coreFunction := coreFunctionNamed(t, program, "Normalize")
	coreFirst := coreFunction.Body.ImmutableLocal
	if program.LanguageContract != coreir.LanguageContractV530 || coreFirst == nil || coreFirst.Initializer == nil || coreFirst.Initializer.Call == nil || len(coreFirst.Initializer.Call.Arguments) != 2 || coreFirst.Return == nil || coreFirst.Return.ImmutableLocal == nil {
		t.Fatalf("Normalize Core = %#v", coreFunction.Body)
	}
	for position, argument := range coreFirst.Initializer.Call.Arguments {
		if argument == nil || argument.Parameter == nil || *argument.Parameter != position {
			t.Fatalf("Core helper argument %d = %#v", position, argument)
		}
	}
	coreSecond := coreFirst.Return.ImmutableLocal
	if coreSecond.Initializer == nil || coreSecond.Initializer.Propagate == nil || coreSecond.Initializer.Propagate.Value == nil || coreSecond.Initializer.Propagate.Value.Parameter == nil || *coreSecond.Initializer.Propagate.Value.Parameter != coreFirst.Position {
		t.Fatalf("propagation Core = %#v", coreFunction.Body)
	}

	entry := coreir.SemanticIdentity{PackageID: string(identity.PackageID), Path: string(identity.Path)}
	textType := coreFunction.Parameters[0].Type
	for _, test := range []struct {
		raw, suffix string
		ok          bool
		want        string
	}{
		{raw: "  ready", suffix: "!  ", ok: true, want: "ready!"},
		{raw: "", suffix: "ignored", ok: false, want: "missing"},
	} {
		outcome, evalErr := coreeval.EvaluateProgram(program, entry, []coreeval.Value{{Type: textType, String: test.raw}, {Type: textType, String: test.suffix}})
		if evalErr != nil || outcome.OK != test.ok {
			t.Fatalf("Normalize(%q, %q) = %#v, %v", test.raw, test.suffix, outcome, evalErr)
		}
		if test.ok && outcome.Value.String != test.want {
			t.Fatalf("success = %#v, want %q", outcome, test.want)
		}
		if !test.ok && (outcome.Failure == nil || outcome.Failure.String != test.want) {
			t.Fatalf("failure = %#v, want %q", outcome, test.want)
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
	for _, fragment := range []string{"p2 := PipeLangValidate(p0, p1)", "if !p2.OK", "p3 := p2.Value", "p4 := pipelangTrimText(p3)"} {
		if !strings.Contains(string(generated), fragment) {
			t.Fatalf("generated Go lacks multi-parameter propagation %q:\n%s", fragment, generated)
		}
	}
	if strings.Count(string(generated), "p2 := PipeLangValidate(p0, p1)") != 1 {
		t.Fatalf("generated Go does not call the helper exactly once:\n%s", generated)
	}
	compileAndRunGeneratedGoFiles(t, generated, []byte(fmt.Sprintf(`package %s

import "testing"

func TestGeneratedMultiParameterPropagation(t *testing.T) {
	success := PipeLangNormalize("  ready", "!  ")
	if !success.OK || success.Value != "ready!" { t.Fatalf("success = %%#v", success) }
	failure := PipeLangNormalize("", "ignored")
	if failure.OK || failure.Error != "missing" { t.Fatalf("failure = %%#v", failure) }
}
`, gobackend.PackageName)))
}

func TestV530MultiParameterHelperPropagationPreservesCarrierMatrix(t *testing.T) {
	cases := []struct {
		name   string
		source string
	}{
		{
			name: "optional primitive",
			source: `public Class Root {
					public Optional<string> Validate(string raw, string suffix) => raw == "" ? none<string>() : some(raw + suffix);
					public Optional<string> Resolve(string raw, string suffix) { Optional<string> carrier = Validate(raw, suffix); string selected = propagate(carrier); return some(selected); }
				}`,
		},
		{
			name: "optional record",
			source: `public Record Row { public string Id; }
				public Class Root {
					public Optional<Row> Find(List<Row> rows, string id) => find_by(rows, Row.Id, id);
					public Optional<Row> Resolve(List<Row> rows, string id) { Optional<Row> carrier = Find(rows, id); Row row = propagate(carrier); return some(row); }
				}`,
		},
		{
			name: "snapshot result",
			source: `public Record Row { public string Id; }
				public Class Root {
					public Result<List<Row>, string> Validate(List<Row> rows, string id) => id == "" ? err<List<Row>, string>("missing") : ok<List<Row>, string>(rows);
					public Result<List<Row>, string> Resolve(List<Row> rows, string id) { Result<List<Row>, string> carrier = Validate(rows, id); List<Row> selected = propagate(carrier); return ok<List<Row>, string>(selected); }
				}`,
		},
		{
			name:   "inherited one parameter",
			source: `public Class Root { public Optional<string> Validate(string raw) => some(raw); public Optional<string> Resolve(string raw) { Optional<string> carrier = Validate(raw); string selected = propagate(carrier); return some(selected); } }`,
		},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			input := semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", test.name+".pipe", test.source)}, nil)
			input.LanguageContract = PipeLangLanguageContractV530
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
		})
	}
}

func TestV530MultiParameterHelperPropagationRejectsExcludedSource(t *testing.T) {
	valid := multiParameterHelperPropagationSource
	cases := []struct {
		name     string
		contract LanguageContract
		source   string
		message  string
	}{
		{name: "prior contract", contract: PipeLangLanguageContractV520, source: valid, message: "sole direct parameter"},
		{name: "reordered", contract: PipeLangLanguageContractV530, source: strings.Replace(valid, "Validate(raw, suffix)", "Validate(suffix, raw)", 1), message: "declaration order"},
		{name: "repeated", contract: PipeLangLanguageContractV530, source: strings.Replace(valid, "Validate(raw, suffix)", "Validate(raw, raw)", 1), message: "declaration order"},
		{name: "omitted", contract: PipeLangLanguageContractV530, source: strings.Replace(valid, "Validate(raw, suffix)", "Validate(raw)", 1), message: "declaration order"},
		{name: "extra", contract: PipeLangLanguageContractV530, source: strings.Replace(valid, "Validate(raw, suffix)", "Validate(raw, suffix, raw)", 1), message: "declaration order"},
		{name: "computed", contract: PipeLangLanguageContractV530, source: strings.Replace(valid, "Validate(raw, suffix)", "Validate(trim(raw), suffix)", 1), message: "declaration order"},
		{name: "zero parameters", contract: PipeLangLanguageContractV530, source: `public Class Root { public Result<string, string> Validate() => ok<string, string>(""); public Result<string, string> Resolve() { Result<string, string> carrier = Validate(); string value = propagate(carrier); return ok<string, string>(value); } }`, message: "bounded snapshot/text Result methods"},
		{name: "carrier not first", contract: PipeLangLanguageContractV530, source: strings.Replace(valid, "Result<string, string> carrier = Validate(raw, suffix);", "string prefix = \"\"; Result<string, string> carrier = Validate(raw, suffix);", 1), message: "one helper-call carrier local"},
		{name: "intervening local", contract: PipeLangLanguageContractV530, source: strings.Replace(valid, "string value = propagate(carrier);", "Result<string, string> copy = carrier; string value = propagate(copy);", 1), message: "second immutable-local initializer"},
		{name: "wrong carrier", contract: PipeLangLanguageContractV530, source: strings.Replace(valid, "propagate(carrier)", "propagate(raw)", 1), message: "immediately preceding"},
		{name: "additional propagation", contract: PipeLangLanguageContractV530, source: strings.Replace(valid, "ok<string, string>(cleaned)", "ok<string, string>(propagate(carrier))", 1), message: "one helper-call carrier local"},
		{name: "arithmetic result", contract: PipeLangLanguageContractV530, source: `public Class Root { public Result<int, ArithmeticError> Add(int left, int right) => left + right; public Result<int, ArithmeticError> Resolve(int left, int right) { Result<int, ArithmeticError> carrier = Add(left, right); int value = propagate(carrier); return value + 0; } }`, message: "admitted Optional or bounded Result"},
	}
	for _, test := range cases {
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

func TestV530MultiParameterHelperPropagationCoreRejectsMalformedArguments(t *testing.T) {
	input := semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", "multi-parameter-helper-propagation-core.pipe", multiParameterHelperPropagationSource)}, nil)
	input.LanguageContract = PipeLangLanguageContractV530
	analysis := AnalyzeSemanticModuleSet(input)
	if err := analysis.Error(); err != nil {
		t.Fatal(err)
	}
	typed, err := LowerSemanticMethodToHIR(analysis, semanticMethodNamed(t, analysis, "Normalize").Identity)
	if err != nil {
		t.Fatal(err)
	}
	program, err := LowerHIRToCore(typed)
	if err != nil {
		t.Fatal(err)
	}

	prior := program
	prior.LanguageContract = coreir.LanguageContractV520
	if err := coreir.ValidateProgram(prior); err == nil || !strings.Contains(err.Error(), "sole direct parameter") {
		t.Fatalf("prior contract error = %v", err)
	}

	malformed := program
	malformed.Functions = append([]coreir.Function(nil), program.Functions...)
	normalize := coreFunctionNamed(t, malformed, "Normalize")
	first := *normalize.Body.ImmutableLocal
	initializer := *first.Initializer
	call := *initializer.Call
	call.Arguments = append([]*coreir.Expr(nil), call.Arguments...)
	call.Arguments[0], call.Arguments[1] = call.Arguments[1], call.Arguments[0]
	initializer.Call = &call
	first.Initializer = &initializer
	normalize.Body.ImmutableLocal = &first
	for index := range malformed.Functions {
		if malformed.Functions[index].Name == "Normalize" {
			malformed.Functions[index] = normalize
		}
	}
	if err := coreir.ValidateProgram(malformed); err == nil || !strings.Contains(err.Error(), "declaration order") {
		t.Fatalf("malformed Core error = %v", err)
	}
	if _, err := gobackend.Generate(malformed); err == nil {
		t.Fatal("Go backend accepted malformed multi-parameter propagation Core")
	}
}

func TestV530PreservesV520CumulativeFanIn(t *testing.T) {
	input := semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", "v530-cumulative-fan-in.pipe", cumulativeFanInChainSource)}, nil)
	input.LanguageContract = PipeLangLanguageContractV530
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
	if program.LanguageContract != coreir.LanguageContractV530 || len(coreCarrierMatchPairs(&combine.Body)) != 4 {
		t.Fatal("v0.53.0 changed inherited cumulative fan-in matching")
	}
}

func TestV530PreservesV510ImmediateOnlyChain(t *testing.T) {
	input := semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", "v530-immediate-chain.pipe", dependentCarrierChainSource)}, nil)
	input.LanguageContract = PipeLangLanguageContractV530
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
	if program.LanguageContract != coreir.LanguageContractV530 || len(coreCarrierMatchPairs(&combine.Body)) != 4 {
		t.Fatal("v0.53.0 changed inherited immediate-only matching")
	}
}
