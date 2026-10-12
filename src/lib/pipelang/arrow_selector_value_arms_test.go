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

const arrowSelectorValueArmsSource = `public Class Choices {public string Select(string raw,bool a,bool b,bool c,bool d,bool e)=>(a ? b : c) ? (d ? raw : "x") : (e ? "y" : raw);}`

// Move only the complete Select return into an arrow. Caller locals retain source order;
// their values are explicit helper parameters. Fail if the source shape differs.
func v1120Arrow(t *testing.T, source string) string {
	t.Helper()
	pattern := regexp.MustCompile(`public ([^{};]+) Select\(([^()]*)\)\{([^{}]*?)return ([^;{}]+);\}`)
	matches := pattern.FindAllStringSubmatchIndex(source, -1)
	if len(matches) != 1 {
		t.Fatal("expected one Select block")
	}
	m := matches[0]
	typ, params, locals, expr := source[m[2]:m[3]], source[m[4]:m[5]], source[m[6]:m[7]], source[m[8]:m[9]]
	replacement := "public " + typ + " Select(" + params + ")=>" + expr + ";"
	if strings.TrimSpace(locals) != "" {
		arguments := []string{}
		for _, param := range strings.Split(params, ",") {
			fields := strings.Fields(param)
			arguments = append(arguments, fields[len(fields)-1])
		}
		helperParams := params
		for _, decl := range strings.Split(locals, ";") {
			if strings.TrimSpace(decl) == "" {
				continue
			}
			binding := strings.TrimSpace(strings.SplitN(decl, "=", 2)[0])
			fields := strings.Fields(binding)
			helperParams += "," + binding
			arguments = append(arguments, fields[len(fields)-1])
		}
		replacement = "public " + typ + " ArrowChoice(" + helperParams + ")=>" + expr + ";public " + typ + " Select(" + params + "){" + locals + "return ArrowChoice(" + strings.Join(arguments, ",") + ");}"
	}
	return source[:m[0]] + replacement + source[m[1]:]
}
func TestV1120ArrowSelectorValueArmsAdmission(t *testing.T) {
	for _, visibility := range []string{"public ", ""} {
		for _, expr := range []string{`(a ? b : c) ? (d ? raw : "x") : raw`, `(a ? b : c) ? raw : (e ? "y" : raw)`, `(a ? b : c) ? (d ? raw : "x") : (e ? "y" : raw)`} {
			source := `public Class Choices {` + visibility + `string Select(string raw,bool a,bool b,bool c,bool d,bool e)=>` + expr + `;}`
			_, p := conditionalLocalTreeProgramVersion(t, PipeLangLanguageContractV1120, source, []string{"Select"})
			if err := coreir.ValidateProgram(p); err != nil {
				t.Fatal(err)
			}
		}
	}
}

func TestV1120ArrowSelectorValueArmsInheritance(t *testing.T) {
	for _, source := range []string{v990Wrap(t, terminalLeafSelectorValueArmsSeed), straightLineSelectorValueArmsSource, terminalCombinedSelectorArmsSource, terminalInnerSelectorArmsSource, terminalSelectorValueArmsSource, terminalBooleanSelectorTestsSource, conditionalBooleanSelectorsSource, straightLineBooleanSelectorInitializersSource, terminalBooleanSelectorInitializersSource, arrowBooleanSelectorsSource, v990Wrap(t, conditionalBooleanSelectorsSource), v940WrapBody(t, depthThreeStraightLineInitializersSource, terminalTrees(3)[25], "a"), depthThreeStraightLineInitializersSource, depthThreeArrowMethodsSource, depthThreeTerminalLeafReturnsSource, depthThreeStraightLineReturnsSource, nestedArrowMethodsSource, nestedTerminalInitializersSource, nestedStraightLineInitializersSource, nestedTerminalLeafReturnsSource, nestedStraightLineReturnsSource, terminalLeafConditionalReturnsSource, conditionalReturnCompositionSource, straightLineConditionalLocalsSource, finiteConditionalLocalsSource, twoConditionalLocalsSource, `public Class Choices {public string Select(string raw,bool pick)=>pick ? raw : trim(raw);}`} {
		var baseline [][]byte
		for _, contract := range []LanguageContract{PipeLangLanguageContractV1110, PipeLangLanguageContractV1120} {
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
			h.LanguageContract = coreir.LanguageContractV1110
			p.LanguageContract = coreir.LanguageContractV1110
			projection.LanguageContract = PipeLangLanguageContractV1110
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

func TestV1120ArrowSelectorValueArmsTypes(t *testing.T) {
	for i := 0; i < 3; i++ {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			arms := i + 1
			testConditionalLocalsTypeMatrix(t, PipeLangLanguageContractV1120, true, arms&1 != 0, arms&2 != 0)
		})
	}
}

func TestV1120ArrowSelectorValueArmsCarriers(t *testing.T) {
	for i := 0; i < 3; i++ {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			arms := i + 1
			testConditionalLocalsCarrierAndHostValues(t, PipeLangLanguageContractV1120, true, arms&1 != 0, arms&2 != 0)
		})
	}
}

func TestV1120ArrowSelectorValueArmsLayouts(t *testing.T) {
	for i := 0; i < 18; i++ {
		t.Run(fmt.Sprint(i), func(t *testing.T) { v1120Layouts(t, i) })
	}
}

func v1120Layouts(t *testing.T, index int) {
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
				a, p := conditionalLocalTreeProgramVersion(t, PipeLangLanguageContractV1120, v1120Arrow(t, source), []string{"Select"})
				aa, pp := conditionalLocalTreeProgramVersion(t, PipeLangLanguageContractV1120, v1120Arrow(t, source), []string{"Select"})
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
				body := f.Body
				if count > 0 {
					body = coreFunctionNamed(t, p, "ArrowChoice").Body
				}
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
					fmt.Fprintf(&orders, "v1120Trace=nil;%s;if !reflect.DeepEqual(v1120Trace,[]string{%s}){t.Fatal(v1120Trace)}\n", call, quotedStrings(trace))
				}
				compileAndRunGeneratedGoFiles(t, generated, []byte(fmt.Sprintf("package %s\nimport \"testing\"\nfunc TestValues(t *testing.T){%s}", gobackend.PackageName, checks.String())))
				observed := string(generated)
				for marker, probe := range map[string]string{"func PipeLangEcho(p0 string) string {": `v1120Trace=append(v1120Trace,"E:"+p0)`, "func PipeLangCheck(p0 string, p1 bool) bool {": `v1120Trace=append(v1120Trace,"C:"+p0)`} {
					if strings.Count(observed, marker) != 1 {
						t.Fatal("missing trace marker")
					}
					observed = strings.Replace(observed, marker, marker+"\n"+probe, 1)
				}
				compileAndRunGeneratedGoFiles(t, []byte(observed), []byte(fmt.Sprintf("package %s\nimport (\"testing\";\"reflect\")\nvar v1120Trace []string\nfunc TestOrder(t *testing.T){%s}", gobackend.PackageName, orders.String())))
			})
		}
	}
}

func TestV1120ArrowSelectorValueArmsComputedConditions(t *testing.T) {
	source := `public Class Choices {public bool Check(bool value)=>value;public string Select(string raw,bool a,bool b,bool c){string first=a ? (b ? (c ? trim(raw) : raw) : raw) : raw;bool selected=first!="";return (Check(a && selected) ? Check(b || first=="") : Check(!c)) ? (Check(b) ? first+"Y" : first+"Y") : (Check(c) ? raw+"Z" : raw+"Z");}}`
	_, p := conditionalLocalTreeProgramVersion(t, PipeLangLanguageContractV1120, v1120Arrow(t, source), []string{"Select"})
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

func TestV1120ArrowSelectorValueArmsComputedCarriers(t *testing.T) {
	for _, unused := range []bool{false, true} {
		t.Run(fmt.Sprint(unused), func(t *testing.T) {
			local := ""
			if unused {
				local = `Result<int,ArithmeticError> unused=a ? (b ? (c ? Make(value) : Make(value)) : Make(value)) : Make(value);`
			}
			source := `public Class Choices {public Result<int,ArithmeticError> Make(int value)=>value+1;public Result<int,ArithmeticError> Select(int value,bool a,bool b,bool c){` + local + `return (a ? b : c) ? (a ? Make(value) : Make(value)) : (b ? Make(0) : Make(0));}}`
			_, p := conditionalLocalTreeProgramVersion(t, PipeLangLanguageContractV1120, v1120Arrow(t, source), []string{"Select"})
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

func TestV1120ArrowSelectorValueArmsMalformedResultArms(t *testing.T) {
	for _, side := range []bool{false, true} {
		for _, slot := range []int{0, 1, 2} {
			for _, mutation := range []string{"missing", "type", "nested", "terminal"} {
				t.Run(fmt.Sprintf("%t/%d/%s", side, slot, mutation), func(t *testing.T) {
					_, p := conditionalLocalTreeProgramVersion(t, PipeLangLanguageContractV1120, arrowSelectorValueArmsSource, []string{"Select"})
					f := &p.Functions[len(p.Functions)-1]
					outer := f.Body.Conditional
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

func TestV1120ArrowSelectorValueArmsInternalCoreBoundary(t *testing.T) {
	for _, slot := range []string{"a", "b", "c", "x", "y", "d", "e", "xx", "xy", "yx", "yy"} {
		t.Run(slot, func(t *testing.T) {
			_, p := conditionalLocalTreeProgramVersion(t, PipeLangLanguageContractV1120, arrowSelectorValueArmsSource, []string{"Select"})
			f := &p.Functions[len(p.Functions)-1]
			outer := f.Body.Conditional
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
			position := len(f.Parameters)
			*target = &coreir.Expr{Kind: coreir.ExprImmutableLocal, Type: value.Type, ImmutableLocal: &coreir.ImmutableLocal{Name: "hidden", Position: position, Type: value.Type, Initializer: value, Return: &coreir.Expr{Kind: coreir.ExprReference, Type: value.Type, Parameter: &position}}}
			if err := coreir.ValidateFunction(*f); err != nil {
				t.Fatalf("generic internal Core narrowed: %v", err)
			}
			assertAdmissionRejected(t, p, "")
		})
	}
}

func TestV1120ArrowSelectorValueArmsEquivalence(t *testing.T) {
	for form, expr := range []string{`(a ? b : c) ? (b ? trim(raw) : raw) : raw`, `(a ? b : c) ? raw : (c ? trim(raw) : raw)`, `(a ? b : c) ? (b ? trim(raw) : raw) : (c ? raw : trim(raw))`, `(Check(a && raw!="") ? Check(b || raw=="") : Check(!c)) ? (Check(b) ? trim(raw) : raw) : (Check(c) ? raw : trim(raw))`} {
		head := `public Class Choices {public bool Check(bool value)=>value;public string Select(string raw,bool a,bool b,bool c)`
		arrow, block := head+"=>"+expr+";}", head+"{return "+expr+";}}"
		a, p := conditionalLocalTreeProgramVersion(t, PipeLangLanguageContractV1120, arrow, []string{"Select"})
		b, q := conditionalLocalTreeProgramVersion(t, PipeLangLanguageContractV1120, block, []string{"Select"})
		if !reflect.DeepEqual(p, q) {
			t.Fatal("arrow/block Core differs")
		}
		h, err := LowerSemanticMethodToHIR(a, semanticMethodNamed(t, a, "Select").Identity)
		if err != nil {
			t.Fatal(err)
		}
		hh, err := LowerSemanticMethodToHIR(b, semanticMethodNamed(t, b, "Select").Identity)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(v920WithoutSpans(t, h), v920WithoutSpans(t, hh)) {
			t.Fatal("arrow/block HIR differs")
		}
		x, err := BuildSemanticProjection(a)
		if err != nil {
			t.Fatal(err)
		}
		y, err := BuildSemanticProjection(b)
		if err != nil {
			t.Fatal(err)
		}
		for i, projection := range []*SemanticProjection{x, y} {
			source := arrow
			if i == 1 {
				source = block
			}
			if len(projection.Modules) != 1 || projection.Modules[0].SourceSHA256 != ModuleSourceSHA256([]SourceInput{{Path: "conditional-local.pipe", Data: []byte(source)}}) {
				t.Fatal("source fingerprint differs")
			}
		}
		if !reflect.DeepEqual(v1000SemanticWithoutSource(t, x), v1000SemanticWithoutSource(t, y)) {
			t.Fatal("arrow/block semantic differs")
		}
		generated, err := gobackend.Generate(p)
		if err != nil {
			t.Fatal(err)
		}
		other, err := gobackend.Generate(q)
		if err != nil || !bytes.Equal(generated, other) {
			t.Fatal("arrow/block Go differs", err)
		}
		f := coreFunctionNamed(t, p, "Select")
		var checks strings.Builder
		for _, raw := range []string{"", " ", " text "} {
			for mask := 0; mask < 8; mask++ {
				a, b, c := mask&1 != 0, mask&2 != 0, mask&4 != 0
				selected := c
				if a {
					selected = b
				}
				if form == 3 {
					selected = !c
					if a && raw != "" {
						selected = b || raw == ""
					}
				}
				trimSelected := selected && b
				if form == 1 {
					trimSelected = !selected && c
				}
				if form >= 2 {
					trimSelected = (selected && b) || (!selected && !c)
				}
				want := raw
				if trimSelected {
					want = strings.TrimSpace(raw)
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
		compileAndRunGeneratedGoFiles(t, generated, []byte(fmt.Sprintf("package %s\nimport \"testing\"\nfunc TestComputed(t *testing.T){%s}", gobackend.PackageName, checks.String())))
	}
}

func TestV1120ArrowSelectorValueArmsSourceRejection(t *testing.T) {
	prefix := `public Class Choices {public string Select(string raw,bool a,bool b,bool c,bool d,bool e) `
	good := `(a ? b : c) ? (d ? raw : "x") : (e ? "y" : raw)`
	for _, body := range []string{
		`{if(a){string local=` + good + `;return local;}else{return raw;}}`,
		`{string local=` + good + `;return local;}`,
		`{return trim(` + good + `);}`,
		`{return ((a ? b : c) ? b : c) ? (d ? raw : "x") : raw;}`,
		`{return (a ? (b ? a : c) : c) ? (d ? raw : "x") : raw;}`,
		`{return (a ? b : (c ? a : b)) ? (d ? raw : "x") : raw;}`,
		`{return (a ? b : c) ? (d ? (e ? raw : "x") : raw) : raw;}`,
		`{return (a ? b : c) ? raw : (d ? raw : (e ? raw : "x"));}`,
		`{return (raw ? b : c) ? (d ? raw : "x") : raw;}`,
		`{return (a ? raw : c) ? (d ? raw : "x") : raw;}`,
		`{return (a ? b : raw) ? (d ? raw : "x") : raw;}`,
		`{return (a ? b : c) ? (raw ? raw : "x") : raw;}`,
		`{return (a ? b : c) ? (d ? true : raw) : raw;}`,
		`{return (a ? b : c) ? (d ? missing : raw) : raw;}`,
		`{bool local=local;return ` + good + `;}`, `{bool local=later;bool later=a;return ` + good + `;}`,
		`{bool a=b;return ` + good + `;}`, `{bool local=a;bool local=b;return ` + good + `;}`,
		`{return (a ? b : c) ? (d ? propagate(raw) : raw) : raw;}`,
		`{return (a ? b : c) ? (d ? match(raw){some(v)=>v,none=>raw} : raw) : raw;}`,
	} {
		if strings.HasPrefix(body, "{return ") {
			body = "=>" + strings.TrimSuffix(strings.TrimPrefix(body, "{return "), "}")
		}
		source := prefix + body + `}`
		input := semanticTestModuleSet("compiler.selfhosting", []ModuleInput{testModule("compiler.selfhosting", "selector.pipe", source)}, nil)
		input.LanguageContract = PipeLangLanguageContractV1120
		analysis := AnalyzeSemanticModuleSet(input)
		if analysis.Error() == nil {
			t.Fatal("admitted", body)
		}
		for _, d := range analysis.Diagnostics {
			if !d.Primary.IsValid() || d.Primary.File != "selector.pipe" || d.Primary.End > len(source) {
				t.Fatal("invalid diagnostic", d)
			}
		}
	}
	for _, visibility := range []string{"private ", "protected "} {
		input := semanticTestModuleSet("compiler.selfhosting", []ModuleInput{testModule("compiler.selfhosting", "selector.pipe", strings.Replace(prefix, "public string", visibility+"string", 1)+`=>`+good+`;}`)}, nil)
		input.LanguageContract = PipeLangLanguageContractV1120
		if AnalyzeSemanticModuleSet(input).Error() == nil {
			t.Fatal("nonpublic admitted", visibility)
		}
	}
	for _, expr := range []string{`(a ? b : c) ? (d ? raw : "x") : raw`, `(a ? b : c) ? raw : (e ? "y" : raw)`, good} {
		for version := 1; version <= 111; version++ {
			input := semanticTestModuleSet("compiler.selfhosting", []ModuleInput{testModule("compiler.selfhosting", "selector.pipe", prefix+`=>`+expr+`;}`)}, nil)
			input.LanguageContract = LanguageContract(fmt.Sprintf("v0.%d.0", version))
			if AnalyzeSemanticModuleSet(input).Error() == nil {
				t.Fatal("older source admitted", version, expr)
			}
		}
	}
}

func TestV1120ArrowSelectorValueArmsPriorCoreContracts(t *testing.T) {
	_, p := conditionalLocalTreeProgramVersion(t, PipeLangLanguageContractV1120, arrowSelectorValueArmsSource, []string{"Select"})
	for version := 1; version <= 112; version++ {
		p.LanguageContract = fmt.Sprintf("v0.%d.0", version)
		if version >= 110 {
			if err := coreir.ValidateProgram(p); err != nil {
				t.Fatal(version, err)
			}
			continue
		}
		assertAdmissionRejected(t, p, "")
	}
}

func TestV1120ArrowSelectorValueArmsMalformedCore(t *testing.T) {
	clone := func(e *coreir.Expr) *coreir.Expr {
		data, _ := json.Marshal(e)
		var result coreir.Expr
		if err := json.Unmarshal(data, &result); err != nil {
			t.Fatal(err)
		}
		return &result
	}
	for _, mutation := range []string{"outer", "selector", "a", "b", "c", "x", "y", "a type", "b type", "c type", "result type", "nested a", "nested b", "nested c", "terminal outer", "terminal selector", "reference", "identity", "version", "argument", "initializer"} {
		t.Run(mutation, func(t *testing.T) {
			_, p := conditionalLocalTreeProgramVersion(t, PipeLangLanguageContractV1120, arrowSelectorValueArmsSource, []string{"Select"})
			f := &p.Functions[len(p.Functions)-1]
			outer := f.Body.Conditional
			selector := outer.Condition.Conditional
			switch mutation {
			case "outer":
				f.Body.Conditional = nil
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
			case "result type":
				f.Body.Type = outer.Condition.Type
			case "nested a":
				selector.Condition = clone(outer.Condition)
			case "nested b":
				selector.WhenTrue = clone(outer.Condition)
			case "nested c":
				selector.WhenFalse = clone(outer.Condition)
			case "terminal outer":
				// Core has no arrow spelling. This becomes an inherited terminal tree.
				outer.TerminalStatement = true
				for _, contract := range []string{coreir.LanguageContractV1110, coreir.LanguageContractV1120} {
					p.LanguageContract = contract
					if err := coreir.ValidateProgram(p); err != nil {
						t.Fatal("inherited terminal Core narrowed", err)
					}
				}
				return
			case "terminal selector":
				selector.TerminalStatement = true
			case "reference":
				position := 999
				selector.Condition = &coreir.Expr{Kind: coreir.ExprReference, Type: selector.Condition.Type, Parameter: &position}
			case "identity":
				p.CompilerContract = "unknown"
			case "version":
				p.LanguageContract = "v0.999.0"
			case "argument":
				body := clone(&f.Body)
				f.Body = coreir.Expr{Kind: coreir.ExprTextTrim, Type: body.Type, TextTrim: &coreir.TextTrim{Value: body}}
			case "initializer":
				body := clone(&f.Body)
				position := len(f.Parameters)
				f.Body = coreir.Expr{Kind: coreir.ExprImmutableLocal, Type: body.Type, ImmutableLocal: &coreir.ImmutableLocal{Name: "value", Position: position, Type: body.Type, Initializer: body, Return: &coreir.Expr{Kind: coreir.ExprReference, Type: body.Type, Parameter: &position}}}
			}
			assertAdmissionRejected(t, p, "")
		})
	}
}
