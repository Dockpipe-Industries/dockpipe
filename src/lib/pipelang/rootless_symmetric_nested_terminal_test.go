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

const rootlessSymmetricNestedTerminalSource = `public Class CompilerRootlessSymmetricNestedTerminal {
	public string Normalize(string raw) => trim(raw);
	public string Select(string raw, bool outer, bool left, bool right) {
		if (outer) {
			string trueValue = raw;
			if (left) {
				string selected = trueValue;
				return selected;
			} else {
				string selected = "outer-true";
				return selected;
			}
		} else {
			string falseValue = trim(raw);
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

func rootlessSymmetricNestedTerminalProgram(t *testing.T) (*Analysis, SemanticIdentity, coreir.Program) {
	t.Helper()
	input := semanticTestModuleSet("compiler.selfhosting", []ModuleInput{testModule("compiler.selfhosting", "symmetric-nested-terminal.pipe", rootlessSymmetricNestedTerminalSource)}, nil)
	input.LanguageContract = PipeLangLanguageContractV780
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

func TestV780RootlessSymmetricNestedTerminalPipeline(t *testing.T) {
	analysis, identity, program := rootlessSymmetricNestedTerminalProgram(t)
	projection, err := BuildSemanticProjection(analysis)
	if err != nil {
		t.Fatal(err)
	}
	if projection.LanguageContract != PipeLangLanguageContractV780 || projection.Schema != PipeLangSemanticProjectionVersion || projection.CompilerContract != PipeLangCompilerContract {
		t.Fatalf("projection contract = %#v", projection)
	}

	typed, err := LowerSemanticMethodToHIR(analysis, identity)
	if err != nil {
		t.Fatal(err)
	}
	function := hirFunctionNamed(t, typed, "Select")
	outer := function.Body.Conditional
	if typed.LanguageContract != coreir.LanguageContractV780 || function.Body.Kind != hir.ExprConditional || outer == nil || !outer.TerminalStatement {
		t.Fatalf("rootless outer HIR = %#v", function.Body)
	}
	trueInner := outer.WhenTrue.ImmutableLocal.Return.Conditional
	falseInner := outer.WhenFalse.ImmutableLocal.Return.Conditional
	if trueInner == nil || falseInner == nil || !trueInner.TerminalStatement || !falseInner.TerminalStatement || trueInner.WhenTrue.ImmutableLocal.Binding.Position != 5 || falseInner.WhenFalse.ImmutableLocal.Binding.Position != 5 {
		t.Fatalf("rootless branch-local HIR positions are invalid")
	}
	coreFunction := coreFunctionNamed(t, program, "Select")
	if program.LanguageContract != coreir.LanguageContractV780 || coreFunction.Body.Kind != coreir.ExprConditional || coreFunction.Body.Conditional == nil {
		t.Fatalf("rootless Core = %#v", coreFunction.Body)
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
	for _, fragment := range []string{"if p1 {", "p4 := p0", "if p2 {", "p5 := p4", `p5 := "outer-true"`, "p4 := pipelangTrimText(p0)", "if p3 {", "p5 := PipeLangNormalize(p4)", `p5 := "outer-false"`} {
		if !strings.Contains(string(generated), fragment) {
			t.Fatalf("generated Go lacks symmetric nested-terminal fragment %q:\n%s", fragment, generated)
		}
	}
	compileAndRunGeneratedGoFiles(t, generated, []byte(fmt.Sprintf(`package %s

import "testing"

func TestGeneratedRootlessSymmetricNestedTerminal(t *testing.T) {
	if got := PipeLangSelect("  source  ", true, true, false); got != "  source  " { t.Fatalf("outer true, inner true = %%q", got) }
	if got := PipeLangSelect("  source  ", true, false, true); got != "outer-true" { t.Fatalf("outer true, inner false = %%q", got) }
	if got := PipeLangSelect("  source  ", false, false, true); got != "source" { t.Fatalf("outer false, inner true = %%q", got) }
	if got := PipeLangSelect("  source  ", false, true, false); got != "outer-false" { t.Fatalf("outer false, inner false = %%q", got) }
}
`, gobackend.PackageName)))
}

func TestV780RootlessSymmetricNestedTerminalGateAndInheritance(t *testing.T) {
	input := semanticTestModuleSet("compiler.selfhosting", []ModuleInput{testModule("compiler.selfhosting", "symmetric-nested-terminal-gate.pipe", rootlessSymmetricNestedTerminalSource)}, nil)
	input.LanguageContract = PipeLangLanguageContractV770
	if err := AnalyzeSemanticModuleSet(input).Error(); err == nil {
		t.Fatal("v0.77.0 accepted symmetric nested terminal branching")
	}
	input.LanguageContract = PipeLangLanguageContractV780
	if err := AnalyzeSemanticModuleSet(input).Error(); err != nil {
		t.Fatal(err)
	}

	inherited := semanticTestModuleSet("compiler.selfhosting", []ModuleInput{testModule("compiler.selfhosting", "root-local-nested-terminal-inherited.pipe", symmetricNestedTerminalSource)}, nil)
	inherited.LanguageContract = PipeLangLanguageContractV780
	if err := AnalyzeSemanticModuleSet(inherited).Error(); err != nil {
		t.Fatalf("v0.78.0 rejected inherited v0.77.0 source: %v", err)
	}
}

func TestV780RootlessSymmetricNestedTerminalRejectsExcludedSource(t *testing.T) {
	tests := []struct {
		name   string
		source string
	}{
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
			input.LanguageContract = PipeLangLanguageContractV780
			if err := AnalyzeSemanticModuleSet(input).Error(); err == nil {
				t.Fatal("excluded v0.78.0 source was accepted")
			}
		})
	}
}

func TestV780RootlessSymmetricNestedTerminalRejectsMalformedCoreAndBackend(t *testing.T) {
	_, _, program := rootlessSymmetricNestedTerminalProgram(t)
	oldContract := program
	oldContract.LanguageContract = coreir.LanguageContractV770
	if err := coreir.ValidateProgram(oldContract); err == nil {
		t.Fatal("v0.77.0 Core accepted symmetric nested terminal branching")
	}
	if _, err := gobackend.Generate(oldContract); err == nil {
		t.Fatal("Go backend accepted symmetric branching under v0.77.0")
	}

	_, _, tooDeep := rootlessSymmetricNestedTerminalProgram(t)
	for index := range tooDeep.Functions {
		if tooDeep.Functions[index].Name != "Select" {
			continue
		}
		outer := tooDeep.Functions[index].Body.Conditional
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
	if err := coreir.ValidateProgram(tooDeep); err == nil {
		t.Fatalf("Core accepted third-level terminal branching: %v", err)
	}
	if _, err := gobackend.Generate(tooDeep); err == nil {
		t.Fatal("Go backend accepted third-level terminal branching")
	}
}

func TestV780RootlessSymmetricScopeAndTypes(t *testing.T) {
	for _, test := range []struct{ name, before, after string }{
		{"outer condition type", "if (outer)", "if (raw)"},
		{"inner condition type", "if (right)", "if (raw)"},
		{"leaf return type", "return selected;", "return true;"},
		{"self reference", "string trueValue = raw;", "string trueValue = trueValue;"},
		{"forward reference", "string trueValue = raw;", "string trueValue = later; string later = raw;"},
		{"duplicate", "string trueValue = raw;", "string trueValue = raw; string trueValue = raw;"},
		{"parameter shadow", "string trueValue = raw;", "string raw = raw;"},
		{"branch shadow", "string selected = trueValue;", "string trueValue = raw; string selected = trueValue;"},
		{"sibling branch reference", "string falseValue = trim(raw);", "string falseValue = trueValue;"},
		{"sibling leaf reference", `string selected = "outer-true";`, "string selected = otherLeaf;"},
		{"leaf escape to condition", "if (left)", "if (selected == raw)"},
		{"conditional leaf local", "string selected = trueValue;", `string selected = left ? raw : "";`},
		{"conditional branch local", "string trueValue = raw;", `string trueValue = left ? raw : "";`},
		{"assignment", "return selected;", "selected = raw; return selected;"},
		{"fallthrough", "return selected;", ""},
		{"early return", "string trueValue = raw;", "return raw; string trueValue = raw;"},
		{"inference", "string trueValue = raw;", "var trueValue = raw;"},
		{"loop", "string trueValue = raw;", "while (outer) { return raw; } string trueValue = raw;"},
	} {
		t.Run(test.name, func(t *testing.T) {
			source := strings.Replace(rootlessSymmetricNestedTerminalSource, test.before, test.after, 1)
			if source == rootlessSymmetricNestedTerminalSource {
				t.Fatal("mutation missed source")
			}
			input := semanticTestModuleSet("compiler.selfhosting", []ModuleInput{testModule("compiler.selfhosting", "invalid.pipe", source)}, nil)
			input.LanguageContract = PipeLangLanguageContractV780
			if err := AnalyzeSemanticModuleSet(input).Error(); err == nil {
				t.Fatal("excluded source accepted")
			}
		})
	}
}

func TestV780RootlessSymmetricCoreRefusesMalformedBindings(t *testing.T) {
	for _, test := range []struct {
		name   string
		mutate func(*coreir.Conditional)
	}{
		{"missing inner", func(o *coreir.Conditional) { o.WhenTrue.ImmutableLocal.Return.Conditional.WhenFalse = nil }},
		{"nonterminal inner", func(o *coreir.Conditional) { o.WhenTrue.ImmutableLocal.Return.Conditional.TerminalStatement = false }},
		{"wrong condition type", func(o *coreir.Conditional) { o.Condition = o.WhenTrue.ImmutableLocal.Initializer }},
		{"self reference", func(o *coreir.Conditional) { n := 4; o.WhenTrue.ImmutableLocal.Initializer.Parameter = &n }},
		{"forward reference", func(o *coreir.Conditional) { n := 5; o.WhenTrue.ImmutableLocal.Initializer.Parameter = &n }},
		{"noncanonical position", func(o *coreir.Conditional) { o.WhenTrue.ImmutableLocal.Position = 8 }},
		{"shadow parameter", func(o *coreir.Conditional) { o.WhenTrue.ImmutableLocal.Name = "raw" }},
		{"shadow outer local", func(o *coreir.Conditional) {
			o.WhenTrue.ImmutableLocal.Return.Conditional.WhenTrue.ImmutableLocal.Name = "trueValue"
		}},
		{"wrong leaf type", func(o *coreir.Conditional) { o.WhenTrue.ImmutableLocal.Return.Conditional.WhenTrue = o.Condition }},
		{"binding escapes into condition", func(o *coreir.Conditional) { n := 5; o.Condition.Parameter = &n }},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, _, program := rootlessSymmetricNestedTerminalProgram(t)
			for i := range program.Functions {
				if program.Functions[i].Name == "Select" {
					test.mutate(program.Functions[i].Body.Conditional)
				}
			}
			if err := coreir.ValidateProgram(program); err == nil {
				t.Fatal("malformed Core accepted")
			}
			if _, err := gobackend.Generate(program); err == nil {
				t.Fatal("backend accepted malformed Core")
			}
		})
	}
}

func TestV780TerminalInheritanceAndLocalCardinality(t *testing.T) {
	noLocals := `public Class Root { public string Select(string raw, bool outer, bool left, bool right) { if (outer) { if (left) { return raw; } else { return "a"; } } else { if (right) { return "b"; } else { return "c"; } } } }`
	manyLocals := strings.Replace(rootlessSymmetricNestedTerminalSource, "string trueValue = raw;", "string first = raw; string second = trim(first); string trueValue = second;", 1)
	manyLocals = strings.Replace(manyLocals, "string selected = trueValue;", "string firstLeaf = trueValue; string secondLeaf = trim(firstLeaf); string selected = secondLeaf;", 1)
	for i, source := range []string{terminalIfStatementSource, terminalBranchLocalSource, terminalBranchLocalSequenceSource, terminalBranchLocalGeneralSequenceSource, terminalIfWithoutTopLevelLocalSource, nestedTerminalIfSource, nestedTerminalInnerLocalSource, rootLocalNestedTerminalSource, symmetricNestedTerminalSource, noLocals, manyLocals} {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			input := semanticTestModuleSet("compiler.selfhosting", []ModuleInput{testModule("compiler.selfhosting", "inherited.pipe", source)}, nil)
			input.LanguageContract = PipeLangLanguageContractV780
			analysis := AnalyzeSemanticModuleSet(input)
			if err := analysis.Error(); err != nil {
				t.Fatal(err)
			}
			// Compile the selected method and its reachable helpers through Core.
			_, err := BuildSemanticProjection(analysis)
			if err != nil {
				t.Fatal(err)
			}
			// All canonical terminal fixtures expose Select.
			identity := semanticMethodNamed(t, analysis, "Select").Identity
			typed, err := LowerSemanticMethodToHIR(analysis, identity)
			if err != nil {
				t.Fatal(err)
			}
			program, err := LowerHIRToCore(typed)
			if err != nil {
				t.Fatal(err)
			}
			if err := coreir.ValidateProgram(program); err != nil {
				t.Fatal(err)
			}
			if _, err := gobackend.Generate(program); err != nil {
				t.Fatal(err)
			}
		})
	}
}
