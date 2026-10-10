package pipelang

import (
	"dockpipe/src/lib/pipelang/coreeval"
	"dockpipe/src/lib/pipelang/coreir"
	"dockpipe/src/lib/pipelang/gobackend"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"
)

const nestedTerminalInitializersSource = `public Class Choices {
 public string Select(string raw, bool pick, bool finish, bool enabled) {
  string first = pick ? (finish ? trim(raw) : raw) : (enabled ? "fallback" : raw);
  if(enabled){
  string second = finish && first != "" ? first + "!" : first;
  string third = enabled && second != "" ? second + "?" : second;
  return enabled ? (finish ? third : first) : (pick ? first : raw);
  }else{string other=pick ? (finish ? first : raw) : (enabled ? first : raw);return other;}
 }
}`

func TestV910NestedTerminalInitializersAdmission(t *testing.T) {
	input := semanticTestModuleSet("compiler.selfhosting", []ModuleInput{testModule("compiler.selfhosting", "nested.pipe", nestedTerminalInitializersSource)}, nil)
	input.LanguageContract = LanguageContract("v0.91.0")
	if err := AnalyzeSemanticModuleSet(input).Error(); err != nil {
		t.Fatal(err)
	}
}

func TestV910NestedTerminalInitializersTypes(t *testing.T) {
	for _, zero := range []bool{false, true} {
		t.Run(fmt.Sprint(zero), func(t *testing.T) { testConditionalLocalsTypeMatrix(t, PipeLangLanguageContractV910, zero) })
	}
	testConditionalLocalsDifferentTypes(t, PipeLangLanguageContractV910)
}
func TestV910NestedTerminalInitializersCarriers(t *testing.T) {
	for _, zero := range []bool{false, true} {
		t.Run(fmt.Sprint(zero), func(t *testing.T) { testConditionalLocalsCarrierAndHostValues(t, PipeLangLanguageContractV910, zero) })
	}
}

// Existing topology and independent path oracle gain bounded nested initializers.
func TestV910NestedTerminalInitializersLayouts(t *testing.T) {
	trees := terminalTrees(3)[1:]
	if len(trees) != 25 {
		t.Fatal("shape inventory drift")
	}
	for layout := 0; layout < 200; layout++ {
		t.Run(fmt.Sprint(layout), func(t *testing.T) {
			enterFiniteShape(t)
			testFiniteConditionalLocalsLayouts(t, PipeLangLanguageContractV910, trees[(layout/4)%25:(layout/4)%25+1], layout >= 100, layout&1 != 0, layout&2 != 0)
		})
	}
}

func TestV910NestedTerminalInitializersSourceRejection(t *testing.T) {
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
		{"depth four", "return enabled ? (finish ? third : first) : (pick ? first : raw);", "if(pick){if(finish){if(enabled){return third;}else{return raw;}}else{return raw;}}else{return raw;}"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source := strings.Replace(nestedTerminalInitializersSource, tc.before, tc.after, 1)
			if source == nestedTerminalInitializersSource {
				t.Fatal("mutation missed")
			}
			assertV910SourceRejected(t, source)
		})
	}
	for _, body := range []string{`=>true ? (false ? "a" : "b") : "c";`, `{return "ordinary";}`, `{}`, `{return true ? (false ? (true ? "a" : "b") : "c") : "d";}`} {
		assertV910SourceRejected(t, `public Class Choices {public string Select()`+body+`}`)
	}
}
func assertV910SourceRejected(t *testing.T, source string) {
	t.Helper()
	input := semanticTestModuleSet("compiler.selfhosting", []ModuleInput{testModule("compiler.selfhosting", "nested.pipe", source)}, nil)
	input.LanguageContract = PipeLangLanguageContractV910
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
func TestV910NestedTerminalInitializersMalformedCore(t *testing.T) {
	for _, mutation := range []string{"missing local", "missing initializer", "missing continuation", "missing conditional", "missing condition", "missing true", "missing false", "inner missing true", "inner condition type", "arm type", "return type", "self", "forward", "position", "duplicate", "depth three", "nested initializer", "nested condition", "inner nested condition", "terminal inner", "return argument", "initializer argument", "unknown", "initializer missing inner", "initializer missing arm", "initializer condition type", "initializer arm type", "initializer terminal marker"} {
		t.Run(mutation, func(t *testing.T) {
			_, p := conditionalLocalTreeProgramVersion(t, PipeLangLanguageContractV910, nestedStraightLineInitializersSource, []string{"Select"})
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

			}
			assertAdmissionRejected(t, p, "")
		})
	}
}
func TestV910NestedTerminalInitializersVersionBoundary(t *testing.T) {
	for version := 1; version <= 90; version++ {
		contract := LanguageContract(fmt.Sprintf("v0.%d.0", version))
		input := semanticTestModuleSet("compiler.selfhosting", []ModuleInput{testModule("compiler.selfhosting", "nested.pipe", nestedTerminalInitializersSource)}, nil)
		input.LanguageContract = contract
		if AnalyzeSemanticModuleSet(input).Error() == nil {
			t.Fatalf("%s admitted nested return", contract)
		}
		_, p := conditionalLocalTreeProgramVersion(t, PipeLangLanguageContractV910, nestedTerminalInitializersSource, []string{"Select"})
		if err := coreir.ValidateFunction(p.Functions[0]); err != nil {
			t.Fatal(err)
		}
		p.LanguageContract = string(contract)
		assertAdmissionRejected(t, p, "")
	}
	for _, contract := range []string{"v0.999.0", "unknown"} {
		_, p := conditionalLocalTreeProgramVersion(t, PipeLangLanguageContractV910, nestedTerminalInitializersSource, []string{"Select"})
		p.LanguageContract = contract
		assertAdmissionRejected(t, p, "")
	}
}
func TestV910NestedTerminalInitializersRepresentations(t *testing.T) {
	for _, source := range []string{nestedStraightLineInitializersSource, `public Class Choices {public string Select(string raw,bool pick,bool finish,bool enabled){return enabled ? (finish ? raw : "a") : (pick ? "b" : "c");}}`} {
		a, p := conditionalLocalTreeProgramVersion(t, PipeLangLanguageContractV910, source, []string{"Select"})
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
func TestV910NestedTerminalInitializersInheritance(t *testing.T) {
	for _, source := range []string{nestedStraightLineInitializersSource, nestedTerminalLeafReturnsSource, nestedStraightLineReturnsSource, terminalLeafConditionalReturnsSource, conditionalReturnCompositionSource, straightLineConditionalLocalsSource, finiteConditionalLocalsSource, twoConditionalLocalsSource, `public Class Choices {public string Select(string raw,bool pick)=>pick ? raw : trim(raw);}`} {
		var baseline [][]byte
		for _, contract := range []LanguageContract{PipeLangLanguageContractV900, PipeLangLanguageContractV910} {
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

func TestV910NestedTerminalInitializersComputedCarriers(t *testing.T) {
	for _, unused := range []bool{false, true} {
		t.Run(fmt.Sprint(unused), func(t *testing.T) {
			local := ""
			if unused {
				local = "Result<int,ArithmeticError> unused=a ? (b ? Make(value) : Make(value)) : (c ? Make(value) : Make(value));"
			}
			source := `public Class Choices {public Result<int,ArithmeticError> Make(int value)=>value+1;
 public Result<int,ArithmeticError> Select(int value,bool a,bool b,bool c){` + local + `if(a){Result<int,ArithmeticError> selected=a ? (b ? Make(value) : Make(0)) : (c ? Make(1) : Make(2)); return selected;}else{Result<int,ArithmeticError> other=a ? (b ? Make(value) : Make(0)) : (c ? Make(1) : Make(2));return other;}}}`
			_, program := conditionalLocalTreeProgramVersion(t, PipeLangLanguageContractV910, source, []string{"Select"})
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

func TestV910NestedTerminalInitializersRefusesEmbeddedLocals(t *testing.T) {
	for _, placement := range []string{"return arm", "inner condition", "outer condition", "initializer arm", "nested initializer arm", "nested initializer condition"} {
		t.Run(placement, func(t *testing.T) {
			_, p := conditionalLocalTreeProgramVersion(t, PipeLangLanguageContractV910, nestedStraightLineInitializersSource, []string{"Select"})
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

func v910WrapSelect(source string) string {
	start := strings.Index(source, " Select(")
	open := strings.Index(source[start:], "{") + start
	depth := 1
	end := open + 1
	for ; depth > 0; end++ {
		if source[end] == '{' {
			depth++
		}
		if source[end] == '}' {
			depth--
		}
	}
	body := source[open+1 : end-1]
	return source[:open+1] + "if(" + v890TypeCondition(source) + "){" + body + "}else{" + body + "}" + source[end-1:]
}

// Exercise the new tree admission itself, separately from inherited straight blocks.
func TestV910NestedTerminalInitializersTreeCoreRefusal(t *testing.T) {
	for _, scope := range []string{"root", "true", "false"} {
		for _, mutation := range []string{"hidden arm", "hidden condition", "depth three", "terminal initializer", "missing arm", "wrong type", "forward", "statement condition", "depth four"} {
			t.Run(scope+"/"+mutation, func(t *testing.T) {
				_, p := conditionalLocalTreeProgramVersion(t, PipeLangLanguageContractV910, nestedTerminalInitializersSource, []string{"Select"})
				f := &p.Functions[0]
				root := f.Body.ImmutableLocal
				tree := root.Return.Conditional
				local := root
				if scope == "true" {
					local = tree.WhenTrue.ImmutableLocal
				}
				if scope == "false" {
					local = tree.WhenFalse.ImmutableLocal
				}
				choice := local.Initializer.Conditional
				switch mutation {
				case "hidden arm", "hidden condition":
					target := &choice.WhenTrue
					if mutation == "hidden condition" {
						target = &choice.Condition
					}
					value := *target
					position := local.Position
					*target = &coreir.Expr{Kind: coreir.ExprImmutableLocal, Type: value.Type, ImmutableLocal: &coreir.ImmutableLocal{Name: "hidden", Position: position, Type: value.Type, Initializer: value, Return: &coreir.Expr{Kind: coreir.ExprReference, Type: value.Type, Parameter: &position}}}
					if err := coreir.ValidateFunction(*f); err != nil {
						t.Fatalf("internal Core narrowed: %v", err)
					}
				case "depth three":
					leaf := choice.WhenTrue
					for i := 0; i < 2; i++ {
						leaf = &coreir.Expr{Kind: coreir.ExprConditional, Type: leaf.Type, Conditional: &coreir.Conditional{Condition: choice.Condition, WhenTrue: leaf, WhenFalse: choice.WhenFalse}}
					}
					choice.WhenTrue = leaf
				case "terminal initializer":
					choice.TerminalStatement = true
				case "missing arm":
					choice.WhenFalse = nil
				case "wrong type":
					choice.WhenFalse = choice.Condition
				case "forward":
					position := local.Position
					choice.WhenFalse = &coreir.Expr{Kind: coreir.ExprReference, Type: local.Type, Parameter: &position}
				case "statement condition":
					tree.Condition = &coreir.Expr{Kind: coreir.ExprConditional, Type: tree.Condition.Type, Conditional: &coreir.Conditional{Condition: tree.Condition, WhenTrue: tree.Condition, WhenFalse: tree.Condition}}
				case "depth four":
					tail := tree.WhenFalse
					for i := 0; i < 3; i++ {
						tail = &coreir.Expr{Kind: coreir.ExprConditional, Type: tail.Type, Conditional: &coreir.Conditional{TerminalStatement: true, Condition: tree.Condition, WhenTrue: tail, WhenFalse: tree.WhenFalse}}
					}
					tree.WhenFalse = tail
				}
				assertAdmissionRejected(t, p, "")
			})
		}
	}
}

func TestV910NestedTerminalInitializersTreeRepresentations(t *testing.T) {
	a, p := conditionalLocalTreeProgramVersion(t, PipeLangLanguageContractV910, nestedTerminalInitializersSource, []string{"Select"})
	h, err := LowerSemanticMethodToHIR(a, semanticMethodNamed(t, a, "Select").Identity)
	if err != nil {
		t.Fatal(err)
	}
	if h.LanguageContract != coreir.LanguageContractV910 {
		t.Fatal("HIR version")
	}
	root := p.Functions[0].Body.ImmutableLocal
	if root == nil || root.Initializer.Conditional.TerminalStatement || !root.Return.Conditional.TerminalStatement {
		t.Fatal("root initializer/statement markers")
	}
	branch := root.Return.Conditional.WhenFalse.ImmutableLocal
	if branch == nil || branch.Initializer.Conditional.TerminalStatement || branch.Initializer.Conditional.WhenTrue.Conditional.TerminalStatement {
		t.Fatal("branch initializer markers")
	}
	// A valid tree must be accepted before each independent malformed-AST probe.
	for _, scope := range []string{"root", "true", "false"} {
		for _, condition := range []bool{false, true} {
			a, _ := conditionalLocalTreeProgramVersion(t, PipeLangLanguageContractV910, nestedTerminalInitializersSource, []string{"Select"})
			body := a.Program.Classes[0].Methods[0].Body
			if !validNestedTerminalInitializers(body) {
				t.Fatal("valid AST refused")
			}
			first := body.(*ImmutableLocalExpr)
			tree := first.Return.(*ConditionalExpr)
			local := first
			if scope == "true" {
				local = tree.WhenTrue.(*ImmutableLocalExpr)
			}
			if scope == "false" {
				local = tree.WhenFalse.(*ImmutableLocalExpr)
			}
			choice := local.Initializer.(*ConditionalExpr)
			target := &choice.WhenTrue
			if condition {
				target = &choice.Condition
			}
			hidden := *local
			hidden.Name = "hidden"
			hidden.Initializer = *target
			hidden.Return = *target
			*target = &hidden
			if validNestedTerminalInitializers(body) {
				t.Fatal("hidden AST local admitted")
			}
		}
	}
}

func TestV910NestedTerminalInitializersDependentConditions(t *testing.T) {
	source := `public Class Choices {public string Select(string raw,bool a,bool b,bool c){
 bool first=a ? (b ? c : b) : (c ? b : c);
 if(first){bool next=b ? (a ? c : a) : (c ? a : c);if(next){return raw;}else{return "next";}}
 else{bool next=c ? (a ? b : a) : (b ? a : b);if(next){return "other";}else{return "none";}}}}`
	_, p := conditionalLocalTreeProgramVersion(t, PipeLangLanguageContractV910, source, []string{"Select"})
	f := p.Functions[0]
	var checks strings.Builder
	for mask := 0; mask < 8; mask++ {
		a, b, c := mask&1 != 0, mask&2 != 0, mask&4 != 0
		want := "none"
		first := false
		if a {
			if b {
				first = c
			} else {
				first = b
			}
		} else {
			if c {
				first = b
			} else {
				first = c
			}
		}
		if first {
			next := false
			if b {
				if a {
					next = c
				} else {
					next = a
				}
			} else {
				if c {
					next = a
				} else {
					next = c
				}
			}
			want = "next"
			if next {
				want = "raw"
			}
		} else {
			next := false
			if c {
				if a {
					next = b
				} else {
					next = a
				}
			} else {
				if b {
					next = a
				} else {
					next = b
				}
			}
			if next {
				want = "other"
			}
		}
		got, err := coreeval.EvaluateProgram(p, f.Identity, []coreeval.Value{{Type: f.Parameters[0].Type, String: "raw"}, {Type: f.Parameters[1].Type, Bool: a}, {Type: f.Parameters[2].Type, Bool: b}, {Type: f.Parameters[3].Type, Bool: c}})
		if err != nil || !got.OK || got.Value.String != want {
			t.Fatalf("%d: %#v %v want %s", mask, got, err, want)
		}
		fmt.Fprintf(&checks, "if got:=PipeLangSelect(\"raw\",%t,%t,%t);got!=%q {t.Fatal(got)}\n", a, b, c, want)
	}
	g, err := gobackend.Generate(p)
	if err != nil {
		t.Fatal(err)
	}
	compileAndRunGeneratedGoFiles(t, g, []byte(fmt.Sprintf("package %s\nimport \"testing\"\nfunc TestConditions(t *testing.T){%s}", gobackend.PackageName, checks.String())))
	for _, bad := range []string{
		strings.Replace(source, "return \"other\";", "return first ? raw : next;", 1),
		strings.Replace(nestedTerminalInitializersSource, "return other;", "return third;", 1),
	} {
		assertV910SourceRejected(t, bad)
	}
}

func TestV910NestedTerminalInitializersReturnShapes(t *testing.T) {
	for shape := -1; shape < 4; shape++ {
		t.Run(fmt.Sprint(shape), func(t *testing.T) {
			left, right := `selected+"T"`, `selected+"F"`
			if shape >= 0 && shape&1 != 0 {
				left = `(yes ? selected+"TY" : selected+"TN")`
			}
			if shape >= 0 && shape&2 != 0 {
				right = `(no ? selected+"FY" : selected+"FN")`
			}
			returned := "choose ? " + left + " : " + right
			if shape < 0 {
				returned = "selected"
			}
			source := `public Class Choices {public string Select(bool branch,bool pick,bool ia,bool ib,bool choose,bool yes,bool no){
 string root=pick ? (ia ? "a" : "b") : (ib ? "c" : "d");
 if(branch){string selected=pick ? (ia ? root+"A" : root+"B") : (ib ? root+"C" : root+"D");return ` + returned + `;}
 else{string selected=pick ? (ia ? root+"E" : root+"F") : (ib ? root+"G" : root+"H");return ` + returned + `;}}}`
			_, p := conditionalLocalTreeProgramVersion(t, PipeLangLanguageContractV910, source, []string{"Select"})
			f := p.Functions[0]
			var checks strings.Builder
			for mask := 0; mask < 128; mask++ {
				index := 3
				if mask&2 != 0 {
					index = 1
					if mask&4 != 0 {
						index = 0
					}
				} else if mask&8 != 0 {
					index = 2
				}
				selected := string("abcd"[index])
				offset := 0
				if mask&1 == 0 {
					offset = 4
				}
				selected += string("ABCDEFGH"[index+offset])
				want := selected
				if shape >= 0 {
					if mask&16 != 0 {
						want += "T"
						if shape&1 != 0 {
							if mask&32 != 0 {
								want += "Y"
							} else {
								want += "N"
							}
						}
					} else {
						want += "F"
						if shape&2 != 0 {
							if mask&64 != 0 {
								want += "Y"
							} else {
								want += "N"
							}
						}
					}
				}
				var args []coreeval.Value
				var call []string
				for bit := 0; bit < 7; bit++ {
					on := mask&(1<<bit) != 0
					args = append(args, coreeval.Value{Type: f.Parameters[bit].Type, Bool: on})
					call = append(call, fmt.Sprint(on))
				}
				got, err := coreeval.EvaluateProgram(p, f.Identity, args)
				if err != nil || !got.OK || got.Value.String != want {
					t.Fatalf("%d: %#v %v want %s", mask, got, err, want)
				}
				fmt.Fprintf(&checks, "if got:=PipeLangSelect(%s);got!=%q {t.Fatal(got)}\n", strings.Join(call, ","), want)
			}
			g, err := gobackend.Generate(p)
			if err != nil {
				t.Fatal(err)
			}
			compileAndRunGeneratedGoFiles(t, g, []byte(fmt.Sprintf("package %s\nimport \"testing\"\nfunc TestReturns(t *testing.T){%s}", gobackend.PackageName, checks.String())))
		})
	}
}
