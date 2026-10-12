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

const laterLocalHelperMatchSource = `public Record Row { public string Id; public string Name; }
public Class Root {
	public Optional<Row> Find(List<Row> rows, string id) => find_by(rows, Row.Id, id);
	public string Normalize(string value) => trim(value);
	public string Resolve(List<Row> rows, string id) {
		string fallback = "";
		string copiedFallback = fallback;
		string selected = match(Find(rows, id)){ some(row) => row.Name, none => copiedFallback };
		string cleaned = Normalize(selected);
		return cleaned;
	}
	public Result<List<Row>, string> Snapshot(Result<List<Row>, string> snapshot) => snapshot;
	public int SnapshotCount(Result<List<Row>, string> snapshot) {
		int fallback = 0;
		int selected = match(Snapshot(snapshot)){ ok(rows) => count(rows), err(problem) => fallback };
		return selected;
	}
	public Result<string, string> Validate(string input) => input == "" ? err<string, string>("missing") : ok<string, string>(input);
	public string Message(string input) {
		string fallback = "missing";
		string selected = match(Validate(input)){ ok(value) => value, err(problem) => fallback };
		return trim(selected);
	}
	public Result<int, ArithmeticError> Add(int left, int right) => left + right;
	public int AddOrZero(int left, int right) {
		int fallback = 0;
		int selected = match(Add(left, right)){ ok(value) => value, err(problem) => fallback };
		int copied = selected;
		return copied;
	}
	public Result<float, ArithmeticError> Divide(float left, float right) => left / right;
	public float DivideOrZero(float left, float right) {
		float fallback = 0.0;
		float selected = match(Divide(left, right)){ ok(value) => value, err(problem) => fallback };
		return selected;
	}
}`

func TestV470PreservesFirstLocalAndTopLevelHelperMatches(t *testing.T) {
	input := semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", "v047-inherited-helper-matches.pipe", helperCarrierMatchLocalSource)}, nil)
	input.LanguageContract = PipeLangLanguageContractV470
	analysis := AnalyzeSemanticModuleSet(input)
	if err := analysis.Error(); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"Resolve", "LegacyResolve"} {
		typed, err := LowerSemanticMethodToHIR(analysis, semanticMethodNamed(t, analysis, name).Identity)
		if err != nil {
			t.Fatalf("lower %s to HIR: %v", name, err)
		}
		if _, err := LowerHIRToCore(typed); err != nil {
			t.Fatalf("lower %s to Core: %v", name, err)
		}
	}
}

func TestV470LaterLocalHelperMatchPipeline(t *testing.T) {
	input := semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", "later-local-helper-match.pipe", laterLocalHelperMatchSource)}, nil)
	input.LanguageContract = PipeLangLanguageContractV470
	analysis := AnalyzeSemanticModuleSet(input)
	if err := analysis.Error(); err != nil {
		t.Fatal(err)
	}
	projection, err := BuildSemanticProjection(analysis)
	if err != nil {
		t.Fatal(err)
	}
	if projection.LanguageContract != PipeLangLanguageContractV470 || projection.Schema != PipeLangSemanticProjectionVersion || projection.CompilerContract != PipeLangCompilerContract {
		t.Fatalf("projection contract = %#v", projection)
	}

	resolveIdentity := semanticMethodNamed(t, analysis, "Resolve").Identity
	typed, err := LowerSemanticMethodToHIR(analysis, resolveIdentity)
	if err != nil {
		t.Fatal(err)
	}
	resolveHIR := hirFunctionNamed(t, typed, "Resolve")
	firstHIR := resolveHIR.Body.ImmutableLocal
	if typed.LanguageContract != coreir.LanguageContractV470 || resolveHIR.Body.Kind != hir.ExprImmutableLocal || firstHIR == nil || firstHIR.Initializer.Kind != hir.ExprLiteral {
		t.Fatalf("Resolve HIR = %#v", resolveHIR.Body)
	}
	secondHIR := firstHIR.Return.ImmutableLocal
	if secondHIR == nil || secondHIR.Return == nil {
		t.Fatalf("missing local chain link: secondHIR = %#v", secondHIR)
	}
	thirdHIR := secondHIR.Return.ImmutableLocal
	if thirdHIR == nil || thirdHIR.Initializer.Kind != hir.ExprMatch || thirdHIR.Initializer.Match == nil || thirdHIR.Initializer.Match.Value == nil || thirdHIR.Initializer.Match.Value.Kind != hir.ExprCall || thirdHIR.Return.Kind != hir.ExprImmutableLocal {
		t.Fatalf("Resolve later-local HIR = %#v", firstHIR.Return)
	}

	program, err := LowerHIRToCore(typed)
	if err != nil {
		t.Fatal(err)
	}
	resolve := coreFunctionNamed(t, program, "Resolve")
	first := resolve.Body.ImmutableLocal
	if first == nil || first.Return == nil {
		t.Fatalf("missing local chain link: first = %#v", first)
	}
	second := first.Return.ImmutableLocal
	third := second.Return.ImmutableLocal
	if resolve.Body.Kind != coreir.ExprImmutableLocal || first == nil || first.Position != 2 || second == nil || second.Position != 3 || third == nil || third.Position != 4 || third.Initializer.Kind != coreir.ExprMatch || third.Initializer.Match == nil || third.Initializer.Match.Value == nil || third.Initializer.Match.Value.Kind != coreir.ExprCall {
		t.Fatalf("Resolve later-local Core = %#v", resolve.Body)
	}
	call := third.Initializer.Match.Value.Call
	if len(call.Arguments) != 2 || call.Arguments[0].Parameter == nil || *call.Arguments[0].Parameter != 0 || call.Arguments[1].Parameter == nil || *call.Arguments[1].Parameter != 1 {
		t.Fatalf("Resolve Core call = %#v", call)
	}

	listType := resolve.Parameters[0].Type
	rowType := listType.List.Element
	row := func(id, name string) coreeval.Value {
		return coreeval.Value{Type: rowType, Record: []coreeval.Value{
			{Type: rowType.Record.Fields[0].Type, String: id},
			{Type: rowType.Record.Fields[1].Type, String: name},
		}}
	}
	rows := coreeval.Value{Type: listType, List: []coreeval.Value{row("one", "  worker  "), row("two", "api")}}
	text := coreir.Type{Kind: coreir.TypePrimitive, Primitive: coreir.PrimitiveString}
	entry := coreir.SemanticIdentity{PackageID: string(resolveIdentity.PackageID), Path: string(resolveIdentity.Path)}
	found, err := coreeval.EvaluateProgram(program, entry, []coreeval.Value{rows, {Type: text, String: "one"}})
	if err != nil || !found.OK || found.Value.String != "worker" {
		t.Fatalf("present result = %#v, %v", found, err)
	}
	missing, err := coreeval.EvaluateProgram(program, entry, []coreeval.Value{rows, {Type: text, String: "missing"}})
	if err != nil || !missing.OK || missing.Value.String != "" {
		t.Fatalf("absent result = %#v, %v", missing, err)
	}

	generated, err := gobackend.Generate(program)
	if err != nil {
		t.Fatal(err)
	}
	generatedAgain, err := gobackend.Generate(program)
	if err != nil || string(generatedAgain) != string(generated) {
		t.Fatalf("later-local helper-match Go is nondeterministic: %v", err)
	}
	if strings.Count(string(generated), "matched := PipeLangFind(") != 1 || !strings.Contains(string(generated), "p4 := func() string") || !strings.Contains(string(generated), "p5 := PipeLangNormalize(p4)") {
		t.Fatalf("generated Go lacks later-local helper match and continuation:\n%s", generated)
	}
	compileAndRunGeneratedGoFiles(t, generated, []byte(fmt.Sprintf(`package %s

import "testing"

func TestGeneratedLaterLocalHelperMatch(t *testing.T) {
	rows := []PipeLangRecordTestPackageAppRootRow{{Id: "one", Name: "  worker  "}, {Id: "two", Name: "api"}}
	if got := PipeLangResolve(rows, "one"); got != "worker" { t.Fatalf("present = %%q", got) }
	if got := PipeLangResolve(rows, "missing"); got != "" { t.Fatalf("absent = %%q", got) }
}
`, gobackend.PackageName)))

	testV450SnapshotMatchLocal(t, analysis)
	testV450TextResultMatchLocal(t, analysis)
	testV470CheckedArithmeticLaterLocal(t, analysis)
	testV460FloatCheckedArithmeticHelperMatchLocal(t, analysis)
}

func testV470CheckedArithmeticLaterLocal(t *testing.T, analysis *Analysis) {
	t.Helper()
	identity := semanticMethodNamed(t, analysis, "AddOrZero").Identity
	typed, err := LowerSemanticMethodToHIR(analysis, identity)
	if err != nil {
		t.Fatal(err)
	}
	program, err := LowerHIRToCore(typed)
	if err != nil {
		t.Fatal(err)
	}
	add := coreFunctionNamed(t, program, "AddOrZero")
	first := add.Body.ImmutableLocal
	if first == nil || first.Return == nil {
		t.Fatalf("missing local chain link: first = %#v", first)
	}
	second := first.Return.ImmutableLocal
	if first.Position != 2 || second == nil || second.Position != 3 || second.Initializer.Kind != coreir.ExprMatch || second.Initializer.Match == nil || second.Initializer.Match.Value == nil || second.Initializer.Match.Value.Type.Kind != coreir.TypeResult || second.Initializer.Match.Value.Type.Result == nil || second.Initializer.Match.Value.Type.Result.Failure.Kind != coreir.TypeArithmeticError {
		t.Fatalf("AddOrZero later-local Core = %#v", add.Body)
	}
	entry := coreir.SemanticIdentity{PackageID: string(identity.PackageID), Path: string(identity.Path)}
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
	if strings.Count(string(generated), "PipeLangAdd(p0, p1)") != 1 {
		t.Fatalf("generated Go does not call the checked helper exactly once:\n%s", generated)
	}
	compileAndRunGeneratedGoFiles(t, generated, []byte(fmt.Sprintf(`package %s

import (
	"math"
	"testing"
)

func TestGeneratedCheckedArithmeticLaterLocal(t *testing.T) {
	if got := PipeLangAddOrZero(20, 22); got != 42 { t.Fatalf("success = %%d", got) }
	if got := PipeLangAddOrZero(math.MaxInt64, 1); got != 0 { t.Fatalf("overflow = %%d", got) }
}
`, gobackend.PackageName)))
}

func TestV470LaterLocalHelperMatchRejectsExcludedSource(t *testing.T) {
	tests := []struct {
		name     string
		contract LanguageContract
		source   string
		message  string
	}{
		{name: "prior contract", contract: PipeLangLanguageContractV460, source: `public Class Root { public Optional<string> Read(string value) => some(value); public string Resolve(string value) { string fallback = ""; string selected = match(Read(value)){ some(item) => item, none => fallback }; return selected; } }`, message: "first immutable-local initializer"},
		{name: "terminal return", contract: PipeLangLanguageContractV470, source: `public Class Root { public Optional<string> Read(string value) => some(value); public string Resolve(string value) { string fallback = ""; return match(Read(value)){ some(item) => item, none => fallback }; } }`, message: "terminal-return"},
		{name: "top-level arithmetic", contract: PipeLangLanguageContractV470, source: `public Class Root { public Result<int, ArithmeticError> Add(int left, int right) => left + right; public int Resolve(int left, int right) => match(Add(left, right)){ ok(value) => value, err(problem) => 0 }; }`, message: "admitted Optional"},
		{name: "multiple matches", contract: PipeLangLanguageContractV470, source: `public Class Root { public Optional<string> Read(string value) => some(value); public string Resolve(string value) { string first = match(Read(value)){ some(item) => item, none => "" }; string second = match(Read(value)){ some(item) => item, none => first }; return second; } }`, message: "exactly one"},
		{name: "computed argument", contract: PipeLangLanguageContractV470, source: `public Class Root { public string Normalize(string value) => trim(value); public Optional<string> Read(string value) => some(value); public string Resolve(string value) { string fallback = ""; string selected = match(Read(Normalize(value))){ some(item) => item, none => fallback }; return selected; } }`, message: "every caller parameter"},
		{name: "nested match", contract: PipeLangLanguageContractV470, source: `public Class Root { public Optional<string> Read(string value) => some(value); public string Resolve(string value) { string fallback = ""; string selected = trim(match(Read(value)){ some(item) => item, none => fallback }); return selected; } }`, message: "immutable-local initializer"},
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

func TestV470LaterLocalHelperMatchCoreRejectsMalformedPlacement(t *testing.T) {
	input := semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", "later-local-helper-match-core.pipe", laterLocalHelperMatchSource)}, nil)
	input.LanguageContract = PipeLangLanguageContractV470
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
	downgraded.LanguageContract = coreir.LanguageContractV460
	if err := coreir.ValidateProgram(downgraded); err == nil || !strings.Contains(err.Error(), "first immutable-local initializer") {
		t.Fatalf("downgraded Core error = %v", err)
	}

	terminal := program
	terminal.Functions = append([]coreir.Function(nil), program.Functions...)
	for index := range terminal.Functions {
		if terminal.Functions[index].Name == "AddOrZero" {
			function := terminal.Functions[index]
			first := *function.Body.ImmutableLocal
			second := *first.Return.ImmutableLocal
			first.Return = second.Initializer
			function.Body.ImmutableLocal = &first
			terminal.Functions[index] = function
		}
	}
	if err := coreir.ValidateProgram(terminal); err == nil || !strings.Contains(err.Error(), "immutable-local initializer") {
		t.Fatalf("terminal-match Core error = %v", err)
	}
	if _, err := gobackend.Generate(terminal); err == nil {
		t.Fatal("Go backend accepted terminal-return helper match Core")
	}
}
