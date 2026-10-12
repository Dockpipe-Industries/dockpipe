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

const symmetricNestedTerminalSource = `public Class CompilerSymmetricNestedTerminal {
	public string Normalize(string raw) => trim(raw);
	public string Select(string raw, bool outer, bool left, bool right) {
		string shared = raw;
		string prepared = trim(shared);
		if (outer) {
			string trueValue = shared;
			if (left) {
				string selected = trueValue;
				return selected;
			} else {
				string selected = "outer-true";
				return selected;
			}
		} else {
			string falseValue = prepared;
			if (right) {
				string selected = Normalize(falseValue);
				return selected;
			} else {
				string selected = "outer-false";
				return selected;
			}
		}
	}
}`

func symmetricNestedTerminalProgram(t *testing.T) (*Analysis, SemanticIdentity, coreir.Program) {
	t.Helper()
	input := semanticTestModuleSet("compiler.selfhosting", []ModuleInput{testModule("compiler.selfhosting", "symmetric-nested-terminal.pipe", symmetricNestedTerminalSource)}, nil)
	input.LanguageContract = PipeLangLanguageContractV770
	analysis := AnalyzeSemanticModuleSet(input)
	if err := analysis.Error(); err != nil {
		t.Fatal(err)
	}
	identity := semanticMethodNamed(t, analysis, "Select").Identity
	typed, err := LowerSemanticMethodToHIR(analysis, identity)
	if err != nil {
		t.Fatal(err)
	}
	program, err := LowerHIRToCore(typed)
	if err != nil {
		t.Fatal(err)
	}
	return analysis, identity, program
}

func TestV770SymmetricNestedTerminalPipeline(t *testing.T) {
	analysis, identity, program := symmetricNestedTerminalProgram(t)
	projection, err := BuildSemanticProjection(analysis)
	if err != nil {
		t.Fatal(err)
	}
	if projection.LanguageContract != PipeLangLanguageContractV770 || projection.Schema != PipeLangSemanticProjectionVersion || projection.CompilerContract != PipeLangCompilerContract {
		t.Fatalf("projection contract = %#v", projection)
	}

	typed, err := LowerSemanticMethodToHIR(analysis, identity)
	if err != nil {
		t.Fatal(err)
	}
	function := hirFunctionNamed(t, typed, "Select")
	first := function.Body.ImmutableLocal
	if typed.LanguageContract != coreir.LanguageContractV770 || function.Body.Kind != hir.ExprImmutableLocal || first == nil || first.Binding.Position != 4 || first.Return == nil || first.Return.ImmutableLocal == nil || first.Return.ImmutableLocal.Binding.Position != 5 {
		t.Fatalf("v0.77.0 root HIR locals = %#v", function.Body)
	}
	outer := first.Return.ImmutableLocal.Return.Conditional
	if outer == nil || !outer.TerminalStatement || outer.WhenTrue == nil || outer.WhenTrue.ImmutableLocal == nil || outer.WhenFalse == nil || outer.WhenFalse.ImmutableLocal == nil {
		t.Fatalf("v0.77.0 outer HIR = %#v", first.Return.ImmutableLocal.Return)
	}
	trueInner := outer.WhenTrue.ImmutableLocal.Return.Conditional
	falseInner := outer.WhenFalse.ImmutableLocal.Return.Conditional
	if trueInner == nil || falseInner == nil || !trueInner.TerminalStatement || !falseInner.TerminalStatement || trueInner.WhenTrue == nil || trueInner.WhenTrue.ImmutableLocal == nil || trueInner.WhenTrue.ImmutableLocal.Binding.Position != 7 || falseInner.WhenFalse == nil || falseInner.WhenFalse.ImmutableLocal == nil || falseInner.WhenFalse.ImmutableLocal.Binding.Position != 7 {
		t.Fatalf("v0.77.0 symmetric inner HIR = true=%#v false=%#v", trueInner, falseInner)
	}

	coreFunction := coreFunctionNamed(t, program, "Select")
	if program.LanguageContract != coreir.LanguageContractV770 || coreFunction.Body.Kind != coreir.ExprImmutableLocal || coreFunction.Body.ImmutableLocal == nil || coreFunction.Body.ImmutableLocal.Return == nil || coreFunction.Body.ImmutableLocal.Return.ImmutableLocal == nil {
		t.Fatalf("v0.77.0 Core = %#v", coreFunction.Body)
	}
	if err := coreir.ValidateProgram(program); err != nil {
		t.Fatal(err)
	}

	entry := coreir.SemanticIdentity{PackageID: string(identity.PackageID), Path: string(identity.Path)}
	stringType := coreFunction.Parameters[0].Type
	boolType := coreFunction.Parameters[1].Type
	for _, test := range []struct {
		raw                string
		outer, left, right bool
		want               string
	}{
		{"  source  ", true, true, false, "  source  "},
		{"  source  ", true, false, true, "outer-true"},
		{"  source  ", false, false, true, "source"},
		{"  source  ", false, true, false, "outer-false"},
	} {
		outcome, err := coreeval.EvaluateProgram(program, entry, []coreeval.Value{{Type: stringType, String: test.raw}, {Type: boolType, Bool: test.outer}, {Type: boolType, Bool: test.left}, {Type: boolType, Bool: test.right}})
		if err != nil || !outcome.OK || outcome.Value.String != test.want {
			t.Fatalf("Select(%q, %t, %t, %t) = %#v, %v", test.raw, test.outer, test.left, test.right, outcome, err)
		}
	}

	generated, err := gobackend.Generate(program)
	if err != nil {
		t.Fatal(err)
	}
	again, err := gobackend.Generate(program)
	if err != nil || string(again) != string(generated) {
		t.Fatalf("generated Go is not deterministic: %v", err)
	}
	for _, fragment := range []string{"p4 := p0", "p5 := pipelangTrimText(p4)", "if p1 {", "p6 := p4", "if p2 {", "p7 := p6", `p7 := "outer-true"`, "p6 := p5", "if p3 {", "p7 := PipeLangNormalize(p6)", `p7 := "outer-false"`} {
		if !strings.Contains(string(generated), fragment) {
			t.Fatalf("generated Go lacks symmetric nested-terminal fragment %q:\n%s", fragment, generated)
		}
	}
	compileAndRunGeneratedGoFiles(t, generated, []byte(fmt.Sprintf(`package %s

import "testing"

func TestGeneratedSymmetricNestedTerminal(t *testing.T) {
	if got := PipeLangSelect("  source  ", true, true, false); got != "  source  " { t.Fatalf("outer true, inner true = %%q", got) }
	if got := PipeLangSelect("  source  ", true, false, true); got != "outer-true" { t.Fatalf("outer true, inner false = %%q", got) }
	if got := PipeLangSelect("  source  ", false, false, true); got != "source" { t.Fatalf("outer false, inner true = %%q", got) }
	if got := PipeLangSelect("  source  ", false, true, false); got != "outer-false" { t.Fatalf("outer false, inner false = %%q", got) }
}
`, gobackend.PackageName)))
}

func TestV770SymmetricNestedTerminalGateAndInheritance(t *testing.T) {
	input := semanticTestModuleSet("compiler.selfhosting", []ModuleInput{testModule("compiler.selfhosting", "symmetric-nested-terminal-gate.pipe", symmetricNestedTerminalSource)}, nil)
	input.LanguageContract = PipeLangLanguageContractV760
	if err := AnalyzeSemanticModuleSet(input).Error(); err == nil {
		t.Fatal("v0.76.0 accepted symmetric nested terminal branching")
	}
	input.LanguageContract = PipeLangLanguageContractV770
	if err := AnalyzeSemanticModuleSet(input).Error(); err != nil {
		t.Fatal(err)
	}

	inherited := semanticTestModuleSet("compiler.selfhosting", []ModuleInput{testModule("compiler.selfhosting", "root-local-nested-terminal-inherited.pipe", rootLocalNestedTerminalSource)}, nil)
	inherited.LanguageContract = PipeLangLanguageContractV770
	if err := AnalyzeSemanticModuleSet(inherited).Error(); err != nil {
		t.Fatalf("v0.77.0 rejected inherited v0.76.0 source: %v", err)
	}
}

func TestV770SymmetricNestedTerminalRejectsExcludedSource(t *testing.T) {
	tests := []struct {
		name   string
		source string
	}{
		{name: "rootless symmetric topology", source: `public Class Root { public string Select(string raw, bool outer, bool left, bool right) { if (outer) { if (left) { return raw; } else { return "a"; } } else { if (right) { return "b"; } else { return "c"; } } } }`},
		{name: "third nesting level", source: `public Class Root { public string Select(string raw, bool outer, bool left, bool deep, bool right) { string shared = raw; if (outer) { if (left) { if (deep) { return shared; } else { return "a"; } } else { return "b"; } } else { if (right) { return "c"; } else { return "d"; } } } }`},
		{name: "conditional expression in root local", source: `public Class Root { public string Select(string raw, bool outer, bool left, bool right) { string shared = outer ? raw : ""; if (outer) { if (left) { return shared; } else { return "a"; } } else { if (right) { return "b"; } else { return "c"; } } } }`},
		{name: "propagation in root local", source: `public Class Root { public Result<string, string> Select(Result<string, string> input, bool outer, bool left, bool right) { string value = propagate(input); if (outer) { if (left) { return ok<string, string>(value); } else { return input; } } else { if (right) { return input; } else { return input; } } } }`},
		{name: "match in root local", source: `public Class Root { public string Select(Result<string, string> input, bool outer, bool left, bool right) { string value = match(input){ ok(okValue) => okValue, err(problem) => problem }; if (outer) { if (left) { return value; } else { return "a"; } } else { if (right) { return "b"; } else { return "c"; } } } }`},
		{name: "outer local escapes to sibling", source: `public Class Root { public string Select(string raw, bool outer, bool left, bool right) { string shared = raw; if (outer) { string selected = shared; if (left) { return selected; } else { return raw; } } else { if (right) { return selected; } else { return "c"; } } } }`},
		{name: "inner leaf shadows root", source: `public Class Root { public string Select(string raw, bool outer, bool left, bool right) { string shared = raw; if (outer) { if (left) { string shared = trim(raw); return shared; } else { return raw; } } else { if (right) { return raw; } else { return shared; } } } }`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			input := semanticTestModuleSet("compiler.selfhosting", []ModuleInput{testModule("compiler.selfhosting", test.name+".pipe", test.source)}, nil)
			input.LanguageContract = PipeLangLanguageContractV770
			if err := AnalyzeSemanticModuleSet(input).Error(); err == nil {
				t.Fatal("excluded v0.77.0 source was accepted")
			}
		})
	}
}

func TestV770SymmetricNestedTerminalRejectsMalformedCoreAndBackend(t *testing.T) {
	_, _, program := symmetricNestedTerminalProgram(t)
	oldContract := program
	oldContract.LanguageContract = coreir.LanguageContractV760
	if err := coreir.ValidateProgram(oldContract); err == nil {
		t.Fatal("v0.76.0 Core accepted symmetric nested terminal branching")
	}
	if _, err := gobackend.Generate(oldContract); err == nil {
		t.Fatal("Go backend accepted symmetric branching under v0.76.0")
	}

	rootless := program
	for index := range rootless.Functions {
		if rootless.Functions[index].Name == "Select" {
			rootless.Functions[index].Body = *rootless.Functions[index].Body.ImmutableLocal.Return.ImmutableLocal.Return
		}
	}
	if err := coreir.ValidateProgram(rootless); err == nil || !strings.Contains(err.Error(), "v0.77.0") {
		t.Fatalf("Core accepted rootless symmetric nested branching: %v", err)
	}
	if _, err := gobackend.Generate(rootless); err == nil {
		t.Fatal("Go backend accepted rootless symmetric nested branching")
	}

	_, _, tooDeep := symmetricNestedTerminalProgram(t)
	for index := range tooDeep.Functions {
		if tooDeep.Functions[index].Name != "Select" {
			continue
		}
		outer := tooDeep.Functions[index].Body.ImmutableLocal.Return.ImmutableLocal.Return.Conditional
		inner := outer.WhenTrue.ImmutableLocal.Return.Conditional
		leaf := *inner.WhenTrue
		inner.WhenTrue = &coreir.Expr{
			Kind: coreir.ExprConditional,
			Type: leaf.Type,
			Conditional: &coreir.Conditional{
				Condition:         inner.Condition,
				WhenTrue:          &leaf,
				WhenFalse:         &leaf,
				TerminalStatement: true,
			},
		}
	}
	if err := coreir.ValidateProgram(tooDeep); err == nil || !strings.Contains(err.Error(), "v0.77.0") {
		t.Fatalf("Core accepted third-level terminal branching: %v", err)
	}
	if _, err := gobackend.Generate(tooDeep); err == nil {
		t.Fatal("Go backend accepted third-level terminal branching")
	}
}
