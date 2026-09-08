package pipelang

import (
	"bytes"
	"dockpipe/src/lib/pipelang/coreir"
	"dockpipe/src/lib/pipelang/gobackend"
	"encoding/json"
	"strings"
	"testing"
)

const straightLineConditionalLocalsSource = `public Class Choices {
 public string Select(string raw, bool pick, bool finish, bool enabled) {
  string first = pick ? trim(raw) : raw;
  string second = finish && first != "" ? first + "!" : first;
  string third = enabled && second != "" ? second + "?" : second;
  return third;
 }
}`

func TestV850StraightLineConditionalLocalsAdmission(t *testing.T) {
	input := semanticTestModuleSet("compiler.selfhosting", []ModuleInput{testModule("compiler.selfhosting", "straight.pipe", straightLineConditionalLocalsSource)}, nil)
	input.LanguageContract = LanguageContract("v0.85.0")
	if err := AnalyzeSemanticModuleSet(input).Error(); err != nil {
		t.Fatal(err)
	}
}

func TestV850StraightLineConditionalLocalsLayouts(t *testing.T) {
	testFiniteConditionalLocalsLayouts(t, PipeLangLanguageContractV850, []*terminalTree{nil})
}
func TestV850StraightLineConditionalLocalsTypes(t *testing.T) {
	testConditionalLocalsTypeMatrix(t, PipeLangLanguageContractV850)
	testConditionalLocalsDifferentTypes(t, PipeLangLanguageContractV850)
}
func TestV850StraightLineConditionalLocalsCarriers(t *testing.T) {
	testConditionalLocalsCarrierAndHostValues(t, PipeLangLanguageContractV850)
}
func TestV850StraightLineConditionalLocalsSourceRejection(t *testing.T) {
	for _, tc := range []struct{ name, before, after string }{
		{"condition type", "pick ? trim(raw)", "raw ? trim(raw)"},
		{"arm type", "pick ? trim(raw)", "pick ? true"},
		{"local type", "string second", "bool second"},
		{"self reference", "pick ? trim(raw)", "pick ? first"},
		{"forward reference", "pick ? trim(raw)", "pick ? third"},
		{"duplicate", "string second", "string first"},
		{"parameter shadow", "string first", "string raw"},
		{"nested", "trim(raw)", "(finish ? raw : raw)"},
		{"nested condition", "finish && first != \"\"", "(pick ? finish : enabled)"},
		{"argument placement", "pick ? trim(raw) : raw", "trim(pick ? raw : raw)"},
		{"return placement", "return third;", "return pick ? third : raw;"},
		{"propagation", "trim(raw)", "propagate(raw)"},
		{"matching", "trim(raw)", "match(raw){some(value)=>value,none=>raw}"},
		{"assignment", "return third;", "first=third;return third;"},
		{"inference", "string second", "var second"},
		{"fallthrough", "return third;", ""},
		{"early return", "string second", "return first;string second"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source := strings.Replace(straightLineConditionalLocalsSource, tc.before, tc.after, 1)
			if source == straightLineConditionalLocalsSource {
				t.Fatal("mutation missed")
			}
			input := semanticTestModuleSet("compiler.selfhosting", []ModuleInput{testModule("compiler.selfhosting", "straight.pipe", source)}, nil)
			input.LanguageContract = PipeLangLanguageContractV850
			if AnalyzeSemanticModuleSet(input).Error() == nil {
				t.Fatal("excluded source accepted")
			}
		})
	}
}
func TestV850StraightLineConditionalLocalsMalformedCore(t *testing.T) {
	for _, mutation := range []string{"missing local", "missing initializer", "missing continuation", "missing choice", "missing condition", "missing arm", "type", "condition type", "local type", "self", "forward", "position", "duplicate", "nested", "terminal", "return placement", "argument placement"} {
		t.Run(mutation, func(t *testing.T) {
			_, program := conditionalLocalTreeProgramVersion(t, PipeLangLanguageContractV850, straightLineConditionalLocalsSource, []string{"Select"})
			root := &program.Functions[0].Body
			first := root.ImmutableLocal
			second := first.Return.ImmutableLocal
			third := second.Return.ImmutableLocal
			switch mutation {
			case "missing local":
				root.ImmutableLocal = nil
			case "missing initializer":
				third.Initializer = nil
			case "missing continuation":
				third.Return = nil
			case "missing choice":
				third.Initializer.Conditional = nil
			case "missing condition":
				third.Initializer.Conditional.Condition = nil
			case "missing arm":
				third.Initializer.Conditional.WhenTrue = nil
			case "type":
				third.Initializer.Conditional.WhenTrue = third.Initializer.Conditional.Condition
			case "condition type":
				third.Initializer.Conditional.Condition = third.Initializer.Conditional.WhenTrue
			case "local type":
				third.Type = coreir.Type{Kind: coreir.TypePrimitive, Primitive: coreir.PrimitiveBool}
			case "self":
				third.Initializer.Conditional.WhenTrue = third.Return
			case "forward":
				first.Initializer.Conditional.WhenTrue = third.Return
			case "position":
				third.Position = 999
			case "duplicate":
				third.Name = first.Name
			case "nested":
				c := third.Initializer.Conditional
				c.WhenTrue = &coreir.Expr{Kind: coreir.ExprConditional, Type: third.Type, Conditional: &coreir.Conditional{Condition: c.Condition, WhenTrue: c.WhenFalse, WhenFalse: c.WhenFalse}}
			case "terminal":
				third.Initializer.Conditional.TerminalStatement = true
			case "return placement":
				c := third.Initializer
				third.Initializer = c.Conditional.WhenTrue
				third.Return = c
			case "argument placement":
				c := third.Initializer
				third.Initializer = &coreir.Expr{Kind: coreir.ExprTextTrim, Type: third.Type, TextTrim: &coreir.TextTrim{Value: c}}
			}
			assertAdmissionRejected(t, program, "")
		})
	}
}
func TestV850StraightLineConditionalLocalsVersionBoundary(t *testing.T) {
	for _, contract := range []LanguageContract{PipeLangLanguageContractV380, PipeLangLanguageContractV400, PipeLangLanguageContractV830, PipeLangLanguageContractV840, "v0.101.0", "unknown"} {
		input := semanticTestModuleSet("compiler.selfhosting", []ModuleInput{testModule("compiler.selfhosting", "straight.pipe", straightLineConditionalLocalsSource)}, nil)
		input.LanguageContract = contract
		if AnalyzeSemanticModuleSet(input).Error() == nil {
			t.Fatalf("%s admitted new form", contract)
		}
		_, program := conditionalLocalTreeProgramVersion(t, PipeLangLanguageContractV850, straightLineConditionalLocalsSource, []string{"Select"})
		if err := coreir.ValidateFunction(program.Functions[0]); err != nil {
			t.Fatal(err)
		}
		program.LanguageContract = string(contract)
		assertAdmissionRejected(t, program, "")
	}
}
func TestV850StraightLineConditionalLocalsTreeInheritance(t *testing.T) {
	for _, tree := range terminalTrees(3)[1:] {
		source := `public Class Choices {public string Echo(string value)=>value;public bool Check(string path,bool value)=>value;` + conditionalChoicesTreeMethod(tree, []string{"R", "RT", "RF", "R"}, false, "Select") + `}`
		var baseline [][]byte
		for _, contract := range []LanguageContract{PipeLangLanguageContractV840, PipeLangLanguageContractV850, PipeLangLanguageContractV860, PipeLangLanguageContractV870, PipeLangLanguageContractV880, PipeLangLanguageContractV890, PipeLangLanguageContractV900, PipeLangLanguageContractV910} {
			analysis, program := conditionalLocalTreeProgramVersion(t, contract, source, []string{"Select"})
			semantic, err := BuildSemanticProjection(analysis)
			if err != nil {
				t.Fatal(err)
			}
			generated, err := gobackend.Generate(program)
			if err != nil {
				t.Fatal(err)
			}
			program.LanguageContract = coreir.LanguageContractV840
			semantic.LanguageContract = PipeLangLanguageContractV840
			artifacts := [][]byte{generated}
			for _, value := range []any{program, semantic} {
				data, err := json.Marshal(value)
				if err != nil {
					t.Fatal(err)
				}
				artifacts = append(artifacts, data)
			}
			if baseline == nil {
				baseline = artifacts
			} else {
				for i := range artifacts {
					if !bytes.Equal(baseline[i], artifacts[i]) {
						t.Fatal("inherited artifact changed")
					}
				}
			}
		}
	}
}
