package pipelang

import "testing"

func TestParseCreatesStructuredNestedTypeRefsWithSpans(t *testing.T) {
	program, err := ParseFile("nested.pipe", []byte(`Class Types { List<List<string>> Matrix; }`))
	if err != nil {
		t.Fatal(err)
	}
	ref := program.Classes[0].Fields[0].Type
	if ref.Kind != TypeRefApplied || ref.String() != "List<List<string>>" || ref.Span.File != "nested.pipe" {
		t.Fatalf("outer ref=%#v", ref)
	}
	inner, ok := ref.ListElementType()
	if !ok || inner.Kind != TypeRefApplied || inner.Span.Start <= ref.Span.Start || inner.Span.End >= ref.Span.End {
		t.Fatalf("inner ref=%#v", inner)
	}
	primitive, ok := inner.ListElementType()
	if !ok || primitive.Kind != TypeRefPrimitive || primitive.Name != "string" || !primitive.Span.IsValid() {
		t.Fatalf("primitive ref=%#v", primitive)
	}
}

func TestAnalyzeResolvesNamedTypesThroughOneOwnedSymbolTable(t *testing.T) {
	analysis := AnalyzeFiles(map[string][]byte{
		"a.pipe": []byte(`Interface Item { string Name; }`),
		"b.pipe": []byte(`Class Items : Item { string Name = ""; List<Item> Values; }`),
	})
	if err := analysis.Error(); err != nil {
		t.Fatal(err)
	}
	symbols := analysis.Symbols.Symbols()
	if len(symbols) != 2 || symbols[0].Name != "Item" || symbols[1].Name != "Items" {
		t.Fatalf("symbols=%#v", symbols)
	}
	for _, symbol := range symbols {
		if symbol.ID == 0 || symbol.Owner != legacySourceSetOwner || !symbol.DeclarationSpan.IsValid() {
			t.Fatalf("symbol ownership=%#v", symbol)
		}
	}
	item, ok := analysis.Symbols.Lookup("Item")
	if !ok {
		t.Fatal("missing Item symbol")
	}
	resolvedOwner, err := analysis.ResolveType(*analysis.Program.Classes[0].Implements)
	if err != nil {
		t.Fatal(err)
	}
	if resolvedOwner.Symbol != item.ID {
		t.Fatalf("implements=%#v item=%#v", resolvedOwner, item)
	}
	resolved, err := analysis.ResolveType(analysis.Program.Classes[0].Fields[1].Type)
	if err != nil {
		t.Fatal(err)
	}
	if resolved.Kind != TypeRefApplied || len(resolved.Arguments) != 1 || resolved.Arguments[0].Symbol != item.ID {
		t.Fatalf("resolved=%#v item=%#v", resolved, item)
	}
}

func TestAnalyzeRejectsCrossKindDuplicateInDeterministicSymbolOrder(t *testing.T) {
	analysis := AnalyzeFiles(map[string][]byte{
		"b.pipe": []byte(`Interface Same { string Name; }`),
		"a.pipe": []byte(`Class Same { string Name = "a"; }`),
	})
	if len(analysis.Diagnostics) != 1 {
		t.Fatalf("diagnostics=%#v", analysis.Diagnostics)
	}
	diagnostic := analysis.Diagnostics[0]
	if diagnostic.Code != CodeDuplicateDecl || diagnostic.Primary.File != "b.pipe" || len(diagnostic.Related) != 1 || diagnostic.Related[0].Span.File != "a.pipe" {
		t.Fatalf("diagnostic=%#v", diagnostic)
	}
}

func TestAnalyzeUnknownTypeUsesTypeSpanAndDeclarationRelation(t *testing.T) {
	analysis := AnalyzeFiles(map[string][]byte{
		"unknown.pipe": []byte(`Class Example { Missing Value; }`),
	})
	if len(analysis.Diagnostics) != 1 {
		t.Fatalf("diagnostics=%#v", analysis.Diagnostics)
	}
	diagnostic := analysis.Diagnostics[0]
	field := analysis.Program.Classes[0].Fields[0]
	if diagnostic.Code != CodeInvalidType || diagnostic.Primary != field.Type.Span || len(diagnostic.Related) != 1 || diagnostic.Related[0].Span != field.Span {
		t.Fatalf("diagnostic=%#v field=%#v", diagnostic, field)
	}
}

func TestConformanceMismatchUsesBothTypeSpans(t *testing.T) {
	analysis := AnalyzeFiles(map[string][]byte{
		"types.pipe": []byte(`Interface Shape { string Value; } Class Bad : Shape { int Value = 1; }`),
	})
	if len(analysis.Diagnostics) != 1 {
		t.Fatalf("diagnostics=%#v", analysis.Diagnostics)
	}
	diagnostic := analysis.Diagnostics[0]
	required := analysis.Program.Interfaces[0].Fields[0].Type.Span
	actual := analysis.Program.Classes[0].Fields[0].Type.Span
	if diagnostic.Code != CodeConformance || diagnostic.Primary != actual || len(diagnostic.Related) != 1 || diagnostic.Related[0].Span != required {
		t.Fatalf("diagnostic=%#v", diagnostic)
	}
}
