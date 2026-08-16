package pipelang

import (
	"fmt"
	"sort"
	"strings"
)

// SymbolID is a deterministic analysis-local identity. It is deliberately not
// a public semantic ID and is not persisted into v0.0.0.1 artifacts.
type SymbolID uint32

type SymbolKind string

const (
	SymbolInterface SymbolKind = "interface"
	SymbolClass     SymbolKind = "class"
)

type SymbolOwnerKind string

const SymbolOwnerLegacySourceSet SymbolOwnerKind = "legacy-source-set"

const SymbolOwnerModule SymbolOwnerKind = "module"

type SymbolOwner struct {
	Kind SymbolOwnerKind
	ID   string
}

var legacySourceSetOwner = SymbolOwner{
	Kind: SymbolOwnerLegacySourceSet,
	ID:   "pipelang:v0.0.0.1:implicit-sibling-source-set",
}

type Symbol struct {
	ID              SymbolID
	Kind            SymbolKind
	Name            string
	Owner           SymbolOwner
	Visibility      Visibility
	SemanticID      SemanticID
	DeclarationSpan Span
}

type symbolEntry struct {
	symbol        Symbol
	interfaceDecl *InterfaceDecl
	classDecl     *ClassDecl
}

// SymbolTable is the one declaration namespace for both the frozen legacy lane
// and explicit module analyses. Iteration order is owner then declaration-span
// order and is independent of Go map order.
type SymbolTable struct {
	ordered     []symbolEntry
	byOwnerName map[symbolOwnerName]int
	byName      map[string][]int
	byID        map[SymbolID]int
}

type symbolOwnerName struct {
	owner SymbolOwner
	name  string
}

func (t *SymbolTable) Symbols() []Symbol {
	if t == nil {
		return nil
	}
	out := make([]Symbol, 0, len(t.ordered))
	for _, entry := range t.ordered {
		out = append(out, entry.symbol)
	}
	return out
}

func (t *SymbolTable) Lookup(name string) (Symbol, bool) {
	entries := t.lookupEntries(name)
	if len(entries) != 1 {
		return Symbol{}, false
	}
	return entries[0].symbol, true
}

func (t *SymbolTable) LookupOwned(owner SymbolOwner, name string) (Symbol, bool) {
	entry, ok := t.lookupOwnedEntry(owner, name)
	if !ok {
		return Symbol{}, false
	}
	return entry.symbol, true
}

func (t *SymbolTable) LookupID(id SymbolID) (Symbol, bool) {
	if t == nil {
		return Symbol{}, false
	}
	idx, ok := t.byID[id]
	if !ok {
		return Symbol{}, false
	}
	return t.ordered[idx].symbol, true
}

func (t *SymbolTable) lookupEntry(name string) (symbolEntry, bool) {
	entries := t.lookupEntries(name)
	if len(entries) != 1 {
		return symbolEntry{}, false
	}
	return entries[0], true
}

func (t *SymbolTable) lookupEntries(name string) []symbolEntry {
	if t == nil {
		return nil
	}
	indices := t.byName[strings.TrimSpace(name)]
	out := make([]symbolEntry, 0, len(indices))
	for _, idx := range indices {
		out = append(out, t.ordered[idx])
	}
	return out
}

func (t *SymbolTable) lookupOwnedEntry(owner SymbolOwner, name string) (symbolEntry, bool) {
	if t == nil {
		return symbolEntry{}, false
	}
	idx, ok := t.byOwnerName[symbolOwnerName{owner: owner, name: strings.TrimSpace(name)}]
	if !ok {
		return symbolEntry{}, false
	}
	return t.ordered[idx], true
}

func (t *SymbolTable) lookupIDEntry(id SymbolID) (symbolEntry, bool) {
	if t == nil {
		return symbolEntry{}, false
	}
	idx, ok := t.byID[id]
	if !ok {
		return symbolEntry{}, false
	}
	return t.ordered[idx], true
}

func buildSymbolTable(sources *SourceSet, program *Program) (*SymbolTable, error) {
	return buildSymbolTableWithOwners(sources, program, nil)
}

func buildSymbolTableWithOwners(sources *SourceSet, program *Program, modules *ModuleGraph) (*SymbolTable, error) {
	if program == nil {
		return nil, oneDiagnostic(sources, CodeInvalidProgram, CategorySemantic, Span{}, "program is nil")
	}
	entries := make([]symbolEntry, 0, len(program.Interfaces)+len(program.Classes))
	for _, decl := range program.Interfaces {
		if decl != nil {
			owner := legacySourceSetOwner
			if modules != nil {
				if resolved, ok := modules.ownerForSpan(decl.Span); ok {
					owner = resolved
				}
			}
			entries = append(entries, symbolEntry{symbol: Symbol{Kind: SymbolInterface, Name: decl.Name, Owner: owner, Visibility: normalizeVisibility(decl.Visibility), DeclarationSpan: decl.Span}, interfaceDecl: decl})
		}
	}
	for _, decl := range program.Classes {
		if decl != nil {
			owner := legacySourceSetOwner
			if modules != nil {
				if resolved, ok := modules.ownerForSpan(decl.Span); ok {
					owner = resolved
				}
			}
			entries = append(entries, symbolEntry{symbol: Symbol{Kind: SymbolClass, Name: decl.Name, Owner: owner, Visibility: normalizeVisibility(decl.Visibility), DeclarationSpan: decl.Span}, classDecl: decl})
		}
	}
	sort.SliceStable(entries, func(i, j int) bool {
		if entries[i].symbol.Owner.Kind != entries[j].symbol.Owner.Kind {
			return entries[i].symbol.Owner.Kind < entries[j].symbol.Owner.Kind
		}
		if entries[i].symbol.Owner.ID != entries[j].symbol.Owner.ID {
			return entries[i].symbol.Owner.ID < entries[j].symbol.Owner.ID
		}
		return compareSpan(entries[i].symbol.DeclarationSpan, entries[j].symbol.DeclarationSpan) < 0
	})
	table := &SymbolTable{byOwnerName: map[symbolOwnerName]int{}, byName: map[string][]int{}, byID: map[SymbolID]int{}}
	for _, entry := range entries {
		name := strings.TrimSpace(entry.symbol.Name)
		if name == "" {
			return nil, oneDiagnostic(sources, CodeInvalidDecl, CategorySemantic, entry.symbol.DeclarationSpan, fmt.Sprintf("%s name is empty", entry.symbol.Kind))
		}
		key := symbolOwnerName{owner: entry.symbol.Owner, name: name}
		if previousIndex, ok := table.byOwnerName[key]; ok {
			previous := table.ordered[previousIndex].symbol
			return nil, oneDiagnostic(sources, CodeDuplicateDecl, CategorySemantic, entry.symbol.DeclarationSpan, fmt.Sprintf("duplicate %s %q", entry.symbol.Kind, name), RelatedSpan{Span: previous.DeclarationSpan, Message: "first declaration"})
		}
		entry.symbol.ID = SymbolID(len(table.ordered) + 1)
		table.byOwnerName[key] = len(table.ordered)
		table.byName[name] = append(table.byName[name], len(table.ordered))
		table.byID[entry.symbol.ID] = len(table.ordered)
		table.ordered = append(table.ordered, entry)
	}
	return table, nil
}

func resolveTypeRef(sources *SourceSet, symbols *SymbolTable, modules *ModuleGraph, owner SymbolOwner, ref UnresolvedTypeRef, related ...RelatedSpan) (ResolvedTypeRef, error) {
	if !ref.IsValid() {
		return ResolvedTypeRef{}, oneDiagnostic(sources, CodeInvalidType, CategorySemantic, ref.Span, fmt.Sprintf("invalid type %q", ref.String()), related...)
	}
	switch ref.Kind {
	case TypeRefPrimitive:
		primitive, _ := primitiveType(ref.Name)
		return resolvedPrimitive(primitive), nil
	case TypeRefNamed:
		var entry symbolEntry
		var ok bool
		code := CodeInvalidType
		var importRelated []RelatedSpan
		if modules == nil {
			entry, ok = symbols.lookupEntry(ref.Name)
		} else {
			entry, code, importRelated, ok = modules.resolveNamed(symbols, owner, ref)
		}
		if !ok {
			allRelated := append(append([]RelatedSpan(nil), related...), importRelated...)
			return ResolvedTypeRef{}, oneDiagnostic(sources, code, CategorySemantic, ref.Span, fmt.Sprintf("unknown type %q", ref.Name), allRelated...)
		}
		symbol := entry.symbol
		return ResolvedTypeRef{Kind: TypeRefNamed, Name: symbol.Name, Symbol: symbol.ID}, nil
	case TypeRefApplied:
		arguments := make([]ResolvedTypeRef, 0, len(ref.Arguments))
		for _, argument := range ref.Arguments {
			resolved, err := resolveTypeRef(sources, symbols, modules, owner, argument, related...)
			if err != nil {
				return ResolvedTypeRef{}, err
			}
			arguments = append(arguments, resolved)
		}
		return ResolvedTypeRef{Kind: TypeRefApplied, Name: ref.Name, Arguments: arguments}, nil
	default:
		return ResolvedTypeRef{}, oneDiagnostic(sources, CodeInvalidType, CategorySemantic, ref.Span, fmt.Sprintf("invalid type %q", ref.String()), related...)
	}
}
