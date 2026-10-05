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

func TestV270RecordListFilterJoinedContainsCaseFoldedPipeline(t *testing.T) {
	analysis, typed, core, generated := v270JoinedFilterPipeline(t)
	projection, err := BuildSemanticProjection(analysis)
	if err != nil {
		t.Fatal(err)
	}
	method := projectedMemberNamed(t, projectedTypeNamed(t, projection, "Root"), "SearchRows")
	if projection.Schema != PipeLangSemanticProjectionVersion || projection.LanguageContract != PipeLangLanguageContractV270 || method.Type.Identity == nil || method.Type.Identity.Path != PipeLangListSemanticPath || len(method.Parameters) != 2 || method.Parameters[1].Type.Primitive != TypeString {
		t.Fatalf("v0.27 projection = %#v / %#v", projection, method)
	}

	hirFilter := typed.Functions[0].Body.ListFilterJoinedContainsCaseFolded
	wantNames := []string{"Name", "State", "Image", "Ports", "Created"}
	wantPositions := []int{1, 2, 3, 4, 5}
	if typed.LanguageContract != coreir.LanguageContractV270 || typed.Functions[0].Body.Kind != hir.ExprListFilterJoinedContainsCaseFolded || hirFilter == nil || hirFilter.Values == nil || hirFilter.Values.Reference == nil || hirFilter.Values.Reference.Position != 0 || hirFilter.Query == nil || hirFilter.Query.Reference == nil || hirFilter.Query.Reference.Position != 1 || len(hirFilter.Selectors) != 5 {
		t.Fatalf("v0.27 joined-filter HIR = %#v", typed)
	}
	for index, selector := range hirFilter.Selectors {
		if selector.Name != wantNames[index] || selector.Position != wantPositions[index] || selector.Field.Path != "app.root.containerrow."+lowerASCII(wantNames[index]) {
			t.Fatalf("HIR selector %d = %#v", index, selector)
		}
	}
	coreFunction := core.Functions[0]
	coreFilter := coreFunction.Body.ListFilterJoinedContainsCaseFolded
	if core.LanguageContract != coreir.LanguageContractV270 || coreFunction.Body.Kind != coreir.ExprListFilterJoinedContainsCaseFolded || coreFilter == nil || len(coreFilter.Selectors) != 5 {
		t.Fatalf("v0.27 joined-filter Core = %#v", core)
	}
	for index, selector := range coreFilter.Selectors {
		if selector.Name != wantNames[index] || selector.Position != wantPositions[index] || selector.Field.Path != "app.root.containerrow."+lowerASCII(wantNames[index]) {
			t.Fatalf("Core selector %d = %#v", index, selector)
		}
	}

	listType := coreFunction.Parameters[0].Type
	rowType := listType.List.Element
	rows := coreeval.Value{Type: listType, List: []coreeval.Value{
		v270ContainerRow(rowType, "one", "api", "running", "nginx:latest", "80:80", "2 hours"),
		v270ContainerRow(rowType, "two", "worker", "exited", "Straße/Image", "", "yesterday"),
		v270ContainerRow(rowType, "three", "db", "healthy", "postgres", "5432", "today"),
	}}
	text := func(value string) coreeval.Value {
		return coreeval.Value{Type: coreir.Type{Kind: coreir.TypePrimitive, Primitive: coreir.PrimitiveString}, String: value}
	}
	for _, test := range []struct {
		query string
		ids   []string
	}{
		{query: "API RUNNING", ids: []string{"one"}},
		{query: "running nginx", ids: []string{"one"}},
		{query: "STRASSE", ids: []string{"two"}},
		{query: "\u3000healthy postgres\u00a0", ids: []string{"three"}},
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
	outcome, err := coreeval.Evaluate(coreFunction, []coreeval.Value{rows, text("api")})
	if err != nil {
		t.Fatal(err)
	}
	rows.List[0].Record[0].String = "mutated"
	if outcome.Value.List[0].Record[0].String != "one" {
		t.Fatal("joined filter aliases caller-owned record storage")
	}

	invalid := rows
	invalid.List = append([]coreeval.Value(nil), rows.List...)
	invalid.List[2] = v270ContainerRow(rowType, string([]byte{0xff}), "db", "healthy", "postgres", "5432", "today")
	if _, err := coreeval.Evaluate(coreFunction, []coreeval.Value{invalid, text("api")}); err == nil {
		t.Fatal("joined filter accepted invalid UTF-8 in an unselected unmatched field")
	}
	if _, err := coreeval.Evaluate(coreFunction, []coreeval.Value{rows, text(string([]byte{0xff}))}); err == nil {
		t.Fatal("joined filter accepted invalid UTF-8 query")
	}
	if _, err := coreeval.Evaluate(coreFunction, []coreeval.Value{{Type: listType}, text("api")}); err == nil {
		t.Fatal("joined filter accepted nil list")
	}

	malformedHIR := typed
	malformedHIR.Functions = append([]hir.Function(nil), typed.Functions...)
	malformedHIR.Functions[0].Body = typed.Functions[0].Body
	malformedHIRFilter := *hirFilter
	malformedHIRFilter.Selectors = append([]hir.ListTextFieldSelector(nil), hirFilter.Selectors[:4]...)
	malformedHIR.Functions[0].Body.ListFilterJoinedContainsCaseFolded = &malformedHIRFilter
	if _, err := LowerHIRToCore(malformedHIR); err == nil {
		t.Fatal("Core lowering accepted four joined selectors")
	}
	priorHIR := typed
	priorHIR.LanguageContract = coreir.LanguageContractV260
	if _, err := LowerHIRToCore(priorHIR); err == nil {
		t.Fatal("v0.26 HIR accepted v0.27 joined filter")
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
	priorCore.LanguageContract = coreir.LanguageContractV260
	if _, err := gobackend.Generate(priorCore); err == nil {
		t.Fatal("v0.26 Core accepted v0.27 joined filter")
	}

	second, err := gobackend.Generate(core)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(generated, second) {
		t.Fatal("joined filter generated Go is nondeterministic")
	}
	compileAndRunGeneratedGoFiles(t, generated, []byte(v270JoinedFilterGeneratedGoTest()))
	assertCompilerGolden(t, "record-list-filter-joined-contains-casefolded.hir.json", canonicalJSON(t, typed))
	assertCompilerGolden(t, "record-list-filter-joined-contains-casefolded.core.json", canonicalJSON(t, core))
	assertCompilerGolden(t, "record-list-filter-joined-contains-casefolded.go", generated)
}

func TestV270JoinedFilterParserAndExcludedForms(t *testing.T) {
	const source = `public Record Row { public string A; public string B; public string C; public string D; public string E; } public Class Root { public List<Row> Search(List<Row> values, string query) => filter_joined_contains_casefolded(values, Row.A, Row.B, Row.C, Row.D, Row.E, query); }`
	sources, diagnostics := NewSourceSet([]SourceInput{{Path: "root.pipe", Data: []byte(source)}})
	if diagnostics.HasErrors() {
		t.Fatal(diagnosticError(sources, diagnostics))
	}
	program, err := parseSourceFileWithLanguageContract(sources, sources.Files()[0], PipeLangLanguageContractV270)
	if err != nil {
		t.Fatal(err)
	}
	filter, ok := program.Classes[0].Methods[0].Body.(*ListFilterJoinedContainsCaseFoldedExpr)
	if !ok || !filter.Span.IsValid() || len(filter.Selectors) != 5 {
		t.Fatalf("joined filter AST = %#v", program.Classes[0].Methods[0].Body)
	}
	for index, selector := range filter.Selectors {
		if selector.Field != string(rune('A'+index)) || !selector.FieldSpan.IsValid() || !selector.RecordType.Span.IsValid() {
			t.Fatalf("selector %d = %#v", index, selector)
		}
	}

	tests := []string{
		`public Record Row { public string A; public string B; public string C; public string D; public string E; } public Class Root { public List<Row> Search(List<Row> values, string query) => filter_joined_contains_casefolded(values, Row.A, Row.B, Row.C, Row.D, query, query); }`,
		`public Record Row { public string A; public string B; public string C; public string D; public string E; } public Class Root { public List<Row> Search(List<Row> values, string query) => filter_joined_contains_casefolded(values, Row.A, Row.B, Row.C, Row.D, Row.A, query); }`,
		`public Record Row { public string A; public string B; public string C; public string D; public int E; } public Class Root { public List<Row> Search(List<Row> values, string query) => filter_joined_contains_casefolded(values, Row.A, Row.B, Row.C, Row.D, Row.E, query); }`,
		`public Record Row { public string A; public string B; public string C; public string D; public string E; } public Class Root { public int Search(List<Row> values, string query) => count(filter_joined_contains_casefolded(values, Row.A, Row.B, Row.C, Row.D, Row.E, query)); }`,
	}
	for index, source := range tests {
		input := semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", "root.pipe", source)}, nil)
		input.LanguageContract = PipeLangLanguageContractV270
		if analysis := AnalyzeSemanticModuleSet(input); !analysis.Diagnostics.HasErrors() {
			t.Fatalf("excluded joined-filter form %d was accepted", index)
		}
	}
	input := semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", "root.pipe", source)}, nil)
	input.LanguageContract = PipeLangLanguageContractV260
	if analysis := AnalyzeSemanticModuleSet(input); !analysis.Diagnostics.HasErrors() {
		t.Fatal("v0.26 implicitly accepted filter_joined_contains_casefolded")
	}
}

func TestV270PreservesV260TextTrim(t *testing.T) {
	source, err := os.ReadFile("testdata/text-trim.pipe")
	if err != nil {
		t.Fatal(err)
	}
	input := semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", "text-trim.pipe", string(source))}, nil)
	input.LanguageContract = PipeLangLanguageContractV270
	analysis := AnalyzeSemanticModuleSet(input)
	if err := analysis.Error(); err != nil {
		t.Fatal(err)
	}
	typed, err := LowerSemanticMethodToHIR(analysis, semanticMethodNamed(t, analysis, "Trim").Identity)
	if err != nil {
		t.Fatal(err)
	}
	core, err := LowerHIRToCore(typed)
	if err != nil {
		t.Fatal(err)
	}
	if typed.Functions[0].Body.Kind != hir.ExprTextTrim || core.Functions[0].Body.Kind != coreir.ExprTextTrim {
		t.Fatalf("v0.27 changed v0.26 trim lowering: %#v / %#v", typed.Functions[0].Body, core.Functions[0].Body)
	}
	if _, err := gobackend.Generate(core); err != nil {
		t.Fatal(err)
	}
}

func v270JoinedFilterPipeline(t *testing.T) (*Analysis, hir.Program, coreir.Program, []byte) {
	t.Helper()
	source, err := os.ReadFile("testdata/record-list-filter-joined-contains-casefolded.pipe")
	if err != nil {
		t.Fatal(err)
	}
	input := semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", "record-list-filter-joined-contains-casefolded.pipe", string(source))}, nil)
	input.LanguageContract = PipeLangLanguageContractV270
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

func v270ContainerRow(rowType coreir.Type, values ...string) coreeval.Value {
	fields := make([]coreeval.Value, len(values))
	for index, value := range values {
		fields[index] = coreeval.Value{Type: rowType.Record.Fields[index].Type, String: value}
	}
	return coreeval.Value{Type: rowType, Record: fields}
}

func lowerASCII(value string) string {
	if value == "" {
		return value
	}
	return string(value[0]+('a'-'A')) + value[1:]
}

func v270JoinedFilterGeneratedGoTest() string {
	return fmt.Sprintf(`package %s

import "testing"

func expectJoinedPanic(t *testing.T, invoke func()) { t.Helper(); defer func() { if recover() == nil { t.Fatal("expected panic") } }(); invoke() }

func TestGeneratedJoinedFilter(t *testing.T) {
	rows := []PipeLangRecordTestPackageAppRootContainerrow{
		{Id: "one", Name: "api", State: "running", Image: "nginx:latest", Ports: "80:80", Created: "2 hours"},
		{Id: "two", Name: "worker", State: "exited", Image: "Straße/Image", Created: "yesterday"},
	}
	if got := PipeLangSearchRows(rows, "\u3000RUNNING NGINX\u00a0"); len(got) != 1 || got[0].Id != "one" { t.Fatalf("joined match = %%#v", got) }
	all := PipeLangSearchRows(rows, " \t")
	if len(all) != 2 { t.Fatalf("empty trimmed query = %%#v", all) }
	all[0].Id = "changed"
	if rows[0].Id != "one" { t.Fatal("generated result aliases input") }
	if missing := PipeLangSearchRows(rows, "missing"); missing == nil || len(missing) != 0 { t.Fatalf("missing = %%#v", missing) }
	invalid := append([]PipeLangRecordTestPackageAppRootContainerrow(nil), rows...)
	invalid[1].Id = string([]byte{0xff})
	expectJoinedPanic(t, func() { PipeLangSearchRows(invalid, "api") })
	expectJoinedPanic(t, func() { PipeLangSearchRows(rows, string([]byte{0xff})) })
	var nilRows []PipeLangRecordTestPackageAppRootContainerrow
	expectJoinedPanic(t, func() { PipeLangSearchRows(nilRows, "api") })
}
`, gobackend.PackageName)
}
