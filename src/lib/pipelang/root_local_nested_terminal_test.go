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

const rootLocalNestedTerminalSource = `public Class CompilerRootLocalNestedTerminal {
	public string Normalize(string raw) => trim(raw);
	public string Select(string raw, bool enabled, bool normalize) {
		string shared = raw;
		string prepared = trim(shared);
		if (enabled) {
			string branchValue = prepared;
			if (normalize) {
				string normalized = Normalize(branchValue);
				string selected = normalized;
				return selected;
			} else {
				string selected = shared;
				return selected;
			}
		} else {
			string disabled = "disabled";
			return disabled;
		}
	}
}`

func rootLocalNestedTerminalProgram(t *testing.T) (*Analysis, SemanticIdentity, coreir.Program) {
	t.Helper()
	input := semanticTestModuleSet("compiler.selfhosting", []ModuleInput{testModule("compiler.selfhosting", "root-local-nested-terminal.pipe", rootLocalNestedTerminalSource)}, nil)
	input.LanguageContract = PipeLangLanguageContractV760
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

func TestV760RootLocalNestedTerminalPipeline(t *testing.T) {
	analysis, identity, program := rootLocalNestedTerminalProgram(t)
	projection, err := BuildSemanticProjection(analysis)
	if err != nil {
		t.Fatal(err)
	}
	if projection.LanguageContract != PipeLangLanguageContractV760 || projection.Schema != PipeLangSemanticProjectionVersion || projection.CompilerContract != PipeLangCompilerContract {
		t.Fatalf("projection contract = %#v", projection)
	}

	typed, err := LowerSemanticMethodToHIR(analysis, identity)
	if err != nil {
		t.Fatal(err)
	}
	function := hirFunctionNamed(t, typed, "Select")
	first := function.Body.ImmutableLocal
	if typed.LanguageContract != coreir.LanguageContractV760 || function.Body.Kind != hir.ExprImmutableLocal || first == nil || first.Binding.Position != 3 || first.Return == nil || first.Return.ImmutableLocal == nil || first.Return.ImmutableLocal.Binding.Position != 4 {
		t.Fatalf("v0.76.0 root HIR locals = %#v", function.Body)
	}
	outer := first.Return.ImmutableLocal.Return.Conditional
	if outer == nil || !outer.TerminalStatement || outer.WhenTrue == nil || outer.WhenTrue.ImmutableLocal == nil || outer.WhenTrue.ImmutableLocal.Binding.Position != 5 || outer.WhenFalse == nil || outer.WhenFalse.ImmutableLocal == nil || outer.WhenFalse.ImmutableLocal.Binding.Position != 5 {
		t.Fatalf("v0.76.0 outer HIR = %#v", first.Return.ImmutableLocal.Return)
	}
	inner := outer.WhenTrue.ImmutableLocal.Return.Conditional
	if inner == nil || !inner.TerminalStatement || inner.WhenTrue == nil || inner.WhenTrue.ImmutableLocal == nil || inner.WhenTrue.ImmutableLocal.Binding.Position != 6 || inner.WhenTrue.ImmutableLocal.Return == nil || inner.WhenTrue.ImmutableLocal.Return.ImmutableLocal == nil || inner.WhenTrue.ImmutableLocal.Return.ImmutableLocal.Binding.Position != 7 || inner.WhenFalse == nil || inner.WhenFalse.ImmutableLocal == nil || inner.WhenFalse.ImmutableLocal.Binding.Position != 6 {
		t.Fatalf("v0.76.0 inner HIR = %#v", outer.WhenTrue)
	}

	coreFunction := coreFunctionNamed(t, program, "Select")
	if program.LanguageContract != coreir.LanguageContractV760 || coreFunction.Body.Kind != coreir.ExprImmutableLocal || coreFunction.Body.ImmutableLocal == nil || coreFunction.Body.ImmutableLocal.Return == nil || coreFunction.Body.ImmutableLocal.Return.ImmutableLocal == nil || coreFunction.Body.ImmutableLocal.Return.ImmutableLocal.Return == nil || coreFunction.Body.ImmutableLocal.Return.ImmutableLocal.Return.Conditional == nil {
		t.Fatalf("v0.76.0 Core = %#v", coreFunction.Body)
	}
	if err := coreir.ValidateProgram(program); err != nil {
		t.Fatal(err)
	}

	entry := coreir.SemanticIdentity{PackageID: string(identity.PackageID), Path: string(identity.Path)}
	stringType := coreFunction.Parameters[0].Type
	boolType := coreFunction.Parameters[1].Type
	for _, test := range []struct {
		raw                string
		enabled, normalize bool
		want               string
	}{{"  source  ", true, true, "source"}, {"  source  ", true, false, "  source  "}, {"  source  ", false, true, "disabled"}} {
		outcome, err := coreeval.EvaluateProgram(program, entry, []coreeval.Value{{Type: stringType, String: test.raw}, {Type: boolType, Bool: test.enabled}, {Type: boolType, Bool: test.normalize}})
		if err != nil || !outcome.OK || outcome.Value.String != test.want {
			t.Fatalf("Select(%q, %t, %t) = %#v, %v", test.raw, test.enabled, test.normalize, outcome, err)
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
	for _, fragment := range []string{"p3 := p0", "p4 := pipelangTrimText(p3)", "if p1 {", "p5 := p4", "if p2 {", "p6 := PipeLangNormalize(p5)", "p7 := p6", "return p7", "p6 := p3", `p5 := "disabled"`} {
		if !strings.Contains(string(generated), fragment) {
			t.Fatalf("generated Go lacks root-local nested-terminal fragment %q:\n%s", fragment, generated)
		}
	}
	compileAndRunGeneratedGoFiles(t, generated, []byte(fmt.Sprintf(`package %s

import "testing"

func TestGeneratedRootLocalNestedTerminal(t *testing.T) {
	if got := PipeLangSelect("  source  ", true, true); got != "source" { t.Fatalf("nested true = %%q", got) }
	if got := PipeLangSelect("  source  ", true, false); got != "  source  " { t.Fatalf("nested false = %%q", got) }
	if got := PipeLangSelect("  source  ", false, true); got != "disabled" { t.Fatalf("outer false = %%q", got) }
}
`, gobackend.PackageName)))
}

func TestV760RootLocalNestedTerminalGateAndInheritance(t *testing.T) {
	input := semanticTestModuleSet("compiler.selfhosting", []ModuleInput{testModule("compiler.selfhosting", "root-local-nested-terminal-gate.pipe", rootLocalNestedTerminalSource)}, nil)
	input.LanguageContract = PipeLangLanguageContractV750
	if err := AnalyzeSemanticModuleSet(input).Error(); err == nil {
		t.Fatal("v0.75.0 accepted top-level locals before the nested terminal topology")
	}
	input.LanguageContract = PipeLangLanguageContractV760
	if err := AnalyzeSemanticModuleSet(input).Error(); err != nil {
		t.Fatal(err)
	}

	conditionInput := semanticTestModuleSet("compiler.selfhosting", []ModuleInput{testModule("compiler.selfhosting", "root-local-outer-condition.pipe", `public Class Root { public string Select(string raw, bool enabled, bool inner) { bool active = enabled; if (active) { if (inner) { return raw; } else { return "fallback"; } } else { return "disabled"; } } }`)}, nil)
	conditionInput.LanguageContract = PipeLangLanguageContractV760
	if err := AnalyzeSemanticModuleSet(conditionInput).Error(); err != nil {
		t.Fatalf("v0.76.0 root local was not visible to the outer condition: %v", err)
	}

	inherited := semanticTestModuleSet("compiler.selfhosting", []ModuleInput{testModule("compiler.selfhosting", "nested-terminal-inner-local-inherited.pipe", nestedTerminalInnerLocalSource)}, nil)
	inherited.LanguageContract = PipeLangLanguageContractV760
	if err := AnalyzeSemanticModuleSet(inherited).Error(); err != nil {
		t.Fatalf("v0.76.0 rejected inherited v0.75.0 source: %v", err)
	}
}

func TestV760RootLocalNestedTerminalRejectsExcludedSource(t *testing.T) {
	tests := []struct {
		name   string
		source string
	}{
		{name: "nested in both outer branches", source: `public Class Root { public string Select(string raw, bool outer, bool inner) { string shared = raw; if (outer) { if (inner) { return shared; } else { return "a"; } } else { if (inner) { return "b"; } else { return "c"; } } } }`},
		{name: "third nesting level", source: `public Class Root { public string Select(string raw, bool outer, bool middle, bool inner) { string shared = raw; if (outer) { if (middle) { if (inner) { return shared; } else { return "a"; } } else { return "b"; } } else { return "c"; } } }`},
		{name: "conditional expression in root local", source: `public Class Root { public string Select(string raw, bool outer, bool inner) { string shared = outer ? raw : ""; if (outer) { if (inner) { return shared; } else { return raw; } } else { return ""; } } }`},
		{name: "propagation in root local", source: `public Class Root { public Result<string, string> Select(Result<string, string> input, bool outer, bool inner) { string value = propagate(input); if (outer) { if (inner) { return ok<string, string>(value); } else { return input; } } else { return input; } } }`},
		{name: "match in root local", source: `public Class Root { public string Select(Result<string, string> input, bool outer, bool inner) { string value = match(input){ ok(okValue) => okValue, err(problem) => problem }; if (outer) { if (inner) { return value; } else { return ""; } } else { return ""; } } }`},
		{name: "root local self reference", source: `public Class Root { public string Select(string raw, bool outer, bool inner) { string shared = shared; if (outer) { if (inner) { return shared; } else { return raw; } } else { return ""; } } }`},
		{name: "root local shadows parameter", source: `public Class Root { public string Select(string raw, bool outer, bool inner) { string raw = ""; if (outer) { if (inner) { return raw; } else { return "a"; } } else { return "b"; } } }`},
		{name: "root local forward reference", source: `public Class Root { public string Select(string raw, bool outer, bool inner) { string first = second; string second = raw; if (outer) { if (inner) { return first; } else { return second; } } else { return raw; } } }`},
		{name: "duplicate root local", source: `public Class Root { public string Select(string raw, bool outer, bool inner) { string shared = raw; string shared = trim(raw); if (outer) { if (inner) { return shared; } else { return raw; } } else { return ""; } } }`},
		{name: "branch local shadows root local", source: `public Class Root { public string Select(string raw, bool outer, bool inner) { string shared = raw; if (outer) { string shared = trim(raw); if (inner) { return shared; } else { return raw; } } else { return shared; } } }`},
		{name: "outer local escapes to sibling", source: `public Class Root { public string Select(string raw, bool outer, bool inner) { string shared = raw; if (outer) { string selected = shared; if (inner) { return selected; } else { return raw; } } else { return selected; } } }`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			input := semanticTestModuleSet("compiler.selfhosting", []ModuleInput{testModule("compiler.selfhosting", test.name+".pipe", test.source)}, nil)
			input.LanguageContract = PipeLangLanguageContractV760
			if err := AnalyzeSemanticModuleSet(input).Error(); err == nil {
				t.Fatal("excluded v0.76.0 source was accepted")
			}
		})
	}
}

func TestV760RootLocalNestedTerminalRejectsMalformedCoreAndBackend(t *testing.T) {
	_, _, program := rootLocalNestedTerminalProgram(t)
	oldContract := program
	oldContract.LanguageContract = coreir.LanguageContractV750
	if err := coreir.ValidateProgram(oldContract); err == nil {
		t.Fatal("v0.75.0 Core accepted top-level locals before nested terminal branching")
	}
	if _, err := gobackend.Generate(oldContract); err == nil {
		t.Fatal("Go backend accepted root-local nested branching under v0.75.0")
	}

	_, _, inherited := nestedTerminalInnerLocalProgram(t)
	inherited.LanguageContract = coreir.LanguageContractV760
	if err := coreir.ValidateProgram(inherited); err != nil {
		t.Fatalf("v0.76.0 rejected inherited rootless nested Core: %v", err)
	}

	_, _, program = rootLocalNestedTerminalProgram(t)
	additional := program
	function := coreFunctionNamed(t, additional, "Select")
	outer := function.Body.ImmutableLocal.Return.ImmutableLocal.Return.Conditional
	nestedBranch := *outer.WhenTrue
	outer.WhenFalse = &nestedBranch
	if err := coreir.ValidateProgram(additional); err == nil || !strings.Contains(err.Error(), "v0.76.0") {
		t.Fatalf("Core accepted root-local nesting in both outer branches: %v", err)
	}
	if _, err := gobackend.Generate(additional); err == nil {
		t.Fatal("Go backend accepted root-local nesting in both outer branches")
	}
}
