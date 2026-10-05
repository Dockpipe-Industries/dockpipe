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

const terminalLeafSelectorValueArmsSeed = `public Class Choices {public string Select(string raw,bool a,bool b,bool c){string local=a ? (b ? (c ? trim(raw) : raw) : raw) : raw;return (a ? b : c) ? (a ? local : raw) : (b ? raw : local);}}`

func TestV1110TerminalLeafSelectorValueArmsAdmission(t *testing.T) {
	for _, source := range []string{terminalLeafSelectorValueArmsSeed, `public Class Choices {public string Select(string raw,bool a,bool b,bool c){return (a ? b : c) ? (b ? trim(raw) : raw) : raw;}}`, `public Class Choices {string Select(string raw,bool a,bool b,bool c){string one=(a ? b : c) ? raw : trim(raw);return (a ? b : c) ? one : (b ? trim(raw) : raw);}}`} {
		_, p := conditionalLocalTreeProgramVersion(t, PipeLangLanguageContractV1110, v990Wrap(t, source), []string{"Select"})
		if err := coreir.ValidateProgram(p); err != nil {
			t.Fatal(err)
		}
	}
}

func TestV1110TerminalLeafSelectorValueArmsTypes(t *testing.T) {
	for i := 0; i < 6; i++ {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			arms := i/2 + 1
			testConditionalLocalsTypeMatrix(t, PipeLangLanguageContractV1110, i%2 != 0, arms&1 != 0, arms&2 != 0)
		})
	}
}

func TestV1110TerminalLeafSelectorValueArmsCarriers(t *testing.T) {
	for i := 0; i < 6; i++ {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			arms := i/2 + 1
			testConditionalLocalsCarrierAndHostValues(t, PipeLangLanguageContractV1110, i%2 != 0, arms&1 != 0, arms&2 != 0)
		})
	}
}

func TestV1110TerminalLeafSelectorValueArmsLayouts(t *testing.T) {
	for i := 0; i < 18; i++ {
		t.Run(fmt.Sprint(i), func(t *testing.T) { v1110Layouts(t, i) })
	}
}
func v1110Layouts(t *testing.T, index int) {
	arms := index/6 + 1
	for _, count := range []int{[]int{0, 1, 4}[(index%6)/2]} {
		for _, unused := range []bool{index%2 != 0} {
			t.Run(fmt.Sprintf("%d/%t", count, unused), func(t *testing.T) {
				source := `public Class Choices {public string Echo(string value)=>value;public bool Check(string name,bool value)=>value;public string Select(string raw,bool a,bool b,bool c,bool q,bool r,bool s,bool d,bool e){`
				previous := "raw"
				for i := 0; i < count; i++ {
					init := fmt.Sprintf(`Check("q%d",q) ? (Check("r%d",r) ? (Check("s%d",s) ? Echo(%s+"T") : Echo(%s+"M")) : Echo(%s+"N")) : Echo(%s+"F")`, i, i, i, previous, previous, previous, previous)
					if i%2 == 1 {
						init = fmt.Sprintf(`Echo(%s+"O")`, previous)
					}
					source += fmt.Sprintf("string l%d=%s;", i, init)
					if !unused || i < count-1 {
						previous = fmt.Sprintf("l%d", i)
					}
				}
				left, right := fmt.Sprintf(`Echo(%s+"Y")`, previous), fmt.Sprintf(`Echo(%s+"Z")`, previous)
				if arms&1 != 0 {
					left = fmt.Sprintf(`(Check("d",d) ? Echo(%s+"X") : Echo(%s+"Y"))`, previous, previous)
				}
				if arms&2 != 0 {
					right = fmt.Sprintf(`(Check("e",e) ? Echo(%s+"Z") : Echo(%s+"W"))`, previous, previous)
				}
				source += fmt.Sprintf(`return (Check("a",a) ? Check("b",b) : Check("c",c)) ? %s : %s;}}`, left, right)
				a, p := conditionalLocalTreeProgramVersion(t, PipeLangLanguageContractV1110, v990Wrap(t, source), []string{"Select"})
				aa, pp := conditionalLocalTreeProgramVersion(t, PipeLangLanguageContractV1110, v990Wrap(t, source), []string{"Select"})
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
				prepared := prepareConformanceProgram(t, p)
				body := *v990FirstLeaf(&f.Body)
				for body.Kind == coreir.ExprImmutableLocal {
					body = *body.ImmutableLocal.Return
				}
				if body.Kind != coreir.ExprConditional || body.Conditional.Condition.Kind != coreir.ExprConditional || body.Conditional.TerminalStatement || body.Conditional.Condition.Conditional.TerminalStatement {
					t.Fatal("selector representation lost")
				}
				var checks, orders strings.Builder
				for mask := 0; mask < 256; mask++ {
					flags := []bool{}
					args := []coreeval.Value{{Type: f.Parameters[0].Type, String: "raw"}}
					call := `PipeLangSelect("raw"`
					for i := 0; i < 8; i++ {
						flag := mask&(1<<i) != 0
						flags = append(flags, flag)
						args = append(args, coreeval.Value{Type: f.Parameters[i+1].Type, Bool: flag})
						call += fmt.Sprintf(",%t", flag)
					}
					call += ")"
					value := "raw"
					trace := []string{}
					for i := 0; i < count; i++ {
						suffix := "O"
						if i%2 == 0 {
							trace = append(trace, fmt.Sprintf("C:q%d", i))
							suffix = "F"
							if flags[3] {
								trace = append(trace, fmt.Sprintf("C:r%d", i))
								suffix = "N"
								if flags[4] {
									trace = append(trace, fmt.Sprintf("C:s%d", i))
									suffix = "M"
									if flags[5] {
										suffix = "T"
									}
								}
							}
						}
						next := value + suffix
						trace = append(trace, "E:"+next)
						if !unused || i < count-1 {
							value = next
						}
					}
					trace = append(trace, "C:a")
					selected := flags[2]
					if flags[0] {
						selected = flags[1]
						trace = append(trace, "C:b")
					} else {
						trace = append(trace, "C:c")
					}
					suffix := "Z"
					if selected {
						suffix = "Y"
						if arms&1 != 0 {
							trace = append(trace, "C:d")
							if flags[6] {
								suffix = "X"
							}
						}
					} else if arms&2 != 0 {
						trace = append(trace, "C:e")
						if !flags[7] {
							suffix = "W"
						}
					}
					value += suffix
					trace = append(trace, "E:"+value)
					got, err := prepared.Evaluate(f.Identity, args)
					if err != nil || !got.OK || got.Value.String != value {
						t.Fatal(mask, got, err, value)
					}
					fmt.Fprintf(&checks, "if got:=%s;got!=%q{t.Fatal(got)}\n", call, value)
					fmt.Fprintf(&orders, "v1100Trace=nil;%s;if !reflect.DeepEqual(v1100Trace,[]string{%s}){t.Fatal(v1100Trace)}\n", call, quotedStrings(trace))
				}
				compileAndRunGeneratedGoFiles(t, generated, []byte(fmt.Sprintf("package %s\nimport \"testing\"\nfunc TestValues(t *testing.T){%s}", gobackend.PackageName, checks.String())))
				observed := string(generated)
				for marker, probe := range map[string]string{"func PipeLangEcho(p0 string) string {": `v1100Trace=append(v1100Trace,"E:"+p0)`, "func PipeLangCheck(p0 string, p1 bool) bool {": `v1100Trace=append(v1100Trace,"C:"+p0)`} {
					if strings.Count(observed, marker) != 1 {
						t.Fatal("missing trace marker")
					}
					observed = strings.Replace(observed, marker, marker+"\n"+probe, 1)
				}
				compileAndRunGeneratedGoFiles(t, []byte(observed), []byte(fmt.Sprintf("package %s\nimport (\"testing\";\"reflect\")\nvar v1100Trace []string\nfunc TestOrder(t *testing.T){%s}", gobackend.PackageName, orders.String())))
			})
		}
	}
}

func TestV1110TerminalLeafSelectorValueArmsPriorCoreContracts(t *testing.T) {
	for _, source := range []string{terminalLeafSelectorValueArmsSeed, `public Class Choices {public string Select(string raw,bool a,bool b,bool c){return (a ? b : c) ? (b ? raw : "x") : raw;}}`, `public Class Choices {public string Select(string raw,bool a,bool b,bool c){return (a ? b : c) ? raw : (b ? raw : "x");}}`} {
		_, p := conditionalLocalTreeProgramVersion(t, PipeLangLanguageContractV1110, v990Wrap(t, source), []string{"Select"})
		for version := 1; version <= 110; version++ {
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
}

func TestV1110TerminalLeafSelectorValueArmsMalformedCore(t *testing.T) {
	clone := func(e *coreir.Expr) *coreir.Expr {
		b, _ := json.Marshal(e)
		var out coreir.Expr
		if err := json.Unmarshal(b, &out); err != nil {
			t.Fatal(err)
		}
		return &out
	}
	for _, mutation := range []string{"outer", "selector", "a", "b", "c", "x", "y", "a type", "b type", "c type", "x type", "result type", "nested a", "nested b", "nested c", "nested x", "nested y", "argument", "initializer", "terminal selector", "terminal leaf", "hidden a", "hidden x", "local", "init", "continuation", "position", "reference", "self", "identity", "version"} {
		t.Run(mutation, func(t *testing.T) {
			_, p := conditionalLocalTreeProgramVersion(t, PipeLangLanguageContractV1110, v990Wrap(t, terminalLeafSelectorValueArmsSeed), []string{"Select"})
			f := &p.Functions[len(p.Functions)-1]
			leaf := v990FirstLeaf(&f.Body)
			local := leaf.ImmutableLocal
			returned := local.Return
			outer := returned.Conditional
			selector := outer.Condition.Conditional
			switch mutation {
			case "outer":
				returned.Conditional = nil
			case "selector":
				outer.Condition.Conditional = nil
			case "a":
				selector.Condition = nil
			case "b":
				selector.WhenTrue = nil
			case "c":
				selector.WhenFalse = nil
			case "x":
				outer.WhenTrue = nil
			case "y":
				outer.WhenFalse = nil
			case "a type":
				selector.Condition = outer.WhenFalse
			case "b type":
				selector.WhenTrue = outer.WhenFalse
			case "c type":
				selector.WhenFalse = outer.WhenFalse
			case "x type":
				outer.WhenTrue = selector.Condition
			case "result type":
				returned.Type = outer.Condition.Type
			case "nested a":
				selector.Condition = clone(outer.Condition)
			case "nested b":
				selector.WhenTrue = clone(outer.Condition)
			case "nested c":
				selector.WhenFalse = clone(outer.Condition)
			case "nested x":
				outer.WhenTrue = clone(returned)
			case "nested y":
				outer.WhenFalse = clone(returned)
			case "argument":
				local.Return = &coreir.Expr{Kind: coreir.ExprTextTrim, Type: returned.Type, TextTrim: &coreir.TextTrim{Value: clone(returned)}}
			case "initializer":
				local.Initializer = clone(returned)
				local.Initializer.Conditional.WhenTrue = clone(outer.WhenFalse)
			case "terminal selector":
				selector.TerminalStatement = true
			case "terminal leaf":
				local.Return = &coreir.Expr{Kind: coreir.ExprConditional, Type: returned.Type, Conditional: &coreir.Conditional{TerminalStatement: true, Condition: clone(selector.Condition), WhenTrue: clone(returned), WhenFalse: clone(outer.WhenFalse)}}
			case "hidden a":
				hidden := clone(leaf)
				hidden.Type = selector.Condition.Type
				hidden.ImmutableLocal.Return = clone(selector.Condition)
				selector.Condition = hidden
			case "hidden x":
				outer.WhenTrue = clone(leaf)
			case "local":
				leaf.ImmutableLocal = nil
			case "init":
				local.Initializer = nil
			case "continuation":
				local.Return = nil
			case "position":
				local.Position = 999
			case "reference":
				ref := 999
				selector.Condition = &coreir.Expr{Kind: coreir.ExprReference, Type: selector.Condition.Type, Parameter: &ref}
			case "self":
				local.Initializer = clone(outer.WhenTrue)
			case "identity":
				p.CompilerContract = "unknown"
			case "version":
				p.LanguageContract = "v0.999.0"
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

func TestV1110TerminalLeafSelectorValueArmsComputedConditions(t *testing.T) {
	source := `public Class Choices {public bool Check(bool value)=>value;public string Select(string raw,bool a,bool b,bool c){string first=a ? (b ? (c ? trim(raw) : raw) : raw) : raw;bool selected=first!="";return (Check(a && selected) ? Check(b || first=="") : Check(!c)) ? (Check(b) ? first+"Y" : first+"Y") : (Check(c) ? raw+"Z" : raw+"Z");}}`
	_, p := conditionalLocalTreeProgramVersion(t, PipeLangLanguageContractV1110, v990Wrap(t, source), []string{"Select"})
	f := coreFunctionNamed(t, p, "Select")
	var checks strings.Builder
	for _, raw := range []string{"", " ", " text "} {
		for mask := 0; mask < 8; mask++ {
			a, b, c := mask&1 != 0, mask&2 != 0, mask&4 != 0
			first := raw
			if a && b && c {
				first = strings.TrimSpace(raw)
			}
			selected := !c
			if a && first != "" {
				selected = b || first == ""
			}
			want := raw + "Z"
			if selected {
				want = first + "Y"
			}
			args := []coreeval.Value{{Type: f.Parameters[0].Type, String: raw}}
			for i, flag := range []bool{a, b, c} {
				args = append(args, coreeval.Value{Type: f.Parameters[i+1].Type, Bool: flag})
			}
			got, err := coreeval.EvaluateProgram(p, f.Identity, args)
			if err != nil || !got.OK || got.Value.String != want {
				t.Fatal(got, err, want)
			}
			fmt.Fprintf(&checks, "if got:=PipeLangSelect(%q,%t,%t,%t);got!=%q{t.Fatal(got)}\n", raw, a, b, c, want)
		}
	}
	generated, err := gobackend.Generate(p)
	if err != nil {
		t.Fatal(err)
	}
	compileAndRunGeneratedGoFiles(t, generated, []byte(fmt.Sprintf("package %s\nimport \"testing\"\nfunc TestComputed(t *testing.T){%s}", gobackend.PackageName, checks.String())))
}

func TestV1110TerminalLeafSelectorValueArmsComputedCarriers(t *testing.T) {
	for _, unused := range []bool{false, true} {
		t.Run(fmt.Sprint(unused), func(t *testing.T) {
			local := ""
			if unused {
				local = `Result<int,ArithmeticError> unused=a ? (b ? (c ? Make(value) : Make(value)) : Make(value)) : Make(value);`
			}
			source := `public Class Choices {public Result<int,ArithmeticError> Make(int value)=>value+1;public Result<int,ArithmeticError> Select(int value,bool a,bool b,bool c){` + local + `return (a ? b : c) ? (a ? Make(value) : Make(value)) : (b ? Make(0) : Make(0));}}`
			_, p := conditionalLocalTreeProgramVersion(t, PipeLangLanguageContractV1110, v990Wrap(t, source), []string{"Select"})
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

func TestV1110TerminalLeafSelectorValueArmsMalformedResultArms(t *testing.T) {
	for _, side := range []bool{false, true} {
		for _, slot := range []int{0, 1, 2} {
			for _, mutation := range []string{"missing", "type", "nested", "terminal"} {
				t.Run(fmt.Sprintf("%t/%d/%s", side, slot, mutation), func(t *testing.T) {
					_, p := conditionalLocalTreeProgramVersion(t, PipeLangLanguageContractV1110, v990Wrap(t, terminalLeafSelectorValueArmsSeed), []string{"Select"})
					f := &p.Functions[len(p.Functions)-1]
					outer := v990FirstLeaf(&f.Body).ImmutableLocal.Return.Conditional
					arm := outer.WhenTrue
					if side {
						arm = outer.WhenFalse
					}
					choice := arm.Conditional
					target := &choice.Condition
					if slot == 1 {
						target = &choice.WhenTrue
					}
					if slot == 2 {
						target = &choice.WhenFalse
					}
					switch mutation {
					case "missing":
						*target = nil
					case "type":
						if slot == 0 {
							*target = choice.WhenTrue
						} else {
							*target = choice.Condition
						}
					case "nested":
						if slot == 0 {
							*target = outer.Condition
						} else {
							data, _ := json.Marshal(arm)
							var copy coreir.Expr
							if err := json.Unmarshal(data, &copy); err != nil {
								t.Fatal(err)
							}
							*target = &copy
						}
					case "terminal":
						choice.TerminalStatement = true
					}
					assertAdmissionRejected(t, p, "")
				})
			}
		}
	}
}

// Marking the outer node as a statement yields an already-supported terminal tree:
// a flat conditional test with ordinary ternary leaf returns. It is not malformed.

func TestV1110TerminalLeafSelectorValueArmsInheritance(t *testing.T) {
	for _, source := range []string{straightLineSelectorValueArmsSource, terminalCombinedSelectorArmsSource, terminalInnerSelectorArmsSource, terminalSelectorValueArmsSource, terminalBooleanSelectorTestsSource, conditionalBooleanSelectorsSource, straightLineBooleanSelectorInitializersSource, terminalBooleanSelectorInitializersSource, arrowBooleanSelectorsSource, v990Wrap(t, conditionalBooleanSelectorsSource), v940WrapBody(t, depthThreeStraightLineInitializersSource, terminalTrees(3)[25], "a"), depthThreeStraightLineInitializersSource, depthThreeArrowMethodsSource, depthThreeTerminalLeafReturnsSource, depthThreeStraightLineReturnsSource, nestedArrowMethodsSource, nestedTerminalInitializersSource, nestedStraightLineInitializersSource, nestedTerminalLeafReturnsSource, nestedStraightLineReturnsSource, terminalLeafConditionalReturnsSource, conditionalReturnCompositionSource, straightLineConditionalLocalsSource, finiteConditionalLocalsSource, twoConditionalLocalsSource, `public Class Choices {public string Select(string raw,bool pick)=>pick ? raw : trim(raw);}`} {
		var baseline [][]byte
		for _, contract := range []LanguageContract{PipeLangLanguageContractV1100, PipeLangLanguageContractV1110} {
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
			h.LanguageContract = coreir.LanguageContractV1100
			p.LanguageContract = coreir.LanguageContractV1100
			projection.LanguageContract = PipeLangLanguageContractV1100
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

func TestV1110TerminalLeafSelectorValueArmsSourceRejection(t *testing.T) {
	good := `(a ? b : c) ? (d ? raw : "x") : (e ? "y" : raw)`
	prefix := `public Class Choices {public string Select(string raw,bool a,bool b,bool c,bool d,bool e)`
	bodies := []string{
		`=>` + good + `;`,
		`{if(a){string local=` + good + `;return local;}else{return raw;}}`,
		`{if(a){return trim(` + good + `);}else{return raw;}}`,
		`{if(a){return (a ? (b ? c : d) : c) ? (d ? raw : "x") : raw;}else{return raw;}}`,
		`{if(a){return (a ? b : c) ? (d ? (e ? raw : "x") : raw) : raw;}else{return raw;}}`,
		`{if(a){return (raw ? b : c) ? (d ? raw : "x") : raw;}else{return raw;}}`,
		`{if(a){return (a ? raw : c) ? (d ? raw : "x") : raw;}else{return raw;}}`,
		`{if(a){return (a ? b : c) ? (raw ? raw : "x") : raw;}else{return raw;}}`,
		`{if(a){return (a ? b : c) ? (d ? true : raw) : raw;}else{return raw;}}`,
		`{if(a){return (a ? b : c) ? (d ? missing : raw) : raw;}else{return raw;}}`,
		`{if(a){bool local=local;return ` + good + `;}else{return raw;}}`,
		`{if(a){bool local=later;bool later=b;return ` + good + `;}else{return raw;}}`,
		`{if(a){bool b=c;return ` + good + `;}else{return raw;}}`,
		`{if(a){bool local=b;bool local=c;return ` + good + `;}else{return raw;}}`,
		`{if(a){string local=raw;return ` + good + `;}else{return local;}}`,
		`{if(a){return (a ? b : c) ? (d ? propagate(raw) : raw) : raw;}else{return raw;}}`,
		`{if(a){return (a ? b : c) ? (d ? match(raw){some(v)=>v,none=>raw} : raw) : raw;}else{return raw;}}`,
		`{if(a){if(b){if(c){if(d){return ` + good + `;}else{return raw;}}else{return raw;}}else{return raw;}}else{return raw;}}`,
	}
	reject := func(source string, contract LanguageContract) {
		t.Helper()
		input := semanticTestModuleSet("compiler.selfhosting", []ModuleInput{testModule("compiler.selfhosting", "selector.pipe", source)}, nil)
		input.LanguageContract = contract
		analysis := AnalyzeSemanticModuleSet(input)
		if analysis.Error() == nil {
			t.Fatal("admitted", contract, source)
		}
		for _, d := range analysis.Diagnostics {
			if !d.Primary.IsValid() || d.Primary.File != "selector.pipe" || d.Primary.End > len(source) {
				t.Fatal("invalid diagnostic", d)
			}
		}
	}
	for _, body := range bodies {
		reject(prefix+body+`}`, PipeLangLanguageContractV1110)
	}
	for _, visibility := range []string{"private ", "protected "} {
		reject(strings.Replace(prefix, "public string", visibility+"string", 1)+`{if(a){return `+good+`;}else{return raw;}}}`, PipeLangLanguageContractV1110)
	}
	for _, expr := range []string{`(a ? b : c) ? (d ? raw : "x") : raw`, `(a ? b : c) ? raw : (e ? "y" : raw)`, good} {
		for version := 1; version <= 110; version++ {
			reject(prefix+`{if(a){return `+expr+`;}else{return raw;}}}`, LanguageContract(fmt.Sprintf("v0.%d.0", version)))
		}
	}
}

func TestV1110TerminalLeafSelectorValueArmsInheritedTestsAndInitializers(t *testing.T) {
	tests := []string{`a`, `a ? b : c`, `a ? (b ? c : d) : e`, `a ? (b ? (c ? d : e) : b) : c`, `(a ? b : c) ? d : e`, `(a ? b : c) ? (d ? e : a) : (e ? a : b)`, `(a ? (b ? c : d) : (c ? d : e)) ? a : b`, `(a ? (b ? c : d) : (c ? d : e)) ? (a ? b : c) : (d ? e : a)`}
	inits := []string{`raw`, `a ? (b ? (c ? raw : "x") : raw) : raw`, `(a ? b : c) ? raw : "x"`}
	for _, test := range tests {
		for _, init := range inits {
			source := `public Class Choices {public string Select(string raw,bool a,bool b,bool c,bool d,bool e){string root=` + init + `;if(` + test + `){string local=` + init + `;return (a ? b : c) ? (d ? local : root) : (e ? raw : root);}else{string local=raw;return local;}}}`
			_, p := conditionalLocalTreeProgramVersion(t, PipeLangLanguageContractV1110, source, []string{"Select"})
			if err := coreir.ValidateProgram(p); err != nil {
				t.Fatal(err)
			}
		}
	}
}

func TestV1110TerminalLeafSelectorValueArmsInternalCoreBoundary(t *testing.T) {
	for _, slot := range []string{"a", "b", "c", "x", "y", "d", "e", "xx", "xy", "yx", "yy"} {
		t.Run(slot, func(t *testing.T) {
			_, p := conditionalLocalTreeProgramVersion(t, PipeLangLanguageContractV1110, v990Wrap(t, straightLineSelectorValueArmsSource), []string{"Select"})
			f := &p.Functions[len(p.Functions)-1]
			outer := v990FirstLeaf(&f.Body).ImmutableLocal.Return.Conditional
			selector := outer.Condition.Conditional
			target := &selector.Condition
			switch slot {
			case "b":
				target = &selector.WhenTrue
			case "c":
				target = &selector.WhenFalse
			case "x":
				target = &outer.WhenTrue
			case "y":
				target = &outer.WhenFalse
			case "d":
				target = &outer.WhenTrue.Conditional.Condition
			case "e":
				target = &outer.WhenFalse.Conditional.Condition
			case "xx":
				target = &outer.WhenTrue.Conditional.WhenTrue
			case "xy":
				target = &outer.WhenTrue.Conditional.WhenFalse
			case "yx":
				target = &outer.WhenFalse.Conditional.WhenTrue
			case "yy":
				target = &outer.WhenFalse.Conditional.WhenFalse
			}
			value := *target
			position := v990FirstLeaf(&f.Body).ImmutableLocal.Position + 1
			*target = &coreir.Expr{Kind: coreir.ExprImmutableLocal, Type: value.Type, ImmutableLocal: &coreir.ImmutableLocal{Name: "hidden", Position: position, Type: value.Type, Initializer: value, Return: &coreir.Expr{Kind: coreir.ExprReference, Type: value.Type, Parameter: &position}}}
			if err := coreir.ValidateFunction(*f); err != nil {
				t.Fatalf("generic internal Core narrowed: %v", err)
			}
			assertAdmissionRejected(t, p, "")
		})
	}
}
