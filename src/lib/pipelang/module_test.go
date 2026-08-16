package pipelang

import (
	"reflect"
	"strings"
	"testing"
)

func TestAnalyzeModuleSetResolvesExplicitSymbolImportDeterministically(t *testing.T) {
	root := testModule("app.root", "root.pipe", `public Class Root : IShared { public string Name; }`, ImportDecl{
		Kind: ImportSymbol, Module: "lib.shared", Symbol: "IShared", Span: Span{File: "root.pipe", Start: 0, End: 0},
	})
	shared := testModule("lib.shared", "shared.pipe", `public Interface IShared { public string Name; }`)
	input := testModuleSet("app.root", []ModuleInput{shared, root}, map[ModuleID][]ModuleID{
		"app.root": {"lib.shared"},
	})

	analysis := AnalyzeModuleSet(input)
	if err := analysis.Error(); err != nil {
		t.Fatalf("AnalyzeModuleSet() error = %v", err)
	}
	if analysis.Modules.Root() != "app.root" {
		t.Fatalf("root = %q", analysis.Modules.Root())
	}
	if analysis.Modules.LanguageContract() != "foundation.test" {
		t.Fatalf("language contract = %q", analysis.Modules.LanguageContract())
	}
	if got := analysis.Modules.LockedModules(); len(got) != 2 || got[0].ID != "app.root" || got[1].ID != "lib.shared" {
		t.Fatalf("locked modules = %#v", got)
	}
	sharedSymbol, ok := analysis.Symbols.LookupOwned(moduleOwner("lib.shared"), "IShared")
	if !ok {
		t.Fatal("missing owned imported symbol")
	}
	if sharedSymbol.Owner != moduleOwner("lib.shared") || !sharedSymbol.DeclarationSpan.IsValid() {
		t.Fatalf("symbol = %#v", sharedSymbol)
	}
	resolved, err := analysis.ResolveType(*analysis.Program.Classes[0].Implements)
	if err != nil {
		t.Fatalf("ResolveType() error = %v", err)
	}
	if resolved.Symbol != sharedSymbol.ID {
		t.Fatalf("resolved symbol = %d, want %d", resolved.Symbol, sharedSymbol.ID)
	}
	if _, err := Check(analysis.Program); err != nil {
		t.Fatalf("rechecking bound program must be idempotent: %v", err)
	}

	reordered := input
	reordered.Modules = []ModuleInput{root, shared}
	reordered.Lock.Modules[0], reordered.Lock.Modules[1] = reordered.Lock.Modules[1], reordered.Lock.Modules[0]
	second := AnalyzeModuleSet(reordered)
	if err := second.Error(); err != nil {
		t.Fatalf("reordered AnalyzeModuleSet() error = %v", err)
	}
	if !reflect.DeepEqual(analysis.Symbols.Symbols(), second.Symbols.Symbols()) {
		t.Fatalf("symbols depend on input order:\nfirst: %#v\nsecond: %#v", analysis.Symbols.Symbols(), second.Symbols.Symbols())
	}
}

func TestAnalyzeModuleSetResolvesQualifiedTypeThroughModuleImport(t *testing.T) {
	root := testModule("app.root", "root.pipe", `public Class Root { public string Name; }`, ImportDecl{
		Kind: ImportModule, Module: "lib.shared", Span: Span{File: "root.pipe", Start: 0, End: 0},
	})
	shared := testModule("lib.shared", "shared.pipe", `public Interface IShared { public string Name; }`)
	analysis := AnalyzeModuleSet(testModuleSet("app.root", []ModuleInput{root, shared}, map[ModuleID][]ModuleID{
		"app.root": {"lib.shared"},
	}))
	if err := analysis.Error(); err != nil {
		t.Fatalf("AnalyzeModuleSet() error = %v", err)
	}
	resolved, err := analysis.ResolveType(UnresolvedTypeRef{
		Kind: TypeRefNamed, Name: "IShared", Qualifier: "lib.shared", Span: Span{File: "root.pipe", Start: 0, End: 0},
	})
	if err != nil {
		t.Fatalf("qualified ResolveType() error = %v", err)
	}
	symbol, ok := analysis.Symbols.LookupOwned(moduleOwner("lib.shared"), "IShared")
	if !ok || resolved.Symbol != symbol.ID {
		t.Fatalf("resolved = %#v, symbol = %#v, ok = %v", resolved, symbol, ok)
	}
	_, err = analysis.ResolveType(UnresolvedTypeRef{
		Kind: TypeRefNamed, Name: "Missing", Qualifier: "lib.shared", Span: Span{File: "root.pipe", Start: 0, End: 0},
	})
	diagnostics, ok := AsDiagnostics(err)
	if !ok || len(diagnostics) != 1 || diagnostics[0].Code != CodeUnknownImport || len(diagnostics[0].Related) != 1 {
		t.Fatalf("unknown qualified type diagnostics = %#v, ok = %v", diagnostics, ok)
	}
}

func TestAnalyzeModuleSetRejectsAmbiguousPrivateUndeclaredAndCyclicImports(t *testing.T) {
	t.Run("ambiguous", func(t *testing.T) {
		root := testModule("app.root", "root.pipe", `public Class Root { public string Name; }`,
			ImportDecl{Kind: ImportSymbol, Module: "lib.a", Symbol: "IShared", Span: Span{File: "root.pipe", Start: 0, End: 0}},
			ImportDecl{Kind: ImportSymbol, Module: "lib.b", Symbol: "IShared", Span: Span{File: "root.pipe", Start: 1, End: 1}},
		)
		a := testModule("lib.a", "a.pipe", `public Interface IShared { public string A; }`)
		b := testModule("lib.b", "b.pipe", `public Interface IShared { public string B; }`)
		analysis := AnalyzeModuleSet(testModuleSet("app.root", []ModuleInput{root, a, b}, map[ModuleID][]ModuleID{
			"app.root": {"lib.a", "lib.b"},
		}))
		diagnostic := assertDiagnosticCode(t, analysis, CodeAmbiguousImport)
		if !diagnostic.Primary.IsValid() || len(diagnostic.Related) != 1 || !diagnostic.Related[0].Span.IsValid() {
			t.Fatalf("ambiguous diagnostic lacks durable primary/related spans: %#v", diagnostic)
		}
	})

	t.Run("private", func(t *testing.T) {
		root := testModule("app.root", "root.pipe", `public Class Root { public string Name; }`, ImportDecl{
			Kind: ImportSymbol, Module: "lib.shared", Symbol: "Hidden", Span: Span{File: "root.pipe", Start: 0, End: 0},
		})
		shared := testModule("lib.shared", "shared.pipe", `private Interface Hidden { public string Name; }`)
		analysis := AnalyzeModuleSet(testModuleSet("app.root", []ModuleInput{root, shared}, map[ModuleID][]ModuleID{
			"app.root": {"lib.shared"},
		}))
		diagnostic := assertDiagnosticCode(t, analysis, CodePrivateImport)
		if !diagnostic.Primary.IsValid() || len(diagnostic.Related) != 1 || !diagnostic.Related[0].Span.IsValid() {
			t.Fatalf("private diagnostic lacks durable primary/related spans: %#v", diagnostic)
		}
	})

	t.Run("undeclared", func(t *testing.T) {
		root := testModule("app.root", "root.pipe", `public Class Root { public string Name; }`, ImportDecl{
			Kind: ImportModule, Module: "lib.shared", Span: Span{File: "root.pipe", Start: 0, End: 0},
		})
		shared := testModule("lib.shared", "shared.pipe", `public Interface IShared { public string Name; }`)
		analysis := AnalyzeModuleSet(testModuleSet("app.root", []ModuleInput{root, shared}, nil))
		assertDiagnosticCode(t, analysis, CodeUndeclaredImport)
	})

	t.Run("cycle", func(t *testing.T) {
		root := testModule("app.root", "root.pipe", `public Class Root { public string Name; }`, ImportDecl{
			Kind: ImportModule, Module: "lib.shared", Span: Span{File: "root.pipe", Start: 0, End: 0},
		})
		shared := testModule("lib.shared", "shared.pipe", `public Interface IShared { public string Name; }`, ImportDecl{
			Kind: ImportModule, Module: "app.root", Span: Span{File: "shared.pipe", Start: 0, End: 0},
		})
		analysis := AnalyzeModuleSet(testModuleSet("app.root", []ModuleInput{root, shared}, map[ModuleID][]ModuleID{
			"app.root":   {"lib.shared"},
			"lib.shared": {"app.root"},
		}))
		diagnostic := assertDiagnosticCode(t, analysis, CodeImportCycle)
		if !diagnostic.Primary.IsValid() || len(diagnostic.Related) == 0 || !diagnostic.Related[0].Span.IsValid() {
			t.Fatalf("cycle diagnostic lacks durable primary/related spans: %#v", diagnostic)
		}
	})
}

func TestAnalyzeModuleSetRejectsLockDriftDuplicateOwnersAndLegacySelection(t *testing.T) {
	root := testModule("app.root", "root.pipe", `public Class Root { public string Name; }`)

	t.Run("lock drift", func(t *testing.T) {
		input := testModuleSet("app.root", []ModuleInput{root}, nil)
		input.Lock.Modules[0].SourceSHA256 = strings.Repeat("0", 64)
		assertDiagnosticCode(t, AnalyzeModuleSet(input), CodeInvalidLock)
	})

	t.Run("duplicate owner", func(t *testing.T) {
		other := testModule("app.root", "other.pipe", `public Class Other { public string Name; }`)
		input := testModuleSet("app.root", []ModuleInput{root, other}, nil)
		diagnostic := assertDiagnosticCode(t, AnalyzeModuleSet(input), CodeDuplicateModule)
		if !diagnostic.Primary.IsValid() || len(diagnostic.Related) != 1 || !diagnostic.Related[0].Span.IsValid() {
			t.Fatalf("duplicate diagnostic lacks durable primary/related spans: %#v", diagnostic)
		}
	})

	t.Run("legacy lane", func(t *testing.T) {
		input := testModuleSet("app.root", []ModuleInput{root}, nil)
		input.LanguageContract = LegacyLanguageContract
		assertDiagnosticCode(t, AnalyzeModuleSet(input), CodeInvalidModule)
	})
}

func testModule(id ModuleID, path, source string, imports ...ImportDecl) ModuleInput {
	return ModuleInput{
		ID:              id,
		Namespace:       SemanticID(id),
		DeclarationSpan: Span{File: FileID(path), Start: 0, End: 0},
		Sources:         []SourceInput{{Path: path, Data: []byte(source)}},
		Imports:         imports,
	}
}

func testModuleSet(root ModuleID, modules []ModuleInput, dependencies map[ModuleID][]ModuleID) ModuleSetInput {
	locked := make([]LockedModule, 0, len(modules))
	for _, module := range modules {
		locked = append(locked, LockedModule{ID: module.ID, SourceSHA256: ModuleSourceSHA256(module.Sources), Dependencies: append([]ModuleID(nil), dependencies[module.ID]...)})
	}
	return ModuleSetInput{LanguageContract: "foundation.test", PackageID: "test.package", Root: root, Modules: modules, Lock: DependencyLock{Modules: locked}}
}

func assertDiagnosticCode(t *testing.T, analysis *Analysis, code DiagnosticCode) Diagnostic {
	t.Helper()
	for _, diagnostic := range analysis.Diagnostics {
		if diagnostic.Code == code {
			if !diagnostic.Primary.IsValid() && code != CodeInvalidModule && code != CodeInvalidLock {
				t.Fatalf("diagnostic %s has invalid primary span: %#v", code, diagnostic)
			}
			return diagnostic
		}
	}
	t.Fatalf("missing diagnostic %s in %#v", code, analysis.Diagnostics)
	return Diagnostic{}
}
