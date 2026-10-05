package pipelang

import (
	"bytes"
	"fmt"
	"os"
	"testing"

	"dockpipe/src/lib/pipelang/coreeval"
	"dockpipe/src/lib/pipelang/coreir"
	"dockpipe/src/lib/pipelang/gobackend"
	"dockpipe/src/lib/pipelang/hir"
)

func TestV310NamedRecordPredicateFilterPipeline(t *testing.T) {
	source, err := os.ReadFile("testdata/record-list-filter-predicate.pipe")
	if err != nil {
		t.Fatal(err)
	}
	input := semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", "record-list-filter-predicate.pipe", string(source))}, nil)
	input.LanguageContract = PipeLangLanguageContractV310
	analysis := AnalyzeSemanticModuleSet(input)
	if err := analysis.Error(); err != nil {
		t.Fatal(err)
	}
	projection, err := BuildSemanticProjection(analysis)
	if err != nil {
		t.Fatal(err)
	}
	if projection.LanguageContract != PipeLangLanguageContractV310 {
		t.Fatalf("projection contract = %q", projection.LanguageContract)
	}
	search := semanticMethodNamed(t, analysis, "Search")
	typed, err := LowerSemanticMethodToHIR(analysis, search.Identity)
	if err != nil {
		t.Fatal(err)
	}
	if len(typed.Functions) != 2 || typed.Functions[0].Name != "Matches" || typed.Functions[1].Name != "Search" || typed.Functions[1].Body.Kind != hir.ExprListFilterPredicate {
		t.Fatalf("HIR = %#v", typed)
	}
	core, err := LowerHIRToCore(typed)
	if err != nil {
		t.Fatal(err)
	}
	if err := coreir.ValidateProgram(core); err != nil {
		t.Fatal(err)
	}
	filter := core.Functions[1].Body.ListFilterPredicate
	if filter == nil || filter.Predicate.Path != core.Functions[0].Identity.Path || len(filter.Arguments) != 1 {
		t.Fatalf("Core filter = %#v", filter)
	}
	missingTarget := core
	missingTarget.Functions = append([]coreir.Function(nil), core.Functions[1:]...)
	if err := coreir.ValidateProgram(missingTarget); err == nil {
		t.Fatal("Core accepted an unresolved predicate identity")
	}
	priorHIR := typed
	priorHIR.LanguageContract = coreir.LanguageContractV300
	if _, err := LowerHIRToCore(priorHIR); err == nil {
		t.Fatal("v0.30 HIR accepted named predicate filtering")
	}
	listType := core.Functions[1].Parameters[0].Type
	rowType := listType.List.Element
	rows := coreeval.Value{Type: listType, List: []coreeval.Value{
		v310PredicateRow(rowType, "one", "nginx", "running"),
		v310PredicateRow(rowType, "two", "redis", "exited"),
		v310PredicateRow(rowType, "three", "api", "RUNNING"),
	}}
	query := coreeval.Value{Type: core.Functions[1].Parameters[1].Type, String: " running "}
	outcome, err := coreeval.EvaluateProgram(core, core.Functions[1].Identity, []coreeval.Value{rows, query})
	if err != nil || !outcome.OK || outcome.Value.List == nil || len(outcome.Value.List) != 2 || outcome.Value.List[0].Record[0].String != "one" || outcome.Value.List[1].Record[0].String != "three" {
		t.Fatalf("outcome = %#v (%v)", outcome, err)
	}
	rows.List[0].Record[0].String = "mutated"
	if outcome.Value.List[0].Record[0].String != "one" {
		t.Fatal("filter result aliases caller-owned records")
	}
	empty := coreeval.Value{Type: listType, List: make([]coreeval.Value, 0)}
	emptyOutcome, err := coreeval.EvaluateProgram(core, core.Functions[1].Identity, []coreeval.Value{empty, query})
	if err != nil || emptyOutcome.Value.List == nil || len(emptyOutcome.Value.List) != 0 {
		t.Fatalf("empty = %#v (%v)", emptyOutcome, err)
	}
	invalid := coreeval.Value{Type: listType, List: append([]coreeval.Value(nil), rows.List...)}
	invalid.List[1] = v310PredicateRow(rowType, "two", string([]byte{0xff}), "exited")
	if _, err := coreeval.EvaluateProgram(core, core.Functions[1].Identity, []coreeval.Value{invalid, query}); err == nil {
		t.Fatal("filter accepted invalid unselected row data")
	}
	invalidQuery := query
	invalidQuery.String = string([]byte{0xff})
	if _, err := coreeval.EvaluateProgram(core, core.Functions[1].Identity, []coreeval.Value{empty, invalidQuery}); err == nil { t.Fatal("empty filter hid an invalid predicate argument") }
	if _, err := coreeval.Evaluate(core.Functions[1], []coreeval.Value{rows, query}); err == nil {
		t.Fatal("single-function evaluator bypassed predicate program resolution")
	}
	generated, err := gobackend.Generate(core)
	if err != nil {
		t.Fatal(err)
	}
	second, err := gobackend.Generate(core)
	if err != nil || !bytes.Equal(generated, second) {
		t.Fatal("generated Go is nondeterministic")
	}
	compileAndRunGeneratedGoFiles(t, generated, []byte(v310PredicateGeneratedGoTest()))
	prior := core
	prior.LanguageContract = coreir.LanguageContractV300
	if _, err := gobackend.Generate(prior); err == nil {
		t.Fatal("v0.30 Core accepted named predicate filtering")
	}
}

func TestV310NamedRecordPredicateFilterExclusions(t *testing.T) {
	valid, err := os.ReadFile("testdata/record-list-filter-predicate.pipe")
	if err != nil {
		t.Fatal(err)
	}
	prior := semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", "prior.pipe", string(valid))}, nil)
	prior.LanguageContract = PipeLangLanguageContractV300
	if analysis := AnalyzeSemanticModuleSet(prior); !analysis.Diagnostics.HasErrors() {
		t.Fatal("v0.30 accepted filter")
	}
	excluded := []string{
		`public Record Row { public string Name; } public Class Root { public bool Matches(Row row) => row.Name == "x"; public List<Row> Search(List<Row> rows) => filter(rows, Matches); }`,
		`public Record Row { public string Name; } public Class Root { private bool Matches(Row row, string q) => contains_casefolded(row.Name, q); public List<Row> Search(List<Row> rows, string q) => filter(rows, Matches, q); }`,
		`public Record Row { public string Name; } public Class Root { public bool Matches(Row row, Row other) => row == other; public List<Row> Search(List<Row> rows, Row other) => filter(rows, Matches, other); }`,
		`public Record Row { public string Name; } public Class Root { public bool Matches(Row row, string q) => row.Name + q == q; public List<Row> Search(List<Row> rows, string q) => filter(rows, Matches, q); }`,
		`public Record Row { public string Name; } public Class Root { public bool Matches(Row row, string q) => contains_casefolded(row.Name, q); public List<Row> Search(List<Row> rows, string q) => filter(rows, Missing, q); }`,
		`public Record Row { public string Name; } public Class Root { public bool Matches(Row row, string q) => contains_casefolded(row.Name, q); public List<Row> Search(List<Row> rows, string q) => filter(rows, Matches, "literal"); }`,
	}
	for index, source := range excluded {
		input := semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", fmt.Sprintf("excluded-%d.pipe", index), source)}, nil)
		input.LanguageContract = PipeLangLanguageContractV310
		if analysis := AnalyzeSemanticModuleSet(input); !analysis.Diagnostics.HasErrors() {
			t.Fatalf("excluded form %d accepted", index)
		}
	}
}

func TestV310PreservesV300MultiKeySort(t *testing.T) {
	source, err := os.ReadFile("testdata/record-list-sort-by-ordinals.pipe")
	if err != nil {
		t.Fatal(err)
	}
	input := semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", "record-list-sort-by-ordinals.pipe", string(source))}, nil)
	input.LanguageContract = PipeLangLanguageContractV310
	analysis := AnalyzeSemanticModuleSet(input)
	if err := analysis.Error(); err != nil {
		t.Fatal(err)
	}
	typed, err := LowerSemanticMethodToHIR(analysis, semanticMethodNamed(t, analysis, "SortRows").Identity)
	if err != nil {
		t.Fatal(err)
	}
	if typed.LanguageContract != coreir.LanguageContractV310 || typed.Functions[0].Body.Kind != hir.ExprListSortByOrdinalTexts {
		t.Fatalf("preserved HIR = %#v", typed)
	}
	core, err := LowerHIRToCore(typed)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := gobackend.Generate(core); err != nil {
		t.Fatal(err)
	}
}

func TestV310NamedPredicateUsesOrdinalTextComparison(t *testing.T) {
	const source = `public Record Row { public string Name; } public Class Root { public bool AtOrAfter(Row row, string floor) => row.Name >= floor; public List<Row> Search(List<Row> rows, string floor) => filter(rows, AtOrAfter, floor); }`
	input := semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", "ordinal-predicate.pipe", source)}, nil)
	input.LanguageContract = PipeLangLanguageContractV310
	analysis := AnalyzeSemanticModuleSet(input)
	if err := analysis.Error(); err != nil { t.Fatal(err) }
	typed, err := LowerSemanticMethodToHIR(analysis, semanticMethodNamed(t, analysis, "Search").Identity)
	if err != nil { t.Fatal(err) }
	core, err := LowerHIRToCore(typed)
	if err != nil { t.Fatal(err) }
	listType := core.Functions[1].Parameters[0].Type
	rowType := listType.List.Element
	rows := coreeval.Value{Type: listType, List: []coreeval.Value{v310PredicateRow(rowType, "a"), v310PredicateRow(rowType, "B"), v310PredicateRow(rowType, "b")}}
	floor := coreeval.Value{Type: core.Functions[1].Parameters[1].Type, String: "a"}
	outcome, err := coreeval.EvaluateProgram(core, core.Functions[1].Identity, []coreeval.Value{rows, floor})
	if err != nil || len(outcome.Value.List) != 2 || outcome.Value.List[0].Record[0].String != "a" || outcome.Value.List[1].Record[0].String != "b" { t.Fatalf("ordinal predicate = %#v (%v)", outcome, err) }
	if _, err := gobackend.Generate(core); err != nil { t.Fatal(err) }
}

func v310PredicateRow(rowType coreir.Type, values ...string) coreeval.Value {
	fields := make([]coreeval.Value, len(values))
	for index, value := range values {
		fields[index] = coreeval.Value{Type: rowType.Record.Fields[index].Type, String: value}
	}
	return coreeval.Value{Type: rowType, Record: fields}
}

func v310PredicateGeneratedGoTest() string {
	return fmt.Sprintf(`package %s

import "testing"

func expectPredicatePanic(t *testing.T, invoke func()) { t.Helper(); defer func() { if recover() == nil { t.Fatal("expected panic") } }(); invoke() }

func TestGeneratedNamedPredicateFilter(t *testing.T) {
	rows := []PipeLangRecordTestPackageAppRootContainerrow{{Id: "one", Name: "nginx", State: "running"}, {Id: "two", Name: "redis", State: "exited"}, {Id: "three", Name: "api", State: "RUNNING"}}
	got := PipeLangSearch(rows, " running ")
	if got == nil || len(got) != 2 || got[0].Id != "one" || got[1].Id != "three" { t.Fatalf("got = %%#v", got) }
	got[0].Id = "changed"
	if rows[0].Id != "one" { t.Fatal("generated filter aliases input") }
	empty := PipeLangSearch([]PipeLangRecordTestPackageAppRootContainerrow{}, "x")
	if empty == nil || len(empty) != 0 { t.Fatalf("empty = %%#v", empty) }
	invalid := append([]PipeLangRecordTestPackageAppRootContainerrow(nil), rows...)
	invalid[1].Name = string([]byte{0xff})
	expectPredicatePanic(t, func() { PipeLangSearch(invalid, "x") })
	expectPredicatePanic(t, func() { PipeLangSearch([]PipeLangRecordTestPackageAppRootContainerrow{}, string([]byte{0xff})) })
	var nilRows []PipeLangRecordTestPackageAppRootContainerrow
	expectPredicatePanic(t, func() { PipeLangSearch(nilRows, "x") })
}
`, gobackend.PackageName)
}
