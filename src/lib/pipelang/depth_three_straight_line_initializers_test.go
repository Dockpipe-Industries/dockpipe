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

const depthThreeStraightLineInitializersSource = `public Class Choices {
 public string Select(string raw,bool a,bool b,bool c) {
 string first=a ? (b ? (c ? trim(raw) : raw) : "B") : "A";
 string second=c ? (b ? first : (a ? first+"!" : first)) : first;
 return a ? (b ? (c ? second : first) : "B") : "A";
 }}`

func TestV960DepthThreeStraightLineInitializersAdmission(t *testing.T) {
	input := semanticTestModuleSet("compiler.selfhosting", []ModuleInput{testModule("compiler.selfhosting", "depth-three-initializer.pipe", depthThreeStraightLineInitializersSource)}, nil)
	input.LanguageContract = LanguageContract("v0.96.0")
	if err := AnalyzeSemanticModuleSet(input).Error(); err != nil {
		t.Fatal(err)
	}
}
func TestV960DepthThreeStraightLineInitializersInheritance(t *testing.T) {
	for _, source := range []string{depthThreeArrowMethodsSource, depthThreeTerminalLeafReturnsSource, depthThreeStraightLineReturnsSource, nestedArrowMethodsSource, nestedTerminalInitializersSource, nestedStraightLineInitializersSource, nestedTerminalLeafReturnsSource, nestedStraightLineReturnsSource, terminalLeafConditionalReturnsSource, conditionalReturnCompositionSource, straightLineConditionalLocalsSource, finiteConditionalLocalsSource, twoConditionalLocalsSource, `public Class Choices {public string Select(string raw,bool pick)=>pick ? raw : trim(raw);}`} {
		var baseline [][]byte
		for _, contract := range []LanguageContract{PipeLangLanguageContractV950, PipeLangLanguageContractV960} {
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
			h.LanguageContract = coreir.LanguageContractV950
			p.LanguageContract = coreir.LanguageContractV950
			projection.LanguageContract = PipeLangLanguageContractV950
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

func TestV960DepthThreeStraightLineInitializersTypes(t *testing.T) {
	testConditionalLocalsTypeMatrix(t, PipeLangLanguageContractV960)
}
func TestV960DepthThreeStraightLineInitializersCarriers(t *testing.T) {
	testConditionalLocalsCarrierAndHostValues(t, PipeLangLanguageContractV960)
}

// Every shape exhausts seven independent initializer bits and one independent return bit.
// All subsets of three local slots cross used/unused final bindings; later locals depend on earlier ones.
// Initializer condition bits are shared between locals: this is a bounded sequence matrix.
func TestV960DepthThreeStraightLineInitializersLayouts(t *testing.T) {
	shapes := terminalTrees(3)[1:]
	if len(shapes) != 25 {
		t.Fatal(len(shapes))
	}
	for shape, tree := range shapes {
		t.Run(fmt.Sprint(shape), func(t *testing.T) {
			enterFiniteShape(t)
			masks := []int{0, 1, 2, 3, 4, 5, 6, 7}
			if shape == 24 {
				masks = append(masks, 21, 31)
			}
			for _, choices := range masks {
				count := 3
				if choices > 7 {
					count = 5
				}
				for _, unused := range []bool{false, true} {
					var source strings.Builder
					source.WriteString(`public Class Choices {public string Echo(string value)=>value;public bool Check(string path,bool value)=>value;public string Select(string raw`)
					for bit := 0; bit < 7; bit++ {
						fmt.Fprintf(&source, ",bool c%d", bit)
					}
					source.WriteString(",bool finish){")
					previous := "raw"
					for i := 0; i < count; i++ {
						init := fmt.Sprintf(`Echo(%s+"O%d")`, previous, i)
						if choices&(1<<i) != 0 {
							localTree := tree
							if count == 5 {
								localTree = shapes[(shape+i)%len(shapes)]
							}
							init = v930ChoiceSource(localTree, 0, previous)
						}
						fmt.Fprintf(&source, "string l%d=%s;", i, init)
						if !unused || i < count-1 {
							previous = fmt.Sprintf("l%d", i)
						}
					}
					if choices%2 == 0 {
						fmt.Fprintf(&source, `return Echo(%s);}}`, previous)
					} else {
						fmt.Fprintf(&source, `return Check("R",finish) ? (c0 ? (c1 ? Echo(%s+"R") : Echo(%s+"S")) : Echo(%s+"T")) : Echo(%s+"F");}}`, previous, previous, previous, previous)
					}
					a, p := conditionalLocalTreeProgramVersion(t, PipeLangLanguageContractV960, source.String(), []string{"Select", "Echo", "Check"})
					aa, pp := conditionalLocalTreeProgramVersion(t, PipeLangLanguageContractV960, source.String(), []string{"Select", "Echo", "Check"})
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
					var checks, orders strings.Builder
					for mask := 0; mask < 128; mask++ {
						for _, finish := range []bool{false, true} {
							want := "raw"
							trace := []string{}

							for i := 0; i < count; i++ {
								value := want + fmt.Sprintf("O%d", i)
								if choices&(1<<i) != 0 {
									localTree := tree
									if count == 5 {
										localTree = shapes[(shape+i)%len(shapes)]
									}
									suffix, path := v930ChoiceWant(localTree, mask)
									value = want + suffix
									for _, event := range path {
										trace = append(trace, "C:"+event)
									}
								}
								trace = append(trace, "E:"+value)
								if !unused || i < count-1 {
									want = value
								}
							}
							if choices%2 != 0 {
								trace = append(trace, "C:R")
								if finish {
									if mask&1 == 0 {
										want += "T"
									} else if mask&2 == 0 {
										want += "S"
									} else {
										want += "R"
									}
								} else {
									want += "F"
								}
							}
							trace = append(trace, "E:"+want)
							args := []coreeval.Value{{Type: f.Parameters[0].Type, String: "raw"}}
							call := `PipeLangSelect("raw"`
							for bit := 0; bit < 7; bit++ {
								flag := mask&(1<<bit) != 0
								args = append(args, coreeval.Value{Type: f.Parameters[bit+1].Type, Bool: flag})
								call += fmt.Sprintf(",%t", flag)
							}
							args = append(args, coreeval.Value{Type: f.Parameters[8].Type, Bool: finish})
							call += fmt.Sprintf(",%t)", finish)
							got, err := prepared.Evaluate(f.Identity, args)
							if err != nil || !got.OK || got.Value.String != want {
								t.Fatalf("%d/%d: %#v %v want %q", choices, mask, got, err, want)
							}
							fmt.Fprintf(&checks, "if got:=%s;got!=%q{t.Fatal(got)}\n", call, want)
							fmt.Fprintf(&orders, "v960Trace=nil;%s;if !reflect.DeepEqual(v960Trace,[]string{%s}){t.Fatal(v960Trace)}\n", call, quotedStrings(trace))
						}
					}
					compileAndRunGeneratedGoFiles(t, generated, []byte(fmt.Sprintf("package %s\nimport \"testing\"\nfunc TestValues(t *testing.T){%s}", gobackend.PackageName, checks.String())))
					observed := string(generated)
					for marker, probe := range map[string]string{"func PipeLangEcho(p0 string) string {": `v960Trace=append(v960Trace,"E:"+p0)`, "func PipeLangCheck(p0 string, p1 bool) bool {": `v960Trace=append(v960Trace,"C:"+p0)`} {
						if strings.Count(observed, marker) != 1 {
							t.Fatal("missing trace marker")
						}
						observed = strings.Replace(observed, marker, marker+"\n"+probe, 1)
					}
					compileAndRunGeneratedGoFiles(t, []byte(observed), []byte(fmt.Sprintf("package %s\nimport (\"testing\";\"reflect\")\nvar v960Trace []string\nfunc TestOrder(t *testing.T){%s}", gobackend.PackageName, orders.String())))
				}
			}
		})
	}
}
func TestV960DepthThreeStraightLineInitializersComputedCarriers(t *testing.T) {
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
			_, program := conditionalLocalTreeProgramVersion(t, PipeLangLanguageContractV960, source, []string{"Select"})
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
func TestV960DepthThreeStraightLineInitializersSourceRejection(t *testing.T) {
	prefix := `public Class Choices {public string Select(string raw,bool a,bool b,bool c) `
	deep := `a ? (b ? (c ? raw : "C") : "B") : "A"`
	cases := []string{
		`{string local=a ? (b ? (c ? (a ? raw : "D") : "C") : "B") : "A";return local;}`,
		`{string local=a ? raw : (b ? raw : (c ? raw : (a ? raw : "D")));return local;}`,
		`{string local=` + deep + `;if(a){return local;}else{return raw;}}`,
		`{string local=(a ? b : c) ? raw : raw;return local;}`,
		`{string local=trim(` + deep + `);return local;}`,
		`{string local=a ? (b ? (c ? trim(a ? raw : "x") : raw) : raw) : raw;return local;}`,
		`{string local=a ? (b ? (raw ? raw : "C") : "B") : "A";return local;}`,
		`{string local=a ? (b ? (c ? true : "C") : "B") : "A";return local;}`,
		`{string local=a ? (b ? (c ? local : "C") : "B") : "A";return local;}`,
		`{string first=a ? (b ? (c ? second : "C") : "B") : "A";string second=raw;return second;}`,
		`{string local=` + deep + `;string local=raw;return local;}`,

		`{if(a){string local=` + deep + `;return local;}else{return raw;}}`,
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
			if strings.HasPrefix(body, "{return ") && strings.HasSuffix(body, ";}") {
				bodies = append(bodies, "=>"+strings.TrimSuffix(strings.TrimPrefix(body, "{return "), "}"))
			}
			for _, body := range bodies {
				source := prefix + body + `}`
				input := semanticTestModuleSet("compiler.selfhosting", []ModuleInput{testModule("compiler.selfhosting", "depth-three.pipe", source)}, nil)
				input.LanguageContract = PipeLangLanguageContractV960
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
	for version := 1; version <= 95; version++ {
		input := semanticTestModuleSet("compiler.selfhosting", []ModuleInput{testModule("compiler.selfhosting", "depth-three.pipe", prefix+`{string local=`+deep+`;return local;}}`)}, nil)
		input.LanguageContract = LanguageContract(fmt.Sprintf("v0.%d.0", version))
		if AnalyzeSemanticModuleSet(input).Error() == nil {
			t.Fatal("older source admitted", version)
		}
	}
}

func TestV960DepthThreeStraightLineInitializersMalformedCore(t *testing.T) {
	clone := func(e *coreir.Expr) *coreir.Expr {
		b, _ := json.Marshal(e)
		var out coreir.Expr
		if err := json.Unmarshal(b, &out); err != nil {
			t.Fatal(err)
		}
		return &out
	}
	for _, mutation := range []string{"local", "initializer", "continuation", "choice", "condition", "true", "false", "third condition", "third true", "third false", "condition type", "arm type", "return type", "depth true", "depth false", "condition placement", "argument placement", "hidden local", "terminal", "self", "forward", "position", "duplicate", "unknown reference", "identity", "version", "downgrade", "statement root", "statement leaf"} {
		t.Run(mutation, func(t *testing.T) {
			_, p := conditionalLocalTreeProgramVersion(t, PipeLangLanguageContractV960, depthThreeStraightLineInitializersSource, []string{"Select"})
			f := &p.Functions[len(p.Functions)-1]
			first := f.Body.ImmutableLocal
			second := first.Return.ImmutableLocal
			root := first.Initializer.Conditional
			third := root.WhenTrue.Conditional.WhenTrue.Conditional
			switch mutation {
			case "local":
				f.Body.ImmutableLocal = nil
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
				p.LanguageContract = coreir.LanguageContractV950
			case "statement root":
				second.Return = &coreir.Expr{Kind: coreir.ExprConditional, Type: f.ReturnType, Conditional: &coreir.Conditional{TerminalStatement: true, Condition: clone(root.Condition), WhenTrue: clone(second.Return), WhenFalse: clone(second.Return)}}
			case "statement leaf":
				f.Body = coreir.Expr{Kind: coreir.ExprConditional, Type: f.ReturnType, Conditional: &coreir.Conditional{TerminalStatement: true, Condition: clone(root.Condition), WhenTrue: clone(&f.Body), WhenFalse: clone(&f.Body)}}
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
