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

const terminalBooleanSelectorTestsSource = `public Class Choices {public string Select(string raw,bool a,bool b,bool c){if((a ? b : c) ? b : c){return trim(raw);}else{return raw;}}}`

func TestV1060TerminalBooleanSelectorTestsAdmission(t *testing.T) {
	for _, source := range []string{terminalBooleanSelectorTestsSource, strings.Replace(terminalBooleanSelectorTestsSource, "public string Select", "string Select", 1), `public Class Choices {public string Select(){if(true ? (false ? true : false) : true){return "a";}else{return "b";}}}`} {
		_, p := conditionalLocalTreeProgramVersion(t, PipeLangLanguageContractV1060, source, []string{"Select"})
		if err := coreir.ValidateProgram(p); err != nil {
			t.Fatal(err)
		}
	}
}

func TestV1060TerminalBooleanSelectorTestsTypes(t *testing.T) {
	testConditionalLocalsTypeMatrix(t, PipeLangLanguageContractV1060, true)
}

func TestV1060TerminalBooleanSelectorTestsCarriers(t *testing.T) {
	testConditionalLocalsCarrierAndHostValues(t, PipeLangLanguageContractV1060, true)
}

func TestV1060TerminalBooleanSelectorTestsInheritance(t *testing.T) {
	for _, source := range []string{depthThreeTerminalConditionalTestsSource, nestedTerminalConditionalTestsSource, terminalConditionalTestsSource, terminalBooleanSelectorInitializersSource, straightLineBooleanSelectorInitializersSource, arrowBooleanSelectorsSource, v990Wrap(t, conditionalBooleanSelectorsSource), conditionalBooleanSelectorsSource, v940WrapBody(t, depthThreeStraightLineInitializersSource, terminalTrees(3)[25], "a"), depthThreeStraightLineInitializersSource, depthThreeArrowMethodsSource, depthThreeTerminalLeafReturnsSource, depthThreeStraightLineReturnsSource, nestedArrowMethodsSource, nestedTerminalInitializersSource, nestedStraightLineInitializersSource, nestedTerminalLeafReturnsSource, nestedStraightLineReturnsSource, terminalLeafConditionalReturnsSource, conditionalReturnCompositionSource, straightLineConditionalLocalsSource, finiteConditionalLocalsSource, twoConditionalLocalsSource, `public Class Choices {public string Select(string raw,bool pick)=>pick ? raw : trim(raw);}`} {
		var baseline [][]byte
		for _, contract := range []LanguageContract{PipeLangLanguageContractV1050, PipeLangLanguageContractV1060} {
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
			h.LanguageContract = coreir.LanguageContractV1050
			p.LanguageContract = coreir.LanguageContractV1050
			projection.LanguageContract = PipeLangLanguageContractV1050
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

func TestV1060TerminalBooleanSelectorTestsLayouts(t *testing.T) {
	shapes := terminalTrees(3)[1:]
	if len(shapes) != 25 {
		t.Fatal("shape inventory drift")
	}
	for partition := 0; partition < 200; partition++ {
		shape := partition / 8
		tree := shapes[shape]
		t.Run(fmt.Sprint(partition), func(t *testing.T) {
			enterFiniteShape(t)
			nodes := []int{}
			var collect func(*terminalTree, int)
			collect = func(n *terminalTree, index int) {
				if n == nil {
					return
				}
				nodes = append(nodes, index)
				collect(n.yes, index*2+1)
				collect(n.no, index*2+2)
			}
			collect(tree, 0)
			for subset := 0; subset < 1<<len(nodes); subset++ {
				if subset%8 != partition%8 {
					continue
				}
				positions := map[int]bool{}
				for bit, index := range nodes {
					positions[index] = subset&(1<<bit) != 0
				}
				// Rotate no-local, one-local and mixed eager/unused sequences independently of shape.
				count := []int{0, 1, 3}[(shape+subset)%3]
				var source strings.Builder
				source.WriteString(`public Class Choices {public string Echo(string value)=>value;public bool Check(string path,bool value)=>value;public string Select(string raw`)
				for bit := 0; bit < 11; bit++ {
					fmt.Fprintf(&source, ",bool c%d", bit)
				}
				source.WriteString("){")
				var emit func(*terminalTree, int, int, string)
				emit = func(n *terminalTree, index, depth int, previous string) {
					for slot := 0; slot < count; slot++ {
						name := fmt.Sprintf("n%dq%d", index, slot)
						init := fmt.Sprintf(`Echo(%s+"O%d")`, previous, index)
						if slot == 1 {
							init = fmt.Sprintf(`(Check("I%dA",c8) ? Check("I%dB",c9) : Check("I%dC",c10)) ? Echo(%s+"T%d") : Echo(%s+"F%d")`, index, index, index, previous, index, previous, index)
						}
						if slot == 2 {
							init = fmt.Sprintf(`Echo(%s+"U%d")`, previous, index)
						}
						fmt.Fprintf(&source, "string %s=%s;", name, init)
						if slot < 2 {
							previous = name
						}
					}
					if n == nil {
						switch (shape + subset) % 3 {
						case 0:
							fmt.Fprintf(&source, `return Echo(%s+"R%d");`, previous, index)
						case 1:
							fmt.Fprintf(&source, `return Check("R9",c8) ? (Check("R10",c9) ? (Check("R11",c10) ? Echo(%s+"T%d") : Echo(%s+"U%d")) : Echo(%s+"V%d")) : Echo(%s+"F%d");`, previous, index, previous, index, previous, index, previous, index)
						case 2:
							fmt.Fprintf(&source, `return (Check("R9",c8) ? Check("R10",c9) : Check("R11",c10)) ? Echo(%s+"T%d") : Echo(%s+"F%d");`, previous, index, previous, index)
						}
						return
					}
					atom := func(label string, bit int, flip bool) string {
						value := fmt.Sprintf("c%d", bit)
						if flip {
							value = fmt.Sprintf("(c%d != c%d)", bit, 5+depth)
						}
						return fmt.Sprintf(`Check("S%d%s",%s)`, index, label, value)
					}
					cond := atom("O", 5+depth, false)
					if positions[index] {
						cond = "(" + atom("A", 0, false) + " ? " + atom("B", 1, false) + " : " + atom("C", 2, false) + ") ? " + atom("D", 3, true) + " : " + atom("E", 4, true)
					} else if index%3 == 1 {
						cond = atom("A", 0, false) + " ? " + atom("C", 2, true) + " : " + atom("F", 4, true)
					} else if index%3 == 2 {
						cond = atom("A", 0, false) + " ? (" + atom("B", 1, false) + " ? " + atom("C", 2, true) + " : " + atom("D", 3, true) + ") : " + atom("F", 4, true)
					}

					fmt.Fprintf(&source, "if(%s){", cond)
					emit(n.yes, index*2+1, depth+1, previous)
					source.WriteString("}else{")
					emit(n.no, index*2+2, depth+1, previous)
					source.WriteString("}")
				}
				emit(tree, 0, 0, "raw")
				source.WriteString("}}")
				a, p := conditionalLocalTreeProgramVersion(t, PipeLangLanguageContractV1060, source.String(), []string{"Select", "Echo", "Check"})
				aa, pp := conditionalLocalTreeProgramVersion(t, PipeLangLanguageContractV1060, source.String(), []string{"Select", "Echo", "Check"})
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
				again, err := BuildSemanticProjection(aa)
				if err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(h, hh) || !reflect.DeepEqual(p, pp) || !reflect.DeepEqual(projection, again) {
					t.Fatal("nondeterministic HIR/Core/semantic")
				}
				generated, err := gobackend.Generate(p)
				if err != nil {
					t.Fatal(err)
				}
				repeated, err := gobackend.Generate(pp)
				if err != nil || !bytes.Equal(generated, repeated) {
					t.Fatal("nondeterministic Go")
				}
				f := coreFunctionNamed(t, p, "Select")
				prepared := prepareConformanceProgram(t, p)
				type row struct {
					Flags []bool
					Value string
					Trace []string
				}
				rows := []row{}
				for bits := 0; bits < 2048; bits++ {
					flags := make([]bool, 11)
					for bit := range flags {
						flags[bit] = bits&(1<<bit) != 0
					}
					trace := []string{}
					want := "raw"
					n, index, depth := tree, 0, 0
					for {
						for slot := 0; slot < count; slot++ {
							suffix := fmt.Sprintf("O%d", index)
							if slot == 1 {
								trace = append(trace, fmt.Sprintf("C:I%dA", index))
								picked := flags[10]
								if flags[8] {
									trace = append(trace, fmt.Sprintf("C:I%dB", index))
									picked = flags[9]
								} else {
									trace = append(trace, fmt.Sprintf("C:I%dC", index))
								}
								suffix = fmt.Sprintf("F%d", index)
								if picked {
									suffix = fmt.Sprintf("T%d", index)
								}
							}
							if slot == 2 {
								suffix = fmt.Sprintf("U%d", index)
							}
							value := want + suffix
							trace = append(trace, "E:"+value)
							if slot < 2 {
								want = value
							}
						}
						if n == nil {
							suffix := "R"
							if (shape+subset)%3 != 0 {
								trace = append(trace, "C:R9")
								suffix = "F"
								if (shape+subset)%3 == 1 {
									if flags[8] {
										trace = append(trace, "C:R10")
										suffix = "V"
										if flags[9] {
											trace = append(trace, "C:R11")
											suffix = "U"
											if flags[10] {
												suffix = "T"
											}
										}
									}
								} else {
									picked := flags[10]
									if flags[8] {
										trace = append(trace, "C:R10")
										picked = flags[9]
									} else {
										trace = append(trace, "C:R11")
									}
									if picked {
										suffix = "T"
									}
								}
							}
							want += fmt.Sprintf("%s%d", suffix, index)
							trace = append(trace, "E:"+want)
							break
						}
						picked := flags[5+depth]
						if positions[index] {
							trace = append(trace, fmt.Sprintf("C:S%dA", index))
							selector := flags[2]
							if flags[0] {
								trace = append(trace, fmt.Sprintf("C:S%dB", index))
								selector = flags[1]
							} else {
								trace = append(trace, fmt.Sprintf("C:S%dC", index))
							}
							bit, label := 4, "E"
							if selector {
								bit, label = 3, "D"
							}
							trace = append(trace, fmt.Sprintf("C:S%d%s", index, label))
							picked = flags[bit] != flags[5+depth]
						} else if index%3 != 0 {
							trace = append(trace, fmt.Sprintf("C:S%dA", index))
							label, bit := "F", 4
							if flags[0] {
								label, bit = "C", 2
								if index%3 == 2 {
									trace = append(trace, fmt.Sprintf("C:S%dB", index))
									if !flags[1] {
										label, bit = "D", 3
									}
								}
							}
							trace = append(trace, fmt.Sprintf("C:S%d%s", index, label))
							picked = flags[bit] != flags[5+depth]
						} else {
							trace = append(trace, fmt.Sprintf("C:S%dO", index))
						}

						if picked {
							n = n.yes
							index = index*2 + 1
						} else {
							n = n.no
							index = index*2 + 2
						}
						depth++
					}
					args := []coreeval.Value{{Type: f.Parameters[0].Type, String: "raw"}}
					for bit, flag := range flags {
						args = append(args, coreeval.Value{Type: f.Parameters[bit+1].Type, Bool: flag})
					}
					got, err := prepared.Evaluate(f.Identity, args)
					if err != nil || !got.OK || got.Value.String != want {
						t.Fatalf("shape=%d subset=%d bits=%d got=%#v err=%v want=%q", shape, subset, bits, got, err, want)
					}
					rows = append(rows, row{flags, want, trace})
				}
				fixture, err := json.Marshal(rows)
				if err != nil {
					t.Fatal(err)
				}
				call := `PipeLangSelect("raw"`
				for bit := 0; bit < 11; bit++ {
					call += fmt.Sprintf(",r.Flags[%d]", bit)
				}
				call += ")"
				loader := `type row struct{Flags []bool;Value string;Trace []string};func load(t *testing.T)[]row{b,e:=os.ReadFile("oracle.json");if e!=nil{t.Fatal(e)};var rows []row;if e=json.Unmarshal(b,&rows);e!=nil{t.Fatal(e)};if len(rows)!=2048{t.Fatal("oracle inventory")};return rows}`
				checks := fmt.Sprintf("package %s\nimport(\"testing\";\"encoding/json\";\"os\")\n%s\nfunc TestValues(t *testing.T){for _,r:=range load(t){if got:=%s;got!=r.Value{t.Fatal(got,r.Value)}}}", gobackend.PackageName, loader, call)
				compileAndRunGeneratedGoFilesWithFixtures(t, generated, []byte(checks), map[string][]byte{"oracle.json": fixture})
				observed := string(generated)
				for marker, probe := range map[string]string{"func PipeLangEcho(p0 string) string {": `v1060Trace=append(v1060Trace,"E:"+p0)`, "func PipeLangCheck(p0 string, p1 bool) bool {": `v1060Trace=append(v1060Trace,"C:"+p0)`} {
					if strings.Count(observed, marker) != 1 {
						t.Fatal("trace marker")
					}
					observed = strings.Replace(observed, marker, marker+"\n"+probe, 1)
				}
				orders := fmt.Sprintf("package %s\nimport(\"testing\";\"encoding/json\";\"os\";\"reflect\")\nvar v1060Trace []string\n%s\nfunc TestOrder(t *testing.T){for _,r:=range load(t){v1060Trace=nil;got:=%s;if got!=r.Value||!reflect.DeepEqual(v1060Trace,r.Trace){t.Fatal(got,r.Value,v1060Trace,r.Trace)}}}", gobackend.PackageName, loader, call)
				compileAndRunGeneratedGoFilesWithFixtures(t, []byte(observed), []byte(orders), map[string][]byte{"oracle.json": fixture})
				t.Logf("v106-layout shape=%d subset=%d locals=%d vectors=%d", shape, subset, count, len(rows))
			}
		})
	}
}

func TestV1060TerminalBooleanSelectorTestsSourceRejection(t *testing.T) {
	bodies := []string{
		`if(a ? ((b ? a : c) ? b : c) : c){return raw;}else{return raw;}`,
		`if(a ? b : ((b ? a : c) ? b : c)){return raw;}else{return raw;}`,
		`if(a ? Check(b ? a : c) : c){return raw;}else{return raw;}`,
		`if(a ? b : Check(b ? a : c)){return raw;}else{return raw;}`,
		`if(a ? ((b ? a : c) && b) : c){return raw;}else{return raw;}`,

		`if(a ? (b ? (a ? (b ? a : c) : c) : c) : c){return raw;}else{return raw;}`,
		`if(a ? b : (c ? a : (a ? b : (b ? a : c)))){return raw;}else{return raw;}`,
		`if(Check(a ? b : c)){return raw;}else{return raw;}`,
		`if(raw ? b : c){return raw;}else{return raw;}`,
		`if(a ? raw : c){return raw;}else{return raw;}`,
		`if(a ? b : raw){return raw;}else{return raw;}`,
		`if(a ? later : c){bool later=b;return raw;}else{return raw;}`,
		`bool local=local;if(a ? local : c){return raw;}else{return raw;}`,
		`bool local=later;bool later=b;if(a ? local : c){return raw;}else{return raw;}`,
		`bool local=b;bool local=c;if(a ? local : c){return raw;}else{return raw;}`,
		`bool local=b;if(a ? local : c){bool local=c;return raw;}else{return raw;}`,
		`if((a ? b : c) ? b : c){bool sibling=b;return raw;}else{if(a ? sibling : c){return raw;}else{return raw;}}`,
		`if((a ? b : c) ? b : c){bool branch=b;return raw;}else{return raw;}return branch;`,
		`if((a ? b : c) ? b : c){return raw;}`,
		`if((a ? b : c) ? b : c){return (a ? b : c) ? (b ? raw : raw) : raw;}else{return raw;}`,
		`if((a ? b : c) ? b : c){string value=(a ? b : c) ? (b ? raw : raw) : raw;return value;}else{return raw;}`,
		`if((a ? b : c) ? b : c){return trim(a ? raw : raw);}else{return raw;}`,
	}
	for depth := 0; depth < 3; depth++ {
		for i, body := range bodies {
			t.Run(fmt.Sprintf("%d/%d", depth, i), func(t *testing.T) {
				input := semanticTestModuleSet("compiler.selfhosting", []ModuleInput{testModule("compiler.selfhosting", "conditional.pipe", v1050ScopeSource(depth, body))}, nil)
				input.LanguageContract = PipeLangLanguageContractV1060
				if AnalyzeSemanticModuleSet(input).Error() == nil {
					t.Fatal("unsupported source admitted", body)
				}
			})
		}
	}
	for _, source := range []string{v1050ScopeSource(3, `if((a ? b : c) ? b : c){return raw;}else{return raw;}`), strings.Replace(terminalBooleanSelectorTestsSource, "public string Select", "private string Select", 1)} {
		input := semanticTestModuleSet("compiler.selfhosting", []ModuleInput{testModule("compiler.selfhosting", "conditional.pipe", source)}, nil)
		input.LanguageContract = PipeLangLanguageContractV1060
		if AnalyzeSemanticModuleSet(input).Error() == nil {
			t.Fatal("depth/private admitted")
		}
	}
}

func TestV1060TerminalBooleanSelectorTestsMalformedCore(t *testing.T) {
	for depth := 0; depth < 3; depth++ {
		for _, mutation := range []string{"condition", "payload", "a", "b", "c", "a type", "b type", "c type", "result type", "nested a", "nested b", "nested c", "terminal test", "reference", "identity", "version", "branch", "deeper"} {
			t.Run(fmt.Sprintf("%d/%s", depth, mutation), func(t *testing.T) {
				_, p := conditionalLocalTreeProgramVersion(t, PipeLangLanguageContractV1060, v1050ScopeSource(depth, `if((a ? b : c) ? b : c){return raw;}else{return raw;}`), []string{"Select"})
				f := &p.Functions[len(p.Functions)-1]
				expr := &f.Body
				for i := 0; i < depth; i++ {
					expr = expr.Conditional.WhenTrue
				}
				branch := expr.Conditional
				test := branch.Condition.Conditional
				switch mutation {
				case "condition":
					branch.Condition = nil
				case "payload":
					branch.Condition.Conditional = nil
				case "a":
					test.Condition = nil
				case "b":
					test.WhenTrue = nil
				case "c":
					test.WhenFalse = nil
				case "a type":
					test.Condition = branch.WhenTrue
				case "b type":
					test.WhenTrue = branch.WhenTrue
				case "c type":
					test.WhenFalse = branch.WhenTrue
				case "result type":
					branch.Condition.Type = branch.WhenTrue.Type
				case "nested a":
					test.Condition = v1050Clone(t, branch.Condition)
				case "nested b":
					test.WhenTrue = v1050Clone(t, branch.Condition)
				case "nested c":
					test.WhenFalse = v1050Clone(t, branch.Condition)
				case "terminal test":
					test.TerminalStatement = true
				case "reference":
					position := 999
					test.WhenTrue = &coreir.Expr{Kind: coreir.ExprReference, Type: test.WhenTrue.Type, Parameter: &position}
				case "identity":
					p.CompilerContract = "unknown"
				case "version":
					p.LanguageContract = "v0.999.0"
				case "branch":
					branch.WhenFalse = nil
				case "deeper":
					for i := depth; i < 3; i++ {
						branch.WhenTrue = v1050Clone(t, expr)
						branch = branch.WhenTrue.Conditional
					}
				}
				assertAdmissionRejected(t, p, "")
			})
		}
	}
}

func TestV1060TerminalBooleanSelectorTestsInternalCoreBoundary(t *testing.T) {
	for depth := 0; depth < 3; depth++ {
		for slot := 0; slot < 7; slot++ {
			t.Run(fmt.Sprintf("%d/%d", depth, slot), func(t *testing.T) {
				_, p := conditionalLocalTreeProgramVersion(t, PipeLangLanguageContractV1060, v1050ScopeSource(depth, `if((a ? b : c) ? b : c){return raw;}else{return raw;}`), []string{"Select"})
				f := &p.Functions[len(p.Functions)-1]
				expr := &f.Body
				for i := 0; i < depth; i++ {
					expr = expr.Conditional.WhenTrue
				}
				targets := []**coreir.Expr{}
				var collect func(**coreir.Expr)
				collect = func(e **coreir.Expr) {
					targets = append(targets, e)
					if (*e).Conditional != nil {
						c := (*e).Conditional
						collect(&c.Condition)
						collect(&c.WhenTrue)
						collect(&c.WhenFalse)
					}
				}
				collect(&expr.Conditional.Condition)
				if len(targets) != 7 {
					t.Fatal("operand inventory", len(targets))
				}
				target := targets[slot]
				value := *target
				position := len(f.Parameters)
				*target = &coreir.Expr{Kind: coreir.ExprImmutableLocal, Type: value.Type, ImmutableLocal: &coreir.ImmutableLocal{Name: "hidden", Position: position, Type: value.Type, Initializer: value, Return: &coreir.Expr{Kind: coreir.ExprReference, Type: value.Type, Parameter: &position}}}
				if err := coreir.ValidateFunction(*f); err != nil {
					t.Fatalf("generic internal Core narrowed: %v", err)
				}
				assertAdmissionRejected(t, p, "")
			})
		}
	}
}

func TestV1060TerminalBooleanSelectorTestsComputedCarriers(t *testing.T) {
	for _, unused := range []bool{false, true} {
		t.Run(fmt.Sprint(unused), func(t *testing.T) {
			local := ""
			if unused {
				local = `Result<int,ArithmeticError> unused=(a ? b : c) ? Make(value) : Make(0);`
			}
			source := `public Class Choices {public Result<int,ArithmeticError> Make(int value)=>value+1;public Result<int,ArithmeticError> Select(int value,bool a,bool b,bool c){` + local + `Result<int,ArithmeticError> selected=(a ? b : c) ? Make(value) : Make(0);return selected;}}`
			source = v940WrapBody(t, source, terminalTrees(3)[25], "(a ? b : c) ? true : false")
			_, p := conditionalLocalTreeProgramVersion(t, PipeLangLanguageContractV1060, source, []string{"Select"})
			f := coreFunctionNamed(t, p, "Select")
			var checks strings.Builder
			for _, value := range []int64{-9223372036854775808, 0, 9223372036854775807} {
				for mask := 0; mask < 8; mask++ {
					selected := mask&4 != 0
					if mask&1 != 0 {
						selected = mask&2 != 0
					}
					want, success := int64(1), true
					if selected {
						success = value != 9223372036854775807
						if success {
							want = value + 1
						}
					}
					args := []coreeval.Value{{Type: f.Parameters[0].Type, Int: value}}
					for i := 0; i < 3; i++ {
						args = append(args, coreeval.Value{Type: f.Parameters[i+1].Type, Bool: mask&(1<<i) != 0})
					}
					got, err := coreeval.EvaluateProgram(p, f.Identity, args)
					if err != nil || got.OK != success || (success && got.Value.Int != want) || (!success && got.Error != coreir.ArithmeticOverflow) {
						t.Fatal(value, mask, got, err)
					}
					fmt.Fprintf(&checks, "{got:=PipeLangSelect(%d,%t,%t,%t);if got.OK!=%t||(got.OK&&got.Value!=%d)||(!got.OK&&string(got.Error)!=\"overflow\"){t.Fatal(got)}}\n", value, mask&1 != 0, mask&2 != 0, mask&4 != 0, success, want)
				}
			}
			generated, err := gobackend.Generate(p)
			if err != nil {
				t.Fatal(err)
			}
			compileAndRunGeneratedGoFiles(t, generated, []byte(fmt.Sprintf("package %s\nimport \"testing\"\nfunc TestComputed(t *testing.T){%s}", gobackend.PackageName, checks.String())))
		})
	}
}

func TestV1060TerminalBooleanSelectorTestsNestedOperands(t *testing.T) {
	for depth := 0; depth < 3; depth++ {
		for arm := 0; arm < 1; arm++ {
			for _, fault := range []string{"payload", "condition", "true", "false", "condition type", "true type", "false type", "result type", "selector", "true depth", "false depth", "terminal"} {
				t.Run(fmt.Sprintf("%d/%d/%s", depth, arm, fault), func(t *testing.T) {
					_, p := conditionalLocalTreeProgramVersion(t, PipeLangLanguageContractV1060, v1050ScopeSource(depth, `if((a ? b : c) ? b : c){return raw;}else{return raw;}`), []string{"Select"})
					branch := p.Functions[len(p.Functions)-1].Body.Conditional
					for i := 0; i < depth; i++ {
						branch = branch.WhenTrue.Conditional
					}
					outer := branch.Condition.Conditional
					value := []*coreir.Expr{outer.Condition}[arm]
					nested := value.Conditional
					switch fault {
					case "payload":
						value.Conditional = nil
					case "condition":
						nested.Condition = nil
					case "true":
						nested.WhenTrue = nil
					case "false":
						nested.WhenFalse = nil
					case "condition type":
						nested.Condition = v1050Clone(t, branch.WhenTrue)
					case "true type":
						nested.WhenTrue = v1050Clone(t, branch.WhenTrue)
					case "false type":
						nested.WhenFalse = v1050Clone(t, branch.WhenTrue)
					case "result type":
						value.Type = branch.WhenTrue.Type
					case "selector":
						nested.Condition = v1050Clone(t, value)
					case "true depth":
						nested.WhenTrue = v1050Clone(t, branch.Condition)
					case "false depth":
						nested.WhenFalse = v1050Clone(t, branch.Condition)
					case "terminal":
						nested.TerminalStatement = true
					}
					assertAdmissionRejected(t, p, "")
				})
			}
		}
	}
}

// Every earlier contract rejects the exact new shape at all three statement depths.
func TestV1060TerminalBooleanSelectorTestsPriorContracts(t *testing.T) {
	for depth := 0; depth < 3; depth++ {
		source := v1050ScopeSource(depth, `if((a ? b : c) ? b : c){return raw;}else{return raw;}`)
		_, p := conditionalLocalTreeProgramVersion(t, PipeLangLanguageContractV1060, source, []string{"Select"})
		for version := 1; version <= 105; version++ {
			contract := LanguageContract(fmt.Sprintf("v0.%d.0", version))
			input := semanticTestModuleSet("compiler.selfhosting", []ModuleInput{testModule("compiler.selfhosting", "selector.pipe", source)}, nil)
			input.LanguageContract = contract
			if AnalyzeSemanticModuleSet(input).Error() == nil {
				t.Fatal("prior source admitted selector", depth, version)
			}
			p.LanguageContract = string(contract)
			assertAdmissionRejected(t, p, "")
		}
	}
}

// Five independent operands, lexical/computed operands, mixed inherited deep tests,
// and branch-local eager unused bindings. The oracle is separate from source emission.
func TestV1060TerminalBooleanSelectorTestsComputedAndDependent(t *testing.T) {
	for depth := 0; depth < 3; depth++ {
		t.Run(fmt.Sprint(depth), func(t *testing.T) {
			body := `string normalized=Echo(trim(raw));bool present=normalized!="";
if((Check("A",a && present) ? Check("B",b || !present) : Check("C",c && present)) ? Check("D",d != present) : Check("E",e || !present)){
 string mid=Echo(normalized+"T");bool derived=mid!="";string unused=Echo(mid+"unused");return derived ? mid : raw;
}else{return Echo(normalized+"F");}`
			for i := 0; i < depth; i++ {
				body = `if(true ? (true ? (true ? true : false) : false) : false){` + body + `}else{return raw;}`
			}
			source := `public Class Choices {public string Echo(string value)=>value;public bool Check(string label,bool value)=>value;public string Select(string raw,bool a,bool b,bool c,bool d,bool e){` + body + `}}`
			_, p := conditionalLocalTreeProgramVersion(t, PipeLangLanguageContractV1060, source, []string{"Select", "Echo", "Check"})
			f := coreFunctionNamed(t, p, "Select")
			var checks, orders strings.Builder
			for _, raw := range []string{"", "  ", " ready ", "no"} {
				for bits := 0; bits < 32; bits++ {
					a, b, c, d, e := bits&1 != 0, bits&2 != 0, bits&4 != 0, bits&8 != 0, bits&16 != 0
					normalized := strings.TrimSpace(raw)
					present := normalized != ""
					trace := []string{"E:" + normalized, "C:A"}
					selector := c && present
					if a && present {
						trace = append(trace, "C:B")
						selector = b || !present
					} else {
						trace = append(trace, "C:C")
					}
					picked := e || !present
					if selector {
						trace = append(trace, "C:D")
						picked = d != present
					} else {
						trace = append(trace, "C:E")
					}
					want := normalized + "F"
					if picked {
						want = normalized + "T"
						trace = append(trace, "E:"+want, "E:"+want+"unused")
					} else {
						trace = append(trace, "E:"+want)
					}
					args := []coreeval.Value{{Type: f.Parameters[0].Type, String: raw}}
					for i, v := range []bool{a, b, c, d, e} {
						args = append(args, coreeval.Value{Type: f.Parameters[i+1].Type, Bool: v})
					}
					got, err := coreeval.EvaluateProgram(p, f.Identity, args)
					if err != nil || !got.OK || got.Value.String != want {
						t.Fatal(bits, got, err, want)
					}
					call := fmt.Sprintf("PipeLangSelect(%q,%t,%t,%t,%t,%t)", raw, a, b, c, d, e)
					fmt.Fprintf(&checks, "if got:=%s;got!=%q{t.Fatal(got)}\n", call, want)
					fmt.Fprintf(&orders, "v1060Trace=nil;if got:=%s;got!=%q||!reflect.DeepEqual(v1060Trace,%#v){t.Fatal(got,v1060Trace)}\n", call, want, trace)
				}
			}
			generated, err := gobackend.Generate(p)
			if err != nil {
				t.Fatal(err)
			}
			compileAndRunGeneratedGoFiles(t, generated, []byte(fmt.Sprintf("package %s\nimport \"testing\"\nfunc TestValues(t *testing.T){%s}", gobackend.PackageName, checks.String())))
			observed := string(generated)
			for marker, probe := range map[string]string{"func PipeLangEcho(p0 string) string {": `v1060Trace=append(v1060Trace,"E:"+p0)`, "func PipeLangCheck(p0 string, p1 bool) bool {": `v1060Trace=append(v1060Trace,"C:"+p0)`} {
				if strings.Count(observed, marker) != 1 {
					t.Fatal("trace marker")
				}
				observed = strings.Replace(observed, marker, marker+"\n"+probe, 1)
			}
			compileAndRunGeneratedGoFiles(t, []byte(observed), []byte(fmt.Sprintf("package %s\nimport(\"testing\";\"reflect\")\nvar v1060Trace []string\nfunc TestOrder(t *testing.T){%s}", gobackend.PackageName, orders.String())))
		})
	}
}

func TestV1060TerminalBooleanSelectorTestsExactBoundary(t *testing.T) {
	// Refuse nesting independently in each of five operands, plus hidden boolean/call placements.
	atoms := []string{"a", "b", "c", "a", "b"}
	for slot := 0; slot < 5; slot++ {
		for _, bad := range []string{"(a ? b : c)", "raw", "Check(a ? b : c)", "((a ? b : c) && a)", "missing"} {
			operands := append([]string(nil), atoms...)
			operands[slot] = bad
			condition := fmt.Sprintf("(%s ? %s : %s) ? %s : %s", operands[0], operands[1], operands[2], operands[3], operands[4])
			for depth := 0; depth < 3; depth++ {
				source := v1050ScopeSource(depth, "if("+condition+"){return raw;}else{return raw;}")
				input := semanticTestModuleSet("compiler.selfhosting", []ModuleInput{testModule("compiler.selfhosting", "selector.pipe", source)}, nil)
				input.LanguageContract = PipeLangLanguageContractV1060
				if AnalyzeSemanticModuleSet(input).Error() == nil {
					t.Fatal("operand boundary", slot, bad, depth)
				}
			}
		}
	}
}
