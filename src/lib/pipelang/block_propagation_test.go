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

func TestV410BlockScopedOptionalPropagationPipeline(t *testing.T) {
	source := `public Class Root {
		public Optional<string> Normalize(Optional<string> carrier) {
			string value = propagate(carrier);
			string cleaned = trim(value);
			return some(cleaned);
		}
	}`
	input := semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", "block-propagation.pipe", source)}, nil)
	input.LanguageContract = PipeLangLanguageContractV410
	analysis := AnalyzeSemanticModuleSet(input)
	if err := analysis.Error(); err != nil {
		t.Fatal(err)
	}
	projection, err := BuildSemanticProjection(analysis)
	if err != nil {
		t.Fatal(err)
	}
	if projection.LanguageContract != PipeLangLanguageContractV410 {
		t.Fatalf("projection language contract = %q", projection.LanguageContract)
	}

	identity := semanticMethodNamed(t, analysis, "Normalize").Identity
	typed, err := LowerSemanticMethodToHIR(analysis, identity)
	if err != nil {
		t.Fatal(err)
	}
	function := hirFunctionNamed(t, typed, "Normalize")
	first := function.Body.ImmutableLocal
	if typed.LanguageContract != coreir.LanguageContractV410 || function.Body.Kind != hir.ExprImmutableLocal || first == nil || first.Initializer.Kind != hir.ExprPropagate || first.Return.Kind != hir.ExprImmutableLocal || first.Return.ImmutableLocal.Initializer.Kind != hir.ExprTextTrim {
		t.Fatalf("Normalize HIR = %#v", function.Body)
	}
	core, err := LowerHIRToCore(typed)
	if err != nil {
		t.Fatal(err)
	}
	coreFunction := coreFunctionNamed(t, core, "Normalize")
	coreFirst := coreFunction.Body.ImmutableLocal
	if coreFunction.Body.Kind != coreir.ExprImmutableLocal || coreFirst == nil || coreFirst.Position != 1 || coreFirst.Initializer.Kind != coreir.ExprPropagate || coreFirst.Return.Kind != coreir.ExprImmutableLocal {
		t.Fatalf("Normalize Core = %#v", coreFunction.Body)
	}
	optional := coreFunction.Parameters[0].Type
	text := optional.Optional.Value
	present := coreeval.Value{Type: optional, Optional: &coreeval.OptionalValue{Present: true, Value: &coreeval.Value{Type: text, String: "  worker  "}}}
	outcome, err := coreeval.EvaluateProgram(core, coreir.SemanticIdentity{PackageID: string(identity.PackageID), Path: string(identity.Path)}, []coreeval.Value{present})
	if err != nil || !outcome.OK || outcome.Value.Optional == nil || !outcome.Value.Optional.Present || outcome.Value.Optional.Value.String != "worker" {
		t.Fatalf("present = %#v, %v", outcome, err)
	}
	absent := coreeval.Value{Type: optional, Optional: &coreeval.OptionalValue{}}
	outcome, err = coreeval.EvaluateProgram(core, coreir.SemanticIdentity{PackageID: string(identity.PackageID), Path: string(identity.Path)}, []coreeval.Value{absent})
	if err != nil || !outcome.OK || outcome.Value.Optional == nil || outcome.Value.Optional.Present {
		t.Fatalf("absent = %#v, %v", outcome, err)
	}

	generated, err := gobackend.Generate(core)
	if err != nil {
		t.Fatal(err)
	}
	generatedAgain, err := gobackend.Generate(core)
	if err != nil || string(generated) != string(generatedAgain) {
		t.Fatalf("block propagation generated Go is nondeterministic: %v", err)
	}
	for _, fragment := range []string{"propagated, present := pipelangPropagateOptional(p0)", "if !present", "p1 := propagated", "p2 := pipelangTrimText(p1)"} {
		if !strings.Contains(string(generated), fragment) {
			t.Fatalf("generated Go lacks block propagation %q:\n%s", fragment, generated)
		}
	}
	compileAndRunGeneratedGoFiles(t, generated, []byte(fmt.Sprintf(`package %s

import "testing"

func TestGeneratedBlockOptionalPropagation(t *testing.T) {
	present := PipeLangNormalize(pipelangSomeValue("  worker  "))
	typed, ok := present.(pipelangOptionalSome[string])
	if !ok || typed.value != "worker" { t.Fatalf("present = %%#v", present) }
	if _, ok := PipeLangNormalize(pipelangNoneValue[string]()).(pipelangOptionalNone[string]); !ok { t.Fatal("absence was not propagated") }
}
`, gobackend.PackageName)))
}

func TestV410BlockScopedTextResultPropagation(t *testing.T) {
	source := `public Class Root {
		public Result<string, string> Normalize(Result<string, string> carrier) {
			string value = propagate(carrier);
			string cleaned = trim(value);
			return ok<string, string>(cleaned);
		}
	}`
	input := semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", "block-result-propagation.pipe", source)}, nil)
	input.LanguageContract = PipeLangLanguageContractV410
	analysis := AnalyzeSemanticModuleSet(input)
	if err := analysis.Error(); err != nil {
		t.Fatal(err)
	}
	identity := semanticMethodNamed(t, analysis, "Normalize").Identity
	typed, err := LowerSemanticMethodToHIR(analysis, identity)
	if err != nil {
		t.Fatal(err)
	}
	core, err := LowerHIRToCore(typed)
	if err != nil {
		t.Fatal(err)
	}
	function := coreFunctionNamed(t, core, "Normalize")
	resultType := function.ReturnType
	text := resultType.Result.Success
	success := coreeval.Value{Type: resultType, Result: &coreeval.Outcome{OK: true, Value: coreeval.Value{Type: text, String: "  ready  "}}}
	outcome, err := coreeval.EvaluateProgram(core, coreir.SemanticIdentity{PackageID: string(identity.PackageID), Path: string(identity.Path)}, []coreeval.Value{success})
	if err != nil || !outcome.OK || outcome.Value.String != "ready" {
		t.Fatalf("success = %#v, %v", outcome, err)
	}
	failureText := coreeval.Value{Type: text, String: "unavailable"}
	failure := coreeval.Value{Type: resultType, Result: &coreeval.Outcome{Value: coreeval.Value{Type: text}, Failure: &failureText}}
	outcome, err = coreeval.EvaluateProgram(core, coreir.SemanticIdentity{PackageID: string(identity.PackageID), Path: string(identity.Path)}, []coreeval.Value{failure})
	if err != nil || outcome.OK || outcome.Failure == nil || outcome.Failure.String != "unavailable" {
		t.Fatalf("failure = %#v, %v", outcome, err)
	}
	generated, err := gobackend.Generate(core)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(generated), "if !p0.OK") || !strings.Contains(string(generated), "p1 := p0.Value") {
		t.Fatalf("generated Go lacks Result block propagation:\n%s", generated)
	}
	compileAndRunGeneratedGoFiles(t, generated, []byte(fmt.Sprintf(`package %s

import "testing"

func TestGeneratedBlockResultPropagation(t *testing.T) {
	success := PipeLangNormalize(pipelangTextResultOK("  ready  "))
	if !success.OK || success.Value != "ready" { t.Fatalf("success = %%#v", success) }
	failure := PipeLangNormalize(pipelangTextResultErr("unavailable"))
	if failure.OK || failure.Error != "unavailable" { t.Fatalf("failure = %%#v", failure) }
}
`, gobackend.PackageName)))
}

func TestV410BlockPropagationRejectsUnselectedShapes(t *testing.T) {
	cases := []struct {
		name     string
		contract LanguageContract
		source   string
		message  string
	}{
		{name: "prior contract", contract: PipeLangLanguageContractV400, source: `public Class Root { public Optional<string> Read(Optional<string> value) { string local = propagate(value); return some(local); } }`, message: "exclude propagation"},
		{name: "not first local", contract: PipeLangLanguageContractV410, source: `public Class Root { public Optional<string> Read(Optional<string> value) { Optional<string> copy = value; string local = propagate(copy); return some(local); } }`, message: "first immutable-local initializer"},
		{name: "computed operand", contract: PipeLangLanguageContractV410, source: `public Class Root { public Optional<string> Read(Optional<string> value) { string local = propagate(some("fixed")); return some(local); } }`, message: "sole direct carrier parameter"},
		{name: "extra parameter", contract: PipeLangLanguageContractV410, source: `public Class Root { public Optional<string> Read(Optional<string> value, string fallback) { string local = propagate(value); return some(local); } }`, message: "sole direct carrier parameter"},
		{name: "mismatched carrier return", contract: PipeLangLanguageContractV410, source: `public Class Root { public Optional<string> Read(Result<string, string> value) { string local = propagate(value); return some(local); } }`, message: "return type must be identical"},
		{name: "mismatched payload local", contract: PipeLangLanguageContractV410, source: `public Class Root { public Optional<string> Read(Optional<string> value) { bool local = propagate(value); return some("fixed"); } }`, message: "payload"},
		{name: "arithmetic Result", contract: PipeLangLanguageContractV410, source: `public Class Root { public Result<int, ArithmeticError> Read(Result<int, ArithmeticError> value) { int local = propagate(value); return ok<int, ArithmeticError>(local); } }`, message: "admitted Optional or bounded Result"},
		{name: "additional propagation", contract: PipeLangLanguageContractV410, source: `public Class Root { public Optional<string> Read(Optional<string> value) { string local = propagate(value); return some(propagate(value)); } }`, message: "exactly one propagation"},
		{name: "propagation in return", contract: PipeLangLanguageContractV410, source: `public Class Root { public Optional<string> Read(Optional<string> value) { Optional<string> copy = value; return some(propagate(copy)); } }`, message: "first immutable-local initializer"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			input := semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", test.name+".pipe", test.source)}, nil)
			input.LanguageContract = test.contract
			analysis := AnalyzeSemanticModuleSet(input)
			if len(analysis.Diagnostics) == 0 || analysis.Diagnostics[0].Code != CodePropagation || !strings.Contains(analysis.Diagnostics[0].Message, test.message) {
				t.Fatalf("diagnostics = %#v", analysis.Diagnostics)
			}
		})
	}
}

func TestV410BlockPropagationAdmitsCompleteCarrierMatrixAndV400Blocks(t *testing.T) {
	cases := []struct {
		name   string
		source string
	}{
		{
			name:   "optional record",
			source: `public Record Row { public string Id; } public Class Root { public Optional<Row> Read(Optional<Row> carrier) { Row row = propagate(carrier); return some(row); } }`,
		},
		{
			name:   "snapshot Result",
			source: `public Record Row { public string Id; } public Class Root { public Result<List<Row>, string> Read(Result<List<Row>, string> carrier) { List<Row> rows = propagate(carrier); return ok<List<Row>, string>(rows); } }`,
		},
		{
			name:   "inherited ordered locals",
			source: `public Class Root { public string Read(string value) { string first = trim(value); string second = first + "!"; return second; } }`,
		},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			input := semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", test.name+".pipe", test.source)}, nil)
			input.LanguageContract = PipeLangLanguageContractV410
			analysis := AnalyzeSemanticModuleSet(input)
			if err := analysis.Error(); err != nil {
				t.Fatal(err)
			}
			identity := semanticMethodNamed(t, analysis, "Read").Identity
			typed, err := LowerSemanticMethodToHIR(analysis, identity)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := LowerHIRToCore(typed); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestV410BlockPropagationCoreRejectsMalformedComposition(t *testing.T) {
	source := `public Class Root { public Optional<string> Read(Optional<string> value) { string local = propagate(value); return some(local); } }`
	input := semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", "block-propagation-core.pipe", source)}, nil)
	input.LanguageContract = PipeLangLanguageContractV410
	analysis := AnalyzeSemanticModuleSet(input)
	if err := analysis.Error(); err != nil {
		t.Fatal(err)
	}
	typed, err := LowerSemanticMethodToHIR(analysis, semanticMethodNamed(t, analysis, "Read").Identity)
	if err != nil {
		t.Fatal(err)
	}
	program, err := LowerHIRToCore(typed)
	if err != nil {
		t.Fatal(err)
	}

	prior := program
	prior.LanguageContract = coreir.LanguageContractV400
	if err := coreir.ValidateProgram(prior); err == nil || !strings.Contains(err.Error(), "exclude propagation") {
		t.Fatalf("prior contract error = %v", err)
	}

	extraParameter := program
	extraParameter.Functions = append([]coreir.Function(nil), program.Functions...)
	function := extraParameter.Functions[0]
	function.Parameters = append(function.Parameters, coreir.Parameter{Position: 1, Name: "extra", Type: function.Body.ImmutableLocal.Type})
	extraParameter.Functions[0] = function
	if err := coreir.ValidateProgram(extraParameter); err == nil || !strings.Contains(err.Error(), "sole direct carrier parameter") {
		t.Fatalf("extra parameter error = %v", err)
	}

	duplicate := program
	duplicate.Functions = append([]coreir.Function(nil), program.Functions...)
	function = duplicate.Functions[0]
	local := *function.Body.ImmutableLocal
	returned := *local.Return
	returned.Kind = coreir.ExprPropagate
	returned.Propagate = local.Initializer.Propagate
	returned.Some = nil
	local.Return = &returned
	function.Body.ImmutableLocal = &local
	duplicate.Functions[0] = function
	if err := coreir.ValidateProgram(duplicate); err == nil || !strings.Contains(err.Error(), "exactly one propagation") {
		t.Fatalf("duplicate propagation error = %v", err)
	}
	if _, err := gobackend.Generate(duplicate); err == nil {
		t.Fatal("Go backend accepted malformed block propagation Core")
	}
}
