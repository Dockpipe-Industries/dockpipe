package pipelang

import (
	"bytes"
	"dockpipe/src/lib/pipelang/coreeval"
	"dockpipe/src/lib/pipelang/coreir"
	"dockpipe/src/lib/pipelang/gobackend"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

const terminalLeafConditionalReturnsSource = `public Class Choices {
 public string Select(string raw, bool pick, bool finish, bool enabled) {
  string first = pick ? trim(raw) : raw;
  if(enabled){
   string second = finish && first != "" ? first + "!" : first;
   string unused = pick ? second : raw;
   return pick ? second : first;
  }else{return finish ? first : raw;}
 }
}`

func TestV870TerminalLeafConditionalReturnsAdmission(t *testing.T) {
	input := semanticTestModuleSet("compiler.selfhosting", []ModuleInput{testModule("compiler.selfhosting", "leaf.pipe", terminalLeafConditionalReturnsSource)}, nil)
	input.LanguageContract = LanguageContract("v0.87.0")
	if err := AnalyzeSemanticModuleSet(input).Error(); err != nil {
		t.Fatal(err)
	}
}

func TestV870TerminalLeafConditionalReturnsLayouts(t *testing.T) {
	trees := terminalTrees(3)[1:]
	if len(trees) != 25 {
		t.Fatal("shape inventory drift")
	}
	testFiniteConditionalLocalsLayouts(t, PipeLangLanguageContractV870, trees, true)
}

// Exhaust every leaf subset, including ordinary-only inherited trees, without a
// local prerequisite. Each path and both independent return choices execute.
func TestV870TerminalLeafConditionalReturnsSubsets(t *testing.T) {
	trees := terminalTrees(3)[1:]
	if len(trees) != 25 {
		t.Fatal("shape inventory drift")
	}
	for shape, tree := range trees {
		t.Run(fmt.Sprint(shape), func(t *testing.T) {
			leaves := []string{}
			var inventory func(*terminalTree, string)
			inventory = func(n *terminalTree, p string) {
				if n == nil {
					leaves = append(leaves, p)
					return
				}
				inventory(n.yes, p+"T")
				inventory(n.no, p+"F")
			}
			inventory(tree, "R")
			outcomes := 0
			for start := 0; start < 1<<len(leaves); start += 16 {
				end := start + 16
				if end > 1<<len(leaves) {
					end = 1 << len(leaves)
				}
				var source, checks strings.Builder
				source.WriteString("public Class Choices {")
				methods := []string{}
				for subset := start; subset < end; subset++ {
					name := fmt.Sprintf("Select%d", subset)
					methods = append(methods, name)
					var emit func(*terminalTree, string) string
					emit = func(n *terminalTree, path string) string {
						if n == nil {
							for i, p := range leaves {
								if p == path && subset&(1<<i) != 0 {
									return fmt.Sprintf("return choose ? %q : %q;", path+"T", path+"F")
								}
							}
							return fmt.Sprintf("return %q;", path)
						}
						return fmt.Sprintf("if(d%d){%s}else{%s}", len(path)-1, emit(n.yes, path+"T"), emit(n.no, path+"F"))
					}
					fmt.Fprintf(&source, "public string %s(bool d0,bool d1,bool d2,bool choose){%s}", name, emit(tree, "R"))
				}
				source.WriteString("}")
				analysis, program := conditionalLocalTreeProgramVersion(t, PipeLangLanguageContractV870, source.String(), methods)
				_, again := conditionalLocalTreeProgramVersion(t, PipeLangLanguageContractV870, source.String(), methods)
				a, _ := json.Marshal(program)
				b, _ := json.Marshal(again)
				if !bytes.Equal(a, b) {
					t.Fatal("Core nondeterminism")
				}
				generated, err := gobackend.Generate(program)
				if err != nil {
					t.Fatal(err)
				}
				for subset := start; subset < end; subset++ {
					name := fmt.Sprintf("Select%d", subset)
					f := coreFunctionNamed(t, program, name)
					typed, err := LowerSemanticMethodToHIR(analysis, semanticMethodNamed(t, analysis, name).Identity)
					if err != nil {
						t.Fatal(err)
					}
					if typed.LanguageContract != coreir.LanguageContractV870 {
						t.Fatal("HIR contract")
					}
					for mask := 0; mask < 16; mask++ {
						node, path := tree, "R"
						for depth := 0; node != nil; depth++ {
							if mask&(1<<depth) != 0 {
								node = node.yes
								path += "T"
							} else {
								node = node.no
								path += "F"
							}
						}
						want := path
						for i, p := range leaves {
							if p == path && subset&(1<<i) != 0 {
								if mask&8 != 0 {
									want += "T"
								} else {
									want += "F"
								}
							}
						}
						args := []coreeval.Value{}
						for bit := 0; bit < 4; bit++ {
							args = append(args, coreeval.Value{Type: f.Parameters[bit].Type, Bool: mask&(1<<bit) != 0})
						}
						got, err := coreeval.EvaluateProgram(program, f.Identity, args)
						if err != nil || !got.OK || got.Value.String != want {
							t.Fatalf("subset %d mask %d: %#v %v want %s", subset, mask, got, err, want)
						}
						fmt.Fprintf(&checks, "if got:=PipeLang%s(%t,%t,%t,%t);got!=%q{t.Fatal(got)}\n", name, mask&1 != 0, mask&2 != 0, mask&4 != 0, mask&8 != 0, want)
						outcomes++
					}
				}
				compileAndRunGeneratedGoFiles(t, generated, []byte(fmt.Sprintf("package %s\nimport \"testing\"\nfunc TestSubsets(t *testing.T){%s}", gobackend.PackageName, checks.String())))
			}
			t.Logf("%d leaves, %d subsets, %d evaluator/Go outcomes", len(leaves), 1<<len(leaves), outcomes)
		})
	}
}

func TestV870TerminalLeafConditionalReturnsTypes(t *testing.T) {
	testConditionalLocalsTypeMatrix(t, PipeLangLanguageContractV870)
	testConditionalLocalsDifferentTypes(t, PipeLangLanguageContractV870)
}
func TestV870TerminalLeafConditionalReturnsCarriers(t *testing.T) {
	testConditionalLocalsCarrierAndHostValues(t, PipeLangLanguageContractV870)
}

func TestV870TerminalLeafConditionalReturnsSourceRejection(t *testing.T) {
	for _, tc := range []struct{ name, before, after string }{
		{"condition type", "return pick ? second", "return raw ? second"},
		{"arm type", "? second : first", "? true : first"},
		{"missing arm", "? second : first", "? second :"},
		{"nested true", "? second : first", "? (finish ? second : first) : first"},
		{"nested false", "? second : first", "? second : (finish ? first : raw)"},
		{"nested condition", "return pick ? second", "return (pick ? enabled : finish) ? second"},
		{"argument", "return pick ? second : first;", "return trim(pick ? second : first);"},
		{"initializer argument", "pick ? trim(raw) : raw", "trim(pick ? raw : raw)"},
		{"tree condition", "if(enabled)", "if(pick ? enabled : finish)"},
		{"nested initializer", "pick ? trim(raw) : raw", "pick ? (finish ? raw : raw) : raw"},
		{"sibling escape", "return finish ? first : raw", "return finish ? second : raw"},
		{"unknown", "? second : first", "? missing : first"},
		{"self", "finish && first", "finish && second"},
		{"forward", "pick ? trim(raw)", "pick ? second"},
		{"shadow", "string second", "string first"},
		{"duplicate", "string unused", "string second"},
		{"assignment", "string unused = pick ? second : raw;", "second=raw;"},
		{"inference", "string unused", "var unused"},
		{"early return", "string second", "return raw; string second"},
		{"fallthrough", "return pick ? second : first;", ""},
		{"propagate", "? second : first", "? propagate(second) : first"},
		{"match", "? second : first", "? match(second){some(v)=>v,none=>raw} : first"},
		{"depth four", "return pick ? second : first;", "if(pick){if(pick){if(pick){return pick ? second : first;}else{return first;}}else{return first;}}else{return first;}"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source := strings.Replace(terminalLeafConditionalReturnsSource, tc.before, tc.after, 1)
			if source == terminalLeafConditionalReturnsSource {
				t.Fatal("mutation missed")
			}
			input := semanticTestModuleSet("compiler.selfhosting", []ModuleInput{testModule("compiler.selfhosting", "leaf.pipe", source)}, nil)
			input.LanguageContract = PipeLangLanguageContractV870
			ds, ok := AsDiagnostics(AnalyzeSemanticModuleSet(input).Error())
			if !ok || len(ds) == 0 {
				t.Fatal("missing rejection")
			}
			for _, d := range ds {
				if !d.Primary.IsValid() || d.Primary.File != "leaf.pipe" || d.Primary.End > len(source) {
					t.Fatal("diagnostic span")
				}
			}
		})
	}
}

func TestV870TerminalLeafConditionalReturnsMalformedCore(t *testing.T) {
	for _, mutation := range []string{"missing condition", "missing true", "missing false", "condition type", "arm type", "nested true", "nested false", "nested condition", "terminal initializer", "sibling escape", "position", "unknown", "argument", "initializer argument", "tree condition", "depth four"} {
		t.Run(mutation, func(t *testing.T) {
			_, program := conditionalLocalTreeProgramVersion(t, PipeLangLanguageContractV870, terminalLeafConditionalReturnsSource, []string{"Select"})
			root := program.Functions[0].Body.ImmutableLocal
			branch := root.Return.Conditional
			local := branch.WhenTrue.ImmutableLocal
			unused := local.Return.ImmutableLocal
			leaf := unused.Return.Conditional
			nested := func() *coreir.Expr {
				return &coreir.Expr{Kind: coreir.ExprConditional, Type: unused.Return.Type, Conditional: &coreir.Conditional{Condition: leaf.Condition, WhenTrue: leaf.WhenTrue, WhenFalse: leaf.WhenFalse}}
			}
			switch mutation {
			case "missing condition":
				leaf.Condition = nil
			case "missing true":
				leaf.WhenTrue = nil
			case "missing false":
				leaf.WhenFalse = nil
			case "condition type":
				leaf.Condition = leaf.WhenTrue
			case "arm type":
				leaf.WhenTrue = leaf.Condition
			case "nested true":
				leaf.WhenTrue = nested()
			case "nested false":
				leaf.WhenFalse = nested()
			case "nested condition":
				leaf.Condition = &coreir.Expr{Kind: coreir.ExprConditional, Type: leaf.Condition.Type, Conditional: &coreir.Conditional{Condition: leaf.Condition, WhenTrue: leaf.Condition, WhenFalse: leaf.Condition}}
			case "terminal initializer":
				root.Initializer.Conditional.TerminalStatement = true
			case "sibling escape":
				branch.WhenFalse.Conditional.WhenTrue = leaf.WhenTrue
			case "position":
				unused.Position = 999
			case "unknown":
				copy := *leaf.WhenTrue
				ref := 999
				copy.Parameter = &ref
				leaf.WhenTrue = &copy
			case "argument":
				init := *unused.Return
				unused.Return = &coreir.Expr{Kind: coreir.ExprTextTrim, Type: init.Type, TextTrim: &coreir.TextTrim{Value: &init}}
			case "initializer argument":
				init := root.Initializer
				root.Initializer = &coreir.Expr{Kind: coreir.ExprTextTrim, Type: init.Type, TextTrim: &coreir.TextTrim{Value: init}}
			case "tree condition":
				branch.Condition = &coreir.Expr{Kind: coreir.ExprConditional, Type: leaf.Condition.Type, Conditional: &coreir.Conditional{Condition: leaf.Condition, WhenTrue: leaf.Condition, WhenFalse: leaf.Condition}}
			case "depth four":
				expr := unused.Return
				for i := 0; i < 3; i++ {
					expr = &coreir.Expr{Kind: coreir.ExprConditional, Type: expr.Type, Conditional: &coreir.Conditional{TerminalStatement: true, Condition: leaf.Condition, WhenTrue: expr, WhenFalse: leaf.WhenFalse}}
				}
				unused.Return = expr
			}
			assertAdmissionRejected(t, program, "")
		})
	}
}

func TestV870TerminalLeafConditionalReturnsVersionBoundary(t *testing.T) {
	for _, contract := range []LanguageContract{PipeLangLanguageContractV810, PipeLangLanguageContractV820, PipeLangLanguageContractV830, PipeLangLanguageContractV840, PipeLangLanguageContractV850, PipeLangLanguageContractV860, "v0.999.0", "unknown"} {
		input := semanticTestModuleSet("compiler.selfhosting", []ModuleInput{testModule("compiler.selfhosting", "leaf.pipe", terminalLeafConditionalReturnsSource)}, nil)
		input.LanguageContract = contract
		if AnalyzeSemanticModuleSet(input).Error() == nil {
			t.Fatalf("%s accepted new source", contract)
		}
		_, program := conditionalLocalTreeProgramVersion(t, PipeLangLanguageContractV870, terminalLeafConditionalReturnsSource, []string{"Select"})
		if err := coreir.ValidateFunction(program.Functions[0]); err != nil {
			t.Fatal(err)
		}
		program.LanguageContract = string(contract)
		assertAdmissionRejected(t, program, "")
	}
}

func TestV870TerminalLeafConditionalReturnsInheritance(t *testing.T) {
	for _, source := range []string{conditionalReturnCompositionSource, straightLineConditionalLocalsSource, finiteConditionalLocalsSource, twoConditionalLocalsSource} {
		var baseline []byte
		for _, contract := range []LanguageContract{PipeLangLanguageContractV860, PipeLangLanguageContractV870, PipeLangLanguageContractV880, PipeLangLanguageContractV890, PipeLangLanguageContractV900, PipeLangLanguageContractV910} {
			analysis, program := conditionalLocalTreeProgramVersion(t, contract, source, []string{"Select"})
			typed, err := LowerSemanticMethodToHIR(analysis, semanticMethodNamed(t, analysis, "Select").Identity)
			if err != nil {
				t.Fatal(err)
			}
			semantic, err := BuildSemanticProjection(analysis)
			if err != nil {
				t.Fatal(err)
			}
			generated, err := gobackend.Generate(program)
			if err != nil {
				t.Fatal(err)
			}
			typed.LanguageContract = coreir.LanguageContractV860
			program.LanguageContract = coreir.LanguageContractV860
			semantic.LanguageContract = PipeLangLanguageContractV860
			encoded, err := json.Marshal([]any{typed, program, semantic, generated})
			if err != nil {
				t.Fatal(err)
			}
			if baseline == nil {
				baseline = encoded
			} else if !bytes.Equal(baseline, encoded) {
				t.Fatal("inherited HIR/Core/semantic/Go changed")
			}
		}
	}
}

func TestV870TerminalLeafConditionalReturnsDependentCondition(t *testing.T) {
	source := strings.Replace(conditionalReturnCompositionSource, "return enabled && third != \"\" ? third : first;", `if(enabled){
 string result=enabled && third != "" ? third : first;
 return result != "" ? result : trim(result);
 }else{return first != "" ? first : trim(first);}`, 1)
	if source == conditionalReturnCompositionSource {
		t.Fatal("fixture replacement missed")
	}
	testConditionalLocalsDependentCondition(t, PipeLangLanguageContractV870, source)
}

func TestV870TerminalLeafConditionalReturnsRepresentations(t *testing.T) {
	analysis, program := conditionalLocalTreeProgramVersion(t, PipeLangLanguageContractV870, terminalLeafConditionalReturnsSource, []string{"Select"})
	typed, err := LowerSemanticMethodToHIR(analysis, semanticMethodNamed(t, analysis, "Select").Identity)
	if err != nil {
		t.Fatal(err)
	}
	h := typed.Functions[len(typed.Functions)-1].Body.ImmutableLocal.Return.Conditional
	c := program.Functions[0].Body.ImmutableLocal.Return.Conditional
	if h == nil || c == nil || !h.TerminalStatement || !c.TerminalStatement {
		t.Fatal("statement marker missing")
	}
	hl := h.WhenTrue.ImmutableLocal.Return.ImmutableLocal.Return.Conditional
	cl := c.WhenTrue.ImmutableLocal.Return.ImmutableLocal.Return.Conditional
	if hl == nil || cl == nil || hl.TerminalStatement || cl.TerminalStatement || h.WhenFalse.Conditional.TerminalStatement || c.WhenFalse.Conditional.TerminalStatement {
		t.Fatal("leaf return choice marker")
	}
}
