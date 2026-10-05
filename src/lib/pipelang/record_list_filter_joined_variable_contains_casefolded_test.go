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

func TestV290RecordListFilterJoinedVariableContainsCaseFoldedPipeline(t *testing.T) {
	analysis, typed, core, generated := v290JoinedFilterPipeline(t)
	projection, err := BuildSemanticProjection(analysis)
	if err != nil {
		t.Fatal(err)
	}
	method := projectedMemberNamed(t, projectedTypeNamed(t, projection, "Root"), "SearchRows")
	if projection.Schema != PipeLangSemanticProjectionVersion || projection.LanguageContract != PipeLangLanguageContractV290 || method.Type.Identity == nil || method.Type.Identity.Path != PipeLangListSemanticPath || len(method.Parameters) != 2 || method.Parameters[1].Type.Primitive != TypeString {
		t.Fatalf("v0.29 projection = %#v / %#v", projection, method)
	}

	hirFilter := typed.Functions[0].Body.ListFilterJoinedContainsCaseFolded
	wantNames := []string{"Name", "Driver", "Scope"}
	wantPositions := []int{1, 2, 3}
	if typed.LanguageContract != coreir.LanguageContractV290 || typed.Functions[0].Body.Kind != hir.ExprListFilterJoinedContainsCaseFolded || hirFilter == nil || hirFilter.Values == nil || hirFilter.Values.Reference == nil || hirFilter.Values.Reference.Position != 0 || hirFilter.Query == nil || hirFilter.Query.Reference == nil || hirFilter.Query.Reference.Position != 1 || len(hirFilter.Selectors) != 3 {
		t.Fatalf("v0.29 joined-filter HIR = %#v", typed)
	}
	for index, selector := range hirFilter.Selectors {
		if selector.Name != wantNames[index] || selector.Position != wantPositions[index] || selector.Field.Path != "app.root.networkrow."+lowerASCII(wantNames[index]) {
			t.Fatalf("HIR selector %d = %#v", index, selector)
		}
	}
	coreFunction := core.Functions[0]
	coreFilter := coreFunction.Body.ListFilterJoinedContainsCaseFolded
	if core.LanguageContract != coreir.LanguageContractV290 || coreFunction.Body.Kind != coreir.ExprListFilterJoinedContainsCaseFolded || coreFilter == nil || len(coreFilter.Selectors) != 3 {
		t.Fatalf("v0.29 joined-filter Core = %#v", core)
	}
	for index, selector := range coreFilter.Selectors {
		if selector.Name != wantNames[index] || selector.Position != wantPositions[index] || selector.Field.Path != "app.root.networkrow."+lowerASCII(wantNames[index]) {
			t.Fatalf("Core selector %d = %#v", index, selector)
		}
	}

	listType := coreFunction.Parameters[0].Type
	rowType := listType.List.Element
	rows := coreeval.Value{Type: listType, List: []coreeval.Value{
		v290NetworkRow(rowType, "one", "frontend", "bridge", "local", "selected first"),
		v290NetworkRow(rowType, "two", "Straße", "overlay", "global", "selected second"),
		v290NetworkRow(rowType, "three", "backend", "host", "local", "selected third"),
	}}
	text := func(value string) coreeval.Value {
		return coreeval.Value{Type: coreir.Type{Kind: coreir.TypePrimitive, Primitive: coreir.PrimitiveString}, String: value}
	}
	for _, test := range []struct {
		query string
		ids   []string
	}{
		{query: "FRONTEND BRIDGE", ids: []string{"one"}},
		{query: "STRASSE OVERLAY", ids: []string{"two"}},
		{query: "\u3000host local\u00a0", ids: []string{"three"}},
		{query: " \t\u3000", ids: []string{"one", "two", "three"}},
		{query: "missing", ids: nil},
	} {
		outcome, err := coreeval.Evaluate(coreFunction, []coreeval.Value{rows, text(test.query)})
		if err != nil || !outcome.OK || outcome.Value.List == nil || len(outcome.Value.List) != len(test.ids) {
			t.Fatalf("query %q = %#v (%v)", test.query, outcome, err)
		}
		for index, id := range test.ids {
			if outcome.Value.List[index].Record[0].String != id {
				t.Fatalf("query %q result %d = %#v", test.query, index, outcome.Value.List[index])
			}
		}
	}
	outcome, err := coreeval.Evaluate(coreFunction, []coreeval.Value{rows, text("frontend")})
	if err != nil {
		t.Fatal(err)
	}
	rows.List[0].Record[0].String = "mutated"
	if outcome.Value.List[0].Record[0].String != "one" {
		t.Fatal("variable joined filter aliases caller-owned record storage")
	}

	invalid := rows
	invalid.List = append([]coreeval.Value(nil), rows.List...)
	invalid.List[2] = v290NetworkRow(rowType, "three", "backend", "host", "local", string([]byte{0xff}))
	if _, err := coreeval.Evaluate(coreFunction, []coreeval.Value{invalid, text("frontend")}); err == nil {
		t.Fatal("variable joined filter accepted invalid UTF-8 in an unselected unmatched field")
	}
	if _, err := coreeval.Evaluate(coreFunction, []coreeval.Value{rows, text(string([]byte{0xff}))}); err == nil {
		t.Fatal("variable joined filter accepted invalid UTF-8 query")
	}
	if _, err := coreeval.Evaluate(coreFunction, []coreeval.Value{{Type: listType}, text("frontend")}); err == nil {
		t.Fatal("variable joined filter accepted nil list")
	}

	malformedHIR := typed
	malformedHIR.Functions = append([]hir.Function(nil), typed.Functions...)
	malformedHIR.Functions[0].Body = typed.Functions[0].Body
	malformedHIRFilter := *hirFilter
	malformedHIRFilter.Selectors = append([]hir.ListTextFieldSelector(nil), hirFilter.Selectors[:1]...)
	malformedHIR.Functions[0].Body.ListFilterJoinedContainsCaseFolded = &malformedHIRFilter
	if _, err := LowerHIRToCore(malformedHIR); err == nil {
		t.Fatal("Core lowering accepted one joined selector")
	}
	priorHIR := typed
	priorHIR.LanguageContract = coreir.LanguageContractV280
	if _, err := LowerHIRToCore(priorHIR); err == nil {
		t.Fatal("v0.28 HIR accepted variable-count joined filter")
	}

	malformedCore := coreFunction
	malformedCore.Body = coreFunction.Body
	malformedCoreFilter := *coreFilter
	malformedCoreFilter.Selectors = append([]coreir.ListTextFieldSelector(nil), coreFilter.Selectors...)
	malformedCoreFilter.Selectors[1] = malformedCoreFilter.Selectors[0]
	malformedCore.Body.ListFilterJoinedContainsCaseFolded = &malformedCoreFilter
	if err := coreir.ValidateFunction(malformedCore); err == nil {
		t.Fatal("Core accepted duplicate joined selectors")
	}
	priorCore := core
	priorCore.LanguageContract = coreir.LanguageContractV280
	if _, err := gobackend.Generate(priorCore); err == nil {
		t.Fatal("v0.28 Core accepted variable-count joined filter")
	}

	second, err := gobackend.Generate(core)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(generated, second) {
		t.Fatal("variable joined filter generated Go is nondeterministic")
	}
	compileAndRunGeneratedGoFiles(t, generated, []byte(v290JoinedFilterGeneratedGoTest()))
	assertCompilerGolden(t, "record-list-filter-joined-variable-contains-casefolded.hir.json", canonicalJSON(t, typed))
	assertCompilerGolden(t, "record-list-filter-joined-variable-contains-casefolded.core.json", canonicalJSON(t, core))
	assertCompilerGolden(t, "record-list-filter-joined-variable-contains-casefolded.go", generated)
}

func TestV290JoinedFilterParserAndExcludedForms(t *testing.T) {
	const source = `public Record Row { public string A; public string B; public string C; } public Class Root { public List<Row> Search(List<Row> values, string query) => filter_joined_contains_casefolded(values, Row.A, Row.B, Row.C, query); }`
	sources, diagnostics := NewSourceSet([]SourceInput{{Path: "root.pipe", Data: []byte(source)}})
	if diagnostics.HasErrors() {
		t.Fatal(diagnosticError(sources, diagnostics))
	}
	program, err := parseSourceFileWithLanguageContract(sources, sources.Files()[0], PipeLangLanguageContractV290)
	if err != nil {
		t.Fatal(err)
	}
	filter, ok := program.Classes[0].Methods[0].Body.(*ListFilterJoinedContainsCaseFoldedExpr)
	if !ok || !filter.Span.IsValid() || len(filter.Selectors) != 3 {
		t.Fatalf("variable joined filter AST = %#v", program.Classes[0].Methods[0].Body)
	}
	for index, selector := range filter.Selectors {
		if selector.Field != string(rune('A'+index)) || !selector.FieldSpan.IsValid() || !selector.RecordType.Span.IsValid() {
			t.Fatalf("selector %d = %#v", index, selector)
		}
	}

	tests := []string{
		`public Record Row { public string A; public string B; } public Class Root { public List<Row> Search(List<Row> values, string query) => filter_joined_contains_casefolded(values, Row.A, query); }`,
		`public Record Row { public string A; public string B; } public Class Root { public List<Row> Search(List<Row> values, string query) => filter_joined_contains_casefolded(values, Row.A, Row.A, query); }`,
		`public Record Row { public string A; private string B; } public Class Root { public List<Row> Search(List<Row> values, string query) => filter_joined_contains_casefolded(values, Row.A, Row.B, query); }`,
		`public Record Row { public string A; public int B; } public Class Root { public List<Row> Search(List<Row> values, string query) => filter_joined_contains_casefolded(values, Row.A, Row.B, query); }`,
		`public Record Row { public string A; public string B; } public Record Other { public string B; } public Class Root { public List<Row> Search(List<Row> values, string query) => filter_joined_contains_casefolded(values, Row.A, Other.B, query); }`,
		`public Record Row { public string A; public string B; } public Class Root { public List<Row> Search(List<Row> values, string query) => filter_joined_contains_casefolded(values, Row.A, Row.B, "query"); }`,
		`public Record Row { public string A; public string B; } public Class Root { public int Search(List<Row> values, string query) => count(filter_joined_contains_casefolded(values, Row.A, Row.B, query)); }`,
	}
	for index, excluded := range tests {
		input := semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", "root.pipe", excluded)}, nil)
		input.LanguageContract = PipeLangLanguageContractV290
		if analysis := AnalyzeSemanticModuleSet(input); !analysis.Diagnostics.HasErrors() {
			t.Fatalf("excluded variable joined-filter form %d was accepted", index)
		}
	}
	input := semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", "root.pipe", source)}, nil)
	input.LanguageContract = PipeLangLanguageContractV280
	if analysis := AnalyzeSemanticModuleSet(input); !analysis.Diagnostics.HasErrors() {
		t.Fatal("v0.28 implicitly accepted variable-count filter_joined_contains_casefolded")
	}
}

func TestV290PreservesV280SortAndV270FiveFieldFilter(t *testing.T) {
	for _, fixture := range []struct {
		path   string
		method string
		kind   hir.ExprKind
	}{
		{path: "testdata/record-list-sort-by-ordinal.pipe", method: "SortRows", kind: hir.ExprListSortByOrdinalText},
		{path: "testdata/record-list-filter-joined-contains-casefolded.pipe", method: "SearchRows", kind: hir.ExprListFilterJoinedContainsCaseFolded},
	} {
		source, err := os.ReadFile(fixture.path)
		if err != nil {
			t.Fatal(err)
		}
		input := semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", fixture.path, string(source))}, nil)
		input.LanguageContract = PipeLangLanguageContractV290
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
			t.Fatalf("v0.29 changed preserved HIR kind for %s: %#v", fixture.path, typed.Functions[0].Body)
		}
		if _, err := gobackend.Generate(core); err != nil {
			t.Fatal(err)
		}
	}
}

func v290JoinedFilterPipeline(t *testing.T) (*Analysis, hir.Program, coreir.Program, []byte) {
	t.Helper()
	source, err := os.ReadFile("testdata/record-list-filter-joined-variable-contains-casefolded.pipe")
	if err != nil {
		t.Fatal(err)
	}
	input := semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", "record-list-filter-joined-variable-contains-casefolded.pipe", string(source))}, nil)
	input.LanguageContract = PipeLangLanguageContractV290
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
	generated, err := gobackend.Generate(core)
	if err != nil {
		t.Fatal(err)
	}
	return analysis, typed, core, generated
}

func v290NetworkRow(rowType coreir.Type, values ...string) coreeval.Value {
	fields := make([]coreeval.Value, len(values))
	for index, value := range values {
		fields[index] = coreeval.Value{Type: rowType.Record.Fields[index].Type, String: value}
	}
	return coreeval.Value{Type: rowType, Record: fields}
}

func v290JoinedFilterGeneratedGoTest() string {
	return fmt.Sprintf(`package %s

import "testing"

func expectVariableJoinedPanic(t *testing.T, invoke func()) { t.Helper(); defer func() { if recover() == nil { t.Fatal("expected panic") } }(); invoke() }

func TestGeneratedVariableJoinedFilter(t *testing.T) {
	rows := []PipeLangRecordTestPackageAppRootNetworkrow{
		{Id: "one", Name: "frontend", Driver: "bridge", Scope: "local", Note: "first"},
		{Id: "two", Name: "Straße", Driver: "overlay", Scope: "global", Note: "second"},
	}
	if got := PipeLangSearchRows(rows, "\u3000STRASSE OVERLAY\u00a0"); len(got) != 1 || got[0].Id != "two" { t.Fatalf("joined match = %%#v", got) }
	all := PipeLangSearchRows(rows, " \t")
	if len(all) != 2 { t.Fatalf("empty trimmed query = %%#v", all) }
	all[0].Id = "changed"
	if rows[0].Id != "one" { t.Fatal("generated result aliases input") }
	if missing := PipeLangSearchRows(rows, "missing"); missing == nil || len(missing) != 0 { t.Fatalf("missing = %%#v", missing) }
	invalid := append([]PipeLangRecordTestPackageAppRootNetworkrow(nil), rows...)
	invalid[1].Note = string([]byte{0xff})
	expectVariableJoinedPanic(t, func() { PipeLangSearchRows(invalid, "frontend") })
	expectVariableJoinedPanic(t, func() { PipeLangSearchRows(rows, string([]byte{0xff})) })
	var nilRows []PipeLangRecordTestPackageAppRootNetworkrow
	expectVariableJoinedPanic(t, func() { PipeLangSearchRows(nilRows, "frontend") })
}
`, gobackend.PackageName)
}
