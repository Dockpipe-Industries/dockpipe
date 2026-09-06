package pipelang

import (
	"bytes"
	"dockpipe/src/lib/pipelang/coreeval"
	"dockpipe/src/lib/pipelang/coreir"
	"dockpipe/src/lib/pipelang/gobackend"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"
)

const nestedTerminalLeafReturnsSource = `public Class Choices {
 public string Select(string raw, bool pick, bool finish, bool enabled) {
  string first = pick ? trim(raw) : raw;
  if(enabled){
   string second = finish && first != "" ? first + "!" : first;
   string unused = pick ? second : raw;
   return pick ? (finish ? second : first) : (finish ? first : raw);
  }else{return finish ? (pick ? first : raw) : raw;}
 }
}`

func TestV890NestedTerminalLeafReturnsAdmission(t *testing.T) {
	input := semanticTestModuleSet("compiler.selfhosting", []ModuleInput{testModule("compiler.selfhosting", "leaf.pipe", nestedTerminalLeafReturnsSource)}, nil)
	input.LanguageContract = LanguageContract("v0.89.0")
	if err := AnalyzeSemanticModuleSet(input).Error(); err != nil {
		t.Fatal(err)
	}
}

func TestV890NestedTerminalLeafReturnsSubsets(t *testing.T) {
	trees := terminalTrees(3)[1:]
	if len(trees) != 25 {
		t.Fatal("shape inventory drift")
	}
	for shape := 0; shape < 100; shape++ {
		tree := trees[shape/4]
		variant := shape % 4
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
									return "return " + v890Choice(path, (variant+i)%4) + ";"
								}
							}
							return fmt.Sprintf("return %q;", path)
						}
						return fmt.Sprintf("if(d%d){%s}else{%s}", len(path)-1, emit(n.yes, path+"T"), emit(n.no, path+"F"))
					}
					fmt.Fprintf(&source, "public string %s(bool d0,bool d1,bool d2,bool choose,bool yes,bool no){%s}", name, emit(tree, "R"))
				}
				source.WriteString("}")
				analysis, program := conditionalLocalTreeProgramVersion(t, PipeLangLanguageContractV890, source.String(), methods)
				_, again := conditionalLocalTreeProgramVersion(t, PipeLangLanguageContractV890, source.String(), methods)
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
					if typed.LanguageContract != coreir.LanguageContractV890 {
						t.Fatal("HIR contract")
					}
					for mask := 0; mask < 64; mask++ {
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
									if (variant+i)%4&1 != 0 {
										if mask&16 != 0 {
											want += "Y"
										} else {
											want += "N"
										}
									}
								} else {
									want += "F"
									if (variant+i)%4&2 != 0 {
										if mask&32 != 0 {
											want += "Y"
										} else {
											want += "N"
										}
									}
								}
							}
						}
						args := []coreeval.Value{}
						for bit := 0; bit < 6; bit++ {
							args = append(args, coreeval.Value{Type: f.Parameters[bit].Type, Bool: mask&(1<<bit) != 0})
						}
						got, err := coreeval.EvaluateProgram(program, f.Identity, args)
						if err != nil || !got.OK || got.Value.String != want {
							t.Fatalf("subset %d mask %d: %#v %v want %s", subset, mask, got, err, want)
						}
						fmt.Fprintf(&checks, "if got:=PipeLang%s(%t,%t,%t,%t,%t,%t);got!=%q{t.Fatal(got)}\n", name, mask&1 != 0, mask&2 != 0, mask&4 != 0, mask&8 != 0, mask&16 != 0, mask&32 != 0, want)
						outcomes++
					}
				}
				compileAndRunGeneratedGoFiles(t, generated, []byte(fmt.Sprintf("package %s\nimport \"testing\"\nfunc TestSubsets(t *testing.T){%s}", gobackend.PackageName, checks.String())))
			}
			t.Logf("%d leaves, %d subsets, %d evaluator/Go outcomes", len(leaves), 1<<len(leaves), outcomes)
		})
	}
}

// Rotate all four bounded return shapes across leaves; the subset matrix then
// crosses all statement shapes, selected-leaf subsets and independent decisions.
func v890Choice(path string, shape int) string {
	yes, no := fmt.Sprintf("%q", path+"T"), fmt.Sprintf("%q", path+"F")
	if shape&1 != 0 {
		yes = fmt.Sprintf("(yes ? %q : %q)", path+"TY", path+"TN")
	}
	if shape&2 != 0 {
		no = fmt.Sprintf("(no ? %q : %q)", path+"FY", path+"FN")
	}
	return "choose ? " + yes + " : " + no
}
func TestV890NestedTerminalLeafReturnsLayouts(t *testing.T) {
	trees := terminalTrees(3)[1:]
	if len(trees) != 25 {
		t.Fatal("shape inventory drift")
	}
	testFiniteConditionalLocalsLayouts(t, PipeLangLanguageContractV890, trees, true)
}
func TestV890NestedTerminalLeafReturnsTypes(t *testing.T) {
	for _, zero := range []bool{false, true} {
		t.Run(fmt.Sprint(zero), func(t *testing.T) { testConditionalLocalsTypeMatrix(t, PipeLangLanguageContractV890, zero) })
	}
	testConditionalLocalsDifferentTypes(t, PipeLangLanguageContractV890)
}
func TestV890NestedTerminalLeafReturnsCarriers(t *testing.T) {
	for _, zero := range []bool{false, true} {
		t.Run(fmt.Sprint(zero), func(t *testing.T) { testConditionalLocalsCarrierAndHostValues(t, PipeLangLanguageContractV890, zero) })
	}
}

func TestV890NestedTerminalLeafConditionalReturnsSourceRejection(t *testing.T) {
	for _, tc := range []struct{ name, before, after string }{
		{"condition type", "return pick ? second", "return raw ? second"},
		{"arm type", "? second : first", "? true : first"},
		{"missing arm", "? second : first", "? second :"},
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
			input.LanguageContract = PipeLangLanguageContractV890
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

func TestV890NestedTerminalLeafReturnsMalformedCore(t *testing.T) {
	for _, mutation := range []string{"missing local", "missing initializer", "missing continuation", "missing conditional", "missing condition", "missing true", "missing false", "inner missing true", "inner condition type", "arm type", "return type", "self", "forward", "position", "duplicate", "depth three", "nested initializer", "nested condition", "inner nested condition", "terminal inner", "return argument", "initializer argument", "unknown"} {
		t.Run(mutation, func(t *testing.T) {
			_, p := conditionalLocalTreeProgramVersion(t, PipeLangLanguageContractV890, nestedTerminalLeafReturnsSource, []string{"Select"})
			f := &p.Functions[0]
			first := f.Body.ImmutableLocal
			second := first.Return.Conditional.WhenTrue.ImmutableLocal
			third := second.Return.ImmutableLocal
			returned := third.Return
			c := returned.Conditional
			inner := c.WhenTrue.Conditional
			choice := func() *coreir.Expr {
				return &coreir.Expr{Kind: coreir.ExprConditional, Type: returned.Type, Conditional: &coreir.Conditional{Condition: inner.Condition, WhenTrue: inner.WhenTrue, WhenFalse: inner.WhenFalse}}
			}
			switch mutation {
			case "missing local":
				f.Body.ImmutableLocal = nil
			case "missing initializer":
				third.Initializer = nil
			case "missing continuation":
				third.Return = nil
			case "missing conditional":
				returned.Conditional = nil
			case "missing condition":
				c.Condition = nil
			case "missing true":
				c.WhenTrue = nil
			case "missing false":
				c.WhenFalse = nil
			case "inner missing true":
				inner.WhenTrue = nil
			case "inner condition type":
				inner.Condition = inner.WhenTrue
			case "arm type":
				inner.WhenTrue = inner.Condition
			case "return type":
				returned.Type = inner.Condition.Type
			case "self":
				third.Initializer.Conditional.WhenTrue = &coreir.Expr{Kind: coreir.ExprReference, Type: third.Type, Parameter: &third.Position}
			case "forward":
				first.Initializer.Conditional.WhenTrue = inner.WhenTrue
			case "position":
				third.Position = 999
			case "duplicate":
				third.Name = first.Name
			case "depth three":
				inner.WhenTrue = choice()
			case "nested initializer":
				third.Initializer.Conditional.WhenTrue = choice()
			case "nested condition", "inner nested condition":
				b := &coreir.Expr{Kind: coreir.ExprConditional, Type: c.Condition.Type, Conditional: &coreir.Conditional{Condition: c.Condition, WhenTrue: c.Condition, WhenFalse: c.Condition}}
				if mutation == "nested condition" {
					c.Condition = b
				} else {
					inner.Condition = b
				}
			case "terminal inner":
				inner.TerminalStatement = true
			case "return argument":
				third.Return = &coreir.Expr{Kind: coreir.ExprTextTrim, Type: returned.Type, TextTrim: &coreir.TextTrim{Value: returned}}
			case "initializer argument":
				init := third.Initializer
				third.Initializer = &coreir.Expr{Kind: coreir.ExprTextTrim, Type: init.Type, TextTrim: &coreir.TextTrim{Value: init}}
			case "unknown":
				ref := 999
				copy := *inner.WhenTrue
				copy.Parameter = &ref
				inner.WhenTrue = &copy
			}
			assertAdmissionRejected(t, p, "")
		})
	}
}
func TestV890NestedTerminalLeafReturnsVersionBoundary(t *testing.T) {
	for version := 1; version <= 88; version++ {
		contract := LanguageContract(fmt.Sprintf("v0.%d.0", version))
		input := semanticTestModuleSet("compiler.selfhosting", []ModuleInput{testModule("compiler.selfhosting", "nested.pipe", nestedTerminalLeafReturnsSource)}, nil)
		input.LanguageContract = contract
		if AnalyzeSemanticModuleSet(input).Error() == nil {
			t.Fatalf("%s admitted nested return", contract)
		}
		_, p := conditionalLocalTreeProgramVersion(t, PipeLangLanguageContractV890, nestedTerminalLeafReturnsSource, []string{"Select"})
		if err := coreir.ValidateFunction(p.Functions[0]); err != nil {
			t.Fatal(err)
		}
		p.LanguageContract = string(contract)
		assertAdmissionRejected(t, p, "")
	}
	for _, contract := range []string{"v0.99.0", "unknown"} {
		_, p := conditionalLocalTreeProgramVersion(t, PipeLangLanguageContractV890, nestedTerminalLeafReturnsSource, []string{"Select"})
		p.LanguageContract = contract
		assertAdmissionRejected(t, p, "")
	}
}

func TestV890NestedTerminalLeafReturnsInheritance(t *testing.T) {
	for _, source := range []string{nestedStraightLineReturnsSource, terminalLeafConditionalReturnsSource, conditionalReturnCompositionSource, straightLineConditionalLocalsSource, finiteConditionalLocalsSource, twoConditionalLocalsSource, `public Class Choices {public string Select(string raw,bool pick)=>pick ? raw : trim(raw);}`} {
		var baseline [][]byte
		for _, contract := range []LanguageContract{PipeLangLanguageContractV880, PipeLangLanguageContractV890, PipeLangLanguageContractV900, PipeLangLanguageContractV910} {
			a, p := conditionalLocalTreeProgramVersion(t, contract, source, []string{"Select"})
			h, err := LowerSemanticMethodToHIR(a, semanticMethodNamed(t, a, "Select").Identity)
			if err != nil {
				t.Fatal(err)
			}
			projection, err := BuildSemanticProjection(a)
			if err != nil {
				t.Fatal(err)
			}
			g, err := gobackend.Generate(p)
			if err != nil {
				t.Fatal(err)
			}
			h.LanguageContract = coreir.LanguageContractV870
			p.LanguageContract = coreir.LanguageContractV870
			projection.LanguageContract = PipeLangLanguageContractV870
			artifacts := [][]byte{g}
			for _, v := range []any{h, p, projection} {
				b, err := json.Marshal(v)
				if err != nil {
					t.Fatal(err)
				}
				artifacts = append(artifacts, b)
			}
			if baseline == nil {
				baseline = artifacts
			} else if !reflect.DeepEqual(baseline, artifacts) {
				t.Fatal("inherited HIR/Core/semantic/Go changed")
			}
		}
	}
}

func TestV890NestedTerminalLeafReturnsComputedCarriers(t *testing.T) {
	for _, unused := range []bool{false, true} {
		t.Run(fmt.Sprint(unused), func(t *testing.T) {
			local := ""
			if unused {
				local = "Result<int,ArithmeticError> unused=Make(value);"
			}
			source := `public Class Choices {public Result<int,ArithmeticError> Make(int value)=>value+1;
 public Result<int,ArithmeticError> Select(int value,bool a,bool b,bool c){` + local + `if(a){return b ? (c ? Make(value) : Make(value)) : Make(0);}else{return c ? Make(1) : (b ? Make(2) : Make(2));}}}`
			_, program := conditionalLocalTreeProgramVersion(t, PipeLangLanguageContractV890, source, []string{"Select"})
			f := coreFunctionNamed(t, program, "Select")
			var checks strings.Builder
			for _, value := range []int64{-9223372036854775808, 0, 9223372036854775807} {
				for mask := 0; mask < 8; mask++ {
					want, success := int64(3), true
					if mask&1 != 0 {
						want = 1
						if mask&2 != 0 {
							success = value != 9223372036854775807
							if success {
								want = value + 1
							}
						}
					} else if mask&4 != 0 {
						want = 2
					}
					args := []coreeval.Value{{Type: f.Parameters[0].Type, Int: value}}
					for bit := 0; bit < 3; bit++ {
						args = append(args, coreeval.Value{Type: f.Parameters[bit+1].Type, Bool: mask&(1<<bit) != 0})
					}
					got, err := coreeval.EvaluateProgram(program, f.Identity, args)
					if err != nil || got.OK != success || (success && got.Value.Int != want) || (!success && got.Error != coreir.ArithmeticOverflow) {
						t.Fatalf("%d/%d: %#v %v", value, mask, got, err)
					}
					fmt.Fprintf(&checks, "{got:=PipeLangSelect(%d,%t,%t,%t);if got.OK!=%t || (got.OK && got.Value!=%d) || (!got.OK && string(got.Error)!=\"overflow\"){t.Fatal(got)}}\n", value, mask&1 != 0, mask&2 != 0, mask&4 != 0, success, want)
				}
			}
			generated, err := gobackend.Generate(program)
			if err != nil {
				t.Fatal(err)
			}
			compileAndRunGeneratedGoFiles(t, generated, []byte(fmt.Sprintf("package %s\nimport \"testing\"\nfunc TestComputed(t *testing.T){%s}", gobackend.PackageName, checks.String())))
		})
	}
}

func TestV890NestedTerminalLeafReturnsRefusesEmbeddedLocals(t *testing.T) {
	for _, placement := range []string{"return arm", "inner condition", "outer condition", "initializer arm", "statement condition", "ordinary sibling operand"} {
		t.Run(placement, func(t *testing.T) {
			_, p := conditionalLocalTreeProgramVersion(t, PipeLangLanguageContractV890, nestedTerminalLeafReturnsSource, []string{"Select"})
			f := &p.Functions[0]
			first := f.Body.ImmutableLocal
			third := first.Return.Conditional.WhenTrue.ImmutableLocal.Return.ImmutableLocal
			returned := third.Return
			choice := returned.Conditional
			position := len(f.Parameters) + 3
			target := &choice.WhenTrue.Conditional.WhenTrue
			switch placement {
			case "inner condition":
				target = &choice.WhenTrue.Conditional.Condition
			case "outer condition":
				target = &choice.Condition
			case "ordinary sibling operand":
				branch := first.Return.Conditional
				value := branch.WhenFalse.Conditional.WhenFalse
				branch.WhenFalse = &coreir.Expr{Kind: coreir.ExprTextTrim, Type: value.Type, TextTrim: &coreir.TextTrim{Value: value}}
				target = &branch.WhenFalse.TextTrim.Value
				position = len(f.Parameters) + 1
			case "statement condition":
				target = &first.Return.Conditional.Condition
				position = len(f.Parameters) + 1
			case "initializer arm":
				target = &third.Initializer.Conditional.WhenTrue
				position--
			}
			value := *target
			reference := &coreir.Expr{Kind: coreir.ExprReference, Type: value.Type, Parameter: &position}
			*target = &coreir.Expr{Kind: coreir.ExprImmutableLocal, Type: value.Type, ImmutableLocal: &coreir.ImmutableLocal{Name: "hidden", Position: position, Type: value.Type, Initializer: value, Return: reference}}
			if err := coreir.ValidateFunction(*f); err != nil {
				t.Fatalf("internal Core must retain local expressions: %v", err)
			}
			assertAdmissionRejected(t, p, "")
		})
	}
}

func TestV890NestedTerminalLeafReturnsNestedSourceRejection(t *testing.T) {
	for _, tc := range []struct{ name, before, after string }{
		{"third level", "finish ? second : first", "finish ? (pick ? second : first) : first"},
		{"inner condition", "finish ? second : first", "(pick ? finish : enabled) ? second : first"},
		{"inner argument", "finish ? second : first", "finish ? trim(pick ? second : first) : first"},
		{"inner type", "finish ? second : first", "finish ? true : first"},
		{"inner missing", "finish ? second : first", "finish ? second :"},
		{"inner unknown", "finish ? second : first", "finish ? missing : first"},
		{"sibling escape", "(pick ? first : raw)", "(pick ? second : raw)"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source := strings.Replace(nestedTerminalLeafReturnsSource, tc.before, tc.after, 1)
			if source == nestedTerminalLeafReturnsSource {
				t.Fatal("mutation missed")
			}
			input := semanticTestModuleSet("compiler.selfhosting", []ModuleInput{testModule("compiler.selfhosting", "nested.pipe", source)}, nil)
			input.LanguageContract = PipeLangLanguageContractV890
			ds, ok := AsDiagnostics(AnalyzeSemanticModuleSet(input).Error())
			if !ok || len(ds) == 0 {
				t.Fatal("missing rejection")
			}
			for _, d := range ds {
				if !d.Primary.IsValid() || d.Primary.File != "nested.pipe" || d.Primary.End > len(source) {
					t.Fatal("diagnostic span")
				}
			}
		})
	}
}

func TestV890NestedTerminalLeafReturnsRepresentations(t *testing.T) {
	for _, source := range []string{nestedTerminalLeafReturnsSource, `public Class Choices {public string Select(string raw,bool pick,bool finish,bool enabled){if(enabled){return enabled ? (finish ? raw : "a") : (pick ? "b" : "c");}else{return raw;}}}`} {
		a, p := conditionalLocalTreeProgramVersion(t, PipeLangLanguageContractV890, source, []string{"Select"})
		h, err := LowerSemanticMethodToHIR(a, semanticMethodNamed(t, a, "Select").Identity)
		if err != nil {
			t.Fatal(err)
		}
		he := h.Functions[len(h.Functions)-1].Body
		ce := p.Functions[0].Body
		for he.ImmutableLocal != nil {
			if ce.ImmutableLocal == nil {
				t.Fatal("missing Core local")
			}
			he = *he.ImmutableLocal.Return
			ce = *ce.ImmutableLocal.Return
		}
		if he.Conditional == nil || ce.Conditional == nil || !he.Conditional.TerminalStatement || !ce.Conditional.TerminalStatement {
			t.Fatal("statement markers")
		}
		he = *he.Conditional.WhenTrue
		ce = *ce.Conditional.WhenTrue
		for he.ImmutableLocal != nil {
			if ce.ImmutableLocal == nil {
				t.Fatal("missing branch local")
			}
			he = *he.ImmutableLocal.Return
			ce = *ce.ImmutableLocal.Return
		}
		if he.Conditional == nil || ce.Conditional == nil || he.Conditional.TerminalStatement || ce.Conditional.TerminalStatement {
			t.Fatal("root choice marker")
		}
		for _, pair := range [][2]bool{{he.Conditional.WhenTrue.Conditional != nil, ce.Conditional.WhenTrue.Conditional != nil}, {he.Conditional.WhenFalse.Conditional != nil, ce.Conditional.WhenFalse.Conditional != nil}} {
			if !pair[0] || !pair[1] {
				t.Fatal("missing nested HIR/Core choice")
			}
		}
	}
}
