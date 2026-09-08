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

const nestedStraightLineInitializersSource = `public Class Choices {
 public string Select(string raw, bool pick, bool finish, bool enabled) {
  string first = pick ? (finish ? trim(raw) : raw) : (enabled ? "fallback" : raw);
  string second = finish && first != "" ? first + "!" : first;
  string third = enabled && second != "" ? second + "?" : second;
  return enabled ? (finish ? third : first) : (pick ? first : raw);
 }
}`

func TestV900NestedStraightLineInitializersAdmission(t *testing.T) {
	input := semanticTestModuleSet("compiler.selfhosting", []ModuleInput{testModule("compiler.selfhosting", "nested.pipe", nestedStraightLineInitializersSource)}, nil)
	input.LanguageContract = LanguageContract("v0.90.0")
	if err := AnalyzeSemanticModuleSet(input).Error(); err != nil {
		t.Fatal(err)
	}
}

func TestV900NestedStraightLineInitializersTypes(t *testing.T) {
	for _, zero := range []bool{false, true} {
		t.Run(fmt.Sprint(zero), func(t *testing.T) { testConditionalLocalsTypeMatrix(t, PipeLangLanguageContractV900, zero) })
	}
	testConditionalLocalsDifferentTypes(t, PipeLangLanguageContractV900)
}
func TestV900NestedStraightLineInitializersCarriers(t *testing.T) {
	for _, zero := range []bool{false, true} {
		t.Run(fmt.Sprint(zero), func(t *testing.T) { testConditionalLocalsCarrierAndHostValues(t, PipeLangLanguageContractV900, zero) })
	}
}

// Five initializer layouts (four uniform shapes plus a mixed rotation) cross
// ordinary returns and four return shapes. Enumerate finite local subsets, unused
// locals and all supplied conditions; expected values/traces use plain Go control flow.
func TestV900NestedStraightLineInitializersLayouts(t *testing.T) {
	methodsTotal, outcomes := 0, 0
	for layout := 0; layout < 25; layout++ {
		shape := layout/5 - 1
		initializerShape := layout % 5
		t.Run(fmt.Sprint(layout), func(t *testing.T) {
			type sample struct {
				name           string
				count, choices int
				unused         bool
			}
			var samples []sample
			for _, count := range []int{1, 2, 3, 5} {
				masks := []int{0, 21, 31}
				if count <= 3 {
					masks = nil
					for i := 0; i < 1<<count; i++ {
						masks = append(masks, i)
					}
				}
				for _, choices := range masks {
					for _, unused := range []bool{false, true} {
						samples = append(samples, sample{fmt.Sprintf("Select%d", len(samples)), count, choices, unused})
					}
				}
			}
			for start := 0; start < len(samples); start += 2 {
				end := min(start+2, len(samples))
				batch := samples[start:end]
				var source strings.Builder
				source.WriteString(`public Class Choices {public string Echo(string value)=>value;public bool Check(string path,bool value)=>value;`)
				names := []string{"Echo", "Check"}
				for _, s := range batch {
					names = append(names, s.name)
					fmt.Fprintf(&source, "public string %s(string raw", s.name)
					for i := 0; i < s.count; i++ {
						fmt.Fprintf(&source, ",bool q%d", i)
					}
					source.WriteString(",bool a,bool b,bool c){")
					previous := "raw"
					for i := 0; i < s.count; i++ {
						if s.choices&(1<<i) != 0 {
							localShape := initializerShape
							if localShape == 4 {
								localShape = i % 4
							}
							left := fmt.Sprintf(`Echo(%s+"T%d")`, previous, i)
							right := fmt.Sprintf(`Echo(%s+"F%d")`, previous, i)
							if localShape&1 != 0 {
								left = fmt.Sprintf(`(Check("I%dT",b) ? Echo(%s+"T%d") : Echo(%s+"U%d"))`, i, previous, i, previous, i)
							}
							if localShape&2 != 0 {
								right = fmt.Sprintf(`(Check("I%dF",c) ? Echo(%s+"F%d") : Echo(%s+"V%d"))`, i, previous, i, previous, i)
							}
							fmt.Fprintf(&source, `string l%d=Check("q%d",q%d) ? %s : %s;`, i, i, i, left, right)
						} else {
							fmt.Fprintf(&source, `string l%d=Echo(%s+"L%d");`, i, previous, i)
						}
						if !s.unused || i < s.count-1 {
							previous = fmt.Sprintf("l%d", i)
						}
					}
					left := fmt.Sprintf(`Echo(%s+"A")`, previous)
					right := fmt.Sprintf(`Echo(%s+"C")`, previous)
					if shape >= 0 && shape&1 != 0 {
						left = fmt.Sprintf(`(Check("T",b) ? Echo(%s+"A") : Echo(%s+"B"))`, previous, previous)
					}
					if shape >= 0 && shape&2 != 0 {
						right = fmt.Sprintf(`(Check("F",c) ? Echo(%s+"C") : Echo(%s+"D"))`, previous, previous)
					}
					if shape < 0 {
						fmt.Fprintf(&source, `return Echo(%s);}`, previous)
					} else {
						fmt.Fprintf(&source, `return Check("R",a) ? %s : %s;}`, left, right)
					}
				}
				source.WriteString("}")
				analysis, program := conditionalLocalTreeProgramVersion(t, PipeLangLanguageContractV900, source.String(), names)
				againAnalysis, again := conditionalLocalTreeProgramVersion(t, PipeLangLanguageContractV900, source.String(), names)
				projection, err := BuildSemanticProjection(analysis)
				if err != nil {
					t.Fatal(err)
				}
				repeatedProjection, err := BuildSemanticProjection(againAnalysis)
				if err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(program, again) || !reflect.DeepEqual(projection, repeatedProjection) {
					t.Fatal("nondeterministic Core/semantic output")
				}
				generated, err := gobackend.Generate(program)
				if err != nil {
					t.Fatal(err)
				}
				repeated, err := gobackend.Generate(again)
				if err != nil || !bytes.Equal(generated, repeated) {
					t.Fatal("nondeterministic Go")
				}
				var checks, orders strings.Builder
				for _, s := range batch {
					f := coreFunctionNamed(t, program, s.name)
					evaluation := program
					evaluation.Functions = []coreir.Function{coreFunctionNamed(t, program, "Echo"), coreFunctionNamed(t, program, "Check"), f}
					prepared := prepareConformanceProgram(t, evaluation)
					var wants, traces strings.Builder
					for mask := 0; mask < 1<<(s.count+3); mask++ {
						previous := "raw"
						trace := []string{}
						for i := 0; i < s.count; i++ {
							suffix := fmt.Sprintf("L%d", i)
							if s.choices&(1<<i) != 0 {
								trace = append(trace, fmt.Sprintf("C:q%d", i))
								suffix = fmt.Sprintf("F%d", i)
								if mask&(1<<i) != 0 {
									suffix = fmt.Sprintf("T%d", i)
								}
								localShape := initializerShape
								if localShape == 4 {
									localShape = i % 4
								}
								if mask&(1<<i) != 0 && localShape&1 != 0 {
									trace = append(trace, fmt.Sprintf("C:I%dT", i))
									if mask&(1<<(s.count+1)) == 0 {
										suffix = fmt.Sprintf("U%d", i)
									}
								} else if mask&(1<<i) == 0 && localShape&2 != 0 {
									trace = append(trace, fmt.Sprintf("C:I%dF", i))
									if mask&(1<<(s.count+2)) == 0 {
										suffix = fmt.Sprintf("V%d", i)
									}
								}
							}
							value := previous + suffix
							trace = append(trace, "E:"+value)
							if !s.unused || i < s.count-1 {
								previous = value
							}
						}
						if shape >= 0 {
							trace = append(trace, "C:R")
						}
						suffix := "C"
						if mask&(1<<s.count) != 0 {
							suffix = "A"
							if shape >= 0 && shape&1 != 0 {
								trace = append(trace, "C:T")
								if mask&(1<<(s.count+1)) == 0 {
									suffix = "B"
								}
							}
						} else if shape >= 0 && shape&2 != 0 {
							trace = append(trace, "C:F")
							if mask&(1<<(s.count+2)) == 0 {
								suffix = "D"
							}
						}
						if shape < 0 {
							suffix = ""
						}
						want := previous + suffix
						trace = append(trace, "E:"+want)
						args := []coreeval.Value{{Type: f.Parameters[0].Type, String: "raw"}}
						for bit := 0; bit < s.count+3; bit++ {
							args = append(args, coreeval.Value{Type: f.Parameters[bit+1].Type, Bool: mask&(1<<bit) != 0})
						}
						got, err := prepared.Evaluate(f.Identity, args)
						if err != nil || !got.OK || got.Value.String != want {
							t.Fatalf("%s/%d: %#v %v want %q", s.name, mask, got, err, want)
						}
						fmt.Fprintf(&wants, "%q,", want)
						traces.WriteString("{")
						for _, event := range trace {
							fmt.Fprintf(&traces, "%q,", event)
						}
						traces.WriteString("},")
						outcomes++
					}
					call := fmt.Sprintf("PipeLang%s(\"raw\"", s.name)
					for bit := 0; bit < s.count+3; bit++ {
						call += fmt.Sprintf(",mask&%d!=0", 1<<bit)
					}
					call += ")"
					fmt.Fprintf(&checks, "func Test%s(t *testing.T){wants:=[]string{%s};for mask,want:=range wants{if got:=%s;got!=want{t.Fatal(mask,got,want)}}}\n", s.name, wants.String(), call)
					fmt.Fprintf(&orders, "func Test%s(t *testing.T){wants:=[][]string{%s};for mask,want:=range wants{v900Trace=nil;%s;if !reflect.DeepEqual(v900Trace,want){t.Fatal(mask,v900Trace,want)}}}\n", s.name, traces.String(), call)
					methodsTotal++
				}
				compileAndRunGeneratedGoFiles(t, generated, []byte(fmt.Sprintf("package %s\nimport \"testing\"\n%s", gobackend.PackageName, checks.String())))
				observed := string(generated)
				for marker, probe := range map[string]string{"func PipeLangEcho(p0 string) string {": "v900Trace=append(v900Trace,\"E:\"+p0)", "func PipeLangCheck(p0 string, p1 bool) bool {": "v900Trace=append(v900Trace,\"C:\"+p0)"} {
					if strings.Count(observed, marker) != 1 {
						t.Fatal("trace marker absent")
					}
					observed = strings.Replace(observed, marker, marker+"\n"+probe, 1)
				}
				compileAndRunGeneratedGoFiles(t, []byte(observed), []byte(fmt.Sprintf("package %s\nimport (\"testing\";\"reflect\")\nvar v900Trace []string\n%s", gobackend.PackageName, orders.String())))
			}
		})
	}
	t.Logf("%d methods, %d evaluator/pristine-Go outcomes and ordered/lazy traces", methodsTotal, outcomes)
}

func TestV900NestedStraightLineInitializersSourceRejection(t *testing.T) {
	for _, tc := range []struct{ name, before, after string }{
		{"initializer outer condition type", "first = pick ?", "first = raw ?"},
		{"initializer inner condition type", "finish ? trim(raw)", "raw ? trim(raw)"},
		{"initializer inner arm type", "finish ? trim(raw)", "finish ? true"},
		{"initializer missing arm", "finish ? trim(raw) : raw", "finish ? trim(raw) :"},
		{"initializer condition placement", "first = pick ?", "first = (finish ? pick : enabled) ?"},
		{"initializer nested condition placement", "finish ? trim(raw)", "(pick ? finish : enabled) ? trim(raw)"},
		{"initializer ordinary sibling argument", "string second = finish", "string second = (pick ? finish : enabled)"},
		{"initializer matching", "finish ? trim(raw)", "finish ? match(raw){some(v)=>v,none=>raw}"},
		{"initializer propagation", "finish ? trim(raw)", "finish ? propagate(raw)"},
		{"condition type", "return enabled ?", "return raw ?"},
		{"inner condition type", "(finish ? third", "(third ? third"},
		{"arm type", "finish ? third", "finish ? true"},
		{"missing arm", "finish ? third : first", "finish ? third :"},
		{"unknown", "finish ? third", "finish ? missing"},
		{"depth three true", "finish ? third", "finish ? (pick ? third : first)"},
		{"depth three false", "pick ? first : raw", "pick ? first : (finish ? raw : first)"},
		{"outer condition nesting", "return enabled ?", "return (pick ? enabled : finish) ?"},
		{"inner condition nesting", "(finish ? third", "((pick ? enabled : finish) ? third"},
		{"depth-three initializer", "finish ? trim(raw)", "finish ? (pick ? raw : raw)"},
		{"initializer argument", "finish ? trim(raw)", "finish ? trim(pick ? raw : raw)"},
		{"return argument", "finish ? third", "finish ? trim(pick ? third : first)"},
		{"self", "finish ? trim(raw)", "finish ? first"},
		{"forward", "finish ? trim(raw)", "finish ? third"},
		{"duplicate", "string second", "string first"},
		{"shadow", "string first", "string raw"},
		{"inference", "string second", "var second"},
		{"assignment", "return enabled", "first=raw;return enabled"},
		{"early return", "string second", "return first;string second"},
		{"match", "finish ? third", "finish ? match(third){some(v)=>v,none=>raw}"},
		{"propagate", "finish ? third", "finish ? propagate(third)"},
		{"statement tree", "return enabled ? (finish ? third : first) : (pick ? first : raw);", "if(enabled){return finish ? (pick ? third : first) : raw;}else{return raw;}"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source := strings.Replace(nestedStraightLineInitializersSource, tc.before, tc.after, 1)
			if source == nestedStraightLineInitializersSource {
				t.Fatal("mutation missed")
			}
			assertV900SourceRejected(t, source)
		})
	}
	for _, body := range []string{`=>true ? (false ? "a" : "b") : "c";`, `{return "ordinary";}`, `{}`, `{return true ? (false ? (true ? "a" : "b") : "c") : "d";}`} {
		assertV900SourceRejected(t, `public Class Choices {public string Select()`+body+`}`)
	}
}
func assertV900SourceRejected(t *testing.T, source string) {
	t.Helper()
	input := semanticTestModuleSet("compiler.selfhosting", []ModuleInput{testModule("compiler.selfhosting", "nested.pipe", source)}, nil)
	input.LanguageContract = PipeLangLanguageContractV900
	diagnostics, ok := AsDiagnostics(AnalyzeSemanticModuleSet(input).Error())
	if !ok || len(diagnostics) == 0 {
		t.Fatal("expected source refusal")
	}
	for _, d := range diagnostics {
		if !d.Primary.IsValid() || d.Primary.File != "nested.pipe" || d.Primary.End > len(source) {
			t.Fatalf("invalid span: %#v", d)
		}
	}
}
func TestV900NestedStraightLineInitializersMalformedCore(t *testing.T) {
	for _, mutation := range []string{"missing local", "missing initializer", "missing continuation", "missing conditional", "missing condition", "missing true", "missing false", "inner missing true", "inner condition type", "arm type", "return type", "self", "forward", "position", "duplicate", "depth three", "nested initializer", "nested condition", "inner nested condition", "terminal inner", "return argument", "initializer argument", "unknown", "statement tree", "initializer missing inner", "initializer missing arm", "initializer condition type", "initializer arm type", "initializer terminal marker"} {
		t.Run(mutation, func(t *testing.T) {
			_, p := conditionalLocalTreeProgramVersion(t, PipeLangLanguageContractV900, nestedStraightLineInitializersSource, []string{"Select"})
			f := &p.Functions[0]
			first := f.Body.ImmutableLocal
			second := first.Return.ImmutableLocal
			third := second.Return.ImmutableLocal
			returned := third.Return
			c := returned.Conditional
			inner := c.WhenTrue.Conditional
			choice := func() *coreir.Expr {
				return &coreir.Expr{Kind: coreir.ExprConditional, Type: returned.Type, Conditional: &coreir.Conditional{Condition: inner.Condition, WhenTrue: inner.WhenTrue, WhenFalse: inner.WhenFalse}}
			}
			switch mutation {
			case "initializer missing inner":
				first.Initializer.Conditional.WhenTrue.Conditional = nil
			case "initializer missing arm":
				first.Initializer.Conditional.WhenTrue.Conditional.WhenFalse = nil
			case "initializer condition type":
				first.Initializer.Conditional.WhenTrue.Conditional.Condition = first.Initializer.Conditional.WhenTrue.Conditional.WhenTrue
			case "initializer arm type":
				first.Initializer.Conditional.WhenTrue.Conditional.WhenTrue = first.Initializer.Conditional.Condition
			case "initializer terminal marker":
				first.Initializer.Conditional.WhenTrue.Conditional.TerminalStatement = true
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
				third.Initializer.Conditional.WhenTrue = inner.WhenTrue
			case "forward":
				first.Initializer.Conditional.WhenTrue = inner.WhenTrue
			case "position":
				third.Position = 999
			case "duplicate":
				third.Name = first.Name
			case "depth three":
				inner.WhenTrue = choice()
			case "nested initializer":
				init := first.Initializer.Conditional.WhenTrue.Conditional
				init.WhenTrue = &coreir.Expr{Kind: coreir.ExprConditional, Type: init.WhenTrue.Type, Conditional: &coreir.Conditional{Condition: init.Condition, WhenTrue: init.WhenTrue, WhenFalse: init.WhenFalse}}
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
			case "statement tree":
				third.Return = &coreir.Expr{Kind: coreir.ExprConditional, Type: returned.Type, Conditional: &coreir.Conditional{TerminalStatement: true, Condition: c.Condition, WhenTrue: returned, WhenFalse: inner.WhenFalse}}
			}
			assertAdmissionRejected(t, p, "")
		})
	}
}
func TestV900NestedStraightLineInitializersVersionBoundary(t *testing.T) {
	for version := 1; version <= 89; version++ {
		contract := LanguageContract(fmt.Sprintf("v0.%d.0", version))
		input := semanticTestModuleSet("compiler.selfhosting", []ModuleInput{testModule("compiler.selfhosting", "nested.pipe", nestedStraightLineInitializersSource)}, nil)
		input.LanguageContract = contract
		if AnalyzeSemanticModuleSet(input).Error() == nil {
			t.Fatalf("%s admitted nested return", contract)
		}
		_, p := conditionalLocalTreeProgramVersion(t, PipeLangLanguageContractV900, nestedStraightLineInitializersSource, []string{"Select"})
		if err := coreir.ValidateFunction(p.Functions[0]); err != nil {
			t.Fatal(err)
		}
		p.LanguageContract = string(contract)
		assertAdmissionRejected(t, p, "")
	}
	for _, contract := range []string{"v0.101.0", "unknown"} {
		_, p := conditionalLocalTreeProgramVersion(t, PipeLangLanguageContractV900, nestedStraightLineInitializersSource, []string{"Select"})
		p.LanguageContract = contract
		assertAdmissionRejected(t, p, "")
	}
}
func TestV900NestedStraightLineInitializersRepresentations(t *testing.T) {
	for _, source := range []string{nestedStraightLineInitializersSource, `public Class Choices {public string Select(string raw,bool pick,bool finish,bool enabled){return enabled ? (finish ? raw : "a") : (pick ? "b" : "c");}}`} {
		a, p := conditionalLocalTreeProgramVersion(t, PipeLangLanguageContractV900, source, []string{"Select"})
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
			hi, ci := he.ImmutableLocal.Initializer, ce.ImmutableLocal.Initializer
			if hi.Conditional == nil || ci.Conditional == nil || hi.Conditional.TerminalStatement || ci.Conditional.TerminalStatement {
				t.Fatal("initializer choice marker")
			}
			if (hi.Conditional.WhenTrue.Conditional != nil) != (ci.Conditional.WhenTrue.Conditional != nil) || (hi.Conditional.WhenFalse.Conditional != nil) != (ci.Conditional.WhenFalse.Conditional != nil) {
				t.Fatal("initializer HIR/Core nesting mismatch")
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
func TestV900NestedStraightLineInitializersInheritance(t *testing.T) {
	for _, source := range []string{nestedTerminalLeafReturnsSource, nestedStraightLineReturnsSource, terminalLeafConditionalReturnsSource, conditionalReturnCompositionSource, straightLineConditionalLocalsSource, finiteConditionalLocalsSource, twoConditionalLocalsSource, `public Class Choices {public string Select(string raw,bool pick)=>pick ? raw : trim(raw);}`} {
		var baseline [][]byte
		for _, contract := range []LanguageContract{PipeLangLanguageContractV890, PipeLangLanguageContractV900, PipeLangLanguageContractV910} {
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

func TestV900NestedStraightLineInitializersComputedCarriers(t *testing.T) {
	for _, unused := range []bool{false, true} {
		t.Run(fmt.Sprint(unused), func(t *testing.T) {
			local := ""
			if unused {
				local = "Result<int,ArithmeticError> unused=a ? (b ? Make(value) : Make(value)) : (c ? Make(value) : Make(value));"
			}
			source := `public Class Choices {public Result<int,ArithmeticError> Make(int value)=>value+1;
 public Result<int,ArithmeticError> Select(int value,bool a,bool b,bool c){` + local + `Result<int,ArithmeticError> selected=a ? (b ? Make(value) : Make(0)) : (c ? Make(1) : Make(2)); return selected;}}`
			_, program := conditionalLocalTreeProgramVersion(t, PipeLangLanguageContractV900, source, []string{"Select"})
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

func TestV900NestedStraightLineInitializersRefusesEmbeddedLocals(t *testing.T) {
	for _, placement := range []string{"return arm", "inner condition", "outer condition", "initializer arm", "nested initializer arm", "nested initializer condition"} {
		t.Run(placement, func(t *testing.T) {
			_, p := conditionalLocalTreeProgramVersion(t, PipeLangLanguageContractV900, nestedStraightLineInitializersSource, []string{"Select"})
			f := &p.Functions[0]
			first := f.Body.ImmutableLocal
			third := first.Return.ImmutableLocal.Return.ImmutableLocal
			returned := third.Return
			choice := returned.Conditional
			position := len(f.Parameters) + 3
			target := &choice.WhenTrue.Conditional.WhenTrue
			switch placement {
			case "inner condition":
				target = &choice.WhenTrue.Conditional.Condition
			case "outer condition":
				target = &choice.Condition
			case "nested initializer arm":
				target = &first.Initializer.Conditional.WhenTrue.Conditional.WhenTrue
				position = len(f.Parameters)
			case "nested initializer condition":
				target = &first.Initializer.Conditional.WhenFalse.Conditional.Condition
				position = len(f.Parameters)
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

// A malformed AST must not use the public initializer admission to smuggle a
// local expression into a nested arm. Source text cannot spell this embedding.
func TestV900NestedStraightLineInitializersSourceASTPlacement(t *testing.T) {
	for _, nested := range []bool{false, true} {
		a, _ := conditionalLocalTreeProgramVersion(t, PipeLangLanguageContractV900, nestedStraightLineInitializersSource, []string{"Select"})
		body := a.Program.Classes[0].Methods[0].Body
		first := body.(*ImmutableLocalExpr)
		target := &first.Initializer.(*ConditionalExpr).WhenTrue
		if nested {
			target = &first.Initializer.(*ConditionalExpr).WhenTrue.(*ConditionalExpr).WhenTrue
		}
		original := *target
		hidden := *first
		hidden.Name = "hidden"
		hidden.Initializer = original
		hidden.Return = original
		*target = &hidden
		if validNestedStraightLineInitializers(body) {
			t.Fatal("hidden AST local admitted")
		}
	}
}
