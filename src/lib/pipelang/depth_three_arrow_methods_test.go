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

const depthThreeArrowMethodsSource = `public Class Choices {
 public string Select(string raw,bool a,bool b,bool c) =>
 a ? (b ? (c ? trim(raw) : raw) : "B") : "A";
}`

func TestV950DepthThreeArrowMethodsAdmission(t *testing.T) {
	input := semanticTestModuleSet("compiler.selfhosting", []ModuleInput{testModule("compiler.selfhosting", "depth-three-arrow.pipe", depthThreeArrowMethodsSource)}, nil)
	input.LanguageContract = LanguageContract("v0.95.0")
	if err := AnalyzeSemanticModuleSet(input).Error(); err != nil {
		t.Fatal(err)
	}
}

func TestV950DepthThreeArrowMethodsInheritance(t *testing.T) {
	for _, source := range []string{depthThreeTerminalLeafReturnsSource, depthThreeStraightLineReturnsSource, nestedArrowMethodsSource, nestedTerminalInitializersSource, nestedStraightLineInitializersSource, nestedTerminalLeafReturnsSource, nestedStraightLineReturnsSource, terminalLeafConditionalReturnsSource, conditionalReturnCompositionSource, straightLineConditionalLocalsSource, finiteConditionalLocalsSource, twoConditionalLocalsSource, `public Class Choices {public string Select(string raw,bool pick)=>pick ? raw : trim(raw);}`} {
		var baseline [][]byte
		for _, contract := range []LanguageContract{PipeLangLanguageContractV940, PipeLangLanguageContractV950} {
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
			h.LanguageContract = coreir.LanguageContractV940
			p.LanguageContract = coreir.LanguageContractV940
			projection.LanguageContract = PipeLangLanguageContractV940
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

func TestV950DepthThreeArrowMethodsTypes(t *testing.T) {
	testConditionalLocalsTypeMatrix(t, PipeLangLanguageContractV950, true)
}
func TestV950DepthThreeArrowMethodsCarriers(t *testing.T) {
	testConditionalLocalsCarrierAndHostValues(t, PipeLangLanguageContractV950, true)
}

// All 25 shapes use independent heap-indexed condition bits. Arrow and block
// spelling must erase to identical executable representations.
func TestV950DepthThreeArrowMethodsLayouts(t *testing.T) {
	shapes := terminalTrees(3)[1:]
	if len(shapes) != 25 {
		t.Fatal(len(shapes))
	}
	for shape, tree := range shapes {
		t.Run(fmt.Sprint(shape), func(t *testing.T) {
			var head strings.Builder
			head.WriteString(`public Class Choices {public string Echo(string value)=>value;public bool Check(string path,bool value)=>value;public string Select(`)
			for bit := 0; bit < 7; bit++ {
				if bit > 0 {
					head.WriteString(",")
				}
				fmt.Fprintf(&head, "bool c%d", bit)
			}
			head.WriteString(")")
			expr := v930ChoiceSource(tree, 0, `"raw"`)
			source := head.String() + "=>" + expr + ";}"
			block := head.String() + "{return " + expr + ";}}"
			a, p := conditionalLocalTreeProgramVersion(t, PipeLangLanguageContractV950, source, []string{"Select"})
			b, q := conditionalLocalTreeProgramVersion(t, PipeLangLanguageContractV950, block, []string{"Select"})
			aa, pp := conditionalLocalTreeProgramVersion(t, PipeLangLanguageContractV950, source, []string{"Select"})
			if !reflect.DeepEqual(p, q) || !reflect.DeepEqual(p, pp) {
				t.Fatal("arrow/block or repeated Core mismatch")
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
				t.Fatal("arrow/block HIR mismatch")
			}
			projection, err := BuildSemanticProjection(a)
			if err != nil {
				t.Fatal(err)
			}
			repeated, err := BuildSemanticProjection(aa)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(projection, repeated) {
				t.Fatal("nondeterministic semantic projection")
			}
			generated, err := gobackend.Generate(p)
			if err != nil {
				t.Fatal(err)
			}
			blockGo, err := gobackend.Generate(q)
			if err != nil || !bytes.Equal(generated, blockGo) {
				t.Fatal("arrow/block Go mismatch")
			}
			f := coreFunctionNamed(t, p, "Select")
			var checks, orders strings.Builder
			for mask := 0; mask < 128; mask++ {
				suffix, path := v930ChoiceWant(tree, mask)
				want := "raw" + suffix
				args := []coreeval.Value{}
				call := "PipeLangSelect("
				trace := []string{}
				for _, event := range path {
					trace = append(trace, "C:"+event)
				}
				trace = append(trace, "E:"+want)
				for bit := 0; bit < 7; bit++ {
					flag := mask&(1<<bit) != 0
					args = append(args, coreeval.Value{Type: f.Parameters[bit].Type, Bool: flag})
					if bit > 0 {
						call += ","
					}
					call += fmt.Sprint(flag)
				}
				call += ")"
				got, err := coreeval.EvaluateProgram(p, f.Identity, args)
				if err != nil || !got.OK || got.Value.String != want {
					t.Fatalf("%d: %#v %v want %q", mask, got, err, want)
				}
				fmt.Fprintf(&checks, "if got:=%s;got!=%q{t.Fatal(got)}\n", call, want)
				fmt.Fprintf(&orders, "v950Trace=nil;%s;if !reflect.DeepEqual(v950Trace,[]string{%s}){t.Fatal(v950Trace)}\n", call, quotedStrings(trace))
			}
			compileAndRunGeneratedGoFiles(t, generated, []byte(fmt.Sprintf("package %s\nimport \"testing\"\nfunc TestValues(t *testing.T){%s}", gobackend.PackageName, checks.String())))
			observed := string(generated)
			for marker, probe := range map[string]string{"func PipeLangEcho(p0 string) string {": `v950Trace=append(v950Trace,"E:"+p0)`, "func PipeLangCheck(p0 string, p1 bool) bool {": `v950Trace=append(v950Trace,"C:"+p0)`} {
				if strings.Count(observed, marker) != 1 {
					t.Fatal("missing trace marker")
				}
				observed = strings.Replace(observed, marker, marker+"\n"+probe, 1)
			}
			compileAndRunGeneratedGoFiles(t, []byte(observed), []byte(fmt.Sprintf("package %s\nimport (\"testing\";\"reflect\")\nvar v950Trace []string\nfunc TestOrder(t *testing.T){%s}", gobackend.PackageName, orders.String())))
		})
	}
}

func TestV950DepthThreeArrowMethodsComputedCarriers(t *testing.T) {
	for _, unused := range []bool{false, true} {
		t.Run(fmt.Sprint(unused), func(t *testing.T) {
			local := ""
			if unused {
				local = "Result<int,ArithmeticError> unused=a ? (b ? Make(value) : Make(value)) : (c ? Make(value) : Make(value));"
			}
			body := `a ? (b ? (c ? Make(value) : Make(0)) : Make(0)) : (c ? Make(1) : Make(2))`
			source := `public Class Choices {public Result<int,ArithmeticError> Make(int value)=>value+1;public Result<int,ArithmeticError> Choose(int value,bool a,bool b,bool c)=>` + body + `;public Result<int,ArithmeticError> Select(int value,bool a,bool b,bool c){` + local + `return Choose(value,a,b,c);}}`

			if !unused {
				source = strings.Replace(source, "{return Choose(value,a,b,c);}", "=>Choose(value,a,b,c);", 1)
			}
			_, program := conditionalLocalTreeProgramVersion(t, PipeLangLanguageContractV950, source, []string{"Select"})
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

func TestV950DepthThreeArrowMethodsSourceRejection(t *testing.T) {
	prefix := `public Class Choices {public string Select(string raw,bool a,bool b,bool c) `
	deep := `a ? (b ? (c ? raw : "C") : "B") : "A"`
	cases := []string{
		`{string local=` + deep + `;return local;}`,
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
				input.LanguageContract = PipeLangLanguageContractV950
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
	for version := 1; version <= 94; version++ {
		input := semanticTestModuleSet("compiler.selfhosting", []ModuleInput{testModule("compiler.selfhosting", "depth-three.pipe", prefix+`=>`+deep+`;}`)}, nil)
		input.LanguageContract = LanguageContract(fmt.Sprintf("v0.%d.0", version))
		if AnalyzeSemanticModuleSet(input).Error() == nil {
			t.Fatal("older source admitted", version)
		}
	}
}

func TestV950DepthThreeArrowMethodsMalformedCore(t *testing.T) {
	source := `public Class Choices {public string Select(string raw,bool a,bool b,bool c) => a ? (b ? (c ? raw : "C") : "B") : "A";}`
	copyExpr := func(e *coreir.Expr) *coreir.Expr {
		b, _ := json.Marshal(e)
		var out coreir.Expr
		if err := json.Unmarshal(b, &out); err != nil {
			t.Fatal(err)
		}
		return &out
	}
	for _, mutation := range []string{"condition", "arm", "missing", "depth", "condition placement", "argument placement", "hidden local", "terminal", "identity", "version", "downgrade", "initializer"} {
		t.Run(mutation, func(t *testing.T) {
			_, p := conditionalLocalTreeProgramVersion(t, PipeLangLanguageContractV950, source, []string{"Select"})
			f := &p.Functions[len(p.Functions)-1]
			root := f.Body.Conditional
			third := root.WhenTrue.Conditional.WhenTrue.Conditional
			switch mutation {
			case "condition":
				third.Condition = third.WhenTrue
			case "arm":
				third.WhenTrue = third.Condition
			case "missing":
				third.WhenFalse = nil
			case "depth":
				third.WhenTrue = copyExpr(root.WhenTrue.Conditional.WhenTrue)
			case "condition placement":
				third.Condition = &coreir.Expr{Kind: coreir.ExprConditional, Type: third.Condition.Type, Conditional: &coreir.Conditional{Condition: third.Condition, WhenTrue: third.Condition, WhenFalse: third.Condition}}
			case "argument placement":
				third.WhenTrue = &coreir.Expr{Kind: coreir.ExprTextTrim, Type: f.ReturnType, TextTrim: &coreir.TextTrim{Value: copyExpr(root.WhenTrue)}}
			case "hidden local":
				third.WhenTrue = &coreir.Expr{Kind: coreir.ExprImmutableLocal, Type: f.ReturnType, ImmutableLocal: &coreir.ImmutableLocal{}}
			case "terminal":
				third.TerminalStatement = true
			case "identity":
				p.CompilerContract = "unknown"
			case "version":
				p.LanguageContract = "unknown"
			case "downgrade":
				p.LanguageContract = coreir.LanguageContractV920
			case "initializer":
				_, q := conditionalLocalTreeProgramVersion(t, PipeLangLanguageContractV950, depthThreeStraightLineReturnsSource, []string{"Select"})
				q.Functions[len(q.Functions)-1].Body.ImmutableLocal.Initializer = copyExpr(&f.Body)
				p = q
				f = &p.Functions[len(p.Functions)-1]
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
	_, p := conditionalLocalTreeProgramVersion(t, PipeLangLanguageContractV950, source, []string{"Select"})
	for _, version := range []string{coreir.LanguageContractV930, coreir.LanguageContractV940, coreir.LanguageContractV950} {
		p.LanguageContract = version
		if err := coreir.ValidateProgram(p); err != nil {
			t.Fatal("inherited block Core rejected", version, err)
		}
		if _, err := gobackend.Generate(p); err != nil {
			t.Fatal(err)
		}
	}
}
