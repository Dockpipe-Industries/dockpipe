package pipelang

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"
)

func TestSemanticProjectionUsesFixedContractsStructuredIdentitiesAndDeterministicBytes(t *testing.T) {
	root := testModule("app.root", "root.pipe", `public Class Root : IShared { public string Name; public List<IShared> Items; public bool Ready(int count) => count > 0; }`, ImportDecl{
		Kind: ImportSymbol, Module: "lib.shared", Symbol: "IShared", Span: Span{File: "root.pipe", Start: 0, End: 0},
	})
	shared := testModule("lib.shared", "shared.pipe", `public Interface IShared { public string Name; }`)
	input := semanticTestModuleSet("app.root", []ModuleInput{shared, root}, map[ModuleID][]ModuleID{"app.root": {"lib.shared"}})

	analysis := AnalyzeSemanticModuleSet(input)
	if err := analysis.Error(); err != nil {
		t.Fatal(err)
	}
	projection, err := BuildSemanticProjection(analysis)
	if err != nil {
		t.Fatal(err)
	}
	if projection.Schema != PipeLangSemanticProjectionVersion || projection.CompilerContract != PipeLangCompilerContract || projection.LanguageName != PipeLangDisplayName || projection.LanguageContract != PipeLangLanguageContract || projection.PackageID != "test.package" || projection.Root.Path != "app.root" || projection.View != SemanticProjectionPublic {
		t.Fatalf("projection header = %#v", projection)
	}
	if len(projection.Modules) != 2 || projection.Modules[0].Module != "app.root" || projection.Modules[1].Module != "lib.shared" {
		t.Fatalf("modules = %#v", projection.Modules)
	}
	rootModule := projection.Modules[0]
	if len(rootModule.Dependencies) != 1 || rootModule.Dependencies[0].Path != "lib.shared" || len(rootModule.Imports) != 1 || rootModule.Imports[0].Symbol == nil || rootModule.Imports[0].Symbol.Path != "lib.shared.ishared" {
		t.Fatalf("root module links = %#v", rootModule)
	}
	if len(rootModule.Types) != 1 || rootModule.Types[0].Identity == nil || rootModule.Types[0].Identity.Path != "app.root.root" || rootModule.Types[0].Implements == nil || rootModule.Types[0].Implements.Identity == nil || rootModule.Types[0].Implements.Identity.Path != "lib.shared.ishared" {
		t.Fatalf("root type = %#v", rootModule.Types)
	}
	items := projectedMemberNamed(t, rootModule.Types[0], "Items")
	if items.Type.Kind != TypeRefApplied || len(items.Type.Arguments) != 1 || items.Type.Arguments[0].Identity == nil || items.Type.Arguments[0].Identity.Path != "lib.shared.ishared" {
		t.Fatalf("projected List<IShared> = %#v", items.Type)
	}
	ready := projectedMemberNamed(t, rootModule.Types[0], "Ready")
	if ready.Identity == nil || ready.Identity.Callable == nil || len(ready.Identity.Callable.Parameters) != 1 || ready.Identity.Callable.Parameters[0].Primitive != TypeInt || ready.Identity.Callable.Returns.Primitive != TypeBool || ready.Parameters[0].Position != 0 {
		t.Fatalf("projected callable identity = %#v", ready)
	}
	firstJSON, err := SemanticProjectionJSON(projection)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(firstJSON, []byte(`"symbol_id"`)) || bytes.Contains(firstJSON, []byte(`"local_id"`)) || bytes.Contains(firstJSON, []byte("/home/")) {
		t.Fatalf("projection leaked analysis-local or machine identity:\n%s", firstJSON)
	}

	reordered := input
	reordered.Modules = []ModuleInput{root, shared}
	reordered.Lock.Modules[0], reordered.Lock.Modules[1] = reordered.Lock.Modules[1], reordered.Lock.Modules[0]
	secondAnalysis := AnalyzeSemanticModuleSet(reordered)
	secondProjection, err := BuildSemanticProjection(secondAnalysis)
	if err != nil {
		t.Fatal(err)
	}
	secondJSON, err := SemanticProjectionJSON(secondProjection)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(firstJSON, secondJSON) {
		t.Fatalf("projection depends on input order:\nfirst:\n%s\nsecond:\n%s", firstJSON, secondJSON)
	}
	wantDigest := "1ec14ab787655dec998ce046163cd803aa538bacf1676d692e771296a7480a74"
	digest := sha256.Sum256(firstJSON)
	gotDigest := hex.EncodeToString(digest[:])
	if gotDigest != wantDigest {
		t.Fatalf("semantic projection digest = %s, want %s", gotDigest, wantDigest)
	}
}

func TestSemanticProjectionSeparatesPublicAndWorkspaceViews(t *testing.T) {
	module := testModule("app.root", "root.pipe", `public Class Root { public string Name; private string Secret; } private Class Helper { private string Value; }`)
	input := semanticTestModuleSet("app.root", []ModuleInput{module}, nil)
	analysis := AnalyzeSemanticModuleSet(input)
	if err := analysis.Error(); err != nil {
		t.Fatal(err)
	}
	public, err := BuildSemanticProjection(analysis)
	if err != nil {
		t.Fatal(err)
	}
	if len(public.Modules[0].Types) != 1 || public.Modules[0].Types[0].Name != "Root" || len(public.Modules[0].Types[0].Members) != 1 {
		t.Fatalf("public projection leaked implementation details: %#v", public.Modules[0].Types)
	}
	workspace, err := BuildWorkspaceSemanticProjection(analysis)
	if err != nil {
		t.Fatal(err)
	}
	if workspace.View != SemanticProjectionWorkspace || len(workspace.Modules[0].Types) != 2 {
		t.Fatalf("workspace projection = %#v", workspace)
	}
	var helper SemanticTypeProjection
	for _, projected := range workspace.Modules[0].Types {
		if projected.Name == "Helper" {
			helper = projected
		}
	}
	if helper.Name == "" || helper.Identity != nil {
		t.Fatalf("private local projection identity = %#v", helper)
	}
	payload, err := SemanticProjectionJSON(public)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(payload), "Helper") || strings.Contains(string(payload), "Secret") {
		t.Fatalf("public semantic projection leaked private names:\n%s", payload)
	}
}

func TestBuildSemanticProjectionRejectsLegacyAnalysis(t *testing.T) {
	legacy := AnalyzeFiles(map[string][]byte{"legacy.pipe": []byte(`Class Legacy { string Name; }`)})
	_, err := BuildSemanticProjection(legacy)
	diagnostics, ok := AsDiagnostics(err)
	if !ok || len(diagnostics) != 1 || diagnostics[0].Code != CodeInvalidProjection {
		t.Fatalf("legacy projection error = %v diagnostics = %#v", err, diagnostics)
	}
}

func TestPublicSemanticProjectionRejectsAbsoluteSourceIdentity(t *testing.T) {
	module := testModule("app.root", "/tmp/root.pipe", `public Class Root { public string Name; }`)
	input := semanticTestModuleSet("app.root", []ModuleInput{module}, nil)
	analysis := AnalyzeSemanticModuleSet(input)
	if err := analysis.Error(); err != nil {
		t.Fatal(err)
	}
	_, err := BuildSemanticProjection(analysis)
	diagnostics, ok := AsDiagnostics(err)
	if !ok || len(diagnostics) != 1 || diagnostics[0].Code != CodeInvalidProjection {
		t.Fatalf("absolute-path projection error = %v diagnostics = %#v", err, diagnostics)
	}
	if _, err := BuildWorkspaceSemanticProjection(analysis); err != nil {
		t.Fatalf("workspace projection rejected local source identity: %v", err)
	}
}

func projectedMemberNamed(t *testing.T, projected SemanticTypeProjection, name string) SemanticMemberProjection {
	t.Helper()
	for _, member := range projected.Members {
		if member.Name == name {
			return member
		}
	}
	t.Fatalf("missing projected member %q in %#v", name, projected.Members)
	return SemanticMemberProjection{}
}
