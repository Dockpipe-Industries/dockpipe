package pipelang

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"dockpipe/src/lib/pipelang/coreeval"
	"dockpipe/src/lib/pipelang/coreir"
	"dockpipe/src/lib/pipelang/gobackend"
)

func terminalTreeProgram(t *testing.T, source string) (*Analysis, SemanticIdentity, coreir.Program) {
	t.Helper()
	input := semanticTestModuleSet("compiler.selfhosting", []ModuleInput{testModule("compiler.selfhosting", "terminal-tree.pipe", source)}, nil)
	input.LanguageContract = PipeLangLanguageContractV810
	analysis := AnalyzeSemanticModuleSet(input)
	if err := analysis.Error(); err != nil {
		t.Fatal(err)
	}
	projection, err := BuildSemanticProjection(analysis)
	if err != nil {
		t.Fatal(err)
	}
	if projection.LanguageContract != PipeLangLanguageContractV810 || projection.Schema != PipeLangSemanticProjectionVersion || projection.CompilerContract != PipeLangCompilerContract {
		t.Fatal("projection identity drift")
	}
	identity := semanticMethodNamed(t, analysis, "Select").Identity
	typed, err := LowerSemanticMethodToHIR(analysis, identity)
	if err != nil {
		t.Fatal(err)
	}
	if typed.LanguageContract != coreir.LanguageContractV810 {
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

func TestV810TerminalTreeRejectsSource(t *testing.T) {
	source := twoExpandedSource(map[int]bool{0: true, 2: true}, true, true)
	cases := []struct{ name, source string }{}
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
			input.LanguageContract = PipeLangLanguageContractV810
			err := AnalyzeSemanticModuleSet(input).Error()
			diagnostics, ok := AsDiagnostics(err)
			if err == nil || !ok || len(diagnostics) == 0 {
				t.Fatalf("expected located diagnostic, got %v", err)
			}
			for _, diagnostic := range diagnostics {
				if !diagnostic.Primary.IsValid() || diagnostic.Primary.File != "invalid.pipe" || diagnostic.Primary.End > len(test.source) {
					t.Fatalf("invalid diagnostic span: %#v", diagnostic.Primary)
				}
			}
		})
	}
}

func TestV810TerminalTreeRejectsMalformedCore(t *testing.T) {
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
		{"depth four", func(o *coreir.Conditional) {
			third := terminalCoreTail(terminalCoreTail(o.WhenFalse).Conditional.WhenTrue).Conditional
			leaf := third.WhenFalse
			third.WhenFalse = &coreir.Expr{Kind: coreir.ExprConditional, Type: leaf.Type, Conditional: &coreir.Conditional{Condition: third.Condition, WhenTrue: leaf, WhenFalse: leaf, TerminalStatement: true}}
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, _, program := terminalTreeProgram(t, twoExpandedSource(map[int]bool{0: true, 2: true}, false, true))
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

// nil is a return leaf. Enumeration is independent of compiler topology checks:
// T(0)=1 and T(d)=1+T(d-1)^2 give 26 shapes including the no-decision leaf.
type terminalTree struct{ yes, no *terminalTree }

func terminalTrees(depth int) []*terminalTree {
	trees := []*terminalTree{nil}
	if depth > 0 {
		children := terminalTrees(depth - 1)
		for _, yes := range children {
			for _, no := range children {
				trees = append(trees, &terminalTree{yes, no})
			}
		}
	}
	return trees
}

func terminalTreeSource(tree *terminalTree, root, locals bool) string {
	var emit func(*terminalTree, string, string, bool) string
	emit = func(node *terminalTree, path, value string, withLocals bool) string {
		prefix := ""
		if withLocals {
			prefix = fmt.Sprintf("string first%s = %s + %q; string value%s = Echo(first%s); ", path, value, path, path, path)
			value = "value" + path
		}
		if node == nil {
			return prefix + fmt.Sprintf("return %s + %q;", value, ":"+path)
		}
		return prefix + fmt.Sprintf("if (Check(%q, d%d)) { %s } else { %s }", path, len(path)-1, emit(node.yes, path+"T", value, locals), emit(node.no, path+"F", value, locals))
	}
	return `public Class TerminalTree {
 public string Echo(string value) => value;
 public bool Check(string path, bool value) => value;
 public string Select(string raw, bool d0, bool d1, bool d2) { ` + emit(tree, "R", "raw", root) + ` }
 }`
}

func TestV810TerminalTreeAllShapesAndPaths(t *testing.T) {
	trees := terminalTrees(3)[1:]
	if len(trees) != 25 {
		t.Fatalf("shape inventory %d", len(trees))
	}
	for index, tree := range trees {
		for _, root := range []bool{false, true} {
			for _, locals := range []bool{false, true} {
				t.Run(fmt.Sprintf("shape%d/root=%t/locals=%t", index, root, locals), func(t *testing.T) {
					source := terminalTreeSource(tree, root, locals)
					analysis, identity, program := terminalTreeProgram(t, source)
					againAnalysis, _, againProgram := terminalTreeProgram(t, source)
					firstJSON, _ := json.Marshal(program)
					againJSON, _ := json.Marshal(againProgram)
					if !bytes.Equal(firstJSON, againJSON) {
						t.Fatal("Core repeat-build drift")
					}
					projection, _ := BuildSemanticProjection(analysis)
					againProjection, _ := BuildSemanticProjection(againAnalysis)
					firstJSON, _ = json.Marshal(projection)
					againJSON, _ = json.Marshal(againProjection)
					if !bytes.Equal(firstJSON, againJSON) {
						t.Fatal("semantic repeat-build drift")
					}
					generated, err := gobackend.Generate(program)
					if err != nil {
						t.Fatal(err)
					}
					again, err := gobackend.Generate(againProgram)
					if err != nil || !bytes.Equal(generated, again) {
						t.Fatal("Go repeat-build drift")
					}
					function := coreFunctionNamed(t, program, "Select")
					entry := coreir.SemanticIdentity{PackageID: string(identity.PackageID), Path: string(identity.Path)}
					paths := map[string]bool{}
					var cases strings.Builder
					for mask := 0; mask < 8; mask++ {
						flags := []bool{mask&1 != 0, mask&2 != 0, mask&4 != 0}
						node, path, want := tree, "R", "value"
						if root {
							want += path
						}
						for depth := 0; node != nil; depth++ {
							if flags[depth] {
								node = node.yes
								path += "T"
							} else {
								node = node.no
								path += "F"
							}
							if locals {
								want += path
							}
						}
						want += ":" + path
						paths[path] = true
						args := []coreeval.Value{{Type: function.Parameters[0].Type, String: "value"}}
						for i, flag := range flags {
							args = append(args, coreeval.Value{Type: function.Parameters[i+1].Type, Bool: flag})
						}
						outcome, err := coreeval.EvaluateProgram(program, entry, args)
						if err != nil || !outcome.OK || outcome.Value.String != want {
							t.Fatalf("mask %d: %#v %v want %q", mask, outcome, err, want)
						}
						fmt.Fprintf(&cases, "if got:=PipeLangSelect(\"value\",%t,%t,%t);got!=%q {t.Fatalf(\"mask %d: %%q\",got)}\n", flags[0], flags[1], flags[2], want, mask)
					}
					if len(paths) != coreConditionalCount(function.Body)+1 {
						t.Fatal("missing return path")
					}
					compileAndRunGeneratedGoFiles(t, generated, []byte(fmt.Sprintf("package %s\nimport \"testing\"\nfunc TestPaths(t *testing.T){%s}\n", gobackend.PackageName, cases.String())))
				})
			}
		}
	}
}

func TestV810TerminalTreeVersionAndInheritance(t *testing.T) {
	for _, source := range []string{
		twoExpandedSource(map[int]bool{0: true, 1: true, 2: true}, false, false),
		twoExpandedSource(map[int]bool{0: true, 1: true, 2: true, 3: true}, true, true),
		terminalTreeSource(&terminalTree{yes: &terminalTree{yes: &terminalTree{}}}, false, true),
	} {
		_, _, program := terminalTreeProgram(t, source)
		input := semanticTestModuleSet("compiler.selfhosting", []ModuleInput{testModule("compiler.selfhosting", "gate.pipe", source)}, nil)
		input.LanguageContract = PipeLangLanguageContractV800
		if err := AnalyzeSemanticModuleSet(input).Error(); err == nil {
			t.Fatal("old source contract accepted new shape")
		}
		program.LanguageContract = coreir.LanguageContractV800
		if err := coreir.ValidateProgram(program); err == nil {
			t.Fatal("old Core contract accepted new shape")
		}
		if _, err := gobackend.Generate(program); err == nil {
			t.Fatal("old backend contract accepted new shape")
		}
	}
	for _, source := range []string{terminalIfStatementSource, terminalBranchLocalSource, terminalBranchLocalSequenceSource, terminalBranchLocalGeneralSequenceSource, terminalIfWithoutTopLevelLocalSource, nestedTerminalIfSource, nestedTerminalInnerLocalSource, rootLocalNestedTerminalSource, symmetricNestedTerminalSource, rootlessSymmetricNestedTerminalSource, boundedDepthThreeTerminalSource, twoExpandedSource(map[int]bool{0: true, 2: true}, true, true)} {
		_, _, old := twoExpandedProgram(t, source)
		_, _, current := terminalTreeProgram(t, source)
		oldGo, err := gobackend.Generate(old)
		if err != nil {
			t.Fatal(err)
		}
		newGo, err := gobackend.Generate(current)
		if err != nil || !bytes.Equal(oldGo, newGo) {
			t.Fatal("inherited generated bytes changed")
		}
	}
}

func TestV810TerminalTreeExecutionOrder(t *testing.T) {
	for i, tree := range terminalTrees(3)[1:] {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			_, _, program := terminalTreeProgram(t, terminalTreeSource(tree, true, true))
			generated, err := gobackend.Generate(program)
			if err != nil {
				t.Fatal(err)
			}
			// This instrumented copy observes calls; pristine output is executed above.
			observed := string(generated)
			for marker, probe := range map[string]string{
				"func PipeLangEcho(p0 string) string {":         "v810Trace=append(v810Trace,\"E:\"+p0)",
				"func PipeLangCheck(p0 string, p1 bool) bool {": "v810Trace=append(v810Trace,\"C:\"+p0)",
			} {
				if strings.Count(observed, marker) != 1 {
					t.Fatal("instrumentation target absent")
				}
				observed = strings.Replace(observed, marker, marker+"\n"+probe, 1)
			}
			var cases strings.Builder
			for mask := 0; mask < 8; mask++ {
				flags := []bool{mask&1 != 0, mask&2 != 0, mask&4 != 0}
				path, value, node := "R", "valueR", tree
				trace := []string{"E:" + value}
				for depth := 0; node != nil; depth++ {
					trace = append(trace, "C:"+path)
					if flags[depth] {
						path += "T"
						node = node.yes
					} else {
						path += "F"
						node = node.no
					}
					value += path
					trace = append(trace, "E:"+value)
				}
				var quoted []string
				for _, v := range trace {
					quoted = append(quoted, fmt.Sprintf("%q", v))
				}
				fmt.Fprintf(&cases, "v810Trace=nil\nPipeLangSelect(\"value\",%t,%t,%t)\nif !reflect.DeepEqual(v810Trace,[]string{%s}) {t.Fatalf(\"order mask %d: %%v\",v810Trace)}\n", flags[0], flags[1], flags[2], strings.Join(quoted, ","), mask)
			}
			compileAndRunGeneratedGoFiles(t, []byte(observed), []byte(fmt.Sprintf("package %s\nimport (\"testing\";\"reflect\")\nvar v810Trace []string\nfunc TestOrder(t *testing.T){%s}\n", gobackend.PackageName, cases.String())))
		})
	}
}

func TestV810InheritedExpressionCapabilities(t *testing.T) {
	for _, source := range []string{
		`public Class Root { public Optional<string> Run(Optional<string> value) => some(propagate(value)); }`,
		`public Class Root { public string Run(Optional<string> value) => match(value){ some(item) => item, none => "empty" }; }`,
		`public Class Root { public string Echo(string value) => value; public string Run(string value) => Echo(trim(value)); }`,
		`public Class Root { public Result<int,ArithmeticError> Echo(Result<int,ArithmeticError> carrier) => carrier; public Result<int,ArithmeticError> Run(Result<int,ArithmeticError> carrier) => Echo(carrier); }`,
	} {
		old := reviewCore(t, PipeLangLanguageContractV800, source, "Run")
		current := reviewCore(t, PipeLangLanguageContractV810, source, "Run")
		oldGo, err := gobackend.Generate(old)
		if err != nil {
			t.Fatal(err)
		}
		newGo, err := gobackend.Generate(current)
		if err != nil || !bytes.Equal(oldGo, newGo) {
			t.Fatal("inherited expression lowering changed")
		}
		admissionOutcomes(t, current)
		reviewCompileGenerated(t, current)
	}
}
