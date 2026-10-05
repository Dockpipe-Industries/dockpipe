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

const boundedDepthThreeTerminalSource = `public Class CompilerBoundedDepthThreeTerminal {
	public string Normalize(string raw) => trim(raw);
	public string Select(string raw, bool outer, bool left, bool deep, bool right) {
		if (outer) {
			string trueValue = raw;
			if (left) {
				string deepValue = trueValue;
				if (deep) {
					string selected = Normalize(deepValue);
					return selected;
				} else {
					string selected = "deep-false";
					return selected;
				}
			} else {
				string selected = "outer-true";
				return selected;
			}
		} else {
			string falseValue = trim(raw);
			if (right) {
				string selected = falseValue;
				return selected;
			} else {
				string selected = "outer-false";
				return selected;
			}
		}
	}
}`

func boundedDepthThreeTerminalProgram(t *testing.T, source string) (*Analysis, SemanticIdentity, coreir.Program) {
	t.Helper()
	input := semanticTestModuleSet("compiler.selfhosting", []ModuleInput{testModule("compiler.selfhosting", "bounded-depth-three-terminal.pipe", source)}, nil)
	input.LanguageContract = PipeLangLanguageContractV790
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

func TestV790BoundedDepthThreeTerminalPipeline(t *testing.T) {
	analysis, identity, program := boundedDepthThreeTerminalProgram(t, boundedDepthThreeTerminalSource)
	projection, err := BuildSemanticProjection(analysis)
	if err != nil {
		t.Fatal(err)
	}
	if projection.LanguageContract != PipeLangLanguageContractV790 || projection.Schema != PipeLangSemanticProjectionVersion || projection.CompilerContract != PipeLangCompilerContract {
		t.Fatalf("projection contract = %#v", projection)
	}

	typed, err := LowerSemanticMethodToHIR(analysis, identity)
	if err != nil {
		t.Fatal(err)
	}
	function := hirFunctionNamed(t, typed, "Select")
	outer := function.Body.Conditional
	if typed.LanguageContract != coreir.LanguageContractV790 || function.Body.Kind != hir.ExprConditional || outer == nil || !outer.TerminalStatement {
		t.Fatalf("depth-three outer HIR = %#v", function.Body)
	}
	trueInner := outer.WhenTrue.ImmutableLocal.Return.Conditional
	falseInner := outer.WhenFalse.ImmutableLocal.Return.Conditional
	if trueInner == nil || falseInner == nil || !trueInner.TerminalStatement || !falseInner.TerminalStatement {
		t.Fatalf("depth-three HIR topology is incomplete")
	}
	third := trueInner.WhenTrue.ImmutableLocal.Return.Conditional
	if third == nil || !third.TerminalStatement {
		t.Fatalf("depth-three HIR topology is incomplete")
	}
	if outer.WhenTrue.ImmutableLocal.Binding.Position != 5 || trueInner.WhenTrue.ImmutableLocal.Binding.Position != 6 || third.WhenTrue.ImmutableLocal.Binding.Position != 7 || outer.WhenFalse.ImmutableLocal.Binding.Position != 5 {
		t.Fatalf("depth-three HIR binding positions are invalid")
	}
	coreFunction := coreFunctionNamed(t, program, "Select")
	if program.LanguageContract != coreir.LanguageContractV790 || coreFunction.Body.Kind != coreir.ExprConditional || coreFunction.Body.Conditional == nil {
		t.Fatalf("depth-three Core = %#v", coreFunction.Body)
	}
	if err := coreir.ValidateProgram(program); err != nil {
		t.Fatal(err)
	}

	entry := coreir.SemanticIdentity{PackageID: string(identity.PackageID), Path: string(identity.Path)}
	stringType := coreFunction.Parameters[0].Type
	boolType := coreFunction.Parameters[1].Type
	for _, test := range []struct {
		raw                      string
		outer, left, deep, right bool
		want                     string
	}{
		{"  source  ", true, true, true, false, "source"},
		{"  source  ", true, true, false, true, "deep-false"},
		{"  source  ", true, false, true, true, "outer-true"},
		{"  source  ", false, false, false, true, "source"},
		{"  source  ", false, true, true, false, "outer-false"},
	} {
		outcome, err := coreeval.EvaluateProgram(program, entry, []coreeval.Value{{Type: stringType, String: test.raw}, {Type: boolType, Bool: test.outer}, {Type: boolType, Bool: test.left}, {Type: boolType, Bool: test.deep}, {Type: boolType, Bool: test.right}})
		if err != nil || !outcome.OK || outcome.Value.String != test.want {
			t.Fatalf("Select(%q, %t, %t, %t, %t) = %#v, %v", test.raw, test.outer, test.left, test.deep, test.right, outcome, err)
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
	for _, fragment := range []string{"if p1 {", "if p2 {", "if p3 {", "if p4 {", "p5 := p0", "p6 := p5", "p7 := PipeLangNormalize(p6)", `p7 := "deep-false"`, `p6 := "outer-true"`, "p5 := pipelangTrimText(p0)", `p6 := "outer-false"`} {
		if !strings.Contains(string(generated), fragment) {
			t.Fatalf("generated Go lacks bounded depth-three fragment %q:\n%s", fragment, generated)
		}
	}
	compileAndRunGeneratedGoFiles(t, generated, []byte(fmt.Sprintf(`package %s

import "testing"

func TestGeneratedBoundedDepthThreeTerminal(t *testing.T) {
	if got := PipeLangSelect("  source  ", true, true, true, false); got != "source" { t.Fatalf("deep true = %%q", got) }
	if got := PipeLangSelect("  source  ", true, true, false, true); got != "deep-false" { t.Fatalf("deep false = %%q", got) }
	if got := PipeLangSelect("  source  ", true, false, true, true); got != "outer-true" { t.Fatalf("outer true sibling = %%q", got) }
	if got := PipeLangSelect("  source  ", false, false, false, true); got != "source" { t.Fatalf("outer false inner true = %%q", got) }
	if got := PipeLangSelect("  source  ", false, true, true, false); got != "outer-false" { t.Fatalf("outer false inner false = %%q", got) }
}
`, gobackend.PackageName)))
}

func TestV790BoundedDepthThreeGateRootFormsAndInheritance(t *testing.T) {
	input := semanticTestModuleSet("compiler.selfhosting", []ModuleInput{testModule("compiler.selfhosting", "bounded-depth-three-gate.pipe", boundedDepthThreeTerminalSource)}, nil)
	input.LanguageContract = PipeLangLanguageContractV780
	if err := AnalyzeSemanticModuleSet(input).Error(); err == nil {
		t.Fatal("v0.78.0 accepted bounded depth-three terminal branching")
	}
	input.LanguageContract = PipeLangLanguageContractV790
	if err := AnalyzeSemanticModuleSet(input).Error(); err != nil {
		t.Fatal(err)
	}

	rootful := strings.Replace(boundedDepthThreeTerminalSource, "public string Select(string raw, bool outer, bool left, bool deep, bool right) {", "public string Select(string raw, bool outer, bool left, bool deep, bool right) { string shared = raw;", 1)
	rootful = strings.Replace(rootful, "string trueValue = raw;", "string trueValue = shared;", 1)
	boundedDepthThreeTerminalProgram(t, rootful)

	for _, source := range []string{symmetricNestedTerminalSource, rootlessSymmetricNestedTerminalSource} {
		inherited := semanticTestModuleSet("compiler.selfhosting", []ModuleInput{testModule("compiler.selfhosting", "inherited.pipe", source)}, nil)
		inherited.LanguageContract = PipeLangLanguageContractV790
		if err := AnalyzeSemanticModuleSet(inherited).Error(); err != nil {
			t.Fatalf("v0.79.0 rejected inherited source: %v", err)
		}
	}
}

func TestV790BoundedDepthThreeRejectsExcludedSource(t *testing.T) {
	twoExpanded := strings.Replace(boundedDepthThreeTerminalSource, "string selected = falseValue;\n\t\t\t\treturn selected;", "if (deep) { return falseValue; } else { return raw; }", 1)
	depthFour := strings.Replace(boundedDepthThreeTerminalSource, "string selected = Normalize(deepValue);\n\t\t\t\t\treturn selected;", "if (right) { return deepValue; } else { return raw; }", 1)
	oneSidedBase := `public Class Root { public string Select(string raw, bool outer, bool left, bool deep, bool right) { if (outer) { if (left) { if (deep) { return raw; } else { return "a"; } } else { return "b"; } } else { return "c"; } } }`
	for _, test := range []struct{ name, source string }{
		{"two expanded depth-two leaves", twoExpanded},
		{"fourth nesting level", depthFour},
		{"non-symmetric depth-two base", oneSidedBase},
		{"conditional expression", strings.Replace(boundedDepthThreeTerminalSource, "string deepValue = trueValue;", `string deepValue = deep ? trueValue : "";`, 1)},
		{"propagation", strings.Replace(boundedDepthThreeTerminalSource, "string deepValue = trueValue;", "string deepValue = propagate(trueValue);", 1)},
		{"match", strings.Replace(boundedDepthThreeTerminalSource, "string deepValue = trueValue;", "string deepValue = match(trueValue){ ok(value) => value, err(problem) => problem };", 1)},
	} {
		t.Run(test.name, func(t *testing.T) {
			input := semanticTestModuleSet("compiler.selfhosting", []ModuleInput{testModule("compiler.selfhosting", "invalid.pipe", test.source)}, nil)
			input.LanguageContract = PipeLangLanguageContractV790
			if err := AnalyzeSemanticModuleSet(input).Error(); err == nil {
				t.Fatal("excluded v0.79.0 source was accepted")
			}
		})
	}
}

func TestV790BoundedDepthThreeScopeAndTypes(t *testing.T) {
	for _, test := range []struct{ name, before, after string }{
		{"outer condition type", "if (outer)", "if (raw)"},
		{"third condition type", "if (deep)", "if (raw)"},
		{"third leaf return type", "return selected;", "return true;"},
		{"self reference", "string deepValue = trueValue;", "string deepValue = deepValue;"},
		{"forward reference", "string deepValue = trueValue;", "string deepValue = later; string later = trueValue;"},
		{"duplicate", "string deepValue = trueValue;", "string deepValue = trueValue; string deepValue = trueValue;"},
		{"parameter shadow", "string deepValue = trueValue;", "string raw = trueValue;"},
		{"outer shadow", "string deepValue = trueValue;", "string trueValue = trueValue;"},
		{"third sibling reference", `string selected = "deep-false";`, "string selected = otherLeaf;"},
		{"third local escapes", "if (deep)", "if (selected == raw)"},
		{"assignment", "return selected;", "selected = raw; return selected;"},
		{"fallthrough", "return selected;", ""},
		{"early return", "string deepValue = trueValue;", "return raw; string deepValue = trueValue;"},
		{"inference", "string deepValue = trueValue;", "var deepValue = trueValue;"},
		{"loop", "string deepValue = trueValue;", "while (deep) { return raw; } string deepValue = trueValue;"},
	} {
		t.Run(test.name, func(t *testing.T) {
			source := strings.Replace(boundedDepthThreeTerminalSource, test.before, test.after, 1)
			if source == boundedDepthThreeTerminalSource {
				t.Fatal("mutation missed source")
			}
			input := semanticTestModuleSet("compiler.selfhosting", []ModuleInput{testModule("compiler.selfhosting", "invalid.pipe", source)}, nil)
			input.LanguageContract = PipeLangLanguageContractV790
			if err := AnalyzeSemanticModuleSet(input).Error(); err == nil {
				t.Fatal("excluded source accepted")
			}
		})
	}
}

func TestV790BoundedDepthThreeRejectsMalformedCoreAndBackend(t *testing.T) {
	_, _, oldContract := boundedDepthThreeTerminalProgram(t, boundedDepthThreeTerminalSource)
	oldContract.LanguageContract = coreir.LanguageContractV780
	if err := coreir.ValidateProgram(oldContract); err == nil {
		t.Fatal("v0.78.0 Core accepted bounded depth-three terminal branching")
	}
	if _, err := gobackend.Generate(oldContract); err == nil {
		t.Fatal("Go backend accepted bounded depth-three branching under v0.78.0")
	}

	for _, test := range []struct {
		name   string
		mutate func(*coreir.Conditional)
	}{
		{"missing third leaf", func(o *coreir.Conditional) {
			o.WhenTrue.ImmutableLocal.Return.Conditional.WhenTrue.ImmutableLocal.Return.Conditional.WhenFalse = nil
		}},
		{"nonterminal third", func(o *coreir.Conditional) {
			o.WhenTrue.ImmutableLocal.Return.Conditional.WhenTrue.ImmutableLocal.Return.Conditional.TerminalStatement = false
		}},
		{"wrong third condition type", func(o *coreir.Conditional) {
			o.WhenTrue.ImmutableLocal.Return.Conditional.WhenTrue.ImmutableLocal.Return.Conditional.Condition = o.WhenTrue.ImmutableLocal.Initializer
		}},
		{"noncanonical third binding", func(o *coreir.Conditional) {
			o.WhenTrue.ImmutableLocal.Return.Conditional.WhenTrue.ImmutableLocal.Return.Conditional.WhenTrue.ImmutableLocal.Position = 20
		}},
		{"third leaf shadows ancestor", func(o *coreir.Conditional) {
			o.WhenTrue.ImmutableLocal.Return.Conditional.WhenTrue.ImmutableLocal.Return.Conditional.WhenTrue.ImmutableLocal.Name = "trueValue"
		}},
		{"wrong third leaf type", func(o *coreir.Conditional) {
			o.WhenTrue.ImmutableLocal.Return.Conditional.WhenTrue.ImmutableLocal.Return.Conditional.WhenTrue = o.Condition
		}},
		{"depth four", func(o *coreir.Conditional) {
			third := o.WhenTrue.ImmutableLocal.Return.Conditional.WhenTrue.ImmutableLocal.Return.Conditional
			leaf := *third.WhenTrue
			third.WhenTrue = &coreir.Expr{Kind: coreir.ExprConditional, Type: leaf.Type, Conditional: &coreir.Conditional{Condition: third.Condition, WhenTrue: &leaf, WhenFalse: &leaf, TerminalStatement: true}}
		}},
		{"second expanded leaf", func(o *coreir.Conditional) {
			inner := o.WhenFalse.ImmutableLocal.Return.Conditional
			leaf := *inner.WhenTrue
			inner.WhenTrue = &coreir.Expr{Kind: coreir.ExprConditional, Type: leaf.Type, Conditional: &coreir.Conditional{Condition: inner.Condition, WhenTrue: &leaf, WhenFalse: &leaf, TerminalStatement: true}}
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, _, program := boundedDepthThreeTerminalProgram(t, boundedDepthThreeTerminalSource)
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

func TestV790TerminalInheritanceAndLocalCardinality(t *testing.T) {
	noLocals := `public Class Root { public string Select(string raw, bool outer, bool left, bool deep, bool right) { if (outer) { if (left) { if (deep) { return raw; } else { return "a"; } } else { return "b"; } } else { if (right) { return "c"; } else { return "d"; } } } }`
	manyLocals := strings.Replace(boundedDepthThreeTerminalSource, "string deepValue = trueValue;", "string first = trueValue; string second = trim(first); string deepValue = second;", 1)
	manyLocals = strings.Replace(manyLocals, "string selected = Normalize(deepValue);", "string firstLeaf = deepValue; string secondLeaf = trim(firstLeaf); string selected = Normalize(secondLeaf);", 1)
	for i, source := range []string{terminalIfStatementSource, terminalBranchLocalSource, terminalBranchLocalSequenceSource, terminalBranchLocalGeneralSequenceSource, terminalIfWithoutTopLevelLocalSource, nestedTerminalIfSource, nestedTerminalInnerLocalSource, rootLocalNestedTerminalSource, symmetricNestedTerminalSource, rootlessSymmetricNestedTerminalSource, noLocals, manyLocals} {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			input := semanticTestModuleSet("compiler.selfhosting", []ModuleInput{testModule("compiler.selfhosting", "inherited.pipe", source)}, nil)
			input.LanguageContract = PipeLangLanguageContractV790
			analysis := AnalyzeSemanticModuleSet(input)
			if err := analysis.Error(); err != nil {
				t.Fatal(err)
			}
			if _, err := BuildSemanticProjection(analysis); err != nil {
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
			if err := coreir.ValidateProgram(program); err != nil {
				t.Fatal(err)
			}
			if _, err := gobackend.Generate(program); err != nil {
				t.Fatal(err)
			}
		})
	}
}
