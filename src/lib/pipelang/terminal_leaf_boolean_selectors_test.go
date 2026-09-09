package pipelang

import (
	"bytes"
	"dockpipe/src/lib/pipelang/coreeval"
	"dockpipe/src/lib/pipelang/coreir"
	"dockpipe/src/lib/pipelang/gobackend"
	"encoding/json"
	"fmt"
	"reflect"
	"regexp"
	"strings"
	"testing"
)

func TestV990TerminalLeafBooleanSelectorsAdmission(t *testing.T) {
	for _, source := range []string{conditionalBooleanSelectorsSource, `public Class Choices {public string Select(string raw,bool a,bool b,bool c){return (a ? b : c) ? trim(raw) : raw;}}`} {
		_, p := conditionalLocalTreeProgramVersion(t, PipeLangLanguageContractV990, v990Wrap(t, source), []string{"Select"})
		if err := coreir.ValidateProgram(p); err != nil {
			t.Fatal(err)
		}
	}
}
func TestV990TerminalLeafBooleanSelectorsInheritance(t *testing.T) {
	for _, source := range []string{conditionalBooleanSelectorsSource, v940WrapBody(t, depthThreeStraightLineInitializersSource, terminalTrees(3)[25], "a"), depthThreeStraightLineInitializersSource, depthThreeArrowMethodsSource, depthThreeTerminalLeafReturnsSource, depthThreeStraightLineReturnsSource, nestedArrowMethodsSource, nestedTerminalInitializersSource, nestedStraightLineInitializersSource, nestedTerminalLeafReturnsSource, nestedStraightLineReturnsSource, terminalLeafConditionalReturnsSource, conditionalReturnCompositionSource, straightLineConditionalLocalsSource, finiteConditionalLocalsSource, twoConditionalLocalsSource, `public Class Choices {public string Select(string raw,bool pick)=>pick ? raw : trim(raw);}`} {
		var baseline [][]byte
		for _, contract := range []LanguageContract{PipeLangLanguageContractV980, PipeLangLanguageContractV990} {
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
			h.LanguageContract = coreir.LanguageContractV980
			p.LanguageContract = coreir.LanguageContractV980
			projection.LanguageContract = PipeLangLanguageContractV980
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

func TestV990TerminalLeafBooleanSelectorsTypes(t *testing.T) {
	for _, zero := range []bool{false, true} {
		t.Run(fmt.Sprint(zero), func(t *testing.T) { testConditionalLocalsTypeMatrix(t, PipeLangLanguageContractV990, zero) })
	}
}
func TestV990TerminalLeafBooleanSelectorsCarriers(t *testing.T) {
	for _, zero := range []bool{false, true} {
		t.Run(fmt.Sprint(zero), func(t *testing.T) { testConditionalLocalsCarrierAndHostValues(t, PipeLangLanguageContractV990, zero) })
	}
}

func TestV990TerminalLeafBooleanSelectorsLayouts(t *testing.T) {
	for _, count := range []int{0, 1, 4} {
		for _, unused := range []bool{false, true} {
			t.Run(fmt.Sprintf("%d/%t", count, unused), func(t *testing.T) {
				source := `public Class Choices {public string Echo(string value)=>value;public bool Check(string name,bool value)=>value;public string Select(string raw,bool a,bool b,bool c,bool q,bool r,bool s){`
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
				source += fmt.Sprintf(`return (Check("a",a) ? Check("b",b) : Check("c",c)) ? Echo(%s+"Y") : Echo(%s+"Z");}}`, previous, previous)
				a, p := conditionalLocalTreeProgramVersion(t, PipeLangLanguageContractV990, v990Wrap(t, source), []string{"Select"})
				aa, pp := conditionalLocalTreeProgramVersion(t, PipeLangLanguageContractV990, v990Wrap(t, source), []string{"Select"})
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
				for mask := 0; mask < 64; mask++ {
					flags := []bool{}
					args := []coreeval.Value{{Type: f.Parameters[0].Type, String: "raw"}}
					call := `PipeLangSelect("raw"`
					for i := 0; i < 6; i++ {
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
					if selected {
						value += "Y"
					} else {
						value += "Z"
					}
					trace = append(trace, "E:"+value)
					got, err := prepared.Evaluate(f.Identity, args)
					if err != nil || !got.OK || got.Value.String != value {
						t.Fatal(mask, got, err, value)
					}
					fmt.Fprintf(&checks, "if got:=%s;got!=%q{t.Fatal(got)}\n", call, value)
					fmt.Fprintf(&orders, "v990Trace=nil;%s;if !reflect.DeepEqual(v990Trace,[]string{%s}){t.Fatal(v990Trace)}\n", call, quotedStrings(trace))
				}
				compileAndRunGeneratedGoFiles(t, generated, []byte(fmt.Sprintf("package %s\nimport \"testing\"\nfunc TestValues(t *testing.T){%s}", gobackend.PackageName, checks.String())))
				observed := string(generated)
				for marker, probe := range map[string]string{"func PipeLangEcho(p0 string) string {": `v990Trace=append(v990Trace,"E:"+p0)`, "func PipeLangCheck(p0 string, p1 bool) bool {": `v990Trace=append(v990Trace,"C:"+p0)`} {
					if strings.Count(observed, marker) != 1 {
						t.Fatal("missing trace marker")
					}
					observed = strings.Replace(observed, marker, marker+"\n"+probe, 1)
				}
				compileAndRunGeneratedGoFiles(t, []byte(observed), []byte(fmt.Sprintf("package %s\nimport (\"testing\";\"reflect\")\nvar v990Trace []string\nfunc TestOrder(t *testing.T){%s}", gobackend.PackageName, orders.String())))
			})
		}
	}
}

func TestV990TerminalLeafBooleanSelectorsSourceRejection(t *testing.T) {
	prefix := `public Class Choices {public string Select(string raw,bool a,bool b,bool c) `
	for _, body := range []string{
		`=> (a ? b : c) ? raw : "x";`,
		`{string local=(a ? b : c) ? raw : "x";return local;}`,
		`{if(a){return (a ? b : c) ? (a ? raw : "x") : raw;}else{return raw;}}`,
		`{if(a ? b : c){return raw;}else{return "x";}}`,
		`{return trim((a ? b : c) ? raw : "x");}`,
		`{return ((a ? b : c) ? b : c) ? raw : "x";}`,
		`{return (a ? (b ? a : c) : c) ? raw : "x";}`,
		`{return (a ? b : (c ? a : b)) ? raw : "x";}`,
		`{return (a ? b : c) ? (a ? raw : "x") : raw;}`,
		`{return (a ? b : c) ? raw : (a ? raw : "x");}`,
		`{return ((a ? b : c) && a) ? raw : "x";}`,
		`{return (a ? raw : c) ? raw : "x";}`,
		`{return (raw ? b : c) ? raw : "x";}`,
		`{return (a ? b : raw) ? raw : "x";}`,
		`{return (a ? b : c) ? true : raw;}`,
		`{return (a ? b : c) ? missing : raw;}`,
		`{bool local=local;return (a ? b : c) ? raw : "x";}`,
		`{bool local=later;bool later=a;return (local ? b : c) ? raw : "x";}`,
		`{bool a=b;return (a ? b : c) ? raw : "x";}`,
		`{bool local=a;bool local=b;return (local ? b : c) ? raw : "x";}`,
		`{return (a ? b : c) ? propagate(raw) : raw;}`,
		`{return (a ? b : c) ? match(raw){some(v)=>v,none=>raw} : raw;}`,
		`{string local=a ? (b ? (c ? (a ? raw : "x") : raw) : raw) : raw;return (a ? b : c) ? local : raw;}`,
	} {
		input := semanticTestModuleSet("compiler.selfhosting", []ModuleInput{testModule("compiler.selfhosting", "selector.pipe", prefix+body+`}`)}, nil)
		input.LanguageContract = PipeLangLanguageContractV990
		analysis := AnalyzeSemanticModuleSet(input)
		if analysis.Error() == nil {
			t.Fatal("admitted", body)
		}
		for _, d := range analysis.Diagnostics {
			if !d.Primary.IsValid() || d.Primary.File != "selector.pipe" || d.Primary.End > len(prefix+body+`}`) {
				t.Fatal("invalid diagnostic", d)
			}
		}
	}
	for version := 1; version <= 98; version++ {
		input := semanticTestModuleSet("compiler.selfhosting", []ModuleInput{testModule("compiler.selfhosting", "selector.pipe", v990Wrap(t, prefix+`{return (a ? b : c) ? raw : "x";}}`))}, nil)
		input.LanguageContract = LanguageContract(fmt.Sprintf("v0.%d.0", version))
		if AnalyzeSemanticModuleSet(input).Error() == nil {
			t.Fatal("older source admitted", version)
		}
	}
}

func TestV990TerminalLeafBooleanSelectorsPriorCoreContracts(t *testing.T) {
	for _, source := range []string{conditionalBooleanSelectorsSource, `public Class Choices {public string Select(string raw,bool a,bool b,bool c){return (a ? b : c) ? raw : "x";}}`} {
		_, p := conditionalLocalTreeProgramVersion(t, PipeLangLanguageContractV990, v990Wrap(t, source), []string{"Select"})
		for version := 1; version <= 98; version++ {
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

func TestV990TerminalLeafBooleanSelectorsMalformedCore(t *testing.T) {
	clone := func(e *coreir.Expr) *coreir.Expr {
		b, _ := json.Marshal(e)
		var out coreir.Expr
		if err := json.Unmarshal(b, &out); err != nil {
			t.Fatal(err)
		}
		return &out
	}
	for _, mutation := range []string{"outer", "selector", "a", "b", "c", "x", "y", "a type", "b type", "c type", "x type", "result type", "nested a", "nested b", "nested c", "nested x", "nested y", "argument", "initializer", "terminal outer", "terminal selector", "hidden a", "hidden x", "local", "init", "continuation", "position", "reference", "self", "identity", "version", "statement condition", "statement depth"} {
		t.Run(mutation, func(t *testing.T) {
			_, p := conditionalLocalTreeProgramVersion(t, PipeLangLanguageContractV990, v990Wrap(t, conditionalBooleanSelectorsSource), []string{"Select"})
			f := &p.Functions[len(p.Functions)-1]
			body := v990FirstLeaf(&f.Body)
			local := body.ImmutableLocal
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
			case "terminal outer":
				outer.TerminalStatement = true
			case "terminal selector":
				selector.TerminalStatement = true
			case "hidden a":
				hidden := clone(body)
				hidden.Type = selector.Condition.Type
				hidden.ImmutableLocal.Return = clone(selector.Condition)
				selector.Condition = hidden
			case "hidden x":
				outer.WhenTrue = clone(body)
			case "local":
				body.ImmutableLocal = nil
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
			case "statement condition":
				f.Body.Conditional.Condition = clone(outer.Condition)
			case "statement depth":
				old := clone(&f.Body)
				f.Body = coreir.Expr{Kind: coreir.ExprConditional, Type: old.Type, Conditional: &coreir.Conditional{TerminalStatement: true, Condition: clone(selector.Condition), WhenTrue: old, WhenFalse: clone(old)}}
			case "identity":
				p.CompilerContract = "unknown"
			case "version":
				p.LanguageContract = "v0.102.0"
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

func TestV990TerminalLeafBooleanSelectorsComputedConditions(t *testing.T) {
	source := `public Class Choices {public bool Check(bool value)=>value;public string Select(string raw,bool a,bool b,bool c){string first=a ? (b ? (c ? trim(raw) : raw) : raw) : raw;bool selected=first!="";return (Check(a && selected) ? Check(b || first=="") : Check(!c)) ? first+"Y" : raw+"Z";}}`
	_, p := conditionalLocalTreeProgramVersion(t, PipeLangLanguageContractV990, v990Wrap(t, source), []string{"Select"})
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

func TestV990TerminalLeafBooleanSelectorsComputedCarriers(t *testing.T) {
	for _, unused := range []bool{false, true} {
		t.Run(fmt.Sprint(unused), func(t *testing.T) {
			local := ""
			if unused {
				local = `Result<int,ArithmeticError> unused=a ? (b ? (c ? Make(value) : Make(value)) : Make(value)) : Make(value);`
			}
			source := `public Class Choices {public Result<int,ArithmeticError> Make(int value)=>value+1;public Result<int,ArithmeticError> Select(int value,bool a,bool b,bool c){` + local + `return (a ? b : c) ? Make(value) : Make(0);}}`
			_, p := conditionalLocalTreeProgramVersion(t, PipeLangLanguageContractV990, v990Wrap(t, source), []string{"Select"})
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

func TestV990TerminalLeafBooleanSelectorsInternalCoreBoundary(t *testing.T) {
	for _, slot := range []string{"a", "b", "c", "x", "y"} {
		t.Run(slot, func(t *testing.T) {
			_, p := conditionalLocalTreeProgramVersion(t, PipeLangLanguageContractV990, v990Wrap(t, conditionalBooleanSelectorsSource), []string{"Select"})
			f := &p.Functions[len(p.Functions)-1]
			body := v990FirstLeaf(&f.Body)
			outer := body.ImmutableLocal.Return.Conditional
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
			}
			value := *target
			position := body.ImmutableLocal.Position + 1
			*target = &coreir.Expr{Kind: coreir.ExprImmutableLocal, Type: value.Type, ImmutableLocal: &coreir.ImmutableLocal{Name: "hidden", Position: position, Type: value.Type, Initializer: value, Return: &coreir.Expr{Kind: coreir.ExprReference, Type: value.Type, Parameter: &position}}}
			if err := coreir.ValidateFunction(*f); err != nil {
				t.Fatalf("generic internal Core narrowed: %v", err)
			}
			assertAdmissionRejected(t, p, "")
		})
	}
}

func v990Wrap(t *testing.T, source string) string {
	t.Helper()
	signature := source[strings.Index(source, " Select("):]
	condition := regexp.MustCompile(`bool ([a-zA-Z_][a-zA-Z0-9_]*)`).FindStringSubmatch(signature)[1]
	return v940WrapBody(t, source, terminalTrees(3)[25], condition)
}
func v990FirstLeaf(body *coreir.Expr) *coreir.Expr {
	for body.Kind == coreir.ExprConditional && body.Conditional.TerminalStatement {
		body = body.Conditional.WhenTrue
	}
	return body
}

func TestV990TerminalLeafBooleanSelectorsLexicalRefusal(t *testing.T) {
	for _, body := range []string{
		`string root=raw;if(a){string root=raw;return (a ? b : c) ? root : raw;}else{return raw;}`,
		`if(a){string branch=raw;return (a ? b : c) ? branch : raw;}else{return (a ? b : c) ? branch : raw;}`,
		`if(a){string local=local;return (a ? b : c) ? local : raw;}else{return raw;}`,
		`if(a){string local=later;string later=raw;return (a ? b : c) ? local : raw;}else{return raw;}`,
		`if(a){string local=raw;string local=raw;return (a ? b : c) ? local : raw;}else{return raw;}`,
		`if(a){string local=(a ? b : c) ? raw : "x";return local;}else{return raw;}`,
		`if(a){if(a ? b : c){return raw;}else{return raw;}}else{return raw;}`,
		`if(a){return trim((a ? b : c) ? raw : "x");}else{return raw;}`,
		`if(a){if(b){if(c){if(a){return (a ? b : c) ? raw : "x";}else{return raw;}}else{return raw;}}else{return raw;}}else{return raw;}`,
	} {
		source := `public Class Choices {public string Select(string raw,bool a,bool b,bool c){` + body + `}}`
		input := semanticTestModuleSet("compiler.selfhosting", []ModuleInput{testModule("compiler.selfhosting", "scope.pipe", source)}, nil)
		input.LanguageContract = PipeLangLanguageContractV990
		if AnalyzeSemanticModuleSet(input).Error() == nil {
			t.Fatal("admitted", body)
		}
	}
}
