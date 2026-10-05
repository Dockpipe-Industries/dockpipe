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

const helperCarrierMatchLocalSource = `public Record Row { public string Id; public string Name; }
public Class Root {
	public Optional<Row> Find(List<Row> rows, string id) => find_by(rows, Row.Id, id);
	public string Normalize(string value) => trim(value);
	public string Resolve(List<Row> rows, string id) {
		string selected = match(Find(rows, id)){ some(row) => row.Name, none => "" };
		string cleaned = Normalize(selected);
		return cleaned;
	}
	public string LegacyResolve(List<Row> rows, string id) => match(Find(rows, id)){ some(row) => Normalize(row.Name), none => "" };
	public Result<List<Row>, string> Snapshot(Result<List<Row>, string> snapshot) => snapshot;
	public int SnapshotCount(Result<List<Row>, string> snapshot) {
		int selected = match(Snapshot(snapshot)){ ok(rows) => count(rows), err(problem) => 0 };
		int copied = selected;
		return copied;
	}
	public Result<string, string> Validate(string input) => input == "" ? err<string, string>("missing") : ok<string, string>(input);
	public string Message(string input) {
		string selected = match(Validate(input)){ ok(value) => value, err(problem) => problem };
		string cleaned = trim(selected);
		return cleaned;
	}
}`

func TestV450HelperCarrierMatchLocalPipeline(t *testing.T) {
	input := semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", "helper-carrier-match-local.pipe", helperCarrierMatchLocalSource)}, nil)
	input.LanguageContract = PipeLangLanguageContractV450
	analysis := AnalyzeSemanticModuleSet(input)
	if err := analysis.Error(); err != nil {
		t.Fatal(err)
	}
	projection, err := BuildSemanticProjection(analysis)
	if err != nil {
		t.Fatal(err)
	}
	if projection.LanguageContract != PipeLangLanguageContractV450 || projection.Schema != PipeLangSemanticProjectionVersion || projection.CompilerContract != PipeLangCompilerContract {
		t.Fatalf("projection contract = %#v", projection)
	}

	resolveIdentity := semanticMethodNamed(t, analysis, "Resolve").Identity
	typed, err := LowerSemanticMethodToHIR(analysis, resolveIdentity)
	if err != nil {
		t.Fatal(err)
	}
	resolveHIR := hirFunctionNamed(t, typed, "Resolve")
	firstHIR := resolveHIR.Body.ImmutableLocal
	if typed.LanguageContract != coreir.LanguageContractV450 || resolveHIR.Body.Kind != hir.ExprImmutableLocal || firstHIR == nil || firstHIR.Initializer.Kind != hir.ExprMatch || firstHIR.Initializer.Match == nil || firstHIR.Initializer.Match.Value == nil || firstHIR.Initializer.Match.Value.Kind != hir.ExprCall || len(firstHIR.Initializer.Match.Value.Call.Arguments) != 2 {
		t.Fatalf("Resolve HIR = %#v", resolveHIR.Body)
	}
	secondHIR := firstHIR.Return.ImmutableLocal
	if firstHIR.Return.Kind != hir.ExprImmutableLocal || secondHIR == nil || secondHIR.Initializer.Kind != hir.ExprCall || secondHIR.Return.Kind != hir.ExprReference {
		t.Fatalf("Resolve continuation HIR = %#v", firstHIR.Return)
	}

	core, err := LowerHIRToCore(typed)
	if err != nil {
		t.Fatal(err)
	}
	resolve := coreFunctionNamed(t, core, "Resolve")
	first := resolve.Body.ImmutableLocal
	if resolve.Body.Kind != coreir.ExprImmutableLocal || first == nil || first.Position != 2 || first.Initializer.Kind != coreir.ExprMatch || first.Initializer.Match == nil || first.Initializer.Match.Value == nil || first.Initializer.Match.Value.Kind != coreir.ExprCall {
		t.Fatalf("Resolve Core = %#v", resolve.Body)
	}
	call := first.Initializer.Match.Value.Call
	if len(call.Arguments) != 2 || call.Arguments[0].Parameter == nil || *call.Arguments[0].Parameter != 0 || call.Arguments[1].Parameter == nil || *call.Arguments[1].Parameter != 1 {
		t.Fatalf("Resolve Core call = %#v", call)
	}
	second := first.Return.ImmutableLocal
	if second == nil || second.Position != 3 || second.Initializer.Kind != coreir.ExprCall || second.Return.Kind != coreir.ExprReference {
		t.Fatalf("Resolve continuation Core = %#v", first.Return)
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
	found, err := coreeval.EvaluateProgram(core, entry, []coreeval.Value{rows, {Type: text, String: "one"}})
	if err != nil || !found.OK || found.Value.String != "worker" {
		t.Fatalf("present result = %#v, %v", found, err)
	}
	missing, err := coreeval.EvaluateProgram(core, entry, []coreeval.Value{rows, {Type: text, String: "missing"}})
	if err != nil || !missing.OK || missing.Value.String != "" {
		t.Fatalf("absent result = %#v, %v", missing, err)
	}

	testV450SnapshotMatchLocal(t, analysis)
	testV450TextResultMatchLocal(t, analysis)

	generated, err := gobackend.Generate(core)
	if err != nil {
		t.Fatal(err)
	}
	generatedAgain, err := gobackend.Generate(core)
	if err != nil || string(generatedAgain) != string(generated) {
		t.Fatalf("helper-carrier match local generated Go is nondeterministic: %v", err)
	}
	if strings.Count(string(generated), "matched := PipeLangFind(") != 1 || !strings.Contains(string(generated), "p2 := func() string") || !strings.Contains(string(generated), "p3 := PipeLangNormalize(p2)") {
		t.Fatalf("generated Go lacks once-only local helper match and continuation:\n%s", generated)
	}
	compileAndRunGeneratedGoFiles(t, generated, []byte(fmt.Sprintf(`package %s

import "testing"

func TestGeneratedHelperCarrierMatchLocal(t *testing.T) {
	rows := []PipeLangRecordTestPackageAppRootRow{{Id: "one", Name: "  worker  "}, {Id: "two", Name: "api"}}
	if got := PipeLangResolve(rows, "one"); got != "worker" { t.Fatalf("present = %%q", got) }
	if got := PipeLangResolve(rows, "missing"); got != "" { t.Fatalf("absent = %%q", got) }
}
`, gobackend.PackageName)))
}

func testV450SnapshotMatchLocal(t *testing.T, analysis *Analysis) {
	t.Helper()
	identity := semanticMethodNamed(t, analysis, "SnapshotCount").Identity
	typed, err := LowerSemanticMethodToHIR(analysis, identity)
	if err != nil {
		t.Fatal(err)
	}
	program, err := LowerHIRToCore(typed)
	if err != nil {
		t.Fatal(err)
	}
	function := coreFunctionNamed(t, program, "SnapshotCount")
	carrier := function.Parameters[0].Type
	rowType := carrier.Result.Success.List.Element
	rows := coreeval.Value{Type: carrier.Result.Success, List: []coreeval.Value{{Type: rowType, Record: []coreeval.Value{{Type: rowType.Record.Fields[0].Type, String: "one"}, {Type: rowType.Record.Fields[1].Type, String: "worker"}}}}}
	success := coreeval.Value{Type: carrier, Result: &coreeval.Outcome{OK: true, Value: rows}}
	entry := coreir.SemanticIdentity{PackageID: string(identity.PackageID), Path: string(identity.Path)}
	counted, err := coreeval.EvaluateProgram(program, entry, []coreeval.Value{success})
	if err != nil || !counted.OK || counted.Value.Int != 1 {
		t.Fatalf("snapshot success = %#v, %v", counted, err)
	}
	failureText := coreeval.Value{Type: carrier.Result.Failure, String: "offline"}
	failure := coreeval.Value{Type: carrier, Result: &coreeval.Outcome{Value: coreeval.Value{Type: carrier.Result.Success}, Failure: &failureText}}
	counted, err = coreeval.EvaluateProgram(program, entry, []coreeval.Value{failure})
	if err != nil || !counted.OK || counted.Value.Int != 0 {
		t.Fatalf("snapshot failure = %#v, %v", counted, err)
	}
	generated, err := gobackend.Generate(program)
	if err != nil {
		t.Fatal(err)
	}
	compileAndRunGeneratedGoFiles(t, generated, []byte(fmt.Sprintf(`package %s

import "testing"

func TestGeneratedSnapshotMatchLocal(t *testing.T) {
	rows := []PipeLangRecordTestPackageAppRootRow{{Id: "one", Name: "worker"}}
	success := PipeLangResult[[]PipeLangRecordTestPackageAppRootRow, string]{OK: true, Value: rows}
	if got := PipeLangSnapshotCount(success); got != 1 { t.Fatalf("success = %%d", got) }
	failure := PipeLangResult[[]PipeLangRecordTestPackageAppRootRow, string]{Error: "offline"}
	if got := PipeLangSnapshotCount(failure); got != 0 { t.Fatalf("failure = %%d", got) }
}
`, gobackend.PackageName)))
}

func testV450TextResultMatchLocal(t *testing.T, analysis *Analysis) {
	t.Helper()
	identity := semanticMethodNamed(t, analysis, "Message").Identity
	typed, err := LowerSemanticMethodToHIR(analysis, identity)
	if err != nil {
		t.Fatal(err)
	}
	program, err := LowerHIRToCore(typed)
	if err != nil {
		t.Fatal(err)
	}
	function := coreFunctionNamed(t, program, "Message")
	entry := coreir.SemanticIdentity{PackageID: string(identity.PackageID), Path: string(identity.Path)}
	for _, test := range []struct{ input, want string }{{input: "  ready  ", want: "ready"}, {input: "", want: "missing"}} {
		outcome, evalErr := coreeval.EvaluateProgram(program, entry, []coreeval.Value{{Type: function.Parameters[0].Type, String: test.input}})
		if evalErr != nil || !outcome.OK || outcome.Value.String != test.want {
			t.Fatalf("Message(%q) = %#v, %v", test.input, outcome, evalErr)
		}
	}
	generated, err := gobackend.Generate(program)
	if err != nil {
		t.Fatal(err)
	}
	compileAndRunGeneratedGoFiles(t, generated, []byte(fmt.Sprintf(`package %s

import "testing"

func TestGeneratedTextResultMatchLocal(t *testing.T) {
	if got := PipeLangMessage("  ready  "); got != "ready" { t.Fatalf("success = %%q", got) }
	if got := PipeLangMessage(""); got != "missing" { t.Fatalf("failure = %%q", got) }
}
`, gobackend.PackageName)))
}

func TestV450HelperCarrierMatchLocalRejectsExcludedSource(t *testing.T) {
	tests := []struct {
		name     string
		contract LanguageContract
		source   string
		message  string
	}{
		{name: "prior contract", contract: PipeLangLanguageContractV440, source: `public Class Root { public Optional<string> Read(string value) => some(value); public string Resolve(string value) { string selected = match(Read(value)){ some(item) => item, none => "" }; return selected; } }`, message: "complete match carrier"},
		{name: "second local", contract: PipeLangLanguageContractV450, source: `public Class Root { public Optional<string> Read(string value) => some(value); public string Resolve(string value) { string first = value; string selected = match(Read(value)){ some(item) => item, none => "" }; return selected; } }`, message: "first immutable-local initializer"},
		{name: "terminal return", contract: PipeLangLanguageContractV450, source: `public Class Root { public Optional<string> Read(string value) => some(value); public string Resolve(string value) { string first = value; return match(Read(value)){ some(item) => item, none => "" }; } }`, message: "first immutable-local initializer"},
		{name: "computed argument", contract: PipeLangLanguageContractV450, source: `public Class Root { public Optional<string> Read(string value) => some(value); public string Resolve(string value) { string selected = match(Read(trim(value))){ some(item) => item, none => "" }; return selected; } }`, message: "every caller parameter"},
		{name: "reordered arguments", contract: PipeLangLanguageContractV450, source: `public Record Row { public string Id; } public Class Root { public Optional<Row> Find(List<Row> rows, string id) => find_by(rows, Row.Id, id); public string Resolve(string id, List<Row> rows) { string selected = match(Find(rows, id)){ some(row) => row.Id, none => "" }; return selected; } }`, message: "declaration order"},
		{name: "multiple matches", contract: PipeLangLanguageContractV450, source: `public Class Root { public Optional<string> Read(string value) => some(value); public string Resolve(string value) { string first = match(Read(value)){ some(item) => item, none => "" }; string second = match(Read(value)){ some(item) => item, none => "" }; return second; } }`, message: "exactly one"},
		{name: "wrong local type", contract: PipeLangLanguageContractV450, source: `public Class Root { public Optional<string> Read(string value) => some(value); public string Resolve(string value) { bool selected = match(Read(value)){ some(item) => item, none => "" }; return value; } }`, message: "initializer has type"},
		{name: "private helper", contract: PipeLangLanguageContractV450, source: `public Class Root { private Optional<string> Read(string value) => some(value); public string Resolve(string value) { string selected = match(Read(value)){ some(item) => item, none => "" }; return selected; } }`, message: "must be public"},
		{name: "arithmetic Result", contract: PipeLangLanguageContractV450, source: `public Class Root { public Result<int, ArithmeticError> Add(int left, int right) => left + right; public int Resolve(int left, int right) { int selected = match(Add(left, right)){ ok(value) => value, err(problem) => 0 }; return selected; } }`, message: "admitted Optional"},
		{name: "reversed arms", contract: PipeLangLanguageContractV450, source: `public Class Root { public Optional<string> Read(string value) => some(value); public string Resolve(string value) { string selected = match(Read(value)){ none => "", some(item) => item }; return selected; } }`, message: "source-ordered"},
		{name: "wildcard arm", contract: PipeLangLanguageContractV450, source: `public Class Root { public Optional<string> Read(string value) => some(value); public string Resolve(string value) { string selected = match(Read(value)){ some(item) => item, _ => "" }; return selected; } }`, message: "source-ordered"},
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

func TestV450HelperCarrierMatchLocalCoreRejectsMalformedComposition(t *testing.T) {
	input := semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", "helper-carrier-match-local-core.pipe", helperCarrierMatchLocalSource)}, nil)
	input.LanguageContract = PipeLangLanguageContractV450
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
	downgraded.LanguageContract = coreir.LanguageContractV440
	if err := coreir.ValidateProgram(downgraded); err == nil || !strings.Contains(err.Error(), "complete match carrier") {
		t.Fatalf("downgraded Core error = %v", err)
	}

	badArms := program
	badArms.Functions = append([]coreir.Function(nil), program.Functions...)
	for index := range badArms.Functions {
		if badArms.Functions[index].Name == "Resolve" {
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
		t.Fatal("Go backend accepted malformed helper-carrier match local Core")
	}
}
