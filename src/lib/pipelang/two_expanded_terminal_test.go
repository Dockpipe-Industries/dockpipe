package pipelang

import (
	"fmt"
	"strings"
	"testing"

	"dockpipe/src/lib/pipelang/coreeval"
	"dockpipe/src/lib/pipelang/coreir"
	"dockpipe/src/lib/pipelang/gobackend"
)

// The four original leaf positions are left/true, left/false, right/true,
// right/false. Each expanded leaf adds one independent bool decision.
func twoExpandedSource(expanded map[int]bool, rootLocals, branchLocals bool) string {
	leaves := make([]string, 4)
	for i := range leaves {
		value := "raw"
		prefix := ""
		if branchLocals {
			prefix = "string first = ancestor; string second = trim(first); string context = Normalize(second); "
			value = "context"
		}
		leaf := func(suffix string) string {
			if branchLocals {
				return fmt.Sprintf("string firstLeaf = %s; string secondLeaf = Normalize(firstLeaf); string selected = secondLeaf + %q; return selected;", value, suffix)
			}
			return fmt.Sprintf("return %s + %q;", value, suffix)
		}
		if expanded[i] {
			leaves[i] = prefix + fmt.Sprintf("if (deep%d) { %s } else { %s }", i, leaf(fmt.Sprintf("%dT", i)), leaf(fmt.Sprintf("%dF", i)))
		} else {
			leaves[i] = prefix + leaf(fmt.Sprint(i))
		}
	}
	root := ""
	value := "raw"
	if rootLocals {
		root = "string rootFirst = raw; string rootSecond = trim(rootFirst); string shared = Normalize(rootSecond); "
		value = "shared"
	}
	prefix := ""
	if branchLocals {
		prefix = "string ancestor = " + value + "; "
	}
	return fmt.Sprintf(`public Class TwoExpanded {
 public string Normalize(string value) => trim(value);
 public string Select(string raw, bool outer, bool left, bool right, bool deep0, bool deep1, bool deep2, bool deep3) {
 %s if (outer) { %s if (left) { %s } else { %s } } else { %s if (right) { %s } else { %s } }
 }
 }`, root, prefix, leaves[0], leaves[1], prefix, leaves[2], leaves[3])
}

func twoExpandedProgram(t *testing.T, source string) (*Analysis, SemanticIdentity, coreir.Program) {
	t.Helper()
	input := semanticTestModuleSet("compiler.selfhosting", []ModuleInput{testModule("compiler.selfhosting", "two-expanded.pipe", source)}, nil)
	input.LanguageContract = PipeLangLanguageContractV800
	analysis := AnalyzeSemanticModuleSet(input)
	if err := analysis.Error(); err != nil {
		t.Fatal(err)
	}
	projection, err := BuildSemanticProjection(analysis)
	if err != nil {
		t.Fatal(err)
	}
	if projection.LanguageContract != PipeLangLanguageContractV800 || projection.Schema != PipeLangSemanticProjectionVersion || projection.CompilerContract != PipeLangCompilerContract {
		t.Fatal("projection identity drift")
	}
	identity := semanticMethodNamed(t, analysis, "Select").Identity
	typed, err := LowerSemanticMethodToHIR(analysis, identity)
	if err != nil {
		t.Fatal(err)
	}
	if typed.LanguageContract != coreir.LanguageContractV800 {
		t.Fatal("HIR language contract drift")
	}
	program, err := LowerHIRToCore(typed)
	if err != nil {
		t.Fatal(err)
	}
	if err := coreir.ValidateProgram(program); err != nil {
		t.Fatal(err)
	}
	return analysis, identity, program
}

func TestV800TwoExpandedAllPairsAndPaths(t *testing.T) {
	for a := 0; a < 4; a++ {
		for b := a + 1; b < 4; b++ {
			for _, root := range []bool{false, true} {
				for _, locals := range []bool{false, true} {
					t.Run(fmt.Sprintf("%d-%d/root=%t/locals=%t", a, b, root, locals), func(t *testing.T) {
						expanded := map[int]bool{a: true, b: true}
						_, identity, program := twoExpandedProgram(t, twoExpandedSource(expanded, root, locals))
						function := coreFunctionNamed(t, program, "Select")
						if program.LanguageContract != coreir.LanguageContractV800 || coreConditionalCount(function.Body) != 5 {
							t.Fatal("Core topology or metadata drift")
						}
						entry := coreir.SemanticIdentity{PackageID: string(identity.PackageID), Path: string(identity.Path)}
						generated, err := gobackend.Generate(program)
						if err != nil {
							t.Fatal(err)
						}
						again, err := gobackend.Generate(program)
						if err != nil || string(again) != string(generated) {
							t.Fatal("nondeterministic Go")
						}
						var cases strings.Builder
						paths := 0
						for leaf := 0; leaf < 4; leaf++ {
							decisions := []bool{false}
							if expanded[leaf] {
								decisions = []bool{false, true}
							}
							for _, deep := range decisions {
								paths++
								// Unselected conditions deliberately vary, exercising all six leaves.
								flags := []bool{leaf < 2, leaf == 0, leaf == 2, true, false, true, false}
								flags[3+leaf] = deep
								suffix := fmt.Sprint(leaf)
								if expanded[leaf] {
									if deep {
										suffix += "T"
									} else {
										suffix += "F"
									}
								}
								raw := "  value  "
								want := raw + suffix
								if locals {
									want = "value" + suffix
								}
								args := []coreeval.Value{{Type: function.Parameters[0].Type, String: raw}}
								var goArgs []string
								for i, flag := range flags {
									args = append(args, coreeval.Value{Type: function.Parameters[i+1].Type, Bool: flag})
									goArgs = append(goArgs, fmt.Sprint(flag))
								}
								outcome, err := coreeval.EvaluateProgram(program, entry, args)
								if err != nil || !outcome.OK || outcome.Value.String != want {
									t.Fatalf("leaf %d deep %t: %#v %v; want %q", leaf, deep, outcome, err, want)
								}
								fmt.Fprintf(&cases, "if got := PipeLangSelect(%q, %s); got != %q { t.Fatalf(\"path %d = %%q\",got) }\n", raw, strings.Join(goArgs, ", "), want, paths)
							}
						}
						if paths != 6 {
							t.Fatalf("covered %d paths", paths)
						}
						compileAndRunGeneratedGoFiles(t, generated, []byte(fmt.Sprintf("package %s\nimport \"testing\"\nfunc TestPaths(t *testing.T) {\n%s}\n", gobackend.PackageName, cases.String())))
					})
				}
			}
		}
	}
}

func coreConditionalCount(expr coreir.Expr) int {
	if expr.ImmutableLocal != nil {
		return coreConditionalCount(*expr.ImmutableLocal.Return)
	}
	if expr.Conditional != nil {
		return 1 + coreConditionalCount(*expr.Conditional.WhenTrue) + coreConditionalCount(*expr.Conditional.WhenFalse)
	}
	return 0
}

func TestV800TwoExpandedVersionAndInheritance(t *testing.T) {
	source := twoExpandedSource(map[int]bool{0: true, 2: true}, false, true)
	input := semanticTestModuleSet("compiler.selfhosting", []ModuleInput{testModule("compiler.selfhosting", "gate.pipe", source)}, nil)
	input.LanguageContract = PipeLangLanguageContractV790
	if err := AnalyzeSemanticModuleSet(input).Error(); err == nil {
		t.Fatal("v0.79 accepted two expansions")
	}
	_, _, program := twoExpandedProgram(t, source)
	program.LanguageContract = coreir.LanguageContractV790
	if err := coreir.ValidateProgram(program); err == nil {
		t.Fatal("old Core accepted new topology")
	}
	if _, err := gobackend.Generate(program); err == nil {
		t.Fatal("backend accepted new topology under old version")
	}
	for i, source := range []string{terminalIfStatementSource, terminalBranchLocalSource, terminalBranchLocalSequenceSource, terminalBranchLocalGeneralSequenceSource, terminalIfWithoutTopLevelLocalSource, nestedTerminalIfSource, nestedTerminalInnerLocalSource, rootLocalNestedTerminalSource, symmetricNestedTerminalSource, rootlessSymmetricNestedTerminalSource, boundedDepthThreeTerminalSource} {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			_, _, program := twoExpandedProgram(t, source)
			if _, err := gobackend.Generate(program); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestV800TwoExpandedRejectsSource(t *testing.T) {
	source := twoExpandedSource(map[int]bool{0: true, 2: true}, true, true)
	cases := []struct{ name, source string }{
		{"three expanded", twoExpandedSource(map[int]bool{0: true, 1: true, 2: true}, false, false)},
		{"four expanded", twoExpandedSource(map[int]bool{0: true, 1: true, 2: true, 3: true}, false, false)},
		{"asymmetric", `public Class Root { public string Select(string raw, bool outer, bool left, bool deep) { if (outer) { if (left) { if (deep) { return raw; } else { return "a"; } } else { return "b"; } } else { return "c"; } } }`},
	}
	for _, mutation := range []struct{ name, before, after string }{
		{"depth four", "return selected;", "if (deep3) { return selected; } else { return raw; }"},
		{"condition type", "if (deep0)", "if (raw)"},
		{"return type", "return selected;", "return true;"},
		{"initializer type", "string context = Normalize(second);", "string context = true;"},
		{"self reference", "string context = Normalize(second);", "string context = context;"},
		{"forward reference", "string context = Normalize(second);", "string context = later; string later = second;"},
		{"duplicate", "string context = Normalize(second);", "string context = second; string context = second;"},
		{"ancestor shadow", "string context = Normalize(second);", "string ancestor = second;"},
		{"parameter shadow", "string context = Normalize(second);", "string raw = second;"},
		{"sibling reference", "string firstLeaf = context;", "string firstLeaf = selected;"},
		{"escaped leaf", "if (deep0)", "if (selected == raw)"},
		{"conditional expression", "string context = Normalize(second);", "string context = deep0 ? second : raw;"},
		{"propagation", "string context = Normalize(second);", "string context = propagate(second);"},
		{"match", "string context = Normalize(second);", "string context = match(second){ ok(value) => value, err(problem) => problem };"},
		{"assignment", "return selected;", "selected = raw; return selected;"},
		{"fallthrough", "return selected;", ""},
		{"early return", "string context = Normalize(second);", "return raw; string context = Normalize(second);"},
		{"loop", "string context = Normalize(second);", "while (deep0) { return raw; } string context = Normalize(second);"},
		{"inference", "string context = Normalize(second);", "var context = Normalize(second);"},
		{"effect", "string context = Normalize(second);", "string context = shell(second);"},
	} {
		changed := strings.Replace(source, mutation.before, mutation.after, 1)
		if changed == source {
			t.Fatal("mutation missed source")
		}
		cases = append(cases, struct{ name, source string }{mutation.name, changed})
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			input := semanticTestModuleSet("compiler.selfhosting", []ModuleInput{testModule("compiler.selfhosting", "invalid.pipe", test.source)}, nil)
			input.LanguageContract = PipeLangLanguageContractV800
			if err := AnalyzeSemanticModuleSet(input).Error(); err == nil {
				t.Fatal("excluded source accepted")
			}
		})
	}
}

func terminalCoreTail(e *coreir.Expr) *coreir.Expr {
	for e.ImmutableLocal != nil {
		e = e.ImmutableLocal.Return
	}
	return e
}

func TestV800TwoExpandedRejectsMalformedCore(t *testing.T) {
	for _, test := range []struct {
		name   string
		mutate func(*coreir.Conditional)
	}{
		{"missing third leaf", func(o *coreir.Conditional) {
			terminalCoreTail(terminalCoreTail(o.WhenFalse).Conditional.WhenTrue).Conditional.WhenFalse = nil
		}},
		{"nonterminal third", func(o *coreir.Conditional) {
			terminalCoreTail(terminalCoreTail(o.WhenFalse).Conditional.WhenTrue).Conditional.TerminalStatement = false
		}},
		{"wrong condition type", func(o *coreir.Conditional) {
			third := terminalCoreTail(terminalCoreTail(o.WhenFalse).Conditional.WhenTrue).Conditional
			third.Condition = third.WhenTrue
		}},
		{"wrong return type", func(o *coreir.Conditional) {
			third := terminalCoreTail(terminalCoreTail(o.WhenFalse).Conditional.WhenTrue).Conditional
			third.WhenTrue = o.Condition
		}},
		{"binding position", func(o *coreir.Conditional) {
			third := terminalCoreTail(terminalCoreTail(o.WhenFalse).Conditional.WhenTrue).Conditional
			third.WhenTrue.ImmutableLocal.Position = 99
		}},
		{"ancestor shadow", func(o *coreir.Conditional) {
			third := terminalCoreTail(terminalCoreTail(o.WhenFalse).Conditional.WhenTrue).Conditional
			third.WhenTrue.ImmutableLocal.Name = "ancestor"
		}},
		{"third expansion", func(o *coreir.Conditional) {
			inner := terminalCoreTail(o.WhenFalse).Conditional
			leaf := inner.WhenFalse
			inner.WhenFalse = &coreir.Expr{Kind: coreir.ExprConditional, Type: leaf.Type, Conditional: &coreir.Conditional{Condition: inner.Condition, WhenTrue: leaf, WhenFalse: leaf, TerminalStatement: true}}
		}},
		{"depth four", func(o *coreir.Conditional) {
			third := terminalCoreTail(terminalCoreTail(o.WhenFalse).Conditional.WhenTrue).Conditional
			leaf := third.WhenFalse
			third.WhenFalse = &coreir.Expr{Kind: coreir.ExprConditional, Type: leaf.Type, Conditional: &coreir.Conditional{Condition: third.Condition, WhenTrue: leaf, WhenFalse: leaf, TerminalStatement: true}}
		}},
		{"asymmetric", func(o *coreir.Conditional) { o.WhenTrue = terminalCoreTail(o.WhenTrue).Conditional.WhenTrue }},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, _, program := twoExpandedProgram(t, twoExpandedSource(map[int]bool{0: true, 2: true}, false, true))
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

func TestV800TwoExpandedInitializerOrderAndLaziness(t *testing.T) {
	source := twoExpandedSource(map[int]bool{0: true, 2: true}, true, true)
	source = strings.ReplaceAll(source, "Normalize(rootSecond)", `Normalize(rootSecond + "R")`)
	source = strings.ReplaceAll(source, "Normalize(second)", `Normalize(second + "D")`)
	source = strings.ReplaceAll(source, "Normalize(firstLeaf)", `Normalize(firstLeaf + "L")`)
	_, identity, program := twoExpandedProgram(t, source)
	function := coreFunctionNamed(t, program, "Select")
	entry := coreir.SemanticIdentity{PackageID: string(identity.PackageID), Path: string(identity.Path)}
	generated, err := gobackend.Generate(program)
	if err != nil {
		t.Fatal(err)
	}
	// Instrument only the pure helper in this test copy to observe evaluation order.
	// The all-pairs test separately compiles and executes pristine backend output.
	marker := "func PipeLangNormalize(p0 string) string {"
	if strings.Count(string(generated), marker) != 1 {
		t.Fatal("missing helper instrumentation target")
	}
	instrumented := strings.Replace(string(generated), marker, marker+"\n v800Trace = append(v800Trace,p0)", 1)
	var cases strings.Builder
	for _, test := range []struct {
		flags  []bool
		suffix string
	}{
		{[]bool{true, true, false, true, false, false, false}, "0T"},
		{[]bool{true, true, false, false, false, false, false}, "0F"},
		{[]bool{true, false, false, true, false, true, false}, "1"},
		{[]bool{false, false, true, false, false, true, false}, "2T"},
		{[]bool{false, false, true, true, false, false, false}, "2F"},
		{[]bool{false, true, false, true, true, true, true}, "3"},
	} {
		want := "valueRDL" + test.suffix
		args := []coreeval.Value{{Type: function.Parameters[0].Type, String: "  value  "}}
		var goArgs []string
		for i, flag := range test.flags {
			args = append(args, coreeval.Value{Type: function.Parameters[i+1].Type, Bool: flag})
			goArgs = append(goArgs, fmt.Sprint(flag))
		}
		outcome, err := coreeval.EvaluateProgram(program, entry, args)
		if err != nil || !outcome.OK || outcome.Value.String != want {
			t.Fatalf("initializer order: %#v %v", outcome, err)
		}
		fmt.Fprintf(&cases, `v800Trace=nil
 if got:=PipeLangSelect("  value  ",%s);got!=%q {t.Fatal(got)}
 if !reflect.DeepEqual(v800Trace,[]string{"valueR","valueRD","valueRDL"}) {t.Fatalf("initializer order or eager sibling: %%v",v800Trace)}
 `, strings.Join(goArgs, ","), want)
	}
	compileAndRunGeneratedGoFiles(t, []byte(instrumented), []byte(fmt.Sprintf("package %s\nimport (\"testing\";\"reflect\")\nvar v800Trace []string\nfunc TestTiming(t *testing.T){%s}\n", gobackend.PackageName, cases.String())))
}
