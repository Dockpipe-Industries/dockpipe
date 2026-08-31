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

const priorLocalCarrierMatchSource = `public Record Row { public string Id; public string Name; }
public Class Root {
	public Optional<Row> Find(List<Row> rows, string id) => find_by(rows, Row.Id, id);
	public string Resolve(List<Row> rows, string id) {
		string fallback = "";
		Optional<Row> carrier = Find(rows, id);
		string selected = match(carrier){ some(row) => row.Name, none => fallback };
		return trim(selected);
	}
	public Result<List<Row>, string> Snapshot(Result<List<Row>, string> snapshot) => snapshot;
	public int SnapshotCount(Result<List<Row>, string> snapshot) {
		Result<List<Row>, string> carrier = Snapshot(snapshot);
		int selected = match(carrier){ ok(rows) => count(rows), err(problem) => 0 };
		return selected;
	}
	public Result<string, string> Validate(string input) => input == "" ? err<string, string>("missing") : ok<string, string>(input);
	public string Message(string input) {
		Result<string, string> carrier = Validate(input);
		string selected = match(carrier){ ok(value) => value, err(problem) => problem };
		return selected;
	}
	public Result<int, ArithmeticError> Add(int left, int right) => left + right;
	public int AddOrZero(int left, int right) {
		Result<int, ArithmeticError> carrier = Add(left, right);
		int selected = match(carrier){ ok(value) => value, err(problem) => 0 };
		return selected;
	}
	public Result<float, ArithmeticError> Divide(float left, float right) => left / right;
	public float DivideOrZero(float left, float right) {
		Result<float, ArithmeticError> carrier = Divide(left, right);
		float selected = match(carrier){ ok(value) => value, err(problem) => 0.0 };
		return selected;
	}
}`

func TestV480PriorLocalCarrierMatchPipeline(t *testing.T) {
	input := semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", "prior-local-carrier-match.pipe", priorLocalCarrierMatchSource)}, nil)
	input.LanguageContract = PipeLangLanguageContractV480
	analysis := AnalyzeSemanticModuleSet(input)
	if err := analysis.Error(); err != nil {
		t.Fatal(err)
	}
	projection, err := BuildSemanticProjection(analysis)
	if err != nil {
		t.Fatal(err)
	}
	if projection.LanguageContract != PipeLangLanguageContractV480 || projection.Schema != PipeLangSemanticProjectionVersion || projection.CompilerContract != PipeLangCompilerContract {
		t.Fatalf("projection contract = %#v", projection)
	}

	identity := semanticMethodNamed(t, analysis, "Resolve").Identity
	typed, err := LowerSemanticMethodToHIR(analysis, identity)
	if err != nil {
		t.Fatal(err)
	}
	resolveHIR := hirFunctionNamed(t, typed, "Resolve")
	firstHIR := resolveHIR.Body.ImmutableLocal
	if typed.LanguageContract != coreir.LanguageContractV480 || firstHIR == nil || firstHIR.Return == nil || firstHIR.Return.ImmutableLocal == nil {
		t.Fatalf("Resolve first-local HIR = %#v", resolveHIR.Body)
	}
	carrierHIR := firstHIR.Return.ImmutableLocal
	if carrierHIR.Return == nil || carrierHIR.Return.ImmutableLocal == nil {
		t.Fatalf("Resolve carrier-local HIR = %#v", resolveHIR.Body)
	}
	matchedHIR := carrierHIR.Return.ImmutableLocal
	if carrierHIR.Initializer.Kind != hir.ExprCall || matchedHIR.Initializer.Kind != hir.ExprMatch || matchedHIR.Initializer.Match == nil || matchedHIR.Initializer.Match.Value == nil || matchedHIR.Initializer.Match.Value.Kind != hir.ExprReference || matchedHIR.Initializer.Match.Value.Reference == nil || matchedHIR.Initializer.Match.Value.Reference.Kind != hir.BindingLocal || matchedHIR.Initializer.Match.Value.Reference.Position != carrierHIR.Binding.Position {
		t.Fatalf("Resolve prior-local HIR = %#v", resolveHIR.Body)
	}

	program, err := LowerHIRToCore(typed)
	if err != nil {
		t.Fatal(err)
	}
	resolve := coreFunctionNamed(t, program, "Resolve")
	first := resolve.Body.ImmutableLocal
	if first == nil || first.Return == nil || first.Return.ImmutableLocal == nil {
		t.Fatalf("Resolve first-local Core = %#v", resolve.Body)
	}
	carrier := first.Return.ImmutableLocal
	if carrier.Return == nil || carrier.Return.ImmutableLocal == nil {
		t.Fatalf("Resolve carrier-local Core = %#v", resolve.Body)
	}
	matched := carrier.Return.ImmutableLocal
	if carrier.Initializer.Kind != coreir.ExprCall || matched.Initializer.Kind != coreir.ExprMatch || matched.Initializer.Match == nil || matched.Initializer.Match.Value == nil || matched.Initializer.Match.Value.Kind != coreir.ExprReference || matched.Initializer.Match.Value.Parameter == nil || *matched.Initializer.Match.Value.Parameter != carrier.Position {
		t.Fatalf("Resolve prior-local Core = %#v", resolve.Body)
	}

	listType := resolve.Parameters[0].Type
	rowType := listType.List.Element
	row := func(id, name string) coreeval.Value {
		return coreeval.Value{Type: rowType, Record: []coreeval.Value{{Type: rowType.Record.Fields[0].Type, String: id}, {Type: rowType.Record.Fields[1].Type, String: name}}}
	}
	rows := coreeval.Value{Type: listType, List: []coreeval.Value{row("one", "  worker  "), row("two", "api")}}
	text := coreir.Type{Kind: coreir.TypePrimitive, Primitive: coreir.PrimitiveString}
	entry := coreir.SemanticIdentity{PackageID: string(identity.PackageID), Path: string(identity.Path)}
	found, err := coreeval.EvaluateProgram(program, entry, []coreeval.Value{rows, {Type: text, String: "one"}})
	if err != nil || !found.OK || found.Value.String != "worker" {
		t.Fatalf("present = %#v, %v", found, err)
	}
	missing, err := coreeval.EvaluateProgram(program, entry, []coreeval.Value{rows, {Type: text, String: "missing"}})
	if err != nil || !missing.OK || missing.Value.String != "" {
		t.Fatalf("absent = %#v, %v", missing, err)
	}

	generated, err := gobackend.Generate(program)
	if err != nil {
		t.Fatal(err)
	}
	generatedAgain, err := gobackend.Generate(program)
	if err != nil || string(generatedAgain) != string(generated) {
		t.Fatalf("prior-local carrier-match Go is nondeterministic: %v", err)
	}
	if strings.Count(string(generated), " := PipeLangFind(") != 1 || !strings.Contains(string(generated), "matched := p3") {
		t.Fatalf("generated Go lacks once-only prior-local carrier matching:\n%s", generated)
	}
	compileAndRunGeneratedGoFiles(t, generated, []byte(fmt.Sprintf(`package %s

import "testing"

func TestGeneratedPriorLocalCarrierMatch(t *testing.T) {
	rows := []PipeLangRecordTestPackageAppRootRow{{Id: "one", Name: "  worker  "}, {Id: "two", Name: "api"}}
	if got := PipeLangResolve(rows, "one"); got != "worker" { t.Fatalf("present = %%q", got) }
	if got := PipeLangResolve(rows, "missing"); got != "" { t.Fatalf("absent = %%q", got) }
}
`, gobackend.PackageName)))

	testV450SnapshotMatchLocal(t, analysis)
	testV480ResultCarrierMatch(t, analysis)
	testV480ArithmeticCarrierMatch(t, analysis)
}

func testV480ResultCarrierMatch(t *testing.T, analysis *Analysis) {
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
	text := function.Parameters[0].Type
	for _, test := range []struct{ input, want string }{{"ready", "ready"}, {"", "missing"}} {
		outcome, evalErr := coreeval.EvaluateProgram(program, entry, []coreeval.Value{{Type: text, String: test.input}})
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

func TestGeneratedResultPriorLocalCarrierMatch(t *testing.T) {
	if got := PipeLangMessage("ready"); got != "ready" { t.Fatalf("success = %%q", got) }
	if got := PipeLangMessage(""); got != "missing" { t.Fatalf("failure = %%q", got) }
}
`, gobackend.PackageName)))
}

func testV480ArithmeticCarrierMatch(t *testing.T, analysis *Analysis) {
	t.Helper()
	for _, test := range []struct {
		method string
		args   []coreeval.Value
		want   float64
	}{
		{method: "AddOrZero", args: coreIntArguments(20, 22), want: 42},
		{method: "AddOrZero", args: coreIntArguments(math.MaxInt64, 1), want: 0},
		{method: "DivideOrZero", args: coreFloatArguments(5, 2), want: 2.5},
		{method: "DivideOrZero", args: coreFloatArguments(1, 0), want: 0},
	} {
		identity := semanticMethodNamed(t, analysis, test.method).Identity
		typed, err := LowerSemanticMethodToHIR(analysis, identity)
		if err != nil {
			t.Fatal(err)
		}
		program, err := LowerHIRToCore(typed)
		if err != nil {
			t.Fatal(err)
		}
		outcome, err := coreeval.EvaluateProgram(program, coreir.SemanticIdentity{PackageID: string(identity.PackageID), Path: string(identity.Path)}, test.args)
		if err != nil || !outcome.OK {
			t.Fatalf("%s = %#v, %v", test.method, outcome, err)
		}
		got := outcome.Value.Float
		if test.method == "AddOrZero" {
			got = float64(outcome.Value.Int)
		}
		if got != test.want {
			t.Fatalf("%s = %v, want %v", test.method, got, test.want)
		}
		generated, err := gobackend.Generate(program)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Count(string(generated), " := PipeLang"+strings.TrimSuffix(test.method, "OrZero")+"(") != 1 {
			t.Fatalf("%s helper is not called exactly once:\n%s", test.method, generated)
		}
	}
}

func TestV480PriorLocalCarrierMatchRejectsExcludedSource(t *testing.T) {
	tests := []struct {
		name     string
		contract LanguageContract
		source   string
		message  string
	}{
		{name: "prior contract", contract: PipeLangLanguageContractV470, source: `public Class Root { public Optional<string> Read(string value) => some(value); public string Resolve(string value) { Optional<string> carrier = Read(value); string selected = match(carrier){ some(item) => item, none => "" }; return selected; } }`, message: "requires language contract v0.48.0"},
		{name: "non-call carrier", contract: PipeLangLanguageContractV480, source: `public Class Root { public string Resolve(Optional<string> value) { Optional<string> carrier = value; string selected = match(carrier){ some(item) => item, none => "" }; return selected; } }`, message: "adjacent helper-call carrier"},
		{name: "intervening local", contract: PipeLangLanguageContractV480, source: `public Class Root { public Optional<string> Read(string value) => some(value); public string Resolve(string value) { Optional<string> carrier = Read(value); Optional<string> copy = carrier; string selected = match(carrier){ some(item) => item, none => "" }; return selected; } }`, message: "adjacent helper-call carrier"},
		{name: "wrong carrier local", contract: PipeLangLanguageContractV480, source: `public Class Root { public Optional<string> Read(string value) => some(value); public string Resolve(string value) { Optional<string> carrier = Read(value); Optional<string> copy = carrier; string selected = match(copy){ some(item) => item, none => "" }; return selected; } }`, message: "adjacent helper-call carrier"},
		{name: "computed argument", contract: PipeLangLanguageContractV480, source: `public Class Root { public Optional<string> Read(string value) => some(value); public string Resolve(string value) { Optional<string> carrier = Read(trim(value)); string selected = match(carrier){ some(item) => item, none => "" }; return selected; } }`, message: "every caller parameter"},
		{name: "multiple matches", contract: PipeLangLanguageContractV480, source: `public Class Root { public Optional<string> Read(string value) => some(value); public string Resolve(string value) { Optional<string> carrier = Read(value); string first = match(carrier){ some(item) => item, none => "" }; string second = match(carrier){ some(item) => item, none => first }; return second; } }`, message: "exactly one"},
		{name: "terminal return", contract: PipeLangLanguageContractV480, source: `public Class Root { public Optional<string> Read(string value) => some(value); public string Resolve(string value) { Optional<string> carrier = Read(value); return match(carrier){ some(item) => item, none => "" }; } }`, message: "adjacent helper-call carrier"},
		{name: "reversed arms", contract: PipeLangLanguageContractV480, source: `public Class Root { public Optional<string> Read(string value) => some(value); public string Resolve(string value) { Optional<string> carrier = Read(value); string selected = match(carrier){ none => "", some(item) => item }; return selected; } }`, message: "source-ordered"},
		{name: "wildcard arm", contract: PipeLangLanguageContractV480, source: `public Class Root { public Optional<string> Read(string value) => some(value); public string Resolve(string value) { Optional<string> carrier = Read(value); string selected = match(carrier){ some(item) => item, _ => "" }; return selected; } }`, message: "source-ordered"},
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

func TestV480PriorLocalCarrierMatchCoreRejectsMalformedComposition(t *testing.T) {
	input := semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", "prior-local-carrier-match-core.pipe", priorLocalCarrierMatchSource)}, nil)
	input.LanguageContract = PipeLangLanguageContractV480
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
	downgraded.LanguageContract = coreir.LanguageContractV470
	if err := coreir.ValidateProgram(downgraded); err == nil || !strings.Contains(err.Error(), "requires language contract \"v0.48.0\"") {
		t.Fatalf("downgraded Core error = %v", err)
	}

	badReference := program
	badReference.Functions = append([]coreir.Function(nil), program.Functions...)
	for index := range badReference.Functions {
		if badReference.Functions[index].Name == "AddOrZero" {
			function := badReference.Functions[index]
			carrier := *function.Body.ImmutableLocal
			matched := *carrier.Return.ImmutableLocal
			initializer := *matched.Initializer
			match := *initializer.Match
			value := *match.Value
			position := 0
			value.Parameter = &position
			match.Value = &value
			initializer.Match = &match
			matched.Initializer = &initializer
			carrier.Return.ImmutableLocal = &matched
			function.Body.ImmutableLocal = &carrier
			badReference.Functions[index] = function
		}
	}
	if err := coreir.ValidateProgram(badReference); err == nil || !strings.Contains(err.Error(), "reference type does not match") {
		t.Fatalf("bad-reference Core error = %v", err)
	}
	if _, err := gobackend.Generate(badReference); err == nil {
		t.Fatal("Go backend accepted malformed prior-local carrier match Core")
	}
}
