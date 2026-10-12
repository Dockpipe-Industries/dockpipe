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

func TestV380ConditionalExpressionPipeline(t *testing.T) {
	source := `public Class Root {
		public string Normalize(string value) => trim(value);
		public string DisplayName(bool useFallback, string name, string fallback) => useFallback ? fallback : Normalize(name);
		public bool Visible(bool running, bool selected) => running && selected ? true : false;
	}`
	input := semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", "conditional.pipe", source)}, nil)
	input.LanguageContract = PipeLangLanguageContractV380
	analysis := AnalyzeSemanticModuleSet(input)
	if err := analysis.Error(); err != nil {
		t.Fatal(err)
	}
	projection, err := BuildSemanticProjection(analysis)
	if err != nil {
		t.Fatal(err)
	}
	if projection.LanguageContract != PipeLangLanguageContractV380 {
		t.Fatalf("projection language contract = %q", projection.LanguageContract)
	}

	identity := semanticMethodNamed(t, analysis, "DisplayName").Identity
	typed, err := LowerSemanticMethodToHIR(analysis, identity)
	if err != nil {
		t.Fatal(err)
	}
	function := hirFunctionNamed(t, typed, "DisplayName")
	if typed.LanguageContract != coreir.LanguageContractV380 || function.Body.Kind != hir.ExprConditional || function.Body.Conditional == nil || function.Body.Conditional.Condition.Kind != hir.ExprReference || function.Body.Conditional.WhenTrue.Kind != hir.ExprReference || function.Body.Conditional.WhenFalse.Kind != hir.ExprCall {
		t.Fatalf("DisplayName HIR = %#v", function.Body)
	}
	core, err := LowerHIRToCore(typed)
	if err != nil {
		t.Fatal(err)
	}
	coreFunction := coreFunctionNamed(t, core, "DisplayName")
	if coreFunction.Body.Kind != coreir.ExprConditional || coreFunction.Body.Conditional == nil || coreFunction.Body.Conditional.WhenFalse.Kind != coreir.ExprCall {
		t.Fatalf("DisplayName Core = %#v", coreFunction.Body)
	}
	text := coreir.Type{Kind: coreir.TypePrimitive, Primitive: coreir.PrimitiveString}
	boolean := coreir.Type{Kind: coreir.TypePrimitive, Primitive: coreir.PrimitiveBool}
	for _, test := range []struct {
		fallback bool
		want     string
	}{
		{fallback: true, want: "fallback"},
		{fallback: false, want: "name"},
	} {
		outcome, evalErr := coreeval.EvaluateProgram(core, coreir.SemanticIdentity{PackageID: string(identity.PackageID), Path: string(identity.Path)}, []coreeval.Value{
			{Type: boolean, Bool: test.fallback},
			{Type: text, String: "  name  "},
			{Type: text, String: "fallback"},
		})
		if evalErr != nil || !outcome.OK || outcome.Value.String != test.want {
			t.Fatalf("DisplayName(%t) = %#v, %v", test.fallback, outcome, evalErr)
		}
	}
	generated, err := gobackend.Generate(core)
	if err != nil {
		t.Fatal(err)
	}
	generatedAgain, err := gobackend.Generate(core)
	if err != nil || string(generated) != string(generatedAgain) {
		t.Fatalf("conditional generated Go is nondeterministic: %v", err)
	}
	if !strings.Contains(string(generated), "if p0") || !strings.Contains(string(generated), "return PipeLangNormalize(p1)") {
		t.Fatalf("generated Go lacks explicit lazy conditional:\n%s", generated)
	}
	compileAndRunGeneratedGoFiles(t, generated, []byte(fmt.Sprintf(`package %s

import "testing"

func TestGeneratedConditional(t *testing.T) {
	if got := PipeLangDisplayName(true, "  name  ", "fallback"); got != "fallback" { t.Fatalf("true = %%q", got) }
	if got := PipeLangDisplayName(false, "  name  ", "fallback"); got != "name" { t.Fatalf("false = %%q", got) }
}
`, gobackend.PackageName)))
}

func TestV380ConditionalExpressionDiscoversDirectBackendSupport(t *testing.T) {
	source := `public Record Row {
		public string Name;
	}
	public Class Root {
		public string Clean(bool enabled, string value) => enabled ? trim(value) : value;
		public List<Row> Order(bool enabled, List<Row> rows) => enabled ? sort_by_ordinal(rows, Row.Name) : rows;
	}`
	input := semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", "conditional-support.pipe", source)}, nil)
	input.LanguageContract = PipeLangLanguageContractV380
	analysis := AnalyzeSemanticModuleSet(input)
	if err := analysis.Error(); err != nil {
		t.Fatal(err)
	}
	for _, method := range []string{"Clean", "Order"} {
		t.Run(method, func(t *testing.T) {
			typed, err := LowerSemanticMethodToHIR(analysis, semanticMethodNamed(t, analysis, method).Identity)
			if err != nil {
				t.Fatal(err)
			}
			core, err := LowerHIRToCore(typed)
			if err != nil {
				t.Fatal(err)
			}
			generated, err := gobackend.Generate(core)
			if err != nil {
				t.Fatal(err)
			}
			compileAndRunGeneratedGoFiles(t, generated, []byte(fmt.Sprintf(`package %s

import "testing"

func TestGeneratedConditionalSupport(t *testing.T) {}
`, gobackend.PackageName)))
		})
	}
}

func TestV380ConditionalExpressionRejectsUnboundedShapes(t *testing.T) {
	cases := []struct {
		name    string
		source  string
		message string
	}{
		{name: "condition", source: `public Class Root { public string Pick(string condition, string left, string right) => condition ? left : right; }`, message: "condition requires bool"},
		{name: "branches", source: `public Class Root { public string Pick(bool condition) => condition ? "left" : 1; }`, message: "exactly the same type"},
		{name: "nested", source: `public Class Root { public string Pick(bool first, bool second) => first ? (second ? "a" : "b") : "c"; }`, message: "exactly one conditional"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			input := semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", test.name+".pipe", test.source)}, nil)
			input.LanguageContract = PipeLangLanguageContractV380
			analysis := AnalyzeSemanticModuleSet(input)
			if len(analysis.Diagnostics) != 1 || analysis.Diagnostics[0].Code != CodeExpressionType || !strings.Contains(analysis.Diagnostics[0].Message, test.message) {
				t.Fatalf("diagnostics = %#v", analysis.Diagnostics)
			}
		})
	}
}

func TestV380ConditionalExpressionDoesNotMigrateV370(t *testing.T) {
	source := `public Class Root { public string Pick(bool condition, string left, string right) => condition ? left : right; }`
	prior := semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", "prior.pipe", source)}, nil)
	prior.LanguageContract = PipeLangLanguageContractV370
	if analysis := AnalyzeSemanticModuleSet(prior); len(analysis.Diagnostics) == 0 {
		t.Fatal("v0.37.0 accepted v0.38.0 conditional source")
	}

	current := prior
	current.LanguageContract = PipeLangLanguageContractV380
	analysis := AnalyzeSemanticModuleSet(current)
	if err := analysis.Error(); err != nil {
		t.Fatal(err)
	}
	typed, err := LowerSemanticMethodToHIR(analysis, semanticMethodNamed(t, analysis, "Pick").Identity)
	if err != nil {
		t.Fatal(err)
	}
	core, err := LowerHIRToCore(typed)
	if err != nil {
		t.Fatal(err)
	}
	core.LanguageContract = coreir.LanguageContractV370
	if err := coreir.ValidateProgram(core); err == nil || !strings.Contains(err.Error(), "v0.38.0") {
		t.Fatalf("v0.37.0 Core conditional error = %v", err)
	}
}

func TestV380ConditionalCoreRejectsMismatchedAndNestedNodes(t *testing.T) {
	source := `public Class Root { public string Pick(bool condition, string left, string right) => condition ? left : right; }`
	input := semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", "core-adversarial.pipe", source)}, nil)
	input.LanguageContract = PipeLangLanguageContractV380
	analysis := AnalyzeSemanticModuleSet(input)
	if err := analysis.Error(); err != nil {
		t.Fatal(err)
	}
	typed, err := LowerSemanticMethodToHIR(analysis, semanticMethodNamed(t, analysis, "Pick").Identity)
	if err != nil {
		t.Fatal(err)
	}
	program, err := LowerHIRToCore(typed)
	if err != nil {
		t.Fatal(err)
	}

	mismatched := program
	mismatched.Functions = append([]coreir.Function(nil), program.Functions...)
	bad := mismatched.Functions[len(mismatched.Functions)-1]
	badConditional := *bad.Body.Conditional
	badConditional.WhenFalse = &coreir.Expr{Kind: coreir.ExprLiteral, Type: coreir.Type{Kind: coreir.TypePrimitive, Primitive: coreir.PrimitiveBool}, Literal: &coreir.Literal{Bool: false}}
	bad.Body.Conditional = &badConditional
	mismatched.Functions[len(mismatched.Functions)-1] = bad
	if err := coreir.ValidateProgram(mismatched); err == nil || !strings.Contains(err.Error(), "exactly the same type") {
		t.Fatalf("mismatched Core error = %v", err)
	}

	nested := program
	nested.Functions = append([]coreir.Function(nil), program.Functions...)
	bad = nested.Functions[len(nested.Functions)-1]
	innerConditional := *bad.Body.Conditional
	inner := coreir.Expr{Kind: coreir.ExprConditional, Type: bad.Body.Type, Conditional: &innerConditional}
	badConditional = *bad.Body.Conditional
	badConditional.WhenFalse = &inner
	bad.Body.Conditional = &badConditional
	nested.Functions[len(nested.Functions)-1] = bad
	if err := coreir.ValidateProgram(nested); err == nil || !strings.Contains(err.Error(), "exactly one conditional") {
		t.Fatalf("nested Core error = %v", err)
	}
}
