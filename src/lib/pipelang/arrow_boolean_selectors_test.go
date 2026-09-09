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

const arrowBooleanSelectorsSource = `public Class Choices {public string Select(string raw,bool a,bool b,bool c)=>(a ? b : c) ? trim(raw) : raw;}`

// Convert only a complete zero-local Select block; failure must never silently test a block.
func v1000Arrow(t *testing.T, source string) string {
	t.Helper()
	pattern := regexp.MustCompile(`(public [^{};]+ Select\([^{}]*\))\{\s*return ([^;{}]+);\}`)
	if len(pattern.FindAllStringIndex(source, -1)) != 1 {
		t.Fatal("expected one zero-local Select block")
	}
	return pattern.ReplaceAllString(source, "$1=>$2;")
}
func TestV1000ArrowBooleanSelectorsAdmission(t *testing.T) {
	for _, source := range []string{arrowBooleanSelectorsSource, strings.Replace(arrowBooleanSelectorsSource, "public string Select", "string Select", 1), `public Class Choices {public string Select()=>(true ? false : true) ? "x" : "y";}`} {
		_, p := conditionalLocalTreeProgramVersion(t, PipeLangLanguageContractV1000, source, []string{"Select"})
		if err := coreir.ValidateProgram(p); err != nil {
			t.Fatal(err)
		}
	}
}
func TestV1000ArrowBooleanSelectorsTypes(t *testing.T) {
	testConditionalLocalsTypeMatrix(t, PipeLangLanguageContractV1000, true)
}
func TestV1000ArrowBooleanSelectorsCarriers(t *testing.T) {
	testConditionalLocalsCarrierAndHostValues(t, PipeLangLanguageContractV1000, true)
}

func TestV1000ArrowBooleanSelectorsInheritance(t *testing.T) {
	for _, source := range []string{v990Wrap(t, conditionalBooleanSelectorsSource), conditionalBooleanSelectorsSource, v940WrapBody(t, depthThreeStraightLineInitializersSource, terminalTrees(3)[25], "a"), depthThreeStraightLineInitializersSource, depthThreeArrowMethodsSource, depthThreeTerminalLeafReturnsSource, depthThreeStraightLineReturnsSource, nestedArrowMethodsSource, nestedTerminalInitializersSource, nestedStraightLineInitializersSource, nestedTerminalLeafReturnsSource, nestedStraightLineReturnsSource, terminalLeafConditionalReturnsSource, conditionalReturnCompositionSource, straightLineConditionalLocalsSource, finiteConditionalLocalsSource, twoConditionalLocalsSource, `public Class Choices {public string Select(string raw,bool pick)=>pick ? raw : trim(raw);}`} {
		var baseline [][]byte
		for _, contract := range []LanguageContract{PipeLangLanguageContractV990, PipeLangLanguageContractV1000} {
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
			h.LanguageContract = coreir.LanguageContractV990
			p.LanguageContract = coreir.LanguageContractV990
			projection.LanguageContract = PipeLangLanguageContractV990
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

func TestV1000ArrowBooleanSelectorsLayouts(t *testing.T) {
	for _, count := range []int{0, 1, 4} {
		for _, unused := range []bool{false, true} {
			t.Run(fmt.Sprintf("%d/%t", count, unused), func(t *testing.T) {
				source := `public Class Choices {public string Echo(string value)=>value;public bool Check(string name,bool value)=>value;public string Choose(string raw,bool a,bool b,bool c)=>(Check("a",a) ? Check("b",b) : Check("c",c)) ? Echo(raw+"Y") : Echo(raw+"Z");public string Select(string raw,bool a,bool b,bool c,bool q,bool r,bool s){`
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
				source += fmt.Sprintf(`return Choose(%s,a,b,c);}}`, previous)
				if count == 0 {
					source = v1000Arrow(t, source)
				}
				a, p := conditionalLocalTreeProgramVersion(t, PipeLangLanguageContractV1000, source, []string{"Select"})
				aa, pp := conditionalLocalTreeProgramVersion(t, PipeLangLanguageContractV1000, source, []string{"Select"})
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
				body := coreFunctionNamed(t, p, "Choose").Body
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
					fmt.Fprintf(&orders, "v1000Trace=nil;%s;if !reflect.DeepEqual(v1000Trace,[]string{%s}){t.Fatal(v1000Trace)}\n", call, quotedStrings(trace))
				}
				compileAndRunGeneratedGoFiles(t, generated, []byte(fmt.Sprintf("package %s\nimport \"testing\"\nfunc TestValues(t *testing.T){%s}", gobackend.PackageName, checks.String())))
				observed := string(generated)
				for marker, probe := range map[string]string{"func PipeLangEcho(p0 string) string {": `v1000Trace=append(v1000Trace,"E:"+p0)`, "func PipeLangCheck(p0 string, p1 bool) bool {": `v1000Trace=append(v1000Trace,"C:"+p0)`} {
					if strings.Count(observed, marker) != 1 {
						t.Fatal("missing trace marker")
					}
					observed = strings.Replace(observed, marker, marker+"\n"+probe, 1)
				}
				compileAndRunGeneratedGoFiles(t, []byte(observed), []byte(fmt.Sprintf("package %s\nimport (\"testing\";\"reflect\")\nvar v1000Trace []string\nfunc TestOrder(t *testing.T){%s}", gobackend.PackageName, orders.String())))
			})
		}
	}
}

func TestV1000ArrowBooleanSelectorsComputedCarriers(t *testing.T) {
	for _, unused := range []bool{false, true} {
		t.Run(fmt.Sprint(unused), func(t *testing.T) {
			local := ""
			if unused {
				local = `Result<int,ArithmeticError> unused=a ? (b ? (c ? Make(value) : Make(value)) : Make(value)) : Make(value);`
			}
			source := `public Class Choices {public Result<int,ArithmeticError> Make(int value)=>value+1;public Result<int,ArithmeticError> Choose(int value,bool a,bool b,bool c)=>(a ? b : c) ? Make(value) : Make(0);public Result<int,ArithmeticError> Select(int value,bool a,bool b,bool c){` + local + `return Choose(value,a,b,c);}}`
			if !unused {
				source = v1000Arrow(t, source)
			}
			_, p := conditionalLocalTreeProgramVersion(t, PipeLangLanguageContractV1000, source, []string{"Select"})
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

func TestV1000ArrowBooleanSelectorsMalformedCore(t *testing.T) {
	clone := func(e *coreir.Expr) *coreir.Expr {
		b, _ := json.Marshal(e)
		var out coreir.Expr
		if err := json.Unmarshal(b, &out); err != nil {
			t.Fatal(err)
		}
		return &out
	}
	for _, mutation := range []string{"outer", "selector", "a", "b", "c", "x", "y", "a type", "b type", "c type", "x type", "result type", "nested a", "nested b", "nested c", "nested x", "nested y", "argument", "initializer", "terminal outer", "terminal selector", "hidden a", "hidden x", "local", "init", "continuation", "position", "reference", "self", "identity", "version"} {
		t.Run(mutation, func(t *testing.T) {
			_, p := conditionalLocalTreeProgramVersion(t, PipeLangLanguageContractV1000, conditionalBooleanSelectorsSource, []string{"Select"})
			f := &p.Functions[len(p.Functions)-1]
			local := f.Body.ImmutableLocal
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
			case "terminal leaf":
				local.Return = &coreir.Expr{Kind: coreir.ExprConditional, Type: returned.Type, Conditional: &coreir.Conditional{TerminalStatement: true, Condition: clone(selector.Condition), WhenTrue: clone(returned), WhenFalse: clone(outer.WhenFalse)}}
			case "hidden a":
				hidden := clone(&f.Body)
				hidden.Type = selector.Condition.Type
				hidden.ImmutableLocal.Return = clone(selector.Condition)
				selector.Condition = hidden
			case "hidden x":
				outer.WhenTrue = clone(&f.Body)
			case "local":
				f.Body.ImmutableLocal = nil
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

func TestV1000ArrowBooleanSelectorsInternalCoreBoundary(t *testing.T) {
	for _, slot := range []string{"a", "b", "c", "x", "y"} {
		t.Run(slot, func(t *testing.T) {
			_, p := conditionalLocalTreeProgramVersion(t, PipeLangLanguageContractV1000, arrowBooleanSelectorsSource, []string{"Select"})
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

func TestV1000ArrowBooleanSelectorsEquivalence(t *testing.T) {
	for _, expr := range []string{`(a ? b : c) ? trim(raw) : raw`, `(Check(a && raw!="") ? Check(b || raw=="") : Check(!c)) ? trim(raw) : raw`} {
		head := `public Class Choices {public bool Check(bool value)=>value;public string Select(string raw,bool a,bool b,bool c)`
		arrow, block := head+"=>"+expr+";}", head+"{return "+expr+";}}"
		a, p := conditionalLocalTreeProgramVersion(t, PipeLangLanguageContractV1000, arrow, []string{"Select"})
		b, q := conditionalLocalTreeProgramVersion(t, PipeLangLanguageContractV1000, block, []string{"Select"})
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
				want := raw
				if selected {
					want = strings.TrimSpace(raw)
				}
				if strings.Contains(expr, "Check(") {
					selected = !c
					if a && raw != "" {
						selected = b || raw == ""
					}
					want = raw
					if selected {
						want = strings.TrimSpace(raw)
					}
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

func TestV1000ArrowBooleanSelectorsSourceRejection(t *testing.T) {
	prefix := `public Class Choices {public string Select(string raw,bool a,bool b,bool c)`
	for _, body := range []string{
		`=> ((a ? b : c) ? b : c) ? raw : "x";`,
		`=> (a ? (b ? a : c) : c) ? raw : "x";`,
		`=> (a ? b : (c ? a : b)) ? raw : "x";`,
		`=> (a ? b : c) ? (a ? raw : "x") : raw;`,
		`=> (a ? b : c) ? raw : (a ? raw : "x");`,
		`=> ((a ? b : c) && a) ? raw : "x";`,
		`=> (raw ? b : c) ? raw : "x";`,
		`=> (a ? raw : c) ? raw : "x";`,
		`=> (a ? b : raw) ? raw : "x";`,
		`=> (a ? b : c) ? true : raw;`,
		`=> (a ? b : c) ? raw : missing;`,
		`=> trim((a ? b : c) ? raw : "x");`,
		`=> (a ? b : c) ? propagate(raw) : raw;`,
		`=> (a ? b : c) ? match(raw){some(v)=>v,none=>raw} : raw;`,
		`{string local=(a ? b : c) ? raw : "x";return local;}`,
		`{if(a ? b : c){return raw;}else{return "x";}}`,
		`{if(a){string local=(a ? b : c) ? raw : "x";return local;}else{return raw;}}`,
		`=> {string local=raw;return (a ? b : c) ? local : raw;};`,
		`=> a ? (b ? (c ? (a ? raw : "x") : raw) : raw) : raw;`,
	} {
		input := semanticTestModuleSet("compiler.selfhosting", []ModuleInput{testModule("compiler.selfhosting", "arrow.pipe", prefix+body+"}")}, nil)
		input.LanguageContract = PipeLangLanguageContractV1000
		analysis := AnalyzeSemanticModuleSet(input)
		if analysis.Error() == nil {
			t.Fatal("excluded source admitted", body)
		}
	}
}

func TestV1000ArrowBooleanSelectorsPriorContracts(t *testing.T) {
	_, p := conditionalLocalTreeProgramVersion(t, PipeLangLanguageContractV1000, arrowBooleanSelectorsSource, []string{"Select"})
	for version := 1; version <= 99; version++ {
		contract := LanguageContract(fmt.Sprintf("v0.%d.0", version))
		input := semanticTestModuleSet("compiler.selfhosting", []ModuleInput{testModule("compiler.selfhosting", "arrow.pipe", arrowBooleanSelectorsSource)}, nil)
		input.LanguageContract = contract
		if AnalyzeSemanticModuleSet(input).Error() == nil {
			t.Fatal("prior source accepted arrow", version)
		}
		p.LanguageContract = string(contract)
		if version < 98 {
			assertAdmissionRejected(t, p, "")
			continue
		}
		// v0.98 and v0.99 already admit precisely this Core through block syntax.
		if err := coreir.ValidateProgram(p); err != nil {
			t.Fatal("prior block Core narrowed", version, err)
		}
		if _, err := gobackend.Generate(p); err != nil {
			t.Fatal(err)
		}
		f := coreFunctionNamed(t, p, "Select")
		args := []coreeval.Value{{Type: f.Parameters[0].Type, String: " x "}}
		for i := 1; i < 4; i++ {
			args = append(args, coreeval.Value{Type: f.Parameters[i].Type, Bool: true})
		}
		got, err := coreeval.EvaluateProgram(p, f.Identity, args)
		if err != nil || !got.OK || got.Value.String != "x" {
			t.Fatal(got, err)
		}
	}
}

// Source spelling changes declarations and byte hashes; semantic identities/types remain exact.
func v1000SemanticWithoutSource(t *testing.T, projection *SemanticProjection) any {
	t.Helper()
	data, err := json.Marshal(projection)
	if err != nil {
		t.Fatal(err)
	}
	var result any
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatal(err)
	}
	var walk func(any)
	walk = func(value any) {
		switch x := value.(type) {
		case map[string]any:
			for key, child := range x {
				if key == "declaration" || key == "source_sha256" {
					delete(x, key)
				} else {
					walk(child)
				}
			}
		case []any:
			for _, child := range x {
				walk(child)
			}
		}
	}
	walk(result)
	return result
}

func TestV1000ArrowBooleanSelectorsVisibility(t *testing.T) {
	for _, visibility := range []string{"private "} {
		source := strings.Replace(arrowBooleanSelectorsSource, "public string Select", visibility+"string Select", 1)
		input := semanticTestModuleSet("compiler.selfhosting", []ModuleInput{testModule("compiler.selfhosting", "arrow.pipe", source)}, nil)
		input.LanguageContract = PipeLangLanguageContractV1000
		if AnalyzeSemanticModuleSet(input).Error() == nil {
			t.Fatal("nonpublic selector arrow admitted")
		}
	}
}
