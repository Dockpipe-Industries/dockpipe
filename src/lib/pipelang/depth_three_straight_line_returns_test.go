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

const depthThreeStraightLineReturnsSource = `public Class Choices {
 public string Select(string raw,bool a,bool b,bool c) {
  string first=a ? (b ? trim(raw) : raw) : raw;
  return a ? (b ? (c ? first : raw) : raw) : raw;
 }
}`

func TestV930DepthThreeStraightLineReturnsAdmission(t *testing.T) {
	input := semanticTestModuleSet("compiler.selfhosting", []ModuleInput{testModule("compiler.selfhosting", "depth-three.pipe", depthThreeStraightLineReturnsSource)}, nil)
	input.LanguageContract = LanguageContract("v0.93.0")
	if err := AnalyzeSemanticModuleSet(input).Error(); err != nil {
		t.Fatal(err)
	}
}

func TestV930DepthThreeReturnsInheritance(t *testing.T) {
	for _, source := range []string{nestedArrowMethodsSource, nestedTerminalInitializersSource, nestedStraightLineInitializersSource, nestedTerminalLeafReturnsSource, nestedStraightLineReturnsSource, terminalLeafConditionalReturnsSource, conditionalReturnCompositionSource, straightLineConditionalLocalsSource, finiteConditionalLocalsSource, twoConditionalLocalsSource, `public Class Choices {public string Select(string raw,bool pick)=>pick ? raw : trim(raw);}`} {
		var baseline [][]byte
		for _, contract := range []LanguageContract{PipeLangLanguageContractV920, PipeLangLanguageContractV930} {
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
			h.LanguageContract = coreir.LanguageContractV920
			p.LanguageContract = coreir.LanguageContractV920
			projection.LanguageContract = PipeLangLanguageContractV920
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

func v930DeepenTypedReturn(t *testing.T, source string) string {
	t.Helper()
	pos := strings.LastIndex(source, " return ")
	if pos < 0 {
		t.Fatal("missing typed return")
	}
	head, tail := source[:pos], source[pos:]
	replacements := [][2]string{{"? shared : right", "? (pick ? shared : shared) : right"}, {"? left : right", "? (pick ? left : left) : right"}, {"? a : fallback", "? (first ? a : a) : fallback"}, {"? value : fallback", "? (first ? value : value) : fallback"}}
	for _, pair := range replacements {
		if strings.Contains(tail, pair[0]) {
			return head + strings.Replace(tail, pair[0], pair[1], 1)
		}
	}
	t.Fatal("typed depth-three replacement missed")
	return ""
}
func TestV930DepthThreeReturnsTypes(t *testing.T) {
	for _, zero := range []bool{false, true} {
		t.Run(fmt.Sprint(zero), func(t *testing.T) { testConditionalLocalsTypeMatrix(t, PipeLangLanguageContractV930, zero) })
	}
}
func TestV930DepthThreeReturnsCarriers(t *testing.T) {
	for _, zero := range []bool{false, true} {
		t.Run(fmt.Sprint(zero), func(t *testing.T) { testConditionalLocalsCarrierAndHostValues(t, PipeLangLanguageContractV930, zero) })
	}
}

// Heap positions give every possible condition an independent input bit.
func v930ChoiceSource(tree *terminalTree, index int, value string) string {
	if tree == nil {
		return fmt.Sprintf(`Echo(%s+"L%d")`, value, index)
	}
	return fmt.Sprintf(`Check("C%d",c%d) ? (%s) : (%s)`, index, index, v930ChoiceSource(tree.yes, index*2+1, value), v930ChoiceSource(tree.no, index*2+2, value))
}
func v930ChoiceWant(tree *terminalTree, mask int) (string, []string) {
	index := 0
	trace := []string{}
	for tree != nil {
		trace = append(trace, fmt.Sprintf("C%d", index))
		if mask&(1<<index) != 0 {
			tree = tree.yes
			index = index*2 + 1
		} else {
			tree = tree.no
			index = index*2 + 2
		}
	}
	return fmt.Sprintf("L%d", index), trace
}
func TestV930DepthThreeReturnsLayouts(t *testing.T) {
	shapes := terminalTrees(3)[1:]
	if len(shapes) != 25 {
		t.Fatal(len(shapes))
	}
	for shape, tree := range shapes {
		t.Run(fmt.Sprint(shape), func(t *testing.T) {
			// Zero locals and mixed dependent ordinary/depth-two locals, including an unused tail.
			for _, count := range []int{0, 1, 3} {
				for _, unused := range []bool{false, true} {
					var source strings.Builder
					source.WriteString(`public Class Choices {public string Echo(string value)=>value;public bool Check(string name,bool value)=>value;public string Select(string raw,bool q,bool r`)
					for i := 0; i < 7; i++ {
						fmt.Fprintf(&source, ",bool c%d", i)
					}
					source.WriteString("){")
					previous := "raw"
					for i := 0; i < count; i++ {
						if i%2 == 0 {
							fmt.Fprintf(&source, `string l%d=Check("q%d",q) ? (Check("r%d",r) ? Echo(%s+"T") : Echo(%s+"M")) : Echo(%s+"F");`, i, i, i, previous, previous, previous)
						} else {
							fmt.Fprintf(&source, `string l%d=Echo(%s+"O");`, i, previous)
						}
						if !unused || i < count-1 {
							previous = fmt.Sprintf("l%d", i)
						}
					}
					fmt.Fprintf(&source, "return %s;}}", v930ChoiceSource(tree, 0, previous))
					a, p := conditionalLocalTreeProgramVersion(t, PipeLangLanguageContractV930, source.String(), []string{"Select"})
					aa, pp := conditionalLocalTreeProgramVersion(t, PipeLangLanguageContractV930, source.String(), []string{"Select"})
					h, err := LowerSemanticMethodToHIR(a, semanticMethodNamed(t, a, "Select").Identity)
					if err != nil {
						t.Fatal(err)
					}
					hh, err := LowerSemanticMethodToHIR(aa, semanticMethodNamed(t, aa, "Select").Identity)
					if err != nil {
						t.Fatal(err)
					}
					projection, err := BuildSemanticProjection(a)
					if err != nil {
						t.Fatal(err)
					}
					repeated, err := BuildSemanticProjection(aa)
					if err != nil {
						t.Fatal(err)
					}
					if !reflect.DeepEqual(h, hh) || !reflect.DeepEqual(p, pp) || !reflect.DeepEqual(projection, repeated) {
						t.Fatal("nondeterministic representations")
					}
					generated, err := gobackend.Generate(p)
					if err != nil {
						t.Fatal(err)
					}
					again, err := gobackend.Generate(pp)
					if err != nil || !bytes.Equal(generated, again) {
						t.Fatal("nondeterministic Go")
					}
					f := coreFunctionNamed(t, p, "Select")
					var checks, orders strings.Builder
					for localMask := 0; localMask < 4; localMask++ {
						for mask := 0; mask < 128; mask++ {
							value := "raw"
							trace := []string{}
							for i := 0; i < count; i++ {
								suffix := "O"
								if i%2 == 0 {
									trace = append(trace, fmt.Sprintf("C:q%d", i))
									suffix = "F"
									if localMask&1 != 0 {
										trace = append(trace, fmt.Sprintf("C:r%d", i))
										suffix = "M"
										if localMask&2 != 0 {
											suffix = "T"
										}
									}
								}
								next := value + suffix
								trace = append(trace, "E:"+next)
								if !unused || i < count-1 {
									value = next
								}
							}
							suffix, path := v930ChoiceWant(tree, mask)
							for _, event := range path {
								trace = append(trace, "C:"+event)
							}
							want := value + suffix
							trace = append(trace, "E:"+want)
							args := []coreeval.Value{{Type: f.Parameters[0].Type, String: "raw"}}
							flags := []bool{localMask&1 != 0, localMask&2 != 0}
							for bit := 0; bit < 7; bit++ {
								flags = append(flags, mask&(1<<bit) != 0)
							}
							call := `PipeLangSelect("raw"`
							for bit, flag := range flags {
								args = append(args, coreeval.Value{Type: f.Parameters[bit+1].Type, Bool: flag})
								call += fmt.Sprintf(",%t", flag)
							}
							call += ")"
							got, err := coreeval.EvaluateProgram(p, f.Identity, args)
							if err != nil || !got.OK || got.Value.String != want {
								t.Fatalf("%d/%d: %#v %v want %q", localMask, mask, got, err, want)
							}
							fmt.Fprintf(&checks, "if got:=%s;got!=%q{t.Fatal(got)}\n", call, want)
							fmt.Fprintf(&orders, "v930Trace=nil;%s;if !reflect.DeepEqual(v930Trace,[]string{%s}){t.Fatal(v930Trace)}\n", call, quotedStrings(trace))
						}
					}
					compileAndRunGeneratedGoFiles(t, generated, []byte(fmt.Sprintf("package %s\nimport \"testing\"\nfunc TestValues(t *testing.T){%s}", gobackend.PackageName, checks.String())))
					observed := string(generated)
					for marker, probe := range map[string]string{"func PipeLangEcho(p0 string) string {": `v930Trace=append(v930Trace,"E:"+p0)`, "func PipeLangCheck(p0 string, p1 bool) bool {": `v930Trace=append(v930Trace,"C:"+p0)`} {
						if strings.Count(observed, marker) != 1 {
							t.Fatal("missing trace marker")
						}
						observed = strings.Replace(observed, marker, marker+"\n"+probe, 1)
					}
					compileAndRunGeneratedGoFiles(t, []byte(observed), []byte(fmt.Sprintf("package %s\nimport (\"testing\";\"reflect\")\nvar v930Trace []string\nfunc TestOrder(t *testing.T){%s}", gobackend.PackageName, orders.String())))
				}
			}
		})
	}
}

func TestV930DepthThreeReturnsComputedCarriers(t *testing.T) {
	for _, unused := range []bool{false, true} {
		t.Run(fmt.Sprint(unused), func(t *testing.T) {
			local := ""
			if unused {
				local = "Result<int,ArithmeticError> unused=a ? (b ? Make(value) : Make(value)) : (c ? Make(value) : Make(value));"
			}
			body := `a ? (b ? (c ? Make(value) : Make(0)) : Make(0)) : (c ? Make(1) : Make(2))`
			source := `public Class Choices {public Result<int,ArithmeticError> Make(int value)=>value+1;public Result<int,ArithmeticError> Choose(int value,bool a,bool b,bool c){return ` + body + `;}public Result<int,ArithmeticError> Select(int value,bool a,bool b,bool c){` + local + `return Choose(value,a,b,c);}}`

			if !unused {
				source = strings.Replace(source, "{return Choose(value,a,b,c);}", "=>Choose(value,a,b,c);", 1)
			}
			_, program := conditionalLocalTreeProgramVersion(t, PipeLangLanguageContractV930, source, []string{"Select"})
			f := coreFunctionNamed(t, program, "Select")
			var checks strings.Builder
			for _, value := range []int64{-9223372036854775808, 0, 9223372036854775807} {
				for mask := 0; mask < 8; mask++ {
					want, success := int64(3), true
					if mask&1 != 0 {
						want = 1
						if mask&2 != 0 && mask&4 != 0 {
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

func TestV930DepthThreeReturnsSourceRejection(t *testing.T) {
	prefix := `public Class Choices {public string Select(string raw,bool a,bool b,bool c) `
	deep := `a ? (b ? (c ? raw : "C") : "B") : "A"`
	cases := []string{
		`=>` + deep + `;`,
		`{string local=` + deep + `;return local;}`,
		`{if(a){return ` + deep + `;}else{return raw;}}`,
		`{string local=raw;if(a){return ` + deep + `;}else{return local;}}`,
		`{return a ? (b ? (c ? (a ? raw : "D") : "C") : "B") : "A";}`,
		`{return a ? raw : (b ? raw : (c ? raw : (a ? raw : "D")));}`,
		`{return (a ? b : c) ? (b ? (c ? raw : "C") : "B") : "A";}`,
		`{return a ? (b ? ((a ? b : c) ? raw : "C") : "B") : "A";}`,
		`{return a ? (b ? (c ? trim(a ? raw : "x") : "C") : "B") : "A";}`,
		`{return trim(` + deep + `);}`,
		`{return a ? (b ? (raw ? raw : "C") : "B") : "A";}`,
		`{return a ? (b ? (c ? true : "C") : "B") : "A";}`,
		`{return a ? (b ? (c ? missing : "C") : "B") : "A";}`,
		`{return a ? (b ? (c ? raw : ) : "B") : "A";}`,
		`{string local=local;return ` + deep + `;}`,
		`{string first=second;string second=raw;return ` + deep + `;}`,
		`{string raw="shadow";return ` + deep + `;}`,
		`{return a ? (b ? (c ? propagate(raw) : "C") : "B") : "A";}`,
		`{return a ? (b ? (c ? match(raw){some(v)=>v,none=>raw} : "C") : "B") : "A";}`,
	}
	for i, body := range cases {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			source := prefix + body + `}`
			input := semanticTestModuleSet("compiler.selfhosting", []ModuleInput{testModule("compiler.selfhosting", "depth-three.pipe", source)}, nil)
			input.LanguageContract = PipeLangLanguageContractV930
			analysis := AnalyzeSemanticModuleSet(input)
			if analysis.Error() == nil {
				t.Fatal("admitted", body)
			}
			for _, d := range analysis.Diagnostics {
				if !d.Primary.IsValid() || d.Primary.File != "depth-three.pipe" || d.Primary.End > len(source) {
					t.Fatal("invalid diagnostic", d)
				}
			}
		})
	}
	for version := 1; version <= 92; version++ {
		input := semanticTestModuleSet("compiler.selfhosting", []ModuleInput{testModule("compiler.selfhosting", "depth-three.pipe", prefix+`{return `+deep+`;}}`)}, nil)
		input.LanguageContract = LanguageContract(fmt.Sprintf("v0.%d.0", version))
		if AnalyzeSemanticModuleSet(input).Error() == nil {
			t.Fatal("older source admitted", version)
		}
	}
}

func TestV930DepthThreeReturnsMalformedCore(t *testing.T) {
	source := `public Class Choices {public string Select(string raw,bool a,bool b,bool c) {return a ? (b ? (c ? raw : "C") : "B") : "A";}}`
	copyExpr := func(e *coreir.Expr) *coreir.Expr {
		b, _ := json.Marshal(e)
		var out coreir.Expr
		if err := json.Unmarshal(b, &out); err != nil {
			t.Fatal(err)
		}
		return &out
	}
	for _, mutation := range []string{"condition", "arm", "missing", "depth", "condition placement", "argument placement", "hidden local", "terminal", "identity", "version", "downgrade", "initializer", "leaf"} {
		t.Run(mutation, func(t *testing.T) {
			_, p := conditionalLocalTreeProgramVersion(t, PipeLangLanguageContractV930, source, []string{"Select"})
			f := &p.Functions[len(p.Functions)-1]
			root := f.Body.Conditional
			third := root.WhenTrue.Conditional.WhenTrue.Conditional
			switch mutation {
			case "condition":
				third.Condition = third.WhenTrue
			case "arm":
				third.WhenTrue = third.Condition
			case "missing":
				third.WhenFalse = nil
			case "depth":
				third.WhenTrue = copyExpr(root.WhenTrue.Conditional.WhenTrue)
			case "condition placement":
				third.Condition = &coreir.Expr{Kind: coreir.ExprConditional, Type: third.Condition.Type, Conditional: &coreir.Conditional{Condition: third.Condition, WhenTrue: third.Condition, WhenFalse: third.Condition}}
			case "argument placement":
				third.WhenTrue = &coreir.Expr{Kind: coreir.ExprTextTrim, Type: f.ReturnType, TextTrim: &coreir.TextTrim{Value: copyExpr(root.WhenTrue)}}
			case "hidden local":
				third.WhenTrue = &coreir.Expr{Kind: coreir.ExprImmutableLocal, Type: f.ReturnType, ImmutableLocal: &coreir.ImmutableLocal{}}
			case "terminal":
				third.TerminalStatement = true
			case "identity":
				p.CompilerContract = "unknown"
			case "version":
				p.LanguageContract = "v0.95.0"
			case "downgrade":
				p.LanguageContract = coreir.LanguageContractV920
			case "initializer":
				_, q := conditionalLocalTreeProgramVersion(t, PipeLangLanguageContractV930, depthThreeStraightLineReturnsSource, []string{"Select"})
				q.Functions[len(q.Functions)-1].Body.ImmutableLocal.Initializer = copyExpr(&f.Body)
				p = q
				f = &p.Functions[len(p.Functions)-1]
			case "leaf":
				body := copyExpr(&f.Body)
				f.Body = coreir.Expr{Kind: coreir.ExprConditional, Type: f.ReturnType, Conditional: &coreir.Conditional{TerminalStatement: true, Condition: root.Condition, WhenTrue: body, WhenFalse: copyExpr(root.WhenFalse)}}
			}
			if coreir.ValidateProgram(p) == nil {
				t.Fatal("malformed Core admitted")
			}
			if _, err := gobackend.Generate(p); err == nil {
				t.Fatal("backend admitted malformed Core")
			}
			if _, err := coreeval.EvaluateProgram(p, f.Identity, nil); err == nil {
				t.Fatal("evaluator admitted malformed Core")
			}
		})
	}
	_, p := conditionalLocalTreeProgramVersion(t, PipeLangLanguageContractV930, source, []string{"Select"})
	for version := 1; version <= 92; version++ {
		p.LanguageContract = fmt.Sprintf("v0.%d.0", version)
		if coreir.ValidateProgram(p) == nil {
			t.Fatal("older Core admitted", version)
		}
	}
	// Existing arrow/block-erased depth-two Core remains valid at its original boundary.
	_, p = conditionalLocalTreeProgramVersion(t, PipeLangLanguageContractV930, nestedArrowMethodsSource, []string{"Select"})
	for version := 88; version <= 93; version++ {
		p.LanguageContract = fmt.Sprintf("v0.%d.0", version)
		if err := coreir.ValidateProgram(p); err != nil {
			t.Fatal(version, err)
		}
	}
}
