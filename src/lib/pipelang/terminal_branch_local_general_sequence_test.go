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

const terminalBranchLocalGeneralSequenceSource = `public Class CompilerBranchLocalGeneralSequence {
	public string Normalize(string raw) => trim(raw);
	public string Select(string raw, bool normalized) {
		string cleaned = raw;
		if (normalized) {
			string first = trim(cleaned);
			string second = Normalize(first);
			string third = second;
			string selected = third;
			return selected;
		} else {
			string first = cleaned;
			string second = first;
			string selected = second;
			return selected;
		}
	}
}`

func terminalBranchLocalGeneralSequenceProgram(t *testing.T) (*Analysis, SemanticIdentity, coreir.Program) {
	t.Helper()
	input := semanticTestModuleSet("compiler.selfhosting", []ModuleInput{testModule("compiler.selfhosting", "terminal-branch-local-general-sequence.pipe", terminalBranchLocalGeneralSequenceSource)}, nil)
	input.LanguageContract = PipeLangLanguageContractV720
	analysis := AnalyzeSemanticModuleSet(input)
	if err := analysis.Error(); err != nil {
		t.Fatal(err)
	}
	identity := semanticMethodNamed(t, analysis, "Select").Identity
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

func TestV720TerminalBranchLocalGeneralSequencePipeline(t *testing.T) {
	analysis, identity, program := terminalBranchLocalGeneralSequenceProgram(t)
	projection, err := BuildSemanticProjection(analysis)
	if err != nil {
		t.Fatal(err)
	}
	if projection.LanguageContract != PipeLangLanguageContractV720 || projection.Schema != PipeLangSemanticProjectionVersion || projection.CompilerContract != PipeLangCompilerContract {
		t.Fatalf("projection contract = %#v", projection)
	}

	typed, err := LowerSemanticMethodToHIR(analysis, identity)
	if err != nil {
		t.Fatal(err)
	}
	function := hirFunctionNamed(t, typed, "Select")
	top := function.Body.ImmutableLocal
	if typed.LanguageContract != coreir.LanguageContractV720 || top == nil || top.Binding.Position != 2 || top.Return == nil || top.Return.Kind != hir.ExprConditional || top.Return.Conditional == nil || !top.Return.Conditional.TerminalStatement {
		t.Fatalf("v0.72.0 terminal branch-local general sequence HIR = %#v", function.Body)
	}
	truePositions := []int{3, 4, 5, 6}
	trueBranch := top.Return.Conditional.WhenTrue
	for _, position := range truePositions {
		if trueBranch == nil || trueBranch.ImmutableLocal == nil || trueBranch.ImmutableLocal.Binding.Position != position {
			t.Fatalf("v0.72.0 true branch position %d = %#v", position, trueBranch)
		}
		trueBranch = trueBranch.ImmutableLocal.Return
	}
	if trueBranch == nil || trueBranch.Reference == nil || trueBranch.Reference.Position != 6 {
		t.Fatalf("v0.72.0 true branch return = %#v", trueBranch)
	}
	falsePositions := []int{3, 4, 5}
	falseBranch := top.Return.Conditional.WhenFalse
	for _, position := range falsePositions {
		if falseBranch == nil || falseBranch.ImmutableLocal == nil || falseBranch.ImmutableLocal.Binding.Position != position {
			t.Fatalf("v0.72.0 false branch position %d = %#v", position, falseBranch)
		}
		falseBranch = falseBranch.ImmutableLocal.Return
	}
	if falseBranch == nil || falseBranch.Reference == nil || falseBranch.Reference.Position != 5 {
		t.Fatalf("v0.72.0 false branch return = %#v", falseBranch)
	}

	coreFunction := coreFunctionNamed(t, program, "Select")
	conditional := coreFunction.Body.ImmutableLocal.Return.Conditional
	trueCore := conditional.WhenTrue
	for _, position := range truePositions {
		if trueCore == nil || trueCore.ImmutableLocal == nil || trueCore.ImmutableLocal.Position != position {
			t.Fatalf("v0.72.0 true Core position %d = %#v", position, trueCore)
		}
		trueCore = trueCore.ImmutableLocal.Return
	}
	if program.LanguageContract != coreir.LanguageContractV720 || trueCore == nil || trueCore.Parameter == nil || *trueCore.Parameter != 6 {
		t.Fatalf("v0.72.0 terminal branch-local general sequence Core = %#v", coreFunction.Body)
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
	for _, fragment := range []string{"if p1 {", "p3 := pipelangTrimText(p2)", "p4 := PipeLangNormalize(p3)", "p5 := p4", "p6 := p5", "return p6"} {
		if !strings.Contains(string(generated), fragment) {
			t.Fatalf("generated Go lacks terminal branch-local general sequence fragment %q:\n%s", fragment, generated)
		}
	}
	compileAndRunGeneratedGoFiles(t, generated, []byte(fmt.Sprintf(`package %s

import "testing"

func TestGeneratedTerminalBranchLocalGeneralSequence(t *testing.T) {
	if got := PipeLangSelect("  source  ", true); got != "source" { t.Fatalf("true = %%q", got) }
	if got := PipeLangSelect("  source  ", false); got != "  source  " { t.Fatalf("false = %%q", got) }
}
`, gobackend.PackageName)))
}

func TestV720TerminalBranchLocalGeneralSequenceGateAndInheritance(t *testing.T) {
	input := semanticTestModuleSet("compiler.selfhosting", []ModuleInput{testModule("compiler.selfhosting", "terminal-branch-local-general-sequence-gate.pipe", terminalBranchLocalGeneralSequenceSource)}, nil)
	input.LanguageContract = PipeLangLanguageContractV710
	if err := AnalyzeSemanticModuleSet(input).Error(); err == nil {
		t.Fatal("v0.71.0 accepted a general terminal branch-local sequence")
	}
	input.LanguageContract = PipeLangLanguageContractV720
	if err := AnalyzeSemanticModuleSet(input).Error(); err != nil {
		t.Fatal(err)
	}

	inherited := semanticTestModuleSet("compiler.selfhosting", []ModuleInput{testModule("compiler.selfhosting", "terminal-branch-local-sequence-inherited.pipe", terminalBranchLocalSequenceSource)}, nil)
	inherited.LanguageContract = PipeLangLanguageContractV720
	if err := AnalyzeSemanticModuleSet(inherited).Error(); err != nil {
		t.Fatalf("v0.72.0 rejected inherited two-local terminal branches: %v", err)
	}
}

func TestV720TerminalBranchLocalGeneralSequenceRejectsExcludedSource(t *testing.T) {
	valid := `public Class Root { public string Select(string raw, bool choose) { string value = raw; if (choose) { string first = trim(value); string second = first; string third = second; return third; } else { return raw; } } }`
	tests := []struct {
		name   string
		source string
	}{
		{name: "no preceding local", source: `public Class Root { public string Select(string raw, bool choose) { if (choose) { string first = trim(raw); string second = first; string third = second; return third; } else { return raw; } } }`},
		{name: "duplicate branch local", source: strings.Replace(valid, "string third = second; return third;", "string first = second; return first;", 1)},
		{name: "shadows preceding local", source: strings.Replace(valid, "string first = trim(value);", "string value = trim(raw);", 1)},
		{name: "branch binding escapes", source: strings.Replace(valid, "else { return raw; }", "else { return third; }", 1)},
		{name: "nested conditional", source: strings.Replace(valid, "string third = second;", "string third = choose ? second : raw;", 1)},
		{name: "propagation in branch", source: `public Class Root { public Result<string, string> Select(Result<string, string> input, bool choose) { Result<string, string> value = input; if (choose) { string first = propagate(value); string second = first; string third = second; return ok<string, string>(third); } else { return input; } } }`},
		{name: "match in branch", source: `public Class Root { public string Select(Result<string, string> input, bool choose) { string value = ""; if (choose) { string first = match(input){ ok(okValue) => okValue, err(problem) => problem }; string second = first; string third = second; return third; } else { return value; } } }`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			input := semanticTestModuleSet("compiler.selfhosting", []ModuleInput{testModule("compiler.selfhosting", test.name+".pipe", test.source)}, nil)
			input.LanguageContract = PipeLangLanguageContractV720
			if err := AnalyzeSemanticModuleSet(input).Error(); err == nil {
				t.Fatal("excluded v0.72.0 source was accepted")
			}
		})
	}
}

func TestV720TerminalBranchLocalGeneralSequenceRejectsMalformedCoreAndBackend(t *testing.T) {
	_, _, malformed := terminalBranchLocalGeneralSequenceProgram(t)
	function := coreFunctionNamed(t, malformed, "Select")
	first := function.Body.ImmutableLocal.Return.Conditional.WhenTrue.ImmutableLocal
	first.Initializer = first.Return
	if err := coreir.ValidateProgram(malformed); err == nil || !strings.Contains(err.Error(), "v0.72.0 terminal if/else") {
		t.Fatalf("Core validator accepted a nested branch-local topology: %v", err)
	}
	if _, err := gobackend.Generate(malformed); err == nil {
		t.Fatal("Go backend accepted a nested branch-local topology")
	}

	_, _, oldContract := terminalBranchLocalGeneralSequenceProgram(t)
	oldContract.LanguageContract = coreir.LanguageContractV710
	if err := coreir.ValidateProgram(oldContract); err == nil {
		t.Fatal("v0.71.0 Core accepted a general terminal branch-local sequence")
	}
	if _, err := gobackend.Generate(oldContract); err == nil {
		t.Fatal("Go backend accepted a v0.71.0 general terminal branch-local sequence")
	}
}
