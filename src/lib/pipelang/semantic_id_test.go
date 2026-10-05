package pipelang

import (
	"reflect"
	"strings"
	"testing"
)

func TestAnalyzeSemanticModuleSetDerivesStableIdentitiesAndAnnotatesDiagnostics(t *testing.T) {
	root := testModule("app.root", "root.pipe", `public Class Root : IShared { public string Name = 1; }`, ImportDecl{
		Kind: ImportSymbol, Module: "lib.shared", Symbol: "IShared", Span: Span{File: "root.pipe", Start: 0, End: 0},
	})
	shared := testModule("lib.shared", "shared.pipe", `public Interface IShared { public string Name; }`)
	input := semanticTestModuleSet("app.root", []ModuleInput{root, shared}, map[ModuleID][]ModuleID{"app.root": {"lib.shared"}})

	analysis := AnalyzeSemanticModuleSet(input)
	if len(analysis.Diagnostics) != 1 || analysis.Diagnostics[0].Code != CodeExpressionType {
		t.Fatalf("diagnostics = %#v", analysis.Diagnostics)
	}
	diagnostic := analysis.Diagnostics[0]
	wantIDs := []SemanticID{"app.root", "app.root.root", "app.root.root.name"}
	if !reflect.DeepEqual(diagnostic.SemanticIDs, wantIDs) {
		t.Fatalf("diagnostic semantic IDs = %#v, want %#v", diagnostic.SemanticIDs, wantIDs)
	}
	if len(diagnostic.SemanticIdentities) != 3 || diagnostic.SemanticIdentities[0].PackageID != "test.package" {
		t.Fatalf("diagnostic semantic identities = %#v", diagnostic.SemanticIdentities)
	}
	if len(diagnostic.Related) != 1 || !reflect.DeepEqual(diagnostic.Related[0].SemanticIDs, diagnostic.SemanticIDs) {
		t.Fatalf("related semantic IDs = %#v", diagnostic.Related)
	}
	rootSymbol, ok := analysis.Symbols.LookupOwned(moduleOwner("app.root"), "Root")
	if !ok || rootSymbol.SemanticID != "app.root.root" || rootSymbol.ID == 0 {
		t.Fatalf("root symbol = %#v, ok = %v", rootSymbol, ok)
	}
	resolved := ResolveDiagnostics(analysis.Sources, analysis.Diagnostics)
	if len(resolved) != 1 || !reflect.DeepEqual(resolved[0].SemanticIdentities, diagnostic.SemanticIdentities) {
		t.Fatalf("resolved diagnostics = %#v", resolved)
	}
}

func TestLegacyDiagnosticJSONOmitsSemanticIdentityFields(t *testing.T) {
	analysis := AnalyzeFiles(map[string][]byte{"legacy.pipe": []byte(`Class Legacy { Missing Value; }`)})
	payload, err := DiagnosticsJSON(analysis.Sources, analysis.Diagnostics)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(payload), "semantic_ids") || strings.Contains(string(payload), "semantic_identities") {
		t.Fatalf("legacy diagnostic JSON drifted:\n%s", payload)
	}
}

func TestAnalyzeSemanticModuleSetDerivesPublicButLeavesPrivateIdentityOptional(t *testing.T) {
	module := testModule("app.root", "root.pipe", `
public Class Root { public string Name; private string Secret; }
private Class Helper { public string Value; }
`)
	input := semanticTestModuleSet("app.root", []ModuleInput{module}, nil)
	analysis := AnalyzeSemanticModuleSet(input)
	if err := analysis.Error(); err != nil {
		t.Fatal(err)
	}
	if _, ok := analysis.SemanticIDs.Lookup("app.root.root"); !ok {
		t.Fatal("missing derived public class identity")
	}
	if _, ok := analysis.SemanticIDs.Lookup("app.root.root.name"); !ok {
		t.Fatal("missing derived public field identity")
	}
	for _, declaration := range analysis.SemanticIDs.Declarations() {
		if declaration.Name == "Helper" || declaration.Name == "Secret" || declaration.Name == "Value" {
			if declaration.Identity.Path != "" {
				t.Fatalf("private/local declaration received public identity: %#v", declaration)
			}
		}
	}
}

func TestAnalyzeSemanticModuleSetRejectsMalformedAndDuplicateDerivedIdentities(t *testing.T) {
	t.Run("malformed package", func(t *testing.T) {
		module := testModule("app.root", "root.pipe", `public Class Root { public string Name; }`)
		input := semanticTestModuleSet("app.root", []ModuleInput{module}, nil)
		input.PackageID = "Bad.Package"
		lockTestSemanticMetadata(&input)
		assertDiagnosticCode(t, AnalyzeSemanticModuleSet(input), CodeInvalidSemanticID)
	})

	t.Run("malformed namespace", func(t *testing.T) {
		module := testModule("app.root", "root.pipe", `public Class Root { public string Name; }`)
		module.Namespace = "App.Root"
		input := semanticTestModuleSet("app.root", []ModuleInput{module}, nil)
		assertDiagnosticCode(t, AnalyzeSemanticModuleSet(input), CodeInvalidSemanticID)
	})

	t.Run("identity-bearing source name", func(t *testing.T) {
		module := testModule("app.root", "root.pipe", `public Class Root_Value { public string Name; }`)
		input := semanticTestModuleSet("app.root", []ModuleInput{module}, nil)
		assertDiagnosticCode(t, AnalyzeSemanticModuleSet(input), CodeInvalidSemanticID)
	})

	t.Run("duplicate migration target", func(t *testing.T) {
		module := testModule("app.root", "root.pipe", `public Class Root { public string Name; }`)
		program, err := ParseFile("root.pipe", module.Sources[0].Data)
		if err != nil {
			t.Fatal(err)
		}
		target := program.Classes[0].Span
		module.SemanticMigrations = []SemanticMigration{
			{Previous: identity("test.package", "old.root"), FormerNames: []string{"Old"}, Target: target, Span: target},
			{Previous: identity("test.package", "older.root"), FormerNames: []string{"Older"}, Target: target, Span: target},
		}
		input := semanticTestModuleSet("app.root", []ModuleInput{module}, nil)
		diagnostic := assertDiagnosticCode(t, AnalyzeSemanticModuleSet(input), CodeDuplicateSemanticTarget)
		if len(diagnostic.Related) != 1 || !diagnostic.Related[0].Span.IsValid() {
			t.Fatalf("duplicate-target diagnostic = %#v", diagnostic)
		}
	})

	t.Run("duplicate former name", func(t *testing.T) {
		module := testModule("app.root", "root.pipe", `public Class Root { public string Name; }`)
		program, err := ParseFile("root.pipe", module.Sources[0].Data)
		if err != nil {
			t.Fatal(err)
		}
		target := program.Classes[0].Span
		module.SemanticMigrations = []SemanticMigration{{Previous: identity("test.package", "old.root"), FormerNames: []string{"Old", "Old"}, Target: target, Span: target}}
		input := semanticTestModuleSet("app.root", []ModuleInput{module}, nil)
		assertDiagnosticCode(t, AnalyzeSemanticModuleSet(input), CodeInvalidSemanticID)
	})

	t.Run("package-wide duplicate", func(t *testing.T) {
		root := testModule("app.root", "root.pipe", `public Class Root { public string Name; }`)
		shared := testModule("lib.shared", "shared.pipe", `public Interface Shared { public string Name; }`)
		shared.Namespace = root.Namespace
		input := semanticTestModuleSet("app.root", []ModuleInput{root, shared}, nil)
		diagnostic := assertDiagnosticCode(t, AnalyzeSemanticModuleSet(input), CodeDuplicateSemanticID)
		if len(diagnostic.Related) != 1 {
			t.Fatalf("duplicate-ID diagnostic = %#v", diagnostic)
		}
	})
}

func TestAnalyzeSemanticModuleSetRejectsMigrationCycles(t *testing.T) {
	module := testModule("app.root", "root.pipe", `
public Class Alpha { public string Name; }
public Class Beta { public string Name; }
`)
	program, err := ParseFile("root.pipe", module.Sources[0].Data)
	if err != nil {
		t.Fatal(err)
	}
	module.SemanticMigrations = []SemanticMigration{
		{Previous: identity("test.package", "app.root.beta"), FormerNames: []string{"Beta"}, Target: program.Classes[0].Span, Span: program.Classes[0].Span},
		{Previous: identity("test.package", "app.root.alpha"), FormerNames: []string{"Alpha"}, Target: program.Classes[1].Span, Span: program.Classes[1].Span},
	}
	input := semanticTestModuleSet("app.root", []ModuleInput{module}, nil)
	diagnostic := assertDiagnosticCode(t, AnalyzeSemanticModuleSet(input), CodeSemanticMigrationCycle)
	if len(diagnostic.Related) != 1 || !diagnostic.Primary.IsValid() || !diagnostic.Related[0].Span.IsValid() {
		t.Fatalf("migration-cycle diagnostic = %#v", diagnostic)
	}
}

func TestSemanticIdentitiesSurviveFileNamespaceAndSymbolRenameThroughMigration(t *testing.T) {
	first := testModule("app.root", "before.pipe", `public Class Before { public string Value; public string Format(int count) => "x"; }`)
	first.Namespace = "stable.app"
	firstInput := semanticTestModuleSet("app.root", []ModuleInput{first}, nil)
	firstAnalysis := AnalyzeSemanticModuleSet(firstInput)
	if err := firstAnalysis.Error(); err != nil {
		t.Fatal(err)
	}

	second := testModule("app.root", "after.pipe", `public Class After { public string Renamed; public string Render(int amount) => "x"; }`)
	second.Namespace = "temporary.names"
	program, err := ParseFile("after.pipe", second.Sources[0].Data)
	if err != nil {
		t.Fatal(err)
	}
	beforeType, _ := firstAnalysis.SemanticIDs.Lookup("stable.app.before")
	beforeField, _ := firstAnalysis.SemanticIDs.Lookup("stable.app.before.value")
	var beforeMethod SemanticDeclaration
	for _, declaration := range firstAnalysis.SemanticIDs.Declarations() {
		if declaration.Kind == SemanticMethod {
			beforeMethod = declaration
		}
	}
	second.SemanticMigrations = []SemanticMigration{
		{Previous: identity("test.package", "stable.app"), FormerNames: []string{"stable.app"}, Target: second.DeclarationSpan, Span: second.DeclarationSpan},
		{Previous: beforeType.Identity, FormerNames: []string{beforeType.Name}, Target: program.Classes[0].Span, Span: program.Classes[0].Span},
		{Previous: beforeField.Identity, FormerNames: []string{beforeField.Name}, Target: program.Classes[0].Fields[0].Span, Span: program.Classes[0].Fields[0].Span},
		{Previous: beforeMethod.Identity, FormerNames: []string{beforeMethod.Name}, Target: program.Classes[0].Methods[0].Span, Span: program.Classes[0].Methods[0].Span},
	}
	secondInput := semanticTestModuleSet("app.root", []ModuleInput{second}, nil)
	secondAnalysis := AnalyzeSemanticModuleSet(secondInput)
	if err := secondAnalysis.Error(); err != nil {
		t.Fatal(err)
	}
	afterType, ok := secondAnalysis.SemanticIDs.Lookup("stable.app.before")
	if !ok || afterType.Name != "After" || !reflect.DeepEqual(afterType.FormerNames, []string{"Before"}) || beforeType.DeclarationSpan.File == afterType.DeclarationSpan.File {
		t.Fatalf("rename stability failed: before=%#v after=%#v", beforeType, afterType)
	}
	afterField, ok := secondAnalysis.SemanticIDs.Lookup("stable.app.before.value")
	if !ok || afterField.Name != "Renamed" {
		t.Fatalf("field migration failed: %#v", afterField)
	}
	var afterMethod SemanticDeclaration
	for _, declaration := range secondAnalysis.SemanticIDs.Declarations() {
		if declaration.Kind == SemanticMethod {
			afterMethod = declaration
		}
	}
	if semanticIdentityKey(beforeMethod.Identity) != semanticIdentityKey(afterMethod.Identity) || afterMethod.Name != "Render" {
		t.Fatalf("callable migration failed: before=%#v after=%#v", beforeMethod, afterMethod)
	}
	projection, err := BuildSemanticProjection(secondAnalysis)
	if err != nil {
		t.Fatal(err)
	}
	if projection.Modules[0].Namespace != "temporary.names" || projection.Modules[0].Identity.Path != "stable.app" || !reflect.DeepEqual(projection.Modules[0].FormerNames, []string{"stable.app"}) {
		t.Fatalf("namespace migration projection = %#v", projection.Modules[0])
	}
}

func TestCallableIdentityUsesParameterAndReturnTypesNotParameterNames(t *testing.T) {
	first := testModule("app.root", "first.pipe", `public Class Parser { public bool Parse(List<string> values) => true; }`)
	firstInput := semanticTestModuleSet("app.root", []ModuleInput{first}, nil)
	firstAnalysis := AnalyzeSemanticModuleSet(firstInput)
	if err := firstAnalysis.Error(); err != nil {
		t.Fatal(err)
	}
	second := testModule("app.root", "second.pipe", `public Class Parser { public bool Parse(List<string> input) => true; }`)
	secondInput := semanticTestModuleSet("app.root", []ModuleInput{second}, nil)
	secondAnalysis := AnalyzeSemanticModuleSet(secondInput)
	if err := secondAnalysis.Error(); err != nil {
		t.Fatal(err)
	}
	firstMethod := semanticMethod(t, firstAnalysis)
	secondMethod := semanticMethod(t, secondAnalysis)
	if semanticIdentityKey(firstMethod.Identity) != semanticIdentityKey(secondMethod.Identity) {
		t.Fatalf("parameter name changed callable identity: first=%#v second=%#v", firstMethod.Identity, secondMethod.Identity)
	}
	if firstMethod.Identity.Callable == nil || len(firstMethod.Identity.Callable.Parameters) != 1 || firstMethod.Identity.Callable.Parameters[0].Kind != TypeRefApplied || firstMethod.Identity.Callable.Returns.Primitive != TypeBool {
		t.Fatalf("structured callable identity = %#v", firstMethod.Identity)
	}
}

func TestFormerPublicTypeNameResolvesThroughStructuredImport(t *testing.T) {
	library := testModule("lib.models", "library.pipe", `public Class Client { public string Name; }`)
	program, err := ParseFile("library.pipe", library.Sources[0].Data)
	if err != nil {
		t.Fatal(err)
	}
	library.SemanticMigrations = []SemanticMigration{{
		Previous: identity("test.package", "lib.models.customer"), FormerNames: []string{"Account", "Customer"},
		Target: program.Classes[0].Span, Span: program.Classes[0].Span,
	}}
	consumer := testModule("app.root", "consumer.pipe", `public Class Use { public Customer Value; }`, ImportDecl{
		Kind: ImportSymbol, Module: "lib.models", Symbol: "Customer", Span: Span{File: "consumer.pipe", Start: 0, End: 0},
	})
	input := semanticTestModuleSet("app.root", []ModuleInput{consumer, library}, map[ModuleID][]ModuleID{"app.root": {"lib.models"}})
	analysis := AnalyzeSemanticModuleSet(input)
	if err := analysis.Error(); err != nil {
		t.Fatal(err)
	}
	deprecated := assertDiagnosticCode(t, analysis, CodeDeprecatedName)
	if len(deprecated.Related) != 1 || len(deprecated.Related[0].SemanticIdentities) == 0 || deprecated.Related[0].SemanticIdentities[len(deprecated.Related[0].SemanticIdentities)-1].Path != "lib.models.customer" {
		t.Fatalf("deprecated-name identity = %#v", deprecated)
	}
	projection, err := BuildSemanticProjection(analysis)
	if err != nil {
		t.Fatal(err)
	}
	if len(projection.Modules[0].Imports) != 1 || projection.Modules[0].Imports[0].Symbol == nil || projection.Modules[0].Imports[0].Symbol.Path != "lib.models.customer" {
		t.Fatalf("former-name import projection = %#v", projection.Modules[0].Imports)
	}
	foundAliases := false
	for _, moduleProjection := range projection.Modules {
		for _, typeProjection := range moduleProjection.Types {
			if typeProjection.Name == "Client" {
				foundAliases = reflect.DeepEqual(typeProjection.FormerNames, []string{"Account", "Customer"})
			}
		}
	}
	if !foundAliases {
		t.Fatalf("former-name aliases were not projected: %#v", projection.Modules)
	}
}

func TestAnalyzeSemanticModuleSetRejectsSemanticLockDrift(t *testing.T) {
	module := testModule("app.root", "root.pipe", `public Class Root { public string Name; }`)
	input := semanticTestModuleSet("app.root", []ModuleInput{module}, nil)
	input.Lock.Modules[0].SemanticSHA256 = strings.Repeat("0", 64)
	assertDiagnosticCode(t, AnalyzeSemanticModuleSet(input), CodeInvalidLock)
}

func TestModuleSemanticSHA256CanonicalizesMigrationAndAliasOrder(t *testing.T) {
	first := []SemanticMigration{
		{Previous: identity("test.package", "stable.app.root"), FormerNames: []string{"Older", "Old"}, Target: Span{File: "root.pipe", Start: 10, End: 20}, Span: Span{File: "root.pipe", Start: 1, End: 2}},
		{Previous: identity("test.package", "stable.app"), FormerNames: []string{"stable.app"}, Target: Span{File: "root.pipe", Start: 0, End: 1}, Span: Span{File: "root.pipe", Start: 0, End: 1}},
	}
	second := []SemanticMigration{
		{Previous: first[1].Previous, FormerNames: []string{"stable.app"}, Target: first[1].Target, Span: first[1].Span},
		{Previous: first[0].Previous, FormerNames: []string{"Old", "Older"}, Target: first[0].Target, Span: first[0].Span},
	}
	firstDigest := ModuleSemanticSHA256("test.package", "app.root", first)
	secondDigest := ModuleSemanticSHA256("test.package", "app.root", second)
	if firstDigest != secondDigest {
		t.Fatalf("semantic lock depends on caller ordering: %s != %s", firstDigest, secondDigest)
	}
}

func TestAnalyzeSemanticModuleSetRequiresFixedPostLegacyContract(t *testing.T) {
	module := testModule("app.root", "root.pipe", `public Class Root { public string Name; }`)
	input := semanticTestModuleSet("app.root", []ModuleInput{module}, nil)
	input.LanguageContract = "future.test"
	assertDiagnosticCode(t, AnalyzeSemanticModuleSet(input), CodeInvalidModule)
}

func semanticTestModuleSet(root ModuleID, modules []ModuleInput, dependencies map[ModuleID][]ModuleID) ModuleSetInput {
	input := testModuleSet(root, modules, dependencies)
	input.LanguageContract = PipeLangLanguageContract
	input.PackageID = "test.package"
	lockTestSemanticMetadata(&input)
	return input
}

func lockTestSemanticMetadata(input *ModuleSetInput) {
	for moduleIndex := range input.Modules {
		for lockIndex := range input.Lock.Modules {
			if input.Lock.Modules[lockIndex].ID == input.Modules[moduleIndex].ID {
				module := input.Modules[moduleIndex]
				input.Lock.Modules[lockIndex].SemanticSHA256 = ModuleSemanticSHA256(input.PackageID, module.Namespace, module.SemanticMigrations)
			}
		}
	}
}

func identity(packageID PackageID, path SemanticID) SemanticIdentity {
	return SemanticIdentity{PackageID: packageID, Path: path}
}

func semanticMethod(t *testing.T, analysis *Analysis) SemanticDeclaration {
	t.Helper()
	for _, declaration := range analysis.SemanticIDs.Declarations() {
		if declaration.Kind == SemanticMethod {
			return declaration
		}
	}
	t.Fatal("missing semantic method")
	return SemanticDeclaration{}
}
