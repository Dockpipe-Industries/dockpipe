package pipelang

import (
	"fmt"
	"math"
	"strings"
	"testing"

	"dockpipe/src/lib/pipelang/coreeval"
	"dockpipe/src/lib/pipelang/coreir"
	"dockpipe/src/lib/pipelang/gobackend"
	"dockpipe/src/lib/pipelang/hir"
)

const checkedArithmeticHelperMatchLocalSource = `public Class Root {
	public Result<int, ArithmeticError> Add(int left, int right) => left + right;
	public int AddOrZero(int left, int right) {
		int selected = match(Add(left, right)){ ok(value) => value, err(problem) => 0 };
		int copied = selected;
		return copied;
	}
	public Result<float, ArithmeticError> Divide(float left, float right) => left / right;
	public float DivideOrZero(float left, float right) {
		float selected = match(Divide(left, right)){ ok(value) => value, err(problem) => 0.0 };
		return selected;
	}
}`

func TestV460CheckedArithmeticHelperMatchLocalPipeline(t *testing.T) {
	input := semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", "checked-arithmetic-helper-match-local.pipe", checkedArithmeticHelperMatchLocalSource)}, nil)
	input.LanguageContract = PipeLangLanguageContractV460
	analysis := AnalyzeSemanticModuleSet(input)
	if err := analysis.Error(); err != nil {
		t.Fatal(err)
	}
	projection, err := BuildSemanticProjection(analysis)
	if err != nil {
		t.Fatal(err)
	}
	if projection.LanguageContract != PipeLangLanguageContractV460 || projection.Schema != PipeLangSemanticProjectionVersion || projection.CompilerContract != PipeLangCompilerContract {
		t.Fatalf("projection contract = %#v", projection)
	}

	addIdentity := semanticMethodNamed(t, analysis, "AddOrZero").Identity
	typed, err := LowerSemanticMethodToHIR(analysis, addIdentity)
	if err != nil {
		t.Fatal(err)
	}
	addHIR := hirFunctionNamed(t, typed, "AddOrZero")
	firstHIR := addHIR.Body.ImmutableLocal
	if typed.LanguageContract != coreir.LanguageContractV460 || addHIR.Body.Kind != hir.ExprImmutableLocal || firstHIR == nil || firstHIR.Initializer.Kind != hir.ExprMatch || firstHIR.Initializer.Match == nil || firstHIR.Initializer.Match.Value == nil || firstHIR.Initializer.Match.Value.Kind != hir.ExprCall || firstHIR.Initializer.Match.Value.Type.Kind != hir.TypeResult || firstHIR.Initializer.Match.Value.Type.Result == nil || firstHIR.Initializer.Match.Value.Type.Result.Failure.Kind != hir.TypeArithmeticError {
		t.Fatalf("AddOrZero HIR = %#v", addHIR.Body)
	}
	if firstHIR.Return.Kind != hir.ExprImmutableLocal || firstHIR.Return.ImmutableLocal == nil || firstHIR.Return.ImmutableLocal.Return.Kind != hir.ExprReference {
		t.Fatalf("AddOrZero continuation HIR = %#v", firstHIR.Return)
	}

	program, err := LowerHIRToCore(typed)
	if err != nil {
		t.Fatal(err)
	}
	add := coreFunctionNamed(t, program, "AddOrZero")
	first := add.Body.ImmutableLocal
	if add.Body.Kind != coreir.ExprImmutableLocal || first == nil || first.Position != 2 || first.Initializer.Kind != coreir.ExprMatch || first.Initializer.Match == nil || first.Initializer.Match.Value == nil || first.Initializer.Match.Value.Kind != coreir.ExprCall || first.Initializer.Match.Value.Type.Kind != coreir.TypeResult || first.Initializer.Match.Value.Type.Result == nil || first.Initializer.Match.Value.Type.Result.Failure.Kind != coreir.TypeArithmeticError {
		t.Fatalf("AddOrZero Core = %#v", add.Body)
	}
	entry := coreir.SemanticIdentity{PackageID: string(addIdentity.PackageID), Path: string(addIdentity.Path)}
	success, err := coreeval.EvaluateProgram(program, entry, coreIntArguments(20, 22))
	if err != nil || !success.OK || success.Value.Int != 42 {
		t.Fatalf("add success = %#v, %v", success, err)
	}
	overflow, err := coreeval.EvaluateProgram(program, entry, coreIntArguments(math.MaxInt64, 1))
	if err != nil || !overflow.OK || overflow.Value.Int != 0 {
		t.Fatalf("add overflow = %#v, %v", overflow, err)
	}

	generated, err := gobackend.Generate(program)
	if err != nil {
		t.Fatal(err)
	}
	generatedAgain, err := gobackend.Generate(program)
	if err != nil || string(generatedAgain) != string(generated) {
		t.Fatalf("checked-arithmetic helper-match Go is nondeterministic: %v", err)
	}
	if strings.Count(string(generated), "PipeLangAdd(p0, p1)") != 1 {
		t.Fatalf("generated Go does not call the checked helper exactly once:\n%s", generated)
	}
	compileAndRunGeneratedGoFiles(t, generated, []byte(fmt.Sprintf(`package %s

import (
	"math"
	"testing"
)

func TestGeneratedCheckedArithmeticHelperMatchLocal(t *testing.T) {
	if got := PipeLangAddOrZero(20, 22); got != 42 { t.Fatalf("success = %%d", got) }
	if got := PipeLangAddOrZero(math.MaxInt64, 1); got != 0 { t.Fatalf("overflow = %%d", got) }
}
`, gobackend.PackageName)))

	testV460FloatCheckedArithmeticHelperMatchLocal(t, analysis)
}

func testV460FloatCheckedArithmeticHelperMatchLocal(t *testing.T, analysis *Analysis) {
	t.Helper()
	identity := semanticMethodNamed(t, analysis, "DivideOrZero").Identity
	typed, err := LowerSemanticMethodToHIR(analysis, identity)
	if err != nil {
		t.Fatal(err)
	}
	program, err := LowerHIRToCore(typed)
	if err != nil {
		t.Fatal(err)
	}
	entry := coreir.SemanticIdentity{PackageID: string(identity.PackageID), Path: string(identity.Path)}
	success, err := coreeval.EvaluateProgram(program, entry, coreFloatArguments(5, 2))
	if err != nil || !success.OK || success.Value.Float != 2.5 {
		t.Fatalf("divide success = %#v, %v", success, err)
	}
	failure, err := coreeval.EvaluateProgram(program, entry, coreFloatArguments(1, 0))
	if err != nil || !failure.OK || failure.Value.Float != 0 {
		t.Fatalf("divide failure = %#v, %v", failure, err)
	}
	generated, err := gobackend.Generate(program)
	if err != nil {
		t.Fatal(err)
	}
	compileAndRunGeneratedGoFiles(t, generated, []byte(fmt.Sprintf(`package %s

import "testing"

func TestGeneratedFloatCheckedArithmeticHelperMatchLocal(t *testing.T) {
	if got := PipeLangDivideOrZero(5, 2); got != 2.5 { t.Fatalf("success = %%v", got) }
	if got := PipeLangDivideOrZero(1, 0); got != 0 { t.Fatalf("failure = %%v", got) }
}
`, gobackend.PackageName)))
}

func TestV460CheckedArithmeticHelperMatchLocalRejectsExcludedSource(t *testing.T) {
	tests := []struct {
		name     string
		contract LanguageContract
		source   string
		message  string
	}{
		{name: "prior contract", contract: PipeLangLanguageContractV450, source: `public Class Root { public Result<int, ArithmeticError> Add(int left, int right) => left + right; public int Resolve(int left, int right) { int selected = match(Add(left, right)){ ok(value) => value, err(problem) => 0 }; return selected; } }`, message: "admitted Optional"},
		{name: "complete method body", contract: PipeLangLanguageContractV460, source: `public Class Root { public Result<int, ArithmeticError> Add(int left, int right) => left + right; public int Resolve(int left, int right) => match(Add(left, right)){ ok(value) => value, err(problem) => 0 }; }`, message: "admitted Optional"},
		{name: "second local", contract: PipeLangLanguageContractV460, source: `public Class Root { public Result<int, ArithmeticError> Add(int left, int right) => left + right; public int Resolve(int left, int right) { int first = left; int selected = match(Add(left, right)){ ok(value) => value, err(problem) => 0 }; return selected; } }`, message: "first immutable-local initializer"},
		{name: "terminal return", contract: PipeLangLanguageContractV460, source: `public Class Root { public Result<int, ArithmeticError> Add(int left, int right) => left + right; public int Resolve(int left, int right) { int first = left; return match(Add(left, right)){ ok(value) => value, err(problem) => 0 }; } }`, message: "first immutable-local initializer"},
		{name: "computed argument", contract: PipeLangLanguageContractV460, source: `public Class Root { public int Identity(int value) => value; public Result<int, ArithmeticError> Add(int left, int right) => left + right; public int Resolve(int left, int right) { int selected = match(Add(Identity(left), right)){ ok(value) => value, err(problem) => 0 }; return selected; } }`, message: "every caller parameter"},
		{name: "reordered arguments", contract: PipeLangLanguageContractV460, source: `public Class Root { public Result<int, ArithmeticError> Add(int left, int right) => left + right; public int Resolve(int right, int left) { int selected = match(Add(left, right)){ ok(value) => value, err(problem) => 0 }; return selected; } }`, message: "declaration order"},
		{name: "omitted caller parameter", contract: PipeLangLanguageContractV460, source: `public Class Root { public Result<int, ArithmeticError> Add(int left, int right) => left + right; public int Resolve(int left, int right, int fallback) { int selected = match(Add(left, right)){ ok(value) => value, err(problem) => fallback }; return selected; } }`, message: "every caller parameter"},
		{name: "extra helper argument", contract: PipeLangLanguageContractV460, source: `public Class Root { public Result<int, ArithmeticError> Add(int left, int right) => left + right; public int Resolve(int left, int right) { int selected = match(Add(left, right, left)){ ok(value) => value, err(problem) => 0 }; return selected; } }`, message: "every caller parameter"},
		{name: "private helper", contract: PipeLangLanguageContractV460, source: `public Class Root { private Result<int, ArithmeticError> Add(int left, int right) => left + right; public int Resolve(int left, int right) { int selected = match(Add(left, right)){ ok(value) => value, err(problem) => 0 }; return selected; } }`, message: "must be public"},
		{name: "cross owner", contract: PipeLangLanguageContractV460, source: `public Class Other { public Result<int, ArithmeticError> Add(int left, int right) => left + right; } public Class Root { public int Resolve(int left, int right) { int selected = match(Add(left, right)){ ok(value) => value, err(problem) => 0 }; return selected; } }`, message: "has no method"},
		{name: "multiple matches", contract: PipeLangLanguageContractV460, source: `public Class Root { public Result<int, ArithmeticError> Add(int left, int right) => left + right; public int Resolve(int left, int right) { int first = match(Add(left, right)){ ok(value) => value, err(problem) => 0 }; int second = match(Add(left, right)){ ok(value) => value, err(problem) => 0 }; return second; } }`, message: "exactly one"},
		{name: "wrong local type", contract: PipeLangLanguageContractV460, source: `public Class Root { public Result<int, ArithmeticError> Add(int left, int right) => left + right; public int Resolve(int left, int right) { float selected = match(Add(left, right)){ ok(value) => value, err(problem) => 0 }; return left; } }`, message: "initializer has type"},
		{name: "reversed arms", contract: PipeLangLanguageContractV460, source: `public Class Root { public Result<int, ArithmeticError> Add(int left, int right) => left + right; public int Resolve(int left, int right) { int selected = match(Add(left, right)){ err(problem) => 0, ok(value) => value }; return selected; } }`, message: "source-ordered"},
		{name: "wildcard arm", contract: PipeLangLanguageContractV460, source: `public Class Root { public Result<int, ArithmeticError> Add(int left, int right) => left + right; public int Resolve(int left, int right) { int selected = match(Add(left, right)){ ok(value) => value, _ => 0 }; return selected; } }`, message: "source-ordered"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			input := semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", test.name+".pipe", test.source)}, nil)
			input.LanguageContract = test.contract
			analysis := AnalyzeSemanticModuleSet(input)
			if len(analysis.Diagnostics) == 0 || !strings.Contains(analysis.Diagnostics[0].Message, test.message) || !analysis.Diagnostics[0].Primary.IsValid() {
				t.Fatalf("diagnostics = %#v", analysis.Diagnostics)
			}
		})
	}
}

func TestV460CheckedArithmeticHelperMatchLocalCoreRejectsMalformedComposition(t *testing.T) {
	input := semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", "checked-arithmetic-helper-match-local-core.pipe", checkedArithmeticHelperMatchLocalSource)}, nil)
	input.LanguageContract = PipeLangLanguageContractV460
	analysis := AnalyzeSemanticModuleSet(input)
	if err := analysis.Error(); err != nil {
		t.Fatal(err)
	}
	typed, err := LowerSemanticMethodToHIR(analysis, semanticMethodNamed(t, analysis, "AddOrZero").Identity)
	if err != nil {
		t.Fatal(err)
	}
	program, err := LowerHIRToCore(typed)
	if err != nil {
		t.Fatal(err)
	}

	downgraded := program
	downgraded.LanguageContract = coreir.LanguageContractV450
	if err := coreir.ValidateProgram(downgraded); err == nil || !strings.Contains(err.Error(), "admitted Optional") {
		t.Fatalf("downgraded Core error = %v", err)
	}

	badArms := program
	badArms.Functions = append([]coreir.Function(nil), program.Functions...)
	for index := range badArms.Functions {
		if badArms.Functions[index].Name == "AddOrZero" {
			function := badArms.Functions[index]
			local := *function.Body.ImmutableLocal
			initializer := *local.Initializer
			match := *initializer.Match
			match.Arms = []coreir.MatchArm{match.Arms[1], match.Arms[0]}
			initializer.Match = &match
			local.Initializer = &initializer
			function.Body.ImmutableLocal = &local
			badArms.Functions[index] = function
		}
	}
	if err := coreir.ValidateProgram(badArms); err == nil || !strings.Contains(err.Error(), "source-ordered") {
		t.Fatalf("bad arms Core error = %v", err)
	}
	if _, err := gobackend.Generate(badArms); err == nil {
		t.Fatal("Go backend accepted malformed checked-arithmetic helper-match Core")
	}
}
