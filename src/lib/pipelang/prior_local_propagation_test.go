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

func TestV420PriorLocalResultPropagationPipeline(t *testing.T) {
	source := `public Class Root {
		public Result<string, string> Validate(string raw) => raw == "" ? err<string, string>("missing") : ok<string, string>(raw);
		public Result<string, string> Normalize(string raw) {
			Result<string, string> carrier = Validate(raw);
			string value = propagate(carrier);
			string cleaned = trim(value);
			return ok<string, string>(cleaned);
		}
	}`
	input := semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", "prior-local-result.pipe", source)}, nil)
	input.LanguageContract = PipeLangLanguageContractV420
	analysis := AnalyzeSemanticModuleSet(input)
	if err := analysis.Error(); err != nil {
		t.Fatal(err)
	}
	projection, err := BuildSemanticProjection(analysis)
	if err != nil {
		t.Fatal(err)
	}
	if projection.LanguageContract != PipeLangLanguageContractV420 {
		t.Fatalf("projection language contract = %q", projection.LanguageContract)
	}

	identity := semanticMethodNamed(t, analysis, "Normalize").Identity
	typed, err := LowerSemanticMethodToHIR(analysis, identity)
	if err != nil {
		t.Fatal(err)
	}
	function := hirFunctionNamed(t, typed, "Normalize")
	first := function.Body.ImmutableLocal
	if typed.LanguageContract != coreir.LanguageContractV420 || function.Body.Kind != hir.ExprImmutableLocal || first == nil || first.Initializer.Kind != hir.ExprCall || first.Return.Kind != hir.ExprImmutableLocal {
		t.Fatalf("Normalize HIR = %#v", function.Body)
	}
	second := first.Return.ImmutableLocal
	if second == nil || second.Initializer.Kind != hir.ExprPropagate || second.Return.Kind != hir.ExprImmutableLocal || second.Return.ImmutableLocal.Initializer.Kind != hir.ExprTextTrim {
		t.Fatalf("Normalize prior-local HIR = %#v", function.Body)
	}

	core, err := LowerHIRToCore(typed)
	if err != nil {
		t.Fatal(err)
	}
	coreFunction := coreFunctionNamed(t, core, "Normalize")
	coreFirst := coreFunction.Body.ImmutableLocal
	if coreFunction.Body.Kind != coreir.ExprImmutableLocal || coreFirst == nil || coreFirst.Position != 1 || coreFirst.Initializer.Kind != coreir.ExprCall || coreFirst.Return.Kind != coreir.ExprImmutableLocal {
		t.Fatalf("Normalize Core = %#v", coreFunction.Body)
	}
	coreSecond := coreFirst.Return.ImmutableLocal
	if coreSecond == nil || coreSecond.Position != 2 || coreSecond.Initializer.Kind != coreir.ExprPropagate || coreSecond.Initializer.Propagate == nil || coreSecond.Initializer.Propagate.Value == nil || coreSecond.Initializer.Propagate.Value.Parameter == nil || *coreSecond.Initializer.Propagate.Value.Parameter != coreFirst.Position {
		t.Fatalf("Normalize prior-local Core = %#v", coreFunction.Body)
	}

	text := coreFunction.Parameters[0].Type
	outcome, err := coreeval.EvaluateProgram(core, coreir.SemanticIdentity{PackageID: string(identity.PackageID), Path: string(identity.Path)}, []coreeval.Value{{Type: text, String: "  ready  "}})
	if err != nil || !outcome.OK || outcome.Value.String != "ready" {
		t.Fatalf("success = %#v, %v", outcome, err)
	}
	outcome, err = coreeval.EvaluateProgram(core, coreir.SemanticIdentity{PackageID: string(identity.PackageID), Path: string(identity.Path)}, []coreeval.Value{{Type: text, String: ""}})
	if err != nil || outcome.OK || outcome.Failure == nil || outcome.Failure.String != "missing" {
		t.Fatalf("failure = %#v, %v", outcome, err)
	}

	generated, err := gobackend.Generate(core)
	if err != nil {
		t.Fatal(err)
	}
	generatedAgain, err := gobackend.Generate(core)
	if err != nil || string(generatedAgain) != string(generated) {
		t.Fatalf("prior-local generated Go is nondeterministic: %v", err)
	}
	for _, fragment := range []string{"p1 := PipeLangValidate(p0)", "if !p1.OK", "p2 := p1.Value", "p3 := pipelangTrimText(p2)"} {
		if !strings.Contains(string(generated), fragment) {
			t.Fatalf("generated Go lacks prior-local propagation %q:\n%s", fragment, generated)
		}
	}
	if strings.Count(string(generated), "p1 := PipeLangValidate(p0)") != 1 {
		t.Fatalf("generated Go does not evaluate the helper exactly once:\n%s", generated)
	}
	compileAndRunGeneratedGoFiles(t, generated, []byte(fmt.Sprintf(`package %s

import "testing"

func TestGeneratedPriorLocalResultPropagation(t *testing.T) {
	success := PipeLangNormalize("  ready  ")
	if !success.OK || success.Value != "ready" { t.Fatalf("success = %%#v", success) }
	failure := PipeLangNormalize("")
	if failure.OK || failure.Error != "missing" { t.Fatalf("failure = %%#v", failure) }
}
`, gobackend.PackageName)))
}

func TestV420PriorLocalPropagationAdmitsCompleteCarrierMatrixAndInheritedDirectPropagation(t *testing.T) {
	cases := []struct {
		name   string
		source string
	}{
		{
			name:   "optional record",
			source: `public Record Row { public string Id; } public Class Root { public Optional<Row> Validate(Row raw) => some(raw); public Optional<Row> Read(Row raw) { Optional<Row> carrier = Validate(raw); Row row = propagate(carrier); return some(row); } }`,
		},
		{
			name:   "snapshot Result",
			source: `public Record Row { public string Id; } public Class Root { public Result<List<Row>, string> Validate(List<Row> raw) => ok<List<Row>, string>(raw); public Result<List<Row>, string> Read(List<Row> raw) { Result<List<Row>, string> carrier = Validate(raw); List<Row> rows = propagate(carrier); return ok<List<Row>, string>(rows); } }`,
		},
		{
			name:   "inherited direct propagation",
			source: `public Class Root { public Optional<string> Read(Optional<string> carrier) { string value = propagate(carrier); return some(value); } }`,
		},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			input := semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", test.name+".pipe", test.source)}, nil)
			input.LanguageContract = PipeLangLanguageContractV420
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
			if _, err := gobackend.Generate(program); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestV420PriorLocalOptionalPropagation(t *testing.T) {
	source := `public Class Root {
		public Optional<string> Validate(string raw) => raw == "" ? none<string>() : some(raw);
		public Optional<string> Normalize(string raw) {
			Optional<string> carrier = Validate(raw);
			string value = propagate(carrier);
			return some(trim(value));
		}
	}`
	input := semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", "prior-local-optional.pipe", source)}, nil)
	input.LanguageContract = PipeLangLanguageContractV420
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
	text := function.Parameters[0].Type
	present, err := coreeval.EvaluateProgram(core, coreir.SemanticIdentity{PackageID: string(identity.PackageID), Path: string(identity.Path)}, []coreeval.Value{{Type: text, String: "  worker  "}})
	if err != nil || !present.OK || present.Value.Optional == nil || !present.Value.Optional.Present || present.Value.Optional.Value.String != "worker" {
		t.Fatalf("present = %#v, %v", present, err)
	}
	absent, err := coreeval.EvaluateProgram(core, coreir.SemanticIdentity{PackageID: string(identity.PackageID), Path: string(identity.Path)}, []coreeval.Value{{Type: text, String: ""}})
	if err != nil || !absent.OK || absent.Value.Optional == nil || absent.Value.Optional.Present {
		t.Fatalf("absent = %#v, %v", absent, err)
	}
	generated, err := gobackend.Generate(core)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(generated), "p1 := PipeLangValidate(p0)") || !strings.Contains(string(generated), "pipelangPropagateOptional(p1)") || !strings.Contains(string(generated), "p2 := propagated") {
		t.Fatalf("generated Go lacks Optional prior-local propagation:\n%s", generated)
	}
	compileAndRunGeneratedGoFiles(t, generated, []byte(fmt.Sprintf(`package %s

import "testing"

func TestGeneratedPriorLocalOptionalPropagation(t *testing.T) {
	present := PipeLangNormalize("  worker  ")
	typed, ok := present.(pipelangOptionalSome[string])
	if !ok || typed.value != "worker" { t.Fatalf("present = %%#v", present) }
	if _, ok := PipeLangNormalize("").(pipelangOptionalNone[string]); !ok { t.Fatal("absence was not propagated") }
}
`, gobackend.PackageName)))
}

func TestV420PriorLocalPropagationRejectsUnselectedShapes(t *testing.T) {
	cases := []struct {
		name     string
		contract LanguageContract
		source   string
		message  string
	}{
		{name: "prior contract", contract: PipeLangLanguageContractV410, source: `public Class Root { public Result<string, string> Validate(string raw) => ok<string, string>(raw); public Result<string, string> Read(string raw) { Result<string, string> carrier = Validate(raw); string value = propagate(carrier); return ok<string, string>(value); } }`, message: "first immutable-local initializer"},
		{name: "non-call carrier", contract: PipeLangLanguageContractV420, source: `public Class Root { public Result<string, string> Read(string raw) { Result<string, string> carrier = ok<string, string>(raw); string value = propagate(carrier); return ok<string, string>(value); } }`, message: "helper-call carrier local"},
		{name: "computed helper argument", contract: PipeLangLanguageContractV420, source: `public Class Root { public Result<string, string> Validate(string raw) => ok<string, string>(raw); public Result<string, string> Read(string raw) { Result<string, string> carrier = Validate(trim(raw)); string value = propagate(carrier); return ok<string, string>(value); } }`, message: "sole direct parameter"},
		{name: "extra helper argument", contract: PipeLangLanguageContractV420, source: `public Class Root { public Result<string, string> Validate(string raw, string fallback) => raw == "" ? err<string, string>(fallback) : ok<string, string>(raw); public Result<string, string> Read(string raw) { Result<string, string> carrier = Validate(raw, raw); string value = propagate(carrier); return ok<string, string>(value); } }`, message: "sole direct parameter"},
		{name: "extra method parameter", contract: PipeLangLanguageContractV420, source: `public Class Root { public Result<string, string> Validate(string raw) => ok<string, string>(raw); public Result<string, string> Read(string raw, string fallback) { Result<string, string> carrier = Validate(raw); string value = propagate(carrier); return ok<string, string>(value); } }`, message: "sole direct parameter"},
		{name: "intervening local", contract: PipeLangLanguageContractV420, source: `public Class Root { public Result<string, string> Validate(string raw) => ok<string, string>(raw); public Result<string, string> Read(string raw) { Result<string, string> carrier = Validate(raw); Result<string, string> copy = carrier; string value = propagate(copy); return ok<string, string>(value); } }`, message: "second immutable-local initializer"},
		{name: "wrong propagated local", contract: PipeLangLanguageContractV420, source: `public Class Root { public Result<string, string> Validate(string raw) => ok<string, string>(raw); public Result<string, string> Read(string raw) { Result<string, string> carrier = Validate(raw); string value = propagate(raw); return ok<string, string>(value); } }`, message: "immediately preceding"},
		{name: "direct computed propagation", contract: PipeLangLanguageContractV420, source: `public Class Root { public Result<string, string> Validate(string raw) => ok<string, string>(raw); public Result<string, string> Read(string raw) { string value = propagate(Validate(raw)); return ok<string, string>(value); } }`, message: "direct match and propagate carriers"},
		{name: "mismatched carrier return", contract: PipeLangLanguageContractV420, source: `public Class Root { public Optional<string> Validate(string raw) => some(raw); public Result<string, string> Read(string raw) { Optional<string> carrier = Validate(raw); string value = propagate(carrier); return ok<string, string>(value); } }`, message: "return type must be identical"},
		{name: "mismatched payload local", contract: PipeLangLanguageContractV420, source: `public Class Root { public Result<string, string> Validate(string raw) => ok<string, string>(raw); public Result<string, string> Read(string raw) { Result<string, string> carrier = Validate(raw); bool value = propagate(carrier); return ok<string, string>(raw); } }`, message: "success payload"},
		{name: "arithmetic Result", contract: PipeLangLanguageContractV420, source: `public Class Root { public Result<int, ArithmeticError> Validate(int raw) => raw + 0; public Result<int, ArithmeticError> Read(int raw) { Result<int, ArithmeticError> carrier = Validate(raw); int value = propagate(carrier); return value + 0; } }`, message: "admitted Optional or bounded Result"},
		{name: "additional propagation", contract: PipeLangLanguageContractV420, source: `public Class Root { public Result<string, string> Validate(string raw) => ok<string, string>(raw); public Result<string, string> Read(string raw) { Result<string, string> carrier = Validate(raw); string value = propagate(carrier); return ok<string, string>(propagate(carrier)); } }`, message: "one helper-call carrier local"},
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

func TestV420PriorLocalPropagationCoreRejectsMalformedComposition(t *testing.T) {
	source := `public Class Root { public Result<string, string> Validate(string raw) => ok<string, string>(raw); public Result<string, string> Read(string raw) { Result<string, string> carrier = Validate(raw); string value = propagate(carrier); return ok<string, string>(value); } }`
	input := semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", "prior-local-core.pipe", source)}, nil)
	input.LanguageContract = PipeLangLanguageContractV420
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
	prior.LanguageContract = coreir.LanguageContractV410
	if err := coreir.ValidateProgram(prior); err == nil || !strings.Contains(err.Error(), "first immutable-local initializer") {
		t.Fatalf("prior contract error = %v", err)
	}

	badArgument := program
	badArgument.Functions = append([]coreir.Function(nil), program.Functions...)
	read := coreFunctionNamed(t, badArgument, "Read")
	first := *read.Body.ImmutableLocal
	call := *first.Initializer.Call
	argument := *call.Arguments[0]
	position := first.Position
	argument.Parameter = &position
	call.Arguments = append([]*coreir.Expr(nil), call.Arguments...)
	call.Arguments[0] = &argument
	initializer := *first.Initializer
	initializer.Call = &call
	first.Initializer = &initializer
	read.Body.ImmutableLocal = &first
	for index := range badArgument.Functions {
		if badArgument.Functions[index].Name == "Read" {
			badArgument.Functions[index] = read
		}
	}
	if err := coreir.ValidateProgram(badArgument); err == nil || !strings.Contains(err.Error(), "sole direct parameter") {
		t.Fatalf("bad argument error = %v", err)
	}

	badCarrier := program
	badCarrier.Functions = append([]coreir.Function(nil), program.Functions...)
	read = coreFunctionNamed(t, badCarrier, "Read")
	first = *read.Body.ImmutableLocal
	secondExpr := *first.Return
	second := *secondExpr.ImmutableLocal
	propagation := *second.Initializer.Propagate
	reference := *propagation.Value
	zero := 0
	reference.Parameter = &zero
	propagation.Value = &reference
	secondInitializer := *second.Initializer
	secondInitializer.Propagate = &propagation
	second.Initializer = &secondInitializer
	secondExpr.ImmutableLocal = &second
	first.Return = &secondExpr
	read.Body.ImmutableLocal = &first
	for index := range badCarrier.Functions {
		if badCarrier.Functions[index].Name == "Read" {
			badCarrier.Functions[index] = read
		}
	}
	if err := coreir.ValidateProgram(badCarrier); err == nil || !strings.Contains(err.Error(), "immediately preceding") {
		t.Fatalf("bad carrier error = %v", err)
	}
}
