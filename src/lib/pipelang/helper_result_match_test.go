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

const helperResultMatchSource = `public Class Root {
	public Result<string, string> Validate(string input) => input == "" ? err<string, string>("missing") : ok<string, string>(input);
	public string Resolve(string input) => match(Validate(input)){ ok(value) => trim(value), err(error) => error };
}`

func TestV430HelperResultMatchPipeline(t *testing.T) {
	input := semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", "helper-result-match.pipe", helperResultMatchSource)}, nil)
	input.LanguageContract = PipeLangLanguageContractV430
	analysis := AnalyzeSemanticModuleSet(input)
	if err := analysis.Error(); err != nil {
		t.Fatal(err)
	}
	projection, err := BuildSemanticProjection(analysis)
	if err != nil {
		t.Fatal(err)
	}
	if projection.LanguageContract != PipeLangLanguageContractV430 || projection.Schema != PipeLangSemanticProjectionVersion || projection.CompilerContract != PipeLangCompilerContract {
		t.Fatalf("projection contract = %#v", projection)
	}

	identity := semanticMethodNamed(t, analysis, "Resolve").Identity
	typed, err := LowerSemanticMethodToHIR(analysis, identity)
	if err != nil {
		t.Fatal(err)
	}
	resolved := hirFunctionNamed(t, typed, "Resolve")
	if typed.LanguageContract != coreir.LanguageContractV430 || resolved.Body.Kind != hir.ExprMatch || resolved.Body.Match == nil || resolved.Body.Match.Value == nil || resolved.Body.Match.Value.Kind != hir.ExprCall || len(resolved.Body.Match.Arms) != 2 {
		t.Fatalf("Resolve HIR = %#v", resolved.Body)
	}
	core, err := LowerHIRToCore(typed)
	if err != nil {
		t.Fatal(err)
	}
	function := coreFunctionNamed(t, core, "Resolve")
	if function.Body.Kind != coreir.ExprMatch || function.Body.Match == nil || function.Body.Match.Value == nil || function.Body.Match.Value.Kind != coreir.ExprCall || len(function.Body.Match.Value.Call.Arguments) != 1 || function.Body.Match.Value.Call.Arguments[0].Parameter == nil || *function.Body.Match.Value.Call.Arguments[0].Parameter != 0 {
		t.Fatalf("Resolve Core = %#v", function.Body)
	}
	text := function.Parameters[0].Type
	entry := coreir.SemanticIdentity{PackageID: string(identity.PackageID), Path: string(identity.Path)}
	success, err := coreeval.EvaluateProgram(core, entry, []coreeval.Value{{Type: text, String: "  ready  "}})
	if err != nil || !success.OK || success.Value.String != "ready" {
		t.Fatalf("success = %#v, %v", success, err)
	}
	failure, err := coreeval.EvaluateProgram(core, entry, []coreeval.Value{{Type: text, String: ""}})
	if err != nil || !failure.OK || failure.Value.String != "missing" {
		t.Fatalf("failure arm = %#v, %v", failure, err)
	}

	generated, err := gobackend.Generate(core)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(generated), "matched := PipeLangValidate(p0)") || !strings.Contains(string(generated), "if matched.OK") || !strings.Contains(string(generated), "if !matched.OK") {
		t.Fatalf("generated Go lacks helper-result match:\n%s", generated)
	}
	compileAndRunGeneratedGoFiles(t, generated, []byte(fmt.Sprintf(`package %s

import "testing"

func TestGeneratedHelperResultMatch(t *testing.T) {
	if got := PipeLangResolve("  ready  "); got != "ready" { t.Fatalf("success = %%q", got) }
	if got := PipeLangResolve(""); got != "missing" { t.Fatalf("failure = %%q", got) }
}
`, gobackend.PackageName)))
}

func TestV430HelperResultMatchRejectsUnselectedShapes(t *testing.T) {
	cases := []struct {
		name     string
		contract LanguageContract
		source   string
		message  string
	}{
		{name: "prior contract", contract: PipeLangLanguageContractV420, source: helperResultMatchSource, message: "direct match and propagate carriers"},
		{name: "computed argument", contract: PipeLangLanguageContractV430, source: `public Class Root { public Result<string, string> Validate(string input) => ok<string, string>(input); public string Resolve(string input) => match(Validate(trim(input))){ ok(value) => value, err(error) => error }; }`, message: "sole direct string parameter"},
		{name: "extra caller parameter", contract: PipeLangLanguageContractV430, source: `public Class Root { public Result<string, string> Validate(string input) => ok<string, string>(input); public string Resolve(string input, string fallback) => match(Validate(input)){ ok(value) => value, err(error) => fallback }; }`, message: "one direct string parameter"},
		{name: "extra helper argument", contract: PipeLangLanguageContractV430, source: `public Class Root { public Result<string, string> Validate(string input) => ok<string, string>(input); public string Resolve(string input) => match(Validate(input, input)){ ok(value) => value, err(error) => error }; }`, message: "sole direct string parameter"},
		{name: "optional carrier", contract: PipeLangLanguageContractV430, source: `public Class Root { public Optional<string> Validate(string input) => some(input); public string Resolve(string input) => match(Validate(input)){ some(value) => value, none => "missing" }; }`, message: "Result<string, string>"},
		{name: "reversed arms", contract: PipeLangLanguageContractV430, source: `public Class Root { public Result<string, string> Validate(string input) => ok<string, string>(input); public string Resolve(string input) => match(Validate(input)){ err(error) => error, ok(value) => value }; }`, message: "source-ordered"},
		{name: "wildcard arm", contract: PipeLangLanguageContractV430, source: `public Class Root { public Result<string, string> Validate(string input) => ok<string, string>(input); public string Resolve(string input) => match(Validate(input)){ ok(value) => value, _ => "missing" }; }`, message: "source-ordered"},
		{name: "nested helper match", contract: PipeLangLanguageContractV430, source: `public Class Root { public Result<string, string> Validate(string input) => ok<string, string>(input); public string Resolve(string input) => match(Validate(input)){ ok(value) => match(Validate(value)){ ok(next) => next, err(problem) => problem }, err(error) => error }; }`, message: "exactly one"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			input := semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", test.name+".pipe", test.source)}, nil)
			input.LanguageContract = test.contract
			analysis := AnalyzeSemanticModuleSet(input)
			if len(analysis.Diagnostics) == 0 || !strings.Contains(analysis.Diagnostics[0].Message, test.message) {
				t.Fatalf("diagnostics = %#v", analysis.Diagnostics)
			}
		})
	}
}

func TestV430HelperResultMatchPreservesDirectCarrierMatch(t *testing.T) {
	source := `public Class Root { public string Read(Result<string, string> value) => match(value){ ok(item) => item, err(problem) => problem }; }`
	input := semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", "direct-result-match.pipe", source)}, nil)
	input.LanguageContract = PipeLangLanguageContractV430
	if err := AnalyzeSemanticModuleSet(input).Error(); err != nil {
		t.Fatal(err)
	}
}

func TestV430HelperResultMatchCoreRejectsMalformedComposition(t *testing.T) {
	input := semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", "helper-result-match-core.pipe", helperResultMatchSource)}, nil)
	input.LanguageContract = PipeLangLanguageContractV430
	analysis := AnalyzeSemanticModuleSet(input)
	if err := analysis.Error(); err != nil {
		t.Fatal(err)
	}
	typed, err := LowerSemanticMethodToHIR(analysis, semanticMethodNamed(t, analysis, "Resolve").Identity)
	if err != nil {
		t.Fatal(err)
	}
	program, err := LowerHIRToCore(typed)
	if err != nil {
		t.Fatal(err)
	}

	downgraded := program
	downgraded.LanguageContract = coreir.LanguageContractV420
	if err := coreir.ValidateProgram(downgraded); err == nil || !strings.Contains(err.Error(), "direct match and propagate carriers") {
		t.Fatalf("downgraded Core error = %v", err)
	}

	badArgument := program
	badArgument.Functions = append([]coreir.Function(nil), program.Functions...)
	for index := range badArgument.Functions {
		if badArgument.Functions[index].Name == "Resolve" {
			function := badArgument.Functions[index]
			match := *function.Body.Match
			value := *match.Value
			call := *value.Call
			call.Arguments = append([]*coreir.Expr(nil), call.Arguments...)
			argument := *call.Arguments[0]
			argument.Kind = coreir.ExprLiteral
			argument.Parameter = nil
			argument.Literal = &coreir.Literal{String: "computed"}
			call.Arguments[0] = &argument
			value.Call = &call
			match.Value = &value
			function.Body.Match = &match
			badArgument.Functions[index] = function
		}
	}
	if err := coreir.ValidateProgram(badArgument); err == nil || !strings.Contains(err.Error(), "complete Result<string, string> match carrier") {
		t.Fatalf("bad argument Core error = %v", err)
	}

	badArms := program
	badArms.Functions = append([]coreir.Function(nil), program.Functions...)
	for index := range badArms.Functions {
		if badArms.Functions[index].Name == "Resolve" {
			function := badArms.Functions[index]
			match := *function.Body.Match
			match.Arms = []coreir.MatchArm{match.Arms[1], match.Arms[0]}
			function.Body.Match = &match
			badArms.Functions[index] = function
		}
	}
	if err := coreir.ValidateProgram(badArms); err == nil || !strings.Contains(err.Error(), "source-ordered") {
		t.Fatalf("bad arms Core error = %v", err)
	}
	if _, err := gobackend.Generate(badArms); err == nil {
		t.Fatal("Go backend accepted malformed helper-result match Core")
	}
}
