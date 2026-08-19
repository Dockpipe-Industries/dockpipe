package pipelang

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
)

// ModuleID is an opaque canonical module identity supplied by the locked
// compiler input. Surface spelling and character rules remain deliberately
// outside this syntax-independent step-4 foundation.
type ModuleID string

// LanguageContract is the explicit, manifest-or-source-selected language
// identity. The semantic lane accepts only explicit supported post-legacy
// values; the frozen legacy lane remains a separate compiler entrypoint.
type LanguageContract string

const (
	LegacyLanguageContract       LanguageContract = "v0.0.0.1"
	PipeLangLanguageContractV010 LanguageContract = "v0.1.0"
	PipeLangLanguageContractV020 LanguageContract = "v0.2.0"
	PipeLangLanguageContractV030 LanguageContract = "v0.3.0"
	PipeLangLanguageContractV040 LanguageContract = "v0.4.0"
	PipeLangLanguageContractV050 LanguageContract = "v0.5.0"
	PipeLangLanguageContractV060 LanguageContract = "v0.6.0"
	PipeLangLanguageContractV070 LanguageContract = "v0.7.0"
	PipeLangLanguageContractV080 LanguageContract = "v0.8.0"
	PipeLangLanguageContractV090 LanguageContract = "v0.9.0"
	PipeLangLanguageContractV100 LanguageContract = "v0.10.0"
	PipeLangLanguageContractV110 LanguageContract = "v0.11.0"
	PipeLangLanguageContractV120 LanguageContract = "v0.12.0"
	PipeLangLanguageContractV130 LanguageContract = "v0.13.0"
	PipeLangLanguageContractV140 LanguageContract = "v0.14.0"
	PipeLangLanguageContractV150 LanguageContract = "v0.15.0"
	PipeLangLanguageContractV160 LanguageContract = "v0.16.0"
	PipeLangLanguageContractV170 LanguageContract = "v0.17.0"
	PipeLangLanguageContractV180 LanguageContract = "v0.18.0"
	PipeLangLanguageContractV190 LanguageContract = "v0.19.0"
	PipeLangLanguageContractV200 LanguageContract = "v0.20.0"
	PipeLangLanguageContractV210 LanguageContract = "v0.21.0"
	PipeLangLanguageContractV220 LanguageContract = "v0.22.0"
	PipeLangLanguageContract                      = PipeLangLanguageContractV010 // compatibility name for the first post-legacy seed
	PipeLangDisplayName                           = "PipeLang"
	PipeLangMachineName                           = "pipelang"
	PipeLangCompilerContract                      = "pipelang.compiler.v1"
)

func isPipeLangSemanticContract(contract LanguageContract) bool {
	return contract == PipeLangLanguageContractV010 || contract == PipeLangLanguageContractV020 || contract == PipeLangLanguageContractV030 || contract == PipeLangLanguageContractV040 || contract == PipeLangLanguageContractV050 || contract == PipeLangLanguageContractV060 || contract == PipeLangLanguageContractV070 || contract == PipeLangLanguageContractV080 || contract == PipeLangLanguageContractV090 || contract == PipeLangLanguageContractV100 || contract == PipeLangLanguageContractV110 || contract == PipeLangLanguageContractV120 || contract == PipeLangLanguageContractV130 || contract == PipeLangLanguageContractV140 || contract == PipeLangLanguageContractV150 || contract == PipeLangLanguageContractV160 || contract == PipeLangLanguageContractV170 || contract == PipeLangLanguageContractV180 || contract == PipeLangLanguageContractV190 || contract == PipeLangLanguageContractV200 || contract == PipeLangLanguageContractV210 || contract == PipeLangLanguageContractV220
}

func hasArithmeticResultSourceContract(contract LanguageContract) bool {
	if contract == PipeLangLanguageContractV220 || contract == PipeLangLanguageContractV210 {
		return true
	}
	return contract == PipeLangLanguageContractV020 || contract == PipeLangLanguageContractV030 || contract == PipeLangLanguageContractV040 || contract == PipeLangLanguageContractV050 || contract == PipeLangLanguageContractV060 || contract == PipeLangLanguageContractV070 || contract == PipeLangLanguageContractV080 || contract == PipeLangLanguageContractV090 || contract == PipeLangLanguageContractV100 || contract == PipeLangLanguageContractV110 || contract == PipeLangLanguageContractV120 || contract == PipeLangLanguageContractV130 || contract == PipeLangLanguageContractV140 || contract == PipeLangLanguageContractV150 || contract == PipeLangLanguageContractV160 || contract == PipeLangLanguageContractV170 || contract == PipeLangLanguageContractV180 || contract == PipeLangLanguageContractV190 || contract == PipeLangLanguageContractV200
}

func hasResultTransportSourceContract(contract LanguageContract) bool {
	if contract == PipeLangLanguageContractV220 || contract == PipeLangLanguageContractV210 {
		return true
	}
	return contract == PipeLangLanguageContractV070 || contract == PipeLangLanguageContractV080 || contract == PipeLangLanguageContractV090 || contract == PipeLangLanguageContractV100 || contract == PipeLangLanguageContractV110 || contract == PipeLangLanguageContractV120 || contract == PipeLangLanguageContractV130 || contract == PipeLangLanguageContractV140 || contract == PipeLangLanguageContractV150 || contract == PipeLangLanguageContractV160 || contract == PipeLangLanguageContractV170 || contract == PipeLangLanguageContractV180 || contract == PipeLangLanguageContractV190 || contract == PipeLangLanguageContractV200
}

func hasOrdinalTextOrderingSourceContract(contract LanguageContract) bool {
	if contract == PipeLangLanguageContractV220 || contract == PipeLangLanguageContractV210 {
		return true
	}
	return contract == PipeLangLanguageContractV080 || contract == PipeLangLanguageContractV090 || contract == PipeLangLanguageContractV100 || contract == PipeLangLanguageContractV110 || contract == PipeLangLanguageContractV120 || contract == PipeLangLanguageContractV130 || contract == PipeLangLanguageContractV140 || contract == PipeLangLanguageContractV150 || contract == PipeLangLanguageContractV160 || contract == PipeLangLanguageContractV170 || contract == PipeLangLanguageContractV180 || contract == PipeLangLanguageContractV190 || contract == PipeLangLanguageContractV200
}

func hasPrimitiveRecordSourceContract(contract LanguageContract) bool {
	if contract == PipeLangLanguageContractV220 || contract == PipeLangLanguageContractV210 {
		return true
	}
	return contract == PipeLangLanguageContractV090 || contract == PipeLangLanguageContractV100 || contract == PipeLangLanguageContractV110 || contract == PipeLangLanguageContractV120 || contract == PipeLangLanguageContractV130 || contract == PipeLangLanguageContractV140 || contract == PipeLangLanguageContractV150 || contract == PipeLangLanguageContractV160 || contract == PipeLangLanguageContractV170 || contract == PipeLangLanguageContractV180 || contract == PipeLangLanguageContractV190 || contract == PipeLangLanguageContractV200
}

func hasRecordFieldProjectionSourceContract(contract LanguageContract) bool {
	if contract == PipeLangLanguageContractV220 || contract == PipeLangLanguageContractV210 {
		return true
	}
	return contract == PipeLangLanguageContractV100 || contract == PipeLangLanguageContractV110 || contract == PipeLangLanguageContractV120 || contract == PipeLangLanguageContractV130 || contract == PipeLangLanguageContractV140 || contract == PipeLangLanguageContractV150 || contract == PipeLangLanguageContractV160 || contract == PipeLangLanguageContractV170 || contract == PipeLangLanguageContractV180 || contract == PipeLangLanguageContractV190 || contract == PipeLangLanguageContractV200
}

func hasPrimitiveRecordConstructionSourceContract(contract LanguageContract) bool {
	if contract == PipeLangLanguageContractV220 || contract == PipeLangLanguageContractV210 {
		return true
	}
	return contract == PipeLangLanguageContractV110 || contract == PipeLangLanguageContractV120 || contract == PipeLangLanguageContractV130 || contract == PipeLangLanguageContractV140 || contract == PipeLangLanguageContractV150 || contract == PipeLangLanguageContractV160 || contract == PipeLangLanguageContractV170 || contract == PipeLangLanguageContractV180 || contract == PipeLangLanguageContractV190 || contract == PipeLangLanguageContractV200
}

func hasPrimitiveRecordEqualitySourceContract(contract LanguageContract) bool {
	if contract == PipeLangLanguageContractV220 || contract == PipeLangLanguageContractV210 {
		return true
	}
	return contract == PipeLangLanguageContractV120 || contract == PipeLangLanguageContractV130 || contract == PipeLangLanguageContractV140 || contract == PipeLangLanguageContractV150 || contract == PipeLangLanguageContractV160 || contract == PipeLangLanguageContractV170 || contract == PipeLangLanguageContractV180 || contract == PipeLangLanguageContractV190 || contract == PipeLangLanguageContractV200
}

func hasPrimitiveOptionalSourceContract(contract LanguageContract) bool {
	if contract == PipeLangLanguageContractV220 || contract == PipeLangLanguageContractV210 {
		return true
	}
	return contract == PipeLangLanguageContractV130 || contract == PipeLangLanguageContractV140 || contract == PipeLangLanguageContractV150 || contract == PipeLangLanguageContractV160 || contract == PipeLangLanguageContractV170 || contract == PipeLangLanguageContractV180 || contract == PipeLangLanguageContractV190 || contract == PipeLangLanguageContractV200
}

func hasPrimitiveOptionalDefaultSourceContract(contract LanguageContract) bool {
	if contract == PipeLangLanguageContractV220 || contract == PipeLangLanguageContractV210 {
		return true
	}
	return contract == PipeLangLanguageContractV140 || contract == PipeLangLanguageContractV150 || contract == PipeLangLanguageContractV160 || contract == PipeLangLanguageContractV170 || contract == PipeLangLanguageContractV180 || contract == PipeLangLanguageContractV190 || contract == PipeLangLanguageContractV200
}

func hasPrimitiveRecordOptionalSourceContract(contract LanguageContract) bool {
	if contract == PipeLangLanguageContractV220 || contract == PipeLangLanguageContractV210 {
		return true
	}
	return contract == PipeLangLanguageContractV180 || contract == PipeLangLanguageContractV190 || contract == PipeLangLanguageContractV200
}

func hasPrimitiveRecordListSourceContract(contract LanguageContract) bool {
	if contract == PipeLangLanguageContractV220 || contract == PipeLangLanguageContractV210 {
		return true
	}
	return contract == PipeLangLanguageContractV150 || contract == PipeLangLanguageContractV160 || contract == PipeLangLanguageContractV170 || contract == PipeLangLanguageContractV180 || contract == PipeLangLanguageContractV190 || contract == PipeLangLanguageContractV200
}

func hasPrimitiveRecordListCountSourceContract(contract LanguageContract) bool {
	if contract == PipeLangLanguageContractV220 || contract == PipeLangLanguageContractV210 {
		return true
	}
	return contract == PipeLangLanguageContractV160 || contract == PipeLangLanguageContractV170 || contract == PipeLangLanguageContractV180 || contract == PipeLangLanguageContractV190 || contract == PipeLangLanguageContractV200
}

func hasPrimitiveRecordListAppendSourceContract(contract LanguageContract) bool {
	if contract == PipeLangLanguageContractV220 || contract == PipeLangLanguageContractV210 {
		return true
	}
	return contract == PipeLangLanguageContractV170 || contract == PipeLangLanguageContractV180 || contract == PipeLangLanguageContractV190 || contract == PipeLangLanguageContractV200
}

func hasSnapshotResultSourceContract(contract LanguageContract) bool {
	if contract == PipeLangLanguageContractV220 || contract == PipeLangLanguageContractV210 {
		return true
	}
	return contract == PipeLangLanguageContractV190 || contract == PipeLangLanguageContractV200
}

func hasPrimitiveRecordListAtSourceContract(contract LanguageContract) bool {
	return contract == PipeLangLanguageContractV200 || contract == PipeLangLanguageContractV210 || contract == PipeLangLanguageContractV220
}

func hasPrimitiveRecordListFindByTextSourceContract(contract LanguageContract) bool {
	return contract == PipeLangLanguageContractV210 || contract == PipeLangLanguageContractV220
}

func hasPrimitiveRecordListFilterByTextSourceContract(contract LanguageContract) bool {
	return contract == PipeLangLanguageContractV220
}

// ImportKind distinguishes a namespace/module import from a single-symbol import.
type ImportKind string

const (
	ImportModule ImportKind = "module"
	ImportSymbol ImportKind = "symbol"
)

// ImportDecl is the structured form consumed by the binder. Its span may
// originate in source or a future manifest, so the binder does not prescribe
// production module/import syntax.
type ImportDecl struct {
	Kind   ImportKind
	Module ModuleID
	Symbol string
	Span   Span
}

// ModuleInput assigns complete source bytes and explicit imports to one module.
// Source paths must be globally unique within the compilation input.
type ModuleInput struct {
	ID                 ModuleID
	Namespace          SemanticID
	DeclarationSpan    Span
	Sources            []SourceInput
	Imports            []ImportDecl
	SemanticMigrations []SemanticMigration
}

// LockedModule is one immutable node in the complete offline dependency graph.
// SourceSHA256 is computed by ModuleSourceSHA256 over normalized, sorted paths
// and exact bytes; compilation never fetches or repairs missing inputs.
type LockedModule struct {
	ID             ModuleID
	SourceSHA256   string
	SemanticSHA256 string
	Dependencies   []ModuleID
}

type DependencyLock struct {
	Modules []LockedModule
}

type ModuleSetInput struct {
	LanguageContract LanguageContract
	PackageID        PackageID
	Root             ModuleID
	Modules          []ModuleInput
	Lock             DependencyLock
}

type Module struct {
	ID              ModuleID
	Namespace       SemanticID
	Identity        SemanticIdentity
	DeclarationSpan Span
	Files           []FileID
	Imports         []ImportDecl
}

type boundImport struct {
	owner  ModuleID
	decl   ImportDecl
	module *Module
	symbol *Symbol
}

type moduleBinding struct {
	module        Module
	locked        LockedModule
	dependencies  map[ModuleID]struct{}
	imports       []boundImport
	symbolImports map[string][]boundImport
	moduleImports map[ModuleID]boundImport
}

// ModuleGraph is the deterministic module/import/dependency-lock contract for
// one analysis. Modules returns declaration-order-independent module-ID order.
type ModuleGraph struct {
	languageContract LanguageContract
	root             ModuleID
	ordered          []*moduleBinding
	byID             map[ModuleID]*moduleBinding
	moduleByFile     map[FileID]ModuleID
}

// LanguageContract returns the explicit non-legacy contract selected for this graph.
func (g *ModuleGraph) LanguageContract() LanguageContract {
	if g == nil {
		return ""
	}
	return g.languageContract
}

func (g *ModuleGraph) Root() ModuleID {
	if g == nil {
		return ""
	}
	return g.root
}

func (g *ModuleGraph) Modules() []Module {
	if g == nil {
		return nil
	}
	out := make([]Module, 0, len(g.ordered))
	for _, binding := range g.ordered {
		module := binding.module
		module.Files = append([]FileID(nil), module.Files...)
		module.Imports = append([]ImportDecl(nil), module.Imports...)
		sort.SliceStable(module.Imports, func(i, j int) bool { return compareImportDecl(module.Imports[i], module.Imports[j]) < 0 })
		out = append(out, module)
	}
	return out
}

// LockedModules returns the complete verified lock in deterministic module-ID
// order, with each direct dependency list sorted by module identity.
func (g *ModuleGraph) LockedModules() []LockedModule {
	if g == nil {
		return nil
	}
	out := make([]LockedModule, 0, len(g.ordered))
	for _, binding := range g.ordered {
		locked := binding.locked
		locked.Dependencies = append([]ModuleID(nil), locked.Dependencies...)
		sort.SliceStable(locked.Dependencies, func(i, j int) bool { return locked.Dependencies[i] < locked.Dependencies[j] })
		out = append(out, locked)
	}
	return out
}

func validModuleID(id ModuleID, allowEmpty bool) bool {
	value := string(id)
	if value == "" {
		return allowEmpty
	}
	return strings.TrimSpace(value) == value
}

// ModuleSourceSHA256 produces a deterministic digest without depending on map
// iteration, host paths, timestamps, locale, or ambient state.
func ModuleSourceSHA256(inputs []SourceInput) string {
	sorted := append([]SourceInput(nil), inputs...)
	sort.SliceStable(sorted, func(i, j int) bool {
		left := normalizeSourcePath(sorted[i].Path)
		right := normalizeSourcePath(sorted[j].Path)
		if left != right {
			return left < right
		}
		return bytes.Compare(sorted[i].Data, sorted[j].Data) < 0
	})
	hash := sha256.New()
	var length [8]byte
	for _, input := range sorted {
		path := []byte(normalizeSourcePath(input.Path))
		binary.BigEndian.PutUint64(length[:], uint64(len(path)))
		_, _ = hash.Write(length[:])
		_, _ = hash.Write(path)
		binary.BigEndian.PutUint64(length[:], uint64(len(input.Data)))
		_, _ = hash.Write(length[:])
		_, _ = hash.Write(input.Data)
	}
	return hex.EncodeToString(hash.Sum(nil))
}

// AnalyzeModuleSet parses the existing declaration/type grammar under an
// explicit structured module/import/lock input. It adds no production syntax
// and emits no artifacts or external effects.
func AnalyzeModuleSet(input ModuleSetInput) *Analysis {
	return analyzeModuleSet(input, false)
}

// AnalyzeSemanticModuleSet derives public semantic identities and applies
// explicit compatibility migrations to the structured module analysis without
// selecting production source syntax.
func AnalyzeSemanticModuleSet(input ModuleSetInput) *Analysis {
	return analyzeModuleSet(input, true)
}

func analyzeModuleSet(input ModuleSetInput, requireSemanticIDs bool) *Analysis {
	allSources := make([]SourceInput, 0)
	for _, module := range input.Modules {
		allSources = append(allSources, module.Sources...)
	}
	sources, diagnostics := NewSourceSet(allSources)
	analysis := &Analysis{Sources: sources, Diagnostics: diagnostics}
	graph, graphDiagnostics := prepareModuleGraph(input, sources, requireSemanticIDs)
	analysis.Modules = graph
	analysis.Diagnostics = append(analysis.Diagnostics, graphDiagnostics...)
	program := &Program{sources: sources, modules: graph}
	files := sources.Files()
	if len(files) > 0 {
		program.Span = Span{File: files[0].ID, Start: 0, End: len(files[0].Text)}
	}
	for _, file := range files {
		parsed, err := parseSourceFileWithLanguageContract(sources, file, input.LanguageContract)
		if err != nil {
			if parsedDiagnostics, ok := AsDiagnostics(err); ok {
				analysis.Diagnostics = append(analysis.Diagnostics, parsedDiagnostics...)
			} else {
				analysis.Diagnostics = append(analysis.Diagnostics, Diagnostic{Code: CodeInvalidProgram, Category: CategorySyntax, Severity: SeverityError, Message: err.Error(), Primary: Span{File: file.ID}})
			}
			continue
		}
		program.Interfaces = append(program.Interfaces, parsed.Interfaces...)
		program.Classes = append(program.Classes, parsed.Classes...)
		program.Records = append(program.Records, parsed.Records...)
	}
	analysis.Program = program
	if !analysis.Diagnostics.HasErrors() {
		symbols, err := buildSymbolTableWithOwners(sources, program, graph)
		if err != nil {
			if checkedDiagnostics, ok := AsDiagnostics(err); ok {
				analysis.Diagnostics = append(analysis.Diagnostics, checkedDiagnostics...)
			} else {
				analysis.Diagnostics = append(analysis.Diagnostics, Diagnostic{Code: CodeInvalidProgram, Category: CategorySemantic, Severity: SeverityError, Message: err.Error(), Primary: program.Span})
			}
		} else {
			analysis.Symbols = symbols
			if requireSemanticIDs {
				analysis.Diagnostics = append(analysis.Diagnostics, bindSemanticAliases(symbols, graph, input)...)
			}
			if importDiagnostics := bindModuleImports(sources, graph, symbols); len(importDiagnostics) > 0 {
				analysis.Diagnostics = append(analysis.Diagnostics, importDiagnostics...)
			}
			if requireSemanticIDs {
				table, semanticDiagnostics := buildSemanticTable(sources, program, graph, symbols, input)
				analysis.SemanticIDs = table
				analysis.Diagnostics = append(analysis.Diagnostics, semanticDiagnostics...)
			}
			if !analysis.Diagnostics.HasErrors() {
				checked, checkErr := checkProgramWithSymbols(sources, program, graph, symbols)
				if checkErr != nil {
					if checkedDiagnostics, ok := AsDiagnostics(checkErr); ok {
						analysis.Diagnostics = append(analysis.Diagnostics, checkedDiagnostics...)
					} else {
						analysis.Diagnostics = append(analysis.Diagnostics, Diagnostic{Code: CodeInvalidProgram, Category: CategorySemantic, Severity: SeverityError, Message: checkErr.Error(), Primary: program.Span})
					}
				} else {
					analysis.checked = checked
				}
			}
		}
	}
	if analysis.SemanticIDs != nil {
		analysis.SemanticIDs.annotateDiagnostics(analysis.Diagnostics)
	}
	analysis.Diagnostics.Sort()
	return analysis
}

func prepareModuleGraph(input ModuleSetInput, sources *SourceSet, requireSemanticIDs bool) (*ModuleGraph, Diagnostics) {
	graph := &ModuleGraph{languageContract: input.LanguageContract, root: input.Root, byID: map[ModuleID]*moduleBinding{}, moduleByFile: map[FileID]ModuleID{}}
	var diagnostics Diagnostics
	if strings.TrimSpace(string(input.LanguageContract)) == "" || strings.TrimSpace(string(input.LanguageContract)) != string(input.LanguageContract) {
		diagnostics = append(diagnostics, moduleDiagnostic(CodeInvalidModule, Span{}, "language contract identity is empty or non-canonical"))
	} else if input.LanguageContract == LegacyLanguageContract {
		diagnostics = append(diagnostics, moduleDiagnostic(CodeInvalidModule, Span{}, "the frozen v0.0.0.1 contract uses the legacy source-set compiler lane"))
	} else if requireSemanticIDs && !isPipeLangSemanticContract(input.LanguageContract) {
		diagnostics = append(diagnostics, moduleDiagnostic(CodeInvalidModule, Span{}, fmt.Sprintf("semantic analysis requires a supported post-legacy language contract through %q", PipeLangLanguageContractV220)))
	}
	if !validModuleID(input.Root, false) {
		diagnostics = append(diagnostics, moduleDiagnostic(CodeInvalidModule, Span{}, "root module identity is empty or non-canonical"))
	}
	modules := append([]ModuleInput(nil), input.Modules...)
	sort.SliceStable(modules, func(i, j int) bool { return modules[i].ID < modules[j].ID })
	for _, in := range modules {
		if !validModuleID(in.ID, false) {
			diagnostics = append(diagnostics, moduleDiagnostic(CodeInvalidModule, in.DeclarationSpan, "module identity is empty or non-canonical"))
			continue
		}
		if previous, exists := graph.byID[in.ID]; exists {
			diagnostics = append(diagnostics, Diagnostic{Code: CodeDuplicateModule, Category: CategorySemantic, Severity: SeverityError, Message: fmt.Sprintf("duplicate module owner %q", in.ID), Primary: in.DeclarationSpan, Related: []RelatedSpan{{Span: previous.module.DeclarationSpan, Message: "first module owner"}}})
			continue
		}
		binding := &moduleBinding{module: Module{ID: in.ID, Namespace: in.Namespace, DeclarationSpan: in.DeclarationSpan, Imports: append([]ImportDecl(nil), in.Imports...)}, dependencies: map[ModuleID]struct{}{}, symbolImports: map[string][]boundImport{}, moduleImports: map[ModuleID]boundImport{}}
		for _, source := range in.Sources {
			id := FileID(normalizeSourcePath(source.Path))
			binding.module.Files = append(binding.module.Files, id)
			if previous, exists := graph.moduleByFile[id]; exists {
				diagnostics = append(diagnostics, Diagnostic{Code: CodeDuplicateSource, Category: CategorySource, Severity: SeverityError, Message: fmt.Sprintf("source %q is owned by both modules %q and %q", id, previous, in.ID), Primary: Span{File: id}})
			} else {
				graph.moduleByFile[id] = in.ID
			}
		}
		sort.SliceStable(binding.module.Files, func(i, j int) bool { return binding.module.Files[i] < binding.module.Files[j] })
		if !in.DeclarationSpan.IsValid() {
			diagnostics = append(diagnostics, moduleDiagnostic(CodeInvalidModule, in.DeclarationSpan, fmt.Sprintf("module %q requires a durable declaration span", in.ID)))
		} else if owner, exists := graph.moduleByFile[in.DeclarationSpan.File]; !exists || owner != in.ID {
			diagnostics = append(diagnostics, moduleDiagnostic(CodeInvalidModule, in.DeclarationSpan, fmt.Sprintf("module %q declaration span is outside its owned source files", in.ID)))
		}
		for _, importDecl := range in.Imports {
			if !importDecl.Span.IsValid() {
				diagnostics = append(diagnostics, moduleDiagnostic(CodeUnknownImport, importDecl.Span, fmt.Sprintf("module %q import requires a durable source span", in.ID)))
			} else if owner, exists := graph.moduleByFile[importDecl.Span.File]; !exists || owner != in.ID {
				diagnostics = append(diagnostics, moduleDiagnostic(CodeUnknownImport, importDecl.Span, fmt.Sprintf("module %q import span is outside its owned source files", in.ID)))
			}
		}
		graph.byID[in.ID] = binding
		graph.ordered = append(graph.ordered, binding)
	}
	if _, exists := graph.byID[input.Root]; validModuleID(input.Root, false) && !exists {
		diagnostics = append(diagnostics, moduleDiagnostic(CodeInvalidModule, Span{}, fmt.Sprintf("root module %q is not present", input.Root)))
	}
	lockByID := map[ModuleID]LockedModule{}
	locked := append([]LockedModule(nil), input.Lock.Modules...)
	sort.SliceStable(locked, func(i, j int) bool { return locked[i].ID < locked[j].ID })
	for _, record := range locked {
		if !validModuleID(record.ID, false) {
			diagnostics = append(diagnostics, moduleDiagnostic(CodeInvalidLock, Span{}, "dependency lock contains an empty or non-canonical module identity"))
			continue
		}
		if _, exists := lockByID[record.ID]; exists {
			diagnostics = append(diagnostics, moduleDiagnostic(CodeInvalidLock, Span{}, fmt.Sprintf("dependency lock contains duplicate module %q", record.ID)))
			continue
		}
		lockByID[record.ID] = record
	}
	for _, binding := range graph.ordered {
		record, exists := lockByID[binding.module.ID]
		if !exists {
			diagnostics = append(diagnostics, moduleDiagnostic(CodeInvalidLock, binding.module.DeclarationSpan, fmt.Sprintf("module %q is absent from dependency lock", binding.module.ID)))
			continue
		}
		binding.locked = record
		var in *ModuleInput
		for i := range modules {
			if modules[i].ID == binding.module.ID {
				in = &modules[i]
				break
			}
		}
		if in != nil {
			actual := ModuleSourceSHA256(in.Sources)
			if record.SourceSHA256 != actual || len(record.SourceSHA256) != sha256.Size*2 {
				diagnostics = append(diagnostics, moduleDiagnostic(CodeInvalidLock, binding.module.DeclarationSpan, fmt.Sprintf("module %q source digest does not match dependency lock", binding.module.ID)))
			}
			if requireSemanticIDs {
				semanticDigest := ModuleSemanticSHA256(input.PackageID, in.Namespace, in.SemanticMigrations)
				if record.SemanticSHA256 != semanticDigest || len(record.SemanticSHA256) != sha256.Size*2 {
					diagnostics = append(diagnostics, moduleDiagnostic(CodeInvalidLock, binding.module.DeclarationSpan, fmt.Sprintf("module %q semantic identity digest does not match dependency lock", binding.module.ID)))
				}
			}
		}
		for _, dependency := range record.Dependencies {
			if !validModuleID(dependency, false) {
				diagnostics = append(diagnostics, moduleDiagnostic(CodeInvalidLock, binding.module.DeclarationSpan, fmt.Sprintf("module %q has an empty or non-canonical locked dependency", binding.module.ID)))
				continue
			}
			if _, duplicate := binding.dependencies[dependency]; duplicate {
				diagnostics = append(diagnostics, moduleDiagnostic(CodeInvalidLock, binding.module.DeclarationSpan, fmt.Sprintf("module %q repeats locked dependency %q", binding.module.ID, dependency)))
				continue
			}
			if dependency == binding.module.ID {
				diagnostics = append(diagnostics, moduleDiagnostic(CodeInvalidLock, binding.module.DeclarationSpan, fmt.Sprintf("module %q locks itself as a dependency", binding.module.ID)))
				continue
			}
			if _, exists := lockByID[dependency]; !exists {
				diagnostics = append(diagnostics, moduleDiagnostic(CodeInvalidLock, binding.module.DeclarationSpan, fmt.Sprintf("module %q locks missing dependency %q", binding.module.ID, dependency)))
				continue
			}
			binding.dependencies[dependency] = struct{}{}
		}
	}
	for id := range lockByID {
		if _, exists := graph.byID[id]; !exists {
			diagnostics = append(diagnostics, moduleDiagnostic(CodeInvalidLock, Span{}, fmt.Sprintf("locked module %q has no supplied source bytes", id)))
		}
	}
	diagnostics.Sort()
	return graph, diagnostics
}

func bindModuleImports(sources *SourceSet, graph *ModuleGraph, symbols *SymbolTable) Diagnostics {
	if graph == nil || symbols == nil {
		return nil
	}
	for _, owner := range graph.ordered {
		owner.imports = nil
		owner.symbolImports = map[string][]boundImport{}
		owner.moduleImports = map[ModuleID]boundImport{}
	}
	var diagnostics Diagnostics
	for _, owner := range graph.ordered {
		imports := append([]ImportDecl(nil), owner.module.Imports...)
		sort.SliceStable(imports, func(i, j int) bool { return compareImportDecl(imports[i], imports[j]) < 0 })
		for _, decl := range imports {
			if decl.Module == owner.module.ID {
				diagnostics = append(diagnostics, moduleDiagnostic(CodeImportCycle, decl.Span, fmt.Sprintf("module %q imports itself", owner.module.ID)))
				continue
			}
			if _, declared := owner.dependencies[decl.Module]; !declared {
				diagnostics = append(diagnostics, moduleDiagnostic(CodeUndeclaredImport, decl.Span, fmt.Sprintf("module %q imports undeclared dependency %q", owner.module.ID, decl.Module)))
				continue
			}
			target, exists := graph.byID[decl.Module]
			if !exists {
				diagnostics = append(diagnostics, moduleDiagnostic(CodeUnknownImport, decl.Span, fmt.Sprintf("imported module %q has no supplied locked source", decl.Module)))
				continue
			}
			bound := boundImport{owner: owner.module.ID, decl: decl, module: &target.module}
			switch decl.Kind {
			case ImportModule:
				if strings.TrimSpace(decl.Symbol) != "" {
					diagnostics = append(diagnostics, moduleDiagnostic(CodeUnknownImport, decl.Span, "module import cannot name a symbol"))
					continue
				}
				if previous, duplicate := owner.moduleImports[decl.Module]; duplicate {
					diagnostics = append(diagnostics, Diagnostic{Code: CodeAmbiguousImport, Category: CategorySemantic, Severity: SeverityError, Message: fmt.Sprintf("duplicate module import %q", decl.Module), Primary: decl.Span, Related: []RelatedSpan{{Span: previous.decl.Span, Message: "first import"}}})
					continue
				}
				owner.moduleImports[decl.Module] = bound
			case ImportSymbol:
				entry, ok := symbols.lookupOwnedEntry(moduleOwner(decl.Module), strings.TrimSpace(decl.Symbol))
				if !ok {
					diagnostics = append(diagnostics, moduleDiagnostic(CodeUnknownImport, decl.Span, fmt.Sprintf("module %q does not export symbol %q", decl.Module, decl.Symbol)))
					continue
				}
				if entry.symbol.Visibility != VisibilityPublic {
					diagnostics = append(diagnostics, Diagnostic{Code: CodePrivateImport, Category: CategorySemantic, Severity: SeverityError, Message: fmt.Sprintf("symbol %q in module %q is private", decl.Symbol, decl.Module), Primary: decl.Span, Related: []RelatedSpan{{Span: entry.symbol.DeclarationSpan, Message: "private declaration"}}})
					continue
				}
				if strings.TrimSpace(decl.Symbol) != entry.symbol.Name {
					diagnostics = append(diagnostics, Diagnostic{Code: CodeDeprecatedName, Category: CategorySemantic, Severity: SeverityWarning, Message: fmt.Sprintf("public name %q is deprecated; use %q", decl.Symbol, entry.symbol.Name), Primary: decl.Span, Related: []RelatedSpan{{Span: entry.symbol.DeclarationSpan, Message: "current declaration"}}})
				}
				if local, exists := symbols.lookupOwnedEntry(moduleOwner(owner.module.ID), strings.TrimSpace(decl.Symbol)); exists {
					diagnostics = append(diagnostics, Diagnostic{Code: CodeAmbiguousImport, Category: CategorySemantic, Severity: SeverityError, Message: fmt.Sprintf("symbol import %q conflicts with a local declaration", decl.Symbol), Primary: decl.Span, Related: []RelatedSpan{{Span: local.symbol.DeclarationSpan, Message: "local declaration"}}})
					continue
				}
				copy := entry.symbol
				bound.symbol = &copy
				name := strings.TrimSpace(decl.Symbol)
				owner.symbolImports[name] = append(owner.symbolImports[name], bound)
			default:
				diagnostics = append(diagnostics, moduleDiagnostic(CodeUnknownImport, decl.Span, fmt.Sprintf("invalid import kind %q", decl.Kind)))
				continue
			}
			owner.imports = append(owner.imports, bound)
		}
		for name, imports := range owner.symbolImports {
			if len(imports) < 2 {
				continue
			}
			for _, duplicate := range imports[1:] {
				diagnostics = append(diagnostics, Diagnostic{Code: CodeAmbiguousImport, Category: CategorySemantic, Severity: SeverityError, Message: fmt.Sprintf("symbol import %q is ambiguous", name), Primary: duplicate.decl.Span, Related: []RelatedSpan{{Span: imports[0].decl.Span, Message: "competing import"}}})
			}
		}
	}
	diagnostics = append(diagnostics, detectImportCycles(graph)...)
	diagnostics.Sort()
	return diagnostics
}

func detectImportCycles(graph *ModuleGraph) Diagnostics {
	state := map[ModuleID]uint8{}
	stack := []boundImport{}
	var diagnostics Diagnostics
	var visit func(*moduleBinding)
	visit = func(owner *moduleBinding) {
		state[owner.module.ID] = 1
		imports := append([]boundImport(nil), owner.imports...)
		sort.SliceStable(imports, func(i, j int) bool {
			if imports[i].module.ID != imports[j].module.ID {
				return imports[i].module.ID < imports[j].module.ID
			}
			return compareSpan(imports[i].decl.Span, imports[j].decl.Span) < 0
		})
		seenTargets := map[ModuleID]struct{}{}
		for _, edge := range imports {
			if _, seen := seenTargets[edge.module.ID]; seen {
				continue
			}
			seenTargets[edge.module.ID] = struct{}{}
			switch state[edge.module.ID] {
			case 0:
				stack = append(stack, edge)
				visit(graph.byID[edge.module.ID])
				stack = stack[:len(stack)-1]
			case 1:
				related := []RelatedSpan{}
				for _, prior := range stack {
					related = append(related, RelatedSpan{Span: prior.decl.Span, Message: fmt.Sprintf("%s imports %s", prior.owner, prior.module.ID)})
				}
				diagnostics = append(diagnostics, Diagnostic{Code: CodeImportCycle, Category: CategorySemantic, Severity: SeverityError, Message: fmt.Sprintf("import cycle reaches module %q", edge.module.ID), Primary: edge.decl.Span, Related: related})
			}
		}
		state[owner.module.ID] = 2
	}
	for _, owner := range graph.ordered {
		if state[owner.module.ID] == 0 {
			visit(owner)
		}
	}
	return diagnostics
}

func (g *ModuleGraph) ownerForSpan(span Span) (SymbolOwner, bool) {
	if g == nil {
		return SymbolOwner{}, false
	}
	id, ok := g.moduleByFile[span.File]
	if !ok {
		return SymbolOwner{}, false
	}
	return moduleOwner(id), true
}

func (g *ModuleGraph) resolveNamed(symbols *SymbolTable, owner SymbolOwner, ref UnresolvedTypeRef) (symbolEntry, DiagnosticCode, []RelatedSpan, bool) {
	if ref.Qualifier == "" {
		if local, ok := symbols.lookupOwnedEntry(owner, ref.Name); ok {
			return local, "", nil, true
		}
		binding := g.byID[ModuleID(owner.ID)]
		if binding == nil {
			return symbolEntry{}, CodeInvalidModule, nil, false
		}
		imports := binding.symbolImports[ref.Name]
		if len(imports) == 1 && imports[0].symbol != nil {
			entry, ok := symbols.lookupIDEntry(imports[0].symbol.ID)
			return entry, "", nil, ok
		}
		if len(imports) > 1 {
			related := make([]RelatedSpan, 0, len(imports))
			for _, imported := range imports {
				related = append(related, RelatedSpan{Span: imported.decl.Span, Message: "competing symbol import"})
			}
			return symbolEntry{}, CodeAmbiguousImport, related, false
		}
		return symbolEntry{}, CodeInvalidType, nil, false
	}
	binding := g.byID[ModuleID(owner.ID)]
	if binding == nil {
		return symbolEntry{}, CodeInvalidModule, nil, false
	}
	imported, ok := binding.moduleImports[ref.Qualifier]
	if !ok {
		return symbolEntry{}, CodeUndeclaredImport, nil, false
	}
	entry, ok := symbols.lookupOwnedEntry(moduleOwner(imported.module.ID), ref.Name)
	if !ok {
		return symbolEntry{}, CodeUnknownImport, []RelatedSpan{{Span: imported.decl.Span, Message: "module import"}}, false
	}
	if entry.symbol.Visibility != VisibilityPublic {
		return symbolEntry{}, CodePrivateImport, []RelatedSpan{{Span: entry.symbol.DeclarationSpan, Message: "private declaration"}}, false
	}
	return entry, "", nil, true
}

func moduleOwner(id ModuleID) SymbolOwner {
	return SymbolOwner{Kind: SymbolOwnerModule, ID: string(id)}
}

func moduleDiagnostic(code DiagnosticCode, span Span, message string) Diagnostic {
	return Diagnostic{Code: code, Category: CategorySemantic, Severity: SeverityError, Message: message, Primary: span}
}

func compareImportDecl(left, right ImportDecl) int {
	if cmp := compareSpan(left.Span, right.Span); cmp != 0 {
		return cmp
	}
	if left.Kind != right.Kind {
		return strings.Compare(string(left.Kind), string(right.Kind))
	}
	if left.Module != right.Module {
		return strings.Compare(string(left.Module), string(right.Module))
	}
	return strings.Compare(left.Symbol, right.Symbol)
}
