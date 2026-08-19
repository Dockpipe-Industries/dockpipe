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

func TestV240RecordListFilterContainsCaseFoldedHIRCoreEvaluatorAndGo(t *testing.T) {
	analysis, typed, core, generated := v240RecordListFilterContainsCaseFoldedPipeline(t)

	projection, err := BuildSemanticProjection(analysis)
	if err != nil {
		t.Fatal(err)
	}
	method := projectedMemberNamed(t, projectedTypeNamed(t, projection, "Root"), "SearchRows")
	if projection.LanguageContract != PipeLangLanguageContractV240 || projection.Schema != PipeLangSemanticProjectionVersion || method.Type.Identity == nil || method.Type.Identity.Path != PipeLangListSemanticPath || len(method.Type.Arguments) != 1 || method.Type.Arguments[0].Identity == nil || method.Type.Arguments[0].Identity.Path != "app.root.containerrow" || len(method.Parameters) != 2 || method.Parameters[0].Type.Identity == nil || method.Parameters[0].Type.Identity.Path != PipeLangListSemanticPath || method.Parameters[1].Type.Primitive != TypeString {
		t.Fatalf("record-list filter_contains_casefolded projection = %#v / %#v", projection, method)
	}

	function := typed.Functions[0]
	if typed.LanguageContract != coreir.LanguageContractV240 || function.Body.Kind != hir.ExprListFilterContainsCaseFolded || function.Body.ListFilterContainsCaseFolded == nil || function.Body.ListFilterContainsCaseFolded.Values == nil || function.Body.ListFilterContainsCaseFolded.Values.Reference == nil || function.Body.ListFilterContainsCaseFolded.Values.Reference.Position != 0 || function.Body.ListFilterContainsCaseFolded.Query == nil || function.Body.ListFilterContainsCaseFolded.Query.Reference == nil || function.Body.ListFilterContainsCaseFolded.Query.Reference.Position != 1 || function.Body.ListFilterContainsCaseFolded.Name != "Name" || function.Body.ListFilterContainsCaseFolded.Position != 1 || function.Body.ListFilterContainsCaseFolded.Field.Path != "app.root.containerrow.name" || function.ReturnType.Kind != hir.TypeList || function.ReturnType.List == nil || function.ReturnType.List.Element.Kind != hir.TypeRecord {
		t.Fatalf("record-list filter_contains_casefolded HIR = %#v", typed)
	}

	coreFunction := core.Functions[0]
	if core.LanguageContract != coreir.LanguageContractV240 || coreFunction.Body.Kind != coreir.ExprListFilterContainsCaseFolded || coreFunction.Body.ListFilterContainsCaseFolded == nil || coreFunction.Body.ListFilterContainsCaseFolded.Values == nil || coreFunction.Body.ListFilterContainsCaseFolded.Values.Parameter == nil || *coreFunction.Body.ListFilterContainsCaseFolded.Values.Parameter != 0 || coreFunction.Body.ListFilterContainsCaseFolded.Query == nil || coreFunction.Body.ListFilterContainsCaseFolded.Query.Parameter == nil || *coreFunction.Body.ListFilterContainsCaseFolded.Query.Parameter != 1 || coreFunction.Body.ListFilterContainsCaseFolded.Name != "Name" || coreFunction.Body.ListFilterContainsCaseFolded.Position != 1 || coreFunction.Body.ListFilterContainsCaseFolded.Field.Path != "app.root.containerrow.name" || coreFunction.ReturnType.Kind != coreir.TypeList || coreFunction.ReturnType.List == nil || coreFunction.ReturnType.List.Element.Kind != coreir.TypeRecord {
		t.Fatalf("record-list filter_contains_casefolded Core = %#v", core)
	}

	listType := coreFunction.Parameters[0].Type
	rowType := listType.List.Element
	rows := coreeval.Value{Type: listType, List: []coreeval.Value{
		optionalRecordValue(rowType, "container-1", "Straße Worker", true),
		optionalRecordValue(rowType, "container-2", "api", false),
		optionalRecordValue(rowType, "container-3", "STRASSE worker", false),
		optionalRecordValue(rowType, "container-4", "e\u0301", false),
		optionalRecordValue(rowType, "container-5", "é", false),
		optionalRecordValue(rowType, "container-6", "Kube", false),
		optionalRecordValue(rowType, "container-7", "İstanbul", false),
	}}
	text := func(value string) coreeval.Value {
		return coreeval.Value{Type: coreir.Type{Kind: coreir.TypePrimitive, Primitive: coreir.PrimitiveString}, String: value}
	}
	for _, test := range []struct {
		name string
		key  string
		ids  []string
	}{
		{name: "full-fold-stable-order", key: "strasse", ids: []string{"container-1", "container-3"}},
		{name: "ascii-case", key: "WORKER", ids: []string{"container-1", "container-3"}},
		{name: "kelvin", key: "KUBE", ids: []string{"container-6"}},
		{name: "dotted-i", key: "i\u0307stanbul", ids: []string{"container-7"}},
		{name: "composed", key: "é", ids: []string{"container-5"}},
		{name: "decomposed", key: "e\u0301", ids: []string{"container-4"}},
		{name: "empty-query", key: "", ids: []string{"container-1", "container-2", "container-3", "container-4", "container-5", "container-6", "container-7"}},
		{name: "canonical-empty", key: "missing"},
	} {
		t.Run(test.name, func(t *testing.T) {
			outcome, err := coreeval.Evaluate(coreFunction, []coreeval.Value{rows, text(test.key)})
			if err != nil || !outcome.OK || outcome.Value.List == nil || len(outcome.Value.List) != len(test.ids) {
				t.Fatalf("filter_contains_casefolded outcome = %#v (%v)", outcome, err)
			}
			for index, id := range test.ids {
				if outcome.Value.List[index].Record[0].String != id {
					t.Fatalf("filter_contains_casefolded result[%d] = %#v, want %q", index, outcome.Value.List[index], id)
				}
			}
		})
	}
	filtered, err := coreeval.Evaluate(coreFunction, []coreeval.Value{rows, text("strasse")})
	if err != nil {
		t.Fatal(err)
	}
	rows.List[0].Record[0].String = "mutated"
	if filtered.Value.List[0].Record[0].String != "container-1" {
		t.Fatal("record-list filter_contains_casefolded result aliases caller-owned record storage")
	}
	invalid := rows
	invalid.List = append([]coreeval.Value(nil), rows.List...)
	invalid.List[1] = optionalRecordValue(rowType, string([]byte{0xff}), "api", false)
	if _, err := coreeval.Evaluate(coreFunction, []coreeval.Value{invalid, text("worker")}); err == nil {
		t.Fatal("record-list filter_contains_casefolded accepted invalid UTF-8 in an unmatched list element")
	}
	invalidSelected := rows
	invalidSelected.List = append([]coreeval.Value(nil), rows.List...)
	invalidSelected.List[1] = optionalRecordValue(rowType, "container-2", string([]byte{0xff}), false)
	if _, err := coreeval.Evaluate(coreFunction, []coreeval.Value{invalidSelected, text("worker")}); err == nil {
		t.Fatal("record-list filter_contains_casefolded accepted invalid UTF-8 in a selected field")
	}
	if _, err := coreeval.Evaluate(coreFunction, []coreeval.Value{rows, text(string([]byte{0xff}))}); err == nil {
		t.Fatal("record-list filter_contains_casefolded accepted an invalid UTF-8 key")
	}
	if _, err := coreeval.Evaluate(coreFunction, []coreeval.Value{{Type: listType}, text("worker")}); err == nil {
		t.Fatal("record-list filter_contains_casefolded accepted a nil list")
	}

	malformedHIR := typed
	malformedHIR.Functions = append([]hir.Function(nil), typed.Functions...)
	malformedHIR.Functions[0].Body = typed.Functions[0].Body
	malformedFilter := *typed.Functions[0].Body.ListFilterContainsCaseFolded
	malformedFilter.Query = nil
	malformedHIR.Functions[0].Body.ListFilterContainsCaseFolded = &malformedFilter
	if _, err := LowerHIRToCore(malformedHIR); err == nil {
		t.Fatal("Core lowering accepted malformed record-list filter_contains_casefolded HIR")
	} else if diagnostics, ok := AsDiagnostics(err); !ok || len(diagnostics) != 1 || diagnostics[0].Code != CodeCoreLowering {
		t.Fatalf("malformed filter_contains_casefolded HIR diagnostic = %#v (%v)", diagnostics, err)
	}
	priorHIR := typed
	priorHIR.LanguageContract = coreir.LanguageContractV230
	if _, err := LowerHIRToCore(priorHIR); err == nil {
		t.Fatal("v0.23.0 HIR implicitly accepted record-list filter_contains_casefolded")
	}

	malformedCore := coreFunction
	malformedCore.Body = coreFunction.Body
	malformedCoreFilter := *coreFunction.Body.ListFilterContainsCaseFolded
	malformedCoreFilter.Field.Path = "app.root.containerrow.id"
	malformedCore.Body.ListFilterContainsCaseFolded = &malformedCoreFilter
	if err := coreir.ValidateFunction(malformedCore); err == nil {
		t.Fatal("Core accepted a mismatched record-list filter_contains_casefolded field identity")
	}
	priorCore := core
	priorCore.LanguageContract = coreir.LanguageContractV230
	if _, err := gobackend.Generate(priorCore); err == nil {
		t.Fatal("v0.23.0 Core implicitly accepted record-list filter_contains_casefolded")
	}

	secondGenerated, err := gobackend.Generate(core)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(generated, secondGenerated) {
		t.Fatal("record-list filter_contains_casefolded generated Go is nondeterministic")
	}
	compileAndRunGeneratedGoFiles(t, generated, []byte(recordListFilterContainsCaseFoldedGeneratedGoTest()))

	assertCompilerGolden(t, "record-list-filter-contains-casefolded.hir.json", canonicalJSON(t, typed))
	assertCompilerGolden(t, "record-list-filter-contains-casefolded.core.json", canonicalJSON(t, core))
	assertCompilerGolden(t, "record-list-filter-contains-casefolded.go", generated)
}

func TestV240ParserPreservesRecordListFilterContainsCaseFoldedSelectorOperandsAndSpan(t *testing.T) {
	const source = `public Record Row { public string Id; } public Class Root { public List<Row> FilterRows(List<Row> values, string key) => filter_contains_casefolded(values, Row.Id, key); }`
	sources, diagnostics := NewSourceSet([]SourceInput{{Path: "root.pipe", Data: []byte(source)}})
	if diagnostics.HasErrors() {
		t.Fatal(diagnosticError(sources, diagnostics))
	}
	program, err := parseSourceFileWithLanguageContract(sources, sources.Files()[0], PipeLangLanguageContractV240)
	if err != nil {
		t.Fatal(err)
	}
	filter, ok := program.Classes[0].Methods[0].Body.(*ListFilterContainsCaseFoldedExpr)
	if !ok {
		t.Fatalf("record-list filter_contains_casefolded AST = %#v", program.Classes[0].Methods[0].Body)
	}
	values, valuesOK := filter.Values.(*IdentExpr)
	key, keyOK := filter.Query.(*IdentExpr)
	if !filter.Span.IsValid() || !valuesOK || values.Name != "values" || !values.Span.IsValid() || filter.RecordType.Name != "Row" || !filter.RecordType.Span.IsValid() || filter.Field != "Id" || !filter.FieldSpan.IsValid() || !keyOK || key.Name != "key" || !key.Span.IsValid() {
		t.Fatalf("record-list filter_contains_casefolded AST = %#v", filter)
	}
}

func TestV240RecordListFilterContainsCaseFoldedRejectsExcludedForms(t *testing.T) {
	tests := []struct {
		name string
		src  string
		code DiagnosticCode
	}{
		{name: "primitive list", src: `public Record Row { public string Id; } public Class Root { public List<string> FilterRows(List<string> values, string key) => filter_contains_casefolded(values, Row.Id, key); }`, code: CodeExpressionType},
		{name: "wrong return", src: `public Record Row { public string Id; } public Class Root { public Optional<Row> FilterRows(List<Row> values, string key) => filter_contains_casefolded(values, Row.Id, key); }`, code: CodeInvalidType},
		{name: "non-string key", src: `public Record Row { public string Id; } public Class Root { public List<Row> FilterRows(List<Row> values, int key) => filter_contains_casefolded(values, Row.Id, key); }`, code: CodeInvalidType},
		{name: "mismatched selector record", src: `public Record Row { public string Id; } public Record Other { public string Id; } public Class Root { public List<Row> FilterRows(List<Row> values, string key) => filter_contains_casefolded(values, Other.Id, key); }`, code: CodeInvalidType},
		{name: "missing selector field", src: `public Record Row { public string Id; } public Class Root { public List<Row> FilterRows(List<Row> values, string key) => filter_contains_casefolded(values, Row.Missing, key); }`, code: CodeInvalidMember},
		{name: "non-string selector", src: `public Record Row { public int Id; } public Class Root { public List<Row> FilterRows(List<Row> values, string key) => filter_contains_casefolded(values, Row.Id, key); }`, code: CodeInvalidType},
		{name: "reordered parameters", src: `public Record Row { public string Id; } public Class Root { public List<Row> FilterRows(string key, List<Row> values) => filter_contains_casefolded(values, Row.Id, key); }`, code: CodeInvalidType},
		{name: "computed list", src: `public Record Row { public string Id; } public Class Root { public List<Row> FilterRows(List<Row> values, string key) => filter_contains_casefolded(append(values, new Row { Id = "fixed" }), Row.Id, key); }`, code: CodeExpressionType},
		{name: "computed key", src: `public Record Row { public string Id; } public Class Root { public List<Row> FilterRows(List<Row> values, string key) => filter_contains_casefolded(values, Row.Id, key + "x"); }`, code: CodeExpressionType},
		{name: "nested filter", src: `public Record Row { public string Id; } public Class Root { public int CountRows(List<Row> values, string key) => count(filter_contains_casefolded(values, Row.Id, key)); }`, code: CodeInvalidType},
		{name: "list field", src: `public Record Row { public string Id; } public Class Root { public List<Row> Values; public List<Row> FilterRows(List<Row> values, string key) => filter_contains_casefolded(values, Row.Id, key); }`, code: CodeInvalidType},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			input := semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", "root.pipe", test.src)}, nil)
			input.LanguageContract = PipeLangLanguageContractV240
			analysis := AnalyzeSemanticModuleSet(input)
			if !analysis.Diagnostics.HasErrors() {
				t.Fatal("excluded v0.24.0 record-list filter_contains_casefolded form was accepted")
			}
			if analysis.Diagnostics[0].Code != test.code {
				t.Fatalf("record-list filter_contains_casefolded diagnostic = %s, want %s: %v", analysis.Diagnostics[0].Code, test.code, analysis.Error())
			}
		})
	}
}

func TestRecordListFilterContainsCaseFoldedRequiresExplicitV240Migration(t *testing.T) {
	source := `public Record Row { public string Id; } public Class Root { public List<Row> FilterRows(List<Row> values, string key) => filter_contains_casefolded(values, Row.Id, key); }`
	for _, contract := range []LanguageContract{PipeLangLanguageContractV010, PipeLangLanguageContractV020, PipeLangLanguageContractV030, PipeLangLanguageContractV040, PipeLangLanguageContractV050, PipeLangLanguageContractV060, PipeLangLanguageContractV070, PipeLangLanguageContractV080, PipeLangLanguageContractV090, PipeLangLanguageContractV100, PipeLangLanguageContractV110, PipeLangLanguageContractV120, PipeLangLanguageContractV130, PipeLangLanguageContractV140, PipeLangLanguageContractV150, PipeLangLanguageContractV160, PipeLangLanguageContractV170, PipeLangLanguageContractV180, PipeLangLanguageContractV190, PipeLangLanguageContractV200, PipeLangLanguageContractV210, PipeLangLanguageContractV220, PipeLangLanguageContractV230} {
		input := semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", "root.pipe", source)}, nil)
		input.LanguageContract = contract
		analysis := AnalyzeSemanticModuleSet(input)
		if !analysis.Diagnostics.HasErrors() {
			t.Fatalf("%s implicitly accepted record-list filter_contains_casefolded", contract)
		}
	}
}

func TestV240PreservesV230ContainsCaseFolded(t *testing.T) {
	source, err := os.ReadFile("testdata/text-contains-casefolded.pipe")
	if err != nil {
		t.Fatal(err)
	}
	input := semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", "text-contains-casefolded.pipe", string(source))}, nil)
	input.LanguageContract = PipeLangLanguageContractV240
	analysis := AnalyzeSemanticModuleSet(input)
	if err := analysis.Error(); err != nil {
		t.Fatal(err)
	}
	typed, err := LowerSemanticMethodToHIR(analysis, semanticMethodNamed(t, analysis, "ContainsCaseFolded").Identity)
	if err != nil {
		t.Fatal(err)
	}
	core, err := LowerHIRToCore(typed)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := gobackend.Generate(core); err != nil {
		t.Fatal(err)
	}
}

func v240RecordListFilterContainsCaseFoldedPipeline(t *testing.T) (*Analysis, hir.Program, coreir.Program, []byte) {
	t.Helper()
	source, err := os.ReadFile("testdata/record-list-filter-contains-casefolded.pipe")
	if err != nil {
		t.Fatal(err)
	}
	input := semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", "record-list-filter-contains-casefolded.pipe", string(source))}, nil)
	input.LanguageContract = PipeLangLanguageContractV240
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

func recordListFilterContainsCaseFoldedGeneratedGoTest() string {
	return fmt.Sprintf(`package %s

import "testing"

func expectRecordListFilterContainsCaseFoldedPanic(t *testing.T, invoke func()) {
	t.Helper()
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic")
		}
	}()
	invoke()
}

func TestGeneratedRecordListFilterContainsCaseFolded(t *testing.T) {
	rows := []PipeLangRecordTestPackageAppRootContainerrow{
		{Id: "container-1", Name: "Straße Worker", Running: true},
		{Id: "container-2", Name: "api"},
		{Id: "container-3", Name: "STRASSE worker"},
		{Id: "container-4", Name: "e\u0301"},
		{Id: "container-5", Name: "é"},
		{Id: "container-6", Name: "Kube"},
		{Id: "container-7", Name: "İstanbul"},
	}
	filtered := PipeLangSearchRows(rows, "strasse")
	if len(filtered) != 2 || filtered[0].Id != "container-1" || filtered[1].Id != "container-3" {
		t.Fatal("stable filter order mismatch")
	}
	filtered[0].Id = "changed"
	if rows[0].Id != "container-1" {
		t.Fatal("filtered list aliases caller storage")
	}
	if missing := PipeLangSearchRows(rows, "missing"); missing == nil || len(missing) != 0 {
		t.Fatal("missing filter is not canonical empty")
	}
	if all := PipeLangSearchRows(rows, ""); len(all) != len(rows) {
		t.Fatal("empty query did not match every row")
	}
	if kelvin := PipeLangSearchRows(rows, "KUBE"); len(kelvin) != 1 || kelvin[0].Id != "container-6" {
		t.Fatal("Kelvin case-fold mismatch")
	}
	if dotted := PipeLangSearchRows(rows, "i\u0307stanbul"); len(dotted) != 1 || dotted[0].Id != "container-7" {
		t.Fatal("dotted-I case-fold mismatch")
	}
	if composed := PipeLangSearchRows(rows, "é"); len(composed) != 1 || composed[0].Id != "container-5" {
		t.Fatal("composed filter mismatch")
	}
	if decomposed := PipeLangSearchRows(rows, "e\u0301"); len(decomposed) != 1 || decomposed[0].Id != "container-4" {
		t.Fatal("decomposed filter mismatch")
	}
	invalid := append([]PipeLangRecordTestPackageAppRootContainerrow(nil), rows...)
	invalid[1].Id = string([]byte{0xff})
	expectRecordListFilterContainsCaseFoldedPanic(t, func() { PipeLangSearchRows(invalid, "worker") })
	invalidSelected := append([]PipeLangRecordTestPackageAppRootContainerrow(nil), rows...)
	invalidSelected[1].Name = string([]byte{0xff})
	expectRecordListFilterContainsCaseFoldedPanic(t, func() { PipeLangSearchRows(invalidSelected, "worker") })
	expectRecordListFilterContainsCaseFoldedPanic(t, func() { PipeLangSearchRows(rows, string([]byte{0xff})) })
	var nilRows []PipeLangRecordTestPackageAppRootContainerrow
	expectRecordListFilterContainsCaseFoldedPanic(t, func() { PipeLangSearchRows(nilRows, "worker") })
}
`, gobackend.PackageName)
}
