package pipelang

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"strings"
	"testing"

	"dockpipe/src/lib/pipelang/coreeval"
	"dockpipe/src/lib/pipelang/coreir"
	"dockpipe/src/lib/pipelang/gobackend"
)

const twoConditionalLocalsSource = `public Class Choices {
 public string Select(string raw, bool pick, bool finish, bool enabled) {
  string first = pick ? trim(raw) : raw;
  string second = finish && first != "" ? first + "!" : first;
  if (enabled) { return second; } else { return first; }
 }
}`

func TestV830TwoConditionalLocalsAdmission(t *testing.T) {
	input := semanticTestModuleSet("compiler.selfhosting", []ModuleInput{testModule("compiler.selfhosting", "two.pipe", twoConditionalLocalsSource)}, nil)
	input.LanguageContract = LanguageContract("v0.83.0")
	if err := AnalyzeSemanticModuleSet(input).Error(); err != nil {
		t.Fatal(err)
	}
}

// Every pair of lexical scopes, including the same scope, ancestors, and siblings.
// Ordinary initializers surround the choices and separate a same-scope pair.
func twoChoiceTreeMethod(tree *terminalTree, targets [2]string, unused bool, name string) string {
	return conditionalChoicesTreeMethod(tree, targets[:], unused, name)
}

func conditionalChoicesTreeMethod(tree *terminalTree, targets []string, unused bool, name string, returnOption ...bool) string {
	returnChoice := len(returnOption) > 0 && returnOption[0]
	independentReturn := len(returnOption) > 1 && returnOption[1]
	nestedReturn := len(returnOption) > 2 && returnOption[2]
	nestedInitializer := len(returnOption) > 3 && returnOption[3]
	var emit func(*terminalTree, string, string) string
	emit = func(node *terminalTree, path, value string) string {
		prefix := fmt.Sprintf("string b%s=Echo(%s+%q);", path, value, path+"B")
		value = "b" + path
		for choice, target := range targets {
			if path != target {
				continue
			}
			binding := fmt.Sprintf("q%d%s", choice, path)
			left := fmt.Sprintf("Echo(%s+%q)", value, fmt.Sprintf("T%d%s", choice, path))
			right := fmt.Sprintf("Echo(%s+%q)", value, fmt.Sprintf("F%d%s", choice, path))
			if nestedInitializer && (choice+1)%4&1 != 0 {
				left = fmt.Sprintf("(Check(%q,ia) ? %s : Echo(%s+%q))", fmt.Sprintf("I%dT", choice), left, value, fmt.Sprintf("U%d%s", choice, path))
			}
			if nestedInitializer && (choice+1)%4&2 != 0 {
				right = fmt.Sprintf("(Check(%q,ib) ? %s : Echo(%s+%q))", fmt.Sprintf("I%dF", choice), right, value, fmt.Sprintf("V%d%s", choice, path))
			}
			prefix += fmt.Sprintf("string %s=Check(%q,p%d) ? %s : %s;", binding, fmt.Sprintf("Q%d:%s", choice, path), choice, left, right)
			if !unused || choice != len(targets)-1 {
				value = binding
			}
			prefix += fmt.Sprintf("string a%d%s=Echo(%s+%q);", choice, path, value, fmt.Sprintf("A%d", choice))
			value = fmt.Sprintf("a%d%s", choice, path)
		}
		if node == nil {
			if nestedReturn {
				return prefix + fmt.Sprintf("return Check(%q,finish) ? (Check(%q,yes) ? Echo(%s+%q) : Echo(raw+%q)) : (Check(%q,no) ? Echo(raw+%q) : Echo(raw+%q));", "RETURN", "YES", value, ":TY", ":TN", "NO", ":FY", ":FN")
			}
			if returnChoice {
				condition := "d0"
				if independentReturn {
					condition = "finish"
				}
				return prefix + fmt.Sprintf("return Check(%q,%s && %s!=\"\") ? Echo(%s+%q) : Echo(raw+%q);", "RETURN", condition, value, value, ":R", ":fallback")
			}
			return prefix + fmt.Sprintf("return %s+%q;", value, ":"+path)
		}
		return prefix + fmt.Sprintf("if(Check(%q,d%d)){%s}else{%s}", path, len(path)-1, emit(node.yes, path+"T", value), emit(node.no, path+"F", value))
	}
	var params strings.Builder
	for i := range targets {
		fmt.Fprintf(&params, ",bool p%d", i)
	}
	if independentReturn {
		params.WriteString(",bool finish")
	}
	if nestedReturn {
		params.WriteString(",bool yes,bool no")
	}
	if nestedInitializer {
		params.WriteString(",bool ia,bool ib")
	}
	return fmt.Sprintf("public string %s(string raw,bool d0,bool d1,bool d2%s){%s}\n", name, params.String(), emit(tree, "R", "raw"))
}

// Independent path interpreter for the test's specified string/trace semantics.
func twoChoiceTreeExpected(tree *terminalTree, targets [2]string, unused bool, mask int) (string, []string) {
	return conditionalChoicesTreeExpected(tree, targets[:], unused, mask)
}

func conditionalChoicesTreeExpected(tree *terminalTree, targets []string, unused bool, mask int, returnOption ...bool) (string, []string) {
	returnChoice := len(returnOption) > 0 && returnOption[0]
	independentReturn := len(returnOption) > 1 && returnOption[1]
	nestedReturn := len(returnOption) > 2 && returnOption[2]
	nestedInitializer := len(returnOption) > 3 && returnOption[3]
	node, path, value := tree, "R", "value"
	var trace []string
	for depth := 0; ; depth++ {
		value += path + "B"
		trace = append(trace, "E:"+value)
		for choice, target := range targets {
			if target != path {
				continue
			}
			trace = append(trace, fmt.Sprintf("C:Q%d:%s", choice, path))
			arm := "F"
			if mask&(1<<(choice+3)) != 0 {
				arm = "T"
			}
			if nestedInitializer {
				bit := len(targets) + 3
				if independentReturn {
					bit++
				}
				if nestedReturn {
					bit += 2
				}
				if arm == "T" && (choice+1)%4&1 != 0 {
					trace = append(trace, fmt.Sprintf("C:I%dT", choice))
					if mask&(1<<bit) == 0 {
						arm = "U"
					}
				}
				if arm == "F" && (choice+1)%4&2 != 0 {
					trace = append(trace, fmt.Sprintf("C:I%dF", choice))
					if mask&(1<<(bit+1)) == 0 {
						arm = "V"
					}
				}
			}
			selected := value + fmt.Sprintf("%s%d%s", arm, choice, path)
			trace = append(trace, "E:"+selected)
			if !unused || choice != len(targets)-1 {
				value = selected
			}
			value += fmt.Sprintf("A%d", choice)
			trace = append(trace, "E:"+value)
		}
		if node == nil {
			if nestedReturn {
				trace = append(trace, "C:RETURN")
				result := "value:FN"
				if mask&(1<<(len(targets)+3)) != 0 {
					trace = append(trace, "C:YES")
					result = "value:TN"
					if mask&(1<<(len(targets)+4)) != 0 {
						result = value + ":TY"
					}
				} else {
					trace = append(trace, "C:NO")
					if mask&(1<<(len(targets)+5)) != 0 {
						result = "value:FY"
					}
				}
				return result, append(trace, "E:"+result)
			}
			if returnChoice {
				trace = append(trace, "C:RETURN")
				result := "value:fallback"
				returnBit := 1
				if independentReturn {
					returnBit = 1 << (len(targets) + 3)
				}
				if mask&returnBit != 0 && value != "" {
					result = value + ":R"
				}
				return result, append(trace, "E:"+result)
			}
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

func TestV830TwoConditionalLocalsAllShapesScopePairs(t *testing.T) {
	methodsTotal, outcomes := 0, 0
	trees := terminalTrees(3)[1:]
	if len(trees) != 25 {
		t.Fatal("shape inventory drift")
	}
	for shape, tree := range trees {
		t.Run(fmt.Sprint(shape), func(t *testing.T) {
			scopes := conditionalTreeScopes(tree, "R")
			var source strings.Builder
			source.WriteString(`public Class Choices {public string Echo(string value)=>value;public bool Check(string path,bool value)=>value;`)
			type sample struct {
				name    string
				targets [2]string
				unused  bool
			}
			var samples []sample
			var methods []string
			for i, first := range scopes {
				for _, second := range scopes[i:] {
					for _, unused := range []bool{false, true} {
						name := fmt.Sprintf("Select%d", len(samples))
						pair := [2]string{first, second}
						samples = append(samples, sample{name, pair, unused})
						methods = append(methods, name)
						source.WriteString(twoChoiceTreeMethod(tree, pair, unused, name))
					}
				}
			}
			source.WriteString("}")
			analysis, program := conditionalLocalTreeProgramVersion(t, PipeLangLanguageContractV830, source.String(), methods)
			againAnalysis, again := conditionalLocalTreeProgramVersion(t, PipeLangLanguageContractV830, source.String(), methods)
			projection, err := BuildSemanticProjection(analysis)
			if err != nil {
				t.Fatal(err)
			}
			repeatedProjection, err := BuildSemanticProjection(againAnalysis)
			if err != nil {
				t.Fatal(err)
			}
			for _, pair := range [][2]any{{program, again}, {projection, repeatedProjection}} {
				a, _ := json.Marshal(pair[0])
				b, _ := json.Marshal(pair[1])
				if !bytes.Equal(a, b) {
					t.Fatal("nondeterministic Core/semantic output")
				}
			}
			generated, err := gobackend.Generate(program)
			if err != nil {
				t.Fatal(err)
			}
			repeated, err := gobackend.Generate(again)
			if err != nil || !bytes.Equal(generated, repeated) {
				t.Fatal("nondeterministic Go")
			}
			var cases, orders strings.Builder
			// Keep every outcome and ordered trace, but compile at most 24
			// sample methods together. The complete program above still proves
			// deterministic generation; these pure functions share only helpers.
			batch := program
			batch.Functions = []coreir.Function{coreFunctionNamed(t, program, "Echo"), coreFunctionNamed(t, program, "Check")}
			flush := func() {
				batchGo, err := gobackend.Generate(batch)
				if err != nil {
					t.Fatal(err)
				}

				compileAndRunGeneratedGoFiles(t, batchGo, []byte(fmt.Sprintf("package %s\nimport \"testing\"\n%s", gobackend.PackageName, cases.String())))
				observed := string(batchGo)
				for marker, probe := range map[string]string{
					"func PipeLangEcho(p0 string) string {":         "v830Trace=append(v830Trace,\"E:\"+p0)",
					"func PipeLangCheck(p0 string, p1 bool) bool {": "v830Trace=append(v830Trace,\"C:\"+p0)",
				} {
					if strings.Count(observed, marker) != 1 {
						t.Fatal("trace marker absent")
					}
					observed = strings.Replace(observed, marker, marker+"\n"+probe, 1)
				}
				compileAndRunGeneratedGoFiles(t, []byte(observed), []byte(fmt.Sprintf("package %s\nimport (\"testing\";\"reflect\")\nvar v830Trace []string\n%s", gobackend.PackageName, orders.String())))
				cases.Reset()
				orders.Reset()
				batch.Functions = batch.Functions[:2]
			}
			for i, sample := range samples {
				function := coreFunctionNamed(t, program, sample.name)
				batch.Functions = append(batch.Functions, function)
				if countConditionalExpressionsInCore(function.Body) != coreConditionalCount(function.Body)+2 {
					t.Fatal("two value conditionals absent from Core")
				}
				// Evaluate only the selected function's dependency closure.
				evaluation := program
				evaluation.Functions = []coreir.Function{coreFunctionNamed(t, program, "Echo"), coreFunctionNamed(t, program, "Check"), function}
				prepared := prepareConformanceProgram(t, evaluation)
				var wantedValues, wantedTraces strings.Builder
				for mask := 0; mask < 32; mask++ {
					want, trace := twoChoiceTreeExpected(tree, sample.targets, sample.unused, mask)
					args := []coreeval.Value{{Type: function.Parameters[0].Type, String: "value"}}
					for bit := 0; bit < 5; bit++ {
						args = append(args, coreeval.Value{Type: function.Parameters[bit+1].Type, Bool: mask&(1<<bit) != 0})
					}
					got, err := prepared.Evaluate(function.Identity, args)
					if err != nil || !got.OK || got.Value.String != want {
						t.Fatalf("%s mask %d: %#v %v want %q", sample.name, mask, got, err, want)
					}
					fmt.Fprintf(&wantedValues, "%q,", want)
					var quoted []string
					for _, event := range trace {
						quoted = append(quoted, fmt.Sprintf("%q", event))
					}
					fmt.Fprintf(&wantedTraces, "{%s},", strings.Join(quoted, ","))
					outcomes++
				}
				call := fmt.Sprintf("PipeLang%s(\"value\",mask&1!=0,mask&2!=0,mask&4!=0,mask&8!=0,mask&16!=0)", sample.name)
				fmt.Fprintf(&cases, "func Test%s(t *testing.T){wants:=[]string{%s};for mask,want:=range wants{if got:=%s;got!=want{t.Fatalf(\"mask %%d: %%q want %%q\",mask,got,want)}}}\n", sample.name, wantedValues.String(), call)
				fmt.Fprintf(&orders, "func Test%s(t *testing.T){wants:=[][]string{%s};for mask,want:=range wants{v830Trace=nil;%s;if !reflect.DeepEqual(v830Trace,want){t.Fatalf(\"mask %%d: %%v want %%v\",mask,v830Trace,want)}}}\n", sample.name, wantedTraces.String(), call)
				if (i+1)%24 == 0 || i+1 == len(samples) {
					flush()
				}

			}
			methodsTotal += len(samples)
		})
	}
	t.Logf("%d shapes, %d methods, %d evaluator/pristine-Go cases and ordered traces", len(trees), methodsTotal, outcomes)
}

func TestV830TwoConditionalLocalsDependentCondition(t *testing.T) {
	_, program := conditionalLocalTreeProgramVersion(t, PipeLangLanguageContractV830, twoConditionalLocalsSource, []string{"Select"})
	function := coreFunctionNamed(t, program, "Select")
	var checks strings.Builder
	for _, raw := range []string{"", "   ", " ready ", "raw"} {
		for mask := 0; mask < 8; mask++ {
			first := raw
			if mask&1 != 0 {
				first = strings.TrimSpace(raw)
			}
			second := first
			if mask&2 != 0 && first != "" {
				second += "!"
			}
			want := first
			if mask&4 != 0 {
				want = second
			}
			args := []coreeval.Value{{Type: function.Parameters[0].Type, String: raw}}
			for bit := 0; bit < 3; bit++ {
				args = append(args, coreeval.Value{Type: function.Parameters[bit+1].Type, Bool: mask&(1<<bit) != 0})
			}
			got, err := coreeval.EvaluateProgram(program, function.Identity, args)
			if err != nil || !got.OK || got.Value.String != want {
				t.Fatalf("dependent choice: %#v %v", got, err)
			}
			standalone, err := coreeval.Evaluate(function, args)
			if err != nil || !reflect.DeepEqual(got, standalone) {
				t.Fatalf("standalone: %#v %v", standalone, err)
			}
			fmt.Fprintf(&checks, "if got:=PipeLangSelect(%q,%t,%t,%t);got!=%q{t.Fatal(got)}\n", raw, mask&1 != 0, mask&2 != 0, mask&4 != 0, want)
		}
	}
	generated, err := gobackend.Generate(program)
	if err != nil {
		t.Fatal(err)
	}
	compileAndRunGeneratedGoFiles(t, generated, []byte(fmt.Sprintf("package %s\nimport \"testing\"\nfunc TestDependent(t *testing.T){%s}", gobackend.PackageName, checks.String())))
}

var twoConditionalRulesSource = strings.Replace(conditionalLocalRulesSource, "string after = chosen;", "string after = pick ? chosen : raw;", 1)

func TestV830TwoConditionalLocalsSourceRejection(t *testing.T) {
	testConditionalLocalsSourceRejection(t, PipeLangLanguageContractV830)
}

func testConditionalLocalsSourceRejection(t *testing.T, contract LanguageContract) {
	for _, tc := range []struct{ name, before, after string }{
		{"third in same scope", "string after = pick ? chosen : raw;", "string after = pick ? chosen : raw; string third = pick ? after : raw;"},
		{"condition type", "pick ? before : raw", "raw ? before : raw"},
		{"arm type", "pick ? before : raw", "pick ? before : true"},
		{"local type", "string chosen", "bool chosen"},
		{"self reference", "pick ? before : raw", "pick ? chosen : raw"},
		{"forward reference", "pick ? before : raw", "pick ? after : raw"},
		{"ancestor shadow", "string chosen", "string root"},
		{"duplicate", "string after = pick ? chosen : raw;", "string chosen = raw; string after = chosen;"},
		{"escaping binding", "return root;", "return chosen;"},
		{"sibling binding", "pick ? before : raw", "pick ? sibling : raw"},
		{"nested choice", "pick ? before : raw", "pick ? (inner ? before : raw) : raw"},
		{"third in sibling", "return branch;", "string second = pick ? branch : raw; return second;"},
		{"nested in call", "pick ? before : raw", "trim(pick ? before : raw)"},
		{"return placement", "string chosen = pick ? before : raw;", "string chosen = raw;"},
		{"terminal condition placement", "if (deep)", "if (pick ? deep : inner)"},
		{"propagation operand", "pick ? before : raw", "pick ? propagate(before) : raw"},
		{"match operand", "pick ? before : raw", "pick ? match(before){ some(item) => item, none => raw } : raw"},
		{"depth four", "return after;", "if (pick) { return after; } else { return raw; }"},
		{"assignment", "string after = pick ? chosen : raw;", "chosen = raw; string after = chosen;"},
		{"inference", "string chosen", "var chosen"},
		{"fallthrough", "return after;", ""},
		{"early return", "string before = branch;", "return raw; string before = branch;"},
	} {
		if (contract == PipeLangLanguageContractV840 || contract == PipeLangLanguageContractV850 || contract == PipeLangLanguageContractV860) && strings.HasPrefix(tc.name, "third ") {
			continue
		}
		t.Run(tc.name, func(t *testing.T) {
			source := strings.Replace(twoConditionalRulesSource, tc.before, tc.after, 1)
			if tc.name == "terminal condition placement" {
				source = strings.Replace(source, "string chosen = pick ? before : raw;", "string chosen = raw;", 1)
			}
			if tc.name == "return placement" {
				source = strings.Replace(source, "return after;", "return pick ? after : raw;", 1)
			}
			if source == twoConditionalRulesSource {
				t.Fatal("mutation missed")
			}
			input := semanticTestModuleSet("compiler.selfhosting", []ModuleInput{testModule("compiler.selfhosting", "invalid.pipe", source)}, nil)
			input.LanguageContract = contract
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

func TestV830TwoConditionalLocalsMalformedCore(t *testing.T) {
	testConditionalLocalsMalformedCore(t, PipeLangLanguageContractV830)
}

func testConditionalLocalsMalformedCore(t *testing.T, contract LanguageContract) {
	for _, tc := range []struct {
		name   string
		mutate func(*coreir.Expr, *coreir.Expr)
	}{
		{"second missing arm", func(_, l *coreir.Expr) {
			l.ImmutableLocal.Return.ImmutableLocal.Initializer.Conditional.WhenFalse = nil
		}},
		{"second self reference", func(_, l *coreir.Expr) {
			next := l.ImmutableLocal.Return.ImmutableLocal
			next.Initializer.Conditional.WhenTrue = next.Return.Conditional.WhenTrue
		}},
		{"third in sibling", func(root, l *coreir.Expr) {
			branch := root.ImmutableLocal.Return.Conditional
			value := branch.WhenFalse
			choice := &coreir.Expr{Kind: coreir.ExprConditional, Type: value.Type, Conditional: &coreir.Conditional{Condition: l.ImmutableLocal.Initializer.Conditional.Condition, WhenTrue: value, WhenFalse: value}}
			branch.WhenFalse = &coreir.Expr{Kind: coreir.ExprImmutableLocal, Type: value.Type, ImmutableLocal: &coreir.ImmutableLocal{Name: "third", Type: value.Type, Position: root.ImmutableLocal.Position + 1, Initializer: choice, Return: value}}
		}},
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
			l.ImmutableLocal.Initializer.Conditional.WhenTrue = l.ImmutableLocal.Return.ImmutableLocal.Initializer.Conditional.WhenTrue
		}},
		{"third initializer", func(root, l *coreir.Expr) {
			value := root.ImmutableLocal.Initializer
			root.ImmutableLocal.Initializer = &coreir.Expr{Kind: coreir.ExprConditional, Type: value.Type, Conditional: &coreir.Conditional{Condition: l.ImmutableLocal.Initializer.Conditional.Condition, WhenTrue: value, WhenFalse: value}}
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
		if (contract == PipeLangLanguageContractV840 || contract == PipeLangLanguageContractV850 || contract == PipeLangLanguageContractV860) && strings.HasPrefix(tc.name, "third ") {
			continue
		}
		t.Run(tc.name, func(t *testing.T) {
			_, program := conditionalLocalTreeProgramVersion(t, contract, twoConditionalRulesSource, []string{"Select"})
			root, local := rulesCoreLocal(&program)
			tc.mutate(root, local)
			if tc.name == "third initializer" || tc.name == "third in sibling" {
				if err := coreir.ValidateFunction(program.Functions[0]); err != nil {
					t.Fatalf("internal Core incorrectly narrowed: %v", err)
				}
			}
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

func TestV830TwoConditionalLocalsTypeMatrix(t *testing.T) {
	testConditionalLocalsTypeMatrix(t, PipeLangLanguageContractV830)
}

func testConditionalLocalsTypeMatrix(t *testing.T, contract LanguageContract, zeroOption ...bool) {
	for _, typ := range []string{"string", "bool", "int", "float", "Row", "List<Row>", "Optional<string>", "Optional<Row>", "Result<string,string>", "Result<List<Row>,string>", "Result<int,ArithmeticError>", "Result<float,ArithmeticError>"} {
		t.Run(typ, func(t *testing.T) {
			source := fmt.Sprintf(`public Record Row {public string Name;} public Class Typed {
   public %[1]s Echo(%[1]s value)=>value;
   public %[1]s Select(%[1]s left,%[1]s right,bool pick,bool outer,bool inner){
    %[1]s shared=pick ? left : right;
    if(outer){%[1]s chosen=inner ? shared : right;if(inner){return chosen;}else{return chosen;}}
    else{return shared;}
   }
  }`, typ)
			if contract == PipeLangLanguageContractV840 {
				source = strings.Replace(source, "if(outer)", fmt.Sprintf("%[1]s third=pick ? shared : right;%[1]s fourth=pick ? third : right;if(outer)", typ), 1)
				source = strings.Replace(source, "chosen=inner ? shared", "chosen=inner ? fourth", 1)
			}
			if contract == PipeLangLanguageContractV850 || contract == PipeLangLanguageContractV860 {
				source = fmt.Sprintf(`public Record Row {public string Name;}public Class Typed {
 public %[1]s Echo(%[1]s value)=>value;
 public %[1]s Select(%[1]s left,%[1]s right,bool pick,bool outer,bool inner){
 %[1]s shared=pick ? left : right;
 %[1]s chosen=inner ? shared : right;
 %[1]s result=outer ? chosen : shared;
 return result;}}`, typ)
				if contract == PipeLangLanguageContractV860 {
					source = strings.Replace(source, fmt.Sprintf("%s result=outer ? chosen : shared;\n return result;", typ), "return outer ? chosen : shared;", 1)
				}
			}
			if contract == PipeLangLanguageContractV870 {
				source = strings.Replace(source, "return chosen;", "return inner ? chosen : shared;", 1)
				source = strings.Replace(source, "else{return chosen;}", "else{return inner ? right : chosen;}", 1)
				source = strings.Replace(source, "else{return shared;}", "else{return pick ? shared : right;}", 1)
			}

			if (contract == PipeLangLanguageContractV960 || contract == PipeLangLanguageContractV970) || contract == PipeLangLanguageContractV950 || contract == PipeLangLanguageContractV940 || contract == PipeLangLanguageContractV930 || contract == PipeLangLanguageContractV920 || contract == PipeLangLanguageContractV880 || contract == PipeLangLanguageContractV890 || (contract == PipeLangLanguageContractV900 || contract == PipeLangLanguageContractV910) {
				locals := fmt.Sprintf("%s shared=pick ? left : right;", typ)
				returned := "outer ? (inner ? shared : right) : (pick ? shared : right)"
				if contract == PipeLangLanguageContractV920 || (len(zeroOption) > 0 && zeroOption[0]) {
					locals = ""
					returned = "outer ? (pick && inner ? left : right) : (pick ? left : right)"
				}
				source = fmt.Sprintf(`public Record Row {public string Name;}public Class Typed {
 public %[1]s Echo(%[1]s value)=>value;
 public %[1]s Select(%[1]s left,%[1]s right,bool pick,bool outer,bool inner){%[2]s return %[3]s;}}`, typ, locals, returned)
			}
			if contract == PipeLangLanguageContractV900 || contract == PipeLangLanguageContractV910 {
				source = strings.Replace(source, "shared=pick ? left : right;", "shared=pick ? (inner ? left : left) : (outer ? right : right);", 1)
			}
			if contract == PipeLangLanguageContractV890 {
				pos := strings.LastIndex(source, " return ")
				end := strings.Index(source[pos:], ";") + pos
				returned := source[pos+1 : end+1]
				source = source[:pos] + " if(" + v890TypeCondition(source) + "){" + returned + "}else{" + returned + "}" + source[end+1:]
			}
			if contract == PipeLangLanguageContractV910 {
				source = v910WrapSelect(source)
			}
			if contract == PipeLangLanguageContractV950 || contract == PipeLangLanguageContractV930 || contract == PipeLangLanguageContractV940 {
				source = v930DeepenTypedReturn(t, source)
				if contract == PipeLangLanguageContractV940 {
					source = v940WrapBody(t, source, terminalTrees(1)[1], v890TypeCondition(source))
				}
			}
			if contract == PipeLangLanguageContractV950 {
				source = v920ArrowSelect(t, source)
			}
			if contract == PipeLangLanguageContractV920 {
				source = v920ArrowSelect(t, source, zeroOption[1:]...)
			}
			if contract == PipeLangLanguageContractV960 || contract == PipeLangLanguageContractV970 {
				source = strings.Replace(source, "shared=pick ? left : right;", "shared=pick ? (inner ? (outer ? left : left) : left) : (outer ? (inner ? right : right) : right);", 1)
				source = strings.Replace(source, "a=first ? value : fallback;", "a=first ? (second ? (enabled ? value : value) : value) : (enabled ? (second ? fallback : fallback) : fallback);", 1)
			}
			if contract == PipeLangLanguageContractV970 {
				source = v940WrapBody(t, source, terminalTrees(3)[25], v890TypeCondition(source))
			}

			if (contract == PipeLangLanguageContractV1100 || contract == PipeLangLanguageContractV1110) || contract == PipeLangLanguageContractV980 || contract == PipeLangLanguageContractV990 || contract == PipeLangLanguageContractV1000 || contract == PipeLangLanguageContractV1010 || (contract == PipeLangLanguageContractV1020 || (contract == PipeLangLanguageContractV1030 || (contract == PipeLangLanguageContractV1040 || (contract == PipeLangLanguageContractV1050 || (contract == PipeLangLanguageContractV1060 || (contract == PipeLangLanguageContractV1070 || (contract == PipeLangLanguageContractV1080 || contract == PipeLangLanguageContractV1090))))))) {
				locals := fmt.Sprintf("%s shared=pick ? (inner ? (outer ? left : left) : left) : right;", typ)
				returned := "(outer ? inner : true) ? shared : right"
				if len(zeroOption) > 0 && zeroOption[0] {
					locals = ""
					returned = "(outer ? pick && inner : pick) ? left : right"
				}
				if contract == PipeLangLanguageContractV1100 || contract == PipeLangLanguageContractV1110 {
					returned = strings.Replace(returned, "? shared : right", "? (pick ? shared : shared) : (inner ? right : right)", 1)
					returned = strings.Replace(returned, "? left : right", "? (pick ? left : left) : (inner ? right : right)", 1)
				}
				if (contract == PipeLangLanguageContractV1100 || contract == PipeLangLanguageContractV1110) && len(zeroOption) > 2 {
					if !zeroOption[1] {
						returned = strings.Replace(returned, "(pick ? shared : shared)", "shared", 1)
						returned = strings.Replace(returned, "(pick ? left : left)", "left", 1)
					}
					if !zeroOption[2] {
						returned = strings.Replace(returned, "(inner ? right : right)", "right", 1)
					}
				}
				source = fmt.Sprintf(`public Record Row {public string Name;}public Class Typed {public %[1]s Echo(%[1]s value)=>value;public %[1]s Select(%[1]s left,%[1]s right,bool pick,bool outer,bool inner){%[2]s return %[3]s;}}`, typ, locals, returned)
			}
			if contract == PipeLangLanguageContractV990 || contract == PipeLangLanguageContractV1110 {
				source = v990Wrap(t, source)
			}
			if contract == PipeLangLanguageContractV1000 {
				source = v1000Arrow(t, source)
			}
			if contract == PipeLangLanguageContractV1010 || (contract == PipeLangLanguageContractV1020 || (contract == PipeLangLanguageContractV1030 || (contract == PipeLangLanguageContractV1040 || (contract == PipeLangLanguageContractV1050 || (contract == PipeLangLanguageContractV1060 || (contract == PipeLangLanguageContractV1070 || (contract == PipeLangLanguageContractV1080 || contract == PipeLangLanguageContractV1090))))))) {
				source = v1010Initializer(t, source)
				if contract == PipeLangLanguageContractV1020 || (contract == PipeLangLanguageContractV1030 || (contract == PipeLangLanguageContractV1040 || (contract == PipeLangLanguageContractV1050 || (contract == PipeLangLanguageContractV1060 || (contract == PipeLangLanguageContractV1070 || (contract == PipeLangLanguageContractV1080 || contract == PipeLangLanguageContractV1090)))))) {
					source = v1020TypedScopes(t, source)
					if contract == PipeLangLanguageContractV1030 || (contract == PipeLangLanguageContractV1040 || (contract == PipeLangLanguageContractV1050 || (contract == PipeLangLanguageContractV1060 || (contract == PipeLangLanguageContractV1070 || (contract == PipeLangLanguageContractV1080 || contract == PipeLangLanguageContractV1090))))) {
						condition := v890TypeCondition(source)
						test := condition + " ? true : false"
						if contract == PipeLangLanguageContractV1040 || (contract == PipeLangLanguageContractV1050 || (contract == PipeLangLanguageContractV1060 || (contract == PipeLangLanguageContractV1070 || (contract == PipeLangLanguageContractV1080 || contract == PipeLangLanguageContractV1090)))) {
							test = condition + " ? (" + condition + " ? true : false) : (" + condition + " ? true : false)"
						}
						if contract == PipeLangLanguageContractV1050 || (contract == PipeLangLanguageContractV1060 || (contract == PipeLangLanguageContractV1070 || (contract == PipeLangLanguageContractV1080 || contract == PipeLangLanguageContractV1090))) {
							test = condition + " ? (" + condition + " ? (" + condition + " ? true : false) : false) : (" + condition + " ? true : (" + condition + " ? true : false))"
						}
						if contract == PipeLangLanguageContractV1060 || (contract == PipeLangLanguageContractV1070 || (contract == PipeLangLanguageContractV1080 || contract == PipeLangLanguageContractV1090)) {
							test = "(" + condition + " ? true : false) ? true : false"
						}
						if contract == PipeLangLanguageContractV1070 || (contract == PipeLangLanguageContractV1080 || contract == PipeLangLanguageContractV1090) {
							test = "(" + condition + " ? true : false) ? (" + condition + " ? true : false) : (" + condition + " ? true : false)"
						}
						if contract == PipeLangLanguageContractV1080 || contract == PipeLangLanguageContractV1090 {
							test = "(" + condition + " ? (" + condition + " ? true : false) : (" + condition + " ? true : false)) ? true : false"
						}
						if contract == PipeLangLanguageContractV1090 {
							test = "(" + condition + " ? (" + condition + " ? true : false) : (" + condition + " ? true : false)) ? (" + condition + " ? true : true) : (" + condition + " ? false : false)"
						}
						source = strings.ReplaceAll(source, "if("+condition+")", "if("+test+")")
					}
				}
			}
			_, program := conditionalLocalTreeProgramVersion(t, contract, source, []string{"Select", "Echo"})
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
				if mask&1 != 0 && (mask&2 == 0 || mask&4 != 0) {
					value = left
				}
				want, err := coreeval.EvaluateProgram(program, echo.Identity, []coreeval.Value{value})
				if err != nil {
					t.Fatal(err)
				}
				got, err := coreeval.EvaluateProgram(program, function.Identity, args)
				standalone, standaloneErr := coreeval.Evaluate(function, args)
				if standaloneErr != nil || !reflect.DeepEqual(standalone, got) {
					t.Fatalf("standalone: %#v %v", standalone, standaloneErr)
				}
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
 for mask:=0;mask<8;mask++{want:=right;if mask&1!=0 && (mask&2==0 || mask&4!=0){want=left};got:=PipeLangSelect(left,right,mask&1!=0,mask&2!=0,mask&4!=0);if !reflect.DeepEqual(got,want){t.Fatalf("mask %%d: %%#v",mask,got)}}
}`, gobackend.PackageName, goType, l, goType, r)
			compileAndRunGeneratedGoFiles(t, generated, []byte(checks))
		})
	}
}

func TestV830TwoConditionalLocalsVersionBoundary(t *testing.T) {
	for _, contract := range []LanguageContract{PipeLangLanguageContractV810, PipeLangLanguageContractV820, "v0.999.0", "unknown"} {
		input := semanticTestModuleSet("compiler.selfhosting", []ModuleInput{testModule("compiler.selfhosting", "two.pipe", twoConditionalLocalsSource)}, nil)
		input.LanguageContract = contract
		if AnalyzeSemanticModuleSet(input).Error() == nil {
			t.Fatalf("%s source admitted two locals", contract)
		}
		_, program := conditionalLocalTreeProgramVersion(t, PipeLangLanguageContractV830, twoConditionalLocalsSource, []string{"Select"})
		program.LanguageContract = string(contract)
		assertAdmissionRejected(t, program, "")
	}
	// Two sequential value choices alone do not introduce a new nonterminal body form.
	source := strings.Replace(twoConditionalLocalsSource, "if (enabled) { return second; } else { return first; }", "return second;", 1)
	input := semanticTestModuleSet("compiler.selfhosting", []ModuleInput{testModule("compiler.selfhosting", "two.pipe", source)}, nil)
	input.LanguageContract = PipeLangLanguageContractV830
	if AnalyzeSemanticModuleSet(input).Error() == nil {
		t.Fatal("two choices without a terminal tree admitted")
	}
}

func TestV830TwoConditionalLocalsInheritance(t *testing.T) {
	// Compare full typed/Core/semantic/Go artifacts, normalizing language metadata only.
	compare := func(source string, methods []string) {
		t.Helper()
		var baseline [][]byte
		for _, contract := range []LanguageContract{PipeLangLanguageContractV820, PipeLangLanguageContractV830, PipeLangLanguageContractV840, PipeLangLanguageContractV850, PipeLangLanguageContractV860, PipeLangLanguageContractV870, PipeLangLanguageContractV880, PipeLangLanguageContractV890, PipeLangLanguageContractV900, PipeLangLanguageContractV910} {
			analysis, program := conditionalLocalTreeProgramVersion(t, contract, source, methods)
			projection, err := BuildSemanticProjection(analysis)
			if err != nil {
				t.Fatal(err)
			}
			generated, err := gobackend.Generate(program)
			if err != nil {
				t.Fatal(err)
			}
			projection.LanguageContract = PipeLangLanguageContractV820
			program.LanguageContract = coreir.LanguageContractV820
			values := []any{projection, program}
			for _, method := range methods {
				typed, err := LowerSemanticMethodToHIR(analysis, semanticMethodNamed(t, analysis, method).Identity)
				if err != nil {
					t.Fatal(err)
				}
				typed.LanguageContract = coreir.LanguageContractV820
				values = append(values, typed)
			}
			artifacts := [][]byte{generated}
			for _, value := range values {
				b, err := json.Marshal(value)
				if err != nil {
					t.Fatal(err)
				}
				artifacts = append(artifacts, b)
			}
			if baseline == nil {
				baseline = artifacts
			} else if !reflect.DeepEqual(artifacts, baseline) {
				t.Fatal("inherited artifacts changed")
			}
		}
	}
	for shape, tree := range terminalTrees(3)[1:] {
		t.Run(fmt.Sprint(shape), func(t *testing.T) {
			compare(terminalTreeSource(tree, true, true), []string{"Select"})
			var source strings.Builder
			source.WriteString(`public Class Choices {public string Echo(string value)=>value;public bool Check(string path,bool value)=>value;`)
			var methods []string
			for _, scope := range conditionalTreeScopes(tree, "R") {
				for layout := 0; layout < 3; layout++ {
					name := fmt.Sprintf("Select%s%d", scope, layout)
					methods = append(methods, name)
					source.WriteString(conditionalTreeMethod(tree, scope, layout, name))
				}
			}
			source.WriteString("}")
			compare(source.String(), methods)
		})
	}
	for _, source := range []string{
		`public Class Choices {public string Select(string raw,bool pick){string value=pick ? raw : trim(raw);return value;}}`,
		`public Class Choices {public Optional<string> Select(Optional<string> value)=>some(propagate(value));}`,
		`public Class Choices {public string Select(Optional<string> value)=>match(value){some(item)=>item,none=>"empty"};}`,
		`public Class Choices {public string Echo(string value)=>value;public string Select(string value)=>Echo(trim(value));}`,
	} {
		compare(source, []string{"Select"})
	}
}

func TestV830TwoConditionalLocalsCarrierAndHostValues(t *testing.T) {
	testConditionalLocalsCarrierAndHostValues(t, PipeLangLanguageContractV830)
}

func testConditionalLocalsCarrierAndHostValues(t *testing.T, contract LanguageContract, zeroOption ...bool) {
	for _, typ := range []string{"string", "Row", "List<Row>", "Optional<string>", "Optional<Row>", "Result<string,string>", "Result<List<Row>,string>", "Result<int,ArithmeticError>", "Result<float,ArithmeticError>"} {
		t.Run(typ, func(t *testing.T) {
			source := fmt.Sprintf(`public Record Row {public string Name;}public Class Choices {
    public %[1]s Echo(%[1]s value)=>value;
    public %[1]s Select(%[1]s value,%[1]s fallback,bool first,bool second,bool enabled) {
     %[1]s a=first ? value : fallback;
     if(enabled){%[1]s b=second ? a : fallback;return b;}else{return a;}
    }
   }`, typ)
			if contract == PipeLangLanguageContractV840 {
				source = strings.Replace(source, "if(enabled)", fmt.Sprintf("%[1]s c=first ? a : fallback;%[1]s d=first ? c : fallback;if(enabled)", typ), 1)
				source = strings.Replace(source, "b=second ? a", "b=second ? d", 1)
			}
			if contract == PipeLangLanguageContractV850 || contract == PipeLangLanguageContractV860 {
				source = fmt.Sprintf(`public Record Row {public string Name;}public Class Choices {
 public %[1]s Echo(%[1]s value)=>value;
 public %[1]s Select(%[1]s value,%[1]s fallback,bool first,bool second,bool enabled){
 %[1]s a=first ? value : fallback;
 %[1]s b=second ? a : fallback;
 %[1]s c=enabled ? b : a;
 return c;}}`, typ)
				if contract == PipeLangLanguageContractV860 {
					source = strings.Replace(source, fmt.Sprintf("%s c=enabled ? b : a;\n return c;", typ), "return enabled ? b : a;", 1)
				}
			}
			if contract == PipeLangLanguageContractV870 {
				source = strings.Replace(source, "return b;", "return second ? a : fallback;", 1)
				source = strings.Replace(source, "return a;", "return first ? value : fallback;", 1)
			}

			if (contract == PipeLangLanguageContractV960 || contract == PipeLangLanguageContractV970) || contract == PipeLangLanguageContractV950 || contract == PipeLangLanguageContractV940 || contract == PipeLangLanguageContractV930 || contract == PipeLangLanguageContractV920 || contract == PipeLangLanguageContractV880 || contract == PipeLangLanguageContractV890 || (contract == PipeLangLanguageContractV900 || contract == PipeLangLanguageContractV910) {
				locals := fmt.Sprintf("%s a=first ? value : fallback;", typ)
				returned := "enabled ? (second ? a : fallback) : (first ? a : fallback)"
				if contract == PipeLangLanguageContractV920 || (len(zeroOption) > 0 && zeroOption[0]) {
					locals = ""
					returned = "enabled ? (first && second ? value : fallback) : (first ? value : fallback)"
				}
				source = fmt.Sprintf(`public Record Row {public string Name;}public Class Choices {
 public %[1]s Echo(%[1]s value)=>value;
 public %[1]s Select(%[1]s value,%[1]s fallback,bool first,bool second,bool enabled){%[2]s return %[3]s;}}`, typ, locals, returned)
			}
			if contract == PipeLangLanguageContractV900 || contract == PipeLangLanguageContractV910 {
				source = strings.Replace(source, "a=first ? value : fallback;", "a=first ? (second ? value : value) : (enabled ? fallback : fallback);", 1)
			}
			if contract == PipeLangLanguageContractV890 {
				pos := strings.LastIndex(source, " return ")
				end := strings.Index(source[pos:], ";") + pos
				returned := source[pos+1 : end+1]
				source = source[:pos] + " if(" + v890TypeCondition(source) + "){" + returned + "}else{" + returned + "}" + source[end+1:]
			}
			if contract == PipeLangLanguageContractV950 || contract == PipeLangLanguageContractV930 || contract == PipeLangLanguageContractV940 {
				source = v930DeepenTypedReturn(t, source)
				if contract == PipeLangLanguageContractV940 {
					source = v940WrapBody(t, source, terminalTrees(1)[1], v890TypeCondition(source))
				}
			}
			if contract == PipeLangLanguageContractV950 {
				source = v920ArrowSelect(t, source)
			}
			if contract == PipeLangLanguageContractV920 {
				source = v920ArrowSelect(t, source, zeroOption[1:]...)
			}
			if contract == PipeLangLanguageContractV960 || contract == PipeLangLanguageContractV970 {
				source = strings.Replace(source, "shared=pick ? left : right;", "shared=pick ? (inner ? (outer ? left : left) : left) : (outer ? (inner ? right : right) : right);", 1)
				source = strings.Replace(source, "a=first ? value : fallback;", "a=first ? (second ? (enabled ? value : value) : value) : (enabled ? (second ? fallback : fallback) : fallback);", 1)
			}
			if contract == PipeLangLanguageContractV970 {
				source = v940WrapBody(t, source, terminalTrees(3)[25], v890TypeCondition(source))
			}

			if (contract == PipeLangLanguageContractV1100 || contract == PipeLangLanguageContractV1110) || contract == PipeLangLanguageContractV980 || contract == PipeLangLanguageContractV990 || contract == PipeLangLanguageContractV1000 || contract == PipeLangLanguageContractV1010 || (contract == PipeLangLanguageContractV1020 || (contract == PipeLangLanguageContractV1030 || (contract == PipeLangLanguageContractV1040 || (contract == PipeLangLanguageContractV1050 || (contract == PipeLangLanguageContractV1060 || (contract == PipeLangLanguageContractV1070 || (contract == PipeLangLanguageContractV1080 || contract == PipeLangLanguageContractV1090))))))) {
				locals := fmt.Sprintf("%s a=first ? (second ? (enabled ? value : value) : value) : fallback;", typ)
				returned := "(enabled ? second : true) ? a : fallback"
				if len(zeroOption) > 0 && zeroOption[0] {
					locals = ""
					returned = "(enabled ? first && second : first) ? value : fallback"
				}
				if contract == PipeLangLanguageContractV1100 || contract == PipeLangLanguageContractV1110 {
					returned = strings.Replace(returned, "? a : fallback", "? (first ? a : a) : (second ? fallback : fallback)", 1)
					returned = strings.Replace(returned, "? value : fallback", "? (first ? value : value) : (second ? fallback : fallback)", 1)
				}
				if (contract == PipeLangLanguageContractV1100 || contract == PipeLangLanguageContractV1110) && len(zeroOption) > 2 {
					if !zeroOption[1] {
						returned = strings.Replace(returned, "(first ? a : a)", "a", 1)
						returned = strings.Replace(returned, "(first ? value : value)", "value", 1)
					}
					if !zeroOption[2] {
						returned = strings.Replace(returned, "(second ? fallback : fallback)", "fallback", 1)
					}
				}
				source = fmt.Sprintf(`public Record Row {public string Name;}public Class Choices {public %[1]s Echo(%[1]s value)=>value;public %[1]s Select(%[1]s value,%[1]s fallback,bool first,bool second,bool enabled){%[2]s return %[3]s;}}`, typ, locals, returned)
			}
			if contract == PipeLangLanguageContractV990 || contract == PipeLangLanguageContractV1110 {
				source = v990Wrap(t, source)
			}
			if contract == PipeLangLanguageContractV1000 {
				source = v1000Arrow(t, source)
			}
			if contract == PipeLangLanguageContractV1010 || (contract == PipeLangLanguageContractV1020 || (contract == PipeLangLanguageContractV1030 || (contract == PipeLangLanguageContractV1040 || (contract == PipeLangLanguageContractV1050 || (contract == PipeLangLanguageContractV1060 || (contract == PipeLangLanguageContractV1070 || (contract == PipeLangLanguageContractV1080 || contract == PipeLangLanguageContractV1090))))))) {
				source = v1010Initializer(t, source)
				if contract == PipeLangLanguageContractV1020 || (contract == PipeLangLanguageContractV1030 || (contract == PipeLangLanguageContractV1040 || (contract == PipeLangLanguageContractV1050 || (contract == PipeLangLanguageContractV1060 || (contract == PipeLangLanguageContractV1070 || (contract == PipeLangLanguageContractV1080 || contract == PipeLangLanguageContractV1090)))))) {
					source = v1020TypedScopes(t, source)
					if contract == PipeLangLanguageContractV1030 || (contract == PipeLangLanguageContractV1040 || (contract == PipeLangLanguageContractV1050 || (contract == PipeLangLanguageContractV1060 || (contract == PipeLangLanguageContractV1070 || (contract == PipeLangLanguageContractV1080 || contract == PipeLangLanguageContractV1090))))) {
						condition := v890TypeCondition(source)
						test := condition + " ? true : false"
						if contract == PipeLangLanguageContractV1040 || (contract == PipeLangLanguageContractV1050 || (contract == PipeLangLanguageContractV1060 || (contract == PipeLangLanguageContractV1070 || (contract == PipeLangLanguageContractV1080 || contract == PipeLangLanguageContractV1090)))) {
							test = condition + " ? (" + condition + " ? true : false) : (" + condition + " ? true : false)"
						}
						if contract == PipeLangLanguageContractV1050 || (contract == PipeLangLanguageContractV1060 || (contract == PipeLangLanguageContractV1070 || (contract == PipeLangLanguageContractV1080 || contract == PipeLangLanguageContractV1090))) {
							test = condition + " ? (" + condition + " ? (" + condition + " ? true : false) : false) : (" + condition + " ? true : (" + condition + " ? true : false))"
						}
						if contract == PipeLangLanguageContractV1060 || (contract == PipeLangLanguageContractV1070 || (contract == PipeLangLanguageContractV1080 || contract == PipeLangLanguageContractV1090)) {
							test = "(" + condition + " ? true : false) ? true : false"
						}
						if contract == PipeLangLanguageContractV1070 || (contract == PipeLangLanguageContractV1080 || contract == PipeLangLanguageContractV1090) {
							test = "(" + condition + " ? true : false) ? (" + condition + " ? true : false) : (" + condition + " ? true : false)"
						}
						if contract == PipeLangLanguageContractV1080 || contract == PipeLangLanguageContractV1090 {
							test = "(" + condition + " ? (" + condition + " ? true : false) : (" + condition + " ? true : false)) ? true : false"
						}
						if contract == PipeLangLanguageContractV1090 {
							test = "(" + condition + " ? (" + condition + " ? true : false) : (" + condition + " ? true : false)) ? (" + condition + " ? true : true) : (" + condition + " ? false : false)"
						}
						source = strings.ReplaceAll(source, "if("+condition+")", "if("+test+")")
					}
				}
			}
			_, program := conditionalLocalTreeProgramVersion(t, contract, source, []string{"Echo", "Select"})
			function := coreFunctionNamed(t, program, "Select")
			generated, err := gobackend.Generate(program)
			if err != nil {
				t.Fatal(err)
			}
			goType, payload := generatedArgumentTypes(t, generated)
			var checks strings.Builder
			checks.WriteString(hostArgumentGoHelpers)
			checks.WriteString("func TestValues(t *testing.T){\n")
			values := canonicalHostArguments(function.Parameters[0].Type)
			var fallback hostArgumentCase
			for _, candidate := range values {
				if candidate.valid && (candidate.value.Result == nil || candidate.value.Result.OK) {
					fallback = candidate
					break
				}
			}
			if !fallback.valid {
				t.Fatal("missing valid fallback")
			}
			fallbackExpression := strings.NewReplacer("$T", goType, "$P", payload).Replace(fallback.goValue)
			for _, tc := range values {
				for mask := 0; mask < 8; mask++ {
					args := []coreeval.Value{tc.value, fallback.value}
					for bit := 0; bit < 3; bit++ {
						args = append(args, coreeval.Value{Type: function.Parameters[bit+2].Type, Bool: mask&(1<<bit) != 0})
					}
					before, _ := json.Marshal(args)
					for _, evaluate := range []func() (coreeval.Outcome, error){func() (coreeval.Outcome, error) { return coreeval.Evaluate(function, args) }, func() (coreeval.Outcome, error) { return coreeval.EvaluateProgram(program, function.Identity, args) }} {
						got, err := evaluate()
						if !tc.valid {
							if err == nil {
								t.Fatalf("%s mask%d malformed argument accepted", tc.name, mask)
							}
							continue
						}
						selected := fallback.value
						if mask&1 != 0 && (mask&4 == 0 || mask&2 != 0) {
							selected = tc.value
						}
						want := coreeval.Outcome{OK: true, Value: selected}
						if selected.Result != nil {
							want = *selected.Result
						}
						if err != nil {
							t.Fatal(err)
						}
						if math.IsNaN(want.Value.Float) {
							if !got.OK || !math.IsNaN(got.Value.Float) {
								t.Fatal("NaN transport")
							}
						} else if !reflect.DeepEqual(got, want) || math.Signbit(got.Value.Float) != math.Signbit(want.Value.Float) {
							t.Fatalf("%s mask%d: %#v want %#v", tc.name, mask, got, want)
						}
					}
					after, _ := json.Marshal(args)
					if !bytes.Equal(before, after) {
						t.Fatal("input carrier mutated")
					}
					expression := strings.NewReplacer("$T", goType, "$P", payload).Replace(tc.goValue)
					fmt.Fprintf(&checks, "t.Run(%q,func(t *testing.T){value:=%s;fallback:=%s;want:=fallback;if %t {want=value};checkHostArgument(t,%t,func(){got:=PipeLangSelect(value,fallback,%t,%t,%t);if !sameHostValue(reflect.ValueOf(got),reflect.ValueOf(want)){t.Fatal(\"transport changed\")}})})\n", fmt.Sprintf("%s/%d", tc.name, mask), expression, fallbackExpression, mask&1 != 0 && (mask&4 == 0 || mask&2 != 0), tc.valid, mask&1 != 0, mask&2 != 0, mask&4 != 0)
				}
			}
			checks.WriteString("}\n")
			compileAndRunGeneratedGoFiles(t, generated, []byte(checks.String()))
		})
	}
}

func TestV830TwoConditionalLocalsDifferentTypes(t *testing.T) {
	testConditionalLocalsDifferentTypes(t, PipeLangLanguageContractV830)
}

func testConditionalLocalsDifferentTypes(t *testing.T, contract LanguageContract) {
	source := `public Class Choices {public string Select(string raw,bool enabled){
  bool empty = raw == "" ? true : false;
  string label = empty ? "empty" : raw;
  if(enabled){return label;}else{return raw;}
 }}`
	if contract == PipeLangLanguageContractV840 {
		source = strings.Replace(source, "if(enabled)", "bool on=enabled ? true : false;string final=on ? label : raw;if(on)", 1)
		source = strings.Replace(source, "return label;", "return final;", 1)
	}
	if contract == PipeLangLanguageContractV850 || contract == PipeLangLanguageContractV860 {
		source = strings.Replace(source, "if(enabled){return label;}else{return raw;}", "string result=enabled ? label : raw;return result;", 1)
	}
	if contract == PipeLangLanguageContractV870 {
		source = strings.Replace(source, "return label;", "return empty ? label : raw;", 1)
	}
	if contract == PipeLangLanguageContractV860 {
		source = strings.Replace(source, "string result=enabled ? label : raw;return result;", "return enabled ? label : raw;", 1)
	}
	if contract == PipeLangLanguageContractV880 || (contract == PipeLangLanguageContractV900 || contract == PipeLangLanguageContractV910) {
		source = strings.Replace(source, "if(enabled){return label;}else{return raw;}", "return enabled ? (empty ? label : raw) : (empty ? raw : raw);", 1)
	}
	if contract == PipeLangLanguageContractV890 {
		source = strings.Replace(source, "return label;", "return empty ? (enabled ? label : raw) : (enabled ? raw : raw);", 1)
	}
	if contract == PipeLangLanguageContractV900 || contract == PipeLangLanguageContractV910 {
		source = strings.Replace(source, `bool empty = raw == "" ? true : false;`, `bool empty = raw == "" ? (enabled ? true : true) : (enabled ? false : false);`, 1)
	}
	if contract == PipeLangLanguageContractV910 {
		source = v910WrapSelect(source)
	}
	_, program := conditionalLocalTreeProgramVersion(t, contract, source, []string{"Select"})
	function := coreFunctionNamed(t, program, "Select")
	var tests strings.Builder
	for _, raw := range []string{"", "present"} {
		for _, enabled := range []bool{false, true} {
			want := raw
			if enabled && raw == "" {
				want = "empty"
			}
			args := []coreeval.Value{{Type: function.Parameters[0].Type, String: raw}, {Type: function.Parameters[1].Type, Bool: enabled}}
			for _, evaluate := range []func() (coreeval.Outcome, error){func() (coreeval.Outcome, error) { return coreeval.Evaluate(function, args) }, func() (coreeval.Outcome, error) { return coreeval.EvaluateProgram(program, function.Identity, args) }} {
				got, err := evaluate()
				if err != nil || !got.OK || got.Value.String != want {
					t.Fatalf("mixed choices: %#v %v", got, err)
				}
			}
			fmt.Fprintf(&tests, "if got:=PipeLangSelect(%q,%t);got!=%q{t.Fatal(got)}\n", raw, enabled, want)
		}
	}
	generated, err := gobackend.Generate(program)
	if err != nil {
		t.Fatal(err)
	}
	compileAndRunGeneratedGoFiles(t, generated, []byte(fmt.Sprintf("package %s\nimport \"testing\"\nfunc TestDifferent(t *testing.T){%s}", gobackend.PackageName, tests.String())))
}

func v890TypeCondition(source string) string {
	if strings.Contains(source, "bool enabled") {
		return "enabled"
	}
	return "outer"
}
