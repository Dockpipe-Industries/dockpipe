package pipelang

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
)

// PackageID is the stable owner portion of a public semantic identity. Package
// versions and content digests remain separate dependency-lock facts.
type PackageID string

func (id PackageID) IsValid() bool { return SemanticID(id).IsValid() }

// SemanticID is the lowercase dotted path portion of a semantic identity. A
// public path is initially derived from namespace, stable owner, and source
// name; a baselined migration may preserve an older path across a rename.
type SemanticID string

func (id SemanticID) IsValid() bool {
	value := string(id)
	if value == "" || strings.TrimSpace(value) != value {
		return false
	}
	for _, segment := range strings.Split(value, ".") {
		if segment == "" || segment[0] < 'a' || segment[0] > 'z' {
			return false
		}
		for i := 1; i < len(segment); i++ {
			ch := segment[i]
			if (ch < 'a' || ch > 'z') && (ch < '0' || ch > '9') {
				return false
			}
		}
	}
	return true
}

// SemanticTypeIdentity is the canonical, target-neutral identity of a type in
// a callable signature. It preserves the existing primitive/named/List<T>
// structure and deliberately carries no source spelling or local SymbolID.
type SemanticTypeIdentity struct {
	Kind      TypeRefKind            `json:"kind"`
	Primitive PrimitiveType          `json:"primitive,omitempty"`
	PackageID PackageID              `json:"package_id,omitempty"`
	Path      SemanticID             `json:"path,omitempty"`
	Name      string                 `json:"name,omitempty"`
	Arguments []SemanticTypeIdentity `json:"arguments,omitempty"`
}

// CallableIdentity distinguishes overloads without forcing parameter and
// return types into an opaque dotted string. Parameter names are not identity.
type CallableIdentity struct {
	Parameters []SemanticTypeIdentity `json:"parameters"`
	Returns    SemanticTypeIdentity   `json:"returns"`
}

// SemanticIdentity is the public declaration identity. Callable is present
// only for methods; non-callable declarations use PackageID plus Path.
type SemanticIdentity struct {
	PackageID PackageID         `json:"package_id"`
	Path      SemanticID        `json:"path"`
	Callable  *CallableIdentity `json:"callable,omitempty"`
}

func (id SemanticIdentity) IsValid() bool {
	if !id.PackageID.IsValid() || !id.Path.IsValid() {
		return false
	}
	if id.Callable == nil {
		return true
	}
	if !id.Callable.Returns.isValid() {
		return false
	}
	for _, parameter := range id.Callable.Parameters {
		if !parameter.isValid() {
			return false
		}
	}
	return true
}

func (id SemanticIdentity) String() string {
	if id.PackageID == "" && id.Path == "" {
		return ""
	}
	if id.Callable == nil {
		return fmt.Sprintf("%s:%s", id.PackageID, id.Path)
	}
	params := make([]string, 0, len(id.Callable.Parameters))
	for _, parameter := range id.Callable.Parameters {
		params = append(params, parameter.String())
	}
	return fmt.Sprintf("%s:%s(%s)->%s", id.PackageID, id.Path, strings.Join(params, ","), id.Callable.Returns.String())
}

func (id SemanticTypeIdentity) String() string {
	switch id.Kind {
	case TypeRefPrimitive:
		return string(id.Primitive)
	case TypeRefNamed:
		return fmt.Sprintf("%s:%s", id.PackageID, id.Path)
	case TypeRefApplied:
		args := make([]string, 0, len(id.Arguments))
		for _, argument := range id.Arguments {
			args = append(args, argument.String())
		}
		return fmt.Sprintf("%s<%s>", id.Name, strings.Join(args, ","))
	default:
		return ""
	}
}

func (id SemanticTypeIdentity) isValid() bool {
	switch id.Kind {
	case TypeRefPrimitive:
		_, ok := primitiveType(string(id.Primitive))
		return ok && id.PackageID == "" && id.Path == "" && id.Name == "" && len(id.Arguments) == 0
	case TypeRefNamed:
		return id.PackageID.IsValid() && id.Path.IsValid() && id.Primitive == "" && id.Name == "" && len(id.Arguments) == 0
	case TypeRefApplied:
		if id.Primitive != "" {
			return false
		}
		switch id.Name {
		case "List":
			return ((id.PackageID == "" && id.Path == "") || (id.PackageID == PipeLangBuiltinPackageID && id.Path == PipeLangListSemanticPath)) && len(id.Arguments) == 1 && id.Arguments[0].isValid()
		case "Result":
			return id.PackageID == PipeLangBuiltinPackageID && id.Path == PipeLangResultSemanticPath && len(id.Arguments) == 2 && id.Arguments[0].isValid() && id.Arguments[1].isValid()
		default:
			return false
		}
	default:
		return false
	}
}

// SemanticMigration is centralized structured compatibility input. Previous
// names the baselined identity preserved by Target; FormerNames are all
// promised deprecated aliases retained for that declaration. Span locates the
// migration record. The slice is treated as a set and canonicalized by the
// compiler, so caller ordering cannot change locks or projections.
type SemanticMigration struct {
	Previous    SemanticIdentity
	FormerNames []string
	Target      Span
	Span        Span
}

type SemanticKind string

const (
	SemanticModule    SemanticKind = "module"
	SemanticInterface SemanticKind = "interface"
	SemanticClass     SemanticKind = "class"
	SemanticRecord    SemanticKind = "record"
	SemanticField     SemanticKind = "field"
	SemanticMethod    SemanticKind = "method"
)

type SemanticDeclaration struct {
	Identity        SemanticIdentity
	Kind            SemanticKind
	Name            string
	FormerNames     []string
	Module          ModuleID
	Parent          SemanticIdentity
	Visibility      Visibility
	DeclarationSpan Span
	required        bool
	parentTarget    Span
	unresolvedType  UnresolvedTypeRef
	params          []Param
}

type SemanticTable struct {
	ordered           []SemanticDeclaration
	byIdentity        map[string]int
	byPath            map[SemanticID][]int
	byTarget          map[Span]int
	migrationByTarget map[Span]SemanticMigration
	derivedByTarget   map[Span]SemanticIdentity
	moduleByFile      map[FileID]SemanticIdentity
	packageID         PackageID
}

func (t *SemanticTable) Declarations() []SemanticDeclaration {
	if t == nil {
		return nil
	}
	out := append([]SemanticDeclaration(nil), t.ordered...)
	for i := range out {
		out[i].FormerNames = append([]string(nil), out[i].FormerNames...)
	}
	return out
}

func (t *SemanticTable) Lookup(path SemanticID) (SemanticDeclaration, bool) {
	if t == nil || len(t.byPath[path]) != 1 {
		return SemanticDeclaration{}, false
	}
	return t.ordered[t.byPath[path][0]], true
}

func (t *SemanticTable) LookupIdentity(identity SemanticIdentity) (SemanticDeclaration, bool) {
	if t == nil {
		return SemanticDeclaration{}, false
	}
	index, ok := t.byIdentity[semanticIdentityKey(identity)]
	if !ok {
		return SemanticDeclaration{}, false
	}
	return t.ordered[index], true
}

func (t *SemanticTable) IdentityForSpan(span Span) (SemanticIdentity, bool) {
	if t == nil {
		return SemanticIdentity{}, false
	}
	index, ok := t.byTarget[span]
	if !ok || t.ordered[index].Identity.Path == "" {
		return SemanticIdentity{}, false
	}
	return t.ordered[index].Identity, true
}

func (t *SemanticTable) IDForSpan(span Span) (SemanticID, bool) {
	identity, ok := t.IdentityForSpan(span)
	return identity.Path, ok
}

// ModuleSemanticSHA256 locks namespace and migration metadata independently
// from source bytes and is insensitive to caller slice order.
func ModuleSemanticSHA256(packageID PackageID, namespace SemanticID, migrations []SemanticMigration) string {
	ordered := append([]SemanticMigration(nil), migrations...)
	sort.SliceStable(ordered, func(i, j int) bool { return compareSemanticMigration(ordered[i], ordered[j]) < 0 })
	hash := sha256.New()
	var length [8]byte
	write := func(value string) {
		binary.BigEndian.PutUint64(length[:], uint64(len(value)))
		_, _ = hash.Write(length[:])
		_, _ = hash.Write([]byte(value))
	}
	write(string(packageID))
	write(string(namespace))
	write(fmt.Sprintf("%d", len(ordered)))
	for _, migration := range ordered {
		write(semanticIdentityKey(migration.Previous))
		formerNames := canonicalFormerNames(migration.FormerNames)
		write(fmt.Sprintf("%d", len(formerNames)))
		for _, formerName := range formerNames {
			write(formerName)
		}
		write(string(migration.Target.File))
		write(fmt.Sprintf("%d:%d", migration.Target.Start, migration.Target.End))
		write(string(migration.Span.File))
		write(fmt.Sprintf("%d:%d", migration.Span.Start, migration.Span.End))
	}
	return hex.EncodeToString(hash.Sum(nil))
}

func buildSemanticTable(sources *SourceSet, program *Program, graph *ModuleGraph, symbols *SymbolTable, input ModuleSetInput) (*SemanticTable, Diagnostics) {
	table := &SemanticTable{
		byIdentity: map[string]int{}, byPath: map[SemanticID][]int{}, byTarget: map[Span]int{},
		migrationByTarget: map[Span]SemanticMigration{}, derivedByTarget: map[Span]SemanticIdentity{},
		moduleByFile: map[FileID]SemanticIdentity{}, packageID: input.PackageID,
	}
	var diagnostics Diagnostics
	if program == nil || graph == nil || symbols == nil {
		return table, Diagnostics{moduleDiagnostic(CodeInvalidSemanticID, Span{}, "semantic identity analysis requires a bound module program")}
	}
	if !input.PackageID.IsValid() {
		diagnostics = append(diagnostics, moduleDiagnostic(CodeInvalidSemanticID, program.Span, fmt.Sprintf("invalid package identity %q", input.PackageID)))
	}

	moduleInputByID := make(map[ModuleID]ModuleInput, len(input.Modules))
	for _, module := range input.Modules {
		moduleInputByID[module.ID] = module
		if !module.Namespace.IsValid() {
			diagnostics = append(diagnostics, moduleDiagnostic(CodeInvalidSemanticID, module.DeclarationSpan, fmt.Sprintf("module %q has invalid namespace %q", module.ID, module.Namespace)))
		}
	}

	for _, binding := range graph.ordered {
		moduleInput := moduleInputByID[binding.module.ID]
		table.addTarget(SemanticDeclaration{Kind: SemanticModule, Name: string(moduleInput.Namespace), Module: binding.module.ID, Visibility: VisibilityPublic, DeclarationSpan: binding.module.DeclarationSpan, required: true})
	}
	for _, entry := range symbols.ordered {
		kind := SemanticClass
		if entry.symbol.Kind == SymbolInterface {
			kind = SemanticInterface
		} else if entry.symbol.Kind == SymbolRecord {
			kind = SemanticRecord
		}
		declaration := SemanticDeclaration{Kind: kind, Name: entry.symbol.Name, Module: ModuleID(entry.symbol.Owner.ID), Visibility: entry.symbol.Visibility, DeclarationSpan: entry.symbol.DeclarationSpan, required: entry.symbol.Visibility == VisibilityPublic}
		table.addTarget(declaration)
		parent := entry.symbol.DeclarationSpan
		if entry.interfaceDecl != nil {
			for _, field := range entry.interfaceDecl.Fields {
				table.addTarget(SemanticDeclaration{Kind: SemanticField, Name: field.Name, Module: declaration.Module, Visibility: field.Visibility, DeclarationSpan: field.Span, required: declaration.required && field.Visibility == VisibilityPublic, parentTarget: parent, unresolvedType: field.Type})
			}
			for _, method := range entry.interfaceDecl.Methods {
				table.addTarget(SemanticDeclaration{Kind: SemanticMethod, Name: method.Name, Module: declaration.Module, Visibility: method.Visibility, DeclarationSpan: method.Span, required: declaration.required && method.Visibility == VisibilityPublic, parentTarget: parent, unresolvedType: method.ReturnType, params: append([]Param(nil), method.Params...)})
			}
		}
		if entry.classDecl != nil {
			for _, field := range entry.classDecl.Fields {
				table.addTarget(SemanticDeclaration{Kind: SemanticField, Name: field.Name, Module: declaration.Module, Visibility: field.Visibility, DeclarationSpan: field.Span, required: declaration.required && field.Visibility == VisibilityPublic, parentTarget: parent, unresolvedType: field.Type})
			}
			for _, method := range entry.classDecl.Methods {
				table.addTarget(SemanticDeclaration{Kind: SemanticMethod, Name: method.Name, Module: declaration.Module, Visibility: method.Visibility, DeclarationSpan: method.Span, required: declaration.required && method.Visibility == VisibilityPublic, parentTarget: parent, unresolvedType: method.ReturnType, params: append([]Param(nil), method.Params...)})
			}
		}
		if entry.recordDecl != nil {
			for _, field := range entry.recordDecl.Fields {
				table.addTarget(SemanticDeclaration{Kind: SemanticField, Name: field.Name, Module: declaration.Module, Visibility: field.Visibility, DeclarationSpan: field.Span, required: declaration.required && field.Visibility == VisibilityPublic, parentTarget: parent, unresolvedType: field.Type})
			}
		}
	}
	table.sortAndIndexTargets()

	modules := append([]ModuleInput(nil), input.Modules...)
	sort.SliceStable(modules, func(i, j int) bool { return modules[i].ID < modules[j].ID })
	migrationByIdentity := map[string]SemanticMigration{}
	for _, module := range modules {
		migrations := append([]SemanticMigration(nil), module.SemanticMigrations...)
		sort.SliceStable(migrations, func(i, j int) bool { return compareSemanticMigration(migrations[i], migrations[j]) < 0 })
		for _, migration := range migrations {
			if !migration.Span.IsValid() || graph.moduleByFile[migration.Span.File] != module.ID {
				diagnostics = append(diagnostics, semanticDiagnostic(CodeInvalidSemanticID, migration.Span, migration.Previous, fmt.Sprintf("semantic migration for module %q requires a durable span in that module", module.ID)))
				continue
			}
			if !migration.Previous.IsValid() || migration.Previous.PackageID != input.PackageID {
				diagnostics = append(diagnostics, semanticDiagnostic(CodeInvalidSemanticID, migration.Span, migration.Previous, fmt.Sprintf("invalid previous semantic identity %q", migration.Previous.String())))
				continue
			}
			targetIndex, ok := table.byTarget[migration.Target]
			if !ok || table.ordered[targetIndex].Module != module.ID {
				diagnostics = append(diagnostics, semanticDiagnostic(CodeInvalidSemanticID, migration.Span, migration.Previous, fmt.Sprintf("semantic migration targets no declaration owned by module %q", module.ID)))
				continue
			}
			formerNames, formerNameErr := validateFormerNames(table.ordered[targetIndex].Kind, migration.FormerNames)
			if formerNameErr != nil {
				diagnostics = append(diagnostics, semanticDiagnostic(CodeInvalidSemanticID, migration.Span, migration.Previous, formerNameErr.Error()))
				continue
			}
			migration.FormerNames = formerNames
			if previous, duplicate := table.migrationByTarget[migration.Target]; duplicate {
				diagnostics = append(diagnostics, Diagnostic{Code: CodeDuplicateSemanticTarget, Category: CategorySemantic, Severity: SeverityError, Message: fmt.Sprintf("declaration has multiple semantic migrations %q and %q", previous.Previous.String(), migration.Previous.String()), Primary: migration.Span, Related: []RelatedSpan{{Span: previous.Span, Message: "first semantic migration", SemanticIDs: []SemanticID{previous.Previous.Path}}}, SemanticIDs: []SemanticID{migration.Previous.Path}})
				continue
			}
			identityKey := semanticIdentityKey(migration.Previous)
			if previous, duplicate := migrationByIdentity[identityKey]; duplicate && previous.Target != migration.Target {
				diagnostics = append(diagnostics, Diagnostic{Code: CodeDuplicateSemanticID, Category: CategorySemantic, Severity: SeverityError, Message: fmt.Sprintf("semantic identity %q migrates to multiple declarations", migration.Previous.String()), Primary: migration.Span, Related: []RelatedSpan{{Span: previous.Span, Message: "first semantic migration", SemanticIDs: []SemanticID{migration.Previous.Path}, SemanticIdentities: []SemanticIdentity{migration.Previous}}}, SemanticIDs: []SemanticID{migration.Previous.Path}, SemanticIdentities: []SemanticIdentity{migration.Previous}})
				continue
			}
			table.migrationByTarget[migration.Target] = migration
			migrationByIdentity[identityKey] = migration
		}
	}

	// Modules establish the stable namespace path first so a namespace migration
	// also preserves every newly derived child identity.
	for index := range table.ordered {
		if table.ordered[index].Kind != SemanticModule {
			continue
		}
		declaration := &table.ordered[index]
		derived := SemanticIdentity{PackageID: input.PackageID, Path: moduleInputByID[declaration.Module].Namespace}
		diagnostics = append(diagnostics, table.assignIdentity(index, derived)...)
	}
	moduleIdentityByID := map[ModuleID]SemanticIdentity{}
	for _, declaration := range table.ordered {
		if declaration.Kind == SemanticModule && declaration.Identity.Path != "" {
			moduleIdentityByID[declaration.Module] = declaration.Identity
		}
	}
	for index := range table.ordered {
		if table.ordered[index].Kind != SemanticClass && table.ordered[index].Kind != SemanticInterface && table.ordered[index].Kind != SemanticRecord {
			continue
		}
		declaration := &table.ordered[index]
		name, ok := deriveIdentitySegment(declaration.Name)
		if !ok {
			diagnostics = append(diagnostics, Diagnostic{Code: CodeInvalidSemanticID, Category: CategorySemantic, Severity: SeverityError, Message: fmt.Sprintf("public %s name %q cannot form a semantic identity", declaration.Kind, declaration.Name), Primary: declaration.DeclarationSpan})
			continue
		}
		owner, ok := moduleIdentityByID[declaration.Module]
		if !ok {
			continue
		}
		derived := SemanticIdentity{PackageID: input.PackageID, Path: joinSemanticPath(owner.Path, name)}
		if declaration.required {
			diagnostics = append(diagnostics, table.assignIdentity(index, derived)...)
		}
	}

	for index := range symbols.ordered {
		if identity, ok := table.IdentityForSpan(symbols.ordered[index].symbol.DeclarationSpan); ok {
			symbols.ordered[index].symbol.SemanticID = identity.Path
		}
	}
	for _, binding := range graph.ordered {
		if identity, ok := table.IdentityForSpan(binding.module.DeclarationSpan); ok {
			binding.module.Identity = identity
			for _, file := range binding.module.Files {
				table.moduleByFile[file] = identity
			}
		}
	}

	for index := range table.ordered {
		declaration := &table.ordered[index]
		if declaration.Kind != SemanticField && declaration.Kind != SemanticMethod {
			continue
		}
		parentIndex, ok := table.byTarget[declaration.parentTarget]
		if !ok || table.ordered[parentIndex].Identity.Path == "" {
			continue
		}
		declaration.Parent = table.ordered[parentIndex].Identity
		if !declaration.required {
			continue
		}
		name, valid := deriveIdentitySegment(declaration.Name)
		if !valid {
			diagnostics = append(diagnostics, Diagnostic{Code: CodeInvalidSemanticID, Category: CategorySemantic, Severity: SeverityError, Message: fmt.Sprintf("public %s name %q cannot form a semantic identity", declaration.Kind, declaration.Name), Primary: declaration.DeclarationSpan})
			continue
		}
		derived := SemanticIdentity{PackageID: input.PackageID, Path: joinSemanticPath(declaration.Parent.Path, name)}
		if declaration.Kind == SemanticMethod {
			callable, err := buildCallableIdentity(sources, symbols, graph, moduleOwner(declaration.Module), input.PackageID, declaration.params, declaration.unresolvedType)
			if err != nil {
				if resolved, ok := AsDiagnostics(err); ok {
					diagnostics = append(diagnostics, resolved...)
				}
				continue
			}
			derived.Callable = &callable
		}
		diagnostics = append(diagnostics, table.assignIdentity(index, derived)...)
	}
	diagnostics = append(diagnostics, table.validateMigrationCycles()...)

	diagnostics.Sort()
	return table, diagnostics
}

// bindSemanticAliases makes former public top-level names resolve through the
// one symbol table before structured imports are bound. Member aliases remain
// projection/compatibility metadata until member access syntax exists.
func bindSemanticAliases(symbols *SymbolTable, graph *ModuleGraph, input ModuleSetInput) Diagnostics {
	if symbols == nil || graph == nil {
		return nil
	}
	var diagnostics Diagnostics
	modules := append([]ModuleInput(nil), input.Modules...)
	sort.SliceStable(modules, func(i, j int) bool { return modules[i].ID < modules[j].ID })
	for _, module := range modules {
		migrations := append([]SemanticMigration(nil), module.SemanticMigrations...)
		sort.SliceStable(migrations, func(i, j int) bool { return compareSemanticMigration(migrations[i], migrations[j]) < 0 })
		for _, migration := range migrations {
			targetIndex := -1
			for index := range symbols.ordered {
				if symbols.ordered[index].symbol.DeclarationSpan == migration.Target {
					targetIndex = index
					break
				}
			}
			if targetIndex < 0 {
				continue
			}
			target := symbols.ordered[targetIndex].symbol
			if target.Owner != moduleOwner(module.ID) || target.Visibility != VisibilityPublic {
				continue
			}
			for _, formerName := range canonicalFormerNames(migration.FormerNames) {
				if !isIdentitySourceName(formerName) || target.Name == formerName {
					continue
				}
				key := symbolOwnerName{owner: target.Owner, name: formerName}
				if previousIndex, exists := symbols.byOwnerName[key]; exists && previousIndex != targetIndex {
					previous := symbols.ordered[previousIndex].symbol
					diagnostics = append(diagnostics, Diagnostic{Code: CodeDuplicateSemanticID, Category: CategorySemantic, Severity: SeverityError, Message: fmt.Sprintf("former public name %q conflicts with an existing declaration", formerName), Primary: migration.Span, Related: []RelatedSpan{{Span: previous.DeclarationSpan, Message: "existing declaration"}}})
					continue
				}
				symbols.byOwnerName[key] = targetIndex
				already := false
				for _, index := range symbols.byName[formerName] {
					if index == targetIndex {
						already = true
						break
					}
				}
				if !already {
					symbols.byName[formerName] = append(symbols.byName[formerName], targetIndex)
				}
			}
		}
	}
	diagnostics.Sort()
	return diagnostics
}

func (t *SemanticTable) assignIdentity(index int, derived SemanticIdentity) Diagnostics {
	declaration := &t.ordered[index]
	identity := derived
	if migration, ok := t.migrationByTarget[declaration.DeclarationSpan]; ok {
		if (migration.Previous.Callable == nil) != (derived.Callable == nil) {
			return Diagnostics{semanticDiagnostic(CodeInvalidSemanticID, migration.Span, migration.Previous, "semantic migration changes callable identity kind")}
		}
		if migration.Previous.Callable != nil && semanticCallableKey(*migration.Previous.Callable) != semanticCallableKey(*derived.Callable) {
			return Diagnostics{semanticDiagnostic(CodeInvalidSemanticID, migration.Span, migration.Previous, "rename migration changes a callable signature")}
		}
		identity = migration.Previous
		declaration.FormerNames = append([]string(nil), migration.FormerNames...)
	}
	t.derivedByTarget[declaration.DeclarationSpan] = derived
	key := semanticIdentityKey(identity)
	if previousIndex, duplicate := t.byIdentity[key]; duplicate {
		previous := t.ordered[previousIndex]
		return Diagnostics{{Code: CodeDuplicateSemanticID, Category: CategorySemantic, Severity: SeverityError, Message: fmt.Sprintf("duplicate semantic identity %q", identity.String()), Primary: declaration.DeclarationSpan, Related: []RelatedSpan{{Span: previous.DeclarationSpan, Message: "first declaration", SemanticIDs: []SemanticID{identity.Path}}}, SemanticIDs: []SemanticID{identity.Path}}}
	}
	identityCopy := identity
	if identity.Callable != nil {
		callable := *identity.Callable
		callable.Parameters = append([]SemanticTypeIdentity(nil), callable.Parameters...)
		identityCopy.Callable = &callable
	}
	declaration.Identity = identityCopy
	t.byIdentity[key] = index
	t.byPath[identity.Path] = append(t.byPath[identity.Path], index)
	return nil
}

func (t *SemanticTable) validateMigrationCycles() Diagnostics {
	type migrationEdge struct {
		from      string
		to        string
		migration SemanticMigration
	}
	edges := map[string]migrationEdge{}
	targets := make([]Span, 0, len(t.migrationByTarget))
	for target := range t.migrationByTarget {
		targets = append(targets, target)
	}
	sort.SliceStable(targets, func(i, j int) bool { return compareSpan(targets[i], targets[j]) < 0 })
	for _, target := range targets {
		migration := t.migrationByTarget[target]
		derived, ok := t.derivedByTarget[target]
		if !ok {
			continue
		}
		from := semanticIdentityKey(derived)
		to := semanticIdentityKey(migration.Previous)
		if from == to {
			continue
		}
		if _, exists := edges[from]; !exists {
			edges[from] = migrationEdge{from: from, to: to, migration: migration}
		}
	}
	keys := make([]string, 0, len(edges))
	for key := range edges {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	state := map[string]uint8{}
	stack := []migrationEdge{}
	stackIndex := map[string]int{}
	var diagnostics Diagnostics
	var visit func(string)
	visit = func(key string) {
		state[key] = 1
		stackIndex[key] = len(stack)
		edge, exists := edges[key]
		if exists {
			stack = append(stack, edge)
			if next, continues := edges[edge.to]; continues {
				switch state[next.from] {
				case 0:
					visit(next.from)
				case 1:
					start := stackIndex[next.from]
					cycle := append([]migrationEdge(nil), stack[start:]...)
					primary := cycle[len(cycle)-1]
					related := make([]RelatedSpan, 0, len(cycle)-1)
					for _, prior := range cycle[:len(cycle)-1] {
						related = append(related, RelatedSpan{Span: prior.migration.Span, Message: fmt.Sprintf("migration preserves %q", prior.migration.Previous.String()), SemanticIDs: []SemanticID{prior.migration.Previous.Path}, SemanticIdentities: []SemanticIdentity{prior.migration.Previous}})
					}
					diagnostics = append(diagnostics, Diagnostic{Code: CodeSemanticMigrationCycle, Category: CategorySemantic, Severity: SeverityError, Message: "semantic identity migrations form a cycle", Primary: primary.migration.Span, Related: related, SemanticIDs: []SemanticID{primary.migration.Previous.Path}, SemanticIdentities: []SemanticIdentity{primary.migration.Previous}})
				}
			}
			stack = stack[:len(stack)-1]
		}
		delete(stackIndex, key)
		state[key] = 2
	}
	for _, key := range keys {
		if state[key] == 0 {
			visit(key)
		}
	}
	diagnostics.Sort()
	return diagnostics
}

func buildCallableIdentity(sources *SourceSet, symbols *SymbolTable, modules *ModuleGraph, owner SymbolOwner, packageID PackageID, params []Param, result UnresolvedTypeRef) (CallableIdentity, error) {
	callable := CallableIdentity{Parameters: []SemanticTypeIdentity{}}
	for _, param := range params {
		resolved, err := resolveTypeRef(sources, symbols, modules, owner, param.Type, RelatedSpan{Span: param.Span, Message: "parameter declaration"})
		if err != nil {
			return CallableIdentity{}, err
		}
		identity, err := semanticTypeIdentity(symbols, packageID, resolved)
		if err != nil {
			return CallableIdentity{}, oneDiagnostic(sources, CodeInvalidSemanticID, CategorySemantic, param.Type.Span, err.Error())
		}
		callable.Parameters = append(callable.Parameters, identity)
	}
	resolved, err := resolveTypeRef(sources, symbols, modules, owner, result)
	if err != nil {
		return CallableIdentity{}, err
	}
	callable.Returns, err = semanticTypeIdentity(symbols, packageID, resolved)
	if err != nil {
		return CallableIdentity{}, oneDiagnostic(sources, CodeInvalidSemanticID, CategorySemantic, result.Span, err.Error())
	}
	return callable, nil
}

func semanticTypeIdentity(symbols *SymbolTable, packageID PackageID, resolved ResolvedTypeRef) (SemanticTypeIdentity, error) {
	identity := SemanticTypeIdentity{Kind: resolved.Kind, Primitive: resolved.Primitive, PackageID: resolved.PackageID, Path: resolved.Path, Name: resolved.Name}
	if resolved.Kind == TypeRefNamed {
		if resolved.PackageID != "" || resolved.Path != "" {
			if !resolved.PackageID.IsValid() || !resolved.Path.IsValid() || resolved.Symbol != 0 {
				return SemanticTypeIdentity{}, fmt.Errorf("resolved built-in type %q has an invalid semantic identity", resolved.Name)
			}
		} else {
			symbol, ok := symbols.LookupID(resolved.Symbol)
			if !ok || symbol.SemanticID == "" {
				return SemanticTypeIdentity{}, fmt.Errorf("resolved type %q has no semantic identity", resolved.Name)
			}
			identity.PackageID = packageID
			identity.Path = symbol.SemanticID
		}
		identity.Name = ""
	}
	for _, argument := range resolved.Arguments {
		projected, err := semanticTypeIdentity(symbols, packageID, argument)
		if err != nil {
			return SemanticTypeIdentity{}, err
		}
		identity.Arguments = append(identity.Arguments, projected)
	}
	return identity, nil
}

func (t *SemanticTable) addTarget(declaration SemanticDeclaration) {
	t.ordered = append(t.ordered, declaration)
}

func (t *SemanticTable) sortAndIndexTargets() {
	sort.SliceStable(t.ordered, func(i, j int) bool {
		if t.ordered[i].Module != t.ordered[j].Module {
			return t.ordered[i].Module < t.ordered[j].Module
		}
		if cmp := compareSpan(t.ordered[i].DeclarationSpan, t.ordered[j].DeclarationSpan); cmp != 0 {
			return cmp < 0
		}
		return t.ordered[i].Kind < t.ordered[j].Kind
	})
	for index := range t.ordered {
		t.byTarget[t.ordered[index].DeclarationSpan] = index
	}
}

func (t *SemanticTable) annotateDiagnostics(diagnostics Diagnostics) {
	for index := range diagnostics {
		if len(diagnostics[index].SemanticIDs) == 0 {
			diagnostics[index].SemanticIDs = t.idsForSpan(diagnostics[index].Primary)
		}
		if len(diagnostics[index].SemanticIdentities) == 0 {
			diagnostics[index].SemanticIdentities = t.identitiesForSpan(diagnostics[index].Primary)
		}
		for relatedIndex := range diagnostics[index].Related {
			if len(diagnostics[index].Related[relatedIndex].SemanticIDs) == 0 {
				diagnostics[index].Related[relatedIndex].SemanticIDs = t.idsForSpan(diagnostics[index].Related[relatedIndex].Span)
			}
			if len(diagnostics[index].Related[relatedIndex].SemanticIdentities) == 0 {
				diagnostics[index].Related[relatedIndex].SemanticIdentities = t.identitiesForSpan(diagnostics[index].Related[relatedIndex].Span)
			}
		}
	}
}

func (t *SemanticTable) identitiesForSpan(span Span) []SemanticIdentity {
	if t == nil || !span.IsValid() {
		return nil
	}
	set := map[string]SemanticIdentity{}
	if module := t.moduleByFile[span.File]; module.Path != "" {
		set[semanticIdentityKey(module)] = module
	}
	for _, declaration := range t.ordered {
		if declaration.Identity.Path == "" || declaration.DeclarationSpan.File != span.File {
			continue
		}
		if declaration.DeclarationSpan.Start <= span.Start && declaration.DeclarationSpan.End >= span.End {
			set[semanticIdentityKey(declaration.Identity)] = declaration.Identity
		}
	}
	keys := make([]string, 0, len(set))
	for key := range set {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	out := make([]SemanticIdentity, 0, len(keys))
	for _, key := range keys {
		out = append(out, set[key])
	}
	return out
}

func (t *SemanticTable) idsForSpan(span Span) []SemanticID {
	identities := t.identitiesForSpan(span)
	set := map[SemanticID]struct{}{}
	for _, identity := range identities {
		set[identity.Path] = struct{}{}
	}
	ids := make([]SemanticID, 0, len(set))
	for id := range set {
		ids = append(ids, id)
	}
	sort.SliceStable(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids
}

func semanticDiagnostic(code DiagnosticCode, span Span, identity SemanticIdentity, message string) Diagnostic {
	diagnostic := Diagnostic{Code: code, Category: CategorySemantic, Severity: SeverityError, Message: message, Primary: span}
	if identity.Path != "" {
		diagnostic.SemanticIDs = []SemanticID{identity.Path}
		diagnostic.SemanticIdentities = []SemanticIdentity{identity}
	}
	return diagnostic
}

func deriveIdentitySegment(name string) (SemanticID, bool) {
	if !isIdentitySourceName(name) {
		return "", false
	}
	bytes := []byte(name)
	for i, ch := range bytes {
		if ch >= 'A' && ch <= 'Z' {
			bytes[i] = ch + ('a' - 'A')
		}
	}
	return SemanticID(bytes), true
}

func isIdentitySourceName(name string) bool {
	if name == "" || !((name[0] >= 'A' && name[0] <= 'Z') || (name[0] >= 'a' && name[0] <= 'z')) {
		return false
	}
	for i := 1; i < len(name); i++ {
		ch := name[i]
		if (ch < 'A' || ch > 'Z') && (ch < 'a' || ch > 'z') && (ch < '0' || ch > '9') {
			return false
		}
	}
	return true
}

func canonicalFormerNames(names []string) []string {
	canonical := append([]string(nil), names...)
	sort.Strings(canonical)
	if len(canonical) < 2 {
		return canonical
	}
	out := canonical[:1]
	for _, name := range canonical[1:] {
		if name != out[len(out)-1] {
			out = append(out, name)
		}
	}
	return out
}

func validateFormerNames(kind SemanticKind, names []string) ([]string, error) {
	canonical := append([]string(nil), names...)
	sort.Strings(canonical)
	for index, name := range canonical {
		valid := isIdentitySourceName(name)
		if kind == SemanticModule {
			valid = SemanticID(name).IsValid()
		}
		if !valid {
			return nil, fmt.Errorf("semantic migration has invalid former name %q", name)
		}
		if index > 0 && name == canonical[index-1] {
			return nil, fmt.Errorf("semantic migration repeats former name %q", name)
		}
	}
	return canonical, nil
}

func joinSemanticPath(parent SemanticID, child SemanticID) SemanticID {
	if parent == "" {
		return child
	}
	return SemanticID(string(parent) + "." + string(child))
}

func semanticIdentityKey(identity SemanticIdentity) string {
	key := string(identity.PackageID) + "\x00" + string(identity.Path)
	if identity.Callable != nil {
		key += "\x00" + semanticCallableKey(*identity.Callable)
	}
	return key
}

func semanticCallableKey(callable CallableIdentity) string {
	var key strings.Builder
	writeCanonicalKeyField(&key, fmt.Sprintf("%d", len(callable.Parameters)))
	for _, parameter := range callable.Parameters {
		writeCanonicalKeyField(&key, semanticTypeKey(parameter))
	}
	writeCanonicalKeyField(&key, semanticTypeKey(callable.Returns))
	return key.String()
}

func semanticTypeKey(identity SemanticTypeIdentity) string {
	var key strings.Builder
	for _, field := range []string{string(identity.Kind), string(identity.Primitive), string(identity.PackageID), string(identity.Path), identity.Name, fmt.Sprintf("%d", len(identity.Arguments))} {
		writeCanonicalKeyField(&key, field)
	}
	for _, argument := range identity.Arguments {
		writeCanonicalKeyField(&key, semanticTypeKey(argument))
	}
	return key.String()
}

func writeCanonicalKeyField(key *strings.Builder, value string) {
	fmt.Fprintf(key, "%d:", len(value))
	key.WriteString(value)
}

func compareSemanticMigration(left, right SemanticMigration) int {
	if leftKey, rightKey := semanticIdentityKey(left.Previous), semanticIdentityKey(right.Previous); leftKey != rightKey {
		return strings.Compare(leftKey, rightKey)
	}
	leftFormerNames := strings.Join(canonicalFormerNames(left.FormerNames), "\x1c")
	rightFormerNames := strings.Join(canonicalFormerNames(right.FormerNames), "\x1c")
	if leftFormerNames != rightFormerNames {
		return strings.Compare(leftFormerNames, rightFormerNames)
	}
	if cmp := compareSpan(left.Target, right.Target); cmp != 0 {
		return cmp
	}
	return compareSpan(left.Span, right.Span)
}
