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

const terminalBranchLocalSequenceSource = `public Class CompilerBranchLocalSequence {
	public string Normalize(string raw) => trim(raw);
	public string Select(string raw, bool normalized) {
		string cleaned = raw;
		if (normalized) {
			string prepared = trim(cleaned);
			string selected = Normalize(prepared);
			return selected;
		} else {
			string prepared = cleaned;
			string selected = prepared;
			return selected;
		}
	}
}`

func terminalBranchLocalSequenceProgram(t *testing.T) (*Analysis, SemanticIdentity, coreir.Program) {
	t.Helper()
	input := semanticTestModuleSet("compiler.selfhosting", []ModuleInput{testModule("compiler.selfhosting", "terminal-branch-local-sequence.pipe", terminalBranchLocalSequenceSource)}, nil)
	input.LanguageContract = PipeLangLanguageContractV710
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

func TestV710TerminalBranchLocalSequencePipeline(t *testing.T) {
	analysis, identity, program := terminalBranchLocalSequenceProgram(t)
	projection, err := BuildSemanticProjection(analysis)
	if err != nil {
		t.Fatal(err)
	}
	if projection.LanguageContract != PipeLangLanguageContractV710 || projection.Schema != PipeLangSemanticProjectionVersion || projection.CompilerContract != PipeLangCompilerContract {
		t.Fatalf("projection contract = %#v", projection)
	}

	typed, err := LowerSemanticMethodToHIR(analysis, identity)
	if err != nil {
		t.Fatal(err)
	}
	function := hirFunctionNamed(t, typed, "Select")
	top := function.Body.ImmutableLocal
	if typed.LanguageContract != coreir.LanguageContractV710 || top == nil || top.Binding.Position != 2 || top.Return == nil || top.Return.Kind != hir.ExprConditional || top.Return.Conditional == nil || !top.Return.Conditional.TerminalStatement {
		t.Fatalf("v0.71.0 terminal branch-local sequence HIR = %#v", function.Body)
	}
	trueFirst := top.Return.Conditional.WhenTrue.ImmutableLocal
	if trueFirst == nil || trueFirst.Return == nil {
		t.Fatalf("missing local chain link: trueFirst = %#v", trueFirst)
	}
	trueSecond := trueFirst.Return.ImmutableLocal
	falseFirst := top.Return.Conditional.WhenFalse.ImmutableLocal
	if falseFirst == nil || falseFirst.Return == nil {
		t.Fatalf("missing local chain link: falseFirst = %#v", falseFirst)
	}
	falseSecond := falseFirst.Return.ImmutableLocal
	if trueFirst == nil || trueSecond == nil || falseFirst == nil || falseSecond == nil || trueFirst.Binding.Position != 3 || trueSecond.Binding.Position != 4 || falseFirst.Binding.Position != 3 || falseSecond.Binding.Position != 4 || trueSecond.Return.Reference == nil || trueSecond.Return.Reference.Position != 4 || falseSecond.Return.Reference == nil || falseSecond.Return.Reference.Position != 4 {
		t.Fatalf("v0.71.0 branch-local HIR sequences = %#v", top.Return.Conditional)
	}

	coreFunction := coreFunctionNamed(t, program, "Select")
	conditional := coreFunction.Body.ImmutableLocal.Return.Conditional
	trueCoreFirst := conditional.WhenTrue.ImmutableLocal
	trueCoreSecond := trueCoreFirst.Return.ImmutableLocal
	if program.LanguageContract != coreir.LanguageContractV710 || trueCoreFirst.Position != 3 || trueCoreSecond.Position != 4 || trueCoreSecond.Return.Parameter == nil || *trueCoreSecond.Return.Parameter != 4 {
		t.Fatalf("v0.71.0 terminal branch-local sequence Core = %#v", coreFunction.Body)
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
	for _, fragment := range []string{"if p1 {", "p3 := pipelangTrimText(p2)", "p4 := PipeLangNormalize(p3)", "return p4"} {
		if !strings.Contains(string(generated), fragment) {
			t.Fatalf("generated Go lacks terminal branch-local sequence fragment %q:\n%s", fragment, generated)
		}
	}
	compileAndRunGeneratedGoFiles(t, generated, []byte(fmt.Sprintf(`package %s

import "testing"

func TestGeneratedTerminalBranchLocalSequence(t *testing.T) {
	if got := PipeLangSelect("  source  ", true); got != "source" { t.Fatalf("true = %%q", got) }
	if got := PipeLangSelect("  source  ", false); got != "  source  " { t.Fatalf("false = %%q", got) }
}
`, gobackend.PackageName)))
}

func TestV710TerminalBranchLocalSequenceGateAndInheritance(t *testing.T) {
	input := semanticTestModuleSet("compiler.selfhosting", []ModuleInput{testModule("compiler.selfhosting", "terminal-branch-local-sequence-gate.pipe", terminalBranchLocalSequenceSource)}, nil)
	input.LanguageContract = PipeLangLanguageContractV700
	if err := AnalyzeSemanticModuleSet(input).Error(); err == nil {
		t.Fatal("v0.70.0 accepted two terminal branch locals")
	}
	input.LanguageContract = PipeLangLanguageContractV710
	if err := AnalyzeSemanticModuleSet(input).Error(); err != nil {
		t.Fatal(err)
	}

	inherited := semanticTestModuleSet("compiler.selfhosting", []ModuleInput{testModule("compiler.selfhosting", "terminal-branch-local-inherited.pipe", terminalBranchLocalSource)}, nil)
	inherited.LanguageContract = PipeLangLanguageContractV710
	if err := AnalyzeSemanticModuleSet(inherited).Error(); err != nil {
		t.Fatalf("v0.71.0 rejected inherited one-local terminal branches: %v", err)
	}
}

func TestV710TerminalBranchLocalSequenceRejectsExcludedSource(t *testing.T) {
	valid := `public Class Root { public string Select(string raw, bool choose) { string value = raw; if (choose) { string first = trim(value); string second = first; return second; } else { return raw; } } }`
	tests := []struct {
		name   string
		source string
	}{
		{name: "no preceding local", source: `public Class Root { public string Select(string raw, bool choose) { if (choose) { string first = trim(raw); string second = first; return second; } else { return raw; } } }`},
		{name: "three true-branch locals", source: strings.Replace(valid, "return second;", "string third = second; return third;", 1)},
		{name: "three false-branch locals", source: strings.Replace(valid, "else { return raw; }", "else { string first = raw; string second = first; string third = second; return third; }", 1)},
		{name: "duplicate branch local", source: strings.Replace(valid, "string second = first; return second;", "string first = first; return first;", 1)},
		{name: "shadows preceding local", source: strings.Replace(valid, "string first = trim(value);", "string value = trim(raw);", 1)},
		{name: "branch binding escapes", source: strings.Replace(valid, "else { return raw; }", "else { return second; }", 1)},
		{name: "nested conditional", source: strings.Replace(valid, "string second = first;", "string second = choose ? first : raw;", 1)},
		{name: "propagation in branch", source: `public Class Root { public Result<string, string> Select(Result<string, string> input, bool choose) { Result<string, string> value = input; if (choose) { string first = propagate(value); string second = first; return ok<string, string>(second); } else { return input; } } }`},
		{name: "match in branch", source: `public Class Root { public string Select(Result<string, string> input, bool choose) { string value = ""; if (choose) { string first = match(input){ ok(okValue) => okValue, err(problem) => problem }; string second = first; return second; } else { return value; } } }`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			input := semanticTestModuleSet("compiler.selfhosting", []ModuleInput{testModule("compiler.selfhosting", test.name+".pipe", test.source)}, nil)
			input.LanguageContract = PipeLangLanguageContractV710
			if err := AnalyzeSemanticModuleSet(input).Error(); err == nil {
				t.Fatal("excluded v0.71.0 source was accepted")
			}
		})
	}
}

func TestV710TerminalBranchLocalSequenceRejectsMalformedCoreAndBackend(t *testing.T) {
	_, _, threeLocals := terminalBranchLocalSequenceProgram(t)
	function := coreFunctionNamed(t, threeLocals, "Select")
	second := function.Body.ImmutableLocal.Return.Conditional.WhenTrue.ImmutableLocal.Return.ImmutableLocal
	secondReturn := second.Return
	second.Return = &coreir.Expr{
		Kind: coreir.ExprImmutableLocal,
		Type: second.Type,
		ImmutableLocal: &coreir.ImmutableLocal{
			Position:    second.Position + 1,
			Name:        "third",
			Type:        second.Type,
			Initializer: second.Initializer,
			Return:      secondReturn,
		},
	}
	if err := coreir.ValidateProgram(threeLocals); err == nil || !strings.Contains(err.Error(), "v0.71.0 terminal if/else") {
		t.Fatalf("Core validator accepted three branch locals: %v", err)
	}
	if _, err := gobackend.Generate(threeLocals); err == nil {
		t.Fatal("Go backend accepted three branch locals")
	}

	_, _, oldContract := terminalBranchLocalSequenceProgram(t)
	oldContract.LanguageContract = coreir.LanguageContractV700
	if err := coreir.ValidateProgram(oldContract); err == nil {
		t.Fatal("v0.70.0 Core accepted two terminal branch locals")
	}
	if _, err := gobackend.Generate(oldContract); err == nil {
		t.Fatal("Go backend accepted v0.70.0 two-local terminal branches")
	}
}
