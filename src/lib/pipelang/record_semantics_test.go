package pipelang

import (
	"bytes"
	"fmt"
	"reflect"
	"testing"

	"dockpipe/src/lib/pipelang/coreeval"
	"dockpipe/src/lib/pipelang/coreir"
	"dockpipe/src/lib/pipelang/gobackend"
	"dockpipe/src/lib/pipelang/hir"
)

const primitiveRecordTransportSource = `public Record Row {
    public string Id;
    public int Count;
    public float Ratio;
    public bool Ready;
}
public Class Root {
    public Row Forward(Row value) => value;
}`

func TestV090PrimitiveRecordTransportHIRCoreAndGo(t *testing.T) {
	input := semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", "root.pipe", primitiveRecordTransportSource)}, nil)
	input.LanguageContract = PipeLangLanguageContractV090
	analysis := AnalyzeSemanticModuleSet(input)
	if err := analysis.Error(); err != nil {
		t.Fatal(err)
	}

	record := semanticDeclarationNamed(t, analysis, SemanticRecord, "Row")
	method := semanticMethodNamed(t, analysis, "Forward")
	if record.Identity.PackageID != "test.package" || record.Identity.Path != "app.root.row" {
		t.Fatalf("record identity = %#v", record.Identity)
	}
	if method.Identity.Callable == nil || len(method.Identity.Callable.Parameters) != 1 || !reflect.DeepEqual(method.Identity.Callable.Parameters[0], method.Identity.Callable.Returns) || method.Identity.Callable.Returns.Kind != TypeRefNamed || method.Identity.Callable.Returns.PackageID != record.Identity.PackageID || method.Identity.Callable.Returns.Path != record.Identity.Path {
		t.Fatalf("record transport callable identity = %#v", method.Identity.Callable)
	}

	projection, err := BuildSemanticProjection(analysis)
	if err != nil {
		t.Fatal(err)
	}
	projectedRecord := projectedTypeNamed(t, projection, "Row")
	projectedMethod := projectedMemberNamed(t, projectedTypeNamed(t, projection, "Root"), "Forward")
	if projection.Schema != PipeLangSemanticProjectionVersion || projection.LanguageContract != PipeLangLanguageContractV090 || projectedRecord.Kind != SemanticRecord || len(projectedRecord.Members) != 4 || projectedMethod.Type.Identity == nil || projectedMethod.Type.Identity.Path != record.Identity.Path || len(projectedMethod.Parameters) != 1 || !reflect.DeepEqual(projectedMethod.Parameters[0].Type, projectedMethod.Type) {
		t.Fatalf("record semantic projection = %#v / %#v", projectedRecord, projectedMethod)
	}
	for index, name := range []string{"Count", "Id", "Ratio", "Ready"} {
		if projectedRecord.Members[index].Name != name || projectedRecord.Members[index].Kind != SemanticField || projectedRecord.Members[index].Identity.Path == "" {
			t.Fatalf("record semantic projection field %d = %#v", index, projectedRecord.Members[index])
		}
	}

	typed, err := LowerSemanticMethodToHIR(analysis, method.Identity)
	if err != nil {
		t.Fatal(err)
	}
	if typed.LanguageContract != coreir.LanguageContractV090 || len(typed.Functions) != 1 || len(typed.Functions[0].Parameters) != 1 || typed.Functions[0].ReturnType.Kind != hir.TypeRecord || typed.Functions[0].ReturnType.Record == nil || len(typed.Functions[0].ReturnType.Record.Fields) != 4 || typed.Functions[0].Body.Kind != hir.ExprReference || typed.Functions[0].Body.Reference == nil || typed.Functions[0].Body.Reference.Position != 0 || !reflect.DeepEqual(typed.Functions[0].Parameters[0].Type, typed.Functions[0].ReturnType) || !reflect.DeepEqual(typed.Functions[0].Body.Type, typed.Functions[0].ReturnType) {
		t.Fatalf("record transport HIR = %#v", typed)
	}
	for index, name := range []string{"Id", "Count", "Ratio", "Ready"} {
		if typed.Functions[0].ReturnType.Record.Fields[index].Name != name || typed.Functions[0].ReturnType.Record.Fields[index].Identity.Path == "" {
			t.Fatalf("record HIR field %d = %#v", index, typed.Functions[0].ReturnType.Record.Fields[index])
		}
	}

	core, err := LowerHIRToCore(typed)
	if err != nil {
		t.Fatal(err)
	}
	if len(core.Functions) != 1 || core.Functions[0].ReturnType.Kind != coreir.TypeRecord || core.Functions[0].Body.Parameter == nil || *core.Functions[0].Body.Parameter != 0 || !coreir.TypeEqual(core.Functions[0].Parameters[0].Type, core.Functions[0].ReturnType) {
		t.Fatalf("record transport Core = %#v", core)
	}
	malformed := core.Functions[0]
	malformed.Parameters = append([]coreir.Parameter(nil), malformed.Parameters...)
	malformed.Parameters[0].Type = coreir.Type{Kind: coreir.TypeRecord, Name: "Row", Identity: malformed.Parameters[0].Type.Identity}
	if err := coreir.ValidateFunction(malformed); err == nil {
		t.Fatal("Core accepted a record parameter without its field schema")
	}
	if _, err := gobackend.Generate(coreir.Program{LanguageContract: coreir.LanguageContractV090, CompilerContract: coreir.CompilerContractV1, Functions: []coreir.Function{malformed}}); err == nil {
		t.Fatal("Go backend accepted a malformed Core record")
	}
	malformedIdentity := core.Functions[0]
	malformedIdentity.Parameters = append([]coreir.Parameter(nil), malformedIdentity.Parameters...)
	malformedIdentity.Parameters[0].Type.Record = &coreir.RecordType{Fields: append([]coreir.RecordField(nil), malformedIdentity.Parameters[0].Type.Record.Fields...)}
	malformedIdentity.Parameters[0].Type.Record.Fields[0].Identity.Path = "outside.record.field"
	if err := coreir.ValidateFunction(malformedIdentity); err == nil {
		t.Fatal("Core accepted a record field identity outside its record identity")
	}
	recordType := core.Functions[0].ReturnType
	argument := coreeval.Value{Type: recordType, Record: []coreeval.Value{
		{Type: recordType.Record.Fields[0].Type, String: "row-1"},
		{Type: recordType.Record.Fields[1].Type, Int: 42},
		{Type: recordType.Record.Fields[2].Type, Float: 1.5},
		{Type: recordType.Record.Fields[3].Type, Bool: true},
	}}
	outcome, err := coreeval.Evaluate(core.Functions[0], []coreeval.Value{argument})
	if err != nil || !outcome.OK || outcome.Error != "" || !reflect.DeepEqual(outcome.Value, argument) {
		t.Fatalf("record Core outcome = %#v (%v)", outcome, err)
	}
	argument.Record[0].String = "mutated-after-call"
	if outcome.Value.Record[0].String != "row-1" {
		t.Fatal("Core record transport leaked mutable field-vector aliasing")
	}
	for _, invalid := range []coreeval.Value{
		{Type: recordType},
		{Type: recordType, Record: []coreeval.Value{{Type: recordType.Record.Fields[0].Type, String: "missing-fields"}}},
		{Type: recordType, Record: []coreeval.Value{{Type: recordType.Record.Fields[0].Type, String: string([]byte{0xff})}, {Type: recordType.Record.Fields[1].Type}, {Type: recordType.Record.Fields[2].Type}, {Type: recordType.Record.Fields[3].Type}}},
	} {
		if _, evaluateErr := coreeval.Evaluate(core.Functions[0], []coreeval.Value{invalid}); evaluateErr == nil {
			t.Fatalf("malformed Core record was accepted: %#v", invalid)
		}
	}

	generated, err := gobackend.Generate(core)
	if err != nil {
		t.Fatal(err)
	}
	secondGenerated, err := gobackend.Generate(core)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(generated, secondGenerated) {
		t.Fatal("record transport generated Go is nondeterministic")
	}
	assertCompilerGolden(t, "record-transport.hir.json", canonicalJSON(t, typed))
	assertCompilerGolden(t, "record-transport.core.json", canonicalJSON(t, core))
	assertCompilerGolden(t, "record-transport.go", generated)
	compileAndRunRecordTransportGo(t, generated, gobackend.FunctionName(core.Functions[0]))
}

func TestV090PreservesEarlierArithmeticResultAndTextContracts(t *testing.T) {
	for _, test := range []struct {
		name   string
		source string
	}{
		{name: "checked add", source: checkedAddSource},
		{name: "checked subtract", source: checkedSubtractSource},
		{name: "checked multiply", source: checkedMultiplySource},
		{name: "checked negate", source: checkedNegateSource},
		{name: "checked divide", source: checkedDivideSource},
		{name: "Result transport", source: resultTransportIntSource},
		{name: "ordinal text", source: `public Class Root { public bool Before(string left, string right) => left < right; }`},
	} {
		t.Run(test.name, func(t *testing.T) {
			input := semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", "root.pipe", test.source)}, nil)
			input.LanguageContract = PipeLangLanguageContractV090
			analysis := AnalyzeSemanticModuleSet(input)
			if err := analysis.Error(); err != nil {
				t.Fatal(err)
			}
			method := semanticMethod(t, analysis)
			typed, err := LowerSemanticMethodToHIR(analysis, method.Identity)
			if err != nil {
				t.Fatal(err)
			}
			core, err := LowerHIRToCore(typed)
			if err != nil {
				t.Fatal(err)
			}
			if core.LanguageContract != coreir.LanguageContractV090 {
				t.Fatalf("Core contract = %q", core.LanguageContract)
			}
			if _, err := gobackend.Generate(core); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestRecordRemainsAContextualPostLegacyKeyword(t *testing.T) {
	program, err := ParseFile("legacy.pipe", []byte(`public Class Record { public string Name; }`))
	if err != nil {
		t.Fatal(err)
	}
	if len(program.Classes) != 1 || program.Classes[0].Name != "Record" || len(program.Records) != 0 {
		t.Fatalf("legacy contextual identifier parsed as %#v", program)
	}
}

func TestV090PrimitiveRecordRejectsExcludedForms(t *testing.T) {
	tests := []struct {
		name string
		src  string
		code DiagnosticCode
	}{
		{name: "empty record", src: `public Record Row {} public Class Root { public bool Ready() => true; }`, code: CodeInvalidDecl},
		{name: "private record", src: `private Record Row { public string Id; } public Class Root { public bool Ready() => true; }`, code: CodeInvalidDecl},
		{name: "annotated record", src: `[kind="row"] public Record Row { public string Id; } public Class Root { public bool Ready() => true; }`, code: CodeInvalidDecl},
		{name: "record implements", src: `public Interface Named { public string Id; } public Record Row : Named { public string Id; } public Class Root { public bool Ready() => true; }`, code: CodeInvalidDecl},
		{name: "record method", src: `public Record Row { public string Id; public bool Ready() => true; } public Class Root { public bool Ready() => true; }`, code: CodeInvalidMember},
		{name: "private field", src: `public Record Row { private string Id; } public Class Root { public bool Ready() => true; }`, code: CodeInvalidMember},
		{name: "annotated field", src: `public Record Row { [kind="id"] public string Id; } public Class Root { public bool Ready() => true; }`, code: CodeInvalidMember},
		{name: "field default", src: `public Record Row { public string Id = ""; } public Class Root { public bool Ready() => true; }`, code: CodeInvalidMember},
		{name: "named record field", src: `public Record Inner { public string Id; } public Record Outer { public Inner Value; } public Class Root { public bool Ready() => true; }`, code: CodeInvalidType},
		{name: "list record field", src: `public Record Row { public List<string> Values; } public Class Root { public bool Ready() => true; }`, code: CodeInvalidType},
		{name: "class record field", src: `public Record Row { public string Id; } public Class Root { public Row Value; public bool Ready() => true; }`, code: CodeInvalidType},
		{name: "interface record", src: `public Record Row { public string Id; } public Interface Transport { public Row Forward(Row value); } public Class Root { public bool Ready() => true; }`, code: CodeInvalidType},
		{name: "extra parameter", src: `public Record Row { public string Id; } public Class Root { public Row Forward(Row value, int extra) => value; }`, code: CodeInvalidType},
		{name: "different return", src: `public Record Row { public string Id; } public Class Root { public bool Forward(Row value) => true; }`, code: CodeInvalidType},
		{name: "different body", src: `public Record Row { public string Id; } public Class Root { public Row Forward(Row value) => missing; }`, code: CodeExpressionType},
		{name: "record equality", src: `public Record Row { public string Id; } public Class Root { public bool Equal(Row left, Row right) => left == right; }`, code: CodeInvalidType},
		{name: "record in Result", src: `public Record Row { public string Id; } public Class Root { public Result<Row, ArithmeticError> Forward(Row value) => value; }`, code: CodeInvalidType},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			input := semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", "root.pipe", test.src)}, nil)
			input.LanguageContract = PipeLangLanguageContractV090
			analysis := AnalyzeSemanticModuleSet(input)
			if !analysis.Diagnostics.HasErrors() {
				t.Fatal("excluded v0.9.0 primitive record form was accepted")
			}
			assertDiagnosticCode(t, analysis, test.code)
		})
	}
}

func TestPrimitiveRecordRequiresExplicitV090Migration(t *testing.T) {
	for _, contract := range []LanguageContract{PipeLangLanguageContractV010, PipeLangLanguageContractV020, PipeLangLanguageContractV030, PipeLangLanguageContractV040, PipeLangLanguageContractV050, PipeLangLanguageContractV060, PipeLangLanguageContractV070, PipeLangLanguageContractV080} {
		input := semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", "root.pipe", primitiveRecordTransportSource)}, nil)
		input.LanguageContract = contract
		analysis := AnalyzeSemanticModuleSet(input)
		if !analysis.Diagnostics.HasErrors() {
			t.Fatalf("%s implicitly accepted primitive records", contract)
		}
		assertDiagnosticCode(t, analysis, CodeUnexpectedToken)
	}
}

func semanticDeclarationNamed(t *testing.T, analysis *Analysis, kind SemanticKind, name string) SemanticDeclaration {
	t.Helper()
	for _, declaration := range analysis.SemanticIDs.Declarations() {
		if declaration.Kind == kind && declaration.Name == name {
			return declaration
		}
	}
	t.Fatalf("missing %s %q", kind, name)
	return SemanticDeclaration{}
}

func projectedTypeNamed(t *testing.T, projection *SemanticProjection, name string) SemanticTypeProjection {
	t.Helper()
	for _, module := range projection.Modules {
		for _, projected := range module.Types {
			if projected.Name == name {
				return projected
			}
		}
	}
	t.Fatalf("missing projected type %q", name)
	return SemanticTypeProjection{}
}

func compileAndRunRecordTransportGo(t *testing.T, generated []byte, functionName string) {
	t.Helper()
	testSource := fmt.Sprintf(`package %s

import "testing"

func TestGeneratedRecordTransport(t *testing.T) {
	value := PipeLangRecordTestPackageAppRootRow{Id: "row-1", Count: 42, Ratio: 1.5, Ready: true}
	if got := %s(value); got != value {
		t.Fatalf("got %%#v", got)
	}
	defer func() {
		if recover() == nil {
			t.Fatal("invalid UTF-8 record field did not fail")
		}
	}()
	value.Id = string([]byte{0xff})
	_ = %s(value)
}
`, gobackend.PackageName, functionName, functionName)
	compileAndRunGeneratedGoFiles(t, generated, []byte(testSource))
}
