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

const generalHelperCarrierMatchSource = `public Record Row { public string Id; public string Name; }
public Class Root {
	public Optional<Row> Find(List<Row> rows, string id) => find_by(rows, Row.Id, id);
	public string Resolve(List<Row> rows, string id) => match(Find(rows, id)){ some(row) => trim(row.Name), none => "" };
	public Result<List<Row>, string> Snapshot(Result<List<Row>, string> snapshot) => snapshot;
	public int SnapshotCount(Result<List<Row>, string> snapshot) => match(Snapshot(snapshot)){ ok(rows) => count(rows), err(problem) => 0 };
	public Result<string, string> Validate(string input) => input == "" ? err<string, string>("missing") : ok<string, string>(input);
	public string Message(string input) => match(Validate(input)){ ok(value) => trim(value), err(problem) => problem };
}`

func TestV440GeneralHelperCarrierMatchPipeline(t *testing.T) {
	input := semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", "general-helper-carrier-match.pipe", generalHelperCarrierMatchSource)}, nil)
	input.LanguageContract = PipeLangLanguageContractV440
	analysis := AnalyzeSemanticModuleSet(input)
	if err := analysis.Error(); err != nil {
		t.Fatal(err)
	}
	projection, err := BuildSemanticProjection(analysis)
	if err != nil {
		t.Fatal(err)
	}
	if projection.LanguageContract != PipeLangLanguageContractV440 || projection.Schema != PipeLangSemanticProjectionVersion || projection.CompilerContract != PipeLangCompilerContract {
		t.Fatalf("projection contract = %#v", projection)
	}

	resolveIdentity := semanticMethodNamed(t, analysis, "Resolve").Identity
	typed, err := LowerSemanticMethodToHIR(analysis, resolveIdentity)
	if err != nil {
		t.Fatal(err)
	}
	resolveHIR := hirFunctionNamed(t, typed, "Resolve")
	if typed.LanguageContract != coreir.LanguageContractV440 || resolveHIR.Body.Kind != hir.ExprMatch || resolveHIR.Body.Match == nil || resolveHIR.Body.Match.Value == nil || resolveHIR.Body.Match.Value.Kind != hir.ExprCall || len(resolveHIR.Body.Match.Value.Call.Arguments) != 2 {
		t.Fatalf("Resolve HIR = %#v", resolveHIR.Body)
	}
	core, err := LowerHIRToCore(typed)
	if err != nil {
		t.Fatal(err)
	}
	resolve := coreFunctionNamed(t, core, "Resolve")
	call := resolve.Body.Match.Value.Call
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
	found, err := coreeval.EvaluateProgram(core, entry, []coreeval.Value{rows, {Type: text, String: "one"}})
	if err != nil || !found.OK || found.Value.String != "worker" {
		t.Fatalf("present result = %#v, %v", found, err)
	}
	missing, err := coreeval.EvaluateProgram(core, entry, []coreeval.Value{rows, {Type: text, String: "missing"}})
	if err != nil || !missing.OK || missing.Value.String != "" {
		t.Fatalf("absent result = %#v, %v", missing, err)
	}

	snapshotIdentity := semanticMethodNamed(t, analysis, "SnapshotCount").Identity
	snapshotHIR, err := LowerSemanticMethodToHIR(analysis, snapshotIdentity)
	if err != nil {
		t.Fatal(err)
	}
	snapshotCore, err := LowerHIRToCore(snapshotHIR)
	if err != nil {
		t.Fatal(err)
	}
	snapshotCount := coreFunctionNamed(t, snapshotCore, "SnapshotCount")
	carrier := snapshotCount.Parameters[0].Type
	snapshotRows := coreeval.Value{Type: carrier.Result.Success, List: rows.List}
	success := coreeval.Value{Type: carrier, Result: &coreeval.Outcome{OK: true, Value: snapshotRows}}
	counted, err := coreeval.EvaluateProgram(snapshotCore, coreir.SemanticIdentity{PackageID: string(snapshotIdentity.PackageID), Path: string(snapshotIdentity.Path)}, []coreeval.Value{success})
	if err != nil || !counted.OK || counted.Value.Int != 2 {
		t.Fatalf("snapshot success = %#v, %v", counted, err)
	}
	failureText := coreeval.Value{Type: carrier.Result.Failure, String: "offline"}
	failure := coreeval.Value{Type: carrier, Result: &coreeval.Outcome{Value: coreeval.Value{Type: carrier.Result.Success}, Failure: &failureText}}
	counted, err = coreeval.EvaluateProgram(snapshotCore, coreir.SemanticIdentity{PackageID: string(snapshotIdentity.PackageID), Path: string(snapshotIdentity.Path)}, []coreeval.Value{failure})
	if err != nil || !counted.OK || counted.Value.Int != 0 {
		t.Fatalf("snapshot failure = %#v, %v", counted, err)
	}
	snapshotGenerated, err := gobackend.Generate(snapshotCore)
	if err != nil {
		t.Fatal(err)
	}
	compileAndRunGeneratedGoFiles(t, snapshotGenerated, []byte(fmt.Sprintf(`package %s

import "testing"

func TestGeneratedSnapshotHelperCarrierMatch(t *testing.T) {
	rows := []PipeLangRecordTestPackageAppRootRow{{Id: "one", Name: "worker"}}
	success := PipeLangResult[[]PipeLangRecordTestPackageAppRootRow, string]{OK: true, Value: rows}
	if got := PipeLangSnapshotCount(success); got != 1 { t.Fatalf("success = %%d", got) }
	failure := PipeLangResult[[]PipeLangRecordTestPackageAppRootRow, string]{Error: "offline"}
	if got := PipeLangSnapshotCount(failure); got != 0 { t.Fatalf("failure = %%d", got) }
}
`, gobackend.PackageName)))

	messageIdentity := semanticMethodNamed(t, analysis, "Message").Identity
	messageHIR, err := LowerSemanticMethodToHIR(analysis, messageIdentity)
	if err != nil {
		t.Fatal(err)
	}
	messageCore, err := LowerHIRToCore(messageHIR)
	if err != nil {
		t.Fatal(err)
	}
	message := coreFunctionNamed(t, messageCore, "Message")
	messageEntry := coreir.SemanticIdentity{PackageID: string(messageIdentity.PackageID), Path: string(messageIdentity.Path)}
	messageOutcome, err := coreeval.EvaluateProgram(messageCore, messageEntry, []coreeval.Value{{Type: message.Parameters[0].Type, String: "  ready  "}})
	if err != nil || !messageOutcome.OK || messageOutcome.Value.String != "ready" {
		t.Fatalf("text Result success = %#v, %v", messageOutcome, err)
	}
	messageOutcome, err = coreeval.EvaluateProgram(messageCore, messageEntry, []coreeval.Value{{Type: message.Parameters[0].Type, String: ""}})
	if err != nil || !messageOutcome.OK || messageOutcome.Value.String != "missing" {
		t.Fatalf("text Result failure = %#v, %v", messageOutcome, err)
	}
	messageGenerated, err := gobackend.Generate(messageCore)
	if err != nil {
		t.Fatal(err)
	}
	compileAndRunGeneratedGoFiles(t, messageGenerated, []byte(fmt.Sprintf(`package %s

import "testing"

func TestGeneratedTextResultHelperCarrierMatch(t *testing.T) {
	if got := PipeLangMessage("  ready  "); got != "ready" { t.Fatalf("success = %%q", got) }
	if got := PipeLangMessage(""); got != "missing" { t.Fatalf("failure = %%q", got) }
}
`, gobackend.PackageName)))

	generated, err := gobackend.Generate(core)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(generated), "matched := PipeLangFind(") || !strings.Contains(string(generated), ", p1)") {
		t.Fatalf("generated Go lacks ordered helper arguments:\n%s", generated)
	}
	compileAndRunGeneratedGoFiles(t, generated, []byte(fmt.Sprintf(`package %s

import "testing"

func TestGeneratedGeneralHelperCarrierMatch(t *testing.T) {
	rows := []PipeLangRecordTestPackageAppRootRow{{Id: "one", Name: "  worker  "}, {Id: "two", Name: "api"}}
	if got := PipeLangResolve(rows, "one"); got != "worker" { t.Fatalf("present = %%q", got) }
	if got := PipeLangResolve(rows, "missing"); got != "" { t.Fatalf("absent = %%q", got) }
}
`, gobackend.PackageName)))
}

func TestV440GeneralHelperCarrierMatchRejectsExcludedSource(t *testing.T) {
	tests := []struct {
		name    string
		source  string
		message string
	}{
		{name: "computed argument", source: `public Record Row { public string Id; } public Class Root { public Optional<Row> Find(List<Row> rows, string id) => find_by(rows, Row.Id, id); public string Resolve(List<Row> rows, string id) => match(Find(rows, trim(id))){ some(row) => row.Id, none => "" }; }`, message: "every caller parameter"},
		{name: "extra helper argument", source: `public Class Root { public Result<string, string> Read(string value) => ok<string, string>(value); public string Resolve(string value) => match(Read(value, value)){ ok(item) => item, err(problem) => problem }; }`, message: "every caller parameter"},
		{name: "omitted caller parameter", source: `public Record Row { public string Id; } public Class Root { public Optional<Row> Find(List<Row> rows, string id) => find_by(rows, Row.Id, id); public string Resolve(List<Row> rows, string id, string fallback) => match(Find(rows, id)){ some(row) => row.Id, none => fallback }; }`, message: "every caller parameter"},
		{name: "reordered caller parameters", source: `public Record Row { public string Id; } public Class Root { public Optional<Row> Find(List<Row> rows, string id) => find_by(rows, Row.Id, id); public string Resolve(string id, List<Row> rows) => match(Find(rows, id)){ some(row) => row.Id, none => "" }; }`, message: "declaration order"},
		{name: "private helper", source: `public Class Root { private Optional<string> Read(string value) => some(value); public string Resolve(string value) => match(Read(value)){ some(item) => item, none => "" }; }`, message: "must be public"},
		{name: "cross owner", source: `public Class Other { public Optional<string> Read(string value) => some(value); } public Class Root { public string Resolve(string value) => match(Read(value)){ some(item) => item, none => "" }; }`, message: "has no method"},
		{name: "zero parameters", source: `public Class Root { public Optional<string> Read() => none<string>(); public string Resolve() => match(Read()){ some(value) => value, none => "" }; }`, message: "one or more direct parameters"},
		{name: "arithmetic result", source: `public Class Root { public Result<int, ArithmeticError> Add(int left, int right) => left + right; public int Resolve(int left, int right) => match(Add(left, right)){ ok(value) => value, err(problem) => 0 }; }`, message: "admitted Optional"},
		{name: "reversed arms", source: `public Class Root { public Optional<string> Read(string value) => some(value); public string Resolve(string value) => match(Read(value)){ none => "", some(item) => item }; }`, message: "source-ordered"},
		{name: "wildcard arm", source: `public Class Root { public Optional<string> Read(string value) => some(value); public string Resolve(string value) => match(Read(value)){ some(item) => item, _ => "" }; }`, message: "source-ordered"},
		{name: "nested match", source: `public Class Root { public Optional<string> Read(string value) => some(value); public string Resolve(string value) => match(Read(value)){ some(item) => match(Read(item)){ some(next) => next, none => "" }, none => "" }; }`, message: "exactly one"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			input := semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", test.name+".pipe", test.source)}, nil)
			input.LanguageContract = PipeLangLanguageContractV440
			analysis := AnalyzeSemanticModuleSet(input)
			if len(analysis.Diagnostics) == 0 || !strings.Contains(analysis.Diagnostics[0].Message, test.message) || !analysis.Diagnostics[0].Primary.IsValid() {
				t.Fatalf("diagnostics = %#v", analysis.Diagnostics)
			}
		})
	}
}

func TestV440GeneralHelperCarrierMatchCoreRejectsMalformedComposition(t *testing.T) {
	input := semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", "general-helper-carrier-match-core.pipe", generalHelperCarrierMatchSource)}, nil)
	input.LanguageContract = PipeLangLanguageContractV440
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
	downgraded.LanguageContract = coreir.LanguageContractV430
	if err := coreir.ValidateProgram(downgraded); err == nil || !strings.Contains(err.Error(), "one string parameter") {
		t.Fatalf("downgraded Core error = %v", err)
	}

	badOrder := program
	badOrder.Functions = append([]coreir.Function(nil), program.Functions...)
	for index := range badOrder.Functions {
		if badOrder.Functions[index].Name == "Resolve" {
			function := badOrder.Functions[index]
			match := *function.Body.Match
			value := *match.Value
			call := *value.Call
			call.Arguments = append([]*coreir.Expr(nil), call.Arguments...)
			call.Arguments[0], call.Arguments[1] = call.Arguments[1], call.Arguments[0]
			value.Call = &call
			match.Value = &value
			function.Body.Match = &match
			badOrder.Functions[index] = function
		}
	}
	if err := coreir.ValidateProgram(badOrder); err == nil || !strings.Contains(err.Error(), "argument 1 type mismatch") {
		t.Fatalf("bad argument order Core error = %v", err)
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
		t.Fatal("Go backend accepted malformed general helper-carrier match Core")
	}
}
