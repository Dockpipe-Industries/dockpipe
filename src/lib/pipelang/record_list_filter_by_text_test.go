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

func TestV220RecordListFilterByTextHIRCoreEvaluatorAndGo(t *testing.T) {
	analysis, typed, core, generated := v220RecordListFilterByTextPipeline(t)

	projection, err := BuildSemanticProjection(analysis)
	if err != nil {
		t.Fatal(err)
	}
	method := projectedMemberNamed(t, projectedTypeNamed(t, projection, "Root"), "FilterRows")
	if projection.LanguageContract != PipeLangLanguageContractV220 || projection.Schema != PipeLangSemanticProjectionVersion || method.Type.Identity == nil || method.Type.Identity.Path != PipeLangListSemanticPath || len(method.Type.Arguments) != 1 || method.Type.Arguments[0].Identity == nil || method.Type.Arguments[0].Identity.Path != "app.root.containerrow" || len(method.Parameters) != 2 || method.Parameters[0].Type.Identity == nil || method.Parameters[0].Type.Identity.Path != PipeLangListSemanticPath || method.Parameters[1].Type.Primitive != TypeString {
		t.Fatalf("record-list filter_by projection = %#v / %#v", projection, method)
	}

	function := typed.Functions[0]
	if typed.LanguageContract != coreir.LanguageContractV220 || function.Body.Kind != hir.ExprListFilterByText || function.Body.ListFilter == nil || function.Body.ListFilter.Values == nil || function.Body.ListFilter.Values.Reference == nil || function.Body.ListFilter.Values.Reference.Position != 0 || function.Body.ListFilter.Key == nil || function.Body.ListFilter.Key.Reference == nil || function.Body.ListFilter.Key.Reference.Position != 1 || function.Body.ListFilter.Name != "Name" || function.Body.ListFilter.Position != 1 || function.Body.ListFilter.Field.Path != "app.root.containerrow.name" || function.ReturnType.Kind != hir.TypeList || function.ReturnType.List == nil || function.ReturnType.List.Element.Kind != hir.TypeRecord {
		t.Fatalf("record-list filter_by HIR = %#v", typed)
	}

	coreFunction := core.Functions[0]
	if core.LanguageContract != coreir.LanguageContractV220 || coreFunction.Body.Kind != coreir.ExprListFilterByText || coreFunction.Body.ListFilter == nil || coreFunction.Body.ListFilter.Values == nil || coreFunction.Body.ListFilter.Values.Parameter == nil || *coreFunction.Body.ListFilter.Values.Parameter != 0 || coreFunction.Body.ListFilter.Key == nil || coreFunction.Body.ListFilter.Key.Parameter == nil || *coreFunction.Body.ListFilter.Key.Parameter != 1 || coreFunction.Body.ListFilter.Name != "Name" || coreFunction.Body.ListFilter.Position != 1 || coreFunction.Body.ListFilter.Field.Path != "app.root.containerrow.name" || coreFunction.ReturnType.Kind != coreir.TypeList || coreFunction.ReturnType.List == nil || coreFunction.ReturnType.List.Element.Kind != coreir.TypeRecord {
		t.Fatalf("record-list filter_by Core = %#v", core)
	}

	listType := coreFunction.Parameters[0].Type
	rowType := listType.List.Element
	rows := coreeval.Value{Type: listType, List: []coreeval.Value{
		optionalRecordValue(rowType, "container-1", "worker", true),
		optionalRecordValue(rowType, "container-2", "api", false),
		optionalRecordValue(rowType, "container-3", "worker", false),
		optionalRecordValue(rowType, "container-4", "e\u0301", false),
		optionalRecordValue(rowType, "container-5", "é", false),
	}}
	text := func(value string) coreeval.Value {
		return coreeval.Value{Type: coreir.Type{Kind: coreir.TypePrimitive, Primitive: coreir.PrimitiveString}, String: value}
	}
	for _, test := range []struct {
		name string
		key  string
		ids  []string
	}{
		{name: "stable-duplicate-order", key: "worker", ids: []string{"container-1", "container-3"}},
		{name: "composed", key: "é", ids: []string{"container-5"}},
		{name: "decomposed", key: "e\u0301", ids: []string{"container-4"}},
		{name: "canonical-empty", key: "missing"},
	} {
		t.Run(test.name, func(t *testing.T) {
			outcome, err := coreeval.Evaluate(coreFunction, []coreeval.Value{rows, text(test.key)})
			if err != nil || !outcome.OK || outcome.Value.List == nil || len(outcome.Value.List) != len(test.ids) {
				t.Fatalf("filter_by outcome = %#v (%v)", outcome, err)
			}
			for index, id := range test.ids {
				if outcome.Value.List[index].Record[0].String != id {
					t.Fatalf("filter_by result[%d] = %#v, want %q", index, outcome.Value.List[index], id)
				}
			}
		})
	}
	filtered, err := coreeval.Evaluate(coreFunction, []coreeval.Value{rows, text("worker")})
	if err != nil {
		t.Fatal(err)
	}
	rows.List[0].Record[0].String = "mutated"
	if filtered.Value.List[0].Record[0].String != "container-1" {
		t.Fatal("record-list filter_by result aliases caller-owned record storage")
	}
	invalid := rows
	invalid.List = append([]coreeval.Value(nil), rows.List...)
	invalid.List[1] = optionalRecordValue(rowType, string([]byte{0xff}), "api", false)
	if _, err := coreeval.Evaluate(coreFunction, []coreeval.Value{invalid, text("worker")}); err == nil {
		t.Fatal("record-list filter_by accepted invalid UTF-8 in an unmatched list element")
	}
	invalidSelected := rows
	invalidSelected.List = append([]coreeval.Value(nil), rows.List...)
	invalidSelected.List[1] = optionalRecordValue(rowType, "container-2", string([]byte{0xff}), false)
	if _, err := coreeval.Evaluate(coreFunction, []coreeval.Value{invalidSelected, text("worker")}); err == nil {
		t.Fatal("record-list filter_by accepted invalid UTF-8 in a selected field")
	}
	if _, err := coreeval.Evaluate(coreFunction, []coreeval.Value{rows, text(string([]byte{0xff}))}); err == nil {
		t.Fatal("record-list filter_by accepted an invalid UTF-8 key")
	}
	if _, err := coreeval.Evaluate(coreFunction, []coreeval.Value{{Type: listType}, text("worker")}); err == nil {
		t.Fatal("record-list filter_by accepted a nil list")
	}

	malformedHIR := typed
	malformedHIR.Functions = append([]hir.Function(nil), typed.Functions...)
	malformedHIR.Functions[0].Body = typed.Functions[0].Body
	malformedFilter := *typed.Functions[0].Body.ListFilter
	malformedFilter.Key = nil
	malformedHIR.Functions[0].Body.ListFilter = &malformedFilter
	if _, err := LowerHIRToCore(malformedHIR); err == nil {
		t.Fatal("Core lowering accepted malformed record-list filter_by HIR")
	} else if diagnostics, ok := AsDiagnostics(err); !ok || len(diagnostics) != 1 || diagnostics[0].Code != CodeCoreLowering {
		t.Fatalf("malformed filter_by HIR diagnostic = %#v (%v)", diagnostics, err)
	}
	priorHIR := typed
	priorHIR.LanguageContract = coreir.LanguageContractV210
	if _, err := LowerHIRToCore(priorHIR); err == nil {
		t.Fatal("v0.21.0 HIR implicitly accepted record-list filter_by")
	}

	malformedCore := coreFunction
	malformedCore.Body = coreFunction.Body
	malformedCoreFilter := *coreFunction.Body.ListFilter
	malformedCoreFilter.Field.Path = "app.root.containerrow.id"
	malformedCore.Body.ListFilter = &malformedCoreFilter
	if err := coreir.ValidateFunction(malformedCore); err == nil {
		t.Fatal("Core accepted a mismatched record-list filter_by field identity")
	}
	priorCore := core
	priorCore.LanguageContract = coreir.LanguageContractV210
	if _, err := gobackend.Generate(priorCore); err == nil {
		t.Fatal("v0.21.0 Core implicitly accepted record-list filter_by")
	}

	secondGenerated, err := gobackend.Generate(core)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(generated, secondGenerated) {
		t.Fatal("record-list filter_by generated Go is nondeterministic")
	}
	compileAndRunGeneratedGoFiles(t, generated, []byte(recordListFilterByTextGeneratedGoTest()))

	assertCompilerGolden(t, "record-list-filter-by-text.hir.json", canonicalJSON(t, typed))
	assertCompilerGolden(t, "record-list-filter-by-text.core.json", canonicalJSON(t, core))
	assertCompilerGolden(t, "record-list-filter-by-text.go", generated)
}

func TestV220ParserPreservesRecordListFilterByTextSelectorOperandsAndSpan(t *testing.T) {
	const source = `public Record Row { public string Id; } public Class Root { public List<Row> FilterRows(List<Row> values, string key) => filter_by(values, Row.Id, key); }`
	sources, diagnostics := NewSourceSet([]SourceInput{{Path: "root.pipe", Data: []byte(source)}})
	if diagnostics.HasErrors() {
		t.Fatal(diagnosticError(sources, diagnostics))
	}
	program, err := parseSourceFileWithLanguageContract(sources, sources.Files()[0], PipeLangLanguageContractV220)
	if err != nil {
		t.Fatal(err)
	}
	filter, ok := program.Classes[0].Methods[0].Body.(*ListFilterByTextExpr)
	if !ok {
		t.Fatalf("record-list filter_by AST = %#v", program.Classes[0].Methods[0].Body)
	}
	values, valuesOK := filter.Values.(*IdentExpr)
	key, keyOK := filter.Key.(*IdentExpr)
	if !filter.Span.IsValid() || !valuesOK || values.Name != "values" || !values.Span.IsValid() || filter.RecordType.Name != "Row" || !filter.RecordType.Span.IsValid() || filter.Field != "Id" || !filter.FieldSpan.IsValid() || !keyOK || key.Name != "key" || !key.Span.IsValid() {
		t.Fatalf("record-list filter_by AST = %#v", filter)
	}
}

func TestV220RecordListFilterByTextRejectsExcludedForms(t *testing.T) {
	tests := []struct {
		name string
		src  string
		code DiagnosticCode
	}{
		{name: "primitive list", src: `public Record Row { public string Id; } public Class Root { public List<string> FilterRows(List<string> values, string key) => filter_by(values, Row.Id, key); }`, code: CodeExpressionType},
		{name: "wrong return", src: `public Record Row { public string Id; } public Class Root { public Optional<Row> FilterRows(List<Row> values, string key) => filter_by(values, Row.Id, key); }`, code: CodeInvalidType},
		{name: "non-string key", src: `public Record Row { public string Id; } public Class Root { public List<Row> FilterRows(List<Row> values, int key) => filter_by(values, Row.Id, key); }`, code: CodeInvalidType},
		{name: "mismatched selector record", src: `public Record Row { public string Id; } public Record Other { public string Id; } public Class Root { public List<Row> FilterRows(List<Row> values, string key) => filter_by(values, Other.Id, key); }`, code: CodeInvalidType},
		{name: "missing selector field", src: `public Record Row { public string Id; } public Class Root { public List<Row> FilterRows(List<Row> values, string key) => filter_by(values, Row.Missing, key); }`, code: CodeInvalidMember},
		{name: "non-string selector", src: `public Record Row { public int Id; } public Class Root { public List<Row> FilterRows(List<Row> values, string key) => filter_by(values, Row.Id, key); }`, code: CodeInvalidType},
		{name: "reordered parameters", src: `public Record Row { public string Id; } public Class Root { public List<Row> FilterRows(string key, List<Row> values) => filter_by(values, Row.Id, key); }`, code: CodeInvalidType},
		{name: "computed list", src: `public Record Row { public string Id; } public Class Root { public List<Row> FilterRows(List<Row> values, string key) => filter_by(append(values, new Row { Id = "fixed" }), Row.Id, key); }`, code: CodeExpressionType},
		{name: "computed key", src: `public Record Row { public string Id; } public Class Root { public List<Row> FilterRows(List<Row> values, string key) => filter_by(values, Row.Id, key + "x"); }`, code: CodeExpressionType},
		{name: "nested filter", src: `public Record Row { public string Id; } public Class Root { public int CountRows(List<Row> values, string key) => count(filter_by(values, Row.Id, key)); }`, code: CodeInvalidType},
		{name: "list field", src: `public Record Row { public string Id; } public Class Root { public List<Row> Values; public List<Row> FilterRows(List<Row> values, string key) => filter_by(values, Row.Id, key); }`, code: CodeInvalidType},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			input := semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", "root.pipe", test.src)}, nil)
			input.LanguageContract = PipeLangLanguageContractV220
			analysis := AnalyzeSemanticModuleSet(input)
			if !analysis.Diagnostics.HasErrors() {
				t.Fatal("excluded v0.22.0 record-list filter_by form was accepted")
			}
			if analysis.Diagnostics[0].Code != test.code {
				t.Fatalf("record-list filter_by diagnostic = %s, want %s: %v", analysis.Diagnostics[0].Code, test.code, analysis.Error())
			}
		})
	}
}

func TestRecordListFilterByTextRequiresExplicitV220Migration(t *testing.T) {
	source := `public Record Row { public string Id; } public Class Root { public List<Row> FilterRows(List<Row> values, string key) => filter_by(values, Row.Id, key); }`
	for _, contract := range []LanguageContract{PipeLangLanguageContractV010, PipeLangLanguageContractV020, PipeLangLanguageContractV030, PipeLangLanguageContractV040, PipeLangLanguageContractV050, PipeLangLanguageContractV060, PipeLangLanguageContractV070, PipeLangLanguageContractV080, PipeLangLanguageContractV090, PipeLangLanguageContractV100, PipeLangLanguageContractV110, PipeLangLanguageContractV120, PipeLangLanguageContractV130, PipeLangLanguageContractV140, PipeLangLanguageContractV150, PipeLangLanguageContractV160, PipeLangLanguageContractV170, PipeLangLanguageContractV180, PipeLangLanguageContractV190, PipeLangLanguageContractV200, PipeLangLanguageContractV210} {
		input := semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", "root.pipe", source)}, nil)
		input.LanguageContract = contract
		analysis := AnalyzeSemanticModuleSet(input)
		if !analysis.Diagnostics.HasErrors() {
			t.Fatalf("%s implicitly accepted record-list filter_by", contract)
		}
	}
}

func TestV220PreservesV210RecordListFindByText(t *testing.T) {
	source, err := os.ReadFile("testdata/record-list-find-by-text.pipe")
	if err != nil {
		t.Fatal(err)
	}
	input := semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", "record-list-find-by-text.pipe", string(source))}, nil)
	input.LanguageContract = PipeLangLanguageContractV220
	analysis := AnalyzeSemanticModuleSet(input)
	if err := analysis.Error(); err != nil {
		t.Fatal(err)
	}
	typed, err := LowerSemanticMethodToHIR(analysis, semanticMethodNamed(t, analysis, "FindRow").Identity)
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

func v220RecordListFilterByTextPipeline(t *testing.T) (*Analysis, hir.Program, coreir.Program, []byte) {
	t.Helper()
	source, err := os.ReadFile("testdata/record-list-filter-by-text.pipe")
	if err != nil {
		t.Fatal(err)
	}
	input := semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", "record-list-filter-by-text.pipe", string(source))}, nil)
	input.LanguageContract = PipeLangLanguageContractV220
	analysis := AnalyzeSemanticModuleSet(input)
	if err := analysis.Error(); err != nil {
		t.Fatal(err)
	}
	typed, err := LowerSemanticMethodToHIR(analysis, semanticMethodNamed(t, analysis, "FilterRows").Identity)
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

func recordListFilterByTextGeneratedGoTest() string {
	return fmt.Sprintf(`package %s

import "testing"

func expectRecordListFilterByTextPanic(t *testing.T, invoke func()) {
	t.Helper()
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic")
		}
	}()
	invoke()
}

func TestGeneratedRecordListFilterByText(t *testing.T) {
	rows := []PipeLangRecordTestPackageAppRootContainerrow{
		{Id: "container-1", Name: "worker", Running: true},
		{Id: "container-2", Name: "api"},
		{Id: "container-3", Name: "worker"},
		{Id: "container-4", Name: "e\u0301"},
		{Id: "container-5", Name: "é"},
	}
	filtered := PipeLangFilterRows(rows, "worker")
	if len(filtered) != 2 || filtered[0].Id != "container-1" || filtered[1].Id != "container-3" {
		t.Fatal("stable filter order mismatch")
	}
	filtered[0].Id = "changed"
	if rows[0].Id != "container-1" {
		t.Fatal("filtered list aliases caller storage")
	}
	if missing := PipeLangFilterRows(rows, "missing"); missing == nil || len(missing) != 0 {
		t.Fatal("missing filter is not canonical empty")
	}
	if composed := PipeLangFilterRows(rows, "é"); len(composed) != 1 || composed[0].Id != "container-5" {
		t.Fatal("composed filter mismatch")
	}
	if decomposed := PipeLangFilterRows(rows, "e\u0301"); len(decomposed) != 1 || decomposed[0].Id != "container-4" {
		t.Fatal("decomposed filter mismatch")
	}
	invalid := append([]PipeLangRecordTestPackageAppRootContainerrow(nil), rows...)
	invalid[1].Id = string([]byte{0xff})
	expectRecordListFilterByTextPanic(t, func() { PipeLangFilterRows(invalid, "worker") })
	invalidSelected := append([]PipeLangRecordTestPackageAppRootContainerrow(nil), rows...)
	invalidSelected[1].Name = string([]byte{0xff})
	expectRecordListFilterByTextPanic(t, func() { PipeLangFilterRows(invalidSelected, "worker") })
	expectRecordListFilterByTextPanic(t, func() { PipeLangFilterRows(rows, string([]byte{0xff})) })
	var nilRows []PipeLangRecordTestPackageAppRootContainerrow
	expectRecordListFilterByTextPanic(t, func() { PipeLangFilterRows(nilRows, "worker") })
}
`, gobackend.PackageName)
}
