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

const nestedStraightLineReturnsSource = `public Class Choices {
 public string Select(string raw, bool pick, bool finish, bool enabled) {
  string first = pick ? trim(raw) : raw;
  string second = finish && first != "" ? first + "!" : first;
  string third = enabled && second != "" ? second + "?" : second;
  return enabled ? (finish ? third : first) : (pick ? first : raw);
 }
}`

func TestV880NestedStraightLineReturnsAdmission(t *testing.T) {
	input := semanticTestModuleSet("compiler.selfhosting", []ModuleInput{testModule("compiler.selfhosting", "nested.pipe", nestedStraightLineReturnsSource)}, nil)
	input.LanguageContract = LanguageContract("v0.88.0")
	if err := AnalyzeSemanticModuleSet(input).Error(); err != nil {
		t.Fatal(err)
	}
}

func TestV880NestedStraightLineReturnsTypes(t *testing.T) {
	for _, zero := range []bool{false, true} {
		t.Run(fmt.Sprint(zero), func(t *testing.T) { testConditionalLocalsTypeMatrix(t, PipeLangLanguageContractV880, zero) })
	}
	testConditionalLocalsDifferentTypes(t, PipeLangLanguageContractV880)
}
func TestV880NestedStraightLineReturnsCarriers(t *testing.T) {
	for _, zero := range []bool{false, true} {
		t.Run(fmt.Sprint(zero), func(t *testing.T) { testConditionalLocalsCarrierAndHostValues(t, PipeLangLanguageContractV880, zero) })
	}
}

// Four root-choice shapes: no nested arm, true only, false only, both.
// Cross ordered locals, conditional-local subsets, unused final locals and all
// independent condition values. Expected results/traces use ordinary Go control flow.
func TestV880NestedStraightLineReturnsLayouts(t *testing.T) {
	methodsTotal, outcomes := 0, 0
	for shape := 0; shape < 4; shape++ {
		t.Run(fmt.Sprint(shape), func(t *testing.T) {
			type sample struct {
				name           string
				count, choices int
				unused         bool
			}
			var samples []sample
			for _, count := range []int{0, 1, 2, 3, 5} {
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
			for start := 0; start < len(samples); start += 12 {
				end := min(start+12, len(samples))
				batch := samples[start:end]
				var source strings.Builder
				source.WriteString(`public Class Choices {public string Echo(string value)=>value;public bool Check(string path,bool value)=>value;`)
				names := []string{}
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
							fmt.Fprintf(&source, `string l%d=Check("q%d",q%d) ? Echo(%s+"T%d") : Echo(%s+"F%d");`, i, i, i, previous, i, previous, i)
						} else {
							fmt.Fprintf(&source, `string l%d=Echo(%s+"L%d");`, i, previous, i)
						}
						if !s.unused || i < s.count-1 {
							previous = fmt.Sprintf("l%d", i)
						}
					}
					left := fmt.Sprintf(`Echo(%s+"A")`, previous)
					right := fmt.Sprintf(`Echo(%s+"C")`, previous)
					if shape&1 != 0 {
						left = fmt.Sprintf(`(Check("T",b) ? Echo(%s+"A") : Echo(%s+"B"))`, previous, previous)
					}
					if shape&2 != 0 {
						right = fmt.Sprintf(`(Check("F",c) ? Echo(%s+"C") : Echo(%s+"D"))`, previous, previous)
					}
					fmt.Fprintf(&source, `return Check("R",a) ? %s : %s;}`, left, right)
				}
				source.WriteString("}")
				analysis, program := conditionalLocalTreeProgramVersion(t, PipeLangLanguageContractV880, source.String(), names)
				againAnalysis, again := conditionalLocalTreeProgramVersion(t, PipeLangLanguageContractV880, source.String(), names)
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
							}
							value := previous + suffix
							trace = append(trace, "E:"+value)
							if !s.unused || i < s.count-1 {
								previous = value
							}
						}
						trace = append(trace, "C:R")
						suffix := "C"
						if mask&(1<<s.count) != 0 {
							suffix = "A"
							if shape&1 != 0 {
								trace = append(trace, "C:T")
								if mask&(1<<(s.count+1)) == 0 {
									suffix = "B"
								}
							}
						} else if shape&2 != 0 {
							trace = append(trace, "C:F")
							if mask&(1<<(s.count+2)) == 0 {
								suffix = "D"
							}
						}
						want := previous + suffix
						trace = append(trace, "E:"+want)
						args := []coreeval.Value{{Type: f.Parameters[0].Type, String: "raw"}}
						for bit := 0; bit < s.count+3; bit++ {
							args = append(args, coreeval.Value{Type: f.Parameters[bit+1].Type, Bool: mask&(1<<bit) != 0})
						}
						got, err := coreeval.EvaluateProgram(evaluation, f.Identity, args)
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
					fmt.Fprintf(&orders, "func Test%s(t *testing.T){wants:=[][]string{%s};for mask,want:=range wants{v880Trace=nil;%s;if !reflect.DeepEqual(v880Trace,want){t.Fatal(mask,v880Trace,want)}}}\n", s.name, traces.String(), call)
					methodsTotal++
				}
				compileAndRunGeneratedGoFiles(t, generated, []byte(fmt.Sprintf("package %s\nimport \"testing\"\n%s", gobackend.PackageName, checks.String())))
				observed := string(generated)
				for marker, probe := range map[string]string{"func PipeLangEcho(p0 string) string {": "v880Trace=append(v880Trace,\"E:\"+p0)", "func PipeLangCheck(p0 string, p1 bool) bool {": "v880Trace=append(v880Trace,\"C:\"+p0)"} {
					if strings.Count(observed, marker) != 1 {
						t.Fatal("trace marker absent")
					}
					observed = strings.Replace(observed, marker, marker+"\n"+probe, 1)
				}
				compileAndRunGeneratedGoFiles(t, []byte(observed), []byte(fmt.Sprintf("package %s\nimport (\"testing\";\"reflect\")\nvar v880Trace []string\n%s", gobackend.PackageName, orders.String())))
			}
		})
	}
	t.Logf("%d methods, %d evaluator/pristine-Go outcomes and ordered/lazy traces", methodsTotal, outcomes)
}

func TestV880NestedStraightLineReturnsSourceRejection(t *testing.T) {
	for _, tc := range []struct{ name, before, after string }{
		{"condition type", "return enabled ?", "return raw ?"},
		{"inner condition type", "(finish ? third", "(third ? third"},
		{"arm type", "finish ? third", "finish ? true"},
		{"missing arm", "finish ? third : first", "finish ? third :"},
		{"unknown", "finish ? third", "finish ? missing"},
		{"depth three true", "finish ? third", "finish ? (pick ? third : first)"},
		{"depth three false", "pick ? first : raw", "pick ? first : (finish ? raw : first)"},
		{"outer condition nesting", "return enabled ?", "return (pick ? enabled : finish) ?"},
		{"inner condition nesting", "(finish ? third", "((pick ? enabled : finish) ? third"},
		{"nested initializer", "pick ? trim(raw)", "pick ? (finish ? raw : raw)"},
		{"initializer argument", "pick ? trim(raw)", "pick ? trim(finish ? raw : raw)"},
		{"return argument", "finish ? third", "finish ? trim(pick ? third : first)"},
		{"self", "pick ? trim(raw)", "pick ? first"},
		{"forward", "pick ? trim(raw)", "pick ? third"},
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
			source := strings.Replace(nestedStraightLineReturnsSource, tc.before, tc.after, 1)
			if source == nestedStraightLineReturnsSource {
				t.Fatal("mutation missed")
			}
			assertV880SourceRejected(t, source)
		})
	}
	for _, body := range []string{`=>true ? (false ? "a" : "b") : "c";`, `{return "ordinary";}`, `{}`, `{return true ? (false ? (true ? "a" : "b") : "c") : "d";}`} {
		assertV880SourceRejected(t, `public Class Choices {public string Select()`+body+`}`)
	}
}
func assertV880SourceRejected(t *testing.T, source string) {
	t.Helper()
	input := semanticTestModuleSet("compiler.selfhosting", []ModuleInput{testModule("compiler.selfhosting", "nested.pipe", source)}, nil)
	input.LanguageContract = PipeLangLanguageContractV880
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
func TestV880NestedStraightLineReturnsMalformedCore(t *testing.T) {
	for _, mutation := range []string{"missing local", "missing initializer", "missing continuation", "missing conditional", "missing condition", "missing true", "missing false", "inner missing true", "inner condition type", "arm type", "return type", "self", "forward", "position", "duplicate", "depth three", "nested initializer", "nested condition", "inner nested condition", "terminal inner", "return argument", "initializer argument", "unknown", "statement tree"} {
		t.Run(mutation, func(t *testing.T) {
			_, p := conditionalLocalTreeProgramVersion(t, PipeLangLanguageContractV880, nestedStraightLineReturnsSource, []string{"Select"})
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
			case "statement tree":
				third.Return = &coreir.Expr{Kind: coreir.ExprConditional, Type: returned.Type, Conditional: &coreir.Conditional{TerminalStatement: true, Condition: c.Condition, WhenTrue: returned, WhenFalse: inner.WhenFalse}}
			}
			assertAdmissionRejected(t, p, "")
		})
	}
}
func TestV880NestedStraightLineReturnsVersionBoundary(t *testing.T) {
	for version := 1; version <= 87; version++ {
		contract := LanguageContract(fmt.Sprintf("v0.%d.0", version))
		input := semanticTestModuleSet("compiler.selfhosting", []ModuleInput{testModule("compiler.selfhosting", "nested.pipe", nestedStraightLineReturnsSource)}, nil)
		input.LanguageContract = contract
		if AnalyzeSemanticModuleSet(input).Error() == nil {
			t.Fatalf("%s admitted nested return", contract)
		}
		_, p := conditionalLocalTreeProgramVersion(t, PipeLangLanguageContractV880, nestedStraightLineReturnsSource, []string{"Select"})
		if err := coreir.ValidateFunction(p.Functions[0]); err != nil {
			t.Fatal(err)
		}
		p.LanguageContract = string(contract)
		assertAdmissionRejected(t, p, "")
	}
	for _, contract := range []string{"v0.101.0", "unknown"} {
		_, p := conditionalLocalTreeProgramVersion(t, PipeLangLanguageContractV880, nestedStraightLineReturnsSource, []string{"Select"})
		p.LanguageContract = contract
		assertAdmissionRejected(t, p, "")
	}
}
func TestV880NestedStraightLineReturnsRepresentations(t *testing.T) {
	for _, source := range []string{nestedStraightLineReturnsSource, `public Class Choices {public string Select(string raw,bool pick,bool finish,bool enabled){return enabled ? (finish ? raw : "a") : (pick ? "b" : "c");}}`} {
		a, p := conditionalLocalTreeProgramVersion(t, PipeLangLanguageContractV880, source, []string{"Select"})
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
func TestV880NestedStraightLineReturnsInheritance(t *testing.T) {
	for _, source := range []string{terminalLeafConditionalReturnsSource, conditionalReturnCompositionSource, straightLineConditionalLocalsSource, finiteConditionalLocalsSource, twoConditionalLocalsSource, `public Class Choices {public string Select(string raw,bool pick)=>pick ? raw : trim(raw);}`} {
		var baseline [][]byte
		for _, contract := range []LanguageContract{PipeLangLanguageContractV870, PipeLangLanguageContractV880, PipeLangLanguageContractV890, PipeLangLanguageContractV900, PipeLangLanguageContractV910} {
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

func TestV880NestedStraightLineReturnsComputedCarriers(t *testing.T) {
	for _, unused := range []bool{false, true} {
		t.Run(fmt.Sprint(unused), func(t *testing.T) {
			local := ""
			if unused {
				local = "Result<int,ArithmeticError> unused=Make(value);"
			}
			source := `public Class Choices {public Result<int,ArithmeticError> Make(int value)=>value+1;
 public Result<int,ArithmeticError> Select(int value,bool a,bool b,bool c){` + local + `return a ? (b ? Make(value) : Make(0)) : (c ? Make(1) : Make(2));}}`
			_, program := conditionalLocalTreeProgramVersion(t, PipeLangLanguageContractV880, source, []string{"Select"})
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

func TestV880NestedStraightLineReturnsRefusesEmbeddedLocals(t *testing.T) {
	for _, placement := range []string{"return arm", "inner condition", "outer condition", "initializer arm"} {
		t.Run(placement, func(t *testing.T) {
			_, p := conditionalLocalTreeProgramVersion(t, PipeLangLanguageContractV880, nestedStraightLineReturnsSource, []string{"Select"})
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
