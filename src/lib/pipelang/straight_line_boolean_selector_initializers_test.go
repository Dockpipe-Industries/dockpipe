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

const straightLineBooleanSelectorInitializersSource = `public Class Choices {public string Select(string raw,bool a,bool b,bool c){string value=(a ? b : c) ? trim(raw) : raw;return value;}}`

// Move exactly one complete selector return into a typed local, preserving its value.
func v1010Initializer(t *testing.T, source string) string {
	t.Helper()
	pattern := regexp.MustCompile(`public ([^{};]+) Select\([^{}]*\)\{`)
	match := pattern.FindStringSubmatch(source)
	if len(match) != 2 || strings.Count(source, "return (") != 1 {
		t.Fatal("expected one selector return")
	}
	start := strings.Index(source, "return (")
	end := strings.Index(source[start:], ";") + start
	if end < start {
		t.Fatal("missing return terminator")
	}
	return source[:start] + match[1] + " selected=" + source[start+7:end] + ";return selected" + source[end:]
}
func TestV1010StraightLineBooleanSelectorInitializersAdmission(t *testing.T) {
	for _, s := range []string{straightLineBooleanSelectorInitializersSource, strings.Replace(straightLineBooleanSelectorInitializersSource, "public string Select", "string Select", 1), `public Class Choices {public string Select(){string x=(true ? false : true) ? "x" : "y";return x;}}`} {
		_, p := conditionalLocalTreeProgramVersion(t, PipeLangLanguageContractV1010, s, []string{"Select"})
		if err := coreir.ValidateProgram(p); err != nil {
			t.Fatal(err)
		}
	}
}
func TestV1010StraightLineBooleanSelectorInitializersTypes(t *testing.T) {
	testConditionalLocalsTypeMatrix(t, PipeLangLanguageContractV1010, true)
}
func TestV1010StraightLineBooleanSelectorInitializersCarriers(t *testing.T) {
	testConditionalLocalsCarrierAndHostValues(t, PipeLangLanguageContractV1010, true)
}

func TestV1010StraightLineBooleanSelectorInitializersInheritance(t *testing.T) {
	for _, source := range []string{arrowBooleanSelectorsSource, v990Wrap(t, conditionalBooleanSelectorsSource), conditionalBooleanSelectorsSource, v940WrapBody(t, depthThreeStraightLineInitializersSource, terminalTrees(3)[25], "a"), depthThreeStraightLineInitializersSource, depthThreeArrowMethodsSource, depthThreeTerminalLeafReturnsSource, depthThreeStraightLineReturnsSource, nestedArrowMethodsSource, nestedTerminalInitializersSource, nestedStraightLineInitializersSource, nestedTerminalLeafReturnsSource, nestedStraightLineReturnsSource, terminalLeafConditionalReturnsSource, conditionalReturnCompositionSource, straightLineConditionalLocalsSource, finiteConditionalLocalsSource, twoConditionalLocalsSource, `public Class Choices {public string Select(string raw,bool pick)=>pick ? raw : trim(raw);}`} {
		var baseline [][]byte
		for _, contract := range []LanguageContract{PipeLangLanguageContractV1000, PipeLangLanguageContractV1010} {
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
			h.LanguageContract = coreir.LanguageContractV1000
			p.LanguageContract = coreir.LanguageContractV1000
			projection.LanguageContract = PipeLangLanguageContractV1000
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

func TestV1010StraightLineBooleanSelectorInitializersComputedCarriers(t *testing.T) {
	for _, unused := range []bool{false, true} {
		t.Run(fmt.Sprint(unused), func(t *testing.T) {
			local := ""
			if unused {
				local = `Result<int,ArithmeticError> unused=(a ? b : c) ? Make(value) : Make(0);`
			}
			source := `public Class Choices {public Result<int,ArithmeticError> Make(int value)=>value+1;public Result<int,ArithmeticError> Select(int value,bool a,bool b,bool c){` + local + `Result<int,ArithmeticError> selected=(a ? b : c) ? Make(value) : Make(0);return selected;}}`
			_, p := conditionalLocalTreeProgramVersion(t, PipeLangLanguageContractV1010, source, []string{"Select"})
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

func TestV1010StraightLineBooleanSelectorInitializersMalformedCore(t *testing.T) {
	clone := func(e *coreir.Expr) *coreir.Expr {
		b, _ := json.Marshal(e)
		var out coreir.Expr
		if err := json.Unmarshal(b, &out); err != nil {
			t.Fatal(err)
		}
		return &out
	}
	for _, mutation := range []string{"outer", "selector", "a", "b", "c", "x", "y", "a type", "b type", "c type", "x type", "result type", "nested a", "nested b", "nested c", "nested x", "nested y", "argument", "terminal root", "terminal branch", "terminal outer", "terminal selector", "hidden a", "hidden x", "local", "init", "continuation", "position", "reference", "self", "identity", "version"} {
		t.Run(mutation, func(t *testing.T) {
			_, p := conditionalLocalTreeProgramVersion(t, PipeLangLanguageContractV1010, straightLineBooleanSelectorInitializersSource, []string{"Select"})
			f := &p.Functions[len(p.Functions)-1]
			local := f.Body.ImmutableLocal
			returned := local.Initializer
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
			case "terminal root":
				local.Return = &coreir.Expr{Kind: coreir.ExprConditional, Type: returned.Type, Conditional: &coreir.Conditional{TerminalStatement: true, Condition: clone(selector.Condition), WhenTrue: clone(local.Return), WhenFalse: clone(outer.WhenFalse)}}
			case "terminal branch":
				f.Body = coreir.Expr{Kind: coreir.ExprConditional, Type: returned.Type, Conditional: &coreir.Conditional{TerminalStatement: true, Condition: clone(selector.Condition), WhenTrue: clone(&f.Body), WhenFalse: clone(outer.WhenFalse)}}
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
				local.Initializer = clone(local.Return)
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

func TestV1010StraightLineBooleanSelectorInitializersInternalCoreBoundary(t *testing.T) {
	for _, slot := range []string{"a", "b", "c", "x", "y"} {
		t.Run(slot, func(t *testing.T) {
			_, p := conditionalLocalTreeProgramVersion(t, PipeLangLanguageContractV1010, straightLineBooleanSelectorInitializersSource, []string{"Select"})
			f := &p.Functions[len(p.Functions)-1]
			outer := f.Body.ImmutableLocal.Initializer.Conditional
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

func TestV1010StraightLineBooleanSelectorInitializersPriorContracts(t *testing.T) {
	_, p := conditionalLocalTreeProgramVersion(t, PipeLangLanguageContractV1010, straightLineBooleanSelectorInitializersSource, []string{"Select"})
	for version := 1; version <= 100; version++ {
		contract := LanguageContract(fmt.Sprintf("v0.%d.0", version))
		input := semanticTestModuleSet("compiler.selfhosting", []ModuleInput{testModule("compiler.selfhosting", "selector.pipe", straightLineBooleanSelectorInitializersSource)}, nil)
		input.LanguageContract = contract
		if AnalyzeSemanticModuleSet(input).Error() == nil {
			t.Fatal("earlier source admitted initializer", version)
		}
		p.LanguageContract = string(contract)
		assertAdmissionRejected(t, p, "")
	}
}
func TestV1010StraightLineBooleanSelectorInitializersSourceRejection(t *testing.T) {
	prefix := `public Class Choices {public string Select(string raw,bool a,bool b,bool c)`
	for _, body := range []string{
		`{string x=((a ? b : c) ? b : c) ? raw : "x";return x;}`,
		`{string x=(a ? (b ? a : c) : c) ? raw : "x";return x;}`,
		`{string x=(a ? b : (c ? a : b)) ? raw : "x";return x;}`,
		`{string x=(a ? b : c) ? (a ? raw : "x") : raw;return x;}`,
		`{string x=(a ? b : c) ? raw : (a ? raw : "x");return x;}`,
		`{string x=((a ? b : c) && a) ? raw : "x";return x;}`,
		`{string x=(raw ? b : c) ? raw : "x";return x;}`,
		`{string x=(a ? raw : c) ? raw : "x";return x;}`,
		`{string x=(a ? b : raw) ? raw : "x";return x;}`,
		`{string x=(a ? b : c) ? true : raw;return x;}`,
		`{bool x=(a ? b : c) ? raw : "x";return raw;}`,
		`{string x=(a ? b : c) ? missing : raw;return x;}`,
		`{string x=(a ? b : c) ? x : raw;return x;}`,
		`{string x=(a ? b : c) ? later : raw;string later=raw;return x;}`,
		`{string raw=(a ? b : c) ? raw : "x";return raw;}`,
		`{string x=(a ? b : c) ? raw : "x";string x=raw;return x;}`,
		`{string x=trim((a ? b : c) ? raw : "x");return x;}`,
		`{string x=(a ? b : c) ? propagate(raw) : raw;return x;}`,
		`{string x=(a ? b : c) ? match(raw){some(v)=>v,none=>raw} : raw;return x;}`,
		`{string x=(a ? b : c) ? raw : "x";if(a){return x;}else{return raw;}}`,
		`{if(a){string x=(a ? b : c) ? raw : "x";return x;}else{return raw;}}`,
		`{if(a){return raw;}else{string x=(a ? b : c) ? raw : "x";return x;}}`,
		`{if(a){if(b){string x=(a ? b : c) ? raw : "x";return x;}else{return raw;}}else{return raw;}}`,
		`{string x=(a ? b : c) ? raw : "x";return trim((a ? b : c) ? x : raw);}`,
		`{string x=(a ? b : c) ? raw : "x";return a ? (b ? (c ? (a ? x : raw) : raw) : raw) : raw;}`,
		`{if(a ? b : c){return raw;}else{return "x";}}`,
		`=> {string x=(a ? b : c) ? raw : "x";return x;};`,
	} {
		input := semanticTestModuleSet("compiler.selfhosting", []ModuleInput{testModule("compiler.selfhosting", "selector.pipe", prefix+body+"}")}, nil)
		input.LanguageContract = PipeLangLanguageContractV1010
		if AnalyzeSemanticModuleSet(input).Error() == nil {
			t.Fatal("excluded source admitted", body)
		}
	}
	for _, source := range []string{strings.Replace(straightLineBooleanSelectorInitializersSource, "public string Select", "private string Select", 1), strings.Replace(arrowBooleanSelectorsSource, "public string Select", "private string Select", 1)} {
		input := semanticTestModuleSet("compiler.selfhosting", []ModuleInput{testModule("compiler.selfhosting", "selector.pipe", source)}, nil)
		input.LanguageContract = PipeLangLanguageContractV1010
		if AnalyzeSemanticModuleSet(input).Error() == nil {
			t.Fatal("private new form admitted")
		}
	}
}

// Twenty-four layouts cover all nonempty subsets of two selector slots, a four-local
// mixed sequence, unused tails and ordinary/depth-three/selector returns.
// The two selector triples and return triple are independent (512 vectors/layout).
func TestV1010StraightLineBooleanSelectorInitializersLayouts(t *testing.T) {
	for shape := 0; shape < 24; shape++ {
		t.Run(fmt.Sprint(shape), func(t *testing.T) {
			subset := []int{1, 2, 3, 5}[shape/6]
			count := 2
			if subset == 5 {
				count = 4
			}
			unused := shape/3%2 == 1
			returnKind := shape % 3
			source := `public Class Choices {public string Echo(string value)=>value;public bool Check(string name,bool value)=>value;public string Select(string raw,bool a,bool b,bool c,bool d,bool e,bool f,bool q,bool r,bool s){`
			previous := "raw"
			choiceIndex := 0
			for i := 0; i < count; i++ {
				init := fmt.Sprintf(`Echo(%s+"O")`, previous)
				if i%2 == 1 {
					init = fmt.Sprintf(`Check("q%d",q) ? (Check("r%d",r) ? (Check("s%d",s) ? Echo(%s+"T") : Echo(%s+"M")) : Echo(%s+"N")) : Echo(%s+"F")`, i, i, i, previous, previous, previous, previous)
				}
				if subset&(1<<i) != 0 {
					names := []string{"a", "b", "c"}
					if choiceIndex == 1 {
						names = []string{"d", "e", "f"}
					}
					choiceIndex++
					init = fmt.Sprintf(`(Check("a%d",%s) ? Check("b%d",%s) : Check("c%d",%s)) ? Echo(%s+"Y") : Echo(%s+"Z")`, i, names[0], i, names[1], i, names[2], previous, previous)
				}
				source += fmt.Sprintf("string l%d=%s;", i, init)
				if !unused || i < count-1 {
					previous = fmt.Sprintf("l%d", i)
				}
			}
			returned := previous
			switch returnKind {
			case 1:
				returned = fmt.Sprintf(`Check("rq",q) ? (Check("rr",r) ? (Check("rs",s) ? Echo(%s+"T") : Echo(%s+"M")) : Echo(%s+"N")) : Echo(%s+"F")`, previous, previous, previous, previous)
			case 2:
				returned = fmt.Sprintf(`(Check("ra",q) ? Check("rb",r) : Check("rc",s)) ? Echo(%s+"Y") : Echo(%s+"Z")`, previous, previous)
			}
			source += "return " + returned + ";}}"
			a, p := conditionalLocalTreeProgramVersion(t, PipeLangLanguageContractV1010, source, []string{"Select"})
			aa, pp := conditionalLocalTreeProgramVersion(t, PipeLangLanguageContractV1010, source, []string{"Select"})
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
				t.Fatal("nondeterministic HIR/Core/semantic")
			}
			generated, err := gobackend.Generate(p)
			if err != nil {
				t.Fatal(err)
			}
			again, err := gobackend.Generate(pp)
			if err != nil || !bytes.Equal(generated, again) {
				t.Fatal("nondeterministic Go", err)
			}
			f := coreFunctionNamed(t, p, "Select")
			body := f.Body
			for i := 0; i < count; i++ {
				if body.ImmutableLocal == nil {
					t.Fatal("local chain lost")
				}
				init := body.ImmutableLocal.Initializer
				if subset&(1<<i) != 0 && (init.Kind != coreir.ExprConditional || init.Conditional.Condition.Kind != coreir.ExprConditional) {
					t.Fatal("selector initializer lost")
				}
				body = *body.ImmutableLocal.Return
			}
			prepared := prepareConformanceProgram(t, p)
			var checks, orders strings.Builder
			for mask := 0; mask < 512; mask++ {
				flags := make([]bool, 9)
				args := []coreeval.Value{{Type: f.Parameters[0].Type, String: "raw"}}
				call := `PipeLangSelect("raw"`
				for bit := 0; bit < 9; bit++ {
					flags[bit] = mask&(1<<bit) != 0
					args = append(args, coreeval.Value{Type: f.Parameters[bit+1].Type, Bool: flags[bit]})
					call += fmt.Sprintf(",%t", flags[bit])
				}
				call += ")"
				value := "raw"
				trace := []string{}
				choiceIndex := 0
				depthSuffix := func(q, r, s string) string {
					trace = append(trace, "C:"+q)
					if !flags[6] {
						return "F"
					}
					trace = append(trace, "C:"+r)
					if !flags[7] {
						return "N"
					}
					trace = append(trace, "C:"+s)
					if flags[8] {
						return "T"
					}
					return "M"
				}
				for i := 0; i < count; i++ {
					suffix := "O"
					if subset&(1<<i) != 0 {
						offset := choiceIndex * 3
						choiceIndex++
						trace = append(trace, fmt.Sprintf("C:a%d", i))
						selected := flags[offset+2]
						if flags[offset] {
							selected = flags[offset+1]
							trace = append(trace, fmt.Sprintf("C:b%d", i))
						} else {
							trace = append(trace, fmt.Sprintf("C:c%d", i))
						}
						suffix = "Z"
						if selected {
							suffix = "Y"
						}
					} else if i%2 == 1 {
						suffix = depthSuffix(fmt.Sprintf("q%d", i), fmt.Sprintf("r%d", i), fmt.Sprintf("s%d", i))
					}
					next := value + suffix
					trace = append(trace, "E:"+next)
					if !unused || i < count-1 {
						value = next
					}
				}
				switch returnKind {
				case 1:
					value += depthSuffix("rq", "rr", "rs")
					trace = append(trace, "E:"+value)
				case 2:
					trace = append(trace, "C:ra")
					selected := flags[8]
					if flags[6] {
						selected = flags[7]
						trace = append(trace, "C:rb")
					} else {
						trace = append(trace, "C:rc")
					}
					if selected {
						value += "Y"
					} else {
						value += "Z"
					}
					trace = append(trace, "E:"+value)
				}
				got, err := prepared.Evaluate(f.Identity, args)
				if err != nil || !got.OK || got.Value.String != value {
					t.Fatal(mask, got, err, value)
				}
				fmt.Fprintf(&checks, "if got:=%s;got!=%q{t.Fatal(got)}\n", call, value)
				fmt.Fprintf(&orders, "v1010Trace=nil;%s;if !reflect.DeepEqual(v1010Trace,[]string{%s}){t.Fatal(v1010Trace)}\n", call, quotedStrings(trace))
			}
			compileAndRunGeneratedGoFiles(t, generated, []byte(fmt.Sprintf("package %s\nimport \"testing\"\nfunc TestValues(t *testing.T){%s}", gobackend.PackageName, checks.String())))
			observed := string(generated)
			for marker, probe := range map[string]string{"func PipeLangEcho(p0 string) string {": `v1010Trace=append(v1010Trace,"E:"+p0)`, "func PipeLangCheck(p0 string, p1 bool) bool {": `v1010Trace=append(v1010Trace,"C:"+p0)`} {
				if strings.Count(observed, marker) != 1 {
					t.Fatal("missing trace marker")
				}
				observed = strings.Replace(observed, marker, marker+"\n"+probe, 1)
			}
			compileAndRunGeneratedGoFiles(t, []byte(observed), []byte(fmt.Sprintf("package %s\nimport(\"testing\";\"reflect\")\nvar v1010Trace []string\nfunc TestOrder(t *testing.T){%s}", gobackend.PackageName, orders.String())))
		})
	}
}

func TestV1010StraightLineBooleanSelectorInitializersDependencies(t *testing.T) {
	source := `public Class Choices {public string Select(string raw,bool a,bool b,bool c){
 bool gate=(a ? b : c) ? true : false;
 bool second=(gate ? c : b) ? !a : a;
 string text=(second ? gate : c) ? trim(raw) : raw;
 return second ? text : raw+"!";}}`
	_, p := conditionalLocalTreeProgramVersion(t, PipeLangLanguageContractV1010, source, []string{"Select"})
	f := coreFunctionNamed(t, p, "Select")
	generated, err := gobackend.Generate(p)
	if err != nil {
		t.Fatal(err)
	}
	var checks strings.Builder
	for _, raw := range []string{"", " ", " text "} {
		for mask := 0; mask < 8; mask++ {
			a, b, c := mask&1 != 0, mask&2 != 0, mask&4 != 0
			gate := c
			if a {
				gate = b
			}
			chosen := b
			if gate {
				chosen = c
			}
			second := a
			if chosen {
				second = !a
			}
			picked := c
			if second {
				picked = gate
			}
			text := raw
			if picked {
				text = strings.TrimSpace(raw)
			}
			want := raw + "!"
			if second {
				want = text
			}
			args := []coreeval.Value{{Type: f.Parameters[0].Type, String: raw}, {Type: f.Parameters[1].Type, Bool: a}, {Type: f.Parameters[2].Type, Bool: b}, {Type: f.Parameters[3].Type, Bool: c}}
			got, err := coreeval.EvaluateProgram(p, f.Identity, args)
			if err != nil || !got.OK || got.Value.String != want {
				t.Fatal(mask, got, err, want)
			}
			fmt.Fprintf(&checks, "if got:=PipeLangSelect(%q,%t,%t,%t);got!=%q{t.Fatal(got)}\n", raw, a, b, c, want)
		}
	}
	compileAndRunGeneratedGoFiles(t, generated, []byte(fmt.Sprintf("package %s\nimport \"testing\"\nfunc TestDependencies(t *testing.T){%s}", gobackend.PackageName, checks.String())))
}
