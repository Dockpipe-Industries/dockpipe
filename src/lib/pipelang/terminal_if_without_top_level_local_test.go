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

const terminalIfWithoutTopLevelLocalSource = `public Class CompilerDirectTerminalBranch {
	public string Normalize(string raw) => trim(raw);
	public string Select(string raw, bool normalized) {
		if (normalized) {
			string cleaned = trim(raw);
			string selected = Normalize(cleaned);
			return selected;
		} else {
			string first = raw;
			string selected = first;
			return selected;
		}
	}
	public string Direct(string raw, bool normalized) {
		if (normalized) { return trim(raw); }
		else { return raw; }
	}
}`

func terminalIfWithoutTopLevelLocalProgram(t *testing.T, method string) (*Analysis, SemanticIdentity, coreir.Program) {
	t.Helper()
	input := semanticTestModuleSet("compiler.selfhosting", []ModuleInput{testModule("compiler.selfhosting", "direct-terminal-branch.pipe", terminalIfWithoutTopLevelLocalSource)}, nil)
	input.LanguageContract = PipeLangLanguageContractV730
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

func TestV730TerminalIfWithoutTopLevelLocalPipeline(t *testing.T) {
	analysis, identity, program := terminalIfWithoutTopLevelLocalProgram(t, "Select")
	projection, err := BuildSemanticProjection(analysis)
	if err != nil {
		t.Fatal(err)
	}
	if projection.LanguageContract != PipeLangLanguageContractV730 || projection.Schema != PipeLangSemanticProjectionVersion || projection.CompilerContract != PipeLangCompilerContract {
		t.Fatalf("projection contract = %#v", projection)
	}

	typed, err := LowerSemanticMethodToHIR(analysis, identity)
	if err != nil {
		t.Fatal(err)
	}
	function := hirFunctionNamed(t, typed, "Select")
	conditional := function.Body.Conditional
	if typed.LanguageContract != coreir.LanguageContractV730 || function.Body.Kind != hir.ExprConditional || conditional == nil || !conditional.TerminalStatement {
		t.Fatalf("v0.73.0 direct terminal if/else HIR = %#v", function.Body)
	}
	for branchName, branch := range map[string]*hir.Expr{"true": conditional.WhenTrue, "false": conditional.WhenFalse} {
		for _, position := range []int{2, 3} {
			if branch == nil || branch.ImmutableLocal == nil || branch.ImmutableLocal.Binding.Position != position {
				t.Fatalf("v0.73.0 %s branch position %d = %#v", branchName, position, branch)
			}
			branch = branch.ImmutableLocal.Return
		}
		if branch == nil || branch.Reference == nil || branch.Reference.Position != 3 {
			t.Fatalf("v0.73.0 %s branch return = %#v", branchName, branch)
		}
	}

	coreFunction := coreFunctionNamed(t, program, "Select")
	coreConditional := coreFunction.Body.Conditional
	if program.LanguageContract != coreir.LanguageContractV730 || coreFunction.Body.Kind != coreir.ExprConditional || coreConditional == nil || !coreConditional.TerminalStatement {
		t.Fatalf("v0.73.0 direct terminal if/else Core = %#v", coreFunction.Body)
	}
	if err := coreir.ValidateProgram(program); err != nil {
		t.Fatal(err)
	}

	entry := coreir.SemanticIdentity{PackageID: string(identity.PackageID), Path: string(identity.Path)}
	stringType := coreFunction.Parameters[0].Type
	boolType := coreFunction.Parameters[1].Type
	for _, test := range []struct {
		choose bool
		want   string
	}{{choose: true, want: "source"}, {choose: false, want: "  source  "}} {
		outcome, err := coreeval.EvaluateProgram(program, entry, []coreeval.Value{{Type: stringType, String: "  source  "}, {Type: boolType, Bool: test.choose}})
		if err != nil || !outcome.OK || outcome.Value.String != test.want {
			t.Fatalf("Select(%t) = %#v, %v", test.choose, outcome, err)
		}
	}

	generated, err := gobackend.Generate(program)
	if err != nil {
		t.Fatal(err)
	}
	again, err := gobackend.Generate(program)
	if err != nil || string(again) != string(generated) {
		t.Fatalf("generated Go is not deterministic: %v", err)
	}
	for _, fragment := range []string{"if p1 {", "p2 := pipelangTrimText(p0)", "p3 := PipeLangNormalize(p2)", "return p3"} {
		if !strings.Contains(string(generated), fragment) {
			t.Fatalf("generated Go lacks direct terminal branch fragment %q:\n%s", fragment, generated)
		}
	}
	compileAndRunGeneratedGoFiles(t, generated, []byte(fmt.Sprintf(`package %s

import "testing"

func TestGeneratedDirectTerminalBranch(t *testing.T) {
	if got := PipeLangSelect("  source  ", true); got != "source" { t.Fatalf("true = %%q", got) }
	if got := PipeLangSelect("  source  ", false); got != "  source  " { t.Fatalf("false = %%q", got) }
}
`, gobackend.PackageName)))
}

func TestV730DirectReturnTerminalIfWithoutTopLevelLocal(t *testing.T) {
	_, identity, program := terminalIfWithoutTopLevelLocalProgram(t, "Direct")
	function := coreFunctionNamed(t, program, "Direct")
	if function.Body.Kind != coreir.ExprConditional || function.Body.Conditional == nil || function.Body.Conditional.WhenTrue == nil || function.Body.Conditional.WhenTrue.Kind == coreir.ExprImmutableLocal || function.Body.Conditional.WhenFalse == nil || function.Body.Conditional.WhenFalse.Kind == coreir.ExprImmutableLocal {
		t.Fatalf("v0.73.0 direct-return branches = %#v", function.Body)
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
			t.Fatalf("Direct(%t) = %#v, %v", test.choose, outcome, err)
		}
	}
	generated, err := gobackend.Generate(program)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(generated), "if p1 {") || !strings.Contains(string(generated), "return pipelangTrimText(p0)") || !strings.Contains(string(generated), "return p0") {
		t.Fatalf("generated Go lost direct-return terminal branches:\n%s", generated)
	}
	compileAndRunGeneratedGoFiles(t, generated, []byte(fmt.Sprintf(`package %s

import "testing"

func TestGeneratedDirectReturnTerminalBranch(t *testing.T) {
	if got := PipeLangDirect("  source  ", true); got != "source" { t.Fatalf("true = %%q", got) }
	if got := PipeLangDirect("  source  ", false); got != "  source  " { t.Fatalf("false = %%q", got) }
}
`, gobackend.PackageName)))
}

func TestV730TerminalIfWithoutTopLevelLocalGateAndInheritance(t *testing.T) {
	input := semanticTestModuleSet("compiler.selfhosting", []ModuleInput{testModule("compiler.selfhosting", "direct-terminal-branch-gate.pipe", terminalIfWithoutTopLevelLocalSource)}, nil)
	input.LanguageContract = PipeLangLanguageContractV720
	if err := AnalyzeSemanticModuleSet(input).Error(); err == nil {
		t.Fatal("v0.72.0 accepted terminal if/else without a top-level local")
	}
	input.LanguageContract = PipeLangLanguageContractV730
	if err := AnalyzeSemanticModuleSet(input).Error(); err != nil {
		t.Fatal(err)
	}

	inherited := semanticTestModuleSet("compiler.selfhosting", []ModuleInput{testModule("compiler.selfhosting", "terminal-branch-local-general-sequence-inherited.pipe", terminalBranchLocalGeneralSequenceSource)}, nil)
	inherited.LanguageContract = PipeLangLanguageContractV730
	if err := AnalyzeSemanticModuleSet(inherited).Error(); err != nil {
		t.Fatalf("v0.73.0 rejected inherited top-level and branch-local sequences: %v", err)
	}
}

func TestV730TerminalIfWithoutTopLevelLocalRejectsExcludedSource(t *testing.T) {
	valid := `public Class Root { public string Select(string raw, bool choose) { if (choose) { string selected = trim(raw); return selected; } else { return raw; } } }`
	tests := []struct {
		name   string
		source string
	}{
		{name: "ordinary zero-local block", source: `public Class Root { public string Select(string raw) { return raw; } }`},
		{name: "duplicate branch local", source: strings.Replace(valid, "return selected;", "string selected = selected; return selected;", 1)},
		{name: "nested branch", source: `public Class Root { public string Select(string raw, bool choose) { if (choose) { if (choose) { return trim(raw); } else { return raw; } } else { return raw; } } }`},
		{name: "propagation in branch", source: `public Class Root { public Result<string, string> Select(Result<string, string> input, bool choose) { if (choose) { string selected = propagate(input); return ok<string, string>(selected); } else { return input; } } }`},
		{name: "match in branch", source: `public Class Root { public string Select(Result<string, string> input, bool choose) { if (choose) { string selected = match(input){ ok(value) => value, err(problem) => problem }; return selected; } else { return ""; } } }`},
		{name: "missing else", source: `public Class Root { public string Select(string raw, bool choose) { if (choose) { return raw; } } }`},
		{name: "fallthrough", source: strings.Replace(valid, "return selected;", "selected;", 1)},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			input := semanticTestModuleSet("compiler.selfhosting", []ModuleInput{testModule("compiler.selfhosting", test.name+".pipe", test.source)}, nil)
			input.LanguageContract = PipeLangLanguageContractV730
			if err := AnalyzeSemanticModuleSet(input).Error(); err == nil {
				t.Fatal("excluded v0.73.0 source was accepted")
			}
		})
	}
}

func TestV730TerminalIfWithoutTopLevelLocalRejectsMalformedCoreAndBackend(t *testing.T) {
	_, _, oldContract := terminalIfWithoutTopLevelLocalProgram(t, "Select")
	oldContract.LanguageContract = coreir.LanguageContractV720
	if err := coreir.ValidateProgram(oldContract); err == nil {
		t.Fatal("v0.72.0 Core accepted terminal if/else without a top-level local")
	}
	if _, err := gobackend.Generate(oldContract); err == nil {
		t.Fatal("Go backend accepted a v0.72.0 terminal if/else without a top-level local")
	}

	_, _, nested := terminalIfWithoutTopLevelLocalProgram(t, "Direct")
	function := coreFunctionNamed(t, nested, "Direct")
	outer := function.Body.Conditional
	inner := *outer
	innerExpr := coreir.Expr{Kind: coreir.ExprConditional, Type: function.ReturnType, Conditional: &inner}
	outer.WhenTrue = &innerExpr
	if err := coreir.ValidateProgram(nested); err == nil || !strings.Contains(err.Error(), "nested conditionals") {
		t.Fatalf("Core validator accepted nested terminal branching: %v", err)
	}
	if _, err := gobackend.Generate(nested); err == nil {
		t.Fatal("Go backend accepted nested terminal branching")
	}
}
