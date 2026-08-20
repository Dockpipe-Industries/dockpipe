package pipelang

import (
	"strings"
	"testing"

	"dockpipe/src/lib/pipelang/coreeval"
	"dockpipe/src/lib/pipelang/coreir"
	"dockpipe/src/lib/pipelang/gobackend"
	"dockpipe/src/lib/pipelang/hir"
)

func TestV360SameClassPureCallsPipeline(t *testing.T) {
	source := `public Class Root {
		public string Normalize(string value) => trim(value);
		public bool Contains(string value, string query) => contains_casefolded(value, query);
		public bool Search(string value, string query) => Contains(Normalize(value), Normalize(query));
	}`
	input := semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", "calls.pipe", source)}, nil)
	input.LanguageContract = PipeLangLanguageContractV360
	analysis := AnalyzeSemanticModuleSet(input)
	if err := analysis.Error(); err != nil {
		t.Fatal(err)
	}
	identity := semanticMethodNamed(t, analysis, "Search").Identity
	typed, err := LowerSemanticMethodToHIR(analysis, identity)
	if err != nil {
		t.Fatal(err)
	}
	if typed.LanguageContract != coreir.LanguageContractV360 || len(typed.Functions) != 3 {
		t.Fatalf("HIR contract/functions = %s/%d", typed.LanguageContract, len(typed.Functions))
	}
	search := hirFunctionNamed(t, typed, "Search")
	if search.Body.Kind != hir.ExprCall || search.Body.Call.TargetName != "Contains" || search.Body.Call.Target.Callable == nil {
		t.Fatalf("Search HIR = %#v", search.Body)
	}
	if search.Body.Call.Arguments[0].Kind != hir.ExprCall || search.Body.Call.Arguments[1].Kind != hir.ExprCall {
		t.Fatalf("Search nested HIR = %#v", search.Body)
	}
	core, err := LowerHIRToCore(typed)
	if err != nil {
		t.Fatal(err)
	}
	outcome, err := coreeval.EvaluateProgram(core, coreir.SemanticIdentity{PackageID: string(identity.PackageID), Path: string(identity.Path)}, []coreeval.Value{
		{Type: coreFunctionNamed(t, core, "Search").Parameters[0].Type, String: "  DockPipe  "},
		{Type: coreFunctionNamed(t, core, "Search").Parameters[1].Type, String: "dockpipe"},
	})
	if err != nil || !outcome.OK || !outcome.Value.Bool {
		t.Fatalf("evaluation = %#v, %v", outcome, err)
	}
	generated, err := gobackend.Generate(core)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(generated), "PipeLangContains(PipeLangNormalize(p0), PipeLangNormalize(p1))") || strings.Count(string(generated), "PipeLangNormalize(") < 3 {
		t.Fatalf("generated Go lacks validated calls:\n%s", generated)
	}
	compileAndRunGeneratedGoFiles(t, generated, []byte(`package pipelanggenerated
import "testing"
func TestPureCalls(t *testing.T) {
	if !PipeLangSearch("  DockPipe  ", "dockpipe") { t.Fatal("nested pure call failed") }
}`))
}

func TestV360PureCallDiagnostics(t *testing.T) {
	cases := []struct {
		name   string
		source string
		code   DiagnosticCode
	}{
		{"cycle", `public Class Root { public string A(string value) => B(value); public string B(string value) => A(value); }`, CodePureCallCycle},
		{"missing", `public Class Root { public string A(string value) => Missing(value); }`, CodeInvalidMember},
		{"private target", `public Class Root { private string Hidden(string value) => value; public string A(string value) => Hidden(value); }`, CodeInvalidMember},
		{"private caller", `public Class Root { public string Visible(string value) => value; private string A(string value) => Visible(value); }`, CodeInvalidMember},
		{"arity", `public Class Root { public string One(string value) => value; public string A(string value) => One(value, value); }`, CodeInvalidType},
		{"type", `public Class Root { public string One(string value) => value; public string A(bool value) => One(value); }`, CodeInvalidType},
		{"wrapped call", `public Class Root { public string One(string value) => value; public string A(string value) => trim(One(value)); }`, CodeExpressionType},
		{"class state target", `public Class Root { public string Prefix = "x"; public string Read(string value) => Prefix; public string A(string value) => Read(value); }`, CodeExpressionType},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			input := semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", "bad.pipe", tc.source)}, nil)
			input.LanguageContract = PipeLangLanguageContractV360
			analysis := AnalyzeSemanticModuleSet(input)
			if len(analysis.Diagnostics) != 1 || analysis.Diagnostics[0].Code != tc.code {
				t.Fatalf("diagnostics = %#v", analysis.Diagnostics)
			}
		})
	}
	prior := semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", "prior.pipe", `public Class Root { public string Identity(string value) => value; public string A(string value) => Identity(value); }`)}, nil)
	prior.LanguageContract = PipeLangLanguageContractV350
	if AnalyzeSemanticModuleSet(prior).Error() == nil {
		t.Fatal("v0.35.0 accepted a pure call")
	}
}

func TestV360CoreRejectsForgedPureCallTargetAndCycle(t *testing.T) {
	source := `public Class Root { public string Identity(string value) => value; public string A(string value) => Identity(value); }`
	input := semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", "core.pipe", source)}, nil)
	input.LanguageContract = PipeLangLanguageContractV360
	analysis := AnalyzeSemanticModuleSet(input)
	if err := analysis.Error(); err != nil {
		t.Fatal(err)
	}
	typed, err := LowerSemanticMethodToHIR(analysis, semanticMethodNamed(t, analysis, "A").Identity)
	if err != nil {
		t.Fatal(err)
	}
	core, err := LowerHIRToCore(typed)
	if err != nil {
		t.Fatal(err)
	}
	forged := core
	for index := range forged.Functions {
		if forged.Functions[index].Name == "A" {
			forged.Functions[index].Body.Call.Target.Path += ".forged"
		}
	}
	if err := coreir.ValidateProgram(forged); err == nil {
		t.Fatal("Core accepted a forged call target")
	}
	cyclic, err := LowerHIRToCore(typed)
	if err != nil {
		t.Fatal(err)
	}
	identityFunction := coreFunctionNamed(t, cyclic, "Identity")
	aFunction := coreFunctionNamed(t, cyclic, "A")
	identityFunction.Body = coreir.Expr{Kind: coreir.ExprCall, Type: identityFunction.ReturnType, Call: &coreir.Call{
		Target: aFunction.Identity, TargetName: aFunction.Name,
		Arguments: []*coreir.Expr{{Kind: coreir.ExprReference, Type: identityFunction.Parameters[0].Type, Parameter: intPointer(0)}},
	}}
	for index := range cyclic.Functions {
		if cyclic.Functions[index].Name == "Identity" {
			cyclic.Functions[index] = identityFunction
		}
	}
	if err := coreir.ValidateProgram(cyclic); err == nil || !strings.Contains(err.Error(), "cycle") {
		t.Fatalf("Core cycle error = %v", err)
	}
}

func hirFunctionNamed(t *testing.T, program hir.Program, name string) hir.Function {
	t.Helper()
	for _, function := range program.Functions {
		if function.Name == name {
			return function
		}
	}
	t.Fatalf("HIR function %s not found", name)
	return hir.Function{}
}

func coreFunctionNamed(t *testing.T, program coreir.Program, name string) coreir.Function {
	t.Helper()
	for _, function := range program.Functions {
		if function.Name == name {
			return function
		}
	}
	t.Fatalf("Core function %s not found", name)
	return coreir.Function{}
}

func intPointer(value int) *int { return &value }
