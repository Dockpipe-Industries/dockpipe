package pipelang

import (
	"bytes"
	"fmt"
	"os"
	"reflect"
	"testing"

	"dockpipe/src/lib/pipelang/coreeval"
	"dockpipe/src/lib/pipelang/coreir"
	"dockpipe/src/lib/pipelang/gobackend"
	"dockpipe/src/lib/pipelang/hir"
)

func TestV110PrimitiveRecordConstructionHIRCoreAndGo(t *testing.T) {
	source, err := os.ReadFile("testdata/record-construction.pipe")
	if err != nil {
		t.Fatal(err)
	}
	input := semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", "record-construction.pipe", string(source))}, nil)
	input.LanguageContract = PipeLangLanguageContractV110
	analysis := AnalyzeSemanticModuleSet(input)
	if err := analysis.Error(); err != nil {
		t.Fatal(err)
	}

	record := semanticDeclarationNamed(t, analysis, SemanticRecord, "Row")
	method := semanticMethodNamed(t, analysis, "Create")
	if method.Identity.Callable == nil || len(method.Identity.Callable.Parameters) != 4 || method.Identity.Callable.Returns.Kind != TypeRefNamed || method.Identity.Callable.Returns.PackageID != record.Identity.PackageID || method.Identity.Callable.Returns.Path != record.Identity.Path {
		t.Fatalf("record construction callable identity = %#v", method.Identity.Callable)
	}
	for index, primitive := range []PrimitiveType{TypeString, TypeInt, TypeFloat, TypeBool} {
		if method.Identity.Callable.Parameters[index].Kind != TypeRefPrimitive || method.Identity.Callable.Parameters[index].Primitive != primitive {
			t.Fatalf("record construction parameter identity %d = %#v", index, method.Identity.Callable.Parameters[index])
		}
	}

	projection, err := BuildSemanticProjection(analysis)
	if err != nil {
		t.Fatal(err)
	}
	projectedRecord := projectedTypeNamed(t, projection, "Row")
	projectedMethod := projectedMemberNamed(t, projectedTypeNamed(t, projection, "Root"), "Create")
	if projection.Schema != PipeLangSemanticProjectionVersion || projection.LanguageContract != PipeLangLanguageContractV110 || len(projectedRecord.Members) != 4 || len(projectedMethod.Parameters) != 4 || projectedMethod.Type.Identity == nil || projectedMethod.Type.Identity.Path != record.Identity.Path {
		t.Fatalf("record construction semantic projection = %#v / %#v", projectedRecord, projectedMethod)
	}

	typed, err := LowerSemanticMethodToHIR(analysis, method.Identity)
	if err != nil {
		t.Fatal(err)
	}
	function := typed.Functions[0]
	construction := function.Body.Record
	if typed.LanguageContract != coreir.LanguageContractV110 || function.Body.Kind != hir.ExprRecordConstruct || construction == nil || construction.Identity.PackageID != string(record.Identity.PackageID) || construction.Identity.Path != string(record.Identity.Path) || len(construction.Fields) != 4 || !reflect.DeepEqual(function.Body.Type, function.ReturnType) {
		t.Fatalf("record construction HIR = %#v", typed)
	}
	for position, field := range construction.Fields {
		if field.Position != position || field.Name != []string{"Id", "Count", "Ratio", "Ready"}[position] || field.Identity.Path == "" || field.Value == nil || field.Value.Kind != hir.ExprReference || field.Value.Reference == nil || field.Value.Reference.Position != position {
			t.Fatalf("record construction HIR field %d = %#v", position, field)
		}
	}

	malformedHIR := typed
	malformedHIR.Functions = append([]hir.Function(nil), typed.Functions...)
	malformedHIR.Functions[0].Body = typed.Functions[0].Body
	malformedHIRRecord := *typed.Functions[0].Body.Record
	malformedHIRRecord.Fields = append([]hir.RecordConstructField(nil), malformedHIRRecord.Fields...)
	malformedHIRRecord.Fields[0].Value = nil
	malformedHIR.Functions[0].Body.Record = &malformedHIRRecord
	_, malformedHIRErr := LowerHIRToCore(malformedHIR)
	malformedHIRDiagnostics, ok := AsDiagnostics(malformedHIRErr)
	if !ok || len(malformedHIRDiagnostics) != 1 || malformedHIRDiagnostics[0].Code != CodeCoreLowering {
		t.Fatalf("malformed record-construction HIR error = %#v (%v)", malformedHIRDiagnostics, malformedHIRErr)
	}

	core, err := LowerHIRToCore(typed)
	if err != nil {
		t.Fatal(err)
	}
	coreFunction := core.Functions[0]
	coreConstruction := coreFunction.Body.Record
	if core.LanguageContract != coreir.LanguageContractV110 || coreFunction.Body.Kind != coreir.ExprRecordConstruct || coreConstruction == nil || len(coreConstruction.Fields) != 4 || !coreir.TypeEqual(coreFunction.Body.Type, coreFunction.ReturnType) {
		t.Fatalf("record construction Core = %#v", core)
	}
	for _, mutate := range []func(*coreir.Expr){
		func(body *coreir.Expr) { body.Record.Identity.Path = "outside.record" },
		func(body *coreir.Expr) { body.Record.Fields[0].Position = 1 },
		func(body *coreir.Expr) { body.Record.Fields[0].Name = "Missing" },
		func(body *coreir.Expr) { body.Record.Fields[0].Identity.Path = "outside.record.field" },
		func(body *coreir.Expr) { body.Record.Fields[0].Value = nil },
		func(body *coreir.Expr) {
			body.Record.Fields[0].Value = &coreir.Expr{Kind: coreir.ExprLiteral, Type: body.Record.Fields[0].Value.Type, Literal: &coreir.Literal{String: "fixed"}}
		},
		func(body *coreir.Expr) { body.Record.Fields = body.Record.Fields[:3] },
	} {
		malformed := cloneCoreFunctionForRecordConstruction(coreFunction)
		mutate(&malformed.Body)
		if err := coreir.ValidateFunction(malformed); err == nil {
			t.Fatal("Core accepted malformed record construction")
		}
	}
	nested := cloneCoreFunctionForRecordConstruction(coreFunction)
	constructed := nested.Body
	field := constructed.Type.Record.Fields[0]
	nested.ReturnType = field.Type
	nested.Body = coreir.Expr{Kind: coreir.ExprFieldProjection, Type: field.Type, Field: &coreir.FieldProjection{Receiver: &constructed, Identity: field.Identity, Name: field.Name, Position: 0}}
	if err := coreir.ValidateFunction(nested); err == nil {
		t.Fatal("Core accepted record construction nested under field projection")
	}

	arguments := []coreeval.Value{
		{Type: coreFunction.Parameters[0].Type, String: "row-1"},
		{Type: coreFunction.Parameters[1].Type, Int: 42},
		{Type: coreFunction.Parameters[2].Type, Float: 1.5},
		{Type: coreFunction.Parameters[3].Type, Bool: true},
	}
	outcome, err := coreeval.Evaluate(coreFunction, arguments)
	if err != nil || !outcome.OK || outcome.Error != "" || !coreir.TypeEqual(outcome.Value.Type, coreFunction.ReturnType) || len(outcome.Value.Record) != 4 || outcome.Value.Record[0].String != "row-1" || outcome.Value.Record[1].Int != 42 || outcome.Value.Record[2].Float != 1.5 || !outcome.Value.Record[3].Bool {
		t.Fatalf("record construction Core outcome = %#v (%v)", outcome, err)
	}
	arguments[0].String = "changed-after-call"
	if outcome.Value.Record[0].String != "row-1" {
		t.Fatal("Core record construction leaked parameter aliasing")
	}
	invalidArguments := append([]coreeval.Value(nil), arguments...)
	invalidArguments[0].String = string([]byte{0xff})
	if _, err := coreeval.Evaluate(coreFunction, invalidArguments); err == nil {
		t.Fatal("Core evaluator accepted invalid UTF-8 record construction input")
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
		t.Fatal("record construction generated Go is nondeterministic")
	}
	assertCompilerGolden(t, "record-construction.hir.json", canonicalJSON(t, typed))
	assertCompilerGolden(t, "record-construction.core.json", canonicalJSON(t, core))
	assertCompilerGolden(t, "record-construction.go", generated)
	compileAndRunRecordConstructionGo(t, generated, gobackend.FunctionName(coreFunction))
}

func TestV110PrimitiveRecordConstructionRejectsExcludedForms(t *testing.T) {
	tests := []struct {
		name string
		src  string
		code DiagnosticCode
	}{
		{name: "missing field", src: `public Record Row { public string Id; public int Count; } public Class Root { public Row Create(string id, int count) => new Row { Id = id }; }`, code: CodeInvalidType},
		{name: "extra field", src: `public Record Row { public string Id; } public Class Root { public Row Create(string id, int extra) => new Row { Id = id, Extra = extra }; }`, code: CodeInvalidType},
		{name: "duplicate field", src: `public Record Row { public string Id; public int Count; } public Class Root { public Row Create(string id, int count) => new Row { Id = id, Id = count }; }`, code: CodeDuplicateMember},
		{name: "unknown field", src: `public Record Row { public string Id; public int Count; } public Class Root { public Row Create(string id, int count) => new Row { Id = id, Missing = count }; }`, code: CodeInvalidMember},
		{name: "reordered fields", src: `public Record Row { public string Id; public int Count; } public Class Root { public Row Create(string id, int count) => new Row { Count = count, Id = id }; }`, code: CodeInvalidType},
		{name: "wrong parameter mapping", src: `public Record Row { public string Id; public string Name; } public Class Root { public Row Create(string id, string name) => new Row { Id = name, Name = id }; }`, code: CodeExpressionType},
		{name: "literal value", src: `public Record Row { public string Id; } public Class Root { public Row Create(string id) => new Row { Id = "fixed" }; }`, code: CodeExpressionType},
		{name: "wrong parameter type", src: `public Record Row { public string Id; } public Class Root { public Row Create(int id) => new Row { Id = id }; }`, code: CodeInvalidType},
		{name: "different record return", src: `public Record Row { public string Id; } public Record Other { public string Id; } public Class Root { public Row Create(string id) => new Other { Id = id }; }`, code: CodeInvalidType},
		{name: "construction projection", src: `public Record Row { public string Id; } public Class Root { public string Create(string id) => new Row { Id = id }.Id; }`, code: CodeExpressionType},
		{name: "record equality", src: `public Record Row { public string Id; } public Class Root { public bool Equal(Row left, Row right) => left == right; }`, code: CodeInvalidType},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			input := semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", "root.pipe", test.src)}, nil)
			input.LanguageContract = PipeLangLanguageContractV110
			analysis := AnalyzeSemanticModuleSet(input)
			if !analysis.Diagnostics.HasErrors() {
				t.Fatal("excluded v0.11.0 record form was accepted")
			}
			assertDiagnosticCode(t, analysis, test.code)
		})
	}
}

func TestPrimitiveRecordConstructionRequiresExplicitV110Migration(t *testing.T) {
	source := `public Record Row { public string Id; } public Class Root { public Row Create(string id) => new Row { Id = id }; }`
	for _, contract := range []LanguageContract{PipeLangLanguageContractV010, PipeLangLanguageContractV020, PipeLangLanguageContractV030, PipeLangLanguageContractV040, PipeLangLanguageContractV050, PipeLangLanguageContractV060, PipeLangLanguageContractV070, PipeLangLanguageContractV080, PipeLangLanguageContractV090, PipeLangLanguageContractV100} {
		input := semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", "root.pipe", source)}, nil)
		input.LanguageContract = contract
		analysis := AnalyzeSemanticModuleSet(input)
		if !analysis.Diagnostics.HasErrors() {
			t.Fatalf("%s implicitly accepted primitive record construction", contract)
		}
	}
}

func TestV110PreservesFrozenArithmeticResultTextAndRecordContracts(t *testing.T) {
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
		{name: "record identity transport", source: primitiveRecordTransportSource},
		{name: "record field projection", source: `public Record Row { public string Id; } public Class Root { public string IdOf(Row value) => value.Id; }`},
	} {
		t.Run(test.name, func(t *testing.T) {
			input := semanticTestModuleSet("app.root", []ModuleInput{testModule("app.root", "root.pipe", test.source)}, nil)
			input.LanguageContract = PipeLangLanguageContractV110
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
			if core.LanguageContract != coreir.LanguageContractV110 {
				t.Fatalf("Core contract = %q", core.LanguageContract)
			}
			if _, err := gobackend.Generate(core); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func cloneCoreFunctionForRecordConstruction(function coreir.Function) coreir.Function {
	cloned := function
	cloned.Body = function.Body
	record := *function.Body.Record
	record.Fields = append([]coreir.RecordConstructField(nil), function.Body.Record.Fields...)
	for index := range record.Fields {
		if record.Fields[index].Value != nil {
			value := *record.Fields[index].Value
			record.Fields[index].Value = &value
		}
	}
	cloned.Body.Record = &record
	return cloned
}

func compileAndRunRecordConstructionGo(t *testing.T, generated []byte, functionName string) {
	t.Helper()
	testSource := fmt.Sprintf(`package %s

import "testing"

func TestGeneratedRecordConstruction(t *testing.T) {
	got := %s("row-1", 42, 1.5, true)
	if got.Id != "row-1" || got.Count != 42 || got.Ratio != 1.5 || !got.Ready {
		t.Fatalf("got %%#v", got)
	}
	defer func() {
		if recover() == nil {
			t.Fatal("invalid UTF-8 record field did not fail")
		}
	}()
	_ = %s(string([]byte{0xff}), 42, 1.5, true)
}
`, gobackend.PackageName, functionName, functionName)
	compileAndRunGeneratedGoFiles(t, generated, []byte(testSource))
}
