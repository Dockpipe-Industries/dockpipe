package pipelang

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"dockpipe/src/lib/pipelang/coreeval"
	"dockpipe/src/lib/pipelang/coreir"
	"dockpipe/src/lib/pipelang/gobackend"
)

const nestedArrowMethodsSource = `public Class Choices {
 public string Select(string raw,bool outer,bool left,bool right) =>
 outer ? (left ? trim(raw) : raw) : (right ? "fallback" : raw);
}`

func TestV920NestedArrowMethodsAdmission(t *testing.T) {
	input := semanticTestModuleSet("compiler.selfhosting", []ModuleInput{testModule("compiler.selfhosting", "arrow.pipe", nestedArrowMethodsSource)}, nil)
	input.LanguageContract = LanguageContract("v0.92.0")
	if err := AnalyzeSemanticModuleSet(input).Error(); err != nil {
		t.Fatal(err)
	}
}

// Only convert the complete, local-free Select return used by the typed fixtures.
func v920ArrowSelect(t *testing.T, source string, shapeBits ...bool) string {
	t.Helper()
	start := strings.Index(source, " Select(")
	if start < 0 {
		t.Fatal("missing Select")
	}
	open := strings.Index(source[start:], "{") + start
	close := strings.Index(source[open:], "}") + open
	body := strings.TrimSpace(source[open+1 : close])
	if !strings.HasPrefix(body, "return ") || strings.Count(body, ";") != 1 {
		t.Fatal("not a single return")
	}
	if len(shapeBits) == 2 {
		shape := 0
		if shapeBits[0] {
			shape |= 1
		}
		if shapeBits[1] {
			shape |= 2
		}
		first, second, outer, yes, no := "pick", "inner", "outer", "left", "right"
		if strings.Contains(source, "bool first") {
			first, second, outer, yes, no = "first", "second", "enabled", "value", "fallback"
		}
		switch shape {
		case 0:
			body = fmt.Sprintf("return %s && (!%s || %s) ? %s : %s;", first, outer, second, yes, no)
		case 1:
			body = fmt.Sprintf("return %s ? (%s && !%s ? %s : %s) : %s;", first, outer, second, no, yes, no)
		case 2:
			body = fmt.Sprintf("return !%s ? %s : (%s && !%s ? %s : %s);", first, no, outer, second, no, yes)
		}
	}
	return source[:open] + "=>" + strings.TrimPrefix(body, "return ") + source[close+1:]
}

func TestV920NestedArrowMethodsTypes(t *testing.T) {
	for shape := 0; shape < 4; shape++ {
		t.Run(fmt.Sprint(shape), func(t *testing.T) {
			testConditionalLocalsTypeMatrix(t, PipeLangLanguageContractV920, true, shape&1 != 0, shape&2 != 0)
		})
	}
}

func TestV920NestedArrowMethodsCarriers(t *testing.T) {
	for shape := 0; shape < 4; shape++ {
		t.Run(fmt.Sprint(shape), func(t *testing.T) {
			testConditionalLocalsCarrierAndHostValues(t, PipeLangLanguageContractV920, true, shape&1 != 0, shape&2 != 0)
		})
	}
}

func TestV920NestedArrowMethodsLayouts(t *testing.T) {
	for shape := 0; shape < 4; shape++ {
		t.Run(fmt.Sprint(shape), func(t *testing.T) {
			left, right := `Echo("A")`, `Echo("C")`
			if shape&1 != 0 {
				left = `Check("left",left) ? Echo("A") : Echo("B")`
			}
			if shape&2 != 0 {
				right = `Check("right",right) ? Echo("C") : Echo("D")`
			}
			expr := fmt.Sprintf(`Check("outer",outer) ? (%s) : (%s)`, left, right)
			source := `public Class Choices {public string Echo(string value)=>value;public bool Check(string path,bool value)=>value;public string Select(bool outer,bool left,bool right)=>` + expr + `;}`
			analysis, program := conditionalLocalTreeProgramVersion(t, PipeLangLanguageContractV920, source, []string{"Select", "Echo", "Check"})
			block := strings.Replace(source, "=>"+expr+";", "{return "+expr+";}", 1)
			blockAnalysis, blockProgram := conditionalLocalTreeProgramVersion(t, PipeLangLanguageContractV920, block, []string{"Select", "Echo", "Check"})
			if !reflect.DeepEqual(program, blockProgram) {
				t.Fatal("arrow/block Core mismatch")
			}
			arrowHIR, err := LowerSemanticMethodToHIR(analysis, semanticMethodNamed(t, analysis, "Select").Identity)
			if err != nil {
				t.Fatal(err)
			}
			blockHIR, err := LowerSemanticMethodToHIR(blockAnalysis, semanticMethodNamed(t, blockAnalysis, "Select").Identity)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(v920WithoutSpans(t, arrowHIR), v920WithoutSpans(t, blockHIR)) {
				t.Fatal("arrow/block typed HIR mismatch")
			}

			generated, err := gobackend.Generate(program)
			if err != nil {
				t.Fatal(err)
			}
			again, err := gobackend.Generate(blockProgram)
			if err != nil || !bytes.Equal(generated, again) {
				t.Fatal("arrow/block Go mismatch")
			}
			f := coreFunctionNamed(t, program, "Select")
			var checks, traces strings.Builder
			for mask := 0; mask < 8; mask++ {
				value := "C"
				trace := []string{"outer"}
				if mask&1 != 0 {
					value = "A"
					if shape&1 != 0 {
						trace = append(trace, "left")
						if mask&2 == 0 {
							value = "B"
						}
					}
				} else if shape&2 != 0 {
					trace = append(trace, "right")
					if mask&4 == 0 {
						value = "D"
					}
				}
				trace = append(trace, value)
				args := []coreeval.Value{}
				for bit := 0; bit < 3; bit++ {
					args = append(args, coreeval.Value{Type: f.Parameters[bit].Type, Bool: mask&(1<<bit) != 0})
				}
				got, err := coreeval.EvaluateProgram(program, f.Identity, args)
				if err != nil || !got.OK || got.Value.String != value {
					t.Fatalf("%d: %#v %v", mask, got, err)
				}
				call := fmt.Sprintf("PipeLangSelect(%t,%t,%t)", mask&1 != 0, mask&2 != 0, mask&4 != 0)
				fmt.Fprintf(&checks, "if got:=%s;got!=%q{t.Fatal(got)}\n", call, value)
				fmt.Fprintf(&traces, "v920Trace=nil;%s;if !reflect.DeepEqual(v920Trace,[]string{%s}){t.Fatal(v920Trace)}\n", call, quotedStrings(trace))
			}
			compileAndRunGeneratedGoFiles(t, generated, []byte(fmt.Sprintf("package %s\nimport \"testing\"\nfunc TestChoices(t *testing.T){%s}", gobackend.PackageName, checks.String())))
			observed := string(generated)
			for marker, probe := range map[string]string{"func PipeLangEcho(p0 string) string {": "v920Trace=append(v920Trace,p0)", "func PipeLangCheck(p0 string, p1 bool) bool {": "v920Trace=append(v920Trace,p0)"} {
				if strings.Count(observed, marker) != 1 {
					t.Fatal("missing trace marker")
				}
				observed = strings.Replace(observed, marker, marker+"\n"+probe, 1)
			}
			compileAndRunGeneratedGoFiles(t, []byte(observed), []byte(fmt.Sprintf("package %s\nimport (\"testing\";\"reflect\")\nvar v920Trace []string\nfunc TestTrace(t *testing.T){%s}", gobackend.PackageName, traces.String())))
		})
	}
}

func quotedStrings(values []string) string {
	out := []string{}
	for _, v := range values {
		out = append(out, fmt.Sprintf("%q", v))
	}
	return strings.Join(out, ",")
}

func v920WithoutSpans(t *testing.T, value any) any {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	var out any
	if err = json.Unmarshal(data, &out); err != nil {
		t.Fatal(err)
	}
	var walk func(any)
	walk = func(v any) {
		switch x := v.(type) {
		case map[string]any:
			for k, child := range x {
				if strings.Contains(k, "span") {
					delete(x, k)
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
	walk(out)
	return out
}

func TestV920NestedArrowMethodsInheritance(t *testing.T) {
	for _, source := range []string{nestedTerminalInitializersSource, nestedStraightLineInitializersSource, nestedTerminalLeafReturnsSource, nestedStraightLineReturnsSource, terminalLeafConditionalReturnsSource, conditionalReturnCompositionSource, straightLineConditionalLocalsSource, finiteConditionalLocalsSource, twoConditionalLocalsSource, `public Class Choices {public string Select(string raw,bool pick)=>pick ? raw : trim(raw);}`} {
		var baseline [][]byte
		for _, contract := range []LanguageContract{PipeLangLanguageContractV910, PipeLangLanguageContractV920} {
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
			h.LanguageContract = coreir.LanguageContractV910
			p.LanguageContract = coreir.LanguageContractV910
			projection.LanguageContract = PipeLangLanguageContractV910
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

func TestV920NestedArrowMethodsSourceRejection(t *testing.T) {
	for _, expr := range []string{
		`outer ? (left ? (right ? raw : "a") : "b") : raw`,
		`outer ? raw : (left ? raw : (right ? "a" : "b"))`,
		`(outer ? left : right) ? raw : "a"`,
		`outer ? ((left ? outer : right) ? raw : "a") : raw`,
		`outer ? trim(left ? raw : "a") : raw`,
		`trim(outer ? (left ? raw : "a") : raw)`,
		`raw ? (left ? raw : "a") : raw`,
		`outer ? (left ? true : raw) : raw`,
		`outer ? (left ? raw : ) : raw`,
		`outer ? (left ? missing : raw) : raw`,
		`outer ? (left ? propagate(raw) : raw) : raw`,
		`outer ? (left ? match(raw){some(v)=>v,none=>raw} : raw) : raw`,
	} {
		source := `public Class Choices {public string Select(string raw,bool outer,bool left,bool right)=>` + expr + `;}`
		input := semanticTestModuleSet("compiler.selfhosting", []ModuleInput{testModule("compiler.selfhosting", "arrow.pipe", source)}, nil)
		input.LanguageContract = PipeLangLanguageContractV920
		analysis := AnalyzeSemanticModuleSet(input)
		if analysis.Error() == nil {
			t.Fatalf("admitted %s", expr)
		}
		for _, d := range analysis.Diagnostics {
			if !d.Primary.IsValid() || d.Primary.File != "arrow.pipe" || d.Primary.End > len(source) {
				t.Fatalf("invalid diagnostic: %#v", d)
			}
		}
	}
	for version := 1; version <= 91; version++ {
		input := semanticTestModuleSet("compiler.selfhosting", []ModuleInput{testModule("compiler.selfhosting", "arrow.pipe", nestedArrowMethodsSource)}, nil)
		input.LanguageContract = LanguageContract(fmt.Sprintf("v0.%d.0", version))
		if AnalyzeSemanticModuleSet(input).Error() == nil {
			t.Fatalf("v%d admitted nested arrow", version)
		}
	}
}

func TestV920NestedArrowMethodsMalformedCore(t *testing.T) {
	for _, mutation := range []string{"condition", "inner type", "arm", "missing", "depth", "condition placement", "argument placement", "hidden local", "terminal", "identity", "version"} {
		t.Run(mutation, func(t *testing.T) {
			_, p := conditionalLocalTreeProgramVersion(t, PipeLangLanguageContractV920, nestedArrowMethodsSource, []string{"Select"})
			f := &p.Functions[len(p.Functions)-1]
			c := f.Body.Conditional
			inner := c.WhenTrue.Conditional
			copyExpr := func(e *coreir.Expr) *coreir.Expr {
				data, _ := json.Marshal(e)
				var out coreir.Expr
				if err := json.Unmarshal(data, &out); err != nil {
					t.Fatal(err)
				}
				return &out
			}
			switch mutation {
			case "condition":
				c.Condition = c.WhenFalse
			case "inner type":
				inner.Condition = inner.WhenFalse
			case "arm":
				inner.WhenTrue = inner.Condition
			case "missing":
				inner.WhenFalse = nil
			case "depth":
				inner.WhenTrue = copyExpr(c.WhenFalse)
			case "condition placement":
				c.Condition = &coreir.Expr{Kind: coreir.ExprConditional, Type: c.Condition.Type, Conditional: &coreir.Conditional{Condition: c.Condition, WhenTrue: c.Condition, WhenFalse: c.Condition}}
			case "argument placement":
				inner.WhenTrue = &coreir.Expr{Kind: coreir.ExprTextTrim, Type: f.ReturnType, TextTrim: &coreir.TextTrim{Value: copyExpr(c.WhenFalse)}}
			case "hidden local":
				inner.WhenTrue = &coreir.Expr{Kind: coreir.ExprImmutableLocal, Type: f.ReturnType, ImmutableLocal: &coreir.ImmutableLocal{}}
			case "terminal":
				inner.TerminalStatement = true
			case "identity":
				p.CompilerContract = "unknown"
			case "version":
				p.LanguageContract = "v0.93.0"
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
	// Core deliberately erases arrow/block spelling. Existing depth-two block Core
	// must remain valid at v0.88+ even though those source versions reject arrows.
	_, p := conditionalLocalTreeProgramVersion(t, PipeLangLanguageContractV920, nestedArrowMethodsSource, []string{"Select"})
	for v := 88; v <= 92; v++ {
		p.LanguageContract = fmt.Sprintf("v0.%d.0", v)
		if err := coreir.ValidateProgram(p); err != nil {
			t.Fatal(err)
		}
	}
	p.LanguageContract = "v0.87.0"
	if coreir.ValidateProgram(p) == nil {
		t.Fatal("depth-two Core admitted before v0.88")
	}
}

func TestV920NestedArrowMethodsComputedCarriers(t *testing.T) {
	for _, unused := range []bool{false, true} {
		t.Run(fmt.Sprint(unused), func(t *testing.T) {
			local := ""
			if unused {
				local = "Result<int,ArithmeticError> unused=a ? (b ? Make(value) : Make(value)) : (c ? Make(value) : Make(value));"
			}
			body := `a ? (b ? Make(value) : Make(0)) : (c ? Make(1) : Make(2))`
			source := `public Class Choices {public Result<int,ArithmeticError> Make(int value)=>value+1;public Result<int,ArithmeticError> Choose(int value,bool a,bool b,bool c)=>` + body + `;public Result<int,ArithmeticError> Select(int value,bool a,bool b,bool c){` + local + `return Choose(value,a,b,c);}}`

			if !unused {
				source = strings.Replace(source, "{return Choose(value,a,b,c);}", "=>Choose(value,a,b,c);", 1)
			}
			_, program := conditionalLocalTreeProgramVersion(t, PipeLangLanguageContractV920, source, []string{"Select"})
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
