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

func TestV150RecordListHIRCoreAndGo(t *testing.T) {
	source, err := os.ReadFile("testdata/record-list.pipe")
	if err != nil {
		t.Fatal(err)
	}
	input := semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", "record-list.pipe", string(source))}, nil)
	input.LanguageContract = PipeLangLanguageContractV150
	analysis := AnalyzeSemanticModuleSet(input)
	if err := analysis.Error(); err != nil {
		t.Fatal(err)
	}

	projection, err := BuildSemanticProjection(analysis)
	if err != nil {
		t.Fatal(err)
	}
	if projection.LanguageContract != PipeLangLanguageContractV150 || projection.Schema != PipeLangSemanticProjectionVersion {
		t.Fatalf("record-list projection header = %#v", projection)
	}
	for _, name := range []string{"EmptyRows", "OneRow", "ForwardRows"} {
		method := projectedMemberNamed(t, projectedTypeNamed(t, projection, "Root"), name)
		if method.Type.Kind != TypeRefApplied || method.Type.Identity == nil || method.Type.Identity.PackageID != PipeLangBuiltinPackageID || method.Type.Identity.Path != PipeLangListSemanticPath || len(method.Type.Arguments) != 1 || method.Type.Arguments[0].Identity == nil || method.Type.Arguments[0].Identity.Path != "app.root.containerrow" {
			t.Fatalf("%s record-list projection = %#v", name, method)
		}
		if name == "ForwardRows" && (len(method.Parameters) != 1 || method.Parameters[0].Position != 0 || method.Parameters[0].Type.Identity == nil || method.Parameters[0].Type.Identity.PackageID != PipeLangBuiltinPackageID || method.Parameters[0].Type.Identity.Path != PipeLangListSemanticPath || len(method.Parameters[0].Type.Arguments) != 1 || method.Parameters[0].Type.Arguments[0].Identity == nil || method.Parameters[0].Type.Arguments[0].Identity.Path != "app.root.containerrow") {
			t.Fatalf("%s record-list parameter projection = %#v", name, method.Parameters)
		}
	}

	var functions []hir.Function
	typedByName := map[string]hir.Program{}
	for _, name := range []string{"MakeRow", "EmptyRows", "OneRow", "ForwardRows"} {
		typed, err := LowerSemanticMethodToHIR(analysis, semanticMethodNamed(t, analysis, name).Identity)
		if err != nil {
			t.Fatal(err)
		}
		typedByName[name] = typed
		functions = append(functions, typed.Functions[0])
	}
	oneTyped := typedByName["OneRow"]
	if oneTyped.LanguageContract != coreir.LanguageContractV150 || oneTyped.Functions[0].Body.Kind != hir.ExprListSingleton || oneTyped.Functions[0].Body.ListOne == nil || oneTyped.Functions[0].Body.ListOne.Value == nil || oneTyped.Functions[0].Body.ListOne.Value.Reference == nil || oneTyped.Functions[0].Body.ListOne.Value.Reference.Position != 0 || oneTyped.Functions[0].ReturnType.Kind != hir.TypeList || oneTyped.Functions[0].ReturnType.List == nil || oneTyped.Functions[0].ReturnType.List.Element.Kind != hir.TypeRecord {
		t.Fatalf("record-list singleton HIR = %#v", oneTyped)
	}
	if _, err := LowerSemanticMethodToHIR(analysis, SemanticIdentity{PackageID: "test.package", Path: "app.root.root.missing"}); err == nil {
		t.Fatal("HIR lowering accepted a missing record-list method")
	} else if diagnostics, ok := AsDiagnostics(err); !ok || len(diagnostics) != 1 || diagnostics[0].Code != CodeHIRLowering {
		t.Fatalf("missing record-list HIR diagnostic = %#v (%v)", diagnostics, err)
	}
	malformedHIR := oneTyped
	malformedHIR.Functions = append([]hir.Function(nil), oneTyped.Functions...)
	malformedHIR.Functions[0].Body = oneTyped.Functions[0].Body
	malformedList := *oneTyped.Functions[0].Body.ListOne
	malformedValue := *malformedList.Value
	malformedReference := *malformedValue.Reference
	malformedReference.Position = 1
	malformedValue.Reference = &malformedReference
	malformedList.Value = &malformedValue
	malformedHIR.Functions[0].Body.ListOne = &malformedList
	if _, err := LowerHIRToCore(malformedHIR); err == nil {
		t.Fatal("Core lowering accepted malformed record-list HIR")
	} else if diagnostics, ok := AsDiagnostics(err); !ok || len(diagnostics) != 1 || diagnostics[0].Code != CodeCoreLowering {
		t.Fatalf("malformed record-list HIR diagnostic = %#v (%v)", diagnostics, err)
	}

	program := hir.Program{LanguageContract: coreir.LanguageContractV150, CompilerContract: coreir.CompilerContractV1, Functions: functions}
	core, err := LowerHIRToCore(program)
	if err != nil {
		t.Fatal(err)
	}
	byName := map[string]coreir.Function{}
	for _, function := range core.Functions {
		byName[function.Name] = function
	}
	if byName["EmptyRows"].Body.Kind != coreir.ExprListEmpty || byName["OneRow"].Body.Kind != coreir.ExprListSingleton || byName["ForwardRows"].Body.Kind != coreir.ExprReference {
		t.Fatalf("record-list Core functions = %#v", byName)
	}

	rowOutcome, err := coreeval.Evaluate(byName["MakeRow"], []coreeval.Value{
		{Type: byName["MakeRow"].Parameters[0].Type, String: "container-1"},
		{Type: byName["MakeRow"].Parameters[1].Type, String: "api"},
		{Type: byName["MakeRow"].Parameters[2].Type, Bool: true},
	})
	if err != nil || !rowOutcome.OK {
		t.Fatalf("record construction = %#v (%v)", rowOutcome, err)
	}
	row := rowOutcome.Value
	empty, err := coreeval.Evaluate(byName["EmptyRows"], nil)
	if err != nil || !empty.OK || empty.Value.List == nil || len(empty.Value.List) != 0 {
		t.Fatalf("empty_list outcome = %#v (%v)", empty, err)
	}
	one, err := coreeval.Evaluate(byName["OneRow"], []coreeval.Value{row})
	if err != nil || !one.OK || len(one.Value.List) != 1 || one.Value.List[0].Record[0].String != "container-1" {
		t.Fatalf("list singleton outcome = %#v (%v)", one, err)
	}
	inputList := coreeval.Value{Type: byName["ForwardRows"].Parameters[0].Type, List: []coreeval.Value{row, row}}
	forwarded, err := coreeval.Evaluate(byName["ForwardRows"], []coreeval.Value{inputList})
	if err != nil || !forwarded.OK || len(forwarded.Value.List) != 2 {
		t.Fatalf("list transport outcome = %#v (%v)", forwarded, err)
	}
	inputList.List[0].Record[0].String = "mutated"
	if forwarded.Value.List[0].Record[0].String != "container-1" {
		t.Fatal("Core record-list transport leaked mutable slice or record storage")
	}
	invalidRow := row
	invalidRow.Record = append([]coreeval.Value(nil), row.Record...)
	invalidRow.Record[0].String = string([]byte{0xff})
	if _, err := coreeval.Evaluate(byName["OneRow"], []coreeval.Value{invalidRow}); err == nil {
		t.Fatal("Core evaluator accepted invalid UTF-8 in a list element")
	}
	if _, err := coreeval.Evaluate(byName["ForwardRows"], []coreeval.Value{{Type: inputList.Type}}); err == nil {
		t.Fatal("Core evaluator accepted a nil record-list representation")
	}

	priorHIR := oneTyped
	priorHIR.LanguageContract = coreir.LanguageContractV140
	if _, err := LowerHIRToCore(priorHIR); err == nil {
		t.Fatal("v0.14.0 HIR implicitly accepted record-list values")
	}
	oneCore, err := LowerHIRToCore(oneTyped)
	if err != nil {
		t.Fatal(err)
	}
	malformedCore := oneCore.Functions[0]
	malformedCore.Body = oneCore.Functions[0].Body
	malformedCoreList := *malformedCore.Body.ListOne
	malformedCoreList.Value = nil
	malformedCore.Body.ListOne = &malformedCoreList
	if err := coreir.ValidateFunction(malformedCore); err == nil {
		t.Fatal("Core accepted a malformed list singleton")
	}
	priorCore := oneCore
	priorCore.LanguageContract = coreir.LanguageContractV140
	if _, err := gobackend.Generate(priorCore); err == nil {
		t.Fatal("v0.14.0 Core implicitly accepted record-list values")
	}

	generated, err := gobackend.Generate(core)
	if err != nil {
		t.Fatal(err)
	}
	second, err := gobackend.Generate(core)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(generated, second) {
		t.Fatal("record-list generated Go is nondeterministic")
	}
	compileAndRunGeneratedGoFiles(t, generated, []byte(recordListGeneratedGoTest()))

	assertCompilerGolden(t, "record-list.hir.json", canonicalJSON(t, oneTyped))
	assertCompilerGolden(t, "record-list.core.json", canonicalJSON(t, oneCore))
	oneGenerated, err := gobackend.Generate(oneCore)
	if err != nil {
		t.Fatal(err)
	}
	assertCompilerGolden(t, "record-list.go", oneGenerated)
}

func TestV150ParserPreservesRecordListOperandsAndSpans(t *testing.T) {
	const source = `public Record Row { public string Id; } public Class Root { public List<Row> Empty() => empty_list<Row>(); public List<Row> One(Row value) => list(value); }`
	sources, diagnostics := NewSourceSet([]SourceInput{{Path: "root.pipe", Data: []byte(source)}})
	if diagnostics.HasErrors() {
		t.Fatal(diagnosticError(sources, diagnostics))
	}
	program, err := parseSourceFileWithLanguageContract(sources, sources.Files()[0], PipeLangLanguageContractV150)
	if err != nil {
		t.Fatal(err)
	}
	if len(program.Classes) != 1 || len(program.Classes[0].Methods) != 2 {
		t.Fatalf("record-list program = %#v", program)
	}
	empty, emptyOK := program.Classes[0].Methods[0].Body.(*ListEmptyExpr)
	one, oneOK := program.Classes[0].Methods[1].Body.(*ListSingletonExpr)
	value, valueOK := one.Value.(*IdentExpr)
	if !emptyOK || !empty.Span.IsValid() || empty.ElementType.Name != "Row" || !empty.ElementType.Span.IsValid() || !oneOK || !one.Span.IsValid() || !valueOK || value.Name != "value" || !value.Span.IsValid() {
		t.Fatalf("record-list AST = %#v / %#v", program.Classes[0].Methods[0].Body, program.Classes[0].Methods[1].Body)
	}
}

func TestV150RecordListRejectsExcludedForms(t *testing.T) {
	tests := []struct {
		name string
		src  string
		code DiagnosticCode
	}{
		{name: "primitive element", src: `public Class Root { public List<string> Values(string value) => list(value); }`, code: CodeExpressionType},
		{name: "nested list", src: `public Record Row { public string Id; } public Class Root { public List<List<Row>> Values(List<Row> value) => list(value); }`, code: CodeInvalidType},
		{name: "literal singleton", src: `public Record Row { public string Id; } public Class Root { public List<Row> Values(Row value) => list(new Row { Id = "fixed" }); }`, code: CodeExpressionType},
		{name: "wrong empty type", src: `public Record Row { public string Id; } public Record Other { public string Id; } public Class Root { public List<Row> Values() => empty_list<Other>(); }`, code: CodeExpressionType},
		{name: "extra singleton parameter", src: `public Record Row { public string Id; } public Class Root { public List<Row> Values(Row value, bool extra) => list(value); }`, code: CodeInvalidType},
		{name: "list field", src: `public Record Row { public string Id; } public Class Root { public List<Row> Values; public bool Ready() => true; }`, code: CodeInvalidType},
		{name: "record list field", src: `public Record Row { public string Id; } public Record Snapshot { public List<Row> Values; } public Class Root { public bool Ready() => true; }`, code: CodeInvalidType},
		{name: "optional element", src: `public Record Row { public string Id; } public Class Root { public List<Optional<Row>> Values() => empty_list<Optional<Row>>(); }`, code: CodeInvalidType},
		{name: "record-list equality", src: `public Record Row { public string Id; } public Class Root { public bool Same(List<Row> left, List<Row> right) => left == right; }`, code: CodeInvalidType},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			input := semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", "root.pipe", test.src)}, nil)
			input.LanguageContract = PipeLangLanguageContractV150
			analysis := AnalyzeSemanticModuleSet(input)
			if !analysis.Diagnostics.HasErrors() {
				t.Fatal("excluded v0.15.0 record-list form was accepted")
			}
			assertDiagnosticCode(t, analysis, test.code)
		})
	}
}

func TestRecordListRequiresExplicitV150Migration(t *testing.T) {
	source := `public Record Row { public string Id; } public Class Root { public List<Row> Values() => empty_list<Row>(); }`
	for _, contract := range []LanguageContract{PipeLangLanguageContractV010, PipeLangLanguageContractV020, PipeLangLanguageContractV030, PipeLangLanguageContractV040, PipeLangLanguageContractV050, PipeLangLanguageContractV060, PipeLangLanguageContractV070, PipeLangLanguageContractV080, PipeLangLanguageContractV090, PipeLangLanguageContractV100, PipeLangLanguageContractV110, PipeLangLanguageContractV120, PipeLangLanguageContractV130, PipeLangLanguageContractV140} {
		input := semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", "root.pipe", source)}, nil)
		input.LanguageContract = contract
		analysis := AnalyzeSemanticModuleSet(input)
		if !analysis.Diagnostics.HasErrors() {
			t.Fatalf("%s implicitly accepted record-list values", contract)
		}
	}
}

func TestV150PreservesFrozenV140Contracts(t *testing.T) {
	for _, test := range []struct {
		name   string
		source string
	}{
		{name: "Optional defaulting", source: `public Class Root { public string ValueOr(Optional<string> value, string fallback) => value_or(value, fallback); }`},
		{name: "checked divide", source: checkedDivideSource},
		{name: "Result transport", source: resultTransportIntSource},
		{name: "ordinal text", source: `public Class Root { public bool Before(string left, string right) => left < right; }`},
		{name: "record equality", source: `public Record Row { public string Id; } public Class Root { public bool Same(Row left, Row right) => left == right; }`},
	} {
		t.Run(test.name, func(t *testing.T) {
			input := semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", "root.pipe", test.source)}, nil)
			input.LanguageContract = PipeLangLanguageContractV150
			analysis := AnalyzeSemanticModuleSet(input)
			if err := analysis.Error(); err != nil {
				t.Fatal(err)
			}
			typed, err := LowerSemanticMethodToHIR(analysis, semanticMethod(t, analysis).Identity)
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
		})
	}
}

func recordListGeneratedGoTest() string {
	return fmt.Sprintf(`package %s

import "testing"

func expectListPanic(t *testing.T, invoke func()) {
	t.Helper()
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic")
		}
	}()
	invoke()
}

func TestGeneratedRecordList(t *testing.T) {
	row := PipeLangMakeRow("container-1", "api", true)
	empty := PipeLangEmptyRows()
	if empty == nil || len(empty) != 0 {
		t.Fatalf("empty = %%#v", empty)
	}
	one := PipeLangOneRow(row)
	if len(one) != 1 || one[0].Id != "container-1" {
		t.Fatalf("one = %%#v", one)
	}
	input := []PipeLangRecordTestPackageAppRootContainerrow{row, row}
	forwarded := PipeLangForwardRows(input)
	input[0].Id = "mutated"
	if len(forwarded) != 2 || forwarded[0].Id != "container-1" {
		t.Fatalf("forwarded = %%#v", forwarded)
	}
	invalid := row
	invalid.Id = string([]byte{0xff})
	expectListPanic(t, func() { PipeLangOneRow(invalid) })
	var nilRows []PipeLangRecordTestPackageAppRootContainerrow
	expectListPanic(t, func() { PipeLangForwardRows(nilRows) })
}
`, gobackend.PackageName)
}
