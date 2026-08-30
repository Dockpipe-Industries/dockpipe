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

func TestV390ImmutableLocalPipeline(t *testing.T) {
	source := `public Class Root {
		public string Normalize(string value) => trim(value);
		public string DisplayName(string name, string fallback) {
			string cleaned = Normalize(name);
			return cleaned == "" ? fallback : cleaned;
		}
	}`
	input := semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", "immutable-local.pipe", source)}, nil)
	input.LanguageContract = PipeLangLanguageContractV390
	analysis := AnalyzeSemanticModuleSet(input)
	if err := analysis.Error(); err != nil {
		t.Fatal(err)
	}
	projection, err := BuildSemanticProjection(analysis)
	if err != nil {
		t.Fatal(err)
	}
	if projection.LanguageContract != PipeLangLanguageContractV390 {
		t.Fatalf("projection language contract = %q", projection.LanguageContract)
	}

	identity := semanticMethodNamed(t, analysis, "DisplayName").Identity
	typed, err := LowerSemanticMethodToHIR(analysis, identity)
	if err != nil {
		t.Fatal(err)
	}
	function := hirFunctionNamed(t, typed, "DisplayName")
	local := function.Body.ImmutableLocal
	if typed.LanguageContract != coreir.LanguageContractV390 || function.Body.Kind != hir.ExprImmutableLocal || local == nil || local.Binding.Kind != hir.BindingLocal || local.Binding.Position != 2 || local.Initializer.Kind != hir.ExprCall || local.Return.Kind != hir.ExprConditional {
		t.Fatalf("DisplayName HIR = %#v", function.Body)
	}
	core, err := LowerHIRToCore(typed)
	if err != nil {
		t.Fatal(err)
	}
	coreFunction := coreFunctionNamed(t, core, "DisplayName")
	coreLocal := coreFunction.Body.ImmutableLocal
	if coreFunction.Body.Kind != coreir.ExprImmutableLocal || coreLocal == nil || coreLocal.Position != 2 || coreLocal.Initializer.Kind != coreir.ExprCall || coreLocal.Return.Kind != coreir.ExprConditional {
		t.Fatalf("DisplayName Core = %#v", coreFunction.Body)
	}
	text := coreir.Type{Kind: coreir.TypePrimitive, Primitive: coreir.PrimitiveString}
	for _, test := range []struct {
		name     string
		fallback string
		want     string
	}{
		{name: "  name  ", fallback: "fallback", want: "name"},
		{name: "   ", fallback: "fallback", want: "fallback"},
	} {
		outcome, evalErr := coreeval.EvaluateProgram(core, coreir.SemanticIdentity{PackageID: string(identity.PackageID), Path: string(identity.Path)}, []coreeval.Value{{Type: text, String: test.name}, {Type: text, String: test.fallback}})
		if evalErr != nil || !outcome.OK || outcome.Value.String != test.want {
			t.Fatalf("DisplayName(%q) = %#v, %v", test.name, outcome, evalErr)
		}
	}
	generated, err := gobackend.Generate(core)
	if err != nil {
		t.Fatal(err)
	}
	generatedAgain, err := gobackend.Generate(core)
	if err != nil || string(generated) != string(generatedAgain) {
		t.Fatalf("immutable local generated Go is nondeterministic: %v", err)
	}
	if !strings.Contains(string(generated), "p2 := PipeLangNormalize(p0)") || !strings.Contains(string(generated), "return func() string") {
		t.Fatalf("generated Go lacks explicit immutable local:\n%s", generated)
	}
	compileAndRunGeneratedGoFiles(t, generated, []byte(fmt.Sprintf(`package %s

import "testing"

func TestGeneratedImmutableLocal(t *testing.T) {
	if got := PipeLangDisplayName("  name  ", "fallback"); got != "name" { t.Fatalf("name = %%q", got) }
	if got := PipeLangDisplayName("   ", "fallback"); got != "fallback" { t.Fatalf("fallback = %%q", got) }
}
`, gobackend.PackageName)))
}

func TestV400OrderedImmutableLocalsPipeline(t *testing.T) {
	source := `public Class Root {
		public string Normalize(string value) => trim(value);
		public string DisplayName(string name, string fallback) {
			string normalized = Normalize(name);
			string selected = normalized == "" ? fallback : normalized;
			string displayed = Normalize(selected);
			return displayed;
		}
	}`
	input := semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", "ordered-immutable-locals.pipe", source)}, nil)
	input.LanguageContract = PipeLangLanguageContractV400
	analysis := AnalyzeSemanticModuleSet(input)
	if err := analysis.Error(); err != nil {
		t.Fatal(err)
	}
	projection, err := BuildSemanticProjection(analysis)
	if err != nil {
		t.Fatal(err)
	}
	if projection.LanguageContract != PipeLangLanguageContractV400 {
		t.Fatalf("projection language contract = %q", projection.LanguageContract)
	}

	identity := semanticMethodNamed(t, analysis, "DisplayName").Identity
	typed, err := LowerSemanticMethodToHIR(analysis, identity)
	if err != nil {
		t.Fatal(err)
	}
	function := hirFunctionNamed(t, typed, "DisplayName")
	first := function.Body.ImmutableLocal
	if typed.LanguageContract != coreir.LanguageContractV400 || function.Body.Kind != hir.ExprImmutableLocal || first == nil || first.Binding.Position != 2 || first.Initializer.Kind != hir.ExprCall {
		t.Fatalf("DisplayName first HIR local = %#v", function.Body)
	}
	second := first.Return.ImmutableLocal
	if first.Return.Kind != hir.ExprImmutableLocal || second == nil || second.Binding.Position != 3 || second.Initializer.Kind != hir.ExprConditional {
		t.Fatalf("DisplayName second HIR local = %#v", first.Return)
	}
	third := second.Return.ImmutableLocal
	if second.Return.Kind != hir.ExprImmutableLocal || third == nil || third.Binding.Position != 4 || third.Initializer.Kind != hir.ExprCall || third.Return.Kind != hir.ExprReference {
		t.Fatalf("DisplayName third HIR local = %#v", second.Return)
	}

	core, err := LowerHIRToCore(typed)
	if err != nil {
		t.Fatal(err)
	}
	coreFunction := coreFunctionNamed(t, core, "DisplayName")
	coreFirst := coreFunction.Body.ImmutableLocal
	if coreFunction.Body.Kind != coreir.ExprImmutableLocal || coreFirst == nil || coreFirst.Position != 2 {
		t.Fatalf("DisplayName first Core local = %#v", coreFunction.Body)
	}
	coreSecond := coreFirst.Return.ImmutableLocal
	coreThird := coreSecond.Return.ImmutableLocal
	if coreSecond == nil || coreSecond.Position != 3 || coreThird == nil || coreThird.Position != 4 || coreThird.Return.Kind != coreir.ExprReference {
		t.Fatalf("DisplayName Core sequence = %#v", coreFunction.Body)
	}

	text := coreir.Type{Kind: coreir.TypePrimitive, Primitive: coreir.PrimitiveString}
	for _, test := range []struct {
		name, fallback, want string
	}{
		{name: "  name  ", fallback: "fallback", want: "name"},
		{name: "   ", fallback: "  fallback  ", want: "fallback"},
	} {
		outcome, evalErr := coreeval.EvaluateProgram(core, coreir.SemanticIdentity{PackageID: string(identity.PackageID), Path: string(identity.Path)}, []coreeval.Value{{Type: text, String: test.name}, {Type: text, String: test.fallback}})
		if evalErr != nil || !outcome.OK || outcome.Value.String != test.want {
			t.Fatalf("DisplayName(%q) = %#v, %v", test.name, outcome, evalErr)
		}
	}
	generated, err := gobackend.Generate(core)
	if err != nil {
		t.Fatal(err)
	}
	generatedAgain, err := gobackend.Generate(core)
	if err != nil || string(generated) != string(generatedAgain) {
		t.Fatalf("ordered immutable locals generated Go is nondeterministic: %v", err)
	}
	for _, fragment := range []string{"p2 := PipeLangNormalize(p0)", "p3 := func() string", "p4 := PipeLangNormalize(p3)"} {
		if !strings.Contains(string(generated), fragment) {
			t.Fatalf("generated Go lacks ordered immutable local %q:\n%s", fragment, generated)
		}
	}
	compileAndRunGeneratedGoFiles(t, generated, []byte(fmt.Sprintf(`package %s

import "testing"

func TestGeneratedOrderedImmutableLocals(t *testing.T) {
	if got := PipeLangDisplayName("  name  ", "fallback"); got != "name" { t.Fatalf("name = %%q", got) }
	if got := PipeLangDisplayName("   ", "  fallback  "); got != "fallback" { t.Fatalf("fallback = %%q", got) }
}
`, gobackend.PackageName)))
}

func TestV390ImmutableLocalSupportsExistingCollectionExpressions(t *testing.T) {
	source := `public Record Row { public string Name; }
	public Class Root {
		public int EmptyCount() {
			List<Row> rows = empty_list<Row>();
			return count(rows);
		}
	}`
	input := semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", "immutable-local-list.pipe", source)}, nil)
	input.LanguageContract = PipeLangLanguageContractV390
	analysis := AnalyzeSemanticModuleSet(input)
	if err := analysis.Error(); err != nil {
		t.Fatal(err)
	}
	identity := semanticMethodNamed(t, analysis, "EmptyCount").Identity
	typed, err := LowerSemanticMethodToHIR(analysis, identity)
	if err != nil {
		t.Fatal(err)
	}
	core, err := LowerHIRToCore(typed)
	if err != nil {
		t.Fatal(err)
	}
	outcome, err := coreeval.EvaluateProgram(core, coreir.SemanticIdentity{PackageID: string(identity.PackageID), Path: string(identity.Path)}, nil)
	if err != nil || !outcome.OK || outcome.Value.Int != 0 {
		t.Fatalf("EmptyCount = %#v, %v", outcome, err)
	}
	generated, err := gobackend.Generate(core)
	if err != nil {
		t.Fatal(err)
	}
	compileAndRunGeneratedGoFiles(t, generated, []byte(fmt.Sprintf(`package %s

import "testing"

func TestGeneratedImmutableLocalList(t *testing.T) {
	if got := PipeLangEmptyCount(); got != 0 { t.Fatalf("count = %%d", got) }
}
`, gobackend.PackageName)))
}

func TestV390ImmutableLocalSupportsHiddenArithmeticResult(t *testing.T) {
	source := `public Class Root {
		public string AddStatus(int left, int right) {
			Result<int, ArithmeticError> sum = left + right;
			return match(sum){ ok(value) => "ok", err(problem) => "error" };
		}
	}`
	input := semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", "immutable-local-result.pipe", source)}, nil)
	input.LanguageContract = PipeLangLanguageContractV390
	analysis := AnalyzeSemanticModuleSet(input)
	if err := analysis.Error(); err != nil {
		t.Fatal(err)
	}
	identity := semanticMethodNamed(t, analysis, "AddStatus").Identity
	typed, err := LowerSemanticMethodToHIR(analysis, identity)
	if err != nil {
		t.Fatal(err)
	}
	core, err := LowerHIRToCore(typed)
	if err != nil {
		t.Fatal(err)
	}
	integer := coreir.SignedInteger(64)
	for _, test := range []struct {
		left, right int64
		want        string
	}{
		{left: 1, right: 2, want: "ok"},
		{left: 9223372036854775807, right: 1, want: "error"},
	} {
		outcome, evalErr := coreeval.EvaluateProgram(core, coreir.SemanticIdentity{PackageID: string(identity.PackageID), Path: string(identity.Path)}, []coreeval.Value{{Type: integer, Int: test.left}, {Type: integer, Int: test.right}})
		if evalErr != nil || !outcome.OK || outcome.Value.String != test.want {
			t.Fatalf("AddStatus(%d, %d) = %#v, %v", test.left, test.right, outcome, evalErr)
		}
	}
	generated, err := gobackend.Generate(core)
	if err != nil {
		t.Fatal(err)
	}
	compileAndRunGeneratedGoFiles(t, generated, []byte(fmt.Sprintf(`package %s

import "testing"

func TestGeneratedImmutableLocalArithmeticResult(t *testing.T) {
	if got := PipeLangAddStatus(1, 2); got != "ok" { t.Fatalf("success = %%q", got) }
	if got := PipeLangAddStatus(9223372036854775807, 1); got != "error" { t.Fatalf("overflow = %%q", got) }
}
`, gobackend.PackageName)))
}

func TestV390ImmutableLocalRejectsInvalidSourceShapes(t *testing.T) {
	cases := []struct {
		name     string
		contract LanguageContract
		source   string
		message  string
	}{
		{name: "prior contract", contract: PipeLangLanguageContractV380, source: `public Class Root { public string Clean(string value) { string local = value; return local; } }`, message: "v0.39.0"},
		{name: "initializer type", contract: PipeLangLanguageContractV390, source: `public Class Root { public string Clean(string value) { bool local = value; return value; } }`, message: "initializer has type"},
		{name: "parameter shadow", contract: PipeLangLanguageContractV390, source: `public Class Root { public string Clean(string value) { string value = "x"; return value; } }`, message: "shadows an existing binding"},
		{name: "field shadow", contract: PipeLangLanguageContractV390, source: `public Class Root { public string Name; public string Clean(string value) { string Name = value; return Name; } }`, message: "shadows an existing binding"},
		{name: "self reference", contract: PipeLangLanguageContractV390, source: `public Class Root { public string Clean(string value) { string local = local; return value; } }`, message: "unknown identifier"},
		{name: "second local", contract: PipeLangLanguageContractV390, source: `public Class Root { public string Clean(string value) { string first = value; string second = value; return first; } }`, message: "expected return"},
		{name: "duplicate local", contract: PipeLangLanguageContractV400, source: `public Class Root { public string Clean(string value) { string first = value; string first = value; return first; } }`, message: "shadows an existing binding"},
		{name: "forward reference", contract: PipeLangLanguageContractV400, source: `public Class Root { public string Clean(string value) { string first = second; string second = value; return first; } }`, message: "unknown identifier"},
		{name: "later initializer type", contract: PipeLangLanguageContractV400, source: `public Class Root { public string Clean(string value) { string first = value; bool second = first; return first; } }`, message: "initializer has type"},
		{name: "propagation", contract: PipeLangLanguageContractV390, source: `public Class Root { public Optional<string> Read(Optional<string> value) { string local = propagate(value); return some(local); } }`, message: "exclude propagation"},
		{name: "sequence propagation", contract: PipeLangLanguageContractV400, source: `public Class Root { public Optional<string> Read(Optional<string> value) { string local = propagate(value); string copy = local; return some(copy); } }`, message: "exclude propagation"},
		{name: "private method", contract: PipeLangLanguageContractV390, source: `public Class Root { private string Clean(string value) { string local = value; return local; } }`, message: "only in public methods"},
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

func TestV390ImmutableLocalCoreRejectsInvalidShapes(t *testing.T) {
	source := `public Class Root { public string Clean(string value) { string local = value; return local; } }`
	input := semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", "immutable-local-core.pipe", source)}, nil)
	input.LanguageContract = PipeLangLanguageContractV390
	analysis := AnalyzeSemanticModuleSet(input)
	if err := analysis.Error(); err != nil {
		t.Fatal(err)
	}
	typed, err := LowerSemanticMethodToHIR(analysis, semanticMethodNamed(t, analysis, "Clean").Identity)
	if err != nil {
		t.Fatal(err)
	}
	program, err := LowerHIRToCore(typed)
	if err != nil {
		t.Fatal(err)
	}

	prior := program
	prior.LanguageContract = coreir.LanguageContractV380
	if err := coreir.ValidateProgram(prior); err == nil || !strings.Contains(err.Error(), "v0.39.0") {
		t.Fatalf("prior contract error = %v", err)
	}

	badPosition := program
	badPosition.Functions = append([]coreir.Function(nil), program.Functions...)
	bad := badPosition.Functions[0]
	local := *bad.Body.ImmutableLocal
	local.Position++
	bad.Body.ImmutableLocal = &local
	badPosition.Functions[0] = bad
	if err := coreir.ValidateProgram(badPosition); err == nil || !strings.Contains(err.Error(), "canonically positioned") {
		t.Fatalf("position error = %v", err)
	}

	nested := program
	nested.Functions = append([]coreir.Function(nil), program.Functions...)
	bad = nested.Functions[0]
	local = *bad.Body.ImmutableLocal
	nestedLocal := bad.Body
	local.Return = &nestedLocal
	bad.Body.ImmutableLocal = &local
	nested.Functions[0] = bad
	if err := coreir.ValidateProgram(nested); err == nil || !strings.Contains(err.Error(), "exactly one top-level") {
		t.Fatalf("nested local error = %v", err)
	}

	propagation := program
	propagation.Functions = append([]coreir.Function(nil), program.Functions...)
	bad = propagation.Functions[0]
	local = *bad.Body.ImmutableLocal
	operand := *local.Initializer
	local.Initializer = &coreir.Expr{Kind: coreir.ExprPropagate, Type: local.Type, Propagate: &coreir.Propagate{Value: &operand, Carrier: operand.Type}}
	bad.Body.ImmutableLocal = &local
	propagation.Functions[0] = bad
	if err := coreir.ValidateProgram(propagation); err == nil || !strings.Contains(err.Error(), "exclude propagation") {
		t.Fatalf("propagation error = %v", err)
	}
}

func TestV400ImmutableLocalCoreRejectsNonCanonicalSequence(t *testing.T) {
	source := `public Class Root { public string Clean(string value) { string first = value; string second = first; return second; } }`
	input := semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", "ordered-immutable-locals-core.pipe", source)}, nil)
	input.LanguageContract = PipeLangLanguageContractV400
	analysis := AnalyzeSemanticModuleSet(input)
	if err := analysis.Error(); err != nil {
		t.Fatal(err)
	}
	typed, err := LowerSemanticMethodToHIR(analysis, semanticMethodNamed(t, analysis, "Clean").Identity)
	if err != nil {
		t.Fatal(err)
	}
	program, err := LowerHIRToCore(typed)
	if err != nil {
		t.Fatal(err)
	}

	prior := program
	prior.LanguageContract = coreir.LanguageContractV390
	if err := coreir.ValidateProgram(prior); err == nil || !strings.Contains(err.Error(), "exactly one top-level") {
		t.Fatalf("prior contract error = %v", err)
	}

	duplicate := program
	duplicate.Functions = append([]coreir.Function(nil), program.Functions...)
	function := duplicate.Functions[0]
	outer := *function.Body.ImmutableLocal
	innerExpr := *outer.Return
	inner := *innerExpr.ImmutableLocal
	inner.Name = outer.Name
	innerExpr.ImmutableLocal = &inner
	outer.Return = &innerExpr
	function.Body.ImmutableLocal = &outer
	duplicate.Functions[0] = function
	if err := coreir.ValidateProgram(duplicate); err == nil || !strings.Contains(err.Error(), "shadows an existing binding") {
		t.Fatalf("duplicate local error = %v", err)
	}

	nestedInitializer := program
	nestedInitializer.Functions = append([]coreir.Function(nil), program.Functions...)
	function = nestedInitializer.Functions[0]
	outer = *function.Body.ImmutableLocal
	innerExpr = *outer.Return
	outer.Initializer = &innerExpr
	function.Body.ImmutableLocal = &outer
	nestedInitializer.Functions[0] = function
	if err := coreir.ValidateProgram(nestedInitializer); err == nil || !strings.Contains(err.Error(), "ordered immutable-local sequence") {
		t.Fatalf("nested initializer error = %v", err)
	}
}
