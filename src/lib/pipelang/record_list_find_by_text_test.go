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

func TestV210RecordListFindByTextHIRCoreEvaluatorAndGo(t *testing.T) {
	analysis, typed, core, generated := v210RecordListFindByTextPipeline(t)

	projection, err := BuildSemanticProjection(analysis)
	if err != nil {
		t.Fatal(err)
	}
	method := projectedMemberNamed(t, projectedTypeNamed(t, projection, "Root"), "FindRow")
	if projection.LanguageContract != PipeLangLanguageContractV210 || projection.Schema != PipeLangSemanticProjectionVersion || method.Type.Identity == nil || method.Type.Identity.PackageID != PipeLangBuiltinPackageID || method.Type.Identity.Path != PipeLangOptionalSemanticPath || len(method.Type.Arguments) != 1 || method.Type.Arguments[0].Identity == nil || method.Type.Arguments[0].Identity.Path != "app.root.containerrow" || len(method.Parameters) != 2 || method.Parameters[0].Type.Identity == nil || method.Parameters[0].Type.Identity.Path != PipeLangListSemanticPath || method.Parameters[1].Type.Primitive != TypeString {
		t.Fatalf("record-list find_by projection = %#v / %#v", projection, method)
	}

	function := typed.Functions[0]
	if typed.LanguageContract != coreir.LanguageContractV210 || function.Body.Kind != hir.ExprListFindByText || function.Body.ListFind == nil || function.Body.ListFind.Values == nil || function.Body.ListFind.Values.Reference == nil || function.Body.ListFind.Values.Reference.Position != 0 || function.Body.ListFind.Key == nil || function.Body.ListFind.Key.Reference == nil || function.Body.ListFind.Key.Reference.Position != 1 || function.Body.ListFind.Name != "Id" || function.Body.ListFind.Position != 0 || function.Body.ListFind.Field.Path != "app.root.containerrow.id" || function.ReturnType.Kind != hir.TypeOptional || function.ReturnType.Optional == nil || function.ReturnType.Optional.Value.Kind != hir.TypeRecord {
		t.Fatalf("record-list find_by HIR = %#v", typed)
	}

	coreFunction := core.Functions[0]
	if core.LanguageContract != coreir.LanguageContractV210 || coreFunction.Body.Kind != coreir.ExprListFindByText || coreFunction.Body.ListFind == nil || coreFunction.Body.ListFind.Values == nil || coreFunction.Body.ListFind.Values.Parameter == nil || *coreFunction.Body.ListFind.Values.Parameter != 0 || coreFunction.Body.ListFind.Key == nil || coreFunction.Body.ListFind.Key.Parameter == nil || *coreFunction.Body.ListFind.Key.Parameter != 1 || coreFunction.Body.ListFind.Name != "Id" || coreFunction.Body.ListFind.Position != 0 || coreFunction.Body.ListFind.Field.Path != "app.root.containerrow.id" || coreFunction.ReturnType.Kind != coreir.TypeOptional || coreFunction.ReturnType.Optional == nil || coreFunction.ReturnType.Optional.Value.Kind != coreir.TypeRecord {
		t.Fatalf("record-list find_by Core = %#v", core)
	}

	listType := coreFunction.Parameters[0].Type
	rowType := listType.List.Element
	rows := coreeval.Value{Type: listType, List: []coreeval.Value{
		optionalRecordValue(rowType, "container-1", "first", true),
		optionalRecordValue(rowType, "container-1", "duplicate", false),
		optionalRecordValue(rowType, "é", "composed", false),
		optionalRecordValue(rowType, "e\u0301", "decomposed", false),
	}}
	for _, test := range []struct {
		name    string
		key     string
		present bool
		rowName string
	}{
		{name: "first-stable-key-match", key: "container-1", present: true, rowName: "first"},
		{name: "composed", key: "é", present: true, rowName: "composed"},
		{name: "decomposed", key: "e\u0301", present: true, rowName: "decomposed"},
		{name: "missing", key: "container-2"},
	} {
		t.Run(test.name, func(t *testing.T) {
			outcome, err := coreeval.Evaluate(coreFunction, []coreeval.Value{rows, {Type: coreir.Type{Kind: coreir.TypePrimitive, Primitive: coreir.PrimitiveString}, String: test.key}})
			if err != nil || !outcome.OK || outcome.Value.Optional == nil || outcome.Value.Optional.Present != test.present {
				t.Fatalf("find_by outcome = %#v (%v)", outcome, err)
			}
			if test.present && (outcome.Value.Optional.Value == nil || outcome.Value.Optional.Value.Record[1].String != test.rowName) {
				t.Fatalf("find_by payload = %#v", outcome.Value.Optional)
			}
			if !test.present && outcome.Value.Optional.Value != nil {
				t.Fatalf("absent find_by payload = %#v", outcome.Value.Optional)
			}
		})
	}
	selected, err := coreeval.Evaluate(coreFunction, []coreeval.Value{rows, {Type: coreir.Type{Kind: coreir.TypePrimitive, Primitive: coreir.PrimitiveString}, String: "container-1"}})
	if err != nil {
		t.Fatal(err)
	}
	rows.List[0].Record[1].String = "mutated"
	if selected.Value.Optional.Value.Record[1].String != "first" {
		t.Fatal("record-list find_by result aliases caller-owned record storage")
	}
	invalid := rows
	invalid.List = append([]coreeval.Value(nil), rows.List...)
	invalid.List[3] = optionalRecordValue(rowType, string([]byte{0xff}), "bad", false)
	if _, err := coreeval.Evaluate(coreFunction, []coreeval.Value{invalid, {Type: coreir.Type{Kind: coreir.TypePrimitive, Primitive: coreir.PrimitiveString}, String: "absent"}}); err == nil {
		t.Fatal("record-list find_by accepted invalid UTF-8 in an unselected list element")
	}
	if _, err := coreeval.Evaluate(coreFunction, []coreeval.Value{rows, {Type: coreir.Type{Kind: coreir.TypePrimitive, Primitive: coreir.PrimitiveString}, String: string([]byte{0xff})}}); err == nil {
		t.Fatal("record-list find_by accepted an invalid UTF-8 key")
	}
	if _, err := coreeval.Evaluate(coreFunction, []coreeval.Value{{Type: listType}, {Type: coreir.Type{Kind: coreir.TypePrimitive, Primitive: coreir.PrimitiveString}}}); err == nil {
		t.Fatal("record-list find_by accepted a nil list")
	}

	malformedHIR := typed
	malformedHIR.Functions = append([]hir.Function(nil), typed.Functions...)
	malformedHIR.Functions[0].Body = typed.Functions[0].Body
	malformedFind := *typed.Functions[0].Body.ListFind
	malformedFind.Key = nil
	malformedHIR.Functions[0].Body.ListFind = &malformedFind
	if _, err := LowerHIRToCore(malformedHIR); err == nil {
		t.Fatal("Core lowering accepted malformed record-list find_by HIR")
	} else if diagnostics, ok := AsDiagnostics(err); !ok || len(diagnostics) != 1 || diagnostics[0].Code != CodeCoreLowering {
		t.Fatalf("malformed find_by HIR diagnostic = %#v (%v)", diagnostics, err)
	}
	priorHIR := typed
	priorHIR.LanguageContract = coreir.LanguageContractV200
	if _, err := LowerHIRToCore(priorHIR); err == nil {
		t.Fatal("v0.20.0 HIR implicitly accepted record-list find_by")
	}

	malformedCore := coreFunction
	malformedCore.Body = coreFunction.Body
	malformedCoreFind := *coreFunction.Body.ListFind
	malformedCoreFind.Field.Path = "app.root.containerrow.name"
	malformedCore.Body.ListFind = &malformedCoreFind
	if err := coreir.ValidateFunction(malformedCore); err == nil {
		t.Fatal("Core accepted a mismatched record-list find_by field identity")
	}
	priorCore := core
	priorCore.LanguageContract = coreir.LanguageContractV200
	if _, err := gobackend.Generate(priorCore); err == nil {
		t.Fatal("v0.20.0 Core implicitly accepted record-list find_by")
	}

	secondGenerated, err := gobackend.Generate(core)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(generated, secondGenerated) {
		t.Fatal("record-list find_by generated Go is nondeterministic")
	}
	compileAndRunGeneratedGoFiles(t, generated, []byte(recordListFindByTextGeneratedGoTest()))

	assertCompilerGolden(t, "record-list-find-by-text.hir.json", canonicalJSON(t, typed))
	assertCompilerGolden(t, "record-list-find-by-text.core.json", canonicalJSON(t, core))
	assertCompilerGolden(t, "record-list-find-by-text.go", generated)
}

func TestV210ParserPreservesRecordListFindByTextSelectorOperandsAndSpan(t *testing.T) {
	const source = `public Record Row { public string Id; } public Class Root { public Optional<Row> FindRow(List<Row> values, string key) => find_by(values, Row.Id, key); }`
	sources, diagnostics := NewSourceSet([]SourceInput{{Path: "root.pipe", Data: []byte(source)}})
	if diagnostics.HasErrors() {
		t.Fatal(diagnosticError(sources, diagnostics))
	}
	program, err := parseSourceFileWithLanguageContract(sources, sources.Files()[0], PipeLangLanguageContractV210)
	if err != nil {
		t.Fatal(err)
	}
	find, ok := program.Classes[0].Methods[0].Body.(*ListFindByTextExpr)
	if !ok {
		t.Fatalf("record-list find_by AST = %#v", program.Classes[0].Methods[0].Body)
	}
	values, valuesOK := find.Values.(*IdentExpr)
	key, keyOK := find.Key.(*IdentExpr)
	if !find.Span.IsValid() || !valuesOK || values.Name != "values" || !values.Span.IsValid() || find.RecordType.Name != "Row" || !find.RecordType.Span.IsValid() || find.Field != "Id" || !find.FieldSpan.IsValid() || !keyOK || key.Name != "key" || !key.Span.IsValid() {
		t.Fatalf("record-list find_by AST = %#v", find)
	}
}

func TestV210RecordListFindByTextSupportsAnotherDeclaredStringField(t *testing.T) {
	const source = `public Record Row { public string Id; public string Name; } public Class Root { public Optional<Row> FindByName(List<Row> values, string key) => find_by(values, Row.Name, key); }`
	input := semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", "root.pipe", source)}, nil)
	input.LanguageContract = PipeLangLanguageContractV210
	analysis := AnalyzeSemanticModuleSet(input)
	if err := analysis.Error(); err != nil {
		t.Fatal(err)
	}
	typed, err := LowerSemanticMethodToHIR(analysis, semanticMethodNamed(t, analysis, "FindByName").Identity)
	if err != nil {
		t.Fatal(err)
	}
	if typed.Functions[0].Body.ListFind == nil || typed.Functions[0].Body.ListFind.Position != 1 || typed.Functions[0].Body.ListFind.Name != "Name" {
		t.Fatalf("Name selector HIR = %#v", typed.Functions[0].Body.ListFind)
	}
	core, err := LowerHIRToCore(typed)
	if err != nil {
		t.Fatal(err)
	}
	function := core.Functions[0]
	rowType := function.Parameters[0].Type.List.Element
	rows := coreeval.Value{Type: function.Parameters[0].Type, List: []coreeval.Value{
		{Type: rowType, Record: []coreeval.Value{{Type: rowType.Record.Fields[0].Type, String: "one"}, {Type: rowType.Record.Fields[1].Type, String: "api"}}},
		{Type: rowType, Record: []coreeval.Value{{Type: rowType.Record.Fields[0].Type, String: "two"}, {Type: rowType.Record.Fields[1].Type, String: "worker"}}},
	}}
	outcome, err := coreeval.Evaluate(function, []coreeval.Value{rows, {Type: coreir.Type{Kind: coreir.TypePrimitive, Primitive: coreir.PrimitiveString}, String: "worker"}})
	if err != nil || !outcome.OK || outcome.Value.Optional == nil || !outcome.Value.Optional.Present || outcome.Value.Optional.Value.Record[0].String != "two" {
		t.Fatalf("Name selector outcome = %#v (%v)", outcome, err)
	}
	generated, err := gobackend.Generate(core)
	if err != nil {
		t.Fatal(err)
	}
	compileAndRunGeneratedGoFiles(t, generated, []byte(fmt.Sprintf(`package %s

import "testing"

func TestGeneratedFindByName(t *testing.T) {
	rows := []PipeLangRecordTestPackageAppRootRow{{Id: "one", Name: "api"}, {Id: "two", Name: "worker"}}
	found, present := PipeLangFindByName(rows, "worker").(pipelangOptionalSome[PipeLangRecordTestPackageAppRootRow])
	if !present || found.value.Id != "two" {
		t.Fatal("Name selector mismatch")
	}
}
`, gobackend.PackageName)))
}

func TestV210RecordListFindByTextRejectsExcludedForms(t *testing.T) {
	tests := []struct {
		name string
		src  string
		code DiagnosticCode
	}{
		{name: "primitive list", src: `public Record Row { public string Id; } public Class Root { public Optional<Row> FindRow(List<string> values, string key) => find_by(values, Row.Id, key); }`, code: CodeInvalidType},
		{name: "wrong return", src: `public Record Row { public string Id; } public Class Root { public Row FindRow(List<Row> values, string key) => find_by(values, Row.Id, key); }`, code: CodeInvalidType},
		{name: "non-string key", src: `public Record Row { public string Id; } public Class Root { public Optional<Row> FindRow(List<Row> values, int key) => find_by(values, Row.Id, key); }`, code: CodeInvalidType},
		{name: "mismatched selector record", src: `public Record Row { public string Id; } public Record Other { public string Id; } public Class Root { public Optional<Row> FindRow(List<Row> values, string key) => find_by(values, Other.Id, key); }`, code: CodeInvalidType},
		{name: "missing selector field", src: `public Record Row { public string Id; } public Class Root { public Optional<Row> FindRow(List<Row> values, string key) => find_by(values, Row.Missing, key); }`, code: CodeInvalidMember},
		{name: "non-string selector", src: `public Record Row { public int Id; } public Class Root { public Optional<Row> FindRow(List<Row> values, string key) => find_by(values, Row.Id, key); }`, code: CodeInvalidType},
		{name: "reordered parameters", src: `public Record Row { public string Id; } public Class Root { public Optional<Row> FindRow(string key, List<Row> values) => find_by(values, Row.Id, key); }`, code: CodeInvalidType},
		{name: "computed list", src: `public Record Row { public string Id; } public Class Root { public Optional<Row> FindRow(List<Row> values, Row value, string key) => find_by(append(values, value), Row.Id, key); }`, code: CodeInvalidType},
		{name: "computed key", src: `public Record Row { public string Id; } public Class Root { public Optional<Row> FindRow(List<Row> values, string key) => find_by(values, Row.Id, key + "x"); }`, code: CodeExpressionType},
		{name: "nested find", src: `public Record Row { public string Id; } public Class Root { public bool HasRow(List<Row> values, string key) => has_value(find_by(values, Row.Id, key)); }`, code: CodeInvalidType},
		{name: "list field", src: `public Record Row { public string Id; } public Class Root { public List<Row> Values; public Optional<Row> FindRow(List<Row> values, string key) => find_by(values, Row.Id, key); }`, code: CodeInvalidType},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			input := semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", "root.pipe", test.src)}, nil)
			input.LanguageContract = PipeLangLanguageContractV210
			analysis := AnalyzeSemanticModuleSet(input)
			if !analysis.Diagnostics.HasErrors() {
				t.Fatal("excluded v0.21.0 record-list find_by form was accepted")
			}
			if analysis.Diagnostics[0].Code != test.code {
				t.Fatalf("record-list find_by diagnostic = %s, want %s: %v", analysis.Diagnostics[0].Code, test.code, analysis.Error())
			}
		})
	}
}

func TestRecordListFindByTextRequiresExplicitV210Migration(t *testing.T) {
	source := `public Record Row { public string Id; } public Class Root { public Optional<Row> FindRow(List<Row> values, string key) => find_by(values, Row.Id, key); }`
	for _, contract := range []LanguageContract{PipeLangLanguageContractV010, PipeLangLanguageContractV020, PipeLangLanguageContractV030, PipeLangLanguageContractV040, PipeLangLanguageContractV050, PipeLangLanguageContractV060, PipeLangLanguageContractV070, PipeLangLanguageContractV080, PipeLangLanguageContractV090, PipeLangLanguageContractV100, PipeLangLanguageContractV110, PipeLangLanguageContractV120, PipeLangLanguageContractV130, PipeLangLanguageContractV140, PipeLangLanguageContractV150, PipeLangLanguageContractV160, PipeLangLanguageContractV170, PipeLangLanguageContractV180, PipeLangLanguageContractV190, PipeLangLanguageContractV200} {
		input := semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", "root.pipe", source)}, nil)
		input.LanguageContract = contract
		analysis := AnalyzeSemanticModuleSet(input)
		if !analysis.Diagnostics.HasErrors() {
			t.Fatalf("%s implicitly accepted record-list find_by", contract)
		}
	}
}

func TestV210PreservesV200RecordListAt(t *testing.T) {
	source, err := os.ReadFile("testdata/record-list-at.pipe")
	if err != nil {
		t.Fatal(err)
	}
	input := semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", "record-list-at.pipe", string(source))}, nil)
	input.LanguageContract = PipeLangLanguageContractV210
	analysis := AnalyzeSemanticModuleSet(input)
	if err := analysis.Error(); err != nil {
		t.Fatal(err)
	}
	typed, err := LowerSemanticMethodToHIR(analysis, semanticMethodNamed(t, analysis, "RowAt").Identity)
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

func v210RecordListFindByTextPipeline(t *testing.T) (*Analysis, hir.Program, coreir.Program, []byte) {
	t.Helper()
	source, err := os.ReadFile("testdata/record-list-find-by-text.pipe")
	if err != nil {
		t.Fatal(err)
	}
	input := semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", "record-list-find-by-text.pipe", string(source))}, nil)
	input.LanguageContract = PipeLangLanguageContractV210
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
	generated, err := gobackend.Generate(core)
	if err != nil {
		t.Fatal(err)
	}
	return analysis, typed, core, generated
}

func recordListFindByTextGeneratedGoTest() string {
	return fmt.Sprintf(`package %s

import "testing"

func expectRecordListFindByTextPanic(t *testing.T, invoke func()) {
	t.Helper()
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic")
		}
	}()
	invoke()
}

func TestGeneratedRecordListFindByText(t *testing.T) {
	rows := []PipeLangRecordTestPackageAppRootContainerrow{
		{Id: "container-1", Name: "first", Running: true},
		{Id: "container-1", Name: "duplicate"},
		{Id: "é", Name: "composed"},
		{Id: "e\u0301", Name: "decomposed"},
	}
	selected := PipeLangFindRow(rows, "container-1")
	selectedRow, present := selected.(pipelangOptionalSome[PipeLangRecordTestPackageAppRootContainerrow])
	if !present || selectedRow.value.Name != "first" {
		t.Fatal("stable-key first match mismatch")
	}
	if _, present := PipeLangFindRow(rows, "missing").(pipelangOptionalSome[PipeLangRecordTestPackageAppRootContainerrow]); present {
		t.Fatal("missing key unexpectedly selected a row")
	}
	composed, present := PipeLangFindRow(rows, "é").(pipelangOptionalSome[PipeLangRecordTestPackageAppRootContainerrow])
	if !present || composed.value.Name != "composed" {
		t.Fatal("composed key mismatch")
	}
	decomposed, present := PipeLangFindRow(rows, "e\u0301").(pipelangOptionalSome[PipeLangRecordTestPackageAppRootContainerrow])
	if !present || decomposed.value.Name != "decomposed" {
		t.Fatal("decomposed key mismatch")
	}
	invalid := append([]PipeLangRecordTestPackageAppRootContainerrow(nil), rows...)
	invalid[3].Id = string([]byte{0xff})
	expectRecordListFindByTextPanic(t, func() { PipeLangFindRow(invalid, "missing") })
	expectRecordListFindByTextPanic(t, func() { PipeLangFindRow(rows, string([]byte{0xff})) })
	var nilRows []PipeLangRecordTestPackageAppRootContainerrow
	expectRecordListFindByTextPanic(t, func() { PipeLangFindRow(nilRows, "missing") })
}
`, gobackend.PackageName)
}
