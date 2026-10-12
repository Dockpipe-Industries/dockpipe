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

const nestedTerminalIfSource = `public Class CompilerNestedTerminalBranch {
	public string Normalize(string raw) => trim(raw);
	public string Select(string raw, bool enabled, bool normalize) {
		if (enabled) {
			string cleaned = trim(raw);
			if (normalize) {
				return Normalize(cleaned);
			} else {
				return raw;
			}
		} else {
			return "disabled";
		}
	}
}`

func nestedTerminalIfProgram(t *testing.T) (*Analysis, SemanticIdentity, coreir.Program) {
	t.Helper()
	input := semanticTestModuleSet("compiler.selfhosting", []ModuleInput{testModule("compiler.selfhosting", "nested-terminal-if.pipe", nestedTerminalIfSource)}, nil)
	input.LanguageContract = PipeLangLanguageContractV740
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

func TestV740NestedTerminalIfPipeline(t *testing.T) {
	analysis, identity, program := nestedTerminalIfProgram(t)
	projection, err := BuildSemanticProjection(analysis)
	if err != nil {
		t.Fatal(err)
	}
	if projection.LanguageContract != PipeLangLanguageContractV740 || projection.Schema != PipeLangSemanticProjectionVersion || projection.CompilerContract != PipeLangCompilerContract {
		t.Fatalf("projection contract = %#v", projection)
	}

	typed, err := LowerSemanticMethodToHIR(analysis, identity)
	if err != nil {
		t.Fatal(err)
	}
	function := hirFunctionNamed(t, typed, "Select")
	outer := function.Body.Conditional
	if typed.LanguageContract != coreir.LanguageContractV740 || function.Body.Kind != hir.ExprConditional || outer == nil || !outer.TerminalStatement || outer.WhenTrue == nil || outer.WhenFalse == nil {
		t.Fatalf("v0.74.0 outer HIR = %#v", function.Body)
	}
	outerLocal := outer.WhenTrue.ImmutableLocal
	if outerLocal == nil || outerLocal.Binding.Position != 3 || outerLocal.Return == nil || outerLocal.Return.Kind != hir.ExprConditional || outerLocal.Return.Conditional == nil || !outerLocal.Return.Conditional.TerminalStatement {
		t.Fatalf("v0.74.0 nested HIR = %#v", outer.WhenTrue)
	}
	inner := outerLocal.Return.Conditional
	if inner.WhenTrue == nil || inner.WhenTrue.Kind != hir.ExprCall || inner.WhenFalse == nil || inner.WhenFalse.Kind != hir.ExprReference || inner.WhenFalse.Reference == nil || inner.WhenFalse.Reference.Position != 0 {
		t.Fatalf("v0.74.0 direct inner leaves = %#v", inner)
	}

	coreFunction := coreFunctionNamed(t, program, "Select")
	coreOuter := coreFunction.Body.Conditional
	if program.LanguageContract != coreir.LanguageContractV740 || coreOuter == nil || !coreOuter.TerminalStatement || coreOuter.WhenTrue == nil || coreOuter.WhenTrue.ImmutableLocal == nil || coreOuter.WhenTrue.ImmutableLocal.Return == nil || coreOuter.WhenTrue.ImmutableLocal.Return.Conditional == nil || !coreOuter.WhenTrue.ImmutableLocal.Return.Conditional.TerminalStatement {
		t.Fatalf("v0.74.0 nested Core = %#v", coreFunction.Body)
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
	for _, fragment := range []string{"if p1 {", "p3 := pipelangTrimText(p0)", "if p2 {", "return PipeLangNormalize(p3)", "return p0", `return "disabled"`} {
		if !strings.Contains(string(generated), fragment) {
			t.Fatalf("generated Go lacks nested terminal fragment %q:\n%s", fragment, generated)
		}
	}
	compileAndRunGeneratedGoFiles(t, generated, []byte(fmt.Sprintf(`package %s

import "testing"

func TestGeneratedNestedTerminalIf(t *testing.T) {
	if got := PipeLangSelect("  source  ", true, true); got != "source" { t.Fatalf("nested true = %%q", got) }
	if got := PipeLangSelect("  source  ", true, false); got != "  source  " { t.Fatalf("nested false = %%q", got) }
	if got := PipeLangSelect("  source  ", false, true); got != "disabled" { t.Fatalf("outer false = %%q", got) }
}
`, gobackend.PackageName)))
}

func TestV740NestedTerminalIfGateAndSymmetry(t *testing.T) {
	input := semanticTestModuleSet("compiler.selfhosting", []ModuleInput{testModule("compiler.selfhosting", "nested-terminal-if-gate.pipe", nestedTerminalIfSource)}, nil)
	input.LanguageContract = PipeLangLanguageContractV730
	if err := AnalyzeSemanticModuleSet(input).Error(); err == nil {
		t.Fatal("v0.73.0 accepted nested terminal if/else")
	}
	input.LanguageContract = PipeLangLanguageContractV740
	if err := AnalyzeSemanticModuleSet(input).Error(); err != nil {
		t.Fatal(err)
	}

	falseNested := `public Class Root { public string Select(string raw, bool outer, bool inner) { if (outer) { return raw; } else { if (inner) { return trim(raw); } else { return raw; } } } }`
	falseInput := semanticTestModuleSet("compiler.selfhosting", []ModuleInput{testModule("compiler.selfhosting", "false-nested-terminal-if.pipe", falseNested)}, nil)
	falseInput.LanguageContract = PipeLangLanguageContractV740
	if err := AnalyzeSemanticModuleSet(falseInput).Error(); err != nil {
		t.Fatalf("v0.74.0 rejected false-branch nesting: %v", err)
	}

	inherited := semanticTestModuleSet("compiler.selfhosting", []ModuleInput{testModule("compiler.selfhosting", "direct-terminal-if-inherited.pipe", terminalIfWithoutTopLevelLocalSource)}, nil)
	inherited.LanguageContract = PipeLangLanguageContractV740
	if err := AnalyzeSemanticModuleSet(inherited).Error(); err != nil {
		t.Fatalf("v0.74.0 rejected inherited v0.73.0 source: %v", err)
	}
}

func TestV740NestedTerminalIfRejectsExcludedSource(t *testing.T) {
	tests := []struct {
		name   string
		source string
	}{
		{name: "nested in both outer branches", source: `public Class Root { public string Select(string raw, bool outer, bool inner) { if (outer) { if (inner) { return raw; } else { return "a"; } } else { if (inner) { return "b"; } else { return "c"; } } } }`},
		{name: "inner branch local", source: `public Class Root { public string Select(string raw, bool outer, bool inner) { if (outer) { if (inner) { string selected = raw; return selected; } else { return raw; } } else { return ""; } } }`},
		{name: "third nesting level", source: `public Class Root { public string Select(string raw, bool outer, bool middle, bool inner) { if (outer) { if (middle) { if (inner) { return raw; } else { return "a"; } } else { return "b"; } } else { return "c"; } } }`},
		{name: "top-level local before nested outer", source: `public Class Root { public string Select(string raw, bool outer, bool inner) { string value = raw; if (outer) { if (inner) { return value; } else { return raw; } } else { return ""; } } }`},
		{name: "conditional expression in outer local", source: `public Class Root { public string Select(string raw, bool outer, bool inner) { if (outer) { string value = inner ? raw : ""; if (inner) { return value; } else { return raw; } } else { return ""; } } }`},
		{name: "propagation in outer branch", source: `public Class Root { public Result<string, string> Select(Result<string, string> input, bool outer, bool inner) { if (outer) { string value = propagate(input); if (inner) { return ok<string, string>(value); } else { return input; } } else { return input; } } }`},
		{name: "match in outer branch", source: `public Class Root { public string Select(Result<string, string> input, bool outer, bool inner) { if (outer) { string value = match(input){ ok(okValue) => okValue, err(problem) => problem }; if (inner) { return value; } else { return ""; } } else { return ""; } } }`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			input := semanticTestModuleSet("compiler.selfhosting", []ModuleInput{testModule("compiler.selfhosting", test.name+".pipe", test.source)}, nil)
			input.LanguageContract = PipeLangLanguageContractV740
			if err := AnalyzeSemanticModuleSet(input).Error(); err == nil {
				t.Fatal("excluded v0.74.0 source was accepted")
			}
		})
	}
}

func TestV740NestedTerminalIfRejectsMalformedCoreAndBackend(t *testing.T) {
	_, _, program := nestedTerminalIfProgram(t)
	oldContract := program
	oldContract.LanguageContract = coreir.LanguageContractV730
	if err := coreir.ValidateProgram(oldContract); err == nil {
		t.Fatal("v0.73.0 Core accepted nested terminal if/else")
	}
	if _, err := gobackend.Generate(oldContract); err == nil {
		t.Fatal("Go backend accepted nested terminal if/else under v0.73.0")
	}

	additional := program
	function := coreFunctionNamed(t, additional, "Select")
	outer := function.Body.Conditional
	nested := outer.WhenTrue.ImmutableLocal.Return
	ordinary := outer.WhenFalse
	sibling := *nested
	siblingConditional := *nested.Conditional
	siblingConditional.WhenTrue = ordinary
	siblingConditional.WhenFalse = ordinary
	sibling.Conditional = &siblingConditional
	outer.WhenFalse = &sibling
	if err := coreir.ValidateProgram(additional); err == nil || !strings.Contains(err.Error(), "v0.74.0") {
		t.Fatalf("Core accepted nesting in both outer branches: %v", err)
	}
	if _, err := gobackend.Generate(additional); err == nil {
		t.Fatal("Go backend accepted nesting in both outer branches")
	}
}
