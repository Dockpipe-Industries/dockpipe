package pipelang

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
)

type SemanticProjectionVersion string

const PipeLangSemanticProjectionVersion SemanticProjectionVersion = "pipelang.semantic.v1"

type SemanticProjectionView string

const (
	SemanticProjectionPublic    SemanticProjectionView = "public"
	SemanticProjectionWorkspace SemanticProjectionView = "workspace"
)

type SemanticProjection struct {
	Schema           SemanticProjectionVersion  `json:"schema"`
	CompilerContract string                     `json:"compiler_contract"`
	LanguageName     string                     `json:"language_name"`
	LanguageContract LanguageContract           `json:"language_contract"`
	PackageID        PackageID                  `json:"package_id"`
	View             SemanticProjectionView     `json:"view"`
	Root             SemanticIdentity           `json:"root"`
	Modules          []SemanticModuleProjection `json:"modules"`
	Diagnostics      []ResolvedDiagnostic       `json:"diagnostics"`
}

type SemanticModuleProjection struct {
	Identity       SemanticIdentity           `json:"identity"`
	Module         ModuleID                   `json:"module"`
	Namespace      SemanticID                 `json:"namespace"`
	FormerNames    []string                   `json:"former_names,omitempty"`
	Declaration    SourceRange                `json:"declaration"`
	SourceSHA256   string                     `json:"source_sha256"`
	SemanticSHA256 string                     `json:"semantic_sha256"`
	Dependencies   []SemanticIdentity         `json:"dependencies"`
	Imports        []SemanticImportProjection `json:"imports"`
	Types          []SemanticTypeProjection   `json:"types"`
}

type SemanticImportProjection struct {
	Kind        ImportKind        `json:"kind"`
	Module      SemanticIdentity  `json:"module"`
	Symbol      *SemanticIdentity `json:"symbol,omitempty"`
	Declaration SourceRange       `json:"declaration"`
}

type SemanticTypeProjection struct {
	Identity    *SemanticIdentity          `json:"identity,omitempty"`
	Kind        SemanticKind               `json:"kind"`
	Name        string                     `json:"name"`
	FormerNames []string                   `json:"former_names,omitempty"`
	Visibility  Visibility                 `json:"visibility"`
	Declaration SourceRange                `json:"declaration"`
	Implements  *SemanticTypeRefProjection `json:"implements,omitempty"`
	Members     []SemanticMemberProjection `json:"members"`
}

type SemanticMemberProjection struct {
	Identity    *SemanticIdentity             `json:"identity,omitempty"`
	Kind        SemanticKind                  `json:"kind"`
	Name        string                        `json:"name"`
	FormerNames []string                      `json:"former_names,omitempty"`
	Visibility  Visibility                    `json:"visibility"`
	Type        SemanticTypeRefProjection     `json:"type"`
	Parameters  []SemanticParameterProjection `json:"parameters,omitempty"`
	Declaration SourceRange                   `json:"declaration"`
}

type SemanticParameterProjection struct {
	Position    int                       `json:"position"`
	Name        string                    `json:"name"`
	Type        SemanticTypeRefProjection `json:"type"`
	Declaration SourceRange               `json:"declaration"`
}

type SemanticTypeRefProjection struct {
	Kind      TypeRefKind                 `json:"kind"`
	Primitive PrimitiveType               `json:"primitive,omitempty"`
	Name      string                      `json:"name,omitempty"`
	Identity  *SemanticIdentity           `json:"identity,omitempty"`
	Arguments []SemanticTypeRefProjection `json:"arguments,omitempty"`
}

func BuildSemanticProjection(analysis *Analysis) (*SemanticProjection, error) {
	return buildSemanticProjection(analysis, SemanticProjectionPublic)
}

func BuildWorkspaceSemanticProjection(analysis *Analysis) (*SemanticProjection, error) {
	return buildSemanticProjection(analysis, SemanticProjectionWorkspace)
}

func buildSemanticProjection(analysis *Analysis, view SemanticProjectionView) (*SemanticProjection, error) {
	if analysis == nil || analysis.Program == nil || analysis.Modules == nil || analysis.Symbols == nil || analysis.SemanticIDs == nil {
		return nil, oneDiagnostic(nil, CodeInvalidProjection, CategorySemantic, Span{}, "semantic projection requires a semantic module analysis")
	}
	if err := analysis.Error(); err != nil {
		return nil, err
	}
	if !isPipeLangSemanticContract(analysis.Modules.languageContract) {
		return nil, projectionError(analysis, fmt.Sprintf("semantic projection requires language contract %q, %q, %q, %q, or %q", PipeLangLanguageContractV010, PipeLangLanguageContractV020, PipeLangLanguageContractV030, PipeLangLanguageContractV040, PipeLangLanguageContractV050))
	}
	if view != SemanticProjectionPublic && view != SemanticProjectionWorkspace {
		return nil, projectionError(analysis, fmt.Sprintf("invalid semantic projection view %q", view))
	}
	if view == SemanticProjectionPublic {
		for _, file := range analysis.Sources.Files() {
			if filepath.IsAbs(file.Path) {
				return nil, projectionError(analysis, fmt.Sprintf("public semantic projection requires package-relative source identity, got %q", file.Path))
			}
		}
	}
	rootBinding := analysis.Modules.byID[analysis.Modules.root]
	if rootBinding == nil || rootBinding.module.Identity.Path == "" {
		return nil, projectionError(analysis, "root module has no semantic identity")
	}
	projection := &SemanticProjection{
		Schema: PipeLangSemanticProjectionVersion, CompilerContract: PipeLangCompilerContract,
		LanguageName: PipeLangDisplayName, LanguageContract: analysis.Modules.languageContract,
		PackageID: analysis.SemanticIDs.packageID, View: view, Root: rootBinding.module.Identity,
		Modules: []SemanticModuleProjection{}, Diagnostics: ResolveDiagnostics(analysis.Sources, analysis.Diagnostics),
	}
	for _, binding := range analysis.Modules.ordered {
		moduleProjection := SemanticModuleProjection{
			Identity: binding.module.Identity, Module: binding.module.ID, Namespace: binding.module.Namespace,
			Declaration:  analysis.Sources.Range(binding.module.DeclarationSpan),
			SourceSHA256: binding.locked.SourceSHA256, SemanticSHA256: binding.locked.SemanticSHA256,
			Dependencies: []SemanticIdentity{}, Imports: []SemanticImportProjection{}, Types: []SemanticTypeProjection{},
		}
		if semantic, ok := analysis.SemanticIDs.LookupIdentity(binding.module.Identity); ok {
			moduleProjection.FormerNames = append([]string(nil), semantic.FormerNames...)
		}
		for dependency := range binding.dependencies {
			if target := analysis.Modules.byID[dependency]; target != nil && target.module.Identity.Path != "" {
				moduleProjection.Dependencies = append(moduleProjection.Dependencies, target.module.Identity)
			}
		}
		sort.SliceStable(moduleProjection.Dependencies, func(i, j int) bool {
			return semanticIdentityKey(moduleProjection.Dependencies[i]) < semanticIdentityKey(moduleProjection.Dependencies[j])
		})
		for _, imported := range binding.imports {
			item := SemanticImportProjection{Kind: imported.decl.Kind, Module: imported.module.Identity, Declaration: analysis.Sources.Range(imported.decl.Span)}
			if imported.symbol != nil {
				if identity, ok := analysis.SemanticIDs.IdentityForSpan(imported.symbol.DeclarationSpan); ok {
					item.Symbol = semanticIdentityPointer(identity)
				}
			}
			moduleProjection.Imports = append(moduleProjection.Imports, item)
		}
		for _, entry := range analysis.Symbols.ordered {
			if entry.symbol.Owner != moduleOwner(binding.module.ID) || (view == SemanticProjectionPublic && entry.symbol.Visibility != VisibilityPublic) {
				continue
			}
			typeProjection, err := projectSymbol(analysis, entry, view)
			if err != nil {
				return nil, err
			}
			moduleProjection.Types = append(moduleProjection.Types, typeProjection)
		}
		sort.SliceStable(moduleProjection.Types, func(i, j int) bool {
			return projectedTypeKey(moduleProjection.Types[i]) < projectedTypeKey(moduleProjection.Types[j])
		})
		projection.Modules = append(projection.Modules, moduleProjection)
	}
	return projection, nil
}

func SemanticProjectionJSON(projection *SemanticProjection) ([]byte, error) {
	if projection == nil {
		return nil, oneDiagnostic(nil, CodeInvalidProjection, CategorySemantic, Span{}, "semantic projection is nil")
	}
	return json.MarshalIndent(projection, "", "  ")
}

func projectSymbol(analysis *Analysis, entry symbolEntry, view SemanticProjectionView) (SemanticTypeProjection, error) {
	projection := SemanticTypeProjection{Kind: SemanticClass, Name: entry.symbol.Name, Visibility: entry.symbol.Visibility, Declaration: analysis.Sources.Range(entry.symbol.DeclarationSpan), Members: []SemanticMemberProjection{}}
	if declaration, ok := analysis.SemanticIDs.IdentityForSpan(entry.symbol.DeclarationSpan); ok {
		projection.Identity = semanticIdentityPointer(declaration)
		if semantic, found := analysis.SemanticIDs.LookupIdentity(declaration); found {
			projection.FormerNames = append([]string(nil), semantic.FormerNames...)
		}
	}
	if entry.symbol.Kind == SymbolInterface {
		projection.Kind = SemanticInterface
		for _, field := range entry.interfaceDecl.Fields {
			if view == SemanticProjectionPublic && field.Visibility != VisibilityPublic {
				continue
			}
			member, err := projectMember(analysis, SemanticField, field.Name, field.Visibility, field.Type, nil, field.Span)
			if err != nil {
				return SemanticTypeProjection{}, err
			}
			projection.Members = append(projection.Members, member)
		}
		for _, method := range entry.interfaceDecl.Methods {
			if view == SemanticProjectionPublic && method.Visibility != VisibilityPublic {
				continue
			}
			member, err := projectMember(analysis, SemanticMethod, method.Name, method.Visibility, method.ReturnType, method.Params, method.Span)
			if err != nil {
				return SemanticTypeProjection{}, err
			}
			projection.Members = append(projection.Members, member)
		}
		sort.SliceStable(projection.Members, func(i, j int) bool {
			return projectedMemberKey(projection.Members[i]) < projectedMemberKey(projection.Members[j])
		})
		return projection, nil
	}
	if entry.classDecl.Implements != nil {
		resolved, err := analysis.ResolveType(*entry.classDecl.Implements)
		if err != nil {
			return SemanticTypeProjection{}, err
		}
		implements, err := projectResolvedType(analysis, resolved)
		if err != nil {
			return SemanticTypeProjection{}, err
		}
		projection.Implements = &implements
	}
	for _, field := range entry.classDecl.Fields {
		if view == SemanticProjectionPublic && field.Visibility != VisibilityPublic {
			continue
		}
		member, err := projectMember(analysis, SemanticField, field.Name, field.Visibility, field.Type, nil, field.Span)
		if err != nil {
			return SemanticTypeProjection{}, err
		}
		projection.Members = append(projection.Members, member)
	}
	for _, method := range entry.classDecl.Methods {
		if view == SemanticProjectionPublic && method.Visibility != VisibilityPublic {
			continue
		}
		member, err := projectMember(analysis, SemanticMethod, method.Name, method.Visibility, method.ReturnType, method.Params, method.Span)
		if err != nil {
			return SemanticTypeProjection{}, err
		}
		projection.Members = append(projection.Members, member)
	}
	sort.SliceStable(projection.Members, func(i, j int) bool {
		return projectedMemberKey(projection.Members[i]) < projectedMemberKey(projection.Members[j])
	})
	return projection, nil
}

func projectMember(analysis *Analysis, kind SemanticKind, name string, visibility Visibility, unresolved UnresolvedTypeRef, params []Param, span Span) (SemanticMemberProjection, error) {
	resolved, err := analysis.ResolveType(unresolved)
	if err != nil {
		return SemanticMemberProjection{}, err
	}
	projectedType, err := projectResolvedType(analysis, resolved)
	if err != nil {
		return SemanticMemberProjection{}, err
	}
	member := SemanticMemberProjection{Kind: kind, Name: name, Visibility: visibility, Type: projectedType, Declaration: analysis.Sources.Range(span)}
	if identity, ok := analysis.SemanticIDs.IdentityForSpan(span); ok {
		member.Identity = semanticIdentityPointer(identity)
		if semantic, found := analysis.SemanticIDs.LookupIdentity(identity); found {
			member.FormerNames = append([]string(nil), semantic.FormerNames...)
		}
	}
	for index, param := range params {
		resolvedParam, resolveErr := analysis.ResolveType(param.Type)
		if resolveErr != nil {
			return SemanticMemberProjection{}, resolveErr
		}
		projectedParam, projectErr := projectResolvedType(analysis, resolvedParam)
		if projectErr != nil {
			return SemanticMemberProjection{}, projectErr
		}
		member.Parameters = append(member.Parameters, SemanticParameterProjection{Position: index, Name: param.Name, Type: projectedParam, Declaration: analysis.Sources.Range(param.Span)})
	}
	return member, nil
}

func projectResolvedType(analysis *Analysis, resolved ResolvedTypeRef) (SemanticTypeRefProjection, error) {
	projection := SemanticTypeRefProjection{Kind: resolved.Kind, Primitive: resolved.Primitive, Name: resolved.Name}
	if resolved.Kind == TypeRefNamed {
		if resolved.PackageID != "" || resolved.Path != "" {
			if !resolved.PackageID.IsValid() || !resolved.Path.IsValid() || resolved.Symbol != 0 {
				return SemanticTypeRefProjection{}, projectionError(analysis, fmt.Sprintf("resolved built-in type %q has an invalid semantic identity", resolved.Name))
			}
			projection.Identity = semanticIdentityPointer(SemanticIdentity{PackageID: resolved.PackageID, Path: resolved.Path})
		} else {
			symbol, ok := analysis.Symbols.LookupID(resolved.Symbol)
			if !ok {
				return SemanticTypeRefProjection{}, projectionError(analysis, fmt.Sprintf("resolved type %q has no symbol", resolved.Name))
			}
			identity, ok := analysis.SemanticIDs.IdentityForSpan(symbol.DeclarationSpan)
			if !ok {
				return SemanticTypeRefProjection{}, projectionError(analysis, fmt.Sprintf("resolved type %q has no semantic identity", resolved.Name))
			}
			projection.Identity = semanticIdentityPointer(identity)
		}
	}
	if resolved.Kind == TypeRefApplied && resolved.PackageID != "" && resolved.Path != "" {
		projection.Identity = semanticIdentityPointer(SemanticIdentity{PackageID: resolved.PackageID, Path: resolved.Path})
	}
	for _, argument := range resolved.Arguments {
		projected, err := projectResolvedType(analysis, argument)
		if err != nil {
			return SemanticTypeRefProjection{}, err
		}
		projection.Arguments = append(projection.Arguments, projected)
	}
	return projection, nil
}

func semanticIdentityPointer(identity SemanticIdentity) *SemanticIdentity {
	copy := identity
	return &copy
}

func projectedTypeKey(projection SemanticTypeProjection) string {
	if projection.Identity != nil {
		return semanticIdentityKey(*projection.Identity)
	}
	return "~" + string(projection.Kind) + "\x00" + projection.Name
}

func projectedMemberKey(projection SemanticMemberProjection) string {
	if projection.Identity != nil {
		return semanticIdentityKey(*projection.Identity)
	}
	return "~" + string(projection.Kind) + "\x00" + projection.Name
}

func projectionError(analysis *Analysis, message string) error {
	span := Span{}
	if analysis != nil && analysis.Program != nil {
		span = analysis.Program.Span
	}
	diagnostic := Diagnostic{Code: CodeInvalidProjection, Category: CategorySemantic, Severity: SeverityError, Message: message, Primary: span}
	diagnostics := Diagnostics{diagnostic}
	if analysis != nil && analysis.SemanticIDs != nil {
		analysis.SemanticIDs.annotateDiagnostics(diagnostics)
	}
	var sources *SourceSet
	if analysis != nil {
		sources = analysis.Sources
	}
	return diagnosticError(sources, diagnostics)
}
