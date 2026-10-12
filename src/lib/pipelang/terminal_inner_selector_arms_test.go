package pipelang

import (
	"bytes"
	"crypto/sha256"
	"dockpipe/src/lib/pipelang/coreeval"
	"dockpipe/src/lib/pipelang/coreir"
	"dockpipe/src/lib/pipelang/gobackend"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"
)

const terminalInnerSelectorArmsSource = `public Class Choices {public string Select(string raw,bool a,bool b,bool c){if((a ? (b ? a : c) : (c ? a : b)) ? b : c){return trim(raw);}else{return raw;}}}`

func TestV1080TerminalInnerSelectorArmsAdmission(t *testing.T) {
	for _, source := range []string{terminalInnerSelectorArmsSource, strings.Replace(terminalInnerSelectorArmsSource, "public string Select", "string Select", 1), `public Class Choices {public string Select(){if(true ? (false ? true : false) : true){return "a";}else{return "b";}}}`} {
		_, p := conditionalLocalTreeProgramVersion(t, PipeLangLanguageContractV1080, source, []string{"Select"})
		if err := coreir.ValidateProgram(p); err != nil {
			t.Fatal(err)
		}
	}
}

func TestV1080TerminalInnerSelectorArmsTypes(t *testing.T) {
	testConditionalLocalsTypeMatrix(t, PipeLangLanguageContractV1080, true)
}

func TestV1080TerminalInnerSelectorArmsCarriers(t *testing.T) {
	testConditionalLocalsCarrierAndHostValues(t, PipeLangLanguageContractV1080, true)
}

func TestV1080TerminalInnerSelectorArmsInheritance(t *testing.T) {
	for _, source := range []string{terminalSelectorValueArmsSource, terminalBooleanSelectorTestsSource, depthThreeTerminalConditionalTestsSource, nestedTerminalConditionalTestsSource, terminalConditionalTestsSource, terminalBooleanSelectorInitializersSource, straightLineBooleanSelectorInitializersSource, arrowBooleanSelectorsSource, v990Wrap(t, conditionalBooleanSelectorsSource), conditionalBooleanSelectorsSource, v940WrapBody(t, depthThreeStraightLineInitializersSource, terminalTrees(3)[25], "a"), depthThreeStraightLineInitializersSource, depthThreeArrowMethodsSource, depthThreeTerminalLeafReturnsSource, depthThreeStraightLineReturnsSource, nestedArrowMethodsSource, nestedTerminalInitializersSource, nestedStraightLineInitializersSource, nestedTerminalLeafReturnsSource, nestedStraightLineReturnsSource, terminalLeafConditionalReturnsSource, conditionalReturnCompositionSource, straightLineConditionalLocalsSource, finiteConditionalLocalsSource, twoConditionalLocalsSource, `public Class Choices {public string Select(string raw,bool pick)=>pick ? raw : trim(raw);}`} {
		var baseline [][]byte
		for _, contract := range []LanguageContract{PipeLangLanguageContractV1070, PipeLangLanguageContractV1080} {
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
			h.LanguageContract = coreir.LanguageContractV1070
			p.LanguageContract = coreir.LanguageContractV1070
			projection.LanguageContract = PipeLangLanguageContractV1070
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

func TestV1080TerminalInnerSelectorArmsLayouts(t *testing.T) {
	bundle := os.Getenv("PIPELANG_NATIVE_BUNDLE") == "1" && os.Getenv("PIPELANG_GENERATED_BATCH") == "1" && os.Getenv("PIPELANG_COMPILED_CACHE") != "" && os.Getenv("GOFLAGS") == "" && os.Getenv("GOENV") == "off"
	shapes := terminalTrees(3)[1:]
	if len(shapes) != 25 {
		t.Fatal("shape inventory drift")
	}
	for partition := 0; partition < 600; partition++ {
		shape := partition / 24
		family := (partition/8)%3 + 1
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
				for bit := 0; bit < 12; bit++ {
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
							value = fmt.Sprintf("(c%d != c%d)", bit, 9+depth)
						}
						return fmt.Sprintf(`Check("S%d%s",%s)`, index, label, value)
					}
					cond := atom("O", 9+depth, false)
					if positions[index] {
						left, right := atom("C", 2, false), atom("G", 6, false)
						if family&1 != 0 {
							left = "(" + atom("B", 1, false) + " ? " + atom("C", 2, false) + " : " + atom("D", 3, false) + ")"
						}
						if family&2 != 0 {
							right = "(" + atom("E", 4, false) + " ? " + atom("F", 5, false) + " : " + atom("G", 6, false) + ")"
						}
						cond = "(" + atom("A", 0, false) + " ? " + left + " : " + right + ") ? " + atom("H", 7, true) + " : " + atom("I", 8, true)
					} else if index%3 == 1 {
						cond = atom("A", 0, false) + " ? " + atom("C", 2, true) + " : " + atom("F", 4, true)
					} else if index%3 == 2 {
						cond = "(" + atom("A", 0, false) + " ? " + atom("B", 1, false) + " : " + atom("B", 1, false) + ") ? (" + atom("C", 2, false) + " ? " + atom("D", 3, true) + " : " + atom("F", 4, true) + ") : " + atom("F", 4, true)
					}

					fmt.Fprintf(&source, "if(%s){", cond)
					emit(n.yes, index*2+1, depth+1, previous)
					source.WriteString("}else{")
					emit(n.no, index*2+2, depth+1, previous)
					source.WriteString("}")
				}
				emit(tree, 0, 0, "raw")
				source.WriteString("}}")
				a, p := conditionalLocalTreeProgramVersion(t, PipeLangLanguageContractV1080, source.String(), []string{"Select", "Echo", "Check"})
				aa, pp := conditionalLocalTreeProgramVersion(t, PipeLangLanguageContractV1080, source.String(), []string{"Select", "Echo", "Check"})
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
				oracleDigest := sha256.New()
				for bits := 0; bits < 4096; bits++ {
					flags := make([]bool, 12)
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
						picked := flags[9+depth]
						if positions[index] {
							trace = append(trace, fmt.Sprintf("C:S%dA", index))
							bit, label := 6, "G"
							if flags[0] {
								bit, label = 2, "C"
								if family&1 != 0 {
									trace = append(trace, fmt.Sprintf("C:S%dB", index))
									if !flags[1] {
										bit, label = 3, "D"
									}
								}
							} else if family&2 != 0 {
								trace = append(trace, fmt.Sprintf("C:S%dE", index))
								if flags[4] {
									bit, label = 5, "F"
								}
							}
							trace = append(trace, fmt.Sprintf("C:S%d%s", index, label))
							outerBit, outerLabel := 8, "I"
							if flags[bit] {
								outerBit, outerLabel = 7, "H"
							}
							trace = append(trace, fmt.Sprintf("C:S%d%s", index, outerLabel))
							picked = flags[outerBit] != flags[9+depth]
						} else if index%3 != 0 {
							trace = append(trace, fmt.Sprintf("C:S%dA", index))
							label, bit := "F", 4
							if index%3 == 1 {
								if flags[0] {
									label, bit = "C", 2
								}
							} else {
								trace = append(trace, fmt.Sprintf("C:S%dB", index))
								if flags[1] {
									trace = append(trace, fmt.Sprintf("C:S%dC", index))
									if flags[2] {
										label, bit = "D", 3
									}
								}
							}
							trace = append(trace, fmt.Sprintf("C:S%d%s", index, label))
							picked = flags[bit] != flags[9+depth]
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
					if os.Getenv("PIPELANG_BUNDLE_AUDIT") == "1" {
						fmt.Fprintf(oracleDigest, "%d %v %q %#v\n", bits, flags, want, trace)
					}
				}
				if os.Getenv("PIPELANG_BUNDLE_AUDIT") == "1" {
					t.Logf("v108_layout_oracle subset=%d vectors=%d sha256=%x", subset, len(rows), oracleDigest.Sum(nil))
				}
				serializedFixture := generatedPhase(t, "fixture_serialization")
				fixtures := map[string][]byte{}
				call := `PipeLangSelect("raw"`
				loader := ""
				imports := `"testing";"encoding/json";"os"`
				loop := "for _,r:=range load(t)"
				if bundle {
					compact := make([]finiteConditionalOracleCase, len(rows))
					for i, row := range rows {
						compact[i] = finiteConditionalOracleCase{Value: row.Value, Trace: row.Trace}
					}
					fixtures["Select.oracle"] = encodeFiniteBinaryOracle(compact)
					imports = `"testing";"pipelang-generated-check/oracle"`
					loop = `for bits,r:=range oracle.Load(t,"Select.oracle",4096)`
				} else {
					fixture, err := json.Marshal(rows)
					if err != nil {
						t.Fatal(err)
					}
					fixtures["oracle.json"] = fixture
					loader = `type row struct{Flags []bool;Value string;Trace []string};func load(t *testing.T)[]row{b,e:=os.ReadFile("oracle.json");if e!=nil{t.Fatal(e)};var rows []row;if e=json.Unmarshal(b,&rows);e!=nil{t.Fatal(e)};if len(rows)!=4096{t.Fatal("oracle inventory")};return rows}`
				}
				serializedFixture()
				for bit := 0; bit < 12; bit++ {
					if bundle {
						// Reproduce the independent oracle's input mapping in vector order.
						call += fmt.Sprintf(",bits&%d!=0", 1<<bit)
					} else {
						call += fmt.Sprintf(",r.Flags[%d]", bit)
					}
				}
				call += ")"
				checks := fmt.Sprintf("package %s\nimport(%s)\n%s\nfunc TestValues(t *testing.T){%s{if got:=%s;got!=r.Value{t.Fatal(got,r.Value)}}}", gobackend.PackageName, imports, loader, loop, call)
				run := func(source []byte, checks string) {
					if bundle {
						if !queueGeneratedBatchWithOracle(t, source, []byte(checks), fixtures, []byte(finiteBinaryOracle)) {
							t.Fatal("v108 layout bundle unexpectedly ineligible")
						}
					} else {
						compileAndRunGeneratedGoFilesWithFixtures(t, source, []byte(checks), fixtures)
					}
				}
				run(generated, checks)
				observed := string(generated)
				for marker, probe := range map[string]string{"func PipeLangEcho(p0 string) string {": `v1080Trace=append(v1080Trace,"E:"+p0)`, "func PipeLangCheck(p0 string, p1 bool) bool {": `v1080Trace=append(v1080Trace,"C:"+p0)`} {
					if strings.Count(observed, marker) != 1 {
						t.Fatal("trace marker")
					}
					observed = strings.Replace(observed, marker, marker+"\n"+probe, 1)
				}
				orders := fmt.Sprintf("package %s\nimport(%s;\"reflect\")\nvar v1080Trace []string\n%s\nfunc TestOrder(t *testing.T){%s{v1080Trace=nil;got:=%s;if got!=r.Value||!reflect.DeepEqual(v1080Trace,r.Trace){t.Fatal(got,r.Value,v1080Trace,r.Trace)}}}", gobackend.PackageName, imports, loader, loop, call)
				run([]byte(observed), orders)
				t.Logf("v108-layout shape=%d family=%d subset=%d locals=%d vectors=%d", shape, family, subset, count, len(rows))
			}
		})
	}
}

func TestV1080TerminalInnerSelectorArmsSourceRejection(t *testing.T) {
	bodies := []string{
		`if((a ? (b ? a : c) : c) ? (b ? a : c) : c){return raw;}else{return raw;}`,
		`if((a ? b : (c ? a : b)) ? b : (c ? a : b)){return raw;}else{return raw;}`,
		`return (a ? (b ? a : c) : c) ? raw : raw;`,
		`string local=(a ? (b ? a : c) : c) ? raw : raw;return local;`,

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
		`if((a ? (b ? a : c) : (c ? a : b)) ? b : c){bool sibling=b;return raw;}else{if(a ? sibling : c){return raw;}else{return raw;}}`,
		`if((a ? (b ? a : c) : (c ? a : b)) ? b : c){bool branch=b;return raw;}else{return raw;}return branch;`,
		`if((a ? (b ? a : c) : (c ? a : b)) ? b : c){return raw;}`,
		`if((a ? (b ? a : c) : (c ? a : b)) ? b : c){return (a ? b : c) ? (b ? raw : raw) : raw;}else{return raw;}`,
		`if((a ? (b ? a : c) : (c ? a : b)) ? b : c){string value=(a ? b : c) ? (b ? raw : raw) : raw;return value;}else{return raw;}`,
		`if((a ? (b ? a : c) : (c ? a : b)) ? b : c){return trim(a ? raw : raw);}else{return raw;}`,
	}
	for depth := 0; depth < 3; depth++ {
		for i, body := range bodies {
			t.Run(fmt.Sprintf("%d/%d", depth, i), func(t *testing.T) {
				input := semanticTestModuleSet("compiler.selfhosting", []ModuleInput{testModule("compiler.selfhosting", "conditional.pipe", v1050ScopeSource(depth, body))}, nil)
				input.LanguageContract = PipeLangLanguageContractV1080
				if AnalyzeSemanticModuleSet(input).Error() == nil {
					t.Fatal("unsupported source admitted", body)
				}
			})
		}
	}
	for _, source := range []string{`public Class Choices {public bool Select(bool a,bool b,bool c)=> (a ? (b ? a : c) : c) ? b : c;}`, v1050ScopeSource(3, `if((a ? (b ? a : c) : (c ? a : b)) ? b : c){return raw;}else{return raw;}`), strings.Replace(terminalInnerSelectorArmsSource, "public string Select", "private string Select", 1)} {
		input := semanticTestModuleSet("compiler.selfhosting", []ModuleInput{testModule("compiler.selfhosting", "conditional.pipe", source)}, nil)
		input.LanguageContract = PipeLangLanguageContractV1080
		if AnalyzeSemanticModuleSet(input).Error() == nil {
			t.Fatal("depth/private admitted")
		}
	}
}

func TestV1080TerminalInnerSelectorArmsMalformedCore(t *testing.T) {
	for depth := 0; depth < 3; depth++ {
		for _, mutation := range []string{"condition", "payload", "a", "b", "c", "a type", "b type", "c type", "result type", "nested a", "nested b", "nested c", "terminal test", "reference", "identity", "version", "branch", "deeper"} {
			t.Run(fmt.Sprintf("%d/%s", depth, mutation), func(t *testing.T) {
				_, p := conditionalLocalTreeProgramVersion(t, PipeLangLanguageContractV1080, v1050ScopeSource(depth, `if((a ? (b ? a : c) : (c ? a : b)) ? b : c){return raw;}else{return raw;}`), []string{"Select"})
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

func TestV1080TerminalInnerSelectorArmsInternalCoreBoundary(t *testing.T) {
	for depth := 0; depth < 3; depth++ {
		for slot := 0; slot < 13; slot++ {
			t.Run(fmt.Sprintf("%d/%d", depth, slot), func(t *testing.T) {
				_, p := conditionalLocalTreeProgramVersion(t, PipeLangLanguageContractV1080, v1050ScopeSource(depth, `if((a ? (b ? a : c) : (c ? a : b)) ? b : c){return raw;}else{return raw;}`), []string{"Select"})
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
				if len(targets) != 13 {
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

func TestV1080TerminalInnerSelectorArmsComputedCarriers(t *testing.T) {
	for _, unused := range []bool{false, true} {
		t.Run(fmt.Sprint(unused), func(t *testing.T) {
			local := ""
			if unused {
				local = `Result<int,ArithmeticError> unused=(a ? b : c) ? Make(value) : Make(0);`
			}
			source := `public Class Choices {public Result<int,ArithmeticError> Make(int value)=>value+1;public Result<int,ArithmeticError> Select(int value,bool a,bool b,bool c){` + local + `Result<int,ArithmeticError> selected=(a ? b : c) ? Make(value) : Make(0);return selected;}}`
			source = v940WrapBody(t, source, terminalTrees(3)[25], "(a ? (b ? true : false) : (c ? true : false)) ? true : false")
			_, p := conditionalLocalTreeProgramVersion(t, PipeLangLanguageContractV1080, source, []string{"Select"})
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

func TestV1080TerminalInnerSelectorArmsNestedOperands(t *testing.T) {
	for depth := 0; depth < 3; depth++ {
		for arm := 0; arm < 3; arm++ {
			for _, fault := range []string{"payload", "condition", "true", "false", "condition type", "true type", "false type", "result type", "selector", "true depth", "false depth", "terminal"} {
				t.Run(fmt.Sprintf("%d/%d/%s", depth, arm, fault), func(t *testing.T) {
					_, p := conditionalLocalTreeProgramVersion(t, PipeLangLanguageContractV1080, v1050ScopeSource(depth, `if((a ? (b ? a : c) : (c ? a : b)) ? b : c){return raw;}else{return raw;}`), []string{"Select"})
					branch := p.Functions[len(p.Functions)-1].Body.Conditional
					for i := 0; i < depth; i++ {
						branch = branch.WhenTrue.Conditional
					}
					outer := branch.Condition.Conditional
					value := []*coreir.Expr{outer.Condition, outer.Condition.Conditional.WhenTrue, outer.Condition.Conditional.WhenFalse}[arm]
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
func TestV1080TerminalInnerSelectorArmsPriorContracts(t *testing.T) {
	for depth := 0; depth < 3; depth++ {
		for _, condition := range []string{"(a ? (b ? a : c) : c) ? b : c", "(a ? b : (c ? a : b)) ? b : c", "(a ? (b ? a : c) : (c ? a : b)) ? b : c"} {
			source := v1050ScopeSource(depth, "if("+condition+"){return raw;}else{return raw;}")
			_, p := conditionalLocalTreeProgramVersion(t, PipeLangLanguageContractV1080, source, []string{"Select"})
			for version := 1; version <= 107; version++ {
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
}

// Five independent operands, lexical/computed operands, mixed inherited deep tests,
// and branch-local eager unused bindings. The oracle is separate from source emission.
func TestV1080TerminalInnerSelectorArmsComputedAndDependent(t *testing.T) {
	for depth := 0; depth < 3; depth++ {
		t.Run(fmt.Sprint(depth), func(t *testing.T) {
			body := `string normalized=Echo(trim(raw));bool present=normalized!="";
if((Check("A",a && present) ? (Check("B",b || !present) ? Check("D",d != present) : Check("F",e && present)) : (Check("C",c && present) ? Check("G",!e) : Check("E",e || !present))) ? Check("H",d || present) : Check("I",!d)){
 string mid=Echo(normalized+"T");bool derived=mid!="";string unused=Echo(mid+"unused");return derived ? mid : raw;
}else{return Echo(normalized+"F");}`
			for i := 0; i < depth; i++ {
				body = `if(true ? (true ? (true ? true : false) : false) : false){` + body + `}else{return raw;}`
			}
			source := `public Class Choices {public string Echo(string value)=>value;public bool Check(string label,bool value)=>value;public string Select(string raw,bool a,bool b,bool c,bool d,bool e){` + body + `}}`
			_, p := conditionalLocalTreeProgramVersion(t, PipeLangLanguageContractV1080, source, []string{"Select", "Echo", "Check"})
			f := coreFunctionNamed(t, p, "Select")
			var checks, orders strings.Builder
			for _, raw := range []string{"", "  ", " ready ", "no"} {
				for bits := 0; bits < 32; bits++ {
					a, b, c, d, e := bits&1 != 0, bits&2 != 0, bits&4 != 0, bits&8 != 0, bits&16 != 0
					normalized := strings.TrimSpace(raw)
					present := normalized != ""
					trace := []string{"E:" + normalized, "C:A"}
					selector := false
					if a && present {
						trace = append(trace, "C:B")
						if b || !present {
							trace = append(trace, "C:D")
							selector = d != present
						} else {
							trace = append(trace, "C:F")
							selector = e && present
						}
					} else {
						trace = append(trace, "C:C")
						if c && present {
							trace = append(trace, "C:G")
							selector = !e
						} else {
							trace = append(trace, "C:E")
							selector = e || !present
						}
					}
					picked := !d
					if selector {
						trace = append(trace, "C:H")
						picked = d || present
					} else {
						trace = append(trace, "C:I")
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
					fmt.Fprintf(&orders, "v1080Trace=nil;if got:=%s;got!=%q||!reflect.DeepEqual(v1080Trace,%#v){t.Fatal(got,v1080Trace)}\n", call, want, trace)
				}
			}
			generated, err := gobackend.Generate(p)
			if err != nil {
				t.Fatal(err)
			}
			compileAndRunGeneratedGoFiles(t, generated, []byte(fmt.Sprintf("package %s\nimport \"testing\"\nfunc TestValues(t *testing.T){%s}", gobackend.PackageName, checks.String())))
			observed := string(generated)
			for marker, probe := range map[string]string{"func PipeLangEcho(p0 string) string {": `v1080Trace=append(v1080Trace,"E:"+p0)`, "func PipeLangCheck(p0 string, p1 bool) bool {": `v1080Trace=append(v1080Trace,"C:"+p0)`} {
				if strings.Count(observed, marker) != 1 {
					t.Fatal("trace marker")
				}
				observed = strings.Replace(observed, marker, marker+"\n"+probe, 1)
			}
			compileAndRunGeneratedGoFiles(t, []byte(observed), []byte(fmt.Sprintf("package %s\nimport(\"testing\";\"reflect\")\nvar v1080Trace []string\nfunc TestOrder(t *testing.T){%s}", gobackend.PackageName, orders.String())))
		})
	}
}

func TestV1080TerminalInnerSelectorArmsExactBoundary(t *testing.T) {
	atoms := []string{"a", "b", "c", "a", "b", "c", "a", "b", "c"}
	for slot := range atoms {
		for _, bad := range []string{"(a ? b : c)", "raw", "Check(a ? b : c)", "((a ? b : c) && a)", "missing"} {
			operands := append([]string(nil), atoms...)
			operands[slot] = bad
			condition := fmt.Sprintf("(%s ? (%s ? %s : %s) : (%s ? %s : %s)) ? %s : %s", operands[0], operands[1], operands[2], operands[3], operands[4], operands[5], operands[6], operands[7], operands[8])
			for depth := 0; depth < 3; depth++ {
				source := v1050ScopeSource(depth, "if("+condition+"){return raw;}else{return raw;}")
				input := semanticTestModuleSet("compiler.selfhosting", []ModuleInput{testModule("compiler.selfhosting", "selector.pipe", source)}, nil)
				input.LanguageContract = PipeLangLanguageContractV1080
				if AnalyzeSemanticModuleSet(input).Error() == nil {
					t.Fatal("operand boundary", slot, bad, depth)
				}
			}
		}
	}
}
