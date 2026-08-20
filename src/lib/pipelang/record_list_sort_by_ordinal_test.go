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

func TestV280RecordListSortByOrdinalPipeline(t *testing.T) {
	analysis, typed, core, generated := v280SortByOrdinalPipeline(t)
	projection, err := BuildSemanticProjection(analysis)
	if err != nil {
		t.Fatal(err)
	}
	method := projectedMemberNamed(t, projectedTypeNamed(t, projection, "Root"), "SortRows")
	if projection.Schema != PipeLangSemanticProjectionVersion || projection.LanguageContract != PipeLangLanguageContractV280 || method.Type.Identity == nil || method.Type.Identity.Path != PipeLangListSemanticPath || len(method.Parameters) != 1 || method.Parameters[0].Type.Identity == nil || method.Parameters[0].Type.Identity.Path != PipeLangListSemanticPath {
		t.Fatalf("v0.28 projection = %#v / %#v", projection, method)
	}

	hirSort := typed.Functions[0].Body.ListSortByOrdinalText
	if typed.LanguageContract != coreir.LanguageContractV280 || typed.Functions[0].Body.Kind != hir.ExprListSortByOrdinalText || hirSort == nil || hirSort.Values == nil || hirSort.Values.Reference == nil || hirSort.Values.Reference.Position != 0 || hirSort.Name != "Name" || hirSort.Position != 1 || hirSort.Field.Path != "app.root.containerrow.name" {
		t.Fatalf("v0.28 sort HIR = %#v", typed)
	}
	coreFunction := core.Functions[0]
	coreSort := coreFunction.Body.ListSortByOrdinalText
	if core.LanguageContract != coreir.LanguageContractV280 || coreFunction.Body.Kind != coreir.ExprListSortByOrdinalText || coreSort == nil || coreSort.Name != "Name" || coreSort.Position != 1 || coreSort.Field.Path != "app.root.containerrow.name" {
		t.Fatalf("v0.28 sort Core = %#v", core)
	}

	listType := coreFunction.Parameters[0].Type
	rowType := listType.List.Element
	rows := coreeval.Value{Type: listType, List: []coreeval.Value{
		v280SortRow(rowType, "one", "beta", "first beta"),
		v280SortRow(rowType, "two", "Alpha", "uppercase"),
		v280SortRow(rowType, "three", "beta", "second beta"),
		v280SortRow(rowType, "four", "alpha", "lowercase"),
		v280SortRow(rowType, "five", "e\u0301", "decomposed"),
		v280SortRow(rowType, "six", "é", "composed"),
	}}
	outcome, err := coreeval.Evaluate(coreFunction, []coreeval.Value{rows})
	if err != nil || !outcome.OK || outcome.Value.List == nil {
		t.Fatalf("sort outcome = %#v (%v)", outcome, err)
	}
	want := []string{"two", "four", "one", "three", "five", "six"}
	for index, id := range want {
		if outcome.Value.List[index].Record[0].String != id {
			t.Fatalf("sorted row %d = %#v", index, outcome.Value.List[index])
		}
	}
	rows.List[0].Record[0].String = "mutated"
	if outcome.Value.List[2].Record[0].String != "one" {
		t.Fatal("sort result aliases caller-owned record storage")
	}
	empty := coreeval.Value{Type: listType, List: make([]coreeval.Value, 0)}
	emptyOutcome, err := coreeval.Evaluate(coreFunction, []coreeval.Value{empty})
	if err != nil || emptyOutcome.Value.List == nil || len(emptyOutcome.Value.List) != 0 {
		t.Fatalf("empty sort = %#v (%v)", emptyOutcome, err)
	}
	invalid := coreeval.Value{Type: listType, List: append([]coreeval.Value(nil), rows.List...)}
	invalid.List[1] = v280SortRow(rowType, "two", "Alpha", string([]byte{0xff}))
	if _, err := coreeval.Evaluate(coreFunction, []coreeval.Value{invalid}); err == nil {
		t.Fatal("sort accepted invalid UTF-8 in an unselected field")
	}
	if _, err := coreeval.Evaluate(coreFunction, []coreeval.Value{{Type: listType}}); err == nil {
		t.Fatal("sort accepted nil list")
	}

	malformedHIR := typed
	malformedHIR.Functions = append([]hir.Function(nil), typed.Functions...)
	malformedHIR.Functions[0].Body = typed.Functions[0].Body
	malformedHIRSort := *hirSort
	malformedHIRSort.Position = 99
	malformedHIR.Functions[0].Body.ListSortByOrdinalText = &malformedHIRSort
	if _, err := LowerHIRToCore(malformedHIR); err == nil {
		t.Fatal("Core lowering accepted out-of-range sort selector")
	}
	priorHIR := typed
	priorHIR.LanguageContract = coreir.LanguageContractV270
	if _, err := LowerHIRToCore(priorHIR); err == nil {
		t.Fatal("v0.27 HIR accepted v0.28 sort")
	}

	malformedCore := coreFunction
	malformedCore.Body = coreFunction.Body
	malformedCoreSort := *coreSort
	malformedCoreSort.Name = "Id"
	malformedCore.Body.ListSortByOrdinalText = &malformedCoreSort
	if err := coreir.ValidateFunction(malformedCore); err == nil {
		t.Fatal("Core accepted mismatched sort field identity")
	}
	priorCore := core
	priorCore.LanguageContract = coreir.LanguageContractV270
	if _, err := gobackend.Generate(priorCore); err == nil {
		t.Fatal("v0.27 Core accepted v0.28 sort")
	}

	second, err := gobackend.Generate(core)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(generated, second) {
		t.Fatal("sort generated Go is nondeterministic")
	}
	compileAndRunGeneratedGoFiles(t, generated, []byte(v280SortGeneratedGoTest()))
	assertCompilerGolden(t, "record-list-sort-by-ordinal.hir.json", canonicalJSON(t, typed))
	assertCompilerGolden(t, "record-list-sort-by-ordinal.core.json", canonicalJSON(t, core))
	assertCompilerGolden(t, "record-list-sort-by-ordinal.go", generated)
}

func TestV280SortByOrdinalParserAndExcludedForms(t *testing.T) {
	const source = `public Record Row { public string Id; public string Name; } public Class Root { public List<Row> Sort(List<Row> values) => sort_by_ordinal(values, Row.Name); }`
	sources, diagnostics := NewSourceSet([]SourceInput{{Path: "root.pipe", Data: []byte(source)}})
	if diagnostics.HasErrors() {
		t.Fatal(diagnosticError(sources, diagnostics))
	}
	program, err := parseSourceFileWithLanguageContract(sources, sources.Files()[0], PipeLangLanguageContractV280)
	if err != nil {
		t.Fatal(err)
	}
	sorted, ok := program.Classes[0].Methods[0].Body.(*ListSortByOrdinalExpr)
	if !ok || !sorted.Span.IsValid() || !sorted.RecordType.Span.IsValid() || !sorted.FieldSpan.IsValid() || sorted.Field != "Name" {
		t.Fatalf("sort AST = %#v", program.Classes[0].Methods[0].Body)
	}

	tests := []string{
		`public Record Row { public string Id; private string Name; } public Class Root { public List<Row> Sort(List<Row> values) => sort_by_ordinal(values, Row.Name); }`,
		`public Record Row { public string Id; public int Name; } public Class Root { public List<Row> Sort(List<Row> values) => sort_by_ordinal(values, Row.Name); }`,
		`public Record Row { public string Id; public string Name; } public Record Other { public string Name; } public Class Root { public List<Row> Sort(List<Row> values) => sort_by_ordinal(values, Other.Name); }`,
		`public Record Row { public string Id; public string Name; } public Class Root { public List<Row> Sort(List<Row> values) => sort_by_ordinal(values, Row.Name, Row.Id); }`,
		`public Record Row { public string Id; public string Name; } public Class Root { public int Sort(List<Row> values) => count(sort_by_ordinal(values, Row.Name)); }`,
	}
	for index, excluded := range tests {
		input := semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", "root.pipe", excluded)}, nil)
		input.LanguageContract = PipeLangLanguageContractV280
		if analysis := AnalyzeSemanticModuleSet(input); !analysis.Diagnostics.HasErrors() {
			t.Fatalf("excluded sort form %d was accepted", index)
		}
	}
	input := semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", "root.pipe", source)}, nil)
	input.LanguageContract = PipeLangLanguageContractV270
	if analysis := AnalyzeSemanticModuleSet(input); !analysis.Diagnostics.HasErrors() {
		t.Fatal("v0.27 implicitly accepted sort_by_ordinal")
	}
}

func TestV280PreservesV270JoinedFilter(t *testing.T) {
	source, err := os.ReadFile("testdata/record-list-filter-joined-contains-casefolded.pipe")
	if err != nil {
		t.Fatal(err)
	}
	input := semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", "joined.pipe", string(source))}, nil)
	input.LanguageContract = PipeLangLanguageContractV280
	analysis := AnalyzeSemanticModuleSet(input)
	if err := analysis.Error(); err != nil {
		t.Fatal(err)
	}
	typed, err := LowerSemanticMethodToHIR(analysis, semanticMethodNamed(t, analysis, "SearchRows").Identity)
	if err != nil {
		t.Fatal(err)
	}
	core, err := LowerHIRToCore(typed)
	if err != nil {
		t.Fatal(err)
	}
	if typed.Functions[0].Body.Kind != hir.ExprListFilterJoinedContainsCaseFolded || core.Functions[0].Body.Kind != coreir.ExprListFilterJoinedContainsCaseFolded {
		t.Fatalf("v0.28 changed v0.27 joined filtering: %#v / %#v", typed.Functions[0].Body, core.Functions[0].Body)
	}
	if _, err := gobackend.Generate(core); err != nil {
		t.Fatal(err)
	}
}

func v280SortByOrdinalPipeline(t *testing.T) (*Analysis, hir.Program, coreir.Program, []byte) {
	t.Helper()
	source, err := os.ReadFile("testdata/record-list-sort-by-ordinal.pipe")
	if err != nil {
		t.Fatal(err)
	}
	input := semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", "record-list-sort-by-ordinal.pipe", string(source))}, nil)
	input.LanguageContract = PipeLangLanguageContractV280
	analysis := AnalyzeSemanticModuleSet(input)
	if err := analysis.Error(); err != nil {
		t.Fatal(err)
	}
	typed, err := LowerSemanticMethodToHIR(analysis, semanticMethodNamed(t, analysis, "SortRows").Identity)
	if err != nil {
		t.Fatal(err)
	}
	core, err := LowerHIRToCore(typed)
	if err != nil {
		t.Fatal(err)
	}
	generated, err := gobackend.Generate(core)
	if err != nil {
		t.Fatal(err)
	}
	return analysis, typed, core, generated
}

func v280SortRow(rowType coreir.Type, values ...string) coreeval.Value {
	fields := make([]coreeval.Value, len(values))
	for index, value := range values {
		fields[index] = coreeval.Value{Type: rowType.Record.Fields[index].Type, String: value}
	}
	return coreeval.Value{Type: rowType, Record: fields}
}

func v280SortGeneratedGoTest() string {
	return fmt.Sprintf(`package %s

import "testing"

func expectSortPanic(t *testing.T, invoke func()) { t.Helper(); defer func() { if recover() == nil { t.Fatal("expected panic") } }(); invoke() }

func TestGeneratedOrdinalSort(t *testing.T) {
	rows := []PipeLangRecordTestPackageAppRootContainerrow{
		{Id: "one", Name: "beta", Note: "first"},
		{Id: "two", Name: "Alpha", Note: "second"},
		{Id: "three", Name: "beta", Note: "third"},
		{Id: "four", Name: "alpha", Note: "fourth"},
	}
	got := PipeLangSortRows(rows)
	want := []string{"two", "four", "one", "three"}
	for index, id := range want { if got[index].Id != id { t.Fatalf("row %%d = %%#v", index, got[index]) } }
	got[0].Id = "changed"
	if rows[1].Id != "two" { t.Fatal("generated sort aliases input") }
	empty := PipeLangSortRows([]PipeLangRecordTestPackageAppRootContainerrow{})
	if empty == nil || len(empty) != 0 { t.Fatalf("empty = %%#v", empty) }
	invalid := append([]PipeLangRecordTestPackageAppRootContainerrow(nil), rows...)
	invalid[0].Note = string([]byte{0xff})
	expectSortPanic(t, func() { PipeLangSortRows(invalid) })
	var nilRows []PipeLangRecordTestPackageAppRootContainerrow
	expectSortPanic(t, func() { PipeLangSortRows(nilRows) })
}
`, gobackend.PackageName)
}
