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

func TestV970DepthThreeTerminalInitializersInheritance(t *testing.T) {
	for _, source := range []string{depthThreeStraightLineInitializersSource, depthThreeArrowMethodsSource, depthThreeTerminalLeafReturnsSource, depthThreeStraightLineReturnsSource, nestedArrowMethodsSource, nestedTerminalInitializersSource, nestedStraightLineInitializersSource, nestedTerminalLeafReturnsSource, nestedStraightLineReturnsSource, terminalLeafConditionalReturnsSource, conditionalReturnCompositionSource, straightLineConditionalLocalsSource, finiteConditionalLocalsSource, twoConditionalLocalsSource, `public Class Choices {public string Select(string raw,bool pick)=>pick ? raw : trim(raw);}`} {
		var baseline [][]byte
		for _, contract := range []LanguageContract{PipeLangLanguageContractV960, PipeLangLanguageContractV970} {
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
			h.LanguageContract = coreir.LanguageContractV960
			p.LanguageContract = coreir.LanguageContractV960
			projection.LanguageContract = PipeLangLanguageContractV960
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

func TestV970DepthThreeTerminalInitializersTypes(t *testing.T) {
	testConditionalLocalsTypeMatrix(t, PipeLangLanguageContractV970)
}
func TestV970DepthThreeTerminalInitializersCarriers(t *testing.T) {
	testConditionalLocalsCarrierAndHostValues(t, PipeLangLanguageContractV970)
}

func TestV970DepthThreeTerminalInitializersComputedCarriers(t *testing.T) {
	for _, unused := range []bool{false, true} {
		t.Run(fmt.Sprint(unused), func(t *testing.T) {
			local := ""
			if unused {
				local = "Result<int,ArithmeticError> unused=a ? (b ? Make(value) : Make(value)) : (c ? Make(value) : Make(value));"
			}
			body := `a ? (b ? (c ? Make(value) : Make(0)) : Make(0)) : (c ? Make(1) : Make(2))`
			source := `public Class Choices {public Result<int,ArithmeticError> Make(int value)=>value+1;public Result<int,ArithmeticError> Choose(int value,bool a,bool b,bool c){Result<int,ArithmeticError> selected=` + body + `;return selected;}public Result<int,ArithmeticError> Select(int value,bool a,bool b,bool c){` + local + `return Choose(value,a,b,c);}}`

			if !unused {
				source = strings.Replace(source, "{return Choose(value,a,b,c);}", "=>Choose(value,a,b,c);", 1)
			}
			source = v940WrapMethod(t, source, terminalTrees(3)[25], "a", "Choose")
			_, program := conditionalLocalTreeProgramVersion(t, PipeLangLanguageContractV970, source, []string{"Select"})
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
func TestV970DepthThreeTerminalInitializersMalformedCore(t *testing.T) {
	clone := func(e *coreir.Expr) *coreir.Expr {
		b, _ := json.Marshal(e)
		var out coreir.Expr
		if err := json.Unmarshal(b, &out); err != nil {
			t.Fatal(err)
		}
		return &out
	}
	for _, mutation := range []string{"local", "initializer", "continuation", "choice", "condition", "true", "false", "third condition", "third true", "third false", "condition type", "arm type", "return type", "depth true", "depth false", "condition placement", "argument placement", "hidden local", "terminal", "self", "forward", "position", "duplicate", "unknown reference", "identity", "version", "downgrade"} {
		t.Run(mutation, func(t *testing.T) {
			_, p := conditionalLocalTreeProgramVersion(t, PipeLangLanguageContractV970, v940WrapBody(t, depthThreeStraightLineInitializersSource, terminalTrees(1)[1], "a"), []string{"Select"})
			f := &p.Functions[len(p.Functions)-1]
			first := f.Body.Conditional.WhenTrue.ImmutableLocal
			second := first.Return.ImmutableLocal
			root := first.Initializer.Conditional
			third := root.WhenTrue.Conditional.WhenTrue.Conditional
			switch mutation {
			case "local":
				f.Body.Conditional.WhenTrue.ImmutableLocal = nil
			case "initializer":
				first.Initializer = nil
			case "continuation":
				first.Return = nil
			case "choice":
				first.Initializer.Conditional = nil
			case "condition":
				root.Condition = nil
			case "true":
				root.WhenTrue = nil
			case "false":
				root.WhenFalse = nil
			case "third condition":
				third.Condition = nil
			case "third true":
				third.WhenTrue = nil
			case "third false":
				third.WhenFalse = nil
			case "condition type":
				third.Condition = third.WhenFalse
			case "arm type":
				third.WhenTrue = third.Condition
			case "return type":
				first.Initializer.Type = third.Condition.Type
			case "depth true":
				third.WhenTrue = clone(root.WhenTrue.Conditional.WhenTrue)
			case "depth false":
				third.WhenFalse = clone(root.WhenTrue.Conditional.WhenTrue)
			case "condition placement":
				third.Condition = &coreir.Expr{Kind: coreir.ExprConditional, Type: third.Condition.Type, Conditional: &coreir.Conditional{Condition: third.Condition, WhenTrue: third.Condition, WhenFalse: third.Condition}}
			case "argument placement":
				first.Initializer = &coreir.Expr{Kind: coreir.ExprTextTrim, Type: first.Initializer.Type, TextTrim: &coreir.TextTrim{Value: clone(first.Initializer)}}
			case "hidden local":
				third.WhenTrue = clone(&f.Body)
			case "terminal":
				third.TerminalStatement = true
			case "self":
				third.WhenTrue = clone(second.Initializer.Conditional.WhenFalse)
			case "forward":
				third.WhenTrue = clone(second.Return.Conditional.WhenTrue.Conditional.WhenTrue.Conditional.WhenTrue)
			case "position":
				first.Position = 999
			case "duplicate":
				second.Name = first.Name
			case "unknown reference":
				ref := 999
				third.WhenFalse = &coreir.Expr{Kind: coreir.ExprReference, Type: third.WhenFalse.Type, Parameter: &ref}
			case "identity":
				p.CompilerContract = "unknown"
			case "version":
				p.LanguageContract = "unknown"
			case "downgrade":
				p.LanguageContract = coreir.LanguageContractV960
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
}

// The model is separate from parser/HIR/Core. Statement bits, initializer bits
// and the return bit are independent. Locals share initializer bits, and the
// scope masks and shape rotations are bounded samples, not a Cartesian claim.
func TestV970DepthThreeTerminalInitializersLayouts(t *testing.T) {
	bundle := os.Getenv("PIPELANG_NATIVE_BUNDLE") == "1" && os.Getenv("PIPELANG_GENERATED_BATCH") == "1" && os.Getenv("PIPELANG_COMPILED_CACHE") != "" && os.Getenv("GOFLAGS") == "" && os.Getenv("GOENV") == "off"
	shapes := terminalTrees(3)[1:]
	if len(shapes) != 25 {
		t.Fatal("shape inventory drift")
	}
	for partition := 0; partition < 100; partition++ {
		shape := partition / 4
		tree := shapes[shape]
		t.Run(fmt.Sprint(partition), func(t *testing.T) {
			enterFiniteShape(t)
			type layout struct {
				choices, scopes, count int
				unused                 bool
			}
			layouts := []layout{}
			for choices := 0; choices < 8; choices++ {
				for _, unused := range []bool{false, true} {
					layouts = append(layouts, layout{choices, 7, 3, unused})
				}
			}
			if shape == 24 {
				for scopes := 0; scopes < 8; scopes++ {
					for _, unused := range []bool{false, true} {
						layouts = append(layouts, layout{7, scopes, 3, unused})
					}
				}
				layouts = append(layouts, layout{21, 7, 5, false}, layout{31, 7, 5, true})
			}
			for sample, l := range layouts {
				if sample%4 != partition%4 {
					continue
				}
				var source strings.Builder
				source.WriteString(`public Class Choices {public string Echo(string value)=>value;public bool Check(string path,bool value)=>value;public string Select(string raw`)
				for _, prefix := range []string{"s", "c"} {
					for bit := 0; bit < 7; bit++ {
						fmt.Fprintf(&source, ",bool %s%d", prefix, bit)
					}
				}
				source.WriteString(",bool finish){")
				scopeKind := func(n *terminalTree, index int) int {
					if index == 0 {
						return 0
					}
					if n == nil {
						return 2
					}
					return 1
				}
				var emit func(*terminalTree, int, string)
				emit = func(n *terminalTree, index int, previous string) {
					for i := 0; i < l.count; i++ {
						name := fmt.Sprintf("n%dq%d", index, i)
						init := fmt.Sprintf(`Echo(%s+"O%d.%d")`, previous, index, i)
						if l.scopes&(1<<scopeKind(n, index)) != 0 && l.choices&(1<<i) != 0 {
							init = v930ChoiceSource(shapes[(shape+index+i)%25], 0, previous)
						}
						fmt.Fprintf(&source, "string %s=%s;", name, init)
						if !l.unused || i < l.count-1 {
							previous = name
						}
					}
					if n == nil {
						if l.choices&1 == 0 {
							fmt.Fprintf(&source, `return Echo(%s+"R%d");`, previous, index)
						} else {
							fmt.Fprintf(&source, `return Check("R",finish) ? (c0 ? (c1 ? Echo(%s+"T%d") : Echo(%s+"U%d")) : Echo(%s+"V%d")) : Echo(%s+"F%d");`, previous, index, previous, index, previous, index, previous, index)
						}
						return
					}
					fmt.Fprintf(&source, `if(Check("S%d",s%d) && %s!=""){`, index, index, previous)
					emit(n.yes, index*2+1, previous)
					source.WriteString("}else{")
					emit(n.no, index*2+2, previous)
					source.WriteString("}")
				}
				emit(tree, 0, "raw")
				source.WriteString("}}")
				a, p := conditionalLocalTreeProgramVersion(t, PipeLangLanguageContractV970, source.String(), []string{"Select", "Echo", "Check"})
				aa, pp := conditionalLocalTreeProgramVersion(t, PipeLangLanguageContractV970, source.String(), []string{"Select", "Echo", "Check"})
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
				// One representative for every reachable statement path; irrelevant bits
				// are collapsed. Exhaust all 128 initializer vectors and both return bits.
				paths := []int{}
				seen := map[string]bool{}
				for bits := 0; bits < 128; bits++ {
					leaf, _ := v930ChoiceWant(tree, bits)
					if !seen[leaf] {
						seen[leaf] = true
						paths = append(paths, bits)
					}
				}
				for _, statement := range paths {
					for initializer := 0; initializer < 128; initializer++ {
						for _, finish := range []bool{false, true} {
							trace := []string{}
							want := "raw"
							n, index := tree, 0
							for {
								for i := 0; i < l.count; i++ {
									value := want + fmt.Sprintf("O%d.%d", index, i)
									if l.scopes&(1<<scopeKind(n, index)) != 0 && l.choices&(1<<i) != 0 {
										suffix, path := v930ChoiceWant(shapes[(shape+index+i)%25], initializer)
										value = want + suffix
										for _, event := range path {
											trace = append(trace, "C:"+event)
										}
									}
									trace = append(trace, "E:"+value)
									if !l.unused || i < l.count-1 {
										want = value
									}
								}
								if n == nil {
									suffix := "R"
									if l.choices&1 != 0 {
										trace = append(trace, "C:R")
										suffix = "F"
										if finish {
											suffix = "V"
											if initializer&1 != 0 {
												suffix = "U"
												if initializer&2 != 0 {
													suffix = "T"
												}
											}
										}
									}
									want += fmt.Sprintf("%s%d", suffix, index)
									trace = append(trace, "E:"+want)
									break
								}
								trace = append(trace, fmt.Sprintf("C:S%d", index))
								if statement&(1<<index) != 0 {
									n = n.yes
									index = index*2 + 1
								} else {
									n = n.no
									index = index*2 + 2
								}
							}
							flags := []bool{}
							for _, bits := range []int{statement, initializer} {
								for bit := 0; bit < 7; bit++ {
									flags = append(flags, bits&(1<<bit) != 0)
								}
							}
							flags = append(flags, finish)
							args := []coreeval.Value{{Type: f.Parameters[0].Type, String: "raw"}}
							for bit, flag := range flags {
								args = append(args, coreeval.Value{Type: f.Parameters[bit+1].Type, Bool: flag})
							}
							got, err := prepared.Evaluate(f.Identity, args)
							if err != nil || !got.OK || got.Value.String != want {
								t.Fatalf("shape %d layout %+v bits %d/%d: %#v %v want %q", shape, l, statement, initializer, got, err, want)
							}
							if os.Getenv("PIPELANG_BUNDLE_AUDIT") == "1" {
								fmt.Fprintf(oracleDigest, "%d %v %q %#v\n", len(rows), flags, want, trace)
							}
							rows = append(rows, row{flags, want, trace})
						}
					}
				}
				if os.Getenv("PIPELANG_BUNDLE_AUDIT") == "1" {
					t.Logf("v097_layout_oracle layout=%d paths=%v vectors=%d sha256=%x", sample, paths, len(rows), oracleDigest.Sum(nil))
				}
				serializedFixture := generatedPhase(t, "fixture_serialization")
				fixtures := map[string][]byte{}
				call := `PipeLangSelect("raw"`
				loader := ""
				imports := `"testing";"encoding/json";"os"`
				loop := "for _,r:=range load(t)"
				if bundle {
					if len(rows) != len(paths)*256 {
						t.Fatal("v097 layout vector inventory")
					}
					compact := make([]finiteConditionalOracleCase, len(rows))
					for vector, row := range rows {
						// Check reconstruction against the independent oracle's ordered flags.
						if len(row.Flags) != 15 {
							t.Fatal("v097 layout input inventory")
						}
						for bit, flag := range row.Flags {
							mask, shift := paths[vector/256], bit
							if bit >= 7 {
								mask, shift = (vector%256)/2|(vector%2)<<7, bit-7
							}
							if flag != (mask&(1<<shift) != 0) {
								t.Fatal("v097 layout input reconstruction", vector, bit)
							}
						}
						compact[vector] = finiteConditionalOracleCase{Value: row.Value, Trace: row.Trace}
					}
					pathLiterals := make([]string, len(paths))
					for i, path := range paths {
						pathLiterals[i] = fmt.Sprint(path)
					}
					loader = "var statementPaths = []int{" + strings.Join(pathLiterals, ",") + "}"
					fixtures["Select.oracle"] = encodeFiniteBinaryOracle(compact)
					imports = `"testing";"pipelang-generated-check/oracle"`
					loop = fmt.Sprintf(`for vector,r:=range oracle.Load(t,"Select.oracle",%d)`, len(rows))
				} else {
					fixture, err := json.Marshal(rows)
					if err != nil {
						t.Fatal(err)
					}
					fixtures["oracle.json"] = fixture
					loader = `type row struct{Flags []bool;Value string;Trace []string};func load(t *testing.T)[]row{b,e:=os.ReadFile("oracle.json");if e!=nil{t.Fatal(e)};var rows []row;if e=json.Unmarshal(b,&rows);e!=nil{t.Fatal(e)};if len(rows)==0{t.Fatal("empty oracle")};return rows}`
				}
				serializedFixture()
				for bit := 0; bit < 15; bit++ {
					if bundle {
						// Finish flags vary fastest within each initializer and statement path.
						if bit < 7 {
							call += fmt.Sprintf(",statementPaths[vector/256]&%d!=0", 1<<bit)
						} else {
							call += fmt.Sprintf(",((vector%%256)/2|(vector%%2)<<7)&%d!=0", 1<<(bit-7))
						}
					} else {
						call += fmt.Sprintf(",r.Flags[%d]", bit)
					}
				}
				call += ")"
				checks := fmt.Sprintf("package %s\nimport(%s)\n%s\nfunc TestValues(t *testing.T){%s{if got:=%s;got!=r.Value{t.Fatal(got,r.Value)}}}", gobackend.PackageName, imports, loader, loop, call)
				run := func(source []byte, checks string) {
					if bundle {
						if !queueGeneratedBatchWithOracle(t, source, []byte(checks), fixtures, []byte(finiteBinaryOracle)) {
							t.Fatal("v097 layout bundle unexpectedly ineligible")
						}
					} else {
						compileAndRunGeneratedGoFilesWithFixtures(t, source, []byte(checks), fixtures)
					}
				}
				run(generated, checks)
				observed := string(generated)
				for marker, probe := range map[string]string{"func PipeLangEcho(p0 string) string {": `v970Trace=append(v970Trace,"E:"+p0)`, "func PipeLangCheck(p0 string, p1 bool) bool {": `v970Trace=append(v970Trace,"C:"+p0)`} {
					if strings.Count(observed, marker) != 1 {
						t.Fatal("missing trace marker")
					}
					observed = strings.Replace(observed, marker, marker+"\n"+probe, 1)
				}
				orders := fmt.Sprintf("package %s\nimport(%s;\"reflect\")\nvar v970Trace []string\n%s\nfunc TestOrder(t *testing.T){%s{v970Trace=nil;got:=%s;if got!=r.Value||!reflect.DeepEqual(v970Trace,r.Trace){t.Fatal(got,r.Value,v970Trace,r.Trace)}}}", gobackend.PackageName, imports, loader, loop, call)
				run([]byte(observed), orders)
				t.Logf("v097-layout shape=%d choices=%d scopes=%d locals=%d unused=%t vectors=%d", shape, l.choices, l.scopes, l.count, l.unused, len(rows))
			}
		})
	}
}
func TestV970DepthThreeTerminalInitializersSourceRejection(t *testing.T) {
	prefix := `public Class Choices {public string Select(string raw,bool a,bool b,bool c) `
	deep := `a ? (b ? (c ? raw : "C") : "B") : "A"`
	cases := []string{
		`{string local=a ? (b ? (c ? (a ? raw : "D") : "C") : "B") : "A";return local;}`,
		`{string local=a ? raw : (b ? raw : (c ? raw : (a ? raw : "D")));return local;}`,
		`{string local=(a ? b : c) ? raw : raw;return local;}`,
		`{string local=trim(` + deep + `);return local;}`,
		`{string local=a ? (b ? (c ? trim(a ? raw : "x") : raw) : raw) : raw;return local;}`,
		`{string local=a ? (b ? (raw ? raw : "C") : "B") : "A";return local;}`,
		`{string local=a ? (b ? (c ? true : "C") : "B") : "A";return local;}`,
		`{string local=a ? (b ? (c ? local : "C") : "B") : "A";return local;}`,
		`{string first=a ? (b ? (c ? second : "C") : "B") : "A";string second=raw;return second;}`,
		`{string local=` + deep + `;string local=raw;return local;}`,

		`{if(a){if(b){if(c){if(a){return raw;}else{return raw;}}else{return raw;}}else{return raw;}}else{return raw;}}`,
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
			bodies := []string{body}
			if strings.HasPrefix(body, "{") {
				bodies = append(bodies, "{if(a)"+body+"else{return raw;}}")
			}
			if strings.HasPrefix(body, "{return ") && strings.HasSuffix(body, ";}") {
				bodies = append(bodies, "=>"+strings.TrimSuffix(strings.TrimPrefix(body, "{return "), "}"))
			}
			for _, body := range bodies {
				source := prefix + body + `}`
				input := semanticTestModuleSet("compiler.selfhosting", []ModuleInput{testModule("compiler.selfhosting", "depth-three.pipe", source)}, nil)
				input.LanguageContract = PipeLangLanguageContractV970
				analysis := AnalyzeSemanticModuleSet(input)
				if analysis.Error() == nil {
					t.Fatal("admitted", body)
				}
				for _, d := range analysis.Diagnostics {
					if !d.Primary.IsValid() || d.Primary.File != "depth-three.pipe" || d.Primary.End > len(source) {
						t.Fatal("invalid diagnostic", d)
					}
				}
			}
		})
	}
	for version := 1; version <= 96; version++ {
		input := semanticTestModuleSet("compiler.selfhosting", []ModuleInput{testModule("compiler.selfhosting", "depth-three.pipe", prefix+`{string local=`+deep+`;if(a){return local;}else{return raw;}}}`)}, nil)
		input.LanguageContract = LanguageContract(fmt.Sprintf("v0.%d.0", version))
		if AnalyzeSemanticModuleSet(input).Error() == nil {
			t.Fatal("older source admitted", version)
		}
	}
}

func TestV970DepthThreeTerminalInitializersPriorCoreContracts(t *testing.T) {
	_, p := conditionalLocalTreeProgramVersion(t, PipeLangLanguageContractV970, v940WrapBody(t, depthThreeStraightLineInitializersSource, terminalTrees(1)[1], "a"), []string{"Select"})
	for version := 1; version <= 96; version++ {
		p.LanguageContract = fmt.Sprintf("v0.%d.0", version)
		if coreir.ValidateProgram(p) == nil {
			t.Fatal("older Core admitted", version)
		}
		if _, err := gobackend.Generate(p); err == nil {
			t.Fatal("older backend admitted", version)
		}
		if _, err := coreeval.EvaluateProgram(p, p.Functions[0].Identity, nil); err == nil {
			t.Fatal("older evaluator admitted", version)
		}
	}
}

// A selected bool controls a descendant branch, and a selected string controls
// its child branch. This prevents a value-independent layout oracle from being
// the only evidence for local-to-condition composition.
func TestV970DepthThreeTerminalInitializersDependentScope(t *testing.T) {
	source := `public Class Choices {public string Select(string raw,bool a,bool b,bool c,bool q,bool r){
 bool selected=a ? (b ? (c ? true : false) : false) : false;
 if(selected){
  string left=q ? (r ? (a ? trim(raw) : raw) : raw) : raw;
  if(left!=""){return left+"!";}else{return left;}
 }else{string right=b ? (c ? (q ? trim(raw) : raw) : raw) : raw;string sibling=right;return sibling+"?";}
 }}`
	_, p := conditionalLocalTreeProgramVersion(t, PipeLangLanguageContractV970, source, []string{"Select"})
	f := coreFunctionNamed(t, p, "Select")
	prepared := prepareConformanceProgram(t, p)
	generated, err := gobackend.Generate(p)
	if err != nil {
		t.Fatal(err)
	}
	var checks strings.Builder
	for _, raw := range []string{"", " ", " value "} {
		for mask := 0; mask < 32; mask++ {
			a, b, c, q, r := mask&1 != 0, mask&2 != 0, mask&4 != 0, mask&8 != 0, mask&16 != 0
			want := raw
			if a && b && c {
				if q && r {
					want = strings.TrimSpace(want)
				}
				if want != "" {
					want += "!"
				}
			} else {
				if b && c && q {
					want = strings.TrimSpace(want)
				}
				want += "?"
			}
			args := []coreeval.Value{{Type: f.Parameters[0].Type, String: raw}}
			for i, flag := range []bool{a, b, c, q, r} {
				args = append(args, coreeval.Value{Type: f.Parameters[i+1].Type, Bool: flag})
			}
			got, err := prepared.Evaluate(f.Identity, args)
			if err != nil || !got.OK || got.Value.String != want {
				t.Fatal(mask, got, err, want)
			}
			fmt.Fprintf(&checks, "if got:=PipeLangSelect(%q,%t,%t,%t,%t,%t);got!=%q{t.Fatal(got)}\n", raw, a, b, c, q, r, want)
		}
	}
	compileAndRunGeneratedGoFiles(t, generated, []byte(fmt.Sprintf("package %s\nimport \"testing\"\nfunc TestScope(t *testing.T){%s}", gobackend.PackageName, checks.String())))
	for _, pair := range [][2]string{
		{`return sibling+"?";`, `return left+"?";`},
		{`string right=b ?`, `string selected=b ?`},
		{`string left=q ?`, `string raw=q ?`},
		{`a ? trim(raw) : raw`, `a ? left : raw`},
		{`bool selected=a ?`, `bool selected=selected ?`},
		{`if(selected)`, `if(left!="")`},
		{`if(left!="")`, `if(q ? r : a)`},
	} {
		bad := strings.Replace(source, pair[0], pair[1], 1)
		if bad == source {
			t.Fatal("missed mutation")
		}
		input := semanticTestModuleSet("compiler.selfhosting", []ModuleInput{testModule("compiler.selfhosting", "scope.pipe", bad)}, nil)
		input.LanguageContract = PipeLangLanguageContractV970
		if AnalyzeSemanticModuleSet(input).Error() == nil {
			t.Fatal("scope violation admitted", pair)
		}
	}
	// Core bindings use lexical positions, which can be reused by siblings.
	// The extra sibling local has a position absent from the receiving branch.
	left := p.Functions[len(p.Functions)-1].Body.ImmutableLocal.Return.Conditional.WhenTrue.ImmutableLocal
	right := p.Functions[len(p.Functions)-1].Body.ImmutableLocal.Return.Conditional.WhenFalse.ImmutableLocal
	left.Return.Conditional.WhenTrue.Binary.Left = right.Return.ImmutableLocal.Return.Binary.Left
	if coreir.ValidateProgram(p) == nil {
		t.Fatal("Core sibling escape admitted")
	}
	if _, err := gobackend.Generate(p); err == nil {
		t.Fatal("backend sibling escape admitted")
	}
	if _, err := coreeval.EvaluateProgram(p, f.Identity, nil); err == nil {
		t.Fatal("evaluator sibling escape admitted")
	}
}

func TestV970DepthThreeTerminalInitializersTreeCoreRefusal(t *testing.T) {
	for _, scope := range []string{"root", "true", "false"} {
		for _, mutation := range []string{"hidden arm", "hidden condition", "depth four choice", "terminal initializer", "missing arm", "wrong type", "forward", "statement condition", "depth four"} {
			t.Run(scope+"/"+mutation, func(t *testing.T) {
				_, p := conditionalLocalTreeProgramVersion(t, PipeLangLanguageContractV970, strings.Replace(nestedTerminalInitializersSource, "finish ? trim(raw)", "finish ? (pick ? trim(raw) : raw)", 1), []string{"Select"})
				f := &p.Functions[0]
				root := f.Body.ImmutableLocal
				tree := root.Return.Conditional
				local := root
				if scope == "true" {
					local = tree.WhenTrue.ImmutableLocal
				}
				if scope == "false" {
					local = tree.WhenFalse.ImmutableLocal
				}
				choice := local.Initializer.Conditional
				switch mutation {
				case "hidden arm", "hidden condition":
					target := &choice.WhenTrue
					if mutation == "hidden condition" {
						target = &choice.Condition
					}
					value := *target
					position := local.Position
					*target = &coreir.Expr{Kind: coreir.ExprImmutableLocal, Type: value.Type, ImmutableLocal: &coreir.ImmutableLocal{Name: "hidden", Position: position, Type: value.Type, Initializer: value, Return: &coreir.Expr{Kind: coreir.ExprReference, Type: value.Type, Parameter: &position}}}
					if err := coreir.ValidateFunction(*f); err != nil {
						t.Fatalf("internal Core narrowed: %v", err)
					}
				case "depth four choice":
					leaf := choice.WhenTrue
					for i := 0; i < 3; i++ {
						leaf = &coreir.Expr{Kind: coreir.ExprConditional, Type: leaf.Type, Conditional: &coreir.Conditional{Condition: choice.Condition, WhenTrue: leaf, WhenFalse: choice.WhenFalse}}
					}
					choice.WhenTrue = leaf
				case "terminal initializer":
					choice.TerminalStatement = true
				case "missing arm":
					choice.WhenFalse = nil
				case "wrong type":
					choice.WhenFalse = choice.Condition
				case "forward":
					position := local.Position
					choice.WhenFalse = &coreir.Expr{Kind: coreir.ExprReference, Type: local.Type, Parameter: &position}
				case "statement condition":
					tree.Condition = &coreir.Expr{Kind: coreir.ExprConditional, Type: tree.Condition.Type, Conditional: &coreir.Conditional{Condition: tree.Condition, WhenTrue: tree.Condition, WhenFalse: tree.Condition}}
				case "depth four":
					tail := tree.WhenFalse
					for i := 0; i < 3; i++ {
						tail = &coreir.Expr{Kind: coreir.ExprConditional, Type: tail.Type, Conditional: &coreir.Conditional{TerminalStatement: true, Condition: tree.Condition, WhenTrue: tail, WhenFalse: tree.WhenFalse}}
					}
					tree.WhenFalse = tail
				}
				assertAdmissionRejected(t, p, "")
			})
		}
	}
}
