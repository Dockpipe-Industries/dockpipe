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

const nestedTerminalInnerLocalSource = `public Class CompilerNestedTerminalLeaf {
	public string Normalize(string raw) => trim(raw);
	public string Select(string raw, bool enabled, bool normalize) {
		if (enabled) {
			string cleaned = trim(raw);
			if (normalize) {
				string normalized = Normalize(cleaned);
				string selected = normalized;
				return selected;
			} else {
				string selected = raw;
				return selected;
			}
		} else {
			return "disabled";
		}
	}
}`

func nestedTerminalInnerLocalProgram(t *testing.T) (*Analysis, SemanticIdentity, coreir.Program) {
	t.Helper()
	input := semanticTestModuleSet("compiler.selfhosting", []ModuleInput{testModule("compiler.selfhosting", "nested-terminal-inner-local.pipe", nestedTerminalInnerLocalSource)}, nil)
	input.LanguageContract = PipeLangLanguageContractV750
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

func TestV750NestedTerminalInnerLocalPipeline(t *testing.T) {
	analysis, identity, program := nestedTerminalInnerLocalProgram(t)
	projection, err := BuildSemanticProjection(analysis)
	if err != nil {
		t.Fatal(err)
	}
	if projection.LanguageContract != PipeLangLanguageContractV750 || projection.Schema != PipeLangSemanticProjectionVersion || projection.CompilerContract != PipeLangCompilerContract {
		t.Fatalf("projection contract = %#v", projection)
	}

	typed, err := LowerSemanticMethodToHIR(analysis, identity)
	if err != nil {
		t.Fatal(err)
	}
	function := hirFunctionNamed(t, typed, "Select")
	outer := function.Body.Conditional
	if typed.LanguageContract != coreir.LanguageContractV750 || outer == nil || outer.WhenTrue == nil || outer.WhenTrue.ImmutableLocal == nil || outer.WhenTrue.ImmutableLocal.Return == nil || outer.WhenTrue.ImmutableLocal.Return.Conditional == nil {
		t.Fatalf("v0.75.0 outer HIR = %#v", function.Body)
	}
	inner := outer.WhenTrue.ImmutableLocal.Return.Conditional
	trueFirst := inner.WhenTrue.ImmutableLocal
	if trueFirst == nil || trueFirst.Binding.Position != 4 || trueFirst.Return == nil || trueFirst.Return.ImmutableLocal == nil || trueFirst.Return.ImmutableLocal.Binding.Position != 5 || trueFirst.Return.ImmutableLocal.Return == nil || trueFirst.Return.ImmutableLocal.Return.Kind != hir.ExprReference {
		t.Fatalf("v0.75.0 true inner-leaf HIR = %#v", inner.WhenTrue)
	}
	falseFirst := inner.WhenFalse.ImmutableLocal
	if falseFirst == nil || falseFirst.Binding.Position != 4 || falseFirst.Return == nil || falseFirst.Return.Kind != hir.ExprReference {
		t.Fatalf("v0.75.0 false inner-leaf HIR = %#v", inner.WhenFalse)
	}

	coreFunction := coreFunctionNamed(t, program, "Select")
	coreOuter := coreFunction.Body.Conditional
	if program.LanguageContract != coreir.LanguageContractV750 || coreOuter == nil || coreOuter.WhenTrue == nil || coreOuter.WhenTrue.ImmutableLocal == nil || coreOuter.WhenTrue.ImmutableLocal.Return == nil || coreOuter.WhenTrue.ImmutableLocal.Return.Conditional == nil {
		t.Fatalf("v0.75.0 nested Core = %#v", coreFunction.Body)
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
	for _, fragment := range []string{"if p1 {", "p3 := pipelangTrimText(p0)", "if p2 {", "p4 := PipeLangNormalize(p3)", "p5 := p4", "return p5", "p4 := p0", "return p4", `return "disabled"`} {
		if !strings.Contains(string(generated), fragment) {
			t.Fatalf("generated Go lacks inner-leaf local fragment %q:\n%s", fragment, generated)
		}
	}
	compileAndRunGeneratedGoFiles(t, generated, []byte(fmt.Sprintf(`package %s

import "testing"

func TestGeneratedNestedTerminalInnerLocals(t *testing.T) {
	if got := PipeLangSelect("  source  ", true, true); got != "source" { t.Fatalf("nested true = %%q", got) }
	if got := PipeLangSelect("  source  ", true, false); got != "  source  " { t.Fatalf("nested false = %%q", got) }
	if got := PipeLangSelect("  source  ", false, true); got != "disabled" { t.Fatalf("outer false = %%q", got) }
}
`, gobackend.PackageName)))
}

func TestV750NestedTerminalInnerLocalGateAndSymmetry(t *testing.T) {
	input := semanticTestModuleSet("compiler.selfhosting", []ModuleInput{testModule("compiler.selfhosting", "nested-terminal-inner-local-gate.pipe", nestedTerminalInnerLocalSource)}, nil)
	input.LanguageContract = PipeLangLanguageContractV740
	if err := AnalyzeSemanticModuleSet(input).Error(); err == nil {
		t.Fatal("v0.74.0 accepted inner terminal branch locals")
	}
	input.LanguageContract = PipeLangLanguageContractV750
	if err := AnalyzeSemanticModuleSet(input).Error(); err != nil {
		t.Fatal(err)
	}

	inherited := semanticTestModuleSet("compiler.selfhosting", []ModuleInput{testModule("compiler.selfhosting", "nested-terminal-if-inherited.pipe", nestedTerminalIfSource)}, nil)
	inherited.LanguageContract = PipeLangLanguageContractV750
	if err := AnalyzeSemanticModuleSet(inherited).Error(); err != nil {
		t.Fatalf("v0.75.0 rejected inherited v0.74.0 source: %v", err)
	}

	falseNested := `public Class Root { public string Select(string raw, bool outer, bool inner) { if (outer) { return raw; } else { string shared = trim(raw); if (inner) { string selected = shared; return selected; } else { return raw; } } } }`
	falseInput := semanticTestModuleSet("compiler.selfhosting", []ModuleInput{testModule("compiler.selfhosting", "false-nested-inner-local.pipe", falseNested)}, nil)
	falseInput.LanguageContract = PipeLangLanguageContractV750
	if err := AnalyzeSemanticModuleSet(falseInput).Error(); err != nil {
		t.Fatalf("v0.75.0 rejected false-branch nesting with an inner local: %v", err)
	}
}

func TestV750NestedTerminalInnerLocalRejectsExcludedSource(t *testing.T) {
	tests := []struct {
		name   string
		source string
	}{
		{name: "nested in both outer branches", source: `public Class Root { public string Select(string raw, bool outer, bool inner) { if (outer) { if (inner) { string selected = raw; return selected; } else { return "a"; } } else { if (inner) { return "b"; } else { return "c"; } } } }`},
		{name: "third nesting level", source: `public Class Root { public string Select(string raw, bool outer, bool middle, bool inner) { if (outer) { if (middle) { string value = raw; if (inner) { return value; } else { return "a"; } } else { return "b"; } } else { return "c"; } } }`},
		{name: "top-level local before nested outer", source: `public Class Root { public string Select(string raw, bool outer, bool inner) { string value = raw; if (outer) { if (inner) { string selected = value; return selected; } else { return raw; } } else { return ""; } } }`},
		{name: "conditional expression in inner local", source: `public Class Root { public string Select(string raw, bool outer, bool inner) { if (outer) { if (inner) { string selected = inner ? raw : ""; return selected; } else { return raw; } } else { return ""; } } }`},
		{name: "propagation in inner local", source: `public Class Root { public Result<string, string> Select(Result<string, string> input, bool outer, bool inner) { if (outer) { if (inner) { string value = propagate(input); return ok<string, string>(value); } else { return input; } } else { return input; } } }`},
		{name: "match in inner local", source: `public Class Root { public string Select(Result<string, string> input, bool outer, bool inner) { if (outer) { if (inner) { string value = match(input){ ok(okValue) => okValue, err(problem) => problem }; return value; } else { return ""; } } else { return ""; } } }`},
		{name: "inner local self reference", source: `public Class Root { public string Select(string raw, bool outer, bool inner) { if (outer) { if (inner) { string selected = selected; return selected; } else { return raw; } } else { return ""; } } }`},
		{name: "inner local forward reference", source: `public Class Root { public string Select(string raw, bool outer, bool inner) { if (outer) { if (inner) { string first = second; string second = raw; return first; } else { return raw; } } else { return ""; } } }`},
		{name: "duplicate inner local", source: `public Class Root { public string Select(string raw, bool outer, bool inner) { if (outer) { if (inner) { string selected = raw; string selected = trim(raw); return selected; } else { return raw; } } else { return ""; } } }`},
		{name: "inner local escapes to sibling leaf", source: `public Class Root { public string Select(string raw, bool outer, bool inner) { if (outer) { if (inner) { string selected = raw; return selected; } else { return selected; } } else { return ""; } } }`},
		{name: "inner local shadows outer local", source: `public Class Root { public string Select(string raw, bool outer, bool inner) { if (outer) { string selected = raw; if (inner) { string selected = trim(raw); return selected; } else { return selected; } } else { return ""; } } }`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			input := semanticTestModuleSet("compiler.selfhosting", []ModuleInput{testModule("compiler.selfhosting", test.name+".pipe", test.source)}, nil)
			input.LanguageContract = PipeLangLanguageContractV750
			if err := AnalyzeSemanticModuleSet(input).Error(); err == nil {
				t.Fatal("excluded v0.75.0 source was accepted")
			}
		})
	}
}

func TestV750NestedTerminalInnerLocalRejectsMalformedCoreAndBackend(t *testing.T) {
	_, _, program := nestedTerminalInnerLocalProgram(t)
	oldContract := program
	oldContract.LanguageContract = coreir.LanguageContractV740
	if err := coreir.ValidateProgram(oldContract); err == nil {
		t.Fatal("v0.74.0 Core accepted inner terminal branch locals")
	}
	if _, err := gobackend.Generate(oldContract); err == nil {
		t.Fatal("Go backend accepted inner terminal branch locals under v0.74.0")
	}

	additional := program
	function := coreFunctionNamed(t, additional, "Select")
	outer := function.Body.Conditional
	nestedBranch := *outer.WhenTrue
	outer.WhenFalse = &nestedBranch
	if err := coreir.ValidateProgram(additional); err == nil || !strings.Contains(err.Error(), "v0.75.0") {
		t.Fatalf("Core accepted nesting in both outer branches: %v", err)
	}
	if _, err := gobackend.Generate(additional); err == nil {
		t.Fatal("Go backend accepted nesting in both outer branches")
	}
}
