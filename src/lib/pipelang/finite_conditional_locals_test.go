package pipelang

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"dockpipe/src/lib/pipelang/coreeval"
	"dockpipe/src/lib/pipelang/coreir"
	"dockpipe/src/lib/pipelang/gobackend"
)

const finiteConditionalLocalsSource = `public Class Choices {
 public string Select(string raw, bool pick, bool finish, bool enabled) {
  string first = pick ? trim(raw) : raw;
  string second = finish && first != "" ? first + "!" : first;
  string third = enabled && second != "" ? second + "?" : second;
  if (enabled) { return third; } else { return first; }
 }
}`

func TestV840FiniteConditionalLocalsAdmission(t *testing.T) {
	input := semanticTestModuleSet("compiler.selfhosting", []ModuleInput{testModule("compiler.selfhosting", "finite.pipe", finiteConditionalLocalsSource)}, nil)
	input.LanguageContract = LanguageContract("v0.84.0")
	if err := AnalyzeSemanticModuleSet(input).Error(); err != nil {
		t.Fatal(err)
	}
}

func TestV840FiniteConditionalLocalsAllShapes(t *testing.T) {
	trees := terminalTrees(3)[1:]
	if len(trees) != 25 {
		t.Fatal("shape inventory drift")
	}
	testFiniteConditionalLocalsLayouts(t, PipeLangLanguageContractV840, trees)
}

var finiteShapeSlots = make(chan struct{}, 4)

func parallelFiniteShapes() bool {
	return os.Getenv("PIPELANG_PARALLEL_SHAPES") == "1" && os.Getenv("PIPELANG_COMPILED_CACHE") != "" && os.Getenv("PIPELANG_GENERATED_BATCH") == "1" && os.Getenv("GOENV") == "off" && os.Getenv("GOFLAGS") == ""
}

func enterFiniteShape(t *testing.T) {
	if !parallelFiniteShapes() {
		return
	}
	t.Parallel()
	finiteShapeSlots <- struct{}{}
	t.Cleanup(func() { <-finiteShapeSlots })
}

func testFiniteConditionalLocalsLayouts(t *testing.T, contract LanguageContract, trees []*terminalTree, returnOption ...bool) {
	returnChoice := len(returnOption) > 0 && returnOption[0]
	nestedInitializer := contract == PipeLangLanguageContractV910
	nestedReturn := contract == PipeLangLanguageContractV890 || (nestedInitializer && returnChoice)
	independentReturn := contract == PipeLangLanguageContractV870 || nestedReturn
	extraBits := 3
	if independentReturn {
		extraBits = 4
	}
	if nestedReturn {
		extraBits = 6
	}
	if nestedInitializer {
		extraBits += 2
	}
	bundle := os.Getenv("PIPELANG_NATIVE_BUNDLE") == "1" && os.Getenv("PIPELANG_GENERATED_BATCH") == "1" && os.Getenv("PIPELANG_COMPILED_CACHE") != "" && os.Getenv("GOFLAGS") == "" && os.Getenv("GOENV") == "off"
	methodsTotal, outcomes := 0, 0
	var totals sync.Mutex
	parallel := len(trees) > 1 && parallelFiniteShapes()
	logTotals := func() {
		totals.Lock()
		defer totals.Unlock()
		t.Logf("%d shapes, %d methods, %d evaluator/pristine-Go cases and ordered traces", len(trees), methodsTotal, outcomes)
	}
	if parallel {
		t.Cleanup(logTotals)
	}

	for shape, tree := range trees {
		t.Run(fmt.Sprint(shape), func(t *testing.T) {
			if parallel {
				enterFiniteShape(t)
			}
			shapeMethods, shapeOutcomes := 0, 0
			defer func() { totals.Lock(); methodsTotal += shapeMethods; outcomes += shapeOutcomes; totals.Unlock() }()

			scopes := conditionalTreeScopes(tree, "R")
			type sample struct {
				name    string
				targets []string
				unused  bool
			}
			var samples []sample
			var layouts [][]string
			for _, scope := range scopes {
				layouts = append(layouts, []string{scope, scope, scope})
			}
			if tree == nil {
				if returnChoice {
					layouts = append(layouts, []string{})
				}
				layouts = append(layouts, []string{"R"}, []string{"R", "R"}, []string{"R", "R", "R", "R", "R"})
			} else {
				layouts = append(layouts, []string{"R", "RT", "RF"}, []string{"R", "R", scopes[len(scopes)-1]}, []string{"R", "RT", "RT", "RF", "RF"})
			}
			if nestedInitializer {
				layouts = append(layouts, []string{})
				for _, scope := range scopes {
					layouts = append(layouts, []string{scope}, []string{scope, scope})
				}
				slots := []string{"R", "RT", "RF"}
				for subset := 1; subset < 7; subset++ {
					var chosen []string
					for i, scope := range slots {
						if subset&(1<<i) != 0 {
							chosen = append(chosen, scope)
						}
					}
					layouts = append(layouts, chosen)
				}
			}
			for _, targets := range layouts {
				for _, unused := range []bool{false, true} {
					name := fmt.Sprintf("Select%d", len(samples))
					samples = append(samples, sample{name, targets, unused})
				}
			}
			if nestedInitializer && len(returnOption) > 2 {
				partition := 0
				if returnOption[1] {
					partition++
				}
				if returnOption[2] {
					partition += 2
				}
				selected := samples[:0]
				for i, sample := range samples {
					if i%4 == partition {
						selected = append(selected, sample)
					}
				}
				samples = selected
			}
			batchSize := len(samples)
			if nestedReturn || nestedInitializer {
				// Runtime oracle fixtures keep compiler input small enough for
				// bounded batches while preserving each named method and vector.
				batchSize = 4
			}
			for start := 0; start < len(samples); start += batchSize {
				end := start + batchSize
				if end > len(samples) {
					end = len(samples)
				}
				samples := samples[start:end]
				var source strings.Builder
				source.WriteString(`public Class Choices {public string Echo(string value)=>value;public bool Check(string path,bool value)=>value;`)
				methods := []string{"Echo", "Check"}
				for _, sample := range samples {
					methods = append(methods, sample.name)
					source.WriteString(conditionalChoicesTreeMethod(tree, sample.targets, sample.unused, sample.name, returnChoice, independentReturn, nestedReturn, nestedInitializer))
				}
				source.WriteString("}")
				analysis, program := conditionalLocalTreeProgramVersion(t, contract, source.String(), methods)
				againAnalysis, again := conditionalLocalTreeProgramVersion(t, contract, source.String(), methods)
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
				fixtures := make(map[string][]byte)
				for _, sample := range samples {
					function := coreFunctionNamed(t, program, sample.name)
					expectedInitializers := len(sample.targets)
					if nestedInitializer {
						for i := range sample.targets {
							shape := (i + 1) % 4
							if shape&1 != 0 {
								expectedInitializers++
							}
							if shape&2 != 0 {
								expectedInitializers++
							}
						}
					}
					if countConditionalExpressionsInCore(function.Body) != coreConditionalCount(function.Body)+expectedInitializers {
						t.Fatal("value conditionals absent from Core")
					}
					// Evaluate only the selected function's dependency closure.
					evaluation := program
					evaluation.Functions = []coreir.Function{coreFunctionNamed(t, program, "Echo"), coreFunctionNamed(t, program, "Check"), function}
					prepared, err := coreeval.PrepareProgram(evaluation)
					if err != nil {
						t.Fatal(err)
					}
					oracle := make([]finiteConditionalOracleCase, 0, 1<<(len(sample.targets)+extraBits))
					for mask := 0; mask < 1<<(len(sample.targets)+extraBits); mask++ {
						want, trace := conditionalChoicesTreeExpected(tree, sample.targets, sample.unused, mask, returnChoice, independentReturn, nestedReturn, nestedInitializer)
						args := []coreeval.Value{{Type: function.Parameters[0].Type, String: "value"}}
						for bit := 0; bit < len(sample.targets)+extraBits; bit++ {
							args = append(args, coreeval.Value{Type: function.Parameters[bit+1].Type, Bool: mask&(1<<bit) != 0})
						}
						got, err := prepared.Evaluate(function.Identity, args)
						if err != nil || !got.OK || got.Value.String != want {
							t.Fatalf("%s mask %d: %#v %v want %q", sample.name, mask, got, err, want)
						}
						oracle = append(oracle, finiteConditionalOracleCase{want, append([]string{}, trace...)})
						shapeOutcomes++
					}
					call := fmt.Sprintf("PipeLang%s(\"value\"", sample.name)
					for bit := 0; bit < len(sample.targets)+extraBits; bit++ {
						call += fmt.Sprintf(",mask&%d!=0", 1<<bit)
					}
					call += ")"
					payload, err := json.Marshal(oracle)
					if err != nil {
						t.Fatal(err)
					}
					fixtures[sample.name+".json"] = payload
					if bundle {
						fmt.Fprintf(&cases, "func Test%s(t *testing.T){oracle.Values(t,%q,%d,func(mask int)string{return %s})}\n", sample.name, sample.name+".json", len(oracle), call)
						fmt.Fprintf(&orders, "func Test%s(t *testing.T){oracle.Traces(t,%q,%d,func(mask int)[]string{v840Trace=nil;%s;return v840Trace})}\n", sample.name, sample.name+".json", len(oracle), call)
					} else {
						fmt.Fprintf(&cases, "func Test%s(t *testing.T){wants:=loadFiniteOracle(t,%q,%d);for mask,want:=range wants{if got:=%s;got!=want.Value{t.Fatalf(\"mask %%d: %%q want %%q\",mask,got,want.Value)}}}\n", sample.name, sample.name+".json", len(oracle), call)
						fmt.Fprintf(&orders, "func Test%s(t *testing.T){wants:=loadFiniteOracle(t,%q,%d);for mask,want:=range wants{v840Trace=nil;%s;if !reflect.DeepEqual(v840Trace,want.Trace){t.Fatalf(\"mask %%d: %%v want %%v\",mask,v840Trace,want.Trace)}}}\n", sample.name, sample.name+".json", len(oracle), call)
					}
				}
				if bundle {
					checks := []byte(fmt.Sprintf("package %s\nimport (\"testing\";\"pipelang-generated-check/oracle\")\n%s", gobackend.PackageName, cases.String()))
					if !queueGeneratedBatchWithOracle(t, generated, checks, fixtures, []byte(finiteSharedOracle)) {
						t.Fatal("finite bundle source unexpectedly ineligible")
					}
				} else {
					compileAndRunGeneratedGoFilesWithFixtures(t, generated, []byte(fmt.Sprintf("package %s\nimport (\"testing\";\"encoding/json\";\"os\")\n%s\n%s", gobackend.PackageName, finiteConditionalOracleLoader, cases.String())), fixtures)
				}
				observed := string(generated)
				for marker, probe := range map[string]string{
					"func PipeLangEcho(p0 string) string {":         "v840Trace=append(v840Trace,\"E:\"+p0)",
					"func PipeLangCheck(p0 string, p1 bool) bool {": "v840Trace=append(v840Trace,\"C:\"+p0)",
				} {
					if strings.Count(observed, marker) != 1 {
						t.Fatal("trace marker absent")
					}
					observed = strings.Replace(observed, marker, marker+"\n"+probe, 1)
				}
				if bundle {
					checks := []byte(fmt.Sprintf("package %s\nimport (\"testing\";\"pipelang-generated-check/oracle\")\nvar v840Trace []string\n%s", gobackend.PackageName, orders.String()))
					if !queueGeneratedBatchWithOracle(t, []byte(observed), checks, fixtures, []byte(finiteSharedOracle)) {
						t.Fatal("finite trace bundle source unexpectedly ineligible")
					}
				} else {
					compileAndRunGeneratedGoFilesWithFixtures(t, []byte(observed), []byte(fmt.Sprintf("package %s\nimport (\"testing\";\"reflect\";\"encoding/json\";\"os\")\nvar v840Trace []string\n%s\n%s", gobackend.PackageName, finiteConditionalOracleLoader, orders.String())), fixtures)
				}
				shapeMethods += len(samples)
			}
		})
	}
	if !parallel {
		logTotals()
	}
}

// Oracle data is produced by the independent tree model, then loaded by the
// generated executable. Keeping it out of Go literals avoids compiling large
// constant tables without removing any vector or ordered trace assertion.
type finiteConditionalOracleCase struct {
	Value string
	Trace []string
}

const finiteConditionalOracleLoader = `
type finiteOracleCase struct { Value string; Trace []string }
func loadFiniteOracle(t *testing.T, name string, count int) []finiteOracleCase {
 t.Helper()
 data, err := os.ReadFile(name)
 if err != nil { t.Fatal(err) }
 var oracle []finiteOracleCase
 if err := json.Unmarshal(data, &oracle); err != nil { t.Fatal(err) }
 if len(oracle) != count { t.Fatalf("oracle %s: %d vectors want %d", name, len(oracle), count) }
 return oracle
}
`

func TestFiniteConditionalOracleTransport(t *testing.T) {
	oracle := []finiteConditionalOracleCase{
		{Value: "", Trace: []string{}},
		{Value: "quote\" slash\\ newline\n tab\t nul\x00 λ", Trace: []string{"C:R", "E:first", "E:first", "E:last\nλ"}},
	}
	data, err := json.Marshal(oracle)
	if err != nil {
		t.Fatal(err)
	}
	testSource := `package ` + gobackend.PackageName + `
import ("testing"; "encoding/json"; "os"; "reflect")
` + finiteConditionalOracleLoader + `
func TestOracle(t *testing.T) {
 got := loadFiniteOracle(t, "oracle.json", 2)
 want := []finiteOracleCase{
  {Value: "", Trace: []string{}},
  {Value: "quote\" slash\\ newline\n tab\t nul\x00 λ", Trace: []string{"C:R", "E:first", "E:first", "E:last\nλ"}},
 }
 if !reflect.DeepEqual(got, want) { t.Fatalf("oracle transport: %#v want %#v", got, want) }
}
`
	compileAndRunGeneratedGoFilesWithFixtures(t, []byte("package "+gobackend.PackageName), []byte(testSource), map[string][]byte{"oracle.json": data})
}

func TestV840FiniteConditionalLocalsSourceRejection(t *testing.T) {
	testConditionalLocalsSourceRejection(t, PipeLangLanguageContractV840)
}

func TestV840FiniteConditionalLocalsMalformedCore(t *testing.T) {
	testConditionalLocalsMalformedCore(t, PipeLangLanguageContractV840)
}

func TestV840FiniteConditionalLocalsTypeMatrix(t *testing.T) {
	testConditionalLocalsTypeMatrix(t, PipeLangLanguageContractV840)
}

func TestV840FiniteConditionalLocalsCarrierAndHostValues(t *testing.T) {
	testConditionalLocalsCarrierAndHostValues(t, PipeLangLanguageContractV840)
}

func TestV840FiniteConditionalLocalsDifferentTypes(t *testing.T) {
	testConditionalLocalsDifferentTypes(t, PipeLangLanguageContractV840)
}

func TestV840FiniteConditionalLocalsVersionBoundary(t *testing.T) {
	for _, contract := range []LanguageContract{PipeLangLanguageContractV810, PipeLangLanguageContractV820, PipeLangLanguageContractV830, "v0.999.0", "unknown"} {
		input := semanticTestModuleSet("compiler.selfhosting", []ModuleInput{testModule("compiler.selfhosting", "finite.pipe", finiteConditionalLocalsSource)}, nil)
		input.LanguageContract = contract
		if AnalyzeSemanticModuleSet(input).Error() == nil {
			t.Fatalf("%s admitted three choices", contract)
		}
		_, program := conditionalLocalTreeProgramVersion(t, PipeLangLanguageContractV840, finiteConditionalLocalsSource, []string{"Select"})
		if err := coreir.ValidateFunction(program.Functions[0]); err != nil {
			t.Fatalf("internal Core narrowed: %v", err)
		}
		program.LanguageContract = string(contract)
		assertAdmissionRejected(t, program, "")
	}
	// Multiple choices still require a terminal tree, even after removing the count limit.
	source := strings.Replace(finiteConditionalLocalsSource, "if (enabled) { return third; } else { return first; }", "return third;", 1)
	input := semanticTestModuleSet("compiler.selfhosting", []ModuleInput{testModule("compiler.selfhosting", "finite.pipe", source)}, nil)
	input.LanguageContract = PipeLangLanguageContractV840
	if AnalyzeSemanticModuleSet(input).Error() == nil {
		t.Fatal("straight-line multiple choices admitted")
	}
	_, program := conditionalLocalTreeProgramVersion(t, PipeLangLanguageContractV840, finiteConditionalLocalsSource, []string{"Select"})
	third := program.Functions[0].Body.ImmutableLocal.Return.ImmutableLocal.Return.ImmutableLocal
	third.Return = third.Return.Conditional.WhenTrue
	assertAdmissionRejected(t, program, "")
}

func TestV840FiniteConditionalLocalsThirdChoiceValidation(t *testing.T) {
	for _, operand := range []string{"third", "missing", "true", "(pick ? second : raw)", "propagate(second)"} {
		source := strings.Replace(finiteConditionalLocalsSource, "second + \"?\"", operand, 1)
		input := semanticTestModuleSet("compiler.selfhosting", []ModuleInput{testModule("compiler.selfhosting", "finite.pipe", source)}, nil)
		input.LanguageContract = PipeLangLanguageContractV840
		if AnalyzeSemanticModuleSet(input).Error() == nil {
			t.Fatalf("third initializer accepted %s", operand)
		}
	}
	for _, mutation := range []string{"missing", "self", "type", "position", "nested", "terminal"} {
		t.Run(mutation, func(t *testing.T) {
			_, program := conditionalLocalTreeProgramVersion(t, PipeLangLanguageContractV840, finiteConditionalLocalsSource, []string{"Select"})
			third := program.Functions[0].Body.ImmutableLocal.Return.ImmutableLocal.Return.ImmutableLocal
			switch mutation {
			case "missing":
				third.Initializer.Conditional.WhenTrue = nil
			case "self":
				third.Initializer.Conditional.WhenTrue = third.Return.Conditional.WhenTrue
			case "type":
				third.Initializer.Conditional.WhenTrue = third.Initializer.Conditional.Condition
			case "position":
				third.Position = 999
			case "nested":
				c := third.Initializer.Conditional
				c.WhenTrue = &coreir.Expr{Kind: coreir.ExprConditional, Type: third.Type, Conditional: &coreir.Conditional{Condition: c.Condition, WhenTrue: c.WhenFalse, WhenFalse: c.WhenFalse}}
			case "terminal":
				third.Initializer.Conditional.TerminalStatement = true
			}
			assertAdmissionRejected(t, program, "")
		})
	}
}

func TestV840FiniteConditionalLocalsDependentCondition(t *testing.T) {
	testConditionalLocalsDependentCondition(t, PipeLangLanguageContractV840, finiteConditionalLocalsSource)
}

func testConditionalLocalsDependentCondition(t *testing.T, contract LanguageContract, source string) {
	_, program := conditionalLocalTreeProgramVersion(t, contract, source, []string{"Select"})
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
				if second != "" {
					want += "?"
				}
			}
			args := []coreeval.Value{{Type: function.Parameters[0].Type, String: raw}}
			for bit := 0; bit < 3; bit++ {
				args = append(args, coreeval.Value{Type: function.Parameters[bit+1].Type, Bool: mask&(1<<bit) != 0})
			}
			got, err := coreeval.EvaluateProgram(program, function.Identity, args)
			standalone, se := coreeval.Evaluate(function, args)
			if err != nil || !got.OK || got.Value.String != want || se != nil || !reflect.DeepEqual(got, standalone) {
				t.Fatalf("dependent %q/%d: %#v %v / %#v %v", raw, mask, got, err, standalone, se)
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

func TestV840FiniteConditionalLocalsBoundedScale(t *testing.T) {
	// A bounded resource observation, not a claim of exhaustive or asymptotic proof.
	for _, count := range []int{3, 8, 32, 128, 256} {
		t.Run(fmt.Sprint(count), func(t *testing.T) {
			start := time.Now()
			var source strings.Builder
			source.WriteString("public Class Choices {public string Select(string raw,bool pick,bool enabled){")
			previous := "raw"
			for i := 0; i < count; i++ {
				name := fmt.Sprintf("q%d", i)
				fmt.Fprintf(&source, "string %s=pick ? %s+\"T\" : %s+\"F\";", name, previous, previous)
				previous = name
			}
			fmt.Fprintf(&source, "if(enabled){return %s;}else{return raw;}}}", previous)
			_, program := conditionalLocalTreeProgramVersion(t, PipeLangLanguageContractV840, source.String(), []string{"Select"})
			function := program.Functions[0]
			if countConditionalExpressionsInCore(function.Body) != count+1 {
				t.Fatal("choice count changed")
			}
			var checks strings.Builder
			for _, pick := range []bool{false, true} {
				for _, enabled := range []bool{false, true} {
					arm := "F"
					if pick {
						arm = "T"
					}
					want := "raw"
					if enabled {
						want += strings.Repeat(arm, count)
					}
					args := []coreeval.Value{{Type: function.Parameters[0].Type, String: "raw"}, {Type: function.Parameters[1].Type, Bool: pick}, {Type: function.Parameters[2].Type, Bool: enabled}}
					got, err := coreeval.EvaluateProgram(program, function.Identity, args)
					if err != nil || !got.OK || got.Value.String != want {
						t.Fatalf("scale: %#v %v", got, err)
					}
					fmt.Fprintf(&checks, "if got:=PipeLangSelect(\"raw\",%t,%t);got!=%q{t.Fatal(got)}\n", pick, enabled, want)
				}
			}
			generated, err := gobackend.Generate(program)
			if err != nil {
				t.Fatal(err)
			}
			compileAndRunGeneratedGoFiles(t, generated, []byte(fmt.Sprintf("package %s\nimport \"testing\"\nfunc TestScale(t *testing.T){%s}", gobackend.PackageName, checks.String())))
			t.Logf("%d choices, %d source bytes, %d Go bytes, %s including pristine Go compile/run", count, source.Len(), len(generated), time.Since(start))
		})
	}
}

func TestV840FiniteConditionalLocalsTwoChoiceInheritance(t *testing.T) {
	for _, source := range []string{twoConditionalLocalsSource, twoConditionalRulesSource} {
		var baseline [][]byte
		for _, contract := range []LanguageContract{PipeLangLanguageContractV830, PipeLangLanguageContractV840, PipeLangLanguageContractV850, PipeLangLanguageContractV860, PipeLangLanguageContractV870, PipeLangLanguageContractV880, PipeLangLanguageContractV890, PipeLangLanguageContractV900, PipeLangLanguageContractV910} {
			analysis, program := conditionalLocalTreeProgramVersion(t, contract, source, []string{"Select"})
			projection, err := BuildSemanticProjection(analysis)
			if err != nil {
				t.Fatal(err)
			}
			typed, err := LowerSemanticMethodToHIR(analysis, semanticMethodNamed(t, analysis, "Select").Identity)
			if err != nil {
				t.Fatal(err)
			}
			generated, err := gobackend.Generate(program)
			if err != nil {
				t.Fatal(err)
			}
			program.LanguageContract = coreir.LanguageContractV830
			typed.LanguageContract = coreir.LanguageContractV830
			projection.LanguageContract = PipeLangLanguageContractV830
			artifacts := [][]byte{generated}
			for _, value := range []any{program, typed, projection} {
				data, err := json.Marshal(value)
				if err != nil {
					t.Fatal(err)
				}
				artifacts = append(artifacts, data)
			}
			if baseline == nil {
				baseline = artifacts
			} else if !reflect.DeepEqual(baseline, artifacts) {
				t.Fatal("inherited two-choice HIR/Core/semantic/Go changed")
			}
		}
	}
}
