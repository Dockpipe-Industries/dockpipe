package pipelang

// Analysis is the single parse/type diagnostic contract consumed by compiler,
// CLI, catalog, materialization, and editor-facing projections.
type Analysis struct {
	Sources     *SourceSet
	Program     *Program
	Modules     *ModuleGraph
	Symbols     *SymbolTable
	SemanticIDs *SemanticTable
	Diagnostics Diagnostics
	checked     *checkedProgram
}

func (a *Analysis) Error() error {
	if a == nil || !a.Diagnostics.HasErrors() {
		return nil
	}
	return diagnosticError(a.Sources, a.Diagnostics)
}

func AnalyzeFiles(files map[string][]byte) *Analysis {
	analysis := ParseFiles(files)
	if analysis.Diagnostics.HasErrors() {
		return analysis
	}
	checked, err := checkProgram(analysis.Sources, analysis.Program)
	if err != nil {
		if checkedDiagnostics, ok := AsDiagnostics(err); ok {
			analysis.Diagnostics = append(analysis.Diagnostics, checkedDiagnostics...)
		} else {
			analysis.Diagnostics = append(analysis.Diagnostics, Diagnostic{
				Code: CodeInvalidProgram, Category: CategorySemantic, Severity: SeverityError,
				Message: err.Error(), Primary: analysis.Program.Span,
			})
		}
	} else {
		analysis.checked = checked
		analysis.Symbols = checked.symbols
	}
	analysis.Diagnostics.Sort()
	return analysis
}

// ResolveType returns the checked form of a parsed type reference. Named
// results identify declarations through Symbols.
func (a *Analysis) ResolveType(ref UnresolvedTypeRef) (ResolvedTypeRef, error) {
	if a == nil || a.checked == nil {
		return ResolvedTypeRef{}, oneDiagnostic(nil, CodeInvalidProgram, CategorySemantic, ref.Span, "analysis is not type-correct")
	}
	return a.checked.resolveType(ref)
}

// ParseFiles parses a deterministic source set without requiring an entry class.
// It is the compatibility-preserving query used by catalog and entry inference.
func ParseFiles(files map[string][]byte) *Analysis {
	sources, diagnostics := NewSourceSetFromMap(files)
	analysis := &Analysis{Sources: sources, Diagnostics: diagnostics}
	merged := &Program{sources: sources}
	validFiles := sources.Files()
	if len(validFiles) > 0 {
		merged.Span = Span{File: validFiles[0].ID, Start: 0, End: len(validFiles[0].Text)}
	}
	for _, file := range validFiles {
		program, err := parseSourceFile(sources, file)
		if err != nil {
			if parsed, ok := AsDiagnostics(err); ok {
				analysis.Diagnostics = append(analysis.Diagnostics, parsed...)
			} else {
				analysis.Diagnostics = append(analysis.Diagnostics, Diagnostic{
					Code: CodeInvalidProgram, Category: CategorySyntax, Severity: SeverityError,
					Message: err.Error(), Primary: Span{File: file.ID, Start: 0, End: 0},
				})
			}
			continue
		}
		merged.Interfaces = append(merged.Interfaces, program.Interfaces...)
		merged.Classes = append(merged.Classes, program.Classes...)
	}
	analysis.Program = merged
	analysis.Diagnostics.Sort()
	return analysis
}
