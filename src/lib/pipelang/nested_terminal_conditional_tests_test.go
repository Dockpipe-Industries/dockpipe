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

const nestedTerminalConditionalTestsSource = `public Class Choices {public string Select(string raw,bool a,bool b,bool c){if(a ? (b ? a : c) : (c ? b : a)){return trim(raw);}else{return raw;}}}`

func TestV1040NestedTerminalConditionalTestsAdmission(t *testing.T) {
	for _, source := range []string{nestedTerminalConditionalTestsSource, strings.Replace(nestedTerminalConditionalTestsSource, "public string Select", "string Select", 1), `public Class Choices {public string Select(){if(true ? (false ? true : false) : true){return "a";}else{return "b";}}}`} {
		_, p := conditionalLocalTreeProgramVersion(t, PipeLangLanguageContractV1040, source, []string{"Select"})
		if err := coreir.ValidateProgram(p); err != nil {
			t.Fatal(err)
		}
	}
}
func TestV1040NestedTerminalConditionalTestsTypes(t *testing.T) {
	testConditionalLocalsTypeMatrix(t, PipeLangLanguageContractV1040, true)
}
func TestV1040NestedTerminalConditionalTestsCarriers(t *testing.T) {
	testConditionalLocalsCarrierAndHostValues(t, PipeLangLanguageContractV1040, true)
}

func TestV1040NestedTerminalConditionalTestsInheritance(t *testing.T) {
	for _, source := range []string{terminalConditionalTestsSource, terminalBooleanSelectorInitializersSource, straightLineBooleanSelectorInitializersSource, arrowBooleanSelectorsSource, v990Wrap(t, conditionalBooleanSelectorsSource), conditionalBooleanSelectorsSource, v940WrapBody(t, depthThreeStraightLineInitializersSource, terminalTrees(3)[25], "a"), depthThreeStraightLineInitializersSource, depthThreeArrowMethodsSource, depthThreeTerminalLeafReturnsSource, depthThreeStraightLineReturnsSource, nestedArrowMethodsSource, nestedTerminalInitializersSource, nestedStraightLineInitializersSource, nestedTerminalLeafReturnsSource, nestedStraightLineReturnsSource, terminalLeafConditionalReturnsSource, conditionalReturnCompositionSource, straightLineConditionalLocalsSource, finiteConditionalLocalsSource, twoConditionalLocalsSource, `public Class Choices {public string Select(string raw,bool pick)=>pick ? raw : trim(raw);}`} {
		var baseline [][]byte
		for _, contract := range []LanguageContract{PipeLangLanguageContractV1030, PipeLangLanguageContractV1040} {
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
			h.LanguageContract = coreir.LanguageContractV1030
			p.LanguageContract = coreir.LanguageContractV1030
			projection.LanguageContract = PipeLangLanguageContractV1030
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

// Every statement-position subset in all 25 shapes is supplied. Seven test operands
// are shared across nodes, with independent result flips at each statement depth;
// three more independent inputs supply locals/returns. Nesting forms rotate by node.
func TestV1040NestedTerminalConditionalTestsLayouts(t *testing.T) {
	bundle := os.Getenv("PIPELANG_NATIVE_BUNDLE") == "1" && os.Getenv("PIPELANG_GENERATED_BATCH") == "1" && os.Getenv("PIPELANG_COMPILED_CACHE") != "" && os.Getenv("GOFLAGS") == "" && os.Getenv("GOENV") == "off"
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
				for bit := 0; bit < 13; bit++ {
					fmt.Fprintf(&source, ",bool c%d", bit)
				}
				source.WriteString("){")
				var emit func(*terminalTree, int, int, string)
				emit = func(n *terminalTree, index, depth int, previous string) {
					for slot := 0; slot < count; slot++ {
						name := fmt.Sprintf("n%dq%d", index, slot)
						init := fmt.Sprintf(`Echo(%s+"O%d")`, previous, index)
						if slot == 1 {
							init = fmt.Sprintf(`(Check("I%dA",c10) ? Check("I%dB",c11) : Check("I%dC",c12)) ? Echo(%s+"T%d") : Echo(%s+"F%d")`, index, index, index, previous, index, previous, index)
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
							fmt.Fprintf(&source, `return Check("R9",c10) ? (Check("R10",c11) ? (Check("R11",c12) ? Echo(%s+"T%d") : Echo(%s+"U%d")) : Echo(%s+"V%d")) : Echo(%s+"F%d");`, previous, index, previous, index, previous, index, previous, index)
						case 2:
							fmt.Fprintf(&source, `return (Check("R9",c10) ? Check("R10",c11) : Check("R11",c12)) ? Echo(%s+"T%d") : Echo(%s+"F%d");`, previous, index, previous, index)
						}
						return
					}
					atom := func(label string, bit int, flip bool) string {
						value := fmt.Sprintf("c%d", bit)
						if flip {
							value = fmt.Sprintf("(c%d != c%d)", bit, 7+depth)
						}
						return fmt.Sprintf(`Check("S%d%s",%s)`, index, label, value)
					}
					cond := atom("O", 7+depth, false)
					if positions[index] {
						yes, no := atom("C", 2, true), atom("F", 5, true)
						mode := (shape + subset + index) % 3
						if mode != 1 {
							yes = "(" + atom("B", 1, false) + " ? " + atom("C", 2, true) + " : " + atom("D", 3, true) + ")"
						}
						if mode != 0 {
							no = "(" + atom("E", 4, false) + " ? " + atom("F", 5, true) + " : " + atom("G", 6, true) + ")"
						}
						cond = atom("A", 0, false) + " ? " + yes + " : " + no
					} else if index%2 == 1 {
						cond = atom("A", 0, false) + " ? " + atom("C", 2, true) + " : " + atom("F", 5, true)
					}

					fmt.Fprintf(&source, "if(%s){", cond)
					emit(n.yes, index*2+1, depth+1, previous)
					source.WriteString("}else{")
					emit(n.no, index*2+2, depth+1, previous)
					source.WriteString("}")
				}
				emit(tree, 0, 0, "raw")
				source.WriteString("}}")
				a, p := conditionalLocalTreeProgramVersion(t, PipeLangLanguageContractV1040, source.String(), []string{"Select", "Echo", "Check"})
				aa, pp := conditionalLocalTreeProgramVersion(t, PipeLangLanguageContractV1040, source.String(), []string{"Select", "Echo", "Check"})
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
				for bits := 0; bits < 8192; bits++ {
					flags := make([]bool, 13)
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
								picked := flags[12]
								if flags[10] {
									trace = append(trace, fmt.Sprintf("C:I%dB", index))
									picked = flags[11]
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
									if flags[10] {
										trace = append(trace, "C:R10")
										suffix = "V"
										if flags[11] {
											trace = append(trace, "C:R11")
											suffix = "U"
											if flags[12] {
												suffix = "T"
											}
										}
									}
								} else {
									picked := flags[12]
									if flags[10] {
										trace = append(trace, "C:R10")
										picked = flags[11]
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
						picked := flags[7+depth]
						if positions[index] || index%2 == 1 {
							trace = append(trace, fmt.Sprintf("C:S%dA", index))
							label, bit := "F", 5
							mode := (shape + subset + index) % 3
							if flags[0] {
								label, bit = "C", 2
								if positions[index] && mode != 1 {
									trace = append(trace, fmt.Sprintf("C:S%dB", index))
									if !flags[1] {
										label, bit = "D", 3
									}
								}
							} else if positions[index] && mode != 0 {
								trace = append(trace, fmt.Sprintf("C:S%dE", index))
								if !flags[4] {
									label, bit = "G", 6
								}
							}
							trace = append(trace, fmt.Sprintf("C:S%d%s", index, label))
							picked = flags[bit] != flags[7+depth]
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
					t.Logf("v104_layout_oracle subset=%d vectors=%d sha256=%x", subset, len(rows), oracleDigest.Sum(nil))
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
					loop = `for bits,r:=range oracle.Load(t,"Select.oracle",8192)`
				} else {
					fixture, err := json.Marshal(rows)
					if err != nil {
						t.Fatal(err)
					}
					fixtures["oracle.json"] = fixture
					loader = `type row struct{Flags []bool;Value string;Trace []string};func load(t *testing.T)[]row{b,e:=os.ReadFile("oracle.json");if e!=nil{t.Fatal(e)};var rows []row;if e=json.Unmarshal(b,&rows);e!=nil{t.Fatal(e)};if len(rows)!=8192{t.Fatal("oracle inventory")};return rows}`
				}
				serializedFixture()
				for bit := 0; bit < 13; bit++ {
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
							t.Fatal("v104 layout bundle unexpectedly ineligible")
						}
					} else {
						compileAndRunGeneratedGoFilesWithFixtures(t, source, []byte(checks), fixtures)
					}
				}
				run(generated, checks)
				observed := string(generated)
				for marker, probe := range map[string]string{"func PipeLangEcho(p0 string) string {": `v1040Trace=append(v1040Trace,"E:"+p0)`, "func PipeLangCheck(p0 string, p1 bool) bool {": `v1040Trace=append(v1040Trace,"C:"+p0)`} {
					if strings.Count(observed, marker) != 1 {
						t.Fatal("trace marker")
					}
					observed = strings.Replace(observed, marker, marker+"\n"+probe, 1)
				}
				orders := fmt.Sprintf("package %s\nimport(%s;\"reflect\")\nvar v1040Trace []string\n%s\nfunc TestOrder(t *testing.T){%s{v1040Trace=nil;got:=%s;if got!=r.Value||!reflect.DeepEqual(v1040Trace,r.Trace){t.Fatal(got,r.Value,v1040Trace,r.Trace)}}}", gobackend.PackageName, imports, loader, loop, call)
				run([]byte(observed), orders)
				t.Logf("v104-layout shape=%d subset=%d locals=%d vectors=%d", shape, subset, count, len(rows))
			}
		})
	}
}

func v1040ScopeSource(depth int, body string) string {
	for i := 0; i < depth; i++ {
		body = "if(a){" + body + "}else{return raw;}"
	}
	return `public Class Choices {public bool Check(bool flag)=>flag;public string Select(string raw,bool a,bool b,bool c){` + body + `}}`
}
func TestV1040NestedTerminalConditionalTestsPriorContracts(t *testing.T) {
	for depth := 0; depth < 3; depth++ {
		for _, condition := range []string{"a ? (b ? a : c) : c", "a ? b : (c ? a : b)", "a ? (b ? a : c) : (c ? b : a)"} {
			source := v1040ScopeSource(depth, "if("+condition+"){return trim(raw);}else{return raw;}")
			_, p := conditionalLocalTreeProgramVersion(t, PipeLangLanguageContractV1040, source, []string{"Select"})
			for version := 1; version <= 103; version++ {
				contract := LanguageContract(fmt.Sprintf("v0.%d.0", version))
				input := semanticTestModuleSet("compiler.selfhosting", []ModuleInput{testModule("compiler.selfhosting", "conditional.pipe", source)}, nil)
				input.LanguageContract = contract
				if AnalyzeSemanticModuleSet(input).Error() == nil {
					t.Fatal("earlier source admitted new condition", depth, version)
				}
				p.LanguageContract = string(contract)
				assertAdmissionRejected(t, p, "")
			}
		}
	}
}
func TestV1040NestedTerminalConditionalTestsSourceRejection(t *testing.T) {
	bodies := []string{
		`if(a ? ((b ? a : c) ? b : c) : c){return raw;}else{return raw;}`,
		`if(a ? b : ((b ? a : c) ? b : c)){return raw;}else{return raw;}`,
		`if(a ? Check(b ? a : c) : c){return raw;}else{return raw;}`,
		`if(a ? b : Check(b ? a : c)){return raw;}else{return raw;}`,
		`if(a ? ((b ? a : c) && b) : c){return raw;}else{return raw;}`,

		`if((a ? b : c) ? b : c){return raw;}else{return raw;}`,
		`if(a ? (b ? (a ? b : c) : c) : c){return raw;}else{return raw;}`,
		`if(a ? b : (c ? a : (a ? b : c))){return raw;}else{return raw;}`,
		`if(Check(a ? b : c)){return raw;}else{return raw;}`,
		`if(raw ? b : c){return raw;}else{return raw;}`,
		`if(a ? raw : c){return raw;}else{return raw;}`,
		`if(a ? b : raw){return raw;}else{return raw;}`,
		`if(a ? later : c){bool later=b;return raw;}else{return raw;}`,
		`bool local=local;if(a ? local : c){return raw;}else{return raw;}`,
		`bool local=later;bool later=b;if(a ? local : c){return raw;}else{return raw;}`,
		`bool local=b;bool local=c;if(a ? local : c){return raw;}else{return raw;}`,
		`bool local=b;if(a ? local : c){bool local=c;return raw;}else{return raw;}`,
		`if(a ? (b ? a : c) : (c ? b : a)){bool sibling=b;return raw;}else{if(a ? sibling : c){return raw;}else{return raw;}}`,
		`if(a ? (b ? a : c) : (c ? b : a)){bool branch=b;return raw;}else{return raw;}return branch;`,
		`if(a ? (b ? a : c) : (c ? b : a)){return raw;}`,
		`if(a ? (b ? a : c) : (c ? b : a)){return (a ? b : c) ? (b ? raw : raw) : raw;}else{return raw;}`,
		`if(a ? (b ? a : c) : (c ? b : a)){string value=(a ? b : c) ? (b ? raw : raw) : raw;return value;}else{return raw;}`,
		`if(a ? (b ? a : c) : (c ? b : a)){return trim(a ? raw : raw);}else{return raw;}`,
	}
	for depth := 0; depth < 3; depth++ {
		for i, body := range bodies {
			t.Run(fmt.Sprintf("%d/%d", depth, i), func(t *testing.T) {
				input := semanticTestModuleSet("compiler.selfhosting", []ModuleInput{testModule("compiler.selfhosting", "conditional.pipe", v1040ScopeSource(depth, body))}, nil)
				input.LanguageContract = PipeLangLanguageContractV1040
				if AnalyzeSemanticModuleSet(input).Error() == nil {
					t.Fatal("unsupported source admitted", body)
				}
			})
		}
	}
	for _, source := range []string{v1040ScopeSource(3, `if(a ? (b ? a : c) : (c ? b : a)){return raw;}else{return raw;}`), strings.Replace(nestedTerminalConditionalTestsSource, "public string Select", "private string Select", 1)} {
		input := semanticTestModuleSet("compiler.selfhosting", []ModuleInput{testModule("compiler.selfhosting", "conditional.pipe", source)}, nil)
		input.LanguageContract = PipeLangLanguageContractV1040
		if AnalyzeSemanticModuleSet(input).Error() == nil {
			t.Fatal("depth/private admitted")
		}
	}
}
func v1040Clone(t *testing.T, e *coreir.Expr) *coreir.Expr {
	t.Helper()
	b, err := json.Marshal(e)
	if err != nil {
		t.Fatal(err)
	}
	var out coreir.Expr
	if err = json.Unmarshal(b, &out); err != nil {
		t.Fatal(err)
	}
	return &out
}
func TestV1040NestedTerminalConditionalTestsMalformedCore(t *testing.T) {
	for depth := 0; depth < 3; depth++ {
		for _, mutation := range []string{"condition", "payload", "a", "b", "c", "a type", "b type", "c type", "result type", "nested a", "nested b", "nested c", "terminal test", "reference", "identity", "version", "branch", "deeper"} {
			t.Run(fmt.Sprintf("%d/%s", depth, mutation), func(t *testing.T) {
				_, p := conditionalLocalTreeProgramVersion(t, PipeLangLanguageContractV1040, v1040ScopeSource(depth, `if(a ? (b ? a : c) : (c ? b : a)){return raw;}else{return raw;}`), []string{"Select"})
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
					test.Condition = v1040Clone(t, branch.Condition)
				case "nested b":
					test.WhenTrue = v1040Clone(t, branch.Condition)
				case "nested c":
					test.WhenFalse = v1040Clone(t, branch.Condition)
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
						branch.WhenTrue = v1040Clone(t, expr)
						branch = branch.WhenTrue.Conditional
					}
				}
				assertAdmissionRejected(t, p, "")
			})
		}
	}
}
func TestV1040NestedTerminalConditionalTestsInternalCoreBoundary(t *testing.T) {
	for depth := 0; depth < 3; depth++ {
		for slot := 0; slot < 10; slot++ {
			t.Run(fmt.Sprintf("%d/%d", depth, slot), func(t *testing.T) {
				_, p := conditionalLocalTreeProgramVersion(t, PipeLangLanguageContractV1040, v1040ScopeSource(depth, `if(a ? (b ? a : c) : (c ? b : a)){return raw;}else{return raw;}`), []string{"Select"})
				f := &p.Functions[len(p.Functions)-1]
				expr := &f.Body
				for i := 0; i < depth; i++ {
					expr = expr.Conditional.WhenTrue
				}
				test := expr.Conditional.Condition.Conditional
				target := []**coreir.Expr{&expr.Conditional.Condition, &test.Condition, &test.WhenTrue, &test.WhenFalse, &test.WhenTrue.Conditional.Condition, &test.WhenTrue.Conditional.WhenTrue, &test.WhenTrue.Conditional.WhenFalse, &test.WhenFalse.Conditional.Condition, &test.WhenFalse.Conditional.WhenTrue, &test.WhenFalse.Conditional.WhenFalse}[slot]
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
func TestV1040NestedTerminalConditionalTestsComputedAndDependent(t *testing.T) {
	source := `public Class Choices {public bool Check(string path,bool value)=>value;public string Echo(string value)=>value;public string Select(string raw,bool a,bool b,bool c){
 string normalized=Echo(trim(raw));bool local=Check("local",normalized!="");
 if(Check("A",a && local) ? (Check("B",b || !local) ? Check("BT",b || !local) : Check("BF",b || !local)) : (Check("C",c && local) ? Check("CT",c && local) : Check("CF",c && local))){
 string mid=Echo(normalized+"T");bool derived=Check("derived",mid!="");
 if(Check("D",derived) ? Check("E",c) : Check("F",b)){string unused=Echo(mid+"unused");return Echo(mid+"Y");}else{return Echo(mid+"N");}
 }else{return Echo(normalized+"F");}}}`
	_, p := conditionalLocalTreeProgramVersion(t, PipeLangLanguageContractV1040, source, []string{"Select", "Echo", "Check"})
	f := coreFunctionNamed(t, p, "Select")
	var checks, orders strings.Builder
	for _, raw := range []string{"", "  ", " ready ", "no"} {
		for bits := 0; bits < 8; bits++ {
			a, b, c := bits&1 != 0, bits&2 != 0, bits&4 != 0
			normalized := strings.TrimSpace(raw)
			local := normalized != ""
			trace := []string{"E:" + normalized, "C:local", "C:A"}
			picked := c && local
			if a && local {
				trace = append(trace, "C:B")
				picked = b || !local
				if picked {
					trace = append(trace, "C:BT")
				} else {
					trace = append(trace, "C:BF")
				}
			} else {
				trace = append(trace, "C:C")
				if picked {
					trace = append(trace, "C:CT")
				} else {
					trace = append(trace, "C:CF")
				}
			}
			want := normalized + "F"
			if picked {
				mid := normalized + "T"
				trace = append(trace, "E:"+mid, "C:derived", "C:D", "C:E")
				want = mid + "N"
				if c {
					trace = append(trace, "E:"+mid+"unused")
					want = mid + "Y"
				}
			}
			trace = append(trace, "E:"+want)
			args := []coreeval.Value{{Type: f.Parameters[0].Type, String: raw}, {Type: f.Parameters[1].Type, Bool: a}, {Type: f.Parameters[2].Type, Bool: b}, {Type: f.Parameters[3].Type, Bool: c}}
			got, err := coreeval.EvaluateProgram(p, f.Identity, args)
			if err != nil || !got.OK || got.Value.String != want {
				t.Fatal(got, err, want)
			}
			call := fmt.Sprintf("PipeLangSelect(%q,%t,%t,%t)", raw, a, b, c)
			fmt.Fprintf(&checks, "if got:=%s;got!=%q{t.Fatal(got)}\n", call, want)
			fmt.Fprintf(&orders, "v1040Trace=nil;if got:=%s;got!=%q||!reflect.DeepEqual(v1040Trace,%#v){t.Fatal(got,v1040Trace)}\n", call, want, trace)
		}
	}
	generated, err := gobackend.Generate(p)
	if err != nil {
		t.Fatal(err)
	}
	compileAndRunGeneratedGoFiles(t, generated, []byte(fmt.Sprintf("package %s\nimport \"testing\"\nfunc TestValues(t *testing.T){%s}", gobackend.PackageName, checks.String())))
	observed := string(generated)
	for marker, probe := range map[string]string{"func PipeLangEcho(p0 string) string {": `v1040Trace=append(v1040Trace,"E:"+p0)`, "func PipeLangCheck(p0 string, p1 bool) bool {": `v1040Trace=append(v1040Trace,"C:"+p0)`} {
		if strings.Count(observed, marker) != 1 {
			t.Fatal("trace marker")
		}
		observed = strings.Replace(observed, marker, marker+"\n"+probe, 1)
	}
	compileAndRunGeneratedGoFiles(t, []byte(observed), []byte(fmt.Sprintf("package %s\nimport(\"testing\";\"reflect\")\nvar v1040Trace []string\nfunc TestOrder(t *testing.T){%s}", gobackend.PackageName, orders.String())))
}

func TestV1040NestedTerminalConditionalTestsComputedCarriers(t *testing.T) {
	for _, unused := range []bool{false, true} {
		t.Run(fmt.Sprint(unused), func(t *testing.T) {
			local := ""
			if unused {
				local = `Result<int,ArithmeticError> unused=(a ? b : c) ? Make(value) : Make(0);`
			}
			source := `public Class Choices {public Result<int,ArithmeticError> Make(int value)=>value+1;public Result<int,ArithmeticError> Select(int value,bool a,bool b,bool c){` + local + `Result<int,ArithmeticError> selected=(a ? b : c) ? Make(value) : Make(0);return selected;}}`
			source = v940WrapBody(t, source, terminalTrees(3)[25], "a ? (b ? true : false) : (c ? true : false)")
			_, p := conditionalLocalTreeProgramVersion(t, PipeLangLanguageContractV1040, source, []string{"Select"})
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

// Public admission must reject malformed operands at both nested arms independently.
func TestV1040NestedTerminalConditionalTestsNestedOperands(t *testing.T) {
	for depth := 0; depth < 3; depth++ {
		for arm := 0; arm < 2; arm++ {
			for _, fault := range []string{"payload", "condition", "true", "false", "condition type", "true type", "false type", "result type", "selector", "true depth", "false depth", "terminal"} {
				t.Run(fmt.Sprintf("%d/%d/%s", depth, arm, fault), func(t *testing.T) {
					_, p := conditionalLocalTreeProgramVersion(t, PipeLangLanguageContractV1040, v1040ScopeSource(depth, `if(a ? (b ? a : c) : (c ? b : a)){return raw;}else{return raw;}`), []string{"Select"})
					branch := p.Functions[len(p.Functions)-1].Body.Conditional
					for i := 0; i < depth; i++ {
						branch = branch.WhenTrue.Conditional
					}
					outer := branch.Condition.Conditional
					value := outer.WhenTrue
					if arm == 1 {
						value = outer.WhenFalse
					}
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
						nested.Condition = v1040Clone(t, branch.WhenTrue)
					case "true type":
						nested.WhenTrue = v1040Clone(t, branch.WhenTrue)
					case "false type":
						nested.WhenFalse = v1040Clone(t, branch.WhenTrue)
					case "result type":
						value.Type = branch.WhenTrue.Type
					case "selector":
						nested.Condition = v1040Clone(t, value)
					case "true depth":
						nested.WhenTrue = v1040Clone(t, value)
					case "false depth":
						nested.WhenFalse = v1040Clone(t, value)
					case "terminal":
						nested.TerminalStatement = true
					}
					assertAdmissionRejected(t, p, "")
				})
			}
		}
	}
}
