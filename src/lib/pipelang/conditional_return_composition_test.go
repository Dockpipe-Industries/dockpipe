package pipelang

import (
	"dockpipe/src/lib/pipelang/coreir"
	"dockpipe/src/lib/pipelang/gobackend"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

const conditionalReturnCompositionSource = `public Class Choices {
 public string Select(string raw, bool pick, bool finish, bool enabled) {
  string first = pick ? trim(raw) : raw;
  string second = finish && first != "" ? first + "!" : first;
  string third = enabled && second != "" ? second + "?" : second;
  return enabled && third != "" ? third : first;
 }
}`

func TestV860ConditionalReturnCompositionAdmission(t *testing.T) {
	input := semanticTestModuleSet("compiler.selfhosting", []ModuleInput{testModule("compiler.selfhosting", "return.pipe", conditionalReturnCompositionSource)}, nil)
	input.LanguageContract = LanguageContract("v0.86.0")
	if err := AnalyzeSemanticModuleSet(input).Error(); err != nil {
		t.Fatal(err)
	}
}

func TestV860ConditionalReturnCompositionLayouts(t *testing.T) {
	testFiniteConditionalLocalsLayouts(t, PipeLangLanguageContractV860, []*terminalTree{nil}, true)
}
func TestV860ConditionalReturnCompositionTypes(t *testing.T) {
	testConditionalLocalsTypeMatrix(t, PipeLangLanguageContractV860)
	testConditionalLocalsDifferentTypes(t, PipeLangLanguageContractV860)
}
func TestV860ConditionalReturnCompositionCarriers(t *testing.T) {
	testConditionalLocalsCarrierAndHostValues(t, PipeLangLanguageContractV860)
}

func TestV860ConditionalReturnCompositionSourceRejection(t *testing.T) {
	for _, tc := range []struct{ name, before, after string }{
		{"return condition type", "enabled && third != \"\" ? third", "third ? third"},
		{"return arm type", "? third : first", "? true : first"},
		{"method type", "public string Select", "public bool Select"},
		{"missing return arm", "? third : first", "? third :"},
		{"missing return condition", "enabled && third != \"\" ? third", "? third"},
		{"unknown return reference", "? third : first", "? missing : first"},
		{"nested true arm", "? third : first", "? (pick ? third : first) : first"},
		{"nested false arm", "? third : first", "? third : (pick ? third : first)"},
		{"nested return condition", "enabled && third != \"\" ? third", "(pick ? enabled : finish) ? third"},
		{"nested initializer", "trim(raw)", "(finish ? raw : raw)"},
		{"return argument placement", "return enabled && third != \"\" ? third : first;", "return trim(enabled ? third : first);"},
		{"initializer argument placement", "pick ? trim(raw) : raw", "trim(pick ? raw : raw)"},
		{"self reference", "pick ? trim(raw)", "pick ? first"},
		{"forward reference", "pick ? trim(raw)", "pick ? third"},
		{"duplicate", "string second", "string first"},
		{"shadow", "string first", "string raw"},
		{"local type", "string second", "bool second"},
		{"return propagate", "? third : first", "? propagate(third) : first"},
		{"return match", "? third : first", "? match(third){some(value)=>value,none=>raw} : first"},
		{"initializer propagate", "trim(raw)", "propagate(raw)"},
		{"inference", "string second", "var second"},
		{"assignment", "return enabled", "first=third;return enabled"},
		{"early return", "string second", "return first;string second"},
		{"fallthrough", "return enabled && third != \"\" ? third : first;", ""},
		{"terminal tree placement", "return enabled && third != \"\" ? third : first;", "if(enabled){return pick ? third : first;}else{return first;}"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source := strings.Replace(conditionalReturnCompositionSource, tc.before, tc.after, 1)
			if source == conditionalReturnCompositionSource {
				t.Fatal("mutation missed")
			}
			input := semanticTestModuleSet("compiler.selfhosting", []ModuleInput{testModule("compiler.selfhosting", "return.pipe", source)}, nil)
			input.LanguageContract = PipeLangLanguageContractV860
			analysis := AnalyzeSemanticModuleSet(input)
			diagnostics, ok := AsDiagnostics(analysis.Error())
			if !ok || len(diagnostics) == 0 {
				t.Fatal("missing source diagnostics")
			}
			for _, d := range diagnostics {
				if !d.Primary.IsValid() || d.Primary.File != "return.pipe" || d.Primary.End > len(source) {
					t.Fatalf("invalid diagnostic span: %#v", d)
				}
			}
		})
	}
	// The inherited terminal-tree exclusion is independently retained.
	testConditionalLocalsSourceRejection(t, PipeLangLanguageContractV860)
}

func TestV860ConditionalReturnCompositionMalformedCore(t *testing.T) {
	for _, mutation := range []string{"missing local", "missing initializer", "missing continuation", "missing return choice", "missing condition", "missing true", "missing false", "condition type", "arm type", "return type", "self", "forward", "position", "duplicate", "nested initializer", "nested return", "nested condition", "terminal initializer", "return argument placement", "initializer argument placement", "unknown return reference"} {
		t.Run(mutation, func(t *testing.T) {
			_, program := conditionalLocalTreeProgramVersion(t, PipeLangLanguageContractV860, conditionalReturnCompositionSource, []string{"Select"})
			f := &program.Functions[0]
			root := &f.Body
			first := root.ImmutableLocal
			second := first.Return.ImmutableLocal
			third := second.Return.ImmutableLocal
			returned := third.Return
			c := returned.Conditional
			switch mutation {
			case "missing local":
				root.ImmutableLocal = nil
			case "missing initializer":
				third.Initializer = nil
			case "missing continuation":
				third.Return = nil
			case "missing return choice":
				returned.Conditional = nil
			case "missing condition":
				c.Condition = nil
			case "missing true":
				c.WhenTrue = nil
			case "missing false":
				c.WhenFalse = nil
			case "condition type":
				c.Condition = c.WhenTrue
			case "arm type":
				c.WhenTrue = c.Condition
			case "return type":
				returned.Type = c.Condition.Type
			case "self":
				third.Initializer.Conditional.WhenTrue = c.WhenTrue
			case "forward":
				first.Initializer.Conditional.WhenTrue = c.WhenTrue
			case "position":
				third.Position = 999
			case "duplicate":
				third.Name = first.Name
			case "nested initializer":
				third.Initializer.Conditional.WhenTrue = returned
			case "nested return":
				c.WhenTrue = &coreir.Expr{Kind: coreir.ExprConditional, Type: returned.Type, Conditional: &coreir.Conditional{Condition: c.Condition, WhenTrue: c.WhenTrue, WhenFalse: c.WhenFalse}}
			case "nested condition":
				c.Condition = &coreir.Expr{Kind: coreir.ExprConditional, Type: c.Condition.Type, Conditional: &coreir.Conditional{Condition: c.Condition, WhenTrue: c.Condition, WhenFalse: c.Condition}}
			case "terminal initializer":
				third.Initializer.Conditional.TerminalStatement = true
			case "return argument placement":
				third.Return = &coreir.Expr{Kind: coreir.ExprTextTrim, Type: returned.Type, TextTrim: &coreir.TextTrim{Value: returned}}
			case "initializer argument placement":
				init := third.Initializer
				third.Initializer = &coreir.Expr{Kind: coreir.ExprTextTrim, Type: init.Type, TextTrim: &coreir.TextTrim{Value: init}}
			case "unknown return reference":
				copy := *c.WhenTrue
				ref := 999
				copy.Parameter = &ref
				c.WhenTrue = &copy
			}
			assertAdmissionRejected(t, program, "")
		})
	}
	testConditionalLocalsMalformedCore(t, PipeLangLanguageContractV860)
}

func TestV860ConditionalReturnCompositionVersionBoundary(t *testing.T) {
	for _, contract := range []LanguageContract{PipeLangLanguageContractV380, PipeLangLanguageContractV400, PipeLangLanguageContractV830, PipeLangLanguageContractV840, PipeLangLanguageContractV850, "v0.101.0", "unknown"} {
		input := semanticTestModuleSet("compiler.selfhosting", []ModuleInput{testModule("compiler.selfhosting", "return.pipe", conditionalReturnCompositionSource)}, nil)
		input.LanguageContract = contract
		if AnalyzeSemanticModuleSet(input).Error() == nil {
			t.Fatalf("%s admitted return composition", contract)
		}
		_, program := conditionalLocalTreeProgramVersion(t, PipeLangLanguageContractV860, conditionalReturnCompositionSource, []string{"Select"})
		if err := coreir.ValidateFunction(program.Functions[0]); err != nil {
			t.Fatalf("internal Core: %v", err)
		}
		program.LanguageContract = string(contract)
		assertAdmissionRejected(t, program, "")
	}
}

func TestV860ConditionalReturnCompositionInheritance(t *testing.T) {
	for _, source := range []string{straightLineConditionalLocalsSource, finiteConditionalLocalsSource, twoConditionalLocalsSource,
		`public Class Choices {public string Select(string raw,bool pick){string a=trim(raw);return pick ? a : raw;}}`,
		`public Class Choices {public string Select(string raw,bool pick)=>pick ? raw : trim(raw);}`} {
		var baseline [][]byte
		for _, contract := range []LanguageContract{PipeLangLanguageContractV850, PipeLangLanguageContractV860, PipeLangLanguageContractV870, PipeLangLanguageContractV880, PipeLangLanguageContractV890, PipeLangLanguageContractV900, PipeLangLanguageContractV910} {
			analysis, program := conditionalLocalTreeProgramVersion(t, contract, source, []string{"Select"})
			typed, err := LowerSemanticMethodToHIR(analysis, semanticMethodNamed(t, analysis, "Select").Identity)
			if err != nil {
				t.Fatal(err)
			}
			semantic, err := BuildSemanticProjection(analysis)
			if err != nil {
				t.Fatal(err)
			}
			generated, err := gobackend.Generate(program)
			if err != nil {
				t.Fatal(err)
			}
			typed.LanguageContract = coreir.LanguageContractV850
			program.LanguageContract = coreir.LanguageContractV850
			semantic.LanguageContract = PipeLangLanguageContractV850
			artifacts := [][]byte{generated}
			for _, value := range []any{typed, program, semantic} {
				b, err := json.Marshal(value)
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

func TestV860ConditionalReturnCompositionDependentCondition(t *testing.T) {
	testConditionalLocalsDependentCondition(t, PipeLangLanguageContractV860, conditionalReturnCompositionSource)
}

func TestV860ConditionalReturnCompositionRepresentations(t *testing.T) {
	analysis, program := conditionalLocalTreeProgramVersion(t, PipeLangLanguageContractV860, conditionalReturnCompositionSource, []string{"Select"})
	typed, err := LowerSemanticMethodToHIR(analysis, semanticMethodNamed(t, analysis, "Select").Identity)
	if err != nil {
		t.Fatal(err)
	}
	h := typed.Functions[len(typed.Functions)-1].Body
	c := program.Functions[0].Body
	for i := 0; i < 3; i++ {
		if h.ImmutableLocal == nil || c.ImmutableLocal == nil {
			t.Fatal("local sequence missing")
		}
		if h.ImmutableLocal.Initializer.Conditional == nil || c.ImmutableLocal.Initializer.Conditional == nil {
			t.Fatal("initializer choice missing")
		}
		h = *h.ImmutableLocal.Return
		c = *c.ImmutableLocal.Return
	}
	if h.Conditional == nil || h.Conditional.TerminalStatement || c.Conditional == nil || c.Conditional.TerminalStatement {
		t.Fatal("return choice lost value placement")
	}
	// Setting only TerminalStatement yields an already admitted v0.85 tree.
	// Rejecting it would incorrectly narrow inherited/internal Core capabilities.
	c.Conditional.TerminalStatement = true
	program.LanguageContract = coreir.LanguageContractV850
	if err := coreir.ValidateProgram(program); err != nil {
		t.Fatalf("inherited terminal tree narrowed: %v", err)
	}
}
