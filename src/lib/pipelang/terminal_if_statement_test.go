package pipelang

import (
	"fmt"
	"strings"
	"testing"

	"dockpipe/src/lib/pipelang/coreeval"
	"dockpipe/src/lib/pipelang/coreir"
	"dockpipe/src/lib/pipelang/gobackend"
	"dockpipe/src/lib/pipelang/hir"
)

const terminalIfStatementSource = `public Class CompilerBranch {
	public string Normalize(string raw) => trim(raw);
	public string Select(string raw, bool normalized) {
		string cleaned = Normalize(raw);
		if (normalized) { return cleaned; } else { return raw; }
	}
	public string SelectTwice(string raw, bool normalized) {
		string cleaned = Normalize(raw);
		string selected = cleaned == "" ? raw : cleaned;
		if (normalized) { return selected; } else { return raw; }
	}
}`

func terminalIfProgram(t *testing.T, method string) (*Analysis, SemanticIdentity, coreir.Program) {
	t.Helper()
	input := semanticTestModuleSet("compiler.selfhosting", []ModuleInput{testModule("compiler.selfhosting", "terminal-if.pipe", terminalIfStatementSource)}, nil)
	input.LanguageContract = PipeLangLanguageContractV690
	analysis := AnalyzeSemanticModuleSet(input)
	if err := analysis.Error(); err != nil {
		t.Fatal(err)
	}
	identity := semanticMethodNamed(t, analysis, method).Identity
	typed, err := LowerSemanticMethodToHIR(analysis, identity)
	if err != nil {
		t.Fatal(err)
	}
	program, err := LowerHIRToCore(typed)
	if err != nil {
		t.Fatal(err)
	}
	return analysis, identity, program
}

func TestV690TerminalIfStatementPipeline(t *testing.T) {
	analysis, identity, program := terminalIfProgram(t, "Select")
	projection, err := BuildSemanticProjection(analysis)
	if err != nil {
		t.Fatal(err)
	}
	if projection.LanguageContract != PipeLangLanguageContractV690 || projection.Schema != PipeLangSemanticProjectionVersion || projection.CompilerContract != PipeLangCompilerContract {
		t.Fatalf("projection contract = %#v", projection)
	}

	typed, err := LowerSemanticMethodToHIR(analysis, identity)
	if err != nil {
		t.Fatal(err)
	}
	function := hirFunctionNamed(t, typed, "Select")
	local := function.Body.ImmutableLocal
	if typed.LanguageContract != coreir.LanguageContractV690 || local == nil || local.Binding.Position != 2 || local.Initializer.Kind != hir.ExprCall || local.Return == nil || local.Return.Kind != hir.ExprConditional || local.Return.Conditional == nil || !local.Return.Conditional.TerminalStatement {
		t.Fatalf("v0.69.0 terminal-if HIR = %#v", function.Body)
	}
	conditional := local.Return.Conditional
	if conditional.Condition.Reference == nil || conditional.Condition.Reference.Position != 1 || conditional.WhenTrue.Reference == nil || conditional.WhenTrue.Reference.Position != 2 || conditional.WhenFalse.Reference == nil || conditional.WhenFalse.Reference.Position != 0 {
		t.Fatalf("v0.69.0 terminal-if HIR operands = %#v", conditional)
	}

	coreFunction := coreFunctionNamed(t, program, "Select")
	coreLocal := coreFunction.Body.ImmutableLocal
	if program.LanguageContract != coreir.LanguageContractV690 || coreLocal == nil || coreLocal.Position != 2 || coreLocal.Return == nil || coreLocal.Return.Kind != coreir.ExprConditional || coreLocal.Return.Conditional == nil || !coreLocal.Return.Conditional.TerminalStatement {
		t.Fatalf("v0.69.0 terminal-if Core = %#v", coreFunction.Body)
	}
	if err := coreir.ValidateProgram(program); err != nil {
		t.Fatal(err)
	}

	entry := coreir.SemanticIdentity{PackageID: string(identity.PackageID), Path: string(identity.Path)}
	stringType := coreFunction.Parameters[0].Type
	boolType := coreFunction.Parameters[1].Type
	selected, err := coreeval.EvaluateProgram(program, entry, []coreeval.Value{{Type: stringType, String: "  source  "}, {Type: boolType, Bool: true}})
	if err != nil || !selected.OK || selected.Value.String != "source" {
		t.Fatalf("selected = %#v, %v", selected, err)
	}
	fallback, err := coreeval.EvaluateProgram(program, entry, []coreeval.Value{{Type: stringType, String: "  source  "}, {Type: boolType, Bool: false}})
	if err != nil || !fallback.OK || fallback.Value.String != "  source  " {
		t.Fatalf("fallback = %#v, %v", fallback, err)
	}

	generated, err := gobackend.Generate(program)
	if err != nil {
		t.Fatal(err)
	}
	again, err := gobackend.Generate(program)
	if err != nil || string(again) != string(generated) {
		t.Fatalf("generated Go is not deterministic: %v", err)
	}
	for _, fragment := range []string{"p2 := PipeLangNormalize(p0)", "if p1 {", "return p2", "return p0"} {
		if !strings.Contains(string(generated), fragment) {
			t.Fatalf("generated Go lacks terminal-if fragment %q:\n%s", fragment, generated)
		}
	}
	compileAndRunGeneratedGoFiles(t, generated, []byte(fmt.Sprintf(`package %s

import "testing"

func TestGeneratedTerminalIf(t *testing.T) {
	if got := PipeLangSelect("  source  ", true); got != "source" { t.Fatalf("true = %%q", got) }
	if got := PipeLangSelect("  source  ", false); got != "  source  " { t.Fatalf("false = %%q", got) }
}

func TestGeneratedTerminalIfRejectsInvalidText(t *testing.T) {
	defer func() { if recover() == nil { t.Fatal("invalid UTF-8 did not panic") } }()
	PipeLangSelect(string([]byte{0xff}), true)
}
`, gobackend.PackageName)))
}

func TestV690TerminalIfStatementGateAndOrderedLocals(t *testing.T) {
	input := semanticTestModuleSet("compiler.selfhosting", []ModuleInput{testModule("compiler.selfhosting", "terminal-if-gate.pipe", terminalIfStatementSource)}, nil)
	input.LanguageContract = PipeLangLanguageContractV680
	if err := AnalyzeSemanticModuleSet(input).Error(); err == nil {
		t.Fatal("v0.68.0 accepted terminal if/else")
	}
	input.LanguageContract = PipeLangLanguageContractV690
	if err := AnalyzeSemanticModuleSet(input).Error(); err != nil {
		t.Fatal(err)
	}
	_, _, twoLocals := terminalIfProgram(t, "SelectTwice")
	function := coreFunctionNamed(t, twoLocals, "SelectTwice")
	if function.Body.ImmutableLocal == nil || function.Body.ImmutableLocal.Return == nil || function.Body.ImmutableLocal.Return.ImmutableLocal == nil || function.Body.ImmutableLocal.Return.ImmutableLocal.Return == nil || function.Body.ImmutableLocal.Return.ImmutableLocal.Return.Conditional == nil || !function.Body.ImmutableLocal.Return.ImmutableLocal.Return.Conditional.TerminalStatement {
		t.Fatalf("two-local terminal if = %#v", function.Body)
	}
}

func TestV690TerminalIfStatementRejectsExcludedSource(t *testing.T) {
	valid := `public Class Root { public string Select(string raw, bool choose) { string value = trim(raw); if (choose) { return value; } else { return raw; } } }`
	tests := []struct {
		name   string
		source string
	}{
		{name: "no preceding local", source: `public Class Root { public string Select(string raw, bool choose) { if (choose) { return raw; } else { return ""; } } }`},
		{name: "missing else", source: strings.Replace(valid, " else { return raw; }", "", 1)},
		{name: "non-bool condition", source: strings.Replace(valid, "if (choose)", "if (raw)", 1)},
		{name: "mismatched branch types", source: strings.Replace(valid, "return raw; } } }", "return choose; } } }", 1)},
		{name: "branch local", source: strings.Replace(valid, "{ return value; }", "{ string nested = value; return nested; }", 1)},
		{name: "nested terminal branch", source: strings.Replace(valid, "{ return value; }", "{ if (choose) { return value; } else { return raw; } }", 1)},
		{name: "two preceding expression conditionals", source: `public Class Root { public string Select(string raw, bool choose) { string first = choose ? raw : ""; string second = choose ? first : raw; if (choose) { return second; } else { return raw; } } }`},
		{name: "fallthrough return", source: strings.Replace(valid, "else { return raw; }", "return raw;", 1)},
		{name: "propagation before branch", source: `public Class Root { public Result<string, string> Select(Result<string, string> input, bool choose) { string value = propagate(input); if (choose) { return ok<string, string>(value); } else { return input; } } }`},
		{name: "match branch", source: `public Class Root { public string Select(Result<string, string> input, bool choose) { string value = ""; if (choose) { return match(input){ ok(okValue) => okValue, err(problem) => problem }; } else { return value; } } }`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			input := semanticTestModuleSet("compiler.selfhosting", []ModuleInput{testModule("compiler.selfhosting", test.name+".pipe", test.source)}, nil)
			input.LanguageContract = PipeLangLanguageContractV690
			if err := AnalyzeSemanticModuleSet(input).Error(); err == nil {
				t.Fatal("excluded v0.69.0 source was accepted")
			}
		})
	}
}

func TestV690TerminalIfStatementRejectsMalformedCoreAndBackend(t *testing.T) {
	_, _, program := terminalIfProgram(t, "Select")
	for index := range program.Functions {
		if program.Functions[index].Name == "Select" {
			program.Functions[index].Body = *program.Functions[index].Body.ImmutableLocal.Return
			parameter := 0
			program.Functions[index].Body.Conditional.WhenTrue.Parameter = &parameter
		}
	}
	if err := coreir.ValidateProgram(program); err == nil || !strings.Contains(err.Error(), "v0.69.0 terminal if/else") {
		t.Fatalf("Core validator accepted terminal if without preceding local: %v", err)
	}
	if _, err := gobackend.Generate(program); err == nil {
		t.Fatal("Go backend accepted malformed terminal-if Core")
	}
}
