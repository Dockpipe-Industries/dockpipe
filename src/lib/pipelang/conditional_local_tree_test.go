package pipelang

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"testing"

	"dockpipe/src/lib/pipelang/coreeval"
	"dockpipe/src/lib/pipelang/coreir"
	"dockpipe/src/lib/pipelang/gobackend"
)

func conditionalLocalTreeProgram(t *testing.T, source string, methods []string) (*Analysis, coreir.Program) {
	t.Helper()
	input := semanticTestModuleSet("compiler.selfhosting", []ModuleInput{testModule("compiler.selfhosting", "conditional-local.pipe", source)}, nil)
	input.LanguageContract = PipeLangLanguageContractV820
	analysis := AnalyzeSemanticModuleSet(input)
	if err := analysis.Error(); err != nil {
		t.Fatal(err)
	}
	projection, err := BuildSemanticProjection(analysis)
	if err != nil {
		t.Fatal(err)
	}
	if projection.LanguageContract != PipeLangLanguageContractV820 || projection.CompilerContract != PipeLangCompilerContract || projection.Schema != PipeLangSemanticProjectionVersion {
		t.Fatal("public identity drift")
	}
	var program coreir.Program
	seen := map[string]bool{}
	for _, method := range methods {
		typed, err := LowerSemanticMethodToHIR(analysis, semanticMethodNamed(t, analysis, method).Identity)
		if err != nil {
			t.Fatal(err)
		}
		if typed.LanguageContract != coreir.LanguageContractV820 {
			t.Fatal("HIR contract drift")
		}
		lowered, err := LowerHIRToCore(typed)
		if err != nil {
			t.Fatal(err)
		}
		if program.CompilerContract == "" {
			program = lowered
			program.Functions = nil
		}
		for _, function := range lowered.Functions {
			if !seen[function.Identity.Path] {
				program.Functions = append(program.Functions, function)
				seen[function.Identity.Path] = true
			}
		}
	}
	sort.Slice(program.Functions, func(i, j int) bool { return program.Functions[i].Identity.Path < program.Functions[j].Identity.Path })
	if err := coreir.ValidateProgram(program); err != nil {
		t.Fatal(err)
	}
	return analysis, program
}

func conditionalTreeScopes(tree *terminalTree, path string) []string {
	paths := []string{path}
	if tree != nil {
		paths = append(paths, conditionalTreeScopes(tree.yes, path+"T")...)
		paths = append(paths, conditionalTreeScopes(tree.no, path+"F")...)
	}
	return paths
}

// Layout 0 has only the chosen local; 1 surrounds it with ordered locals; 2
// leaves the chosen local unused after a preceding initializer.
func conditionalTreeMethod(tree *terminalTree, target string, layout int, name string) string {
	var emit func(*terminalTree, string, string) string
	emit = func(node *terminalTree, path, value string) string {
		prefix := ""
		if layout > 0 {
			prefix += fmt.Sprintf("string b%s = Echo(%s + %q);", path, value, path+"B")
			value = "b" + path
		}
		if path == target {
			prefix += fmt.Sprintf("string chosen%s = Check(%q,pick) ? Echo(%s + %q) : Echo(%s + %q);", path, "Q:"+path, value, "T"+path, value, "F"+path)
			if layout != 2 {
				value = "chosen" + path
			}
		}
		if layout == 1 {
			prefix += fmt.Sprintf("string a%s = Echo(%s + %q);", path, value, path+"A")
			value = "a" + path
		}
		if node == nil {
			return prefix + fmt.Sprintf("return %s + %q;", value, ":"+path)
		}
		return prefix + fmt.Sprintf("if (Check(%q,d%d)) {%s} else {%s}", path, len(path)-1, emit(node.yes, path+"T", value), emit(node.no, path+"F", value))
	}
	return fmt.Sprintf("public string %s(string raw,bool d0,bool d1,bool d2,bool pick) {%s}\n", name, emit(tree, "R", "raw"))
}

func conditionalTreeExpected(tree *terminalTree, target string, layout, mask int) (string, []string) {
	node, path, value := tree, "R", "value"
	var trace []string
	for depth := 0; ; depth++ {
		if layout > 0 {
			value += path + "B"
			trace = append(trace, "E:"+value)
		}
		if path == target {
			trace = append(trace, "C:Q:"+path)
			arm := "F"
			if mask&8 != 0 {
				arm = "T"
			}
			selected := value + arm + path
			trace = append(trace, "E:"+selected)
			if layout != 2 {
				value = selected
			}
		}
		if layout == 1 {
			value += path + "A"
			trace = append(trace, "E:"+value)
		}
		if node == nil {
			return value + ":" + path, trace
		}
		trace = append(trace, "C:"+path)
		if mask&(1<<depth) != 0 {
			node = node.yes
			path += "T"
		} else {
			node = node.no
			path += "F"
		}
	}
}

func TestV820ConditionalLocalAllShapesScopesAndPaths(t *testing.T) {
	trees := terminalTrees(3)[1:]
	if len(trees) != 25 {
		t.Fatal("tree inventory drift")
	}
	for index, tree := range trees {
		t.Run(fmt.Sprint(index), func(t *testing.T) {
			var source strings.Builder
			source.WriteString(`public Class Choices { public string Echo(string value) => value; public bool Check(string path,bool value) => value;`)
			var methods []string
			for _, scope := range conditionalTreeScopes(tree, "R") {
				for layout := 0; layout < 3; layout++ {
					name := fmt.Sprintf("Select%s%d", scope, layout)
					methods = append(methods, name)
					source.WriteString(conditionalTreeMethod(tree, scope, layout, name))
				}
			}
			source.WriteString("}")
			analysis, program := conditionalLocalTreeProgram(t, source.String(), methods)
			againAnalysis, again := conditionalLocalTreeProgram(t, source.String(), methods)
			firstJSON, _ := json.Marshal(program)
			againJSON, _ := json.Marshal(again)
			if !bytes.Equal(firstJSON, againJSON) {
				t.Fatal("Core artifact drift")
			}
			projection, _ := BuildSemanticProjection(analysis)
			againProjection, _ := BuildSemanticProjection(againAnalysis)
			firstJSON, _ = json.Marshal(projection)
			againJSON, _ = json.Marshal(againProjection)
			if !bytes.Equal(firstJSON, againJSON) {
				t.Fatal("semantic artifact drift")
			}
			generated, err := gobackend.Generate(program)
			if err != nil {
				t.Fatal(err)
			}
			repeated, err := gobackend.Generate(again)
			if err != nil || !bytes.Equal(generated, repeated) {
				t.Fatal("Go artifact drift")
			}
			var cases, orders strings.Builder
			for _, scope := range conditionalTreeScopes(tree, "R") {
				for layout := 0; layout < 3; layout++ {
					name := fmt.Sprintf("Select%s%d", scope, layout)
					function := coreFunctionNamed(t, program, name)
					// Every method retains both kinds of conditionals in normalized Core.
					if countConditionalExpressionsInCore(function.Body) != coreConditionalCount(function.Body)+1 {
						t.Fatal("value conditional missing")
					}
					for mask := 0; mask < 16; mask++ {
						want, trace := conditionalTreeExpected(tree, scope, layout, mask)
						args := []coreeval.Value{{Type: function.Parameters[0].Type, String: "value"}}
						for bit := 0; bit < 4; bit++ {
							args = append(args, coreeval.Value{Type: function.Parameters[bit+1].Type, Bool: mask&(1<<bit) != 0})
						}
						outcome, err := coreeval.EvaluateProgram(program, function.Identity, args)
						if err != nil || !outcome.OK || outcome.Value.String != want {
							t.Fatalf("%s mask%d: %#v %v want %q", name, mask, outcome, err, want)
						}
						call := fmt.Sprintf("PipeLang%s(\"value\",%t,%t,%t,%t)", name, mask&1 != 0, mask&2 != 0, mask&4 != 0, mask&8 != 0)
						fmt.Fprintf(&cases, "if got:=%s;got!=%q {t.Fatalf(%q,got)}\n", call, want, name+" value: %q")
						var quoted []string
						for _, event := range trace {
							quoted = append(quoted, fmt.Sprintf("%q", event))
						}
						fmt.Fprintf(&orders, "v820Trace=nil\n%s\nif !reflect.DeepEqual(v820Trace,[]string{%s}){t.Fatalf(%q,v820Trace)}\n", call, strings.Join(quoted, ","), name+" order: %v")
					}
				}
			}
			compileAndRunGeneratedGoFiles(t, generated, []byte(fmt.Sprintf("package %s\nimport \"testing\"\nfunc TestValues(t *testing.T){%s}", gobackend.PackageName, cases.String())))
			observed := string(generated)
			for marker, probe := range map[string]string{
				"func PipeLangEcho(p0 string) string {":         "v820Trace=append(v820Trace,\"E:\"+p0)",
				"func PipeLangCheck(p0 string, p1 bool) bool {": "v820Trace=append(v820Trace,\"C:\"+p0)",
			} {
				if strings.Count(observed, marker) != 1 {
					t.Fatal("instrumentation target absent")
				}
				observed = strings.Replace(observed, marker, marker+"\n"+probe, 1)
			}
			compileAndRunGeneratedGoFiles(t, []byte(observed), []byte(fmt.Sprintf("package %s\nimport (\"testing\";\"reflect\")\nvar v820Trace []string\nfunc TestOrder(t *testing.T){%s}", gobackend.PackageName, orders.String())))
		})
	}
}

func countConditionalExpressionsInCore(expression coreir.Expr) int {
	data, _ := json.Marshal(expression)
	return bytes.Count(data, []byte(`"kind":"conditional"`))
}

const conditionalLocalRulesSource = `public Class Rules {
 public string Select(string raw,bool outer,bool inner,bool deep,bool pick) {
  string root = raw;
  if (outer) {
   string branch = root;
   if (inner) {
    string before = branch;
    string chosen = pick ? before : raw;
    string after = chosen;
    if (deep) { return after; } else { return raw; }
   } else { return branch; }
  } else { return root; }
 }
}`

func TestV820ConditionalLocalSourceRejection(t *testing.T) {
	for _, tc := range []struct{ name, before, after string }{
		{"condition type", "pick ? before : raw", "raw ? before : raw"},
		{"arm type", "pick ? before : raw", "pick ? before : true"},
		{"local type", "string chosen", "bool chosen"},
		{"self reference", "pick ? before : raw", "pick ? chosen : raw"},
		{"forward reference", "pick ? before : raw", "pick ? after : raw"},
		{"ancestor shadow", "string chosen", "string root"},
		{"duplicate", "string after = chosen;", "string chosen = raw; string after = chosen;"},
		{"escaping binding", "return root;", "return chosen;"},
		{"sibling binding", "pick ? before : raw", "pick ? sibling : raw"},
		{"nested choice", "pick ? before : raw", "pick ? (inner ? before : raw) : raw"},
		{"second in sibling", "return branch;", "string second = pick ? branch : raw; return second;"},
		{"nested in call", "pick ? before : raw", "trim(pick ? before : raw)"},
		{"return placement", "string chosen = pick ? before : raw;", "string chosen = raw;"},
		{"terminal condition placement", "if (deep)", "if (pick ? deep : inner)"},
		{"propagation operand", "pick ? before : raw", "pick ? propagate(before) : raw"},
		{"match operand", "pick ? before : raw", "pick ? match(before){ some(item) => item, none => raw } : raw"},
		{"depth four", "return after;", "if (pick) { return after; } else { return raw; }"},
		{"assignment", "string after = chosen;", "chosen = raw; string after = chosen;"},
		{"inference", "string chosen", "var chosen"},
		{"fallthrough", "return after;", ""},
		{"early return", "string before = branch;", "return raw; string before = branch;"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source := strings.Replace(conditionalLocalRulesSource, tc.before, tc.after, 1)
			if tc.name == "terminal condition placement" {
				source = strings.Replace(source, "string chosen = pick ? before : raw;", "string chosen = raw;", 1)
			}
			if tc.name == "return placement" {
				source = strings.Replace(source, "return after;", "return pick ? after : raw;", 1)
			}
			if source == conditionalLocalRulesSource {
				t.Fatal("mutation missed")
			}
			input := semanticTestModuleSet("compiler.selfhosting", []ModuleInput{testModule("compiler.selfhosting", "invalid.pipe", source)}, nil)
			input.LanguageContract = PipeLangLanguageContractV820
			err := AnalyzeSemanticModuleSet(input).Error()
			diagnostics, ok := AsDiagnostics(err)
			if err == nil || !ok || len(diagnostics) == 0 {
				t.Fatal("missing source diagnostic")
			}
			for _, d := range diagnostics {
				if !d.Primary.IsValid() || d.Primary.File != "invalid.pipe" || d.Primary.End > len(source) {
					t.Fatalf("invalid diagnostic span: %#v", d)
				}
			}
		})
	}
}

func rulesCoreLocal(program *coreir.Program) (*coreir.Expr, *coreir.Expr) {
	root := &program.Functions[0].Body
	chosen := root.ImmutableLocal.Return.Conditional.WhenTrue.ImmutableLocal.Return.Conditional.WhenTrue.ImmutableLocal.Return
	return root, chosen
}

func TestV820ConditionalLocalMalformedCore(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(*coreir.Expr, *coreir.Expr)
	}{
		{"missing initializer", func(_, l *coreir.Expr) { l.ImmutableLocal.Initializer = nil }},
		{"missing choice payload", func(_, l *coreir.Expr) { l.ImmutableLocal.Initializer.Conditional = nil }},
		{"missing condition", func(_, l *coreir.Expr) { l.ImmutableLocal.Initializer.Conditional.Condition = nil }},
		{"missing arm", func(_, l *coreir.Expr) { l.ImmutableLocal.Initializer.Conditional.WhenFalse = nil }},
		{"terminal initializer", func(_, l *coreir.Expr) { l.ImmutableLocal.Initializer.Conditional.TerminalStatement = true }},
		{"condition type", func(_, l *coreir.Expr) { c := l.ImmutableLocal.Initializer.Conditional; c.Condition = c.WhenTrue }},
		{"arm type", func(_, l *coreir.Expr) { c := l.ImmutableLocal.Initializer.Conditional; c.WhenFalse = c.Condition }},
		{"position", func(_, l *coreir.Expr) { l.ImmutableLocal.Position = 999 }},
		{"shadow", func(_, l *coreir.Expr) { l.ImmutableLocal.Name = "root" }},
		{"self reference", func(_, l *coreir.Expr) {
			l.ImmutableLocal.Initializer.Conditional.WhenTrue = l.ImmutableLocal.Return.ImmutableLocal.Initializer
		}},
		{"second initializer", func(_, l *coreir.Expr) {
			l.ImmutableLocal.Return.ImmutableLocal.Initializer = l.ImmutableLocal.Initializer
		}},
		{"nested choice", func(_, l *coreir.Expr) {
			copy := *l.ImmutableLocal.Initializer
			arms := *copy.Conditional
			copy.Conditional = &arms
			l.ImmutableLocal.Initializer.Conditional.WhenFalse = &copy
		}},
		{"return placement", func(_, l *coreir.Expr) {
			choice := l.ImmutableLocal.Initializer
			l.ImmutableLocal.Initializer = choice.Conditional.WhenTrue
			l.ImmutableLocal.Return.ImmutableLocal.Return.Conditional.WhenTrue = choice
		}},
		{"argument placement", func(_, l *coreir.Expr) {
			choice := l.ImmutableLocal.Initializer
			l.ImmutableLocal.Initializer = &coreir.Expr{Kind: coreir.ExprTextTrim, Type: choice.Type, TextTrim: &coreir.TextTrim{Value: choice}}
		}},
		{"condition placement", func(_, l *coreir.Expr) {
			choice := l.ImmutableLocal.Initializer.Conditional
			branch := l.ImmutableLocal.Return.ImmutableLocal.Return.Conditional
			l.ImmutableLocal.Initializer = choice.WhenTrue
			branch.Condition = &coreir.Expr{Kind: coreir.ExprConditional, Type: branch.Condition.Type, Conditional: &coreir.Conditional{Condition: choice.Condition, WhenTrue: branch.Condition, WhenFalse: choice.Condition}}
		}},
		{"escaping scope", func(root, l *coreir.Expr) {
			root.ImmutableLocal.Return.Conditional.WhenFalse = l.ImmutableLocal.Return.ImmutableLocal.Initializer
		}},
		{"depth four", func(_, l *coreir.Expr) {
			b := l.ImmutableLocal.Return.ImmutableLocal.Return.Conditional
			copy := *l.ImmutableLocal.Return.ImmutableLocal.Return
			branch := *copy.Conditional
			copy.Conditional = &branch
			b.WhenTrue = &copy
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, program := conditionalLocalTreeProgram(t, conditionalLocalRulesSource, []string{"Select"})
			root, local := rulesCoreLocal(&program)
			tc.mutate(root, local)
			if err := coreir.ValidateProgram(program); err == nil {
				t.Fatal("invalid Core accepted")
			}
			if _, err := gobackend.Generate(program); err == nil {
				t.Fatal("backend accepted invalid Core")
			}
			function := program.Functions[0]
			var args []coreeval.Value
			for _, p := range function.Parameters {
				args = append(args, admissionValue(p.Type))
			}
			if _, err := coreeval.EvaluateProgram(program, function.Identity, args); err == nil {
				t.Fatal("evaluator accepted invalid Core")
			}
		})
	}
}

func TestV820ConditionalLocalVersionAndInheritance(t *testing.T) {
	input := semanticTestModuleSet("compiler.selfhosting", []ModuleInput{testModule("compiler.selfhosting", "old.pipe", conditionalLocalRulesSource)}, nil)
	input.LanguageContract = PipeLangLanguageContractV810
	if err := AnalyzeSemanticModuleSet(input).Error(); err == nil {
		t.Fatal("old source admitted new composition")
	}
	_, program := conditionalLocalTreeProgram(t, conditionalLocalRulesSource, []string{"Select"})
	program.LanguageContract = coreir.LanguageContractV810
	if err := coreir.ValidateProgram(program); err == nil {
		t.Fatal("old Core admitted new composition")
	}
	if _, err := gobackend.Generate(program); err == nil {
		t.Fatal("old backend admitted new composition")
	}
	for _, tree := range terminalTrees(3)[1:] {
		source := terminalTreeSource(tree, true, true)
		_, _, old := terminalTreeProgram(t, source)
		_, current := conditionalLocalTreeProgram(t, source, []string{"Select"})
		oldGo, err := gobackend.Generate(old)
		if err != nil {
			t.Fatal(err)
		}
		newGo, err := gobackend.Generate(current)
		if err != nil || !bytes.Equal(oldGo, newGo) {
			t.Fatal("inherited terminal bytes changed")
		}
	}
	for _, source := range []string{
		`public Class Root {public string Select(string raw,bool pick){string value=pick ? raw : trim(raw);if(pick){return value;}else{return raw;}}}`,
		`public Class Root {public string Select(string raw,bool pick){string value=pick ? raw : trim(raw);return value;}}`,
		`public Class Root {public Optional<string> Select(Optional<string> value)=>some(propagate(value));}`,
		`public Class Root {public string Select(Optional<string> value)=>match(value){some(item)=>item,none=>"empty"};}`,
		`public Class Root {public string Echo(string value)=>value;public string Select(string value)=>Echo(trim(value));}`,
		`public Class Root {public Result<int,ArithmeticError> Echo(Result<int,ArithmeticError> value)=>value;public Result<int,ArithmeticError> Select(Result<int,ArithmeticError> value)=>Echo(value);}`,
	} {
		old := reviewCore(t, PipeLangLanguageContractV810, source, "Select")
		current := reviewCore(t, PipeLangLanguageContractV820, source, "Select")
		oldGo, err := gobackend.Generate(old)
		if err != nil {
			t.Fatal(err)
		}
		newGo, err := gobackend.Generate(current)
		if err != nil || !bytes.Equal(oldGo, newGo) {
			t.Fatal("inherited expression bytes changed")
		}
		admissionOutcomes(t, current)
		reviewCompileGenerated(t, current)
	}
}

func conditionalTypedValue(typ coreir.Type, right bool) coreeval.Value {
	v := admissionValue(typ)
	switch typ.Kind {
	case coreir.TypePrimitive:
		switch typ.Primitive {
		case coreir.PrimitiveString:
			v.String = "left"
			if right {
				v.String = "right"
			}
		case coreir.PrimitiveBool:
			v.Bool = !right
		}
	case coreir.TypeNumeric:
		v.Int = 11
		v.Float = 1.25
		if right {
			v.Int = 22
			v.Float = 2.5
		}
	case coreir.TypeRecord:
		for i, f := range typ.Record.Fields {
			v.Record[i] = conditionalTypedValue(f.Type, right)
		}
	case coreir.TypeList:
		v.List = []coreeval.Value{conditionalTypedValue(typ.List.Element, right)}
	case coreir.TypeOptional:
		payload := conditionalTypedValue(typ.Optional.Value, right)
		v.Optional = &coreeval.OptionalValue{Present: true, Value: &payload}
	case coreir.TypeResult:
		v.Result = &coreeval.Outcome{OK: true, Value: conditionalTypedValue(typ.Result.Success, right)}
	}
	return v
}

func TestV820ConditionalLocalTypeMatrix(t *testing.T) {
	for _, typ := range []string{"string", "bool", "int", "float", "Row", "List<Row>", "Optional<string>", "Optional<Row>", "Result<string,string>", "Result<List<Row>,string>", "Result<int,ArithmeticError>", "Result<float,ArithmeticError>"} {
		t.Run(typ, func(t *testing.T) {
			source := fmt.Sprintf(`public Record Row {public string Name;} public Class Typed {
   public %[1]s Echo(%[1]s value)=>value;
   public %[1]s Select(%[1]s left,%[1]s right,bool pick,bool outer,bool inner){
    %[1]s shared=left;
    if(outer){%[1]s chosen=pick ? Echo(shared) : Echo(right);if(inner){return chosen;}else{return chosen;}}
    else{return right;}
   }
  }`, typ)
			_, program := conditionalLocalTreeProgram(t, source, []string{"Select"})
			function := coreFunctionNamed(t, program, "Select")
			echo := coreFunctionNamed(t, program, "Echo")
			left := conditionalTypedValue(function.Parameters[0].Type, false)
			right := conditionalTypedValue(function.Parameters[1].Type, true)
			for mask := 0; mask < 8; mask++ {
				args := []coreeval.Value{left, right}
				for bit := 0; bit < 3; bit++ {
					args = append(args, coreeval.Value{Type: function.Parameters[bit+2].Type, Bool: mask&(1<<bit) != 0})
				}
				value := right
				if mask&1 != 0 && mask&2 != 0 {
					value = left
				}
				want, err := coreeval.EvaluateProgram(program, echo.Identity, []coreeval.Value{value})
				if err != nil {
					t.Fatal(err)
				}
				got, err := coreeval.EvaluateProgram(program, function.Identity, args)
				if err != nil || !reflect.DeepEqual(got, want) {
					t.Fatalf("typed choice mask %d: %#v %v", mask, got, err)
				}
			}
			generated, err := gobackend.Generate(program)
			if err != nil {
				t.Fatal(err)
			}
			goType, payload := generatedArgumentTypes(t, generated)
			l, r := `"left"`, `"right"`
			switch typ {
			case "bool":
				l, r = "true", "false"
			case "int":
				l, r = "int64(11)", "int64(22)"
			case "float":
				l, r = "float64(1.25)", "float64(2.5)"
			case "Row":
				l, r = goType+`{Name:"left"}`, goType+`{Name:"right"}`
			case "List<Row>":
				l, r = goType+`{{Name:"left"}}`, goType+`{{Name:"right"}}`
			case "Optional<string>":
				l, r = `pipelangOptionalSome[string]{value:"left"}`, `pipelangOptionalSome[string]{value:"right"}`
			case "Optional<Row>":
				l, r = `pipelangOptionalSome[`+payload+`]{value:`+payload+`{Name:"left"}}`, `pipelangOptionalSome[`+payload+`]{value:`+payload+`{Name:"right"}}`
			case "Result<string,string>":
				l, r = goType+`{OK:true,Value:"left"}`, goType+`{OK:true,Value:"right"}`
			case "Result<List<Row>,string>":
				l, r = goType+`{OK:true,Value:`+payload+`{{Name:"left"}}}`, goType+`{OK:true,Value:`+payload+`{{Name:"right"}}}`
			case "Result<int,ArithmeticError>":
				l, r = goType+`{OK:true,Value:11}`, goType+`{OK:true,Value:22}`
			case "Result<float,ArithmeticError>":
				l, r = goType+`{OK:true,Value:1.25}`, goType+`{OK:true,Value:2.5}`
			}
			checks := fmt.Sprintf(`package %s
import ("testing";"reflect")
func TestTypedChoice(t *testing.T){
 var left %s=%s;var right %s=%s
 for mask:=0;mask<8;mask++{want:=right;if mask&1!=0 && mask&2!=0{want=left};got:=PipeLangSelect(left,right,mask&1!=0,mask&2!=0,mask&4!=0);if !reflect.DeepEqual(got,want){t.Fatalf("mask %%d: %%#v",mask,got)}}
}`, gobackend.PackageName, goType, l, goType, r)
			compileAndRunGeneratedGoFiles(t, generated, []byte(checks))
		})
	}
}
