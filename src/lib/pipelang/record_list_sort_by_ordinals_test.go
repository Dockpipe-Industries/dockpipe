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

func TestV300RecordListSortByOrdinalsPipeline(t *testing.T) {
	analysis, typed, core, generated := v300SortByOrdinalsPipeline(t)
	projection, err := BuildSemanticProjection(analysis)
	if err != nil {
		t.Fatal(err)
	}
	method := projectedMemberNamed(t, projectedTypeNamed(t, projection, "Root"), "SortRows")
	if projection.Schema != PipeLangSemanticProjectionVersion || projection.LanguageContract != PipeLangLanguageContractV300 || method.Type.Identity == nil || method.Type.Identity.Path != PipeLangListSemanticPath || len(method.Parameters) != 1 || method.Parameters[0].Type.Identity == nil || method.Parameters[0].Type.Identity.Path != PipeLangListSemanticPath {
		t.Fatalf("v0.30 projection = %#v / %#v", projection, method)
	}

	hirSort := typed.Functions[0].Body.ListSortByOrdinalTexts
	wantNames := []string{"State", "Name"}
	wantPositions := []int{1, 2}
	if typed.LanguageContract != coreir.LanguageContractV300 || typed.Functions[0].Body.Kind != hir.ExprListSortByOrdinalTexts || hirSort == nil || hirSort.Values == nil || hirSort.Values.Reference == nil || hirSort.Values.Reference.Position != 0 || len(hirSort.Selectors) != 2 {
		t.Fatalf("v0.30 multi-key sort HIR = %#v", typed)
	}
	for index, selector := range hirSort.Selectors {
		if selector.Name != wantNames[index] || selector.Position != wantPositions[index] || selector.Field.Path != "app.root.containerrow."+lowerASCII(wantNames[index]) {
			t.Fatalf("HIR selector %d = %#v", index, selector)
		}
	}
	coreFunction := core.Functions[0]
	coreSort := coreFunction.Body.ListSortByOrdinalTexts
	if core.LanguageContract != coreir.LanguageContractV300 || coreFunction.Body.Kind != coreir.ExprListSortByOrdinalTexts || coreSort == nil || len(coreSort.Selectors) != 2 {
		t.Fatalf("v0.30 multi-key sort Core = %#v", core)
	}
	for index, selector := range coreSort.Selectors {
		if selector.Name != wantNames[index] || selector.Position != wantPositions[index] || selector.Field.Path != "app.root.containerrow."+lowerASCII(wantNames[index]) {
			t.Fatalf("Core selector %d = %#v", index, selector)
		}
	}

	listType := coreFunction.Parameters[0].Type
	rowType := listType.List.Element
	rows := coreeval.Value{Type: listType, List: []coreeval.Value{
		v300SortRow(rowType, "one", "running", "beta", "first"),
		v300SortRow(rowType, "two", "exited", "zeta", "second"),
		v300SortRow(rowType, "three", "running", "Alpha", "third"),
		v300SortRow(rowType, "four", "running", "alpha", "fourth"),
		v300SortRow(rowType, "five", "exited", "alpha", "fifth"),
		v300SortRow(rowType, "six", "running", "Alpha", "sixth"),
	}}
	outcome, err := coreeval.Evaluate(coreFunction, []coreeval.Value{rows})
	if err != nil || !outcome.OK || outcome.Value.List == nil {
		t.Fatalf("multi-key sort outcome = %#v (%v)", outcome, err)
	}
	want := []string{"five", "two", "three", "six", "four", "one"}
	for index, id := range want {
		if outcome.Value.List[index].Record[0].String != id {
			t.Fatalf("multi-key sorted row %d = %#v", index, outcome.Value.List[index])
		}
	}
	rows.List[0].Record[0].String = "mutated"
	if outcome.Value.List[5].Record[0].String != "one" {
		t.Fatal("multi-key sort result aliases caller-owned record storage")
	}
	empty := coreeval.Value{Type: listType, List: make([]coreeval.Value, 0)}
	emptyOutcome, err := coreeval.Evaluate(coreFunction, []coreeval.Value{empty})
	if err != nil || emptyOutcome.Value.List == nil || len(emptyOutcome.Value.List) != 0 {
		t.Fatalf("empty multi-key sort = %#v (%v)", emptyOutcome, err)
	}
	invalid := coreeval.Value{Type: listType, List: append([]coreeval.Value(nil), rows.List...)}
	invalid.List[1] = v300SortRow(rowType, "two", "exited", "zeta", string([]byte{0xff}))
	if _, err := coreeval.Evaluate(coreFunction, []coreeval.Value{invalid}); err == nil {
		t.Fatal("multi-key sort accepted invalid UTF-8 in an unselected field")
	}
	if _, err := coreeval.Evaluate(coreFunction, []coreeval.Value{{Type: listType}}); err == nil {
		t.Fatal("multi-key sort accepted nil list")
	}

	malformedHIR := typed
	malformedHIR.Functions = append([]hir.Function(nil), typed.Functions...)
	malformedHIR.Functions[0].Body = typed.Functions[0].Body
	malformedHIRSort := *hirSort
	malformedHIRSort.Selectors = append([]hir.ListTextFieldSelector(nil), hirSort.Selectors[:1]...)
	malformedHIR.Functions[0].Body.ListSortByOrdinalTexts = &malformedHIRSort
	if _, err := LowerHIRToCore(malformedHIR); err == nil {
		t.Fatal("Core lowering accepted one selector in multi-key sort HIR")
	}
	priorHIR := typed
	priorHIR.LanguageContract = coreir.LanguageContractV290
	if _, err := LowerHIRToCore(priorHIR); err == nil {
		t.Fatal("v0.29 HIR accepted multi-key sort")
	}

	malformedCore := coreFunction
	malformedCore.Body = coreFunction.Body
	malformedCoreSort := *coreSort
	malformedCoreSort.Selectors = append([]coreir.ListTextFieldSelector(nil), coreSort.Selectors...)
	malformedCoreSort.Selectors[1] = malformedCoreSort.Selectors[0]
	malformedCore.Body.ListSortByOrdinalTexts = &malformedCoreSort
	if err := coreir.ValidateFunction(malformedCore); err == nil {
		t.Fatal("Core accepted duplicate multi-key sort selectors")
	}
	priorCore := core
	priorCore.LanguageContract = coreir.LanguageContractV290
	if _, err := gobackend.Generate(priorCore); err == nil {
		t.Fatal("v0.29 Core accepted multi-key sort")
	}

	second, err := gobackend.Generate(core)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(generated, second) {
		t.Fatal("multi-key sort generated Go is nondeterministic")
	}
	compileAndRunGeneratedGoFiles(t, generated, []byte(v300SortGeneratedGoTest()))
	assertCompilerGolden(t, "record-list-sort-by-ordinals.hir.json", canonicalJSON(t, typed))
	assertCompilerGolden(t, "record-list-sort-by-ordinals.core.json", canonicalJSON(t, core))
	assertCompilerGolden(t, "record-list-sort-by-ordinals.go", generated)
}

func TestV300SortByOrdinalsParserAndExcludedForms(t *testing.T) {
	const source = `public Record Row { public string Id; public string State; public string Name; } public Class Root { public List<Row> Sort(List<Row> values) => sort_by_ordinal(values, Row.State, Row.Name); }`
	sources, diagnostics := NewSourceSet([]SourceInput{{Path: "root.pipe", Data: []byte(source)}})
	if diagnostics.HasErrors() {
		t.Fatal(diagnosticError(sources, diagnostics))
	}
	program, err := parseSourceFileWithLanguageContract(sources, sources.Files()[0], PipeLangLanguageContractV300)
	if err != nil {
		t.Fatal(err)
	}
	sorted, ok := program.Classes[0].Methods[0].Body.(*ListSortByOrdinalsExpr)
	if !ok || !sorted.Span.IsValid() || len(sorted.Selectors) != 2 {
		t.Fatalf("multi-key sort AST = %#v", program.Classes[0].Methods[0].Body)
	}
	for index, selector := range sorted.Selectors {
		if !selector.FieldSpan.IsValid() || !selector.RecordType.Span.IsValid() || selector.Field != []string{"State", "Name"}[index] {
			t.Fatalf("selector %d = %#v", index, selector)
		}
	}

	tests := []string{
		`public Record Row { public string A; public string B; } public Class Root { public List<Row> Sort(List<Row> values) => sort_by_ordinal(values, Row.A, Row.A); }`,
		`public Record Row { public string A; private string B; } public Class Root { public List<Row> Sort(List<Row> values) => sort_by_ordinal(values, Row.A, Row.B); }`,
		`public Record Row { public string A; public int B; } public Class Root { public List<Row> Sort(List<Row> values) => sort_by_ordinal(values, Row.A, Row.B); }`,
		`public Record Row { public string A; public string B; } public Record Other { public string B; } public Class Root { public List<Row> Sort(List<Row> values) => sort_by_ordinal(values, Row.A, Other.B); }`,
		`public Record Row { public string A; public string B; } public Class Root { public int Sort(List<Row> values) => count(sort_by_ordinal(values, Row.A, Row.B)); }`,
	}
	for index, excluded := range tests {
		input := semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", "root.pipe", excluded)}, nil)
		input.LanguageContract = PipeLangLanguageContractV300
		if analysis := AnalyzeSemanticModuleSet(input); !analysis.Diagnostics.HasErrors() {
			t.Fatalf("excluded multi-key sort form %d was accepted", index)
		}
	}
	input := semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", "root.pipe", source)}, nil)
	input.LanguageContract = PipeLangLanguageContractV290
	if analysis := AnalyzeSemanticModuleSet(input); !analysis.Diagnostics.HasErrors() {
		t.Fatal("v0.29 implicitly accepted multi-key sort_by_ordinal")
	}
}

func TestV300PreservesV290VariableFilterAndV280OneKeySort(t *testing.T) {
	for _, fixture := range []struct {
		path   string
		method string
		kind   hir.ExprKind
	}{
		{path: "testdata/record-list-filter-joined-variable-contains-casefolded.pipe", method: "SearchRows", kind: hir.ExprListFilterJoinedContainsCaseFolded},
		{path: "testdata/record-list-sort-by-ordinal.pipe", method: "SortRows", kind: hir.ExprListSortByOrdinalText},
	} {
		source, err := os.ReadFile(fixture.path)
		if err != nil {
			t.Fatal(err)
		}
		input := semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", fixture.path, string(source))}, nil)
		input.LanguageContract = PipeLangLanguageContractV300
		analysis := AnalyzeSemanticModuleSet(input)
		if err := analysis.Error(); err != nil {
			t.Fatal(err)
		}
		typed, err := LowerSemanticMethodToHIR(analysis, semanticMethodNamed(t, analysis, fixture.method).Identity)
		if err != nil {
			t.Fatal(err)
		}
		core, err := LowerHIRToCore(typed)
		if err != nil {
			t.Fatal(err)
		}
		if typed.Functions[0].Body.Kind != fixture.kind {
			t.Fatalf("v0.30 changed preserved HIR kind for %s: %#v", fixture.path, typed.Functions[0].Body)
		}
		if _, err := gobackend.Generate(core); err != nil {
			t.Fatal(err)
		}
	}
}

func v300SortByOrdinalsPipeline(t *testing.T) (*Analysis, hir.Program, coreir.Program, []byte) {
	t.Helper()
	source, err := os.ReadFile("testdata/record-list-sort-by-ordinals.pipe")
	if err != nil {
		t.Fatal(err)
	}
	input := semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", "record-list-sort-by-ordinals.pipe", string(source))}, nil)
	input.LanguageContract = PipeLangLanguageContractV300
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

func v300SortRow(rowType coreir.Type, values ...string) coreeval.Value {
	fields := make([]coreeval.Value, len(values))
	for index, value := range values {
		fields[index] = coreeval.Value{Type: rowType.Record.Fields[index].Type, String: value}
	}
	return coreeval.Value{Type: rowType, Record: fields}
}

func v300SortGeneratedGoTest() string {
	return fmt.Sprintf(`package %s

import "testing"

func expectMultiKeySortPanic(t *testing.T, invoke func()) { t.Helper(); defer func() { if recover() == nil { t.Fatal("expected panic") } }(); invoke() }

func TestGeneratedMultiKeyOrdinalSort(t *testing.T) {
	rows := []PipeLangRecordTestPackageAppRootContainerrow{
		{Id: "one", State: "running", Name: "beta", Note: "first"},
		{Id: "two", State: "exited", Name: "zeta", Note: "second"},
		{Id: "three", State: "running", Name: "Alpha", Note: "third"},
		{Id: "four", State: "running", Name: "alpha", Note: "fourth"},
		{Id: "five", State: "exited", Name: "alpha", Note: "fifth"},
		{Id: "six", State: "running", Name: "Alpha", Note: "sixth"},
	}
	got := PipeLangSortRows(rows)
	want := []string{"five", "two", "three", "six", "four", "one"}
	for index, id := range want { if got[index].Id != id { t.Fatalf("row %%d = %%#v", index, got[index]) } }
	got[0].Id = "changed"
	if rows[4].Id != "five" { t.Fatal("generated multi-key sort aliases input") }
	empty := PipeLangSortRows([]PipeLangRecordTestPackageAppRootContainerrow{})
	if empty == nil || len(empty) != 0 { t.Fatalf("empty = %%#v", empty) }
	invalid := append([]PipeLangRecordTestPackageAppRootContainerrow(nil), rows...)
	invalid[0].Note = string([]byte{0xff})
	expectMultiKeySortPanic(t, func() { PipeLangSortRows(invalid) })
	var nilRows []PipeLangRecordTestPackageAppRootContainerrow
	expectMultiKeySortPanic(t, func() { PipeLangSortRows(nilRows) })
}

`, gobackend.PackageName)
}
