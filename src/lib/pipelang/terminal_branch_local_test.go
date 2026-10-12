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

const terminalBranchLocalSource = `public Class CompilerBranchLocal {
	public string Normalize(string raw) => trim(raw);
	public string Select(string raw, bool normalized) {
		string cleaned = raw;
		if (normalized) {
			string selected = Normalize(cleaned);
			return selected;
		} else {
			return raw;
		}
	}
	public string SelectBoth(string raw, bool normalized) {
		string cleaned = raw;
		if (normalized) {
			string selected = Normalize(cleaned);
			return selected;
		} else {
			string selected = cleaned;
			return selected;
		}
	}
}`

func terminalBranchLocalProgram(t *testing.T, method string) (*Analysis, SemanticIdentity, coreir.Program) {
	t.Helper()
	input := semanticTestModuleSet("compiler.selfhosting", []ModuleInput{testModule("compiler.selfhosting", "terminal-branch-local.pipe", terminalBranchLocalSource)}, nil)
	input.LanguageContract = PipeLangLanguageContractV700
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

func TestV700TerminalBranchLocalPipeline(t *testing.T) {
	analysis, identity, program := terminalBranchLocalProgram(t, "Select")
	projection, err := BuildSemanticProjection(analysis)
	if err != nil {
		t.Fatal(err)
	}
	if projection.LanguageContract != PipeLangLanguageContractV700 || projection.Schema != PipeLangSemanticProjectionVersion || projection.CompilerContract != PipeLangCompilerContract {
		t.Fatalf("projection contract = %#v", projection)
	}

	typed, err := LowerSemanticMethodToHIR(analysis, identity)
	if err != nil {
		t.Fatal(err)
	}
	function := hirFunctionNamed(t, typed, "Select")
	top := function.Body.ImmutableLocal
	if typed.LanguageContract != coreir.LanguageContractV700 || top == nil || top.Binding.Position != 2 || top.Return == nil || top.Return.Kind != hir.ExprConditional || top.Return.Conditional == nil || !top.Return.Conditional.TerminalStatement {
		t.Fatalf("v0.70.0 terminal branch-local HIR = %#v", function.Body)
	}
	conditional := top.Return.Conditional
	branch := conditional.WhenTrue.ImmutableLocal
	if branch == nil || branch.Binding.Position != 3 || branch.Initializer == nil || branch.Initializer.Kind != hir.ExprCall || branch.Return == nil || branch.Return.Reference == nil || branch.Return.Reference.Position != 3 || conditional.WhenFalse.Reference == nil || conditional.WhenFalse.Reference.Position != 0 {
		t.Fatalf("v0.70.0 true branch local HIR = %#v", conditional)
	}

	coreFunction := coreFunctionNamed(t, program, "Select")
	coreTop := coreFunction.Body.ImmutableLocal
	coreConditional := coreTop.Return.Conditional
	coreBranch := coreConditional.WhenTrue.ImmutableLocal
	if program.LanguageContract != coreir.LanguageContractV700 || coreBranch == nil || coreBranch.Position != 3 || coreBranch.Return == nil || coreBranch.Return.Parameter == nil || *coreBranch.Return.Parameter != 3 || !coreConditional.TerminalStatement {
		t.Fatalf("v0.70.0 terminal branch-local Core = %#v", coreFunction.Body)
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
	for _, fragment := range []string{"p2 := p0", "if p1 {", "p3 := PipeLangNormalize(p2)", "return p3", "return p0"} {
		if !strings.Contains(string(generated), fragment) {
			t.Fatalf("generated Go lacks terminal branch-local fragment %q:\n%s", fragment, generated)
		}
	}
	compileAndRunGeneratedGoFiles(t, generated, []byte(fmt.Sprintf(`package %s

import "testing"

func TestGeneratedTerminalBranchLocal(t *testing.T) {
	if got := PipeLangSelect("  source  ", true); got != "source" { t.Fatalf("true = %%q", got) }
	if got := PipeLangSelect("  source  ", false); got != "  source  " { t.Fatalf("false = %%q", got) }
}
`, gobackend.PackageName)))
}

func TestV700TerminalBranchLocalsHaveIndependentScopes(t *testing.T) {
	_, identity, program := terminalBranchLocalProgram(t, "SelectBoth")
	function := coreFunctionNamed(t, program, "SelectBoth")
	conditional := function.Body.ImmutableLocal.Return.Conditional
	trueLocal := conditional.WhenTrue.ImmutableLocal
	falseLocal := conditional.WhenFalse.ImmutableLocal
	if trueLocal == nil || falseLocal == nil || trueLocal.Name != "selected" || falseLocal.Name != "selected" || trueLocal.Position != 3 || falseLocal.Position != 3 {
		t.Fatalf("branch-local scopes are not independent: %#v", conditional)
	}
	if err := coreir.ValidateProgram(program); err != nil {
		t.Fatal(err)
	}
	entry := coreir.SemanticIdentity{PackageID: string(identity.PackageID), Path: string(identity.Path)}
	stringType := function.Parameters[0].Type
	boolType := function.Parameters[1].Type
	for _, test := range []struct {
		choose bool
		want   string
	}{{choose: true, want: "source"}, {choose: false, want: "  source  "}} {
		outcome, err := coreeval.EvaluateProgram(program, entry, []coreeval.Value{{Type: stringType, String: "  source  "}, {Type: boolType, Bool: test.choose}})
		if err != nil || !outcome.OK || outcome.Value.String != test.want {
			t.Fatalf("SelectBoth(%t) = %#v, %v", test.choose, outcome, err)
		}
	}
	generated, err := gobackend.Generate(program)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(generated), "p3 :=") != 2 {
		t.Fatalf("generated Go did not preserve two independent branch-local scopes:\n%s", generated)
	}
	compileAndRunGeneratedGoFiles(t, generated, []byte(fmt.Sprintf(`package %s

import "testing"

func TestGeneratedIndependentBranchLocals(t *testing.T) {
	if got := PipeLangSelectBoth("  source  ", true); got != "source" { t.Fatalf("true = %%q", got) }
	if got := PipeLangSelectBoth("  source  ", false); got != "  source  " { t.Fatalf("false = %%q", got) }
}
`, gobackend.PackageName)))
}

func TestV700TerminalBranchLocalGateAndInheritance(t *testing.T) {
	input := semanticTestModuleSet("compiler.selfhosting", []ModuleInput{testModule("compiler.selfhosting", "terminal-branch-local-gate.pipe", terminalBranchLocalSource)}, nil)
	input.LanguageContract = PipeLangLanguageContractV690
	if err := AnalyzeSemanticModuleSet(input).Error(); err == nil {
		t.Fatal("v0.69.0 accepted terminal branch local")
	}
	input.LanguageContract = PipeLangLanguageContractV700
	if err := AnalyzeSemanticModuleSet(input).Error(); err != nil {
		t.Fatal(err)
	}

	inherited := semanticTestModuleSet("compiler.selfhosting", []ModuleInput{testModule("compiler.selfhosting", "terminal-if-inherited.pipe", terminalIfStatementSource)}, nil)
	inherited.LanguageContract = PipeLangLanguageContractV700
	if err := AnalyzeSemanticModuleSet(inherited).Error(); err != nil {
		t.Fatalf("v0.70.0 rejected inherited direct terminal branches: %v", err)
	}
}

func TestV700TerminalBranchLocalRejectsExcludedSource(t *testing.T) {
	valid := `public Class Root { public string Select(string raw, bool choose) { string value = raw; if (choose) { string selected = trim(value); return selected; } else { return raw; } } }`
	tests := []struct {
		name   string
		source string
	}{
		{name: "no preceding local", source: `public Class Root { public string Select(string raw, bool choose) { if (choose) { string selected = trim(raw); return selected; } else { return raw; } } }`},
		{name: "two true-branch locals", source: strings.Replace(valid, "return selected;", "string second = selected; return second;", 1)},
		{name: "two false-branch locals", source: strings.Replace(valid, "else { return raw; }", "else { string first = raw; string second = first; return second; }", 1)},
		{name: "nested branch", source: strings.Replace(valid, "string selected = trim(value); return selected;", "string selected = choose ? value : raw; return selected;", 1)},
		{name: "shadows parameter", source: strings.Replace(valid, "string selected = trim(value); return selected;", "string raw = trim(value); return raw;", 1)},
		{name: "shadows preceding local", source: strings.Replace(valid, "string selected = trim(value); return selected;", "string value = trim(raw); return value;", 1)},
		{name: "branch binding escapes", source: strings.Replace(valid, "else { return raw; }", "else { return selected; }", 1)},
		{name: "propagation in branch", source: `public Class Root { public Result<string, string> Select(Result<string, string> input, bool choose) { Result<string, string> value = input; if (choose) { string selected = propagate(value); return ok<string, string>(selected); } else { return input; } } }`},
		{name: "match in branch", source: `public Class Root { public string Select(Result<string, string> input, bool choose) { string value = ""; if (choose) { string selected = match(input){ ok(okValue) => okValue, err(problem) => problem }; return selected; } else { return value; } } }`},
		{name: "mismatched local type", source: strings.Replace(valid, "string selected = trim(value);", "bool selected = trim(value);", 1)},
		{name: "mismatched return type", source: strings.Replace(valid, "return selected;", "return choose;", 1)},
		{name: "fallthrough after local", source: strings.Replace(valid, "return selected;", "selected;", 1)},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			input := semanticTestModuleSet("compiler.selfhosting", []ModuleInput{testModule("compiler.selfhosting", test.name+".pipe", test.source)}, nil)
			input.LanguageContract = PipeLangLanguageContractV700
			if err := AnalyzeSemanticModuleSet(input).Error(); err == nil {
				t.Fatal("excluded v0.70.0 source was accepted")
			}
		})
	}
}

func TestV700TerminalBranchLocalRejectsMalformedCoreAndBackend(t *testing.T) {
	_, _, twoLocals := terminalBranchLocalProgram(t, "Select")
	function := coreFunctionNamed(t, twoLocals, "Select")
	first := function.Body.ImmutableLocal.Return.Conditional.WhenTrue.ImmutableLocal
	secondReturn := first.Return
	first.Return = &coreir.Expr{
		Kind: coreir.ExprImmutableLocal,
		Type: first.Type,
		ImmutableLocal: &coreir.ImmutableLocal{
			Position:    first.Position + 1,
			Name:        "second",
			Type:        first.Type,
			Initializer: first.Initializer,
			Return:      secondReturn,
		},
	}
	if err := coreir.ValidateProgram(twoLocals); err == nil || !strings.Contains(err.Error(), "v0.70.0 terminal if/else") {
		t.Fatalf("Core validator accepted two branch locals: %v", err)
	}
	if _, err := gobackend.Generate(twoLocals); err == nil {
		t.Fatal("Go backend accepted two branch locals")
	}

	_, _, oldContract := terminalBranchLocalProgram(t, "Select")
	oldContract.LanguageContract = coreir.LanguageContractV690
	if err := coreir.ValidateProgram(oldContract); err == nil {
		t.Fatal("v0.69.0 Core accepted a terminal branch local")
	}
	if _, err := gobackend.Generate(oldContract); err == nil {
		t.Fatal("Go backend accepted a v0.69.0 terminal branch local")
	}
}
