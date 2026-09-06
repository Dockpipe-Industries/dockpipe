package pipelang

import (
	"fmt"
	"strings"
)

type checkedProgram struct {
	program *Program
	symbols *SymbolTable
	sources *SourceSet
	modules *ModuleGraph
}

func Check(prog *Program) (*checkedProgram, error) {
	if prog == nil {
		return checkProgram(nil, prog)
	}
	return checkProgramWithModules(prog.sources, prog, prog.modules)
}

func checkProgram(sources *SourceSet, prog *Program) (*checkedProgram, error) {
	return checkProgramWithModules(sources, prog, nil)
}

func checkProgramWithModules(sources *SourceSet, prog *Program, modules *ModuleGraph) (*checkedProgram, error) {
	if prog == nil {
		return nil, oneDiagnostic(sources, CodeInvalidProgram, CategorySemantic, Span{}, "program is nil")
	}
	symbols, err := buildSymbolTableWithOwners(sources, prog, modules)
	if err != nil {
		return nil, err
	}
	if diagnostics := bindModuleImports(sources, modules, symbols); diagnostics.HasErrors() {
		return nil, diagnosticError(sources, diagnostics)
	}
	return checkProgramWithSymbols(sources, prog, modules, symbols)
}

func checkProgramWithSymbols(sources *SourceSet, prog *Program, modules *ModuleGraph, symbols *SymbolTable) (*checkedProgram, error) {
	cp := &checkedProgram{program: prog, symbols: symbols, sources: sources, modules: modules}
	for _, entry := range symbols.ordered {
		if modules != nil && ((hasArithmeticResultSourceContract(modules.LanguageContract()) && (entry.symbol.Name == "Result" || entry.symbol.Name == "ArithmeticError")) || (hasPrimitiveOptionalSourceContract(modules.LanguageContract()) && entry.symbol.Name == "Optional")) {
			return nil, oneDiagnostic(sources, CodeInvalidDecl, CategorySemantic, entry.symbol.DeclarationSpan, fmt.Sprintf("type name %q is reserved by language contract %q", entry.symbol.Name, modules.LanguageContract()))
		}
		switch entry.symbol.Kind {
		case SymbolInterface:
			decl := entry.interfaceDecl
			if !decl.Visibility.IsValid() {
				return nil, oneDiagnostic(sources, CodeInvalidDecl, CategorySemantic, decl.Span, fmt.Sprintf("interface %s has invalid visibility %q", decl.Name, decl.Visibility))
			}
			if err := cp.validateInterface(decl); err != nil {
				return nil, err
			}
		case SymbolClass:
			decl := entry.classDecl
			if !decl.Visibility.IsValid() {
				return nil, oneDiagnostic(sources, CodeInvalidDecl, CategorySemantic, decl.Span, fmt.Sprintf("class %s has invalid visibility %q", decl.Name, decl.Visibility))
			}
			if err := cp.validateClass(decl); err != nil {
				return nil, err
			}
		case SymbolRecord:
			decl := entry.recordDecl
			if err := cp.validateRecord(decl); err != nil {
				return nil, err
			}
		}
	}
	if len(prog.Classes) == 0 {
		return nil, oneDiagnostic(sources, CodeEntrySelection, CategorySemantic, prog.Span, "no class declarations found")
	}
	for _, class := range prog.Classes {
		if err := cp.validateImplements(class); err != nil {
			return nil, err
		}
	}
	return cp, nil
}

func (cp *checkedProgram) isResolvedRecordType(ref ResolvedTypeRef) bool {
	if cp == nil || cp.symbols == nil || ref.Kind != TypeRefNamed || ref.Symbol == 0 || ref.PackageID != "" || ref.Path != "" {
		return false
	}
	entry, ok := cp.symbols.lookupIDEntry(ref.Symbol)
	return ok && entry.symbol.Kind == SymbolRecord && entry.recordDecl != nil
}

func (cp *checkedProgram) containsResolvedRecordType(ref ResolvedTypeRef) bool {
	if cp.isResolvedRecordType(ref) {
		return true
	}
	for _, argument := range ref.Arguments {
		if cp.containsResolvedRecordType(argument) {
			return true
		}
	}
	return false
}

func (cp *checkedProgram) validateRecord(decl *RecordDecl) error {
	if decl == nil {
		return oneDiagnostic(cp.sources, CodeInvalidDecl, CategorySemantic, Span{}, "record declaration is nil")
	}
	contract := cp.modules.LanguageContract()
	if !hasPrimitiveRecordSourceContract(contract) {
		return oneDiagnostic(cp.sources, CodeInvalidDecl, CategorySemantic, decl.Span, fmt.Sprintf("record %s requires language contract %q", decl.Name, PipeLangLanguageContractV090))
	}
	if normalizeVisibility(decl.Visibility) != VisibilityPublic {
		return oneDiagnostic(cp.sources, CodeInvalidDecl, CategorySemantic, decl.Span, fmt.Sprintf("%s primitive record %s must be public", contract, decl.Name))
	}
	if len(decl.Annotations) != 0 {
		return oneDiagnostic(cp.sources, CodeInvalidDecl, CategorySemantic, decl.Annotations[0].Span, fmt.Sprintf("%s primitive record %s does not admit annotations", contract, decl.Name))
	}
	if decl.Implements != nil {
		return oneDiagnostic(cp.sources, CodeInvalidDecl, CategorySemantic, decl.Implements.Span, fmt.Sprintf("%s primitive record %s cannot implement another type", contract, decl.Name))
	}
	if len(decl.Methods) != 0 {
		return oneDiagnostic(cp.sources, CodeInvalidMember, CategorySemantic, decl.Methods[0].Span, fmt.Sprintf("%s primitive record %s admits fields only", contract, decl.Name))
	}
	if len(decl.Fields) == 0 {
		return oneDiagnostic(cp.sources, CodeInvalidDecl, CategorySemantic, decl.Span, fmt.Sprintf("%s primitive record %s requires at least one field", contract, decl.Name))
	}
	seen := map[string]Span{}
	for _, field := range decl.Fields {
		if normalizeVisibility(field.Visibility) != VisibilityPublic {
			return oneDiagnostic(cp.sources, CodeInvalidMember, CategorySemantic, field.Span, fmt.Sprintf("%s primitive record %s field %s must be public", contract, decl.Name, field.Name))
		}
		if len(field.Annotations) != 0 {
			return oneDiagnostic(cp.sources, CodeInvalidMember, CategorySemantic, field.Annotations[0].Span, fmt.Sprintf("%s primitive record %s field %s does not admit annotations", contract, decl.Name, field.Name))
		}
		if field.Default != nil {
			return oneDiagnostic(cp.sources, CodeInvalidMember, CategorySemantic, field.Default.SourceSpan(), fmt.Sprintf("%s primitive record %s field %s does not admit a default", contract, decl.Name, field.Name))
		}
		resolved, err := cp.resolveType(field.Type, RelatedSpan{Span: field.Span, Message: "record field declaration"})
		if err != nil {
			return prefixDiagnostic(err, fmt.Sprintf("record %s field %s: ", decl.Name, field.Name))
		}
		if resolved.Kind != TypeRefPrimitive {
			return oneDiagnostic(cp.sources, CodeInvalidType, CategorySemantic, field.Type.Span, fmt.Sprintf("%s primitive record %s field %s requires string, int, float, or bool", contract, decl.Name, field.Name))
		}
		if previous, ok := seen[field.Name]; ok {
			return oneDiagnostic(cp.sources, CodeDuplicateMember, CategorySemantic, field.Span, fmt.Sprintf("record %s has duplicate field %q", decl.Name, field.Name), RelatedSpan{Span: previous, Message: "first field"})
		}
		seen[field.Name] = field.Span
	}
	return nil
}

func (cp *checkedProgram) resolveType(ref UnresolvedTypeRef, related ...RelatedSpan) (ResolvedTypeRef, error) {
	owner := legacySourceSetOwner
	if cp.modules != nil {
		resolved, ok := cp.modules.ownerForSpan(ref.Span)
		if !ok {
			return ResolvedTypeRef{}, oneDiagnostic(cp.sources, CodeInvalidModule, CategorySemantic, ref.Span, "type reference has no owning module")
		}
		owner = resolved
	}
	return resolveTypeRef(cp.sources, cp.symbols, cp.modules, owner, ref, related...)
}

func (cp *checkedProgram) validateInterface(decl *InterfaceDecl) error {
	seen := map[string]Span{}
	for _, field := range decl.Fields {
		if normalizeVisibility(field.Visibility) != VisibilityPublic {
			return oneDiagnostic(cp.sources, CodeInvalidMember, CategorySemantic, field.Span, fmt.Sprintf("interface %s field %s must be public", decl.Name, field.Name))
		}
		resolved, err := cp.resolveType(field.Type, RelatedSpan{Span: field.Span, Message: "field declaration"})
		if err != nil {
			return prefixDiagnostic(err, fmt.Sprintf("interface %s field %s: ", decl.Name, field.Name))
		}
		if containsResolvedResult(resolved) || containsResolvedArithmeticContractType(resolved) {
			return oneDiagnostic(cp.sources, CodeInvalidType, CategorySemantic, field.Type.Span, fmt.Sprintf("the %s Result contract is not admitted in interface fields", cp.modules.LanguageContract()))
		}
		if cp.containsResolvedRecordType(resolved) {
			return oneDiagnostic(cp.sources, CodeInvalidType, CategorySemantic, field.Type.Span, fmt.Sprintf("the %s primitive record is admitted only in one exact class identity-transport method", cp.modules.LanguageContract()))
		}
		if containsResolvedOptional(resolved) {
			return oneDiagnostic(cp.sources, CodeInvalidType, CategorySemantic, field.Type.Span, fmt.Sprintf("the %s primitive Optional is admitted only in exact class methods", cp.modules.LanguageContract()))
		}
		if previous, ok := seen[field.Name]; ok {
			return oneDiagnostic(cp.sources, CodeDuplicateMember, CategorySemantic, field.Span, fmt.Sprintf("interface %s has duplicate member %q", decl.Name, field.Name), RelatedSpan{Span: previous, Message: "first member"})
		}
		seen[field.Name] = field.Span
	}
	for _, method := range decl.Methods {
		if normalizeVisibility(method.Visibility) != VisibilityPublic {
			return oneDiagnostic(cp.sources, CodeInvalidMember, CategorySemantic, method.Span, fmt.Sprintf("interface %s method %s must be public", decl.Name, method.Name))
		}
		resolved, err := cp.resolveType(method.ReturnType, RelatedSpan{Span: method.Span, Message: "method declaration"})
		if err != nil {
			return prefixDiagnostic(err, fmt.Sprintf("interface %s method %s return: ", decl.Name, method.Name))
		}
		if containsResolvedResult(resolved) || containsResolvedArithmeticContractType(resolved) {
			return oneDiagnostic(cp.sources, CodeInvalidType, CategorySemantic, method.ReturnType.Span, fmt.Sprintf("the %s Result contract requires one exact class method body", cp.modules.LanguageContract()))
		}
		if cp.containsResolvedRecordType(resolved) {
			return oneDiagnostic(cp.sources, CodeInvalidType, CategorySemantic, method.ReturnType.Span, fmt.Sprintf("the %s primitive record requires one exact class identity-transport method", cp.modules.LanguageContract()))
		}
		if previous, ok := seen[method.Name]; ok {
			return oneDiagnostic(cp.sources, CodeDuplicateMember, CategorySemantic, method.Span, fmt.Sprintf("interface %s has duplicate member %q", decl.Name, method.Name), RelatedSpan{Span: previous, Message: "first member"})
		}
		seen[method.Name] = method.Span
		if containsResolvedOptional(resolved) {
			return oneDiagnostic(cp.sources, CodeInvalidType, CategorySemantic, method.ReturnType.Span, fmt.Sprintf("the %s primitive Optional is not admitted in interface signatures", cp.modules.LanguageContract()))
		}
		if err := cp.validateParams(decl.Name, method.Name, method.Params, false, false, false); err != nil {
			return err
		}
	}
	return nil
}

func (cp *checkedProgram) validateClass(decl *ClassDecl) error {
	seen := map[string]Span{}
	fieldTypes := map[string]ResolvedTypeRef{}
	for _, field := range decl.Fields {
		if !field.Visibility.IsValid() {
			return oneDiagnostic(cp.sources, CodeInvalidMember, CategorySemantic, field.Span, fmt.Sprintf("class %s field %s has invalid visibility %q", decl.Name, field.Name, field.Visibility))
		}
		fieldType, err := cp.resolveType(field.Type, RelatedSpan{Span: field.Span, Message: "field declaration"})
		if err != nil {
			return prefixDiagnostic(err, fmt.Sprintf("class %s field %s: ", decl.Name, field.Name))
		}
		if containsResolvedResult(fieldType) || containsResolvedArithmeticContractType(fieldType) {
			return oneDiagnostic(cp.sources, CodeInvalidType, CategorySemantic, field.Type.Span, fmt.Sprintf("the %s Result contract is not admitted in class fields", cp.modules.LanguageContract()))
		}
		if cp.containsResolvedRecordType(fieldType) {
			return oneDiagnostic(cp.sources, CodeInvalidType, CategorySemantic, field.Type.Span, fmt.Sprintf("the %s primitive record is admitted only in one exact identity-transport parameter and return", cp.modules.LanguageContract()))
		}
		if containsResolvedOptional(fieldType) {
			return oneDiagnostic(cp.sources, CodeInvalidType, CategorySemantic, field.Type.Span, fmt.Sprintf("the %s primitive Optional is admitted only in exact class methods", cp.modules.LanguageContract()))
		}
		if previous, ok := seen[field.Name]; ok {
			return oneDiagnostic(cp.sources, CodeDuplicateMember, CategorySemantic, field.Span, fmt.Sprintf("class %s has duplicate member %q", decl.Name, field.Name), RelatedSpan{Span: previous, Message: "first member"})
		}
		seen[field.Name] = field.Span
		fieldTypes[field.Name] = fieldType
	}
	for _, method := range decl.Methods {
		if !method.Visibility.IsValid() {
			return oneDiagnostic(cp.sources, CodeInvalidMember, CategorySemantic, method.Span, fmt.Sprintf("class %s method %s has invalid visibility %q", decl.Name, method.Name, method.Visibility))
		}
		resolved, err := cp.resolveType(method.ReturnType, RelatedSpan{Span: method.Span, Message: "method declaration"})
		if err != nil {
			return prefixDiagnostic(err, fmt.Sprintf("class %s method %s return: ", decl.Name, method.Name))
		}
		if containsResolvedResult(resolved) && !isResolvedSourceArithmeticResult(cp.modules.LanguageContract(), resolved) && !isResolvedBoundedValueResult(cp.modules.LanguageContract(), resolved) {
			return oneDiagnostic(cp.sources, CodeInvalidType, CategorySemantic, method.ReturnType.Span, fmt.Sprintf("the %s Result slice admits only its exact checked-arithmetic, Result<List<R>,string>, or v0.25.0 Result<string,string> class method shapes", cp.modules.LanguageContract()))
		}
		if previous, ok := seen[method.Name]; ok {
			return oneDiagnostic(cp.sources, CodeDuplicateMember, CategorySemantic, method.Span, fmt.Sprintf("class %s has duplicate member %q", decl.Name, method.Name), RelatedSpan{Span: previous, Message: "first member"})
		}
		seen[method.Name] = method.Span
		allowResultParameter, err := cp.validateResultSignature(method, resolved)
		if err != nil {
			return err
		}
		allowRecordParameter, err := cp.validateRecordTransportSignature(method, resolved)
		if err != nil {
			return err
		}
		allowOptionalParameter, err := cp.validateOptionalSignature(method, resolved)
		if err != nil {
			return err
		}
		if err := cp.validateParams(decl.Name, method.Name, method.Params, allowResultParameter, allowRecordParameter, allowOptionalParameter); err != nil {
			return err
		}
	}
	if cp.modules != nil && hasPureCallSourceContract(cp.modules.LanguageContract()) {
		if err := cp.bindPureCalls(decl); err != nil {
			return err
		}
	}
	for _, field := range decl.Fields {
		if field.Default == nil {
			continue
		}
		if containsCallExpression(field.Default) {
			return oneDiagnostic(cp.sources, CodeExpressionType, CategorySemantic, field.Default.SourceSpan(), "v0.36.0 same-class calls are admitted only in public expression-bodied methods")
		}
		if containsConditionalExpression(field.Default) {
			return oneDiagnostic(cp.sources, CodeExpressionType, CategorySemantic, field.Default.SourceSpan(), "v0.38.0 conditional expressions are admitted only in expression-bodied methods")
		}
		if containsOptionalExpression(field.Default) {
			return oneDiagnostic(cp.sources, CodeExpressionType, CategorySemantic, field.Default.SourceSpan(), fmt.Sprintf("%s primitive Optional expressions are admitted only as complete class method bodies", cp.modules.LanguageContract()))
		}
		if containsListExpression(field.Default) {
			return oneDiagnostic(cp.sources, CodeExpressionType, CategorySemantic, field.Default.SourceSpan(), fmt.Sprintf("%s record-list expressions are admitted only as complete class method bodies", cp.modules.LanguageContract()))
		}
		if containsResultExpression(field.Default) {
			return oneDiagnostic(cp.sources, CodeExpressionType, CategorySemantic, field.Default.SourceSpan(), fmt.Sprintf("%s snapshot Result expressions are admitted only as complete class method bodies", cp.modules.LanguageContract()))
		}
		if containsCaseFoldedTextExpression(field.Default) {
			return oneDiagnostic(cp.sources, CodeExpressionType, CategorySemantic, field.Default.SourceSpan(), fmt.Sprintf("%s contains_casefolded is admitted only as a complete class method body", cp.modules.LanguageContract()))
		}
		if containsTextTrimExpression(field.Default) {
			return oneDiagnostic(cp.sources, CodeExpressionType, CategorySemantic, field.Default.SourceSpan(), fmt.Sprintf("%s trim is admitted only as a complete class method body", cp.modules.LanguageContract()))
		}
		inferred, err := cp.inferExprType(field.Default, map[string]ResolvedTypeRef{})
		if err != nil {
			return prefixDiagnostic(err, fmt.Sprintf("class %s field %s default: ", decl.Name, field.Name))
		}
		declared := fieldTypes[field.Name]
		if !inferred.Equal(declared) {
			return oneDiagnostic(cp.sources, CodeExpressionType, CategorySemantic, field.Default.SourceSpan(), fmt.Sprintf("class %s field %s default type %s does not match %s", decl.Name, field.Name, inferred, declared), RelatedSpan{Span: field.Type.Span, Message: "declared field type"})
		}
	}
	for _, method := range decl.Methods {
		languageContract := cp.modules.LanguageContract()
		contract := languageContract
		if isV740OrLaterSourceContract(contract) {
			contract = PipeLangLanguageContractV730
		}
		if _, local := method.Body.(*ImmutableLocalExpr); local {
			if normalizeVisibility(method.Visibility) != VisibilityPublic {
				return oneDiagnostic(cp.sources, CodeInvalidMember, CategorySemantic, method.Span, "v0.39.0 immutable local blocks are admitted only in public methods")
			}
			if containsPropagationExpression(method.Body) && !hasBlockPropagationSourceContract(cp.modules.LanguageContract()) {
				return oneDiagnostic(cp.sources, CodePropagation, CategorySemantic, method.Body.SourceSpan(), fmt.Sprintf("%s immutable local initializer and return exclude propagation", cp.modules.LanguageContract()))
			}
		}
		if (contract == PipeLangLanguageContractV480 || contract == PipeLangLanguageContractV490 || contract == PipeLangLanguageContractV500 || contract == PipeLangLanguageContractV510 || (contract == PipeLangLanguageContractV730 || contract == PipeLangLanguageContractV720 || contract == PipeLangLanguageContractV710 || contract == PipeLangLanguageContractV700 || contract == PipeLangLanguageContractV690 || contract == PipeLangLanguageContractV680 || contract == PipeLangLanguageContractV670 || contract == PipeLangLanguageContractV660 || contract == PipeLangLanguageContractV650 || contract == PipeLangLanguageContractV640 || contract == PipeLangLanguageContractV630 || contract == PipeLangLanguageContractV620 || contract == PipeLangLanguageContractV610 || contract == PipeLangLanguageContractV600 || contract == PipeLangLanguageContractV590 || contract == PipeLangLanguageContractV580) || contract == PipeLangLanguageContractV570 || contract == PipeLangLanguageContractV560 || contract == PipeLangLanguageContractV550 || contract == PipeLangLanguageContractV540 || contract == PipeLangLanguageContractV530 || contract == PipeLangLanguageContractV520) && countMatchExpressions(method.Body) != 0 {
			var err error
			if (contract == PipeLangLanguageContractV730 || contract == PipeLangLanguageContractV720 || contract == PipeLangLanguageContractV710 || contract == PipeLangLanguageContractV700 || contract == PipeLangLanguageContractV690 || contract == PipeLangLanguageContractV680 || contract == PipeLangLanguageContractV670 || contract == PipeLangLanguageContractV660 || contract == PipeLangLanguageContractV650 || contract == PipeLangLanguageContractV640 || contract == PipeLangLanguageContractV630 || contract == PipeLangLanguageContractV620 || contract == PipeLangLanguageContractV610 || contract == PipeLangLanguageContractV600 || contract == PipeLangLanguageContractV590 || contract == PipeLangLanguageContractV580) || contract == PipeLangLanguageContractV570 || contract == PipeLangLanguageContractV560 || contract == PipeLangLanguageContractV550 || contract == PipeLangLanguageContractV540 || contract == PipeLangLanguageContractV530 || contract == PipeLangLanguageContractV520 {
				err = cp.validateV520MatchMethod(method)
			} else if contract == PipeLangLanguageContractV510 {
				err = cp.validateV510MatchMethod(method)
			} else if contract == PipeLangLanguageContractV500 {
				err = cp.validateV500MatchMethod(method)
			} else if contract == PipeLangLanguageContractV490 {
				err = cp.validateV490MatchMethod(method)
			} else {
				err = cp.validateV480MatchMethod(method)
			}
			if err != nil {
				return err
			}
		}
		if contract != PipeLangLanguageContractV480 && contract != PipeLangLanguageContractV490 && contract != PipeLangLanguageContractV500 && contract != PipeLangLanguageContractV510 && (contract != PipeLangLanguageContractV730 && contract != PipeLangLanguageContractV720 && contract != PipeLangLanguageContractV710 && contract != PipeLangLanguageContractV700 && contract != PipeLangLanguageContractV690 && contract != PipeLangLanguageContractV680 && contract != PipeLangLanguageContractV670 && contract != PipeLangLanguageContractV660 && contract != PipeLangLanguageContractV650 && contract != PipeLangLanguageContractV640 && contract != PipeLangLanguageContractV630 && contract != PipeLangLanguageContractV620 && contract != PipeLangLanguageContractV610 && contract != PipeLangLanguageContractV600 && contract != PipeLangLanguageContractV590 && contract != PipeLangLanguageContractV580) && contract != PipeLangLanguageContractV570 && contract != PipeLangLanguageContractV560 && contract != PipeLangLanguageContractV550 && contract != PipeLangLanguageContractV540 && contract != PipeLangLanguageContractV530 && contract != PipeLangLanguageContractV520 {
			if _, _, priorLocalCarrier := findPriorLocalCarrierMatch(method.Body); priorLocalCarrier {
				return oneDiagnostic(cp.sources, CodeExpressionType, CategorySemantic, method.Body.SourceSpan(), "prior-local carrier matching requires language contract v0.48.0")
			}
		}
		if containsCallExpression(method.Body) {
			if contract == PipeLangLanguageContractV360 && !validPureCallPlacement(method.Body) {
				return oneDiagnostic(cp.sources, CodeExpressionType, CategorySemantic, method.Body.SourceSpan(), "v0.36.0 same-class calls must be the complete method body or a directly nested call argument")
			}
			if (contract == PipeLangLanguageContractV370 || contract == PipeLangLanguageContractV380 || contract == PipeLangLanguageContractV390 || contract == PipeLangLanguageContractV400 || contract == PipeLangLanguageContractV410 || contract == PipeLangLanguageContractV420) && !validGeneralPureCallPlacement(method.Body) {
				return oneDiagnostic(cp.sources, CodeExpressionType, CategorySemantic, method.Body.SourceSpan(), fmt.Sprintf("%s composed pure calls retain direct match and propagate carriers", contract))
			}
			if contract == PipeLangLanguageContractV430 || contract == PipeLangLanguageContractV440 || contract == PipeLangLanguageContractV450 || contract == PipeLangLanguageContractV460 || contract == PipeLangLanguageContractV470 || contract == PipeLangLanguageContractV480 || contract == PipeLangLanguageContractV490 || contract == PipeLangLanguageContractV500 || contract == PipeLangLanguageContractV510 || (contract == PipeLangLanguageContractV730 || contract == PipeLangLanguageContractV720 || contract == PipeLangLanguageContractV710 || contract == PipeLangLanguageContractV700 || contract == PipeLangLanguageContractV690 || contract == PipeLangLanguageContractV680 || contract == PipeLangLanguageContractV670 || contract == PipeLangLanguageContractV660 || contract == PipeLangLanguageContractV650 || contract == PipeLangLanguageContractV640 || contract == PipeLangLanguageContractV630 || contract == PipeLangLanguageContractV620 || contract == PipeLangLanguageContractV610 || contract == PipeLangLanguageContractV600 || contract == PipeLangLanguageContractV590 || contract == PipeLangLanguageContractV580) || contract == PipeLangLanguageContractV570 || contract == PipeLangLanguageContractV560 || contract == PipeLangLanguageContractV550 || contract == PipeLangLanguageContractV540 || contract == PipeLangLanguageContractV530 || contract == PipeLangLanguageContractV520 {
				if contract == PipeLangLanguageContractV450 || contract == PipeLangLanguageContractV460 || contract == PipeLangLanguageContractV470 {
					if _, _, _, helperCarrier := findHelperCarrierMatchLocal(method.Body); helperCarrier {
						if err := cp.validateHelperCarrierMatchLocalMethod(method); err != nil {
							return err
						}
					}
				}
				if match, ok := method.Body.(*MatchExpr); ok {
					if _, helperCarrier := match.Value.(*CallExpr); helperCarrier {
						var err error
						if contract == PipeLangLanguageContractV440 || contract == PipeLangLanguageContractV450 || contract == PipeLangLanguageContractV460 || contract == PipeLangLanguageContractV470 {
							err = cp.validateGeneralHelperCarrierMatchMethod(method)
						} else {
							err = cp.validateHelperResultMatchMethod(method)
						}
						if err != nil {
							return err
						}
					}
				}
				validPlacement := validHelperCarrierMatchPureCallPlacement(method.Body)
				if contract == PipeLangLanguageContractV450 || contract == PipeLangLanguageContractV460 {
					validPlacement = validHelperCarrierMatchLocalPureCallPlacement(method.Body)
				} else if contract == PipeLangLanguageContractV470 || contract == PipeLangLanguageContractV480 || contract == PipeLangLanguageContractV490 || contract == PipeLangLanguageContractV500 || contract == PipeLangLanguageContractV510 || (contract == PipeLangLanguageContractV730 || contract == PipeLangLanguageContractV720 || contract == PipeLangLanguageContractV710 || contract == PipeLangLanguageContractV700 || contract == PipeLangLanguageContractV690 || contract == PipeLangLanguageContractV680 || contract == PipeLangLanguageContractV670 || contract == PipeLangLanguageContractV660 || contract == PipeLangLanguageContractV650 || contract == PipeLangLanguageContractV640 || contract == PipeLangLanguageContractV630 || contract == PipeLangLanguageContractV620 || contract == PipeLangLanguageContractV610 || contract == PipeLangLanguageContractV600 || contract == PipeLangLanguageContractV590 || contract == PipeLangLanguageContractV580) || contract == PipeLangLanguageContractV570 || contract == PipeLangLanguageContractV560 || contract == PipeLangLanguageContractV550 || contract == PipeLangLanguageContractV540 || contract == PipeLangLanguageContractV530 || contract == PipeLangLanguageContractV520 {
					validPlacement = validHelperCarrierMatchLaterLocalPureCallPlacement(method.Body)
				}
				if !validPlacement {
					if contract == PipeLangLanguageContractV430 {
						return oneDiagnostic(cp.sources, CodeExpressionType, CategorySemantic, method.Body.SourceSpan(), "v0.43.0 admits helper calls only as one complete Result<string, string> match carrier over the direct caller parameter; nested match and computed carriers remain excluded")
					}
					if contract == PipeLangLanguageContractV450 || contract == PipeLangLanguageContractV460 {
						return oneDiagnostic(cp.sources, CodeExpressionType, CategorySemantic, method.Body.SourceSpan(), fmt.Sprintf("%s admits one bounded helper-carrier match only as the first immutable-local initializer or inherited complete method body; later, nested, and computed match carriers remain excluded", contract))
					}
					if contract == PipeLangLanguageContractV470 {
						return oneDiagnostic(cp.sources, CodeExpressionType, CategorySemantic, method.Body.SourceSpan(), "v0.47.0 admits one bounded helper-carrier match only as an immutable-local initializer or inherited complete method body; terminal-return, nested, and computed match carriers remain excluded")
					}
					if contract == PipeLangLanguageContractV480 {
						return oneDiagnostic(cp.sources, CodeExpressionType, CategorySemantic, method.Body.SourceSpan(), "v0.48.0 admits one bounded helper-carrier match only in an inherited placement or as an adjacent helper-call carrier local and match local")
					}
					if contract == PipeLangLanguageContractV490 {
						return oneDiagnostic(cp.sources, CodeExpressionType, CategorySemantic, method.Body.SourceSpan(), "v0.49.0 admits either one inherited bounded match or exactly two adjacent helper-call carrier and match-local pairs")
					}
					if contract == PipeLangLanguageContractV500 {
						return oneDiagnostic(cp.sources, CodeExpressionType, CategorySemantic, method.Body.SourceSpan(), "v0.50.0 admits inherited bounded matches or one contiguous dependent second carrier stage")
					}
					if contract == PipeLangLanguageContractV510 {
						return oneDiagnostic(cp.sources, CodeExpressionType, CategorySemantic, method.Body.SourceSpan(), "v0.51.0 admits inherited bounded matches or one contiguous dependent carrier chain")
					}
					if (contract == PipeLangLanguageContractV730 || contract == PipeLangLanguageContractV720 || contract == PipeLangLanguageContractV710 || contract == PipeLangLanguageContractV700 || contract == PipeLangLanguageContractV690 || contract == PipeLangLanguageContractV680 || contract == PipeLangLanguageContractV670 || contract == PipeLangLanguageContractV660 || contract == PipeLangLanguageContractV650 || contract == PipeLangLanguageContractV640 || contract == PipeLangLanguageContractV630 || contract == PipeLangLanguageContractV620 || contract == PipeLangLanguageContractV610 || contract == PipeLangLanguageContractV600 || contract == PipeLangLanguageContractV590 || contract == PipeLangLanguageContractV580) || contract == PipeLangLanguageContractV570 || contract == PipeLangLanguageContractV560 || contract == PipeLangLanguageContractV550 || contract == PipeLangLanguageContractV540 || contract == PipeLangLanguageContractV530 || contract == PipeLangLanguageContractV520 {
						return oneDiagnostic(cp.sources, CodeExpressionType, CategorySemantic, method.Body.SourceSpan(), "v0.52.0 admits inherited bounded matches, one v0.51 immediate-only chain, or one cumulative fan-in chain")
					}
					return oneDiagnostic(cp.sources, CodeExpressionType, CategorySemantic, method.Body.SourceSpan(), fmt.Sprintf("%s admits helper calls only as one complete match carrier over direct caller parameters; nested match and computed carriers remain excluded", contract))
				}
			}
		}
		terminalIf := containsTerminalIfStatement(method.Body)
		if containsConditionalExpression(method.Body) && (!hasConditionalSourceContract(languageContract) || (!terminalIf && !validBoundedConditionalExpression(method.Body) && !((languageContract == PipeLangLanguageContractV850 || (languageContract == PipeLangLanguageContractV870 || languageContract == PipeLangLanguageContractV860)) && validStraightLineConditionalLocals(method.Body)) && !((languageContract == PipeLangLanguageContractV870 || languageContract == PipeLangLanguageContractV860) && validConditionalReturnComposition(method.Body)))) {
			if languageContract == PipeLangLanguageContractV870 {
				return oneDiagnostic(cp.sources, CodeExpressionType, CategorySemantic, method.Body.SourceSpan(), "v0.87.0 admits complete nonnested ternary returns in straight-line methods and terminal-tree leaves; nested ternaries and new argument/condition placements remain excluded")
			}
			if languageContract == PipeLangLanguageContractV860 {
				return oneDiagnostic(cp.sources, CodeExpressionType, CategorySemantic, method.Body.SourceSpan(), "v0.86.0 admits complete ternary returns after straight-line typed conditional locals; nested ternaries, new argument/condition placements, terminal-tree return choices, match, and propagation combinations remain excluded")
			}
			if languageContract == PipeLangLanguageContractV850 {
				return oneDiagnostic(cp.sources, CodeExpressionType, CategorySemantic, method.Body.SourceSpan(), "v0.85.0 admits finite conditional sequences only as complete typed immutable-local initializers followed by an ordinary return or an inherited terminal tree; nested ternaries, new return/condition/argument placements, match, and propagation combinations remain excluded")
			}
			return oneDiagnostic(cp.sources, CodeExpressionType, CategorySemantic, method.Body.SourceSpan(), "v0.38.0 admits exactly one conditional over existing eager pure expressions; nested conditionals, match, and propagate operands are excluded")
		}
		if terminalIf {
			if !hasTerminalIfSourceContract(languageContract) || !validTerminalIfStatement(languageContract, method.Body) || containsPropagationExpression(method.Body) || countMatchExpressions(method.Body) != 0 {
				if languageContract == PipeLangLanguageContractV870 {
					return oneDiagnostic(cp.sources, CodeExpressionType, CategorySemantic, method.Body.SourceSpan(), "v0.87.0 admits complete nonnested ternary returns in terminal-tree leaves through statement depth three; deeper trees, nested ternaries, new argument/condition placements, match, propagation and fallthrough remain excluded")
				}
				if (languageContract == PipeLangLanguageContractV870 || languageContract == PipeLangLanguageContractV860) || languageContract == PipeLangLanguageContractV850 || languageContract == PipeLangLanguageContractV840 {
					return oneDiagnostic(cp.sources, CodeExpressionType, CategorySemantic, method.Body.SourceSpan(), "v0.84.0 admits finite ternary sequences only as complete typed immutable-local initializers in terminal trees through depth three; nested ternaries, new return/condition/argument placements, propagation, match, and fallthrough remain excluded")
				}
				if languageContract == PipeLangLanguageContractV830 {
					return oneDiagnostic(cp.sources, CodeExpressionType, CategorySemantic, method.Body.SourceSpan(), "v0.83.0 admits at most two ternaries, each a complete typed immutable-local initializer in terminal trees through depth three; third or nested ternaries, new return/condition/argument placements, propagation, match, and fallthrough remain excluded")
				}
				if languageContract == PipeLangLanguageContractV820 {
					return oneDiagnostic(cp.sources, CodeExpressionType, CategorySemantic, method.Body.SourceSpan(), "v0.82.0 admits one ternary only as a complete typed immutable-local initializer in terminal trees through depth three; new return/condition placements, nested or multiple ternaries, propagation, match, and fallthrough remain excluded")
				}
				if languageContract == PipeLangLanguageContractV810 {
					return oneDiagnostic(cp.sources, CodeExpressionType, CategorySemantic, method.Body.SourceSpan(), "v0.81.0 admits terminal if/else trees through depth three with typed immutable locals; depth four, new conditional-expression combinations, propagation, match, and fallthrough remain excluded")
				}
				if languageContract == PipeLangLanguageContractV800 {
					return oneDiagnostic(cp.sources, CodeExpressionType, CategorySemantic, method.Body.SourceSpan(), "v0.80.0 additionally admits exactly two expanded leaves on a rootful or rootless symmetric depth-two terminal if/else base; a third expansion, depth four, asymmetric bases, conditional expressions, propagation, match, and fallthrough remain excluded")
				}
				if languageContract == PipeLangLanguageContractV790 {
					return oneDiagnostic(cp.sources, CodeExpressionType, CategorySemantic, method.Body.SourceSpan(), "v0.79.0 admits an inherited rootful or rootless symmetric depth-two terminal if/else with exactly one of its four leaves expanded into one third-level terminal if/else; additional expansion or depth, conditional expressions, propagation, match, and fallthrough remain excluded")
				}
				if languageContract == PipeLangLanguageContractV780 {
					return oneDiagnostic(cp.sources, CodeExpressionType, CategorySemantic, method.Body.SourceSpan(), "v0.78.0 admits rootless symmetric depth-two terminal if/else with typed lexical locals; additional nesting, conditional expressions, propagation, match, and fallthrough within the topology remain excluded")
				}
				if languageContract == PipeLangLanguageContractV770 {
					return oneDiagnostic(cp.sources, CodeExpressionType, CategorySemantic, method.Body.SourceSpan(), "v0.77.0 permits one or more top-level typed immutable locals before an outer terminal if/else whose two branches each end in one inner terminal if/else; every inner leaf may contain finite local sequences, while additional depth, conditional expressions within the topology, propagation, match, and fallthrough are excluded")
				}
				if languageContract == PipeLangLanguageContractV760 {
					return oneDiagnostic(cp.sources, CodeExpressionType, CategorySemantic, method.Body.SourceSpan(), "v0.76.0 permits one or more top-level typed immutable locals before the exact v0.75.0 one-branch nested terminal if/else; both nested leaves may contain finite local sequences, while nesting in both outer branches, additional depth, propagation, match, conditional expressions within the topology, and fallthrough are excluded")
				}
				if languageContract == PipeLangLanguageContractV750 {
					return oneDiagnostic(cp.sources, CodeExpressionType, CategorySemantic, method.Body.SourceSpan(), "v0.75.0 permits one complete outer terminal if/else with exactly one branch ending in one nested terminal if/else; inner leaves may contain finite typed immutable-local sequences, while propagation, match, additional nesting, and fallthrough are excluded")
				}
				if languageContract == PipeLangLanguageContractV740 {
					return oneDiagnostic(cp.sources, CodeExpressionType, CategorySemantic, method.Body.SourceSpan(), "v0.74.0 permits one complete outer terminal if/else with exactly one branch ending in one nested terminal if/else after any finite immutable-local sequence; nested leaves return directly, and propagation, match, additional nesting, and fallthrough are excluded")
				}
				if contract == PipeLangLanguageContractV730 {
					return oneDiagnostic(cp.sources, CodeExpressionType, CategorySemantic, method.Body.SourceSpan(), "v0.73.0 terminal if/else may be the complete method body and permits any finite sequence of explicitly typed ordered immutable locals per branch; branch locals are lexical, and propagation, match, nested branches, and fallthrough are excluded")
				}
				if contract == PipeLangLanguageContractV720 {
					return oneDiagnostic(cp.sources, CodeExpressionType, CategorySemantic, method.Body.SourceSpan(), "v0.72.0 terminal if/else permits any finite sequence of explicitly typed ordered immutable locals per branch; branch locals are lexical, and propagation, match, nested branches, and fallthrough are excluded")
				}
				if contract == PipeLangLanguageContractV710 {
					return oneDiagnostic(cp.sources, CodeExpressionType, CategorySemantic, method.Body.SourceSpan(), "v0.71.0 terminal if/else permits at most two explicitly typed ordered immutable locals per branch; branch locals are lexical, and propagation, match, nested branches, three branch locals, and fallthrough are excluded")
				}
				if contract == PipeLangLanguageContractV700 {
					return oneDiagnostic(cp.sources, CodeExpressionType, CategorySemantic, method.Body.SourceSpan(), "v0.70.0 terminal if/else permits at most one explicitly typed immutable local per branch; branch locals are lexical, and propagation, match, nested branches, multiple branch locals, and fallthrough are excluded")
				}
				return oneDiagnostic(cp.sources, CodeExpressionType, CategorySemantic, method.Body.SourceSpan(), "v0.69.0 terminal if/else requires one or more ordered immutable locals followed by one terminal bool branch over existing eager pure expressions; propagation, match, nested branches, branch locals, and fallthrough are excluded")
			}
		}
		env := map[string]ResolvedTypeRef{}
		for name, fieldType := range fieldTypes {
			env[name] = fieldType
		}
		for _, param := range method.Params {
			paramType, err := cp.resolveType(param.Type, RelatedSpan{Span: param.Span, Message: "parameter declaration"})
			if err != nil {
				return err
			}
			env[param.Name] = paramType
		}
		declared, err := cp.resolveType(method.ReturnType)
		if err != nil {
			return err
		}
		if _, local := method.Body.(*ImmutableLocalExpr); local && containsPropagationExpression(method.Body) {
			if err := cp.validateBlockPropagationMethod(method, declared); err != nil {
				return err
			}
		}
		inferred, err := cp.inferMethodBodyType(method, env, declared)
		if err != nil {
			return prefixDiagnostic(err, fmt.Sprintf("class %s method %s: ", decl.Name, method.Name))
		}
		if !inferred.Equal(declared) {
			return oneDiagnostic(cp.sources, CodeExpressionType, CategorySemantic, method.Body.SourceSpan(), fmt.Sprintf("class %s method %s returns %s but declared %s", decl.Name, method.Name, inferred, declared), RelatedSpan{Span: method.ReturnType.Span, Message: "declared return type"})
		}
	}
	return nil
}

func (cp *checkedProgram) validateHelperResultMatchMethod(method MethodDecl) error {
	fail := func(span Span, detail string) error {
		return oneDiagnostic(cp.sources, CodeExpressionType, CategorySemantic, span, "v0.43.0 helper-result matching "+detail)
	}
	match, ok := method.Body.(*MatchExpr)
	if !ok || countMatchExpressions(method.Body) != 1 {
		return fail(method.Body.SourceSpan(), "requires exactly one complete method-body match")
	}
	call, ok := match.Value.(*CallExpr)
	if !ok {
		return fail(match.Value.SourceSpan(), "requires one same-class helper call as the match carrier")
	}
	text := resolvedPrimitive(TypeString)
	declared, err := cp.resolveType(method.ReturnType)
	if err != nil {
		return err
	}
	if normalizeVisibility(method.Visibility) != VisibilityPublic || len(method.Params) != 1 || !declared.Equal(text) {
		return fail(method.Span, "requires one public string-returning method with one direct string parameter")
	}
	parameter, err := cp.resolveType(method.Params[0].Type)
	if err != nil {
		return err
	}
	if len(call.Arguments) != 1 {
		return fail(call.Span, "requires the sole direct string parameter as the helper argument")
	}
	argument, direct := call.Arguments[0].(*IdentExpr)
	if !direct || argument.Name != method.Params[0].Name || !parameter.Equal(text) {
		return fail(call.Span, "requires the sole direct string parameter as the helper argument")
	}
	_, helper := methodBySpan(cp.program, call.TargetSpan)
	if helper == nil || normalizeVisibility(helper.Visibility) != VisibilityPublic || len(helper.Params) != 1 {
		return fail(call.NameSpan, "requires one resolved public same-class helper")
	}
	helperParameter, err := cp.resolveType(helper.Params[0].Type)
	if err != nil {
		return err
	}
	helperResult, err := cp.resolveType(helper.ReturnType)
	if err != nil {
		return err
	}
	if !helperParameter.Equal(text) || !isResolvedTextResult(helperResult) {
		return fail(call.Span, "requires an exact string -> Result<string, string> helper signature")
	}
	if len(match.Arms) != 2 || match.Arms[0].Tag != "ok" || match.Arms[0].Binding == "" || match.Arms[1].Tag != "err" || match.Arms[1].Binding == "" {
		return fail(match.Span, "requires source-ordered ok(binding) and err(binding) arms")
	}
	return nil
}

func (cp *checkedProgram) validateGeneralHelperCarrierMatchMethod(method MethodDecl) error {
	match, ok := method.Body.(*MatchExpr)
	if !ok || countMatchExpressions(method.Body) != 1 {
		return oneDiagnostic(cp.sources, CodeExpressionType, CategorySemantic, method.Body.SourceSpan(), "v0.44.0 helper-carrier matching requires exactly one complete method-body match")
	}
	return cp.validateGeneralHelperCarrierMatch(method, match, "v0.44.0 helper-carrier matching", false)
}

func (cp *checkedProgram) validateV480MatchMethod(method MethodDecl) error {
	if carrier, match, ok := findPriorLocalCarrierMatch(method.Body); ok {
		if countMatchExpressions(method.Body) != 1 {
			return oneDiagnostic(cp.sources, CodeExpressionType, CategorySemantic, match.Span, "v0.48.0 prior-local carrier matching requires exactly one match in the immutable-local sequence")
		}
		synthetic := *match
		synthetic.Value = carrier.Initializer
		return cp.validateGeneralHelperCarrierMatch(method, &synthetic, "v0.48.0 prior-local carrier match", true)
	}
	if _, match, _, ok := findHelperCarrierMatchLocal(method.Body); ok {
		if countMatchExpressions(method.Body) != 1 {
			return oneDiagnostic(cp.sources, CodeExpressionType, CategorySemantic, match.Span, "v0.48.0 inherited helper-carrier matching requires exactly one match in the immutable-local sequence")
		}
		return cp.validateGeneralHelperCarrierMatch(method, match, "v0.47.0 later-local helper-carrier match", true)
	}
	if match, ok := method.Body.(*MatchExpr); ok {
		if _, helper := match.Value.(*CallExpr); helper {
			return cp.validateGeneralHelperCarrierMatch(method, match, "v0.44.0 helper-carrier matching", false)
		}
		if identifier, direct := match.Value.(*IdentExpr); direct && countMatchExpressions(method.Body) == 1 {
			for _, parameter := range method.Params {
				if identifier.Name == parameter.Name {
					return nil
				}
			}
		}
	}
	return oneDiagnostic(cp.sources, CodeExpressionType, CategorySemantic, method.Body.SourceSpan(), "v0.48.0 match must be an inherited complete-body/helper-local form or an adjacent helper-call carrier local and match local")
}

func (cp *checkedProgram) validateV490MatchMethod(method MethodDecl) error {
	matchCount := countMatchExpressions(method.Body)
	if matchCount == 1 {
		return cp.validateV480MatchMethod(method)
	}
	if matchCount != 2 {
		return oneDiagnostic(cp.sources, CodeExpressionType, CategorySemantic, method.Body.SourceSpan(), "v0.49.0 admits either one inherited bounded match or exactly two matches in adjacent helper-call carrier and match-local pairs")
	}
	pairs := findPriorLocalCarrierMatches(method.Body)
	if len(pairs) != 2 {
		return oneDiagnostic(cp.sources, CodeExpressionType, CategorySemantic, method.Body.SourceSpan(), "v0.49.0 two-match composition requires exactly two non-overlapping adjacent helper-call carrier and match-local pairs")
	}
	for position, pair := range pairs {
		synthetic := *pair.match
		synthetic.Value = pair.carrier.Initializer
		if err := cp.validateGeneralHelperCarrierMatch(method, &synthetic, fmt.Sprintf("v0.49.0 helper-carrier match pair %d", position+1), true); err != nil {
			return err
		}
	}
	return nil
}

func (cp *checkedProgram) validateV500MatchMethod(method MethodDecl) error {
	matchCount := countMatchExpressions(method.Body)
	if matchCount == 1 {
		return cp.validateV480MatchMethod(method)
	}
	if matchCount != 2 {
		return oneDiagnostic(cp.sources, CodeExpressionType, CategorySemantic, method.Body.SourceSpan(), "v0.50.0 admits inherited bounded matches or exactly two matches in one contiguous dependent carrier stage")
	}
	pairs := findPriorLocalCarrierMatches(method.Body)
	if len(pairs) != 2 {
		return oneDiagnostic(cp.sources, CodeExpressionType, CategorySemantic, method.Body.SourceSpan(), "v0.50.0 dependent composition requires exactly two adjacent carrier and match-local pairs")
	}
	if err := cp.validateV490MatchMethod(method); err == nil {
		return nil
	}
	nextCarrier, contiguous := pairs[0].matched.Return.(*ImmutableLocalExpr)
	if !contiguous || nextCarrier != pairs[1].carrier {
		return oneDiagnostic(cp.sources, CodeExpressionType, CategorySemantic, pairs[1].carrier.Span, "v0.50.0 dependent composition requires four contiguous carrier, match, carrier, and match locals")
	}
	first := *pairs[0].match
	first.Value = pairs[0].carrier.Initializer
	if err := cp.validateGeneralHelperCarrierMatch(method, &first, "v0.50.0 first helper-carrier match pair", true); err != nil {
		return err
	}
	second := *pairs[1].match
	second.Value = pairs[1].carrier.Initializer
	return cp.validateDependentSecondHelperCarrierMatch(method, pairs[0].matched, &second)
}

func (cp *checkedProgram) validateV510MatchMethod(method MethodDecl) error {
	matchCount := countMatchExpressions(method.Body)
	if matchCount <= 2 {
		return cp.validateV500MatchMethod(method)
	}
	pairs := findPriorLocalCarrierMatches(method.Body)
	if len(pairs) != matchCount {
		return oneDiagnostic(cp.sources, CodeExpressionType, CategorySemantic, method.Body.SourceSpan(), "v0.51.0 dependent carrier chain requires every match in an adjacent helper-call carrier and match-local pair")
	}
	for position := 1; position < len(pairs); position++ {
		nextCarrier, contiguous := pairs[position-1].matched.Return.(*ImmutableLocalExpr)
		if !contiguous || nextCarrier != pairs[position].carrier {
			return oneDiagnostic(cp.sources, CodeExpressionType, CategorySemantic, pairs[position].carrier.Span, "v0.51.0 dependent carrier chain requires one contiguous carrier and match-local sequence")
		}
	}
	first := *pairs[0].match
	first.Value = pairs[0].carrier.Initializer
	if err := cp.validateGeneralHelperCarrierMatch(method, &first, "v0.51.0 first helper-carrier match pair", true); err != nil {
		return err
	}
	for position := 1; position < len(pairs); position++ {
		matched := *pairs[position].match
		matched.Value = pairs[position].carrier.Initializer
		if err := cp.validateDependentHelperCarrierMatch(method, pairs[position-1].matched, &matched, fmt.Sprintf("v0.51.0 dependent carrier chain stage %d", position+1), "immediately preceding selected local", matchCount); err != nil {
			return err
		}
	}
	return nil
}

func (cp *checkedProgram) validateV520MatchMethod(method MethodDecl) error {
	matchCount := countMatchExpressions(method.Body)
	if matchCount <= 2 {
		return cp.validateV500MatchMethod(method)
	}
	if err := cp.validateV510MatchMethod(method); err == nil {
		return nil
	}
	pairs := findPriorLocalCarrierMatches(method.Body)
	if len(pairs) != matchCount {
		return oneDiagnostic(cp.sources, CodeExpressionType, CategorySemantic, method.Body.SourceSpan(), "v0.52.0 cumulative fan-in chain requires every match in an adjacent helper-call carrier and match-local pair")
	}
	for position := 1; position < len(pairs); position++ {
		nextCarrier, contiguous := pairs[position-1].matched.Return.(*ImmutableLocalExpr)
		if !contiguous || nextCarrier != pairs[position].carrier {
			return oneDiagnostic(cp.sources, CodeExpressionType, CategorySemantic, pairs[position].carrier.Span, "v0.52.0 cumulative fan-in chain requires one contiguous carrier and match-local sequence")
		}
	}
	first := *pairs[0].match
	first.Value = pairs[0].carrier.Initializer
	if err := cp.validateGeneralHelperCarrierMatch(method, &first, "v0.52.0 first helper-carrier match pair", true); err != nil {
		return err
	}
	for position := 1; position < len(pairs); position++ {
		matched := *pairs[position].match
		matched.Value = pairs[position].carrier.Initializer
		selected := make([]*ImmutableLocalExpr, position)
		for prior := 0; prior < position; prior++ {
			selected[prior] = pairs[prior].matched
		}
		if err := cp.validateSelectedHelperCarrierMatch(method, selected, &matched, fmt.Sprintf("v0.52.0 cumulative fan-in chain stage %d", position+1), "every prior selected local in chain order", matchCount); err != nil {
			return err
		}
	}
	return nil
}

func (cp *checkedProgram) validateDependentSecondHelperCarrierMatch(method MethodDecl, firstSelected *ImmutableLocalExpr, match *MatchExpr) error {
	return cp.validateDependentHelperCarrierMatch(method, firstSelected, match, "v0.50.0 dependent second helper-carrier match", "first selected local", 2)
}

func (cp *checkedProgram) validateDependentHelperCarrierMatch(method MethodDecl, previousSelected *ImmutableLocalExpr, match *MatchExpr, contractName string, selectedDescription string, expectedMatches int) error {
	return cp.validateSelectedHelperCarrierMatch(method, []*ImmutableLocalExpr{previousSelected}, match, contractName, selectedDescription, expectedMatches)
}

func (cp *checkedProgram) validateSelectedHelperCarrierMatch(method MethodDecl, selectedLocals []*ImmutableLocalExpr, match *MatchExpr, contractName string, selectedDescription string, expectedMatches int) error {
	fail := func(span Span, detail string) error {
		return oneDiagnostic(cp.sources, CodeExpressionType, CategorySemantic, span, contractName+" "+detail)
	}
	call, ok := match.Value.(*CallExpr)
	if !ok || len(selectedLocals) == 0 || countMatchExpressions(method.Body) != expectedMatches {
		return fail(match.Value.SourceSpan(), fmt.Sprintf("requires one same-class helper call after the %s in a method with exactly %d matches", selectedDescription, expectedMatches))
	}
	for _, selected := range selectedLocals {
		if selected == nil {
			return fail(match.Value.SourceSpan(), "requires a complete selected-local dependency sequence")
		}
	}
	if normalizeVisibility(method.Visibility) != VisibilityPublic || len(method.Params) == 0 {
		return fail(method.Span, "requires one public caller with one or more direct parameters")
	}
	if len(call.Arguments) != len(method.Params)+len(selectedLocals) {
		return fail(call.Span, "requires the "+selectedDescription+" followed by every caller parameter exactly once in declaration order")
	}
	for position, selected := range selectedLocals {
		selectedArgument, direct := call.Arguments[position].(*IdentExpr)
		if !direct || selectedArgument.Name != selected.Name {
			return fail(call.Arguments[position].SourceSpan(), fmt.Sprintf("requires the %s as helper argument %d", selectedDescription, position+1))
		}
	}
	callerOwner, _ := methodBySpan(cp.program, method.Span)
	helperOwner, helper := methodBySpan(cp.program, call.TargetSpan)
	if callerOwner == nil || helperOwner == nil || helper == nil || callerOwner.Span != helperOwner.Span || normalizeVisibility(helper.Visibility) != VisibilityPublic || len(helper.Params) != len(method.Params)+len(selectedLocals) {
		return fail(call.NameSpan, "requires one resolved public same-class helper with the exact dependent signature")
	}
	for position, selected := range selectedLocals {
		selectedType, err := cp.resolveType(selected.Type)
		if err != nil {
			return err
		}
		helperSelectedType, err := cp.resolveType(helper.Params[position].Type)
		if err != nil {
			return err
		}
		if !selectedType.Equal(helperSelectedType) {
			return fail(call.Arguments[position].SourceSpan(), fmt.Sprintf("requires helper parameter %d to exactly match the %s type", position+1, selectedDescription))
		}
	}
	for position, parameter := range method.Params {
		offset := position + len(selectedLocals)
		argument := call.Arguments[offset]
		identifier, direct := argument.(*IdentExpr)
		if !direct || identifier.Name != parameter.Name {
			return fail(argument.SourceSpan(), "requires every caller parameter exactly once after the "+selectedDescription+" in declaration order")
		}
		callerType, err := cp.resolveType(parameter.Type)
		if err != nil {
			return err
		}
		helperType, err := cp.resolveType(helper.Params[offset].Type)
		if err != nil {
			return err
		}
		if !callerType.Equal(helperType) {
			return fail(argument.SourceSpan(), "requires dependent helper parameter types to exactly match the caller parameters")
		}
	}
	carrier, err := cp.resolveType(helper.ReturnType)
	if err != nil {
		return err
	}
	if !cp.isResolvedOptionalValue(cp.modules.LanguageContract(), carrier) && !isResolvedBoundedValueResult(cp.modules.LanguageContract(), carrier) && !isResolvedSourceArithmeticResult(cp.modules.LanguageContract(), carrier) {
		return fail(call.Span, "requires an admitted Optional<T>, bounded value Result, or checked-arithmetic Result helper carrier")
	}
	if isResolvedOptional(carrier) {
		if len(match.Arms) != 2 || match.Arms[0].Tag != "some" || match.Arms[0].Binding == "" || match.Arms[1].Tag != "none" || match.Arms[1].Binding != "" {
			return fail(match.Span, "requires source-ordered some(binding) and none arms")
		}
		return nil
	}
	if len(match.Arms) != 2 || match.Arms[0].Tag != "ok" || match.Arms[0].Binding == "" || match.Arms[1].Tag != "err" || match.Arms[1].Binding == "" {
		return fail(match.Span, "requires source-ordered ok(binding) and err(binding) arms")
	}
	return nil
}

func (cp *checkedProgram) validateHelperCarrierMatchLocalMethod(method MethodDecl) error {
	contractName := "v0.45.0 helper-carrier match local"
	allowArithmetic := false
	if cp.modules.LanguageContract() == PipeLangLanguageContractV460 {
		contractName = "v0.46.0 checked-arithmetic helper-carrier match local"
		allowArithmetic = true
	} else if cp.modules.LanguageContract() == PipeLangLanguageContractV470 {
		contractName = "v0.47.0 later-local helper-carrier match"
		allowArithmetic = true
	}
	local, match, position, ok := findHelperCarrierMatchLocal(method.Body)
	if !ok {
		requirement := " requires one match as the first immutable-local initializer"
		if cp.modules.LanguageContract() == PipeLangLanguageContractV470 {
			requirement = " requires one match as an immutable-local initializer"
		}
		return oneDiagnostic(cp.sources, CodeExpressionType, CategorySemantic, method.Body.SourceSpan(), contractName+requirement)
	}
	if cp.modules.LanguageContract() != PipeLangLanguageContractV470 && position != 0 {
		return oneDiagnostic(cp.sources, CodeExpressionType, CategorySemantic, local.Initializer.SourceSpan(), contractName+" requires one match as the first immutable-local initializer")
	}
	if countMatchExpressions(method.Body) != 1 {
		requirement := " requires exactly one match as the first immutable-local initializer"
		if cp.modules.LanguageContract() == PipeLangLanguageContractV470 {
			requirement = " requires exactly one match in the immutable-local sequence"
		}
		return oneDiagnostic(cp.sources, CodeExpressionType, CategorySemantic, local.Initializer.SourceSpan(), contractName+requirement)
	}
	return cp.validateGeneralHelperCarrierMatch(method, match, contractName, allowArithmetic)
}

func findHelperCarrierMatchLocal(body Expr) (*ImmutableLocalExpr, *MatchExpr, int, bool) {
	local, ok := body.(*ImmutableLocalExpr)
	for position := 0; ok; position++ {
		if match, matched := local.Initializer.(*MatchExpr); matched {
			if _, helperCarrier := match.Value.(*CallExpr); helperCarrier {
				return local, match, position, true
			}
		}
		local, ok = local.Return.(*ImmutableLocalExpr)
	}
	return nil, nil, 0, false
}

func findPriorLocalCarrierMatch(body Expr) (*ImmutableLocalExpr, *MatchExpr, bool) {
	local, ok := body.(*ImmutableLocalExpr)
	for ok {
		next, nextLocal := local.Return.(*ImmutableLocalExpr)
		_, helper := local.Initializer.(*CallExpr)
		if nextLocal && helper {
			if match, matched := next.Initializer.(*MatchExpr); matched {
				if reference, direct := match.Value.(*IdentExpr); direct && reference.Name == local.Name {
					return local, match, true
				}
			}
		}
		local, ok = local.Return.(*ImmutableLocalExpr)
	}
	return nil, nil, false
}

type priorLocalCarrierMatchPair struct {
	carrier *ImmutableLocalExpr
	matched *ImmutableLocalExpr
	match   *MatchExpr
}

func findPriorLocalCarrierMatches(body Expr) []priorLocalCarrierMatchPair {
	pairs := make([]priorLocalCarrierMatchPair, 0, 2)
	local, ok := body.(*ImmutableLocalExpr)
	for ok {
		next, nextLocal := local.Return.(*ImmutableLocalExpr)
		_, helper := local.Initializer.(*CallExpr)
		if nextLocal && helper {
			if match, matched := next.Initializer.(*MatchExpr); matched {
				if reference, direct := match.Value.(*IdentExpr); direct && reference.Name == local.Name {
					pairs = append(pairs, priorLocalCarrierMatchPair{carrier: local, matched: next, match: match})
					local, ok = next.Return.(*ImmutableLocalExpr)
					continue
				}
			}
		}
		local, ok = local.Return.(*ImmutableLocalExpr)
	}
	return pairs
}

func (cp *checkedProgram) validateGeneralHelperCarrierMatch(method MethodDecl, match *MatchExpr, contractName string, allowArithmetic bool) error {
	fail := func(span Span, detail string) error {
		return oneDiagnostic(cp.sources, CodeExpressionType, CategorySemantic, span, contractName+" "+detail)
	}
	call, ok := match.Value.(*CallExpr)
	if !ok {
		return fail(match.Value.SourceSpan(), "requires one same-class helper call as the match carrier")
	}
	if normalizeVisibility(method.Visibility) != VisibilityPublic || len(method.Params) == 0 {
		return fail(method.Span, "requires one public caller with one or more direct parameters")
	}
	if len(call.Arguments) != len(method.Params) {
		return fail(call.Span, "requires every caller parameter exactly once in declaration order")
	}
	callerOwner, _ := methodBySpan(cp.program, method.Span)
	helperOwner, helper := methodBySpan(cp.program, call.TargetSpan)
	if callerOwner == nil || helperOwner == nil || helper == nil || callerOwner.Span != helperOwner.Span || normalizeVisibility(helper.Visibility) != VisibilityPublic || len(helper.Params) != len(method.Params) {
		return fail(call.NameSpan, "requires one resolved public same-class helper with the exact caller parameter list")
	}
	for position, argument := range call.Arguments {
		identifier, direct := argument.(*IdentExpr)
		if !direct || identifier.Name != method.Params[position].Name {
			return fail(argument.SourceSpan(), "requires every caller parameter exactly once in declaration order")
		}
		callerType, err := cp.resolveType(method.Params[position].Type)
		if err != nil {
			return err
		}
		helperType, err := cp.resolveType(helper.Params[position].Type)
		if err != nil {
			return err
		}
		if !callerType.Equal(helperType) {
			return fail(argument.SourceSpan(), "requires the helper parameter types to exactly match the caller parameters")
		}
	}
	carrier, err := cp.resolveType(helper.ReturnType)
	if err != nil {
		return err
	}
	if !cp.isResolvedOptionalValue(cp.modules.LanguageContract(), carrier) && !isResolvedBoundedValueResult(cp.modules.LanguageContract(), carrier) && !(allowArithmetic && isResolvedSourceArithmeticResult(cp.modules.LanguageContract(), carrier)) {
		if allowArithmetic {
			return fail(call.Span, "requires an admitted Optional<T>, bounded value Result, or checked-arithmetic Result helper carrier")
		}
		return fail(call.Span, "requires an admitted Optional<T>, Result<List<R>, string>, or Result<string, string> helper carrier")
	}
	if isResolvedOptional(carrier) {
		if len(match.Arms) != 2 || match.Arms[0].Tag != "some" || match.Arms[0].Binding == "" || match.Arms[1].Tag != "none" || match.Arms[1].Binding != "" {
			return fail(match.Span, "requires source-ordered some(binding) and none arms")
		}
		return nil
	}
	if len(match.Arms) != 2 || match.Arms[0].Tag != "ok" || match.Arms[0].Binding == "" || match.Arms[1].Tag != "err" || match.Arms[1].Binding == "" {
		return fail(match.Span, "requires source-ordered ok(binding) and err(binding) arms")
	}
	return nil
}

func (cp *checkedProgram) validateBlockPropagationMethod(method MethodDecl, declared ResolvedTypeRef) error {
	outer, ok := method.Body.(*ImmutableLocalExpr)
	if !ok || !hasBlockPropagationSourceContract(cp.modules.LanguageContract()) {
		return oneDiagnostic(cp.sources, CodePropagation, CategorySemantic, method.Body.SourceSpan(), "block-scoped propagation requires language contract v0.41.0")
	}
	if _, direct := outer.Initializer.(*PropagateExpr); direct {
		contract := inheritedLanguageContract(cp.modules.LanguageContract())
		if (contract == PipeLangLanguageContractV730 || contract == PipeLangLanguageContractV720 || contract == PipeLangLanguageContractV710 || contract == PipeLangLanguageContractV700 || contract == PipeLangLanguageContractV690 || contract == PipeLangLanguageContractV680 || contract == PipeLangLanguageContractV670 || contract == PipeLangLanguageContractV660 || contract == PipeLangLanguageContractV650 || contract == PipeLangLanguageContractV640 || contract == PipeLangLanguageContractV630 || contract == PipeLangLanguageContractV620 || contract == PipeLangLanguageContractV610 || contract == PipeLangLanguageContractV600 || contract == PipeLangLanguageContractV590) && len(method.Params) > 0 {
			carrier, err := cp.resolveType(method.Params[0].Type, RelatedSpan{Span: method.Params[0].Span, Message: "cross-payload Result carrier"})
			if err != nil {
				return err
			}
			oneStageContextual := ((contract == PipeLangLanguageContractV730 || contract == PipeLangLanguageContractV720 || contract == PipeLangLanguageContractV710 || contract == PipeLangLanguageContractV700 || contract == PipeLangLanguageContractV690 || contract == PipeLangLanguageContractV680) && len(method.Params) >= 2) || ((contract == PipeLangLanguageContractV670 || contract == PipeLangLanguageContractV660 || contract == PipeLangLanguageContractV650 || contract == PipeLangLanguageContractV640 || contract == PipeLangLanguageContractV630) && len(method.Params) == 2)
			if oneStageContextual && countPropagationExpressions(method.Body) == 1 && isResolvedBoundedValueResult(contract, carrier) && isResolvedBoundedValueResult(contract, declared) {
				return cp.validateSingleStageContextualCrossPayloadResultPropagation(method, declared, outer, carrier)
			}
			if (contract == PipeLangLanguageContractV730 || contract == PipeLangLanguageContractV720 || contract == PipeLangLanguageContractV710 || contract == PipeLangLanguageContractV700 || contract == PipeLangLanguageContractV690 || contract == PipeLangLanguageContractV680 || contract == PipeLangLanguageContractV670 || contract == PipeLangLanguageContractV660 || contract == PipeLangLanguageContractV650 || contract == PipeLangLanguageContractV640 || contract == PipeLangLanguageContractV630 || contract == PipeLangLanguageContractV620) && len(method.Params) >= 2 && countPropagationExpressions(method.Body) >= 2 && isResolvedBoundedValueResult(contract, carrier) && isResolvedBoundedValueResult(contract, declared) {
				return cp.validateContextualCrossPayloadResultPropagation(method, declared, outer, carrier)
			}
			if (contract == PipeLangLanguageContractV730 || contract == PipeLangLanguageContractV720 || contract == PipeLangLanguageContractV710 || contract == PipeLangLanguageContractV700 || contract == PipeLangLanguageContractV690 || contract == PipeLangLanguageContractV680 || contract == PipeLangLanguageContractV670 || contract == PipeLangLanguageContractV660 || contract == PipeLangLanguageContractV650 || contract == PipeLangLanguageContractV640 || contract == PipeLangLanguageContractV630 || contract == PipeLangLanguageContractV620 || contract == PipeLangLanguageContractV610) && countPropagationExpressions(method.Body) >= 2 && isResolvedBoundedValueResult(contract, carrier) && isResolvedBoundedValueResult(contract, declared) {
				return cp.validateGeneralizedCrossPayloadResultPropagation(method, declared, outer, carrier)
			}
			if contract == PipeLangLanguageContractV600 && countPropagationExpressions(method.Body) == 2 && isResolvedBoundedValueResult(contract, carrier) && isResolvedBoundedValueResult(contract, declared) {
				return cp.validateTwoStageCrossPayloadResultPropagation(method, declared, outer, carrier)
			}
			if isResolvedBoundedValueResult(contract, carrier) && isResolvedBoundedValueResult(contract, declared) && !carrier.Arguments[0].Equal(declared.Arguments[0]) {
				return cp.validateCrossPayloadResultPropagation(method, declared, outer, carrier)
			}
		}
		if (contract == PipeLangLanguageContractV730 || contract == PipeLangLanguageContractV720 || contract == PipeLangLanguageContractV710 || contract == PipeLangLanguageContractV700 || contract == PipeLangLanguageContractV690 || contract == PipeLangLanguageContractV680 || contract == PipeLangLanguageContractV670 || contract == PipeLangLanguageContractV660 || contract == PipeLangLanguageContractV650 || contract == PipeLangLanguageContractV640 || contract == PipeLangLanguageContractV630 || contract == PipeLangLanguageContractV620 || contract == PipeLangLanguageContractV610 || contract == PipeLangLanguageContractV600 || contract == PipeLangLanguageContractV590 || contract == PipeLangLanguageContractV580) && isResolvedSourceArithmeticResult(contract, declared) && (len(method.Params) >= 3 || countPropagationExpressions(method.Body) != 1) {
			return cp.validateCheckedPropagationChain(method, declared, outer)
		}
		if contract == PipeLangLanguageContractV570 && isResolvedSourceArithmeticResult(contract, declared) && (len(method.Params) == 3 || countPropagationExpressions(method.Body) != 1) {
			return cp.validateTwoStageCheckedPropagation(method, declared, outer)
		}
		return cp.validateDirectParameterBlockPropagation(method, declared, outer)
	}
	if ((cp.modules.LanguageContract() == PipeLangLanguageContractV730 || cp.modules.LanguageContract() == PipeLangLanguageContractV720 || cp.modules.LanguageContract() == PipeLangLanguageContractV710 || cp.modules.LanguageContract() == PipeLangLanguageContractV700 || cp.modules.LanguageContract() == PipeLangLanguageContractV690 || cp.modules.LanguageContract() == PipeLangLanguageContractV680 || cp.modules.LanguageContract() == PipeLangLanguageContractV670 || cp.modules.LanguageContract() == PipeLangLanguageContractV660 || cp.modules.LanguageContract() == PipeLangLanguageContractV650 || cp.modules.LanguageContract() == PipeLangLanguageContractV640 || cp.modules.LanguageContract() == PipeLangLanguageContractV630 || cp.modules.LanguageContract() == PipeLangLanguageContractV620 || cp.modules.LanguageContract() == PipeLangLanguageContractV610 || cp.modules.LanguageContract() == PipeLangLanguageContractV600 || cp.modules.LanguageContract() == PipeLangLanguageContractV590 || cp.modules.LanguageContract() == PipeLangLanguageContractV580) || cp.modules.LanguageContract() == PipeLangLanguageContractV570 || cp.modules.LanguageContract() == PipeLangLanguageContractV560 || cp.modules.LanguageContract() == PipeLangLanguageContractV550) && isResolvedSourceArithmeticResult(cp.modules.LanguageContract(), declared) {
		if _, helper := outer.Initializer.(*CallExpr); !helper {
			return cp.validateDirectParameterBlockPropagation(method, declared, outer)
		}
	}
	if hasPriorLocalPropagationSourceContract(cp.modules.LanguageContract()) {
		return cp.validatePriorLocalBlockPropagation(method, declared, outer)
	}
	return oneDiagnostic(cp.sources, CodePropagation, CategorySemantic, method.Body.SourceSpan(), "v0.41.0 admits exactly one propagation as the first immutable-local initializer")
}

func (cp *checkedProgram) validateGeneralizedCrossPayloadResultPropagation(method MethodDecl, declared ResolvedTypeRef, first *ImmutableLocalExpr, sourceCarrier ResolvedTypeRef) error {
	return cp.validateCrossPayloadResultPropagationChain(method, declared, first, sourceCarrier, false)
}

func (cp *checkedProgram) validateContextualCrossPayloadResultPropagation(method MethodDecl, declared ResolvedTypeRef, first *ImmutableLocalExpr, sourceCarrier ResolvedTypeRef) error {
	return cp.validateCrossPayloadResultPropagationChain(method, declared, first, sourceCarrier, true)
}

func (cp *checkedProgram) validateCrossPayloadResultPropagationChain(method MethodDecl, declared ResolvedTypeRef, first *ImmutableLocalExpr, sourceCarrier ResolvedTypeRef, contextual bool) error {
	feature := "v0.61.0 generalized cross-payload Result propagation"
	expectedParameters := 1
	multiContext := contextual && (cp.modules.LanguageContract() == PipeLangLanguageContractV730 || cp.modules.LanguageContract() == PipeLangLanguageContractV720 || cp.modules.LanguageContract() == PipeLangLanguageContractV710 || cp.modules.LanguageContract() == PipeLangLanguageContractV700 || cp.modules.LanguageContract() == PipeLangLanguageContractV690 || cp.modules.LanguageContract() == PipeLangLanguageContractV680 || cp.modules.LanguageContract() == PipeLangLanguageContractV670)
	if contextual {
		feature = "v0.62.0 contextual cross-payload Result propagation"
		expectedParameters = 2
	}
	propagationCount := countPropagationExpressions(method.Body)
	samePayloadAdmitted := contextual && (cp.modules.LanguageContract() == PipeLangLanguageContractV730 || cp.modules.LanguageContract() == PipeLangLanguageContractV720 || cp.modules.LanguageContract() == PipeLangLanguageContractV710 || cp.modules.LanguageContract() == PipeLangLanguageContractV700 || cp.modules.LanguageContract() == PipeLangLanguageContractV690 || cp.modules.LanguageContract() == PipeLangLanguageContractV680 || cp.modules.LanguageContract() == PipeLangLanguageContractV670 || cp.modules.LanguageContract() == PipeLangLanguageContractV660 || (cp.modules.LanguageContract() == PipeLangLanguageContractV650 && propagationCount == 2))
	if multiContext {
		feature = "v0.67.0 generalized shared-context Result propagation"
		expectedParameters = len(method.Params)
	} else if contextual && cp.modules.LanguageContract() == PipeLangLanguageContractV660 {
		feature = "v0.66.0 generalized contextual bounded Result propagation"
	} else if samePayloadAdmitted {
		feature = "v0.65.0 exact two-stage contextual bounded Result propagation"
	}
	fail := func(span Span, detail string) error {
		return oneDiagnostic(cp.sources, CodePropagation, CategorySemantic, span, feature+" "+detail)
	}
	if normalizeVisibility(method.Visibility) != VisibilityPublic || len(method.Params) != expectedParameters || (multiContext && len(method.Params) < 2) || propagationCount < 2 {
		if contextual {
			if multiContext {
				return fail(method.Span, "requires one public pure method with one direct carrier, one or more direct string context parameters, and at least two propagations")
			}
			return fail(method.Span, "requires one public pure method with one direct carrier, one direct string context parameter, and at least two propagations")
		}
		return fail(method.Span, "requires one public pure method with exactly one direct carrier parameter and at least two propagations")
	}
	if contextual {
		for position := 1; position < len(method.Params); position++ {
			contextType, err := cp.resolveType(method.Params[position].Type, RelatedSpan{Span: method.Params[position].Span, Message: "contextual cross-payload string parameter"})
			if err != nil {
				return err
			}
			if !contextType.Equal(resolvedPrimitive(TypeString)) {
				return fail(method.Params[position].Type.Span, fmt.Sprintf("requires direct context parameter %d to be exactly string", position))
			}
		}
	}
	if !isResolvedBoundedValueResult(cp.modules.LanguageContract(), sourceCarrier) || !isResolvedBoundedValueResult(cp.modules.LanguageContract(), declared) {
		return fail(method.Body.SourceSpan(), "requires admitted source and target Results with string failure payloads")
	}
	firstPropagation, ok := first.Initializer.(*PropagateExpr)
	if !ok {
		return fail(first.Initializer.SourceSpan(), "requires propagation of the incoming carrier as the first local initializer")
	}
	sourceReference, ok := firstPropagation.Value.(*IdentExpr)
	if !ok || sourceReference.Name != method.Params[0].Name {
		return fail(firstPropagation.Value.SourceSpan(), "requires the sole direct carrier parameter")
	}
	payloadType, err := cp.resolveType(first.Type, RelatedSpan{Span: first.Type.Span, Message: "first propagated payload local"})
	if err != nil {
		return err
	}
	if !payloadType.Equal(sourceCarrier.Arguments[0]) {
		return fail(first.Type.Span, "requires the first local to exactly match the source success payload")
	}

	payloadLocal := first
	observedPropagations := 1
	stage := 1
	for {
		if terminal, terminalOK := payloadLocal.Return.(*CallExpr); terminalOK {
			if stage < 2 || observedPropagations != propagationCount {
				return fail(terminal.Span, "requires a contiguous chain of at least two helper stages")
			}
			if !samePayloadAdmitted && payloadType.Equal(declared.Arguments[0]) {
				return fail(terminal.Span, "requires adjacent distinct admitted payloads; equal payloads require v0.66.0 or the exact v0.65.0 two-stage contextual form")
			}
			if err := cp.validateGeneralizedCrossPayloadHelper(method, terminal, payloadLocal.Name, payloadType, declared, stage, contextual, feature); err != nil {
				return err
			}
			return nil
		}

		carrierLocal, ok := payloadLocal.Return.(*ImmutableLocalExpr)
		if !ok {
			return fail(payloadLocal.Return.SourceSpan(), fmt.Sprintf("stage %d requires an explicit helper Result carrier local", stage))
		}
		carrierType, err := cp.resolveType(carrierLocal.Type, RelatedSpan{Span: carrierLocal.Type.Span, Message: "intermediate Result carrier local"})
		if err != nil {
			return err
		}
		if !isResolvedBoundedValueResult(cp.modules.LanguageContract(), carrierType) || !carrierType.Arguments[1].Equal(sourceCarrier.Arguments[1]) || (!samePayloadAdmitted && carrierType.Arguments[0].Equal(payloadType)) {
			return fail(carrierLocal.Type.Span, fmt.Sprintf("stage %d requires an admitted payload with the shared string failure type; equal payloads require v0.66.0 or the exact v0.65.0 two-stage contextual form", stage))
		}
		call, ok := carrierLocal.Initializer.(*CallExpr)
		if !ok {
			return fail(carrierLocal.Initializer.SourceSpan(), fmt.Sprintf("stage %d carrier must be initialized by one helper call", stage))
		}
		if err := cp.validateGeneralizedCrossPayloadHelper(method, call, payloadLocal.Name, payloadType, carrierType, stage, contextual, feature); err != nil {
			return err
		}
		nextPayload, ok := carrierLocal.Return.(*ImmutableLocalExpr)
		if !ok {
			return fail(carrierLocal.Return.SourceSpan(), fmt.Sprintf("stage %d carrier must be propagated by the immediately following payload local", stage))
		}
		nextPayloadType, err := cp.resolveType(nextPayload.Type, RelatedSpan{Span: nextPayload.Type.Span, Message: "propagated intermediate payload local"})
		if err != nil {
			return err
		}
		nextPropagation, ok := nextPayload.Initializer.(*PropagateExpr)
		if !ok {
			return fail(nextPayload.Initializer.SourceSpan(), fmt.Sprintf("stage %d carrier must initialize the next payload through propagation", stage))
		}
		carrierReference, ok := nextPropagation.Value.(*IdentExpr)
		if !ok || carrierReference.Name != carrierLocal.Name || !nextPayloadType.Equal(carrierType.Arguments[0]) {
			return fail(nextPropagation.Value.SourceSpan(), fmt.Sprintf("propagation after stage %d must directly consume its explicit carrier local", stage))
		}
		payloadLocal = nextPayload
		payloadType = nextPayloadType
		observedPropagations++
		stage++
	}
}

func (cp *checkedProgram) validateGeneralizedCrossPayloadHelper(method MethodDecl, call *CallExpr, expectedArgument string, parameterType, returnType ResolvedTypeRef, stage int, contextual bool, feature string) error {
	fail := func(span Span, detail string) error {
		return oneDiagnostic(cp.sources, CodePropagation, CategorySemantic, span, fmt.Sprintf("%s stage %d helper %s", feature, stage, detail))
	}
	expectedArguments := 1
	if contextual {
		expectedArguments = len(method.Params)
	}
	if len(call.Arguments) != expectedArguments {
		if contextual {
			return fail(call.Span, "requires the direct preceding payload local followed by every direct string context parameter in declaration order")
		}
		return fail(call.Span, "requires the direct preceding payload local as its sole argument")
	}
	argument, ok := call.Arguments[0].(*IdentExpr)
	if !ok || argument.Name != expectedArgument {
		return fail(call.Arguments[0].SourceSpan(), "requires the direct preceding payload local as the first argument")
	}
	if contextual {
		for position := 1; position < len(method.Params); position++ {
			context, ok := call.Arguments[position].(*IdentExpr)
			if !ok || context.Name != method.Params[position].Name {
				return fail(call.Arguments[position].SourceSpan(), "requires every direct string context parameter in declaration order after the payload")
			}
		}
	}
	callerOwner, _ := methodBySpan(cp.program, method.Span)
	helperOwner, helper := methodBySpan(cp.program, call.TargetSpan)
	if callerOwner == nil || helperOwner == nil || helper == nil || callerOwner.Span != helperOwner.Span || normalizeVisibility(helper.Visibility) != VisibilityPublic || len(helper.Params) != expectedArguments {
		return fail(call.NameSpan, "must resolve to one public pure same-class method")
	}
	helperParameter, err := cp.resolveType(helper.Params[0].Type, RelatedSpan{Span: helper.Params[0].Span, Message: "cross-payload helper parameter"})
	if err != nil {
		return err
	}
	if contextual {
		for position := 1; position < len(helper.Params); position++ {
			helperContext, err := cp.resolveType(helper.Params[position].Type, RelatedSpan{Span: helper.Params[position].Span, Message: "cross-payload helper context parameter"})
			if err != nil {
				return err
			}
			callerContext, err := cp.resolveType(method.Params[position].Type)
			if err != nil {
				return err
			}
			if !helperContext.Equal(resolvedPrimitive(TypeString)) || !helperContext.Equal(callerContext) {
				return fail(helper.Params[position].Type.Span, "requires every helper context parameter to exactly match the caller's direct string context parameters")
			}
		}
	}
	helperResult, err := cp.resolveType(helper.ReturnType, RelatedSpan{Span: helper.ReturnType.Span, Message: "cross-payload helper result"})
	if err != nil {
		return err
	}
	if !helperParameter.Equal(parameterType) || !helperResult.Equal(returnType) {
		if contextual {
			return fail(call.Span, "requires the exact payload-and-ordered-string-contexts-to-Result signature")
		}
		return fail(call.Span, "requires the exact payload-to-Result signature")
	}
	return nil
}

func (cp *checkedProgram) validateTwoStageCrossPayloadResultPropagation(method MethodDecl, declared ResolvedTypeRef, first *ImmutableLocalExpr, sourceCarrier ResolvedTypeRef) error {
	fail := func(span Span, detail string) error {
		return oneDiagnostic(cp.sources, CodePropagation, CategorySemantic, span, "v0.60.0 two-stage cross-payload Result propagation "+detail)
	}
	if normalizeVisibility(method.Visibility) != VisibilityPublic || len(method.Params) != 1 || countPropagationExpressions(method.Body) != 2 {
		return fail(method.Span, "requires one public pure method with exactly one direct carrier parameter and two propagations")
	}
	if !isResolvedBoundedValueResult(PipeLangLanguageContractV600, sourceCarrier) || !isResolvedBoundedValueResult(PipeLangLanguageContractV600, declared) {
		return fail(method.Body.SourceSpan(), "requires admitted source and target Results with string failure payloads")
	}
	firstPropagation, ok := first.Initializer.(*PropagateExpr)
	if !ok {
		return fail(first.Initializer.SourceSpan(), "requires propagation of the incoming carrier as the first local initializer")
	}
	sourceReference, ok := firstPropagation.Value.(*IdentExpr)
	if !ok || sourceReference.Name != method.Params[0].Name {
		return fail(firstPropagation.Value.SourceSpan(), "requires the sole direct carrier parameter")
	}
	firstType, err := cp.resolveType(first.Type, RelatedSpan{Span: first.Type.Span, Message: "first propagated payload local"})
	if err != nil {
		return err
	}
	if !firstType.Equal(sourceCarrier.Arguments[0]) {
		return fail(first.Type.Span, "requires the first local to exactly match the source success payload")
	}

	intermediate, ok := first.Return.(*ImmutableLocalExpr)
	if !ok {
		return fail(first.Return.SourceSpan(), "requires an explicit intermediate helper Result local")
	}
	intermediateType, err := cp.resolveType(intermediate.Type, RelatedSpan{Span: intermediate.Type.Span, Message: "intermediate Result carrier local"})
	if err != nil {
		return err
	}
	if !isResolvedBoundedValueResult(PipeLangLanguageContractV600, intermediateType) || sourceCarrier.Arguments[0].Equal(intermediateType.Arguments[0]) || intermediateType.Arguments[0].Equal(declared.Arguments[0]) {
		return fail(intermediate.Type.Span, "requires adjacent distinct admitted payloads with one shared string failure type")
	}
	firstCall, ok := intermediate.Initializer.(*CallExpr)
	if !ok || len(firstCall.Arguments) != 1 {
		return fail(intermediate.Initializer.SourceSpan(), "requires the intermediate carrier to be initialized by the first helper")
	}
	firstArgument, ok := firstCall.Arguments[0].(*IdentExpr)
	if !ok || firstArgument.Name != first.Name {
		return fail(firstCall.Arguments[0].SourceSpan(), "requires the first propagated local as the sole first-helper argument")
	}
	if err := cp.validateTwoStageCrossPayloadHelper(method, firstCall, firstType, intermediateType, "first"); err != nil {
		return err
	}

	second, ok := intermediate.Return.(*ImmutableLocalExpr)
	if !ok {
		return fail(intermediate.Return.SourceSpan(), "requires the intermediate carrier to be propagated by the next local")
	}
	secondType, err := cp.resolveType(second.Type, RelatedSpan{Span: second.Type.Span, Message: "second propagated payload local"})
	if err != nil {
		return err
	}
	secondPropagation, ok := second.Initializer.(*PropagateExpr)
	if !ok {
		return fail(second.Initializer.SourceSpan(), "requires propagation of the intermediate carrier as the third local initializer")
	}
	intermediateReference, ok := secondPropagation.Value.(*IdentExpr)
	if !ok || intermediateReference.Name != intermediate.Name || !secondType.Equal(intermediateType.Arguments[0]) {
		return fail(secondPropagation.Value.SourceSpan(), "requires the direct intermediate carrier and its exact success payload")
	}
	secondCall, ok := second.Return.(*CallExpr)
	if !ok || len(secondCall.Arguments) != 1 {
		return fail(second.Return.SourceSpan(), "requires one terminal second-helper call")
	}
	secondArgument, ok := secondCall.Arguments[0].(*IdentExpr)
	if !ok || secondArgument.Name != second.Name {
		return fail(secondCall.Arguments[0].SourceSpan(), "requires the second propagated local as the sole second-helper argument")
	}
	return cp.validateTwoStageCrossPayloadHelper(method, secondCall, secondType, declared, "second")
}

func (cp *checkedProgram) validateTwoStageCrossPayloadHelper(method MethodDecl, call *CallExpr, parameterType, returnType ResolvedTypeRef, stage string) error {
	fail := func(span Span, detail string) error {
		return oneDiagnostic(cp.sources, CodePropagation, CategorySemantic, span, "v0.60.0 two-stage cross-payload Result propagation "+stage+" helper "+detail)
	}
	callerOwner, _ := methodBySpan(cp.program, method.Span)
	helperOwner, helper := methodBySpan(cp.program, call.TargetSpan)
	if callerOwner == nil || helperOwner == nil || helper == nil || callerOwner.Span != helperOwner.Span || normalizeVisibility(helper.Visibility) != VisibilityPublic || len(helper.Params) != 1 {
		return fail(call.NameSpan, "must resolve to one public pure same-class method")
	}
	helperParameter, err := cp.resolveType(helper.Params[0].Type, RelatedSpan{Span: helper.Params[0].Span, Message: stage + " cross-payload helper parameter"})
	if err != nil {
		return err
	}
	helperResult, err := cp.resolveType(helper.ReturnType, RelatedSpan{Span: helper.ReturnType.Span, Message: stage + " cross-payload helper result"})
	if err != nil {
		return err
	}
	if !helperParameter.Equal(parameterType) || !helperResult.Equal(returnType) {
		return fail(call.Span, "requires the exact payload-to-Result signature")
	}
	return nil
}

func (cp *checkedProgram) validateCrossPayloadResultPropagation(method MethodDecl, declared ResolvedTypeRef, outer *ImmutableLocalExpr, carrier ResolvedTypeRef) error {
	return cp.validateCrossPayloadResultPropagationForm(method, declared, outer, carrier, false)
}

func (cp *checkedProgram) validateSingleStageContextualCrossPayloadResultPropagation(method MethodDecl, declared ResolvedTypeRef, outer *ImmutableLocalExpr, carrier ResolvedTypeRef) error {
	return cp.validateCrossPayloadResultPropagationForm(method, declared, outer, carrier, true)
}

func (cp *checkedProgram) validateCrossPayloadResultPropagationForm(method MethodDecl, declared ResolvedTypeRef, outer *ImmutableLocalExpr, carrier ResolvedTypeRef, contextual bool) error {
	feature := "v0.59.0 cross-payload Result propagation"
	expectedParameters := 1
	multiContext := contextual && (cp.modules.LanguageContract() == PipeLangLanguageContractV730 || cp.modules.LanguageContract() == PipeLangLanguageContractV720 || cp.modules.LanguageContract() == PipeLangLanguageContractV710 || cp.modules.LanguageContract() == PipeLangLanguageContractV700 || cp.modules.LanguageContract() == PipeLangLanguageContractV690 || cp.modules.LanguageContract() == PipeLangLanguageContractV680)
	if contextual {
		feature = "v0.63.0 one-stage contextual cross-payload Result propagation"
		if multiContext {
			feature = "v0.68.0 generalized one-stage shared-context Result propagation"
			expectedParameters = len(method.Params)
		} else if cp.modules.LanguageContract() == PipeLangLanguageContractV670 || cp.modules.LanguageContract() == PipeLangLanguageContractV660 || cp.modules.LanguageContract() == PipeLangLanguageContractV650 || cp.modules.LanguageContract() == PipeLangLanguageContractV640 {
			feature = "v0.64.0 one-stage contextual bounded Result propagation"
		}
		if !multiContext {
			expectedParameters = 2
		}
	}
	fail := func(span Span, detail string) error {
		return oneDiagnostic(cp.sources, CodePropagation, CategorySemantic, span, feature+" "+detail)
	}
	if normalizeVisibility(method.Visibility) != VisibilityPublic || len(method.Params) != expectedParameters || (multiContext && len(method.Params) < 2) {
		if contextual {
			if multiContext {
				return fail(method.Span, "requires one public pure method with one direct carrier and one or more direct string context parameters")
			}
			return fail(method.Span, "requires one public pure method with one direct carrier and one direct string context parameter")
		}
		return fail(method.Span, "requires one public pure method with exactly one direct carrier parameter")
	}
	if contextual {
		for position := 1; position < len(method.Params); position++ {
			contextType, err := cp.resolveType(method.Params[position].Type, RelatedSpan{Span: method.Params[position].Span, Message: "one-stage contextual string parameter"})
			if err != nil {
				return err
			}
			if !contextType.Equal(resolvedPrimitive(TypeString)) {
				return fail(method.Params[position].Type.Span, fmt.Sprintf("requires direct context parameter %d to be exactly string", position))
			}
		}
	}
	samePayloadAdmitted := contextual && (cp.modules.LanguageContract() == PipeLangLanguageContractV730 || cp.modules.LanguageContract() == PipeLangLanguageContractV720 || cp.modules.LanguageContract() == PipeLangLanguageContractV710 || cp.modules.LanguageContract() == PipeLangLanguageContractV700 || cp.modules.LanguageContract() == PipeLangLanguageContractV690 || cp.modules.LanguageContract() == PipeLangLanguageContractV680 || cp.modules.LanguageContract() == PipeLangLanguageContractV670 || cp.modules.LanguageContract() == PipeLangLanguageContractV660 || cp.modules.LanguageContract() == PipeLangLanguageContractV650 || cp.modules.LanguageContract() == PipeLangLanguageContractV640)
	if countPropagationExpressions(method.Body) != 1 || !isResolvedBoundedValueResult(cp.modules.LanguageContract(), carrier) || !isResolvedBoundedValueResult(cp.modules.LanguageContract(), declared) || (!samePayloadAdmitted && carrier.Arguments[0].Equal(declared.Arguments[0])) {
		return fail(method.Body.SourceSpan(), "requires admitted source and target payloads with the same string failure type; equal payloads require v0.64.0 contextual propagation")
	}
	propagated, ok := outer.Initializer.(*PropagateExpr)
	if !ok {
		return fail(outer.Initializer.SourceSpan(), "requires propagation as the first immutable-local initializer")
	}
	reference, ok := propagated.Value.(*IdentExpr)
	if !ok || reference.Name != method.Params[0].Name {
		return fail(propagated.Value.SourceSpan(), "requires the sole direct carrier parameter")
	}
	localType, err := cp.resolveType(outer.Type, RelatedSpan{Span: outer.Type.Span, Message: "propagated source payload local"})
	if err != nil {
		return err
	}
	if !localType.Equal(carrier.Arguments[0]) {
		return fail(outer.Type.Span, "requires the first immutable local to exactly match the source success payload")
	}
	call, ok := outer.Return.(*CallExpr)
	if !ok || len(call.Arguments) != expectedParameters {
		if contextual {
			if multiContext {
				return fail(outer.Return.SourceSpan(), "requires one terminal same-class helper call with the propagated local followed by every direct string context parameter")
			}
			return fail(outer.Return.SourceSpan(), "requires one terminal same-class helper call with the propagated local followed by the direct string context")
		}
		return fail(outer.Return.SourceSpan(), "requires one terminal same-class helper call with the propagated local as its sole argument")
	}
	argument, ok := call.Arguments[0].(*IdentExpr)
	if !ok || argument.Name != outer.Name {
		return fail(call.Arguments[0].SourceSpan(), "requires the direct propagated local as the helper argument")
	}
	if contextual {
		for position := 1; position < len(method.Params); position++ {
			context, ok := call.Arguments[position].(*IdentExpr)
			if !ok || context.Name != method.Params[position].Name {
				return fail(call.Arguments[position].SourceSpan(), "requires every direct string context parameter exactly once in declaration order after the payload")
			}
		}
	}
	callerOwner, _ := methodBySpan(cp.program, method.Span)
	helperOwner, helper := methodBySpan(cp.program, call.TargetSpan)
	if callerOwner == nil || helperOwner == nil || helper == nil || callerOwner.Span != helperOwner.Span || normalizeVisibility(helper.Visibility) != VisibilityPublic || len(helper.Params) != expectedParameters {
		return fail(call.NameSpan, "requires one resolved public same-class helper")
	}
	helperParameter, err := cp.resolveType(helper.Params[0].Type, RelatedSpan{Span: helper.Params[0].Span, Message: "cross-payload helper parameter"})
	if err != nil {
		return err
	}
	if contextual {
		for position := 1; position < len(helper.Params); position++ {
			helperContext, err := cp.resolveType(helper.Params[position].Type, RelatedSpan{Span: helper.Params[position].Span, Message: "one-stage contextual helper context parameter"})
			if err != nil {
				return err
			}
			callerContext, err := cp.resolveType(method.Params[position].Type)
			if err != nil {
				return err
			}
			if !helperContext.Equal(resolvedPrimitive(TypeString)) || !helperContext.Equal(callerContext) {
				return fail(helper.Params[position].Type.Span, "requires every helper context parameter to exactly match the caller's direct string context parameters")
			}
		}
	}
	helperResult, err := cp.resolveType(helper.ReturnType, RelatedSpan{Span: helper.ReturnType.Span, Message: "cross-payload helper result"})
	if err != nil {
		return err
	}
	if !helperParameter.Equal(carrier.Arguments[0]) || !helperResult.Equal(declared) {
		if contextual {
			return fail(call.Span, "requires the exact payload-and-ordered-string-contexts-to-Result signature")
		}
		return fail(call.Span, "requires an exact source-payload to target-Result helper signature")
	}
	return nil
}

func (cp *checkedProgram) validateCheckedPropagationChain(method MethodDecl, declared ResolvedTypeRef, first *ImmutableLocalExpr) error {
	stageCount := len(method.Params) - 1
	if stageCount < 2 {
		return oneDiagnostic(cp.sources, CodePropagation, CategorySemantic, method.Span, "v0.58.0 checked propagation chain requires a carrier and at least two payload operands")
	}
	if countPropagationExpressions(method.Body) != stageCount {
		return oneDiagnostic(cp.sources, CodePropagation, CategorySemantic, method.Body.SourceSpan(), "v0.58.0 checked propagation chain requires one propagation for the incoming carrier and every non-terminal checked stage")
	}
	firstPropagation, ok := first.Initializer.(*PropagateExpr)
	if !ok {
		return oneDiagnostic(cp.sources, CodePropagation, CategorySemantic, first.Initializer.SourceSpan(), "v0.58.0 checked propagation chain requires the incoming carrier propagation first")
	}
	carrierReference, ok := firstPropagation.Value.(*IdentExpr)
	if !ok || carrierReference.Name != method.Params[0].Name {
		return oneDiagnostic(cp.sources, CodePropagation, CategorySemantic, firstPropagation.Value.SourceSpan(), "v0.58.0 checked propagation chain requires the first direct carrier parameter")
	}
	carrier, err := cp.resolveType(method.Params[0].Type, RelatedSpan{Span: method.Params[0].Span, Message: "incoming checked carrier"})
	if err != nil {
		return err
	}
	if !carrier.Equal(declared) || !isResolvedSourceArithmeticResult(PipeLangLanguageContractV580, carrier) {
		return oneDiagnostic(cp.sources, CodePropagation, CategorySemantic, method.Params[0].Type.Span, "v0.58.0 incoming checked carrier and method return type must be the same arithmetic Result")
	}
	payload := carrier.Arguments[0]
	firstLocalType, err := cp.resolveType(first.Type, RelatedSpan{Span: first.Span, Message: "first propagated payload local"})
	if err != nil {
		return err
	}
	if !firstLocalType.Equal(payload) {
		return oneDiagnostic(cp.sources, CodePropagation, CategorySemantic, first.Type.Span, "v0.58.0 first propagated local must match the arithmetic payload")
	}
	for position := 1; position < len(method.Params); position++ {
		operand, resolveErr := cp.resolveType(method.Params[position].Type, RelatedSpan{Span: method.Params[position].Span, Message: "checked arithmetic operand"})
		if resolveErr != nil {
			return resolveErr
		}
		if !operand.Equal(payload) {
			return oneDiagnostic(cp.sources, CodePropagation, CategorySemantic, method.Params[position].Type.Span, "v0.58.0 checked propagation chain operand types must match the propagated payload")
		}
	}

	payloadLocal := first
	for stage := 1; stage < stageCount; stage++ {
		checkedCarrier, ok := payloadLocal.Return.(*ImmutableLocalExpr)
		if !ok {
			return oneDiagnostic(cp.sources, CodePropagation, CategorySemantic, payloadLocal.Return.SourceSpan(), "v0.58.0 each non-terminal checked stage requires an explicit arithmetic Result carrier local")
		}
		checkedCarrierType, resolveErr := cp.resolveType(checkedCarrier.Type, RelatedSpan{Span: checkedCarrier.Span, Message: "intermediate checked carrier local"})
		if resolveErr != nil {
			return resolveErr
		}
		if !checkedCarrierType.Equal(carrier) || !isAdmittedMultiParameterCheckedArithmeticSourceContinuation(checkedCarrier.Initializer, payload, payloadLocal.Name, method.Params[stage].Name) {
			return oneDiagnostic(cp.sources, CodePropagation, CategorySemantic, checkedCarrier.Initializer.SourceSpan(), fmt.Sprintf("v0.58.0 checked stage %d requires the preceding propagated local as its left operand and payload parameter %d as its right operand", stage, stage))
		}
		nextPayload, ok := checkedCarrier.Return.(*ImmutableLocalExpr)
		if !ok {
			return oneDiagnostic(cp.sources, CodePropagation, CategorySemantic, checkedCarrier.Return.SourceSpan(), fmt.Sprintf("v0.58.0 checked stage %d carrier must be propagated by the immediately following payload local", stage))
		}
		nextPayloadType, resolveErr := cp.resolveType(nextPayload.Type, RelatedSpan{Span: nextPayload.Span, Message: "propagated intermediate payload local"})
		if resolveErr != nil {
			return resolveErr
		}
		nextPropagation, ok := nextPayload.Initializer.(*PropagateExpr)
		if !ok {
			return oneDiagnostic(cp.sources, CodePropagation, CategorySemantic, nextPayload.Initializer.SourceSpan(), fmt.Sprintf("v0.58.0 checked stage %d carrier must initialize the next payload through propagation", stage))
		}
		nextCarrierReference, ok := nextPropagation.Value.(*IdentExpr)
		if !ok || nextCarrierReference.Name != checkedCarrier.Name || !nextPayloadType.Equal(payload) {
			return oneDiagnostic(cp.sources, CodePropagation, CategorySemantic, nextPropagation.Value.SourceSpan(), fmt.Sprintf("v0.58.0 propagation after checked stage %d must directly consume its explicit carrier local", stage))
		}
		payloadLocal = nextPayload
	}
	if !isAdmittedMultiParameterCheckedArithmeticSourceContinuation(payloadLocal.Return, payload, payloadLocal.Name, method.Params[stageCount].Name) {
		return oneDiagnostic(cp.sources, CodePropagation, CategorySemantic, payloadLocal.Return.SourceSpan(), "v0.58.0 terminal checked stage requires the final propagated local as its left operand and final payload parameter as its right operand")
	}
	return nil
}

func (cp *checkedProgram) validateDirectParameterBlockPropagation(method MethodDecl, declared ResolvedTypeRef, outer *ImmutableLocalExpr) error {
	propagated, first := outer.Initializer.(*PropagateExpr)
	if !first || countPropagationExpressions(method.Body) != 1 {
		return oneDiagnostic(cp.sources, CodePropagation, CategorySemantic, method.Body.SourceSpan(), "v0.41.0 admits exactly one propagation as the first immutable-local initializer")
	}
	identifier, direct := propagated.Value.(*IdentExpr)
	if !direct || len(method.Params) == 0 || identifier.Name != method.Params[0].Name {
		return oneDiagnostic(cp.sources, CodePropagation, CategorySemantic, propagated.Value.SourceSpan(), "v0.41.0 block propagation requires the method's sole direct carrier parameter")
	}
	carrier, err := cp.resolveType(method.Params[0].Type, RelatedSpan{Span: method.Params[0].Span, Message: "propagated carrier parameter"})
	if err != nil {
		return err
	}
	if !carrier.Equal(declared) {
		return oneDiagnostic(cp.sources, CodePropagation, CategorySemantic, propagated.Span, "propagated carrier parameter and method return type must be identical", RelatedSpan{Span: method.ReturnType.Span, Message: "method return carrier"})
	}
	contract := inheritedLanguageContract(cp.modules.LanguageContract())
	allowArithmetic := ((contract == PipeLangLanguageContractV730 || contract == PipeLangLanguageContractV720 || contract == PipeLangLanguageContractV710 || contract == PipeLangLanguageContractV700 || contract == PipeLangLanguageContractV690 || contract == PipeLangLanguageContractV680 || contract == PipeLangLanguageContractV670 || contract == PipeLangLanguageContractV660 || contract == PipeLangLanguageContractV650 || contract == PipeLangLanguageContractV640 || contract == PipeLangLanguageContractV630 || contract == PipeLangLanguageContractV620 || contract == PipeLangLanguageContractV610 || contract == PipeLangLanguageContractV600 || contract == PipeLangLanguageContractV590 || contract == PipeLangLanguageContractV580) || contract == PipeLangLanguageContractV570 || contract == PipeLangLanguageContractV560 || contract == PipeLangLanguageContractV550) && isResolvedSourceArithmeticResult(contract, carrier)
	multiParameterArithmetic := ((contract == PipeLangLanguageContractV730 || contract == PipeLangLanguageContractV720 || contract == PipeLangLanguageContractV710 || contract == PipeLangLanguageContractV700 || contract == PipeLangLanguageContractV690 || contract == PipeLangLanguageContractV680 || contract == PipeLangLanguageContractV670 || contract == PipeLangLanguageContractV660 || contract == PipeLangLanguageContractV650 || contract == PipeLangLanguageContractV640 || contract == PipeLangLanguageContractV630 || contract == PipeLangLanguageContractV620 || contract == PipeLangLanguageContractV610 || contract == PipeLangLanguageContractV600 || contract == PipeLangLanguageContractV590 || contract == PipeLangLanguageContractV580) || contract == PipeLangLanguageContractV570 || contract == PipeLangLanguageContractV560) && allowArithmetic && len(method.Params) == 2
	if len(method.Params) != 1 && !multiParameterArithmetic {
		if ((contract == PipeLangLanguageContractV730 || contract == PipeLangLanguageContractV720 || contract == PipeLangLanguageContractV710 || contract == PipeLangLanguageContractV700 || contract == PipeLangLanguageContractV690 || contract == PipeLangLanguageContractV680 || contract == PipeLangLanguageContractV670 || contract == PipeLangLanguageContractV660 || contract == PipeLangLanguageContractV650 || contract == PipeLangLanguageContractV640 || contract == PipeLangLanguageContractV630 || contract == PipeLangLanguageContractV620 || contract == PipeLangLanguageContractV610 || contract == PipeLangLanguageContractV600 || contract == PipeLangLanguageContractV590 || contract == PipeLangLanguageContractV580) || contract == PipeLangLanguageContractV570 || contract == PipeLangLanguageContractV560) && allowArithmetic {
			return oneDiagnostic(cp.sources, CodePropagation, CategorySemantic, propagated.Value.SourceSpan(), "v0.56.0 direct checked-arithmetic propagation requires either the inherited sole carrier parameter or exactly carrier and payload operand parameters")
		}
		return oneDiagnostic(cp.sources, CodePropagation, CategorySemantic, propagated.Value.SourceSpan(), "v0.41.0 block propagation requires the method's sole direct carrier parameter")
	}
	if !cp.isResolvedOptionalValue(contract, carrier) && !isResolvedBoundedValueResult(contract, carrier) && !allowArithmetic {
		if (contract == PipeLangLanguageContractV730 || contract == PipeLangLanguageContractV720 || contract == PipeLangLanguageContractV710 || contract == PipeLangLanguageContractV700 || contract == PipeLangLanguageContractV690 || contract == PipeLangLanguageContractV680 || contract == PipeLangLanguageContractV670 || contract == PipeLangLanguageContractV660 || contract == PipeLangLanguageContractV650 || contract == PipeLangLanguageContractV640 || contract == PipeLangLanguageContractV630 || contract == PipeLangLanguageContractV620 || contract == PipeLangLanguageContractV610 || contract == PipeLangLanguageContractV600 || contract == PipeLangLanguageContractV590 || contract == PipeLangLanguageContractV580) || contract == PipeLangLanguageContractV570 || contract == PipeLangLanguageContractV560 || contract == PipeLangLanguageContractV550 {
			return oneDiagnostic(cp.sources, CodePropagation, CategorySemantic, propagated.Value.SourceSpan(), fmt.Sprintf("v0.55.0 block propagation requires an admitted Optional, bounded Result, or checked-arithmetic Result, got %s", carrier))
		}
		return oneDiagnostic(cp.sources, CodePropagation, CategorySemantic, propagated.Value.SourceSpan(), fmt.Sprintf("v0.41.0 block propagation requires an admitted Optional or bounded Result, got %s", carrier))
	}
	localType, err := cp.resolveType(outer.Type, RelatedSpan{Span: outer.Span, Message: "propagation payload local"})
	if err != nil {
		return err
	}
	if len(carrier.Arguments) == 0 || !localType.Equal(carrier.Arguments[0]) {
		return oneDiagnostic(cp.sources, CodePropagation, CategorySemantic, outer.Type.Span, "first immutable local type must exactly match the propagated success payload")
	}
	if multiParameterArithmetic {
		operand, err := cp.resolveType(method.Params[1].Type, RelatedSpan{Span: method.Params[1].Span, Message: "checked arithmetic operand parameter"})
		if err != nil {
			return err
		}
		if !operand.Equal(localType) {
			return oneDiagnostic(cp.sources, CodePropagation, CategorySemantic, method.Params[1].Type.Span, "v0.56.0 direct checked-arithmetic operand type must exactly match the propagated payload")
		}
		if !isAdmittedMultiParameterCheckedArithmeticSourceContinuation(outer.Return, localType, outer.Name, method.Params[1].Name) {
			return oneDiagnostic(cp.sources, CodePropagation, CategorySemantic, outer.Return.SourceSpan(), "v0.56.0 multi-parameter direct checked-arithmetic propagation requires the propagated local as the left operand and the second direct parameter as the right operand")
		}
	} else if allowArithmetic && !isAdmittedCheckedArithmeticSourceContinuation(outer.Return, localType) {
		return oneDiagnostic(cp.sources, CodePropagation, CategorySemantic, outer.Return.SourceSpan(), "v0.55.0 direct checked-arithmetic propagation requires one admitted checked arithmetic continuation")
	}
	return nil
}

func (cp *checkedProgram) validateTwoStageCheckedPropagation(method MethodDecl, declared ResolvedTypeRef, first *ImmutableLocalExpr) error {
	if countPropagationExpressions(method.Body) != 2 {
		return oneDiagnostic(cp.sources, CodePropagation, CategorySemantic, method.Body.SourceSpan(), "v0.57.0 two-stage checked propagation requires exactly two propagations")
	}
	if len(method.Params) != 3 {
		return oneDiagnostic(cp.sources, CodePropagation, CategorySemantic, method.Span, "v0.57.0 two-stage checked propagation requires exactly carrier and two payload operands")
	}
	firstPropagation, ok := first.Initializer.(*PropagateExpr)
	if !ok {
		return oneDiagnostic(cp.sources, CodePropagation, CategorySemantic, first.Initializer.SourceSpan(), "v0.57.0 two-stage checked propagation requires the incoming carrier propagation first")
	}
	carrierReference, ok := firstPropagation.Value.(*IdentExpr)
	if !ok || carrierReference.Name != method.Params[0].Name {
		return oneDiagnostic(cp.sources, CodePropagation, CategorySemantic, firstPropagation.Value.SourceSpan(), "v0.57.0 two-stage checked propagation requires the first direct carrier parameter")
	}
	carrier, err := cp.resolveType(method.Params[0].Type, RelatedSpan{Span: method.Params[0].Span, Message: "incoming checked carrier"})
	if err != nil {
		return err
	}
	if !carrier.Equal(declared) || !isResolvedSourceArithmeticResult(PipeLangLanguageContractV570, carrier) {
		return oneDiagnostic(cp.sources, CodePropagation, CategorySemantic, method.Params[0].Type.Span, "v0.57.0 incoming checked carrier and method return type must be the same arithmetic Result")
	}
	payload := carrier.Arguments[0]
	firstLocalType, err := cp.resolveType(first.Type, RelatedSpan{Span: first.Span, Message: "first propagated payload local"})
	if err != nil {
		return err
	}
	if !firstLocalType.Equal(payload) {
		return oneDiagnostic(cp.sources, CodePropagation, CategorySemantic, first.Type.Span, "v0.57.0 first propagated local must match the arithmetic payload")
	}
	for position := 1; position <= 2; position++ {
		operand, resolveErr := cp.resolveType(method.Params[position].Type, RelatedSpan{Span: method.Params[position].Span, Message: "checked arithmetic operand"})
		if resolveErr != nil {
			return resolveErr
		}
		if !operand.Equal(payload) {
			return oneDiagnostic(cp.sources, CodePropagation, CategorySemantic, method.Params[position].Type.Span, "v0.57.0 two-stage checked propagation operand types must match the propagated payload")
		}
	}
	checkedCarrier, ok := first.Return.(*ImmutableLocalExpr)
	if !ok {
		return oneDiagnostic(cp.sources, CodePropagation, CategorySemantic, first.Return.SourceSpan(), "v0.57.0 two-stage checked propagation requires an explicit checked carrier local")
	}
	checkedCarrierType, err := cp.resolveType(checkedCarrier.Type, RelatedSpan{Span: checkedCarrier.Span, Message: "intermediate checked carrier local"})
	if err != nil {
		return err
	}
	if !checkedCarrierType.Equal(carrier) || !isAdmittedMultiParameterCheckedArithmeticSourceContinuation(checkedCarrier.Initializer, payload, first.Name, method.Params[1].Name) {
		return oneDiagnostic(cp.sources, CodePropagation, CategorySemantic, checkedCarrier.Initializer.SourceSpan(), "v0.57.0 first checked stage requires the first propagated local as the left operand and the first payload parameter as the right operand")
	}
	second, ok := checkedCarrier.Return.(*ImmutableLocalExpr)
	if !ok {
		return oneDiagnostic(cp.sources, CodePropagation, CategorySemantic, checkedCarrier.Return.SourceSpan(), "v0.57.0 two-stage checked propagation requires the intermediate carrier to be propagated by the next local")
	}
	secondType, err := cp.resolveType(second.Type, RelatedSpan{Span: second.Span, Message: "second propagated payload local"})
	if err != nil {
		return err
	}
	secondPropagation, ok := second.Initializer.(*PropagateExpr)
	if !ok {
		return oneDiagnostic(cp.sources, CodePropagation, CategorySemantic, second.Initializer.SourceSpan(), "v0.57.0 second propagation must initialize the next local")
	}
	secondCarrierReference, ok := secondPropagation.Value.(*IdentExpr)
	if !ok || secondCarrierReference.Name != checkedCarrier.Name || !secondType.Equal(payload) {
		return oneDiagnostic(cp.sources, CodePropagation, CategorySemantic, secondPropagation.Value.SourceSpan(), "v0.57.0 second propagation must directly consume the explicit checked carrier local")
	}
	if !isAdmittedMultiParameterCheckedArithmeticSourceContinuation(second.Return, payload, second.Name, method.Params[2].Name) {
		return oneDiagnostic(cp.sources, CodePropagation, CategorySemantic, second.Return.SourceSpan(), "v0.57.0 terminal checked stage requires the second propagated local as the left operand and the second payload parameter as the right operand")
	}
	return nil
}

func isAdmittedMultiParameterCheckedArithmeticSourceContinuation(expression Expr, payload ResolvedTypeRef, localName, operandName string) bool {
	binary, ok := expression.(*BinaryExpr)
	if !ok {
		return false
	}
	left, leftOK := binary.Left.(*IdentExpr)
	right, rightOK := binary.Right.(*IdentExpr)
	if !leftOK || !rightOK || left.Name != localName || right.Name != operandName {
		return false
	}
	if payload.Equal(resolvedPrimitive(TypeFloat)) {
		return binary.Op == "/"
	}
	return payload.Equal(resolvedPrimitive(TypeInt)) && (binary.Op == "+" || binary.Op == "-" || binary.Op == "*")
}

func isAdmittedCheckedArithmeticSourceContinuation(expression Expr, payload ResolvedTypeRef) bool {
	if payload.Equal(resolvedPrimitive(TypeFloat)) {
		binary, ok := expression.(*BinaryExpr)
		return ok && binary.Op == "/"
	}
	if !payload.Equal(resolvedPrimitive(TypeInt)) {
		return false
	}
	if unary, ok := expression.(*UnaryExpr); ok {
		return unary.Op == "-"
	}
	binary, ok := expression.(*BinaryExpr)
	return ok && (binary.Op == "+" || binary.Op == "-" || binary.Op == "*")
}

func (cp *checkedProgram) validatePriorLocalBlockPropagation(method MethodDecl, declared ResolvedTypeRef, outer *ImmutableLocalExpr) error {
	called, firstCall := outer.Initializer.(*CallExpr)
	second, secondLocal := outer.Return.(*ImmutableLocalExpr)
	if !firstCall || !secondLocal || countPropagationExpressions(method.Body) != 1 {
		return oneDiagnostic(cp.sources, CodePropagation, CategorySemantic, method.Body.SourceSpan(), "v0.42.0 prior-local propagation requires one helper-call carrier local followed immediately by one propagation local")
	}
	propagated, secondPropagation := second.Initializer.(*PropagateExpr)
	if !secondPropagation {
		return oneDiagnostic(cp.sources, CodePropagation, CategorySemantic, second.Initializer.SourceSpan(), "v0.42.0 prior-local propagation requires propagation as the second immutable-local initializer")
	}
	contract := inheritedLanguageContract(cp.modules.LanguageContract())
	if (contract == PipeLangLanguageContractV730 || contract == PipeLangLanguageContractV720 || contract == PipeLangLanguageContractV710 || contract == PipeLangLanguageContractV700 || contract == PipeLangLanguageContractV690 || contract == PipeLangLanguageContractV680 || contract == PipeLangLanguageContractV670 || contract == PipeLangLanguageContractV660 || contract == PipeLangLanguageContractV650 || contract == PipeLangLanguageContractV640 || contract == PipeLangLanguageContractV630 || contract == PipeLangLanguageContractV620 || contract == PipeLangLanguageContractV610 || contract == PipeLangLanguageContractV600 || contract == PipeLangLanguageContractV590 || contract == PipeLangLanguageContractV580) || contract == PipeLangLanguageContractV570 || contract == PipeLangLanguageContractV560 || contract == PipeLangLanguageContractV550 || contract == PipeLangLanguageContractV540 || contract == PipeLangLanguageContractV530 {
		if len(method.Params) == 0 || !directCallerArguments(called.Arguments, method.Params) {
			return oneDiagnostic(cp.sources, CodePropagation, CategorySemantic, called.SourceSpan(), fmt.Sprintf("%s multi-parameter helper propagation requires every caller parameter directly once in declaration order", contract))
		}
	} else {
		argument, directArgument := onlyDirectIdentifier(called.Arguments)
		if !directArgument || len(method.Params) != 1 || argument.Name != method.Params[0].Name {
			return oneDiagnostic(cp.sources, CodePropagation, CategorySemantic, called.SourceSpan(), "v0.42.0 prior-local propagation requires one helper call over the method's sole direct parameter")
		}
	}
	carrierReference, directCarrier := propagated.Value.(*IdentExpr)
	if !directCarrier || carrierReference.Name != outer.Name {
		return oneDiagnostic(cp.sources, CodePropagation, CategorySemantic, propagated.Value.SourceSpan(), "v0.42.0 propagation requires a direct reference to the immediately preceding helper-call carrier local")
	}
	carrier, err := cp.resolveType(outer.Type, RelatedSpan{Span: outer.Span, Message: "helper-call carrier local"})
	if err != nil {
		return err
	}
	if !carrier.Equal(declared) {
		return oneDiagnostic(cp.sources, CodePropagation, CategorySemantic, outer.Type.Span, "helper-call carrier local and method return type must be identical", RelatedSpan{Span: method.ReturnType.Span, Message: "method return carrier"})
	}
	allowArithmetic := ((contract == PipeLangLanguageContractV730 || contract == PipeLangLanguageContractV720 || contract == PipeLangLanguageContractV710 || contract == PipeLangLanguageContractV700 || contract == PipeLangLanguageContractV690 || contract == PipeLangLanguageContractV680 || contract == PipeLangLanguageContractV670 || contract == PipeLangLanguageContractV660 || contract == PipeLangLanguageContractV650 || contract == PipeLangLanguageContractV640 || contract == PipeLangLanguageContractV630 || contract == PipeLangLanguageContractV620 || contract == PipeLangLanguageContractV610 || contract == PipeLangLanguageContractV600 || contract == PipeLangLanguageContractV590 || contract == PipeLangLanguageContractV580) || contract == PipeLangLanguageContractV570 || contract == PipeLangLanguageContractV560 || contract == PipeLangLanguageContractV550 || contract == PipeLangLanguageContractV540) && isResolvedSourceArithmeticResult(contract, carrier)
	if !cp.isResolvedOptionalValue(contract, carrier) && !isResolvedBoundedValueResult(contract, carrier) && !allowArithmetic {
		if (contract == PipeLangLanguageContractV730 || contract == PipeLangLanguageContractV720 || contract == PipeLangLanguageContractV710 || contract == PipeLangLanguageContractV700 || contract == PipeLangLanguageContractV690 || contract == PipeLangLanguageContractV680 || contract == PipeLangLanguageContractV670 || contract == PipeLangLanguageContractV660 || contract == PipeLangLanguageContractV650 || contract == PipeLangLanguageContractV640 || contract == PipeLangLanguageContractV630 || contract == PipeLangLanguageContractV620 || contract == PipeLangLanguageContractV610 || contract == PipeLangLanguageContractV600 || contract == PipeLangLanguageContractV590 || contract == PipeLangLanguageContractV580) || contract == PipeLangLanguageContractV570 || contract == PipeLangLanguageContractV560 || contract == PipeLangLanguageContractV550 || contract == PipeLangLanguageContractV540 {
			return oneDiagnostic(cp.sources, CodePropagation, CategorySemantic, outer.Type.Span, fmt.Sprintf("v0.54.0 prior-local propagation requires an admitted Optional, bounded Result, or checked-arithmetic Result, got %s", carrier))
		}
		return oneDiagnostic(cp.sources, CodePropagation, CategorySemantic, outer.Type.Span, fmt.Sprintf("v0.42.0 prior-local propagation requires an admitted Optional or bounded Result, got %s", carrier))
	}
	payload, err := cp.resolveType(second.Type, RelatedSpan{Span: second.Span, Message: "propagation payload local"})
	if err != nil {
		return err
	}
	if len(carrier.Arguments) == 0 || !payload.Equal(carrier.Arguments[0]) {
		return oneDiagnostic(cp.sources, CodePropagation, CategorySemantic, second.Type.Span, "second immutable local type must exactly match the propagated success payload")
	}
	return nil
}

func directCallerArguments(arguments []Expr, parameters []Param) bool {
	if len(arguments) != len(parameters) {
		return false
	}
	for position, parameter := range parameters {
		identifier, direct := arguments[position].(*IdentExpr)
		if !direct || identifier.Name != parameter.Name {
			return false
		}
	}
	return true
}

func onlyDirectIdentifier(arguments []Expr) (*IdentExpr, bool) {
	if len(arguments) != 1 {
		return nil, false
	}
	identifier, ok := arguments[0].(*IdentExpr)
	return identifier, ok
}

func containsPropagationExpression(expression Expr) bool {
	if _, ok := expression.(*PropagateExpr); ok {
		return true
	}
	for _, child := range expressionChildren(expression) {
		if containsPropagationExpression(child) {
			return true
		}
	}
	return false
}

func (cp *checkedProgram) validateRecordTransportSignature(method MethodDecl, result ResolvedTypeRef) (bool, error) {
	resolvedParameters := make([]ResolvedTypeRef, 0, len(method.Params))
	hasRecord := cp.containsResolvedRecordType(result)
	for _, param := range method.Params {
		resolved, err := cp.resolveType(param.Type, RelatedSpan{Span: param.Span, Message: "parameter declaration"})
		if err != nil {
			return false, prefixDiagnostic(err, fmt.Sprintf("class method %s parameter %s: ", method.Name, param.Name))
		}
		resolvedParameters = append(resolvedParameters, resolved)
		hasRecord = hasRecord || cp.containsResolvedRecordType(resolved)
	}
	if !hasRecord {
		return false, nil
	}
	contract := inheritedLanguageContract(cp.modules.LanguageContract())
	identityTransport := len(resolvedParameters) == 1 && cp.isResolvedRecordType(result) && resolvedParameters[0].Equal(result)
	fieldProjection := hasRecordFieldProjectionSourceContract(contract) && len(resolvedParameters) == 1 && cp.isResolvedRecordType(resolvedParameters[0]) && result.Kind == TypeRefPrimitive
	recordConstruction := hasPrimitiveRecordConstructionSourceContract(contract) && cp.recordConstructionSignatureMatches(result, resolvedParameters)
	recordEquality := hasPrimitiveRecordEqualitySourceContract(contract) && cp.recordEqualitySignatureMatches(result, resolvedParameters)
	recordList := hasPrimitiveRecordListSourceContract(contract) && isResolvedRecordList(result) && (len(resolvedParameters) == 0 || (len(resolvedParameters) == 1 && (resolvedParameters[0].Equal(result.Arguments[0]) || resolvedParameters[0].Equal(result))))
	recordListCount := hasPrimitiveRecordListCountSourceContract(contract) && result.Equal(resolvedPrimitive(TypeInt)) && len(resolvedParameters) == 1 && isResolvedRecordList(resolvedParameters[0])
	recordListAppend := hasPrimitiveRecordListAppendSourceContract(contract) && isResolvedRecordList(result) && len(resolvedParameters) == 2 && resolvedParameters[0].Equal(result) && resolvedParameters[1].Equal(result.Arguments[0])
	recordListAt := hasPrimitiveRecordListAtSourceContract(contract) && isResolvedRecordOptional(result) && cp.isResolvedRecordType(result.Arguments[0]) && len(resolvedParameters) == 2 && isResolvedRecordList(resolvedParameters[0]) && resolvedParameters[0].Arguments[0].Equal(result.Arguments[0]) && resolvedParameters[1].Equal(resolvedPrimitive(TypeInt))
	recordListFindByText := hasPrimitiveRecordListFindByTextSourceContract(contract) && isResolvedRecordOptional(result) && cp.isResolvedRecordType(result.Arguments[0]) && len(resolvedParameters) == 2 && isResolvedRecordList(resolvedParameters[0]) && resolvedParameters[0].Arguments[0].Equal(result.Arguments[0]) && resolvedParameters[1].Equal(resolvedPrimitive(TypeString))
	recordListFilterByText := hasPrimitiveRecordListFilterByTextSourceContract(contract) && isResolvedRecordList(result) && cp.isResolvedRecordType(result.Arguments[0]) && len(resolvedParameters) == 2 && resolvedParameters[0].Equal(result) && resolvedParameters[1].Equal(resolvedPrimitive(TypeString))
	recordListFilterContainsCaseFolded := hasPrimitiveRecordListFilterContainsCaseFoldedSourceContract(contract) && isResolvedRecordList(result) && cp.isResolvedRecordType(result.Arguments[0]) && len(resolvedParameters) == 2 && resolvedParameters[0].Equal(result) && resolvedParameters[1].Equal(resolvedPrimitive(TypeString))
	namedPredicate := false
	recordListFilterPredicate := false
	if hasNamedRecordPredicateSourceContract(contract) {
		namedPredicate = result.Equal(resolvedPrimitive(TypeBool)) && len(resolvedParameters) >= 2 && cp.isResolvedRecordType(resolvedParameters[0])
		recordListFilterPredicate = isResolvedRecordList(result) && cp.isResolvedRecordType(result.Arguments[0]) && len(resolvedParameters) >= 2 && resolvedParameters[0].Equal(result)
		for _, parameter := range resolvedParameters[1:] {
			namedPredicate = namedPredicate && parameter.Kind == TypeRefPrimitive
			recordListFilterPredicate = recordListFilterPredicate && parameter.Kind == TypeRefPrimitive
		}
	}
	recordOptional := hasPrimitiveRecordOptionalSourceContract(contract) && cp.optionalRecordSignatureMatches(result, resolvedParameters)
	snapshotResult := hasSnapshotResultSourceContract(contract) && cp.boundedResultSignatureMatches(result, resolvedParameters)
	dependentCarrierHelper := cp.dependentCarrierHelperSignature(method, result, resolvedParameters)
	boundedConditional := hasConditionalSourceContract(contract) && containsConditionalExpression(method.Body) && validBoundedConditionalExpression(method.Body)
	_, immutableLocal := method.Body.(*ImmutableLocalExpr)
	composedRecordOptionalMatch := false
	if ((contract == PipeLangLanguageContractV730 || contract == PipeLangLanguageContractV720 || contract == PipeLangLanguageContractV710 || contract == PipeLangLanguageContractV700 || contract == PipeLangLanguageContractV690 || contract == PipeLangLanguageContractV680 || contract == PipeLangLanguageContractV670 || contract == PipeLangLanguageContractV660 || contract == PipeLangLanguageContractV650 || contract == PipeLangLanguageContractV640 || contract == PipeLangLanguageContractV630 || contract == PipeLangLanguageContractV620 || contract == PipeLangLanguageContractV610 || contract == PipeLangLanguageContractV600 || contract == PipeLangLanguageContractV590 || contract == PipeLangLanguageContractV580) || contract == PipeLangLanguageContractV570 || contract == PipeLangLanguageContractV560 || contract == PipeLangLanguageContractV550 || contract == PipeLangLanguageContractV540 || contract == PipeLangLanguageContractV530 || contract == PipeLangLanguageContractV520 || contract == PipeLangLanguageContractV510 || contract == PipeLangLanguageContractV500 || contract == PipeLangLanguageContractV490 || contract == PipeLangLanguageContractV370 || contract == PipeLangLanguageContractV380 || contract == PipeLangLanguageContractV390 || contract == PipeLangLanguageContractV400 || contract == PipeLangLanguageContractV410 || contract == PipeLangLanguageContractV420 || contract == PipeLangLanguageContractV430 || contract == PipeLangLanguageContractV440 || contract == PipeLangLanguageContractV450 || contract == PipeLangLanguageContractV460 || contract == PipeLangLanguageContractV470 || contract == PipeLangLanguageContractV480) && containsCallExpression(method.Body) && result.Kind == TypeRefPrimitive && len(resolvedParameters) == 1 {
		_, matchBody := method.Body.(*MatchExpr)
		composedRecordOptionalMatch = matchBody && isResolvedRecordOptional(resolvedParameters[0]) && cp.isResolvedRecordType(resolvedParameters[0].Arguments[0])
	}
	_, helperCarrierMatch := method.Body.(*MatchExpr)
	helperCarrierMatch = helperCarrierMatch && ((contract == PipeLangLanguageContractV730 || contract == PipeLangLanguageContractV720 || contract == PipeLangLanguageContractV710 || contract == PipeLangLanguageContractV700 || contract == PipeLangLanguageContractV690 || contract == PipeLangLanguageContractV680 || contract == PipeLangLanguageContractV670 || contract == PipeLangLanguageContractV660 || contract == PipeLangLanguageContractV650 || contract == PipeLangLanguageContractV640 || contract == PipeLangLanguageContractV630 || contract == PipeLangLanguageContractV620 || contract == PipeLangLanguageContractV610 || contract == PipeLangLanguageContractV600 || contract == PipeLangLanguageContractV590 || contract == PipeLangLanguageContractV580) || contract == PipeLangLanguageContractV570 || contract == PipeLangLanguageContractV560 || contract == PipeLangLanguageContractV550 || contract == PipeLangLanguageContractV540 || contract == PipeLangLanguageContractV530 || contract == PipeLangLanguageContractV520 || contract == PipeLangLanguageContractV510 || contract == PipeLangLanguageContractV500 || contract == PipeLangLanguageContractV490 || contract == PipeLangLanguageContractV440 || contract == PipeLangLanguageContractV450 || contract == PipeLangLanguageContractV460 || contract == PipeLangLanguageContractV470 || contract == PipeLangLanguageContractV480) && containsCallExpression(method.Body)
	if !hasPrimitiveRecordSourceContract(contract) || (!identityTransport && !fieldProjection && !recordConstruction && !recordEquality && !recordList && !recordListCount && !recordListAppend && !recordListAt && !recordListFindByText && !recordListFilterByText && !recordListFilterContainsCaseFolded && !namedPredicate && !recordListFilterPredicate && !recordOptional && !snapshotResult && !dependentCarrierHelper && !composedRecordOptionalMatch && !helperCarrierMatch && !boundedConditional && !immutableLocal) {
		return false, oneDiagnostic(cp.sources, CodeInvalidType, CategorySemantic, method.Span, fmt.Sprintf("the %s primitive record is admitted only as one exact identity transport, one-hop primitive field projection, direct declaration-ordered construction, direct structural equality, bounded record-list method, or bounded Optional<R> method", contract))
	}
	return true, nil
}

func (cp *checkedProgram) isResolvedOptionalValue(contract LanguageContract, value ResolvedTypeRef) bool {
	return isResolvedPrimitiveOptional(value) || (hasPrimitiveRecordOptionalSourceContract(contract) && isResolvedRecordOptional(value) && cp.isResolvedRecordType(value.Arguments[0]))
}

func (cp *checkedProgram) optionalRecordSignatureMatches(result ResolvedTypeRef, parameters []ResolvedTypeRef) bool {
	if isResolvedRecordOptional(result) && cp.isResolvedRecordType(result.Arguments[0]) {
		return len(parameters) == 0 || (len(parameters) == 1 && (parameters[0].Equal(result.Arguments[0]) || parameters[0].Equal(result)))
	}
	if result.Equal(resolvedPrimitive(TypeBool)) {
		return len(parameters) == 1 && isResolvedRecordOptional(parameters[0]) && cp.isResolvedRecordType(parameters[0].Arguments[0])
	}
	return cp.isResolvedRecordType(result) && len(parameters) == 2 && isResolvedRecordOptional(parameters[0]) && parameters[0].Arguments[0].Equal(result) && parameters[1].Equal(result)
}

func (cp *checkedProgram) recordConstructionSignatureMatches(result ResolvedTypeRef, parameters []ResolvedTypeRef) bool {
	if !cp.isResolvedRecordType(result) {
		return false
	}
	entry, ok := cp.symbols.lookupIDEntry(result.Symbol)
	if !ok || entry.recordDecl == nil || len(parameters) != len(entry.recordDecl.Fields) {
		return false
	}
	for index, field := range entry.recordDecl.Fields {
		fieldType, err := cp.resolveType(field.Type)
		if err != nil || !parameters[index].Equal(fieldType) {
			return false
		}
	}
	return true
}

func (cp *checkedProgram) recordEqualitySignatureMatches(result ResolvedTypeRef, parameters []ResolvedTypeRef) bool {
	return result.Equal(resolvedPrimitive(TypeBool)) && len(parameters) == 2 && cp.isResolvedRecordType(parameters[0]) && parameters[0].Equal(parameters[1])
}

func (cp *checkedProgram) boundedResultSignatureMatches(result ResolvedTypeRef, parameters []ResolvedTypeRef) bool {
	contract := cp.modules.LanguageContract()
	if isResolvedBoundedValueResult(contract, result) {
		return len(parameters) == 1 && (parameters[0].Equal(result.Arguments[0]) || parameters[0].Equal(result.Arguments[1]) || parameters[0].Equal(result))
	}
	if result.Equal(resolvedPrimitive(TypeBool)) {
		return len(parameters) == 1 && isResolvedBoundedValueResult(contract, parameters[0])
	}
	if isResolvedRecordList(result) {
		return len(parameters) == 2 && isResolvedSnapshotResult(parameters[0]) && parameters[0].Arguments[0].Equal(result) && parameters[1].Equal(result)
	}
	if result.Equal(resolvedPrimitive(TypeString)) {
		return len(parameters) == 2 && isResolvedBoundedValueResult(contract, parameters[0]) && (parameters[0].Arguments[0].Equal(result) || parameters[0].Arguments[1].Equal(result)) && parameters[1].Equal(result)
	}
	return false
}

func (cp *checkedProgram) validateResultSignature(method MethodDecl, result ResolvedTypeRef) (bool, error) {
	hasResult := containsResolvedResult(result)
	hasResultParameter := false
	resolvedParameters := make([]ResolvedTypeRef, 0, len(method.Params))
	for _, param := range method.Params {
		resolved, err := cp.resolveType(param.Type, RelatedSpan{Span: param.Span, Message: "parameter declaration"})
		if err != nil {
			return false, prefixDiagnostic(err, fmt.Sprintf("class method %s parameter %s: ", method.Name, param.Name))
		}
		resolvedParameters = append(resolvedParameters, resolved)
		hasResultParameter = hasResultParameter || containsResolvedResult(resolved)
		hasResult = hasResult || containsResolvedResult(resolved)
	}
	if !hasResult {
		return false, nil
	}
	contract := cp.modules.LanguageContract()
	if hasConditionalSourceContract(contract) && containsConditionalExpression(method.Body) && validBoundedConditionalExpression(method.Body) {
		return hasResultParameter, nil
	}
	if hasImmutableLocalSourceContract(contract) {
		if _, ok := method.Body.(*ImmutableLocalExpr); ok {
			return hasResultParameter, nil
		}
	}
	if _, ok := method.Body.(*MatchExpr); ok && hasMatchSourceContract(contract) && len(resolvedParameters) == 1 && (isResolvedBoundedValueResult(contract, resolvedParameters[0]) || isResolvedSourceArithmeticResult(contract, resolvedParameters[0])) {
		return true, nil
	}
	if hasSnapshotResultSourceContract(contract) && cp.boundedResultSignatureMatches(result, resolvedParameters) {
		return hasResultParameter, nil
	}
	if cp.dependentCarrierHelperSignature(method, result, resolvedParameters) {
		return hasResultParameter, nil
	}
	if isResolvedSourceArithmeticResult(contract, result) && !hasResultParameter {
		return false, nil
	}
	if !hasResultTransportSourceContract(contract) || len(resolvedParameters) != 1 || !isResolvedSourceArithmeticResult(contract, result) || !resolvedParameters[0].Equal(result) {
		return false, oneDiagnostic(cp.sources, CodeInvalidType, CategorySemantic, method.Span, fmt.Sprintf("the %s Result contract admits only exact checked-arithmetic transport or bounded snapshot/text Result methods", contract))
	}
	return true, nil
}

func (cp *checkedProgram) validateOptionalSignature(method MethodDecl, result ResolvedTypeRef) (bool, error) {
	resolvedParameters := make([]ResolvedTypeRef, 0, len(method.Params))
	hasOptional := containsResolvedOptional(result)
	for _, param := range method.Params {
		resolved, err := cp.resolveType(param.Type, RelatedSpan{Span: param.Span, Message: "parameter declaration"})
		if err != nil {
			return false, prefixDiagnostic(err, fmt.Sprintf("class method %s parameter %s: ", method.Name, param.Name))
		}
		resolvedParameters = append(resolvedParameters, resolved)
		hasOptional = hasOptional || containsResolvedOptional(resolved)
	}
	if !hasOptional {
		return false, nil
	}
	contract := cp.modules.LanguageContract()
	if !hasPrimitiveOptionalSourceContract(contract) {
		return false, oneDiagnostic(cp.sources, CodeInvalidType, CategorySemantic, method.Span, fmt.Sprintf("primitive Optional methods require language contract %q", PipeLangLanguageContractV130))
	}
	if hasConditionalSourceContract(contract) && containsConditionalExpression(method.Body) && validBoundedConditionalExpression(method.Body) {
		return true, nil
	}
	if hasImmutableLocalSourceContract(contract) {
		if _, ok := method.Body.(*ImmutableLocalExpr); ok {
			return true, nil
		}
	}
	valid := false
	if _, ok := method.Body.(*MatchExpr); ok && hasMatchSourceContract(contract) && len(resolvedParameters) == 1 && cp.isResolvedOptionalValue(contract, resolvedParameters[0]) {
		return true, nil
	}
	_, findByTextBody := method.Body.(*ListFindByTextExpr)
	if findByTextBody && hasPrimitiveRecordListFindByTextSourceContract(contract) {
		valid = isResolvedRecordOptional(result) && cp.isResolvedRecordType(result.Arguments[0]) && len(resolvedParameters) == 2 && isResolvedRecordList(resolvedParameters[0]) && resolvedParameters[0].Arguments[0].Equal(result.Arguments[0]) && resolvedParameters[1].Equal(resolvedPrimitive(TypeString))
	} else if hasPrimitiveRecordListAtSourceContract(contract) && isResolvedRecordOptional(result) && cp.isResolvedRecordType(result.Arguments[0]) && len(resolvedParameters) == 2 && isResolvedRecordList(resolvedParameters[0]) && resolvedParameters[0].Arguments[0].Equal(result.Arguments[0]) && resolvedParameters[1].Equal(resolvedPrimitive(TypeInt)) {
		valid = true
	} else if cp.isResolvedOptionalValue(contract, result) {
		valid = len(resolvedParameters) == 0 ||
			(len(resolvedParameters) == 1 && (resolvedParameters[0].Equal(result.Arguments[0]) || resolvedParameters[0].Equal(result)))
	} else if hasPrimitiveOptionalDefaultSourceContract(contract) {
		valid = len(resolvedParameters) == 2 && cp.isResolvedOptionalValue(contract, resolvedParameters[0]) && resolvedParameters[0].Arguments[0].Equal(result) && resolvedParameters[1].Equal(result)
		if !valid && result.Equal(resolvedPrimitive(TypeBool)) {
			valid = len(resolvedParameters) == 1 && cp.isResolvedOptionalValue(contract, resolvedParameters[0])
		}
	} else if result.Equal(resolvedPrimitive(TypeBool)) {
		valid = len(resolvedParameters) == 1 && cp.isResolvedOptionalValue(contract, resolvedParameters[0])
	}
	if !valid {
		valid = cp.dependentCarrierHelperSignature(method, result, resolvedParameters)
	}
	if !valid {
		forms := "direct some, none, identity transport, or has_value class methods"
		if hasPrimitiveOptionalDefaultSourceContract(contract) {
			forms = "direct some, none, identity transport, has_value, or value_or class methods"
		}
		return false, oneDiagnostic(cp.sources, CodeInvalidType, CategorySemantic, method.Span, fmt.Sprintf("the %s primitive Optional is admitted only as %s", contract, forms))
	}
	for _, parameter := range resolvedParameters {
		if cp.isResolvedOptionalValue(contract, parameter) {
			return true, nil
		}
	}
	return false, nil
}

func (cp *checkedProgram) dependentCarrierHelperSignature(method MethodDecl, result ResolvedTypeRef, parameters []ResolvedTypeRef) bool {
	if cp == nil || cp.modules == nil || (cp.modules.LanguageContract() != PipeLangLanguageContractV500 && cp.modules.LanguageContract() != PipeLangLanguageContractV510 && (cp.modules.LanguageContract() != PipeLangLanguageContractV730 && cp.modules.LanguageContract() != PipeLangLanguageContractV720 && cp.modules.LanguageContract() != PipeLangLanguageContractV710 && cp.modules.LanguageContract() != PipeLangLanguageContractV700 && cp.modules.LanguageContract() != PipeLangLanguageContractV690 && cp.modules.LanguageContract() != PipeLangLanguageContractV680 && cp.modules.LanguageContract() != PipeLangLanguageContractV670 && cp.modules.LanguageContract() != PipeLangLanguageContractV660 && cp.modules.LanguageContract() != PipeLangLanguageContractV650 && cp.modules.LanguageContract() != PipeLangLanguageContractV640 && cp.modules.LanguageContract() != PipeLangLanguageContractV630 && cp.modules.LanguageContract() != PipeLangLanguageContractV620 && cp.modules.LanguageContract() != PipeLangLanguageContractV610 && cp.modules.LanguageContract() != PipeLangLanguageContractV600 && cp.modules.LanguageContract() != PipeLangLanguageContractV590 && cp.modules.LanguageContract() != PipeLangLanguageContractV580) && cp.modules.LanguageContract() != PipeLangLanguageContractV570 && cp.modules.LanguageContract() != PipeLangLanguageContractV560 && cp.modules.LanguageContract() != PipeLangLanguageContractV550 && cp.modules.LanguageContract() != PipeLangLanguageContractV540 && cp.modules.LanguageContract() != PipeLangLanguageContractV530 && cp.modules.LanguageContract() != PipeLangLanguageContractV520) || normalizeVisibility(method.Visibility) != VisibilityPublic {
		return false
	}
	owner := cp.ownerClassForMethod(method)
	if owner == nil {
		return false
	}
	for _, caller := range owner.Methods {
		pairs := findPriorLocalCarrierMatches(caller.Body)
		if len(pairs) < 2 || (cp.modules.LanguageContract() == PipeLangLanguageContractV500 && len(pairs) != 2) {
			continue
		}
		for position := 1; position < len(pairs); position++ {
			call, ok := pairs[position].carrier.Initializer.(*CallExpr)
			if !ok || call.Name != method.Name {
				continue
			}
			selectedCount := 1
			if ((cp.modules.LanguageContract() == PipeLangLanguageContractV730 || cp.modules.LanguageContract() == PipeLangLanguageContractV720 || cp.modules.LanguageContract() == PipeLangLanguageContractV710 || cp.modules.LanguageContract() == PipeLangLanguageContractV700 || cp.modules.LanguageContract() == PipeLangLanguageContractV690 || cp.modules.LanguageContract() == PipeLangLanguageContractV680 || cp.modules.LanguageContract() == PipeLangLanguageContractV670 || cp.modules.LanguageContract() == PipeLangLanguageContractV660 || cp.modules.LanguageContract() == PipeLangLanguageContractV650 || cp.modules.LanguageContract() == PipeLangLanguageContractV640 || cp.modules.LanguageContract() == PipeLangLanguageContractV630 || cp.modules.LanguageContract() == PipeLangLanguageContractV620 || cp.modules.LanguageContract() == PipeLangLanguageContractV610 || cp.modules.LanguageContract() == PipeLangLanguageContractV600 || cp.modules.LanguageContract() == PipeLangLanguageContractV590 || cp.modules.LanguageContract() == PipeLangLanguageContractV580) || cp.modules.LanguageContract() == PipeLangLanguageContractV570 || cp.modules.LanguageContract() == PipeLangLanguageContractV560 || cp.modules.LanguageContract() == PipeLangLanguageContractV550 || cp.modules.LanguageContract() == PipeLangLanguageContractV540 || cp.modules.LanguageContract() == PipeLangLanguageContractV530 || cp.modules.LanguageContract() == PipeLangLanguageContractV520) && len(parameters) == len(caller.Params)+position {
				selectedCount = position
			}
			if len(parameters) != len(caller.Params)+selectedCount {
				continue
			}
			exact := true
			for selectedPosition := 0; selectedPosition < selectedCount; selectedPosition++ {
				pairPosition := position - 1
				if selectedCount > 1 {
					pairPosition = selectedPosition
				}
				selected, err := cp.resolveType(pairs[pairPosition].matched.Type)
				if err != nil || !parameters[selectedPosition].Equal(selected) {
					exact = false
					break
				}
			}
			if !exact {
				continue
			}
			carrier, err := cp.resolveType(pairs[position].carrier.Type)
			if err != nil || !carrier.Equal(result) {
				continue
			}
			for index, callerParameter := range caller.Params {
				resolved, resolveErr := cp.resolveType(callerParameter.Type)
				if resolveErr != nil || !parameters[index+selectedCount].Equal(resolved) {
					exact = false
					break
				}
			}
			if exact {
				return true
			}
		}
	}
	return false
}

func (cp *checkedProgram) validateParams(owner, method string, params []Param, allowArithmeticResult, allowRecord, allowOptional bool) error {
	seen := map[string]Span{}
	for _, param := range params {
		resolved, err := cp.resolveType(param.Type, RelatedSpan{Span: param.Span, Message: "parameter declaration"})
		if err != nil {
			return prefixDiagnostic(err, fmt.Sprintf("%s method %s parameter %s: ", owner, method, param.Name))
		}
		if (containsResolvedResult(resolved) || containsResolvedArithmeticContractType(resolved)) && !allowArithmeticResult {
			return oneDiagnostic(cp.sources, CodeInvalidType, CategorySemantic, param.Type.Span, fmt.Sprintf("the %s Result contract is not admitted in this parameter position", cp.modules.LanguageContract()))
		}
		if cp.containsResolvedRecordType(resolved) && !allowRecord {
			return oneDiagnostic(cp.sources, CodeInvalidType, CategorySemantic, param.Type.Span, fmt.Sprintf("the %s primitive record is not admitted in this parameter position", cp.modules.LanguageContract()))
		}
		if containsResolvedOptional(resolved) && !allowOptional {
			return oneDiagnostic(cp.sources, CodeInvalidType, CategorySemantic, param.Type.Span, fmt.Sprintf("the %s primitive Optional is not admitted in this parameter position", cp.modules.LanguageContract()))
		}
		if previous, ok := seen[param.Name]; ok {
			return oneDiagnostic(cp.sources, CodeDuplicateMember, CategorySemantic, param.Span, fmt.Sprintf("%s method %s has duplicate parameter %q", owner, method, param.Name), RelatedSpan{Span: previous, Message: "first parameter"})
		}
		seen[param.Name] = param.Span
	}
	return nil
}

func (cp *checkedProgram) validateImplements(class *ClassDecl) error {
	if class.Implements == nil {
		return nil
	}
	name := strings.TrimSpace(class.Implements.Name)
	if name == "IComparable" {
		return nil
	}
	entry, err := cp.resolveNamedEntry(*class.Implements)
	ok := err == nil
	if !ok {
		if diagnostics, structured := AsDiagnostics(err); structured && len(diagnostics) > 0 {
			diagnostic := diagnostics[0]
			diagnostic.Code = CodeUnknownInterface
			diagnostic.Message = fmt.Sprintf("class %s implements unknown interface %q", class.Name, name)
			diagnostic.Related = append(diagnostic.Related, RelatedSpan{Span: class.Span, Message: "implementing class"})
			return diagnosticError(cp.sources, Diagnostics{diagnostic})
		}
		return oneDiagnostic(cp.sources, CodeUnknownInterface, CategorySemantic, class.Implements.Span, fmt.Sprintf("class %s implements unknown interface %q", class.Name, name), RelatedSpan{Span: class.Span, Message: "implementing class"})
	}
	if entry.symbol.Kind != SymbolInterface {
		return oneDiagnostic(cp.sources, CodeUnknownInterface, CategorySemantic, class.Implements.Span, fmt.Sprintf("class %s cannot implement non-interface %q", class.Name, name), RelatedSpan{Span: entry.symbol.DeclarationSpan, Message: "resolved class declaration"})
	}
	iface := entry.interfaceDecl
	fields := map[string]FieldDecl{}
	for _, field := range class.Fields {
		fields[field.Name] = field
	}
	methods := map[string]MethodDecl{}
	for _, method := range class.Methods {
		methods[method.Name] = method
	}
	for _, required := range iface.Fields {
		actual, ok := fields[required.Name]
		if !ok {
			return oneDiagnostic(cp.sources, CodeConformance, CategorySemantic, class.Span, fmt.Sprintf("class %s missing interface field %s.%s", class.Name, iface.Name, required.Name), RelatedSpan{Span: required.Span, Message: "required interface field"})
		}
		if normalizeVisibility(actual.Visibility) != VisibilityPublic {
			return oneDiagnostic(cp.sources, CodeConformance, CategorySemantic, actual.Span, fmt.Sprintf("class %s field %s must be public to satisfy interface %s", class.Name, required.Name, iface.Name), RelatedSpan{Span: required.Span, Message: "required interface field"})
		}
		actualType, err := cp.resolveType(actual.Type)
		if err != nil {
			return err
		}
		requiredType, err := cp.resolveType(required.Type)
		if err != nil {
			return err
		}
		if !actualType.Equal(requiredType) {
			return oneDiagnostic(cp.sources, CodeConformance, CategorySemantic, actual.Type.Span, fmt.Sprintf("class %s field %s type %s does not match interface %s", class.Name, required.Name, actualType, requiredType), RelatedSpan{Span: required.Type.Span, Message: "interface field type"})
		}
	}
	for _, required := range iface.Methods {
		actual, ok := methods[required.Name]
		if !ok {
			return oneDiagnostic(cp.sources, CodeConformance, CategorySemantic, class.Span, fmt.Sprintf("class %s missing interface method %s.%s", class.Name, iface.Name, required.Name), RelatedSpan{Span: required.Span, Message: "required interface method"})
		}
		if normalizeVisibility(actual.Visibility) != VisibilityPublic {
			return oneDiagnostic(cp.sources, CodeConformance, CategorySemantic, actual.Span, fmt.Sprintf("class %s method %s must be public to satisfy interface %s", class.Name, required.Name, iface.Name), RelatedSpan{Span: required.Span, Message: "interface method signature"})
		}
		actualReturn, err := cp.resolveType(actual.ReturnType)
		if err != nil {
			return err
		}
		requiredReturn, err := cp.resolveType(required.ReturnType)
		if err != nil {
			return err
		}
		if !actualReturn.Equal(requiredReturn) {
			return oneDiagnostic(cp.sources, CodeConformance, CategorySemantic, actual.ReturnType.Span, fmt.Sprintf("class %s method %s return type %s does not match interface %s", class.Name, required.Name, actualReturn, requiredReturn), RelatedSpan{Span: required.ReturnType.Span, Message: "interface method signature"})
		}
		if len(actual.Params) != len(required.Params) {
			return oneDiagnostic(cp.sources, CodeConformance, CategorySemantic, actual.Span, fmt.Sprintf("class %s method %s parameter count mismatch", class.Name, required.Name), RelatedSpan{Span: required.Span, Message: "interface method signature"})
		}
		for idx := range actual.Params {
			actualParam, err := cp.resolveType(actual.Params[idx].Type)
			if err != nil {
				return err
			}
			requiredParam, err := cp.resolveType(required.Params[idx].Type)
			if err != nil {
				return err
			}
			if !actualParam.Equal(requiredParam) {
				return oneDiagnostic(cp.sources, CodeConformance, CategorySemantic, actual.Params[idx].Type.Span, fmt.Sprintf("class %s method %s parameter %d type %s does not match interface %s", class.Name, required.Name, idx+1, actualParam, iface.Name), RelatedSpan{Span: required.Params[idx].Type.Span, Message: "interface parameter type"})
			}
		}
	}
	return nil
}

func (cp *checkedProgram) resolveNamedEntry(ref UnresolvedTypeRef) (symbolEntry, error) {
	owner := legacySourceSetOwner
	if cp.modules != nil {
		resolved, ok := cp.modules.ownerForSpan(ref.Span)
		if !ok {
			return symbolEntry{}, oneDiagnostic(cp.sources, CodeInvalidModule, CategorySemantic, ref.Span, "type reference has no owning module")
		}
		owner = resolved
		entry, code, related, ok := cp.modules.resolveNamed(cp.symbols, owner, ref)
		if !ok {
			return symbolEntry{}, oneDiagnostic(cp.sources, code, CategorySemantic, ref.Span, fmt.Sprintf("unknown type %q", ref.Name), related...)
		}
		return entry, nil
	}
	entry, ok := cp.symbols.lookupOwnedEntry(owner, ref.Name)
	if !ok {
		return symbolEntry{}, oneDiagnostic(cp.sources, CodeInvalidType, CategorySemantic, ref.Span, fmt.Sprintf("unknown type %q", ref.Name))
	}
	return entry, nil
}

func inferExprType(sources *SourceSet, expr Expr, env map[string]ResolvedTypeRef) (ResolvedTypeRef, error) {
	return inferExprTypeWithPolicy(sources, expr, env, false)
}

func (cp *checkedProgram) inferExprType(expr Expr, env map[string]ResolvedTypeRef) (ResolvedTypeRef, error) {
	if local, ok := expr.(*ImmutableLocalExpr); ok {
		if cp == nil || cp.modules == nil || !hasImmutableLocalSourceContract(cp.modules.LanguageContract()) {
			return ResolvedTypeRef{}, oneDiagnostic(cp.sources, CodeExpressionType, CategorySemantic, local.Span, "immutable local blocks require language contract v0.39.0")
		}
		if _, exists := env[local.Name]; exists {
			return ResolvedTypeRef{}, oneDiagnostic(cp.sources, CodeExpressionType, CategorySemantic, local.NameSpan, fmt.Sprintf("immutable local %q shadows an existing binding", local.Name))
		}
		declared, err := cp.resolveType(local.Type, RelatedSpan{Span: local.Span, Message: "immutable local declaration"})
		if err != nil {
			return ResolvedTypeRef{}, err
		}
		var initialized ResolvedTypeRef
		if isResolvedSourceArithmeticResult(cp.modules.LanguageContract(), declared) {
			initialized, err = cp.inferMethodBodyType(MethodDecl{Body: local.Initializer, ReturnType: local.Type}, env, declared)
		} else {
			initialized, err = cp.inferExprType(local.Initializer, env)
		}
		if err != nil {
			return ResolvedTypeRef{}, err
		}
		if !initialized.Equal(declared) {
			return ResolvedTypeRef{}, oneDiagnostic(cp.sources, CodeExpressionType, CategorySemantic, local.Initializer.SourceSpan(), fmt.Sprintf("immutable local %s initializer has type %s, declared %s", local.Name, initialized, declared), RelatedSpan{Span: local.Type.Span, Message: "declared local type"})
		}
		scoped := make(map[string]ResolvedTypeRef, len(env)+1)
		for name, resolved := range env {
			scoped[name] = resolved
		}
		scoped[local.Name] = declared
		return cp.inferExprType(local.Return, scoped)
	}
	if conditional, ok := expr.(*ConditionalExpr); ok {
		if cp == nil || cp.modules == nil || !hasConditionalSourceContract(cp.modules.LanguageContract()) {
			return ResolvedTypeRef{}, oneDiagnostic(cp.sources, CodeExpressionType, CategorySemantic, conditional.Span, "conditional expressions require language contract v0.38.0")
		}
		condition, err := cp.inferExprType(conditional.Condition, env)
		if err != nil {
			return ResolvedTypeRef{}, err
		}
		if !condition.Equal(resolvedPrimitive(TypeBool)) {
			return ResolvedTypeRef{}, oneDiagnostic(cp.sources, CodeExpressionType, CategorySemantic, conditional.Condition.SourceSpan(), fmt.Sprintf("conditional condition requires bool, got %s", condition))
		}
		whenTrue, err := cp.inferExprType(conditional.WhenTrue, env)
		if err != nil {
			return ResolvedTypeRef{}, err
		}
		whenFalse, err := cp.inferExprType(conditional.WhenFalse, env)
		if err != nil {
			return ResolvedTypeRef{}, err
		}
		if !whenTrue.Equal(whenFalse) {
			return ResolvedTypeRef{}, oneDiagnostic(cp.sources, CodeExpressionType, CategorySemantic, conditional.Span, fmt.Sprintf("conditional branches must have exactly the same type, got %s and %s", whenTrue, whenFalse), RelatedSpan{Span: conditional.WhenTrue.SourceSpan(), Message: "true branch"}, RelatedSpan{Span: conditional.WhenFalse.SourceSpan(), Message: "false branch"})
		}
		return whenTrue, nil
	}
	if call, ok := expr.(*CallExpr); ok {
		if cp == nil || cp.modules == nil || !hasPureCallSourceContract(cp.modules.LanguageContract()) {
			return ResolvedTypeRef{}, oneDiagnostic(cp.sources, CodeExpressionType, CategorySemantic, call.Span, "same-class pure calls require language contract v0.36.0 or later")
		}
		_, target := methodBySpan(cp.program, call.TargetSpan)
		if target == nil || normalizeVisibility(target.Visibility) != VisibilityPublic {
			return ResolvedTypeRef{}, oneDiagnostic(cp.sources, CodeInvalidMember, CategorySemantic, call.NameSpan, fmt.Sprintf("public same-class method %q was not resolved", call.Name))
		}
		if len(call.Arguments) != len(target.Params) {
			return ResolvedTypeRef{}, oneDiagnostic(cp.sources, CodeInvalidType, CategorySemantic, call.Span, fmt.Sprintf("call to %s expects %d arguments, got %d", call.Name, len(target.Params), len(call.Arguments)), RelatedSpan{Span: target.Span, Message: "called method declaration"})
		}
		for position, argument := range call.Arguments {
			actual, err := cp.inferExprType(argument, env)
			if err != nil {
				return ResolvedTypeRef{}, err
			}
			expected, err := cp.resolveType(target.Params[position].Type)
			if err != nil {
				return ResolvedTypeRef{}, err
			}
			if !actual.Equal(expected) {
				return ResolvedTypeRef{}, oneDiagnostic(cp.sources, CodeInvalidType, CategorySemantic, argument.SourceSpan(), fmt.Sprintf("call argument %d to %s requires %s, got %s", position+1, call.Name, expected, actual), RelatedSpan{Span: target.Params[position].Span, Message: "called parameter declaration"})
			}
		}
		return cp.resolveType(target.ReturnType, RelatedSpan{Span: target.Span, Message: "called method declaration"})
	}
	if match, ok := expr.(*MatchExpr); ok {
		if cp == nil || cp.modules == nil || !hasMatchSourceContract(cp.modules.LanguageContract()) {
			return ResolvedTypeRef{}, oneDiagnostic(cp.sources, CodeMatchNonExhaustive, CategorySemantic, match.Span, "match requires language contract v0.35.0")
		}
		identifier, direct := match.Value.(*IdentExpr)
		_, helperResult := match.Value.(*CallExpr)
		carrier, err := cp.inferExprType(match.Value, env)
		if err != nil {
			return ResolvedTypeRef{}, err
		}
		directCarrier := direct && identifier.Name != "" && (cp.isResolvedOptionalValue(cp.modules.LanguageContract(), carrier) || isResolvedBoundedValueResult(cp.modules.LanguageContract(), carrier) || isResolvedSourceArithmeticResult(cp.modules.LanguageContract(), carrier))
		helperCarrier := helperResult && ((hasGeneralHelperCarrierMatchSourceContract(cp.modules.LanguageContract()) && (cp.isResolvedOptionalValue(cp.modules.LanguageContract(), carrier) || isResolvedBoundedValueResult(cp.modules.LanguageContract(), carrier) || ((cp.modules.LanguageContract() == PipeLangLanguageContractV460 || cp.modules.LanguageContract() == PipeLangLanguageContractV470 || cp.modules.LanguageContract() == PipeLangLanguageContractV480 || cp.modules.LanguageContract() == PipeLangLanguageContractV490 || cp.modules.LanguageContract() == PipeLangLanguageContractV500 || cp.modules.LanguageContract() == PipeLangLanguageContractV510 || (cp.modules.LanguageContract() == PipeLangLanguageContractV730 || cp.modules.LanguageContract() == PipeLangLanguageContractV720 || cp.modules.LanguageContract() == PipeLangLanguageContractV710 || cp.modules.LanguageContract() == PipeLangLanguageContractV700 || cp.modules.LanguageContract() == PipeLangLanguageContractV690 || cp.modules.LanguageContract() == PipeLangLanguageContractV680 || cp.modules.LanguageContract() == PipeLangLanguageContractV670 || cp.modules.LanguageContract() == PipeLangLanguageContractV660 || cp.modules.LanguageContract() == PipeLangLanguageContractV650 || cp.modules.LanguageContract() == PipeLangLanguageContractV640 || cp.modules.LanguageContract() == PipeLangLanguageContractV630 || cp.modules.LanguageContract() == PipeLangLanguageContractV620 || cp.modules.LanguageContract() == PipeLangLanguageContractV610 || cp.modules.LanguageContract() == PipeLangLanguageContractV600 || cp.modules.LanguageContract() == PipeLangLanguageContractV590 || cp.modules.LanguageContract() == PipeLangLanguageContractV580) || cp.modules.LanguageContract() == PipeLangLanguageContractV570 || cp.modules.LanguageContract() == PipeLangLanguageContractV560 || cp.modules.LanguageContract() == PipeLangLanguageContractV550 || cp.modules.LanguageContract() == PipeLangLanguageContractV540 || cp.modules.LanguageContract() == PipeLangLanguageContractV530 || cp.modules.LanguageContract() == PipeLangLanguageContractV520) && isResolvedSourceArithmeticResult(cp.modules.LanguageContract(), carrier)))) || (hasHelperResultMatchSourceContract(cp.modules.LanguageContract()) && isResolvedTextResult(carrier)))
		if !directCarrier && !helperCarrier {
			return ResolvedTypeRef{}, oneDiagnostic(cp.sources, CodeInvalidType, CategorySemantic, match.Value.SourceSpan(), "match requires one direct Optional or admitted Result parameter")
		}
		tags := []string{"some", "none"}
		payloads := map[string]ResolvedTypeRef{"some": carrier.Arguments[0]}
		if carrier.Kind == TypeRefApplied && carrier.Name == "Result" {
			tags = []string{"ok", "err"}
			payloads = map[string]ResolvedTypeRef{"ok": carrier.Arguments[0], "err": carrier.Arguments[1]}
		}
		seen := map[string]bool{}
		wildcard := false
		var unified ResolvedTypeRef
		for i, arm := range match.Arms {
			if wildcard {
				return ResolvedTypeRef{}, oneDiagnostic(cp.sources, CodeMatchUnreachable, CategorySemantic, arm.PatternSpan, "match arm is unreachable after _")
			}
			if arm.Tag == "_" {
				wildcard = true
			} else {
				valid := false
				for _, tag := range tags {
					if tag == arm.Tag {
						valid = true
					}
				}
				if !valid {
					return ResolvedTypeRef{}, oneDiagnostic(cp.sources, CodeInvalidType, CategorySemantic, arm.PatternSpan, fmt.Sprintf("pattern %s is not valid for %s", arm.Tag, carrier))
				}
				if seen[arm.Tag] {
					return ResolvedTypeRef{}, oneDiagnostic(cp.sources, CodeMatchDuplicate, CategorySemantic, arm.PatternSpan, fmt.Sprintf("duplicate %s match arm", arm.Tag))
				}
				seen[arm.Tag] = true
			}
			armEnv := make(map[string]ResolvedTypeRef, len(env)+1)
			for k, v := range env {
				armEnv[k] = v
			}
			payload, hasPayload := payloads[arm.Tag]
			if arm.Binding != "" {
				if !hasPayload {
					return ResolvedTypeRef{}, oneDiagnostic(cp.sources, CodeInvalidType, CategorySemantic, arm.PatternSpan, "this match pattern does not bind a payload")
				}
				armEnv[arm.Binding] = payload
			} else if hasPayload {
				return ResolvedTypeRef{}, oneDiagnostic(cp.sources, CodeInvalidType, CategorySemantic, arm.PatternSpan, fmt.Sprintf("%s pattern requires one binding", arm.Tag))
			}
			armType, e := cp.inferExprType(arm.Body, armEnv)
			if e != nil {
				return ResolvedTypeRef{}, e
			}
			if i == 0 {
				unified = armType
			} else if !unified.Equal(armType) {
				return ResolvedTypeRef{}, oneDiagnostic(cp.sources, CodeInvalidType, CategorySemantic, arm.Body.SourceSpan(), fmt.Sprintf("match arms must have exactly one type; expected %s, got %s", unified, armType))
			}
		}
		if !wildcard {
			for _, tag := range tags {
				if !seen[tag] {
					return ResolvedTypeRef{}, oneDiagnostic(cp.sources, CodeMatchNonExhaustive, CategorySemantic, match.Span, fmt.Sprintf("match is not exhaustive; missing %s", tag))
				}
			}
		}
		return unified, nil
	}
	if propagation, ok := expr.(*PropagateExpr); ok {
		if cp == nil || cp.modules == nil || !hasPropagationSourceContract(cp.modules.LanguageContract()) {
			return ResolvedTypeRef{}, oneDiagnostic(cp.sources, CodePropagation, CategorySemantic, propagation.Span, "propagate requires language contract v0.34.0")
		}
		carrier, err := cp.inferExprType(propagation.Value, env)
		if err != nil {
			return ResolvedTypeRef{}, err
		}
		if containsResolvedOptional(carrier) || containsResolvedResult(carrier) {
			return carrier.Arguments[0], nil
		}
		return ResolvedTypeRef{}, oneDiagnostic(cp.sources, CodePropagation, CategorySemantic, propagation.Value.SourceSpan(), fmt.Sprintf("propagate requires an admitted Optional or bounded Result, got %s", carrier))
	}
	if trim, ok := expr.(*TextTrimExpr); ok {
		if cp == nil || cp.modules == nil || !hasTextTrimSourceContract(cp.modules.LanguageContract()) {
			return ResolvedTypeRef{}, oneDiagnostic(cp.sources, CodeExpressionType, CategorySemantic, trim.Span, "text trimming requires language contract v0.26.0")
		}
		value, err := cp.inferExprType(trim.Value, env)
		if err != nil {
			return ResolvedTypeRef{}, err
		}
		stringType := resolvedPrimitive(TypeString)
		if !value.Equal(stringType) {
			return ResolvedTypeRef{}, oneDiagnostic(cp.sources, CodeInvalidType, CategorySemantic, trim.Span, fmt.Sprintf("trim requires string, got %s", value))
		}
		return stringType, nil
	}
	if text, ok := expr.(*TextContainsCaseFoldedExpr); ok {
		if cp == nil || cp.modules == nil || !hasCaseFoldedTextContainmentSourceContract(cp.modules.LanguageContract()) {
			return ResolvedTypeRef{}, oneDiagnostic(cp.sources, CodeExpressionType, CategorySemantic, text.Span, "case-folded text containment requires language contract v0.23.0")
		}
		value, err := cp.inferExprType(text.Value, env)
		if err != nil {
			return ResolvedTypeRef{}, err
		}
		query, err := cp.inferExprType(text.Query, env)
		if err != nil {
			return ResolvedTypeRef{}, err
		}
		stringType := resolvedPrimitive(TypeString)
		if !value.Equal(stringType) || !query.Equal(stringType) {
			return ResolvedTypeRef{}, oneDiagnostic(cp.sources, CodeInvalidType, CategorySemantic, text.Span, fmt.Sprintf("contains_casefolded requires string and string, got %s and %s", value, query))
		}
		return resolvedPrimitive(TypeBool), nil
	}
	if cp != nil && cp.modules != nil && hasSnapshotResultSourceContract(cp.modules.LanguageContract()) {
		switch result := expr.(type) {
		case *ResultOKExpr:
			success, err := cp.resolveType(result.SuccessType)
			if err != nil {
				return ResolvedTypeRef{}, err
			}
			failure, err := cp.resolveType(result.FailureType)
			if err != nil {
				return ResolvedTypeRef{}, err
			}
			resolved := resolvedResult(success, failure)
			if !isResolvedBoundedValueResult(cp.modules.LanguageContract(), resolved) {
				return ResolvedTypeRef{}, oneDiagnostic(cp.sources, CodeInvalidType, CategorySemantic, result.Span, fmt.Sprintf("ok admits only Result<List<R>,string> or v0.25.0 Result<string,string>, got %s", resolved))
			}
			value, err := cp.inferExprType(result.Value, env)
			if err != nil {
				return ResolvedTypeRef{}, err
			}
			if !value.Equal(success) {
				return ResolvedTypeRef{}, oneDiagnostic(cp.sources, CodeInvalidType, CategorySemantic, result.Value.SourceSpan(), fmt.Sprintf("ok payload requires %s, got %s", success, value))
			}
			return resolved, nil
		case *ResultErrExpr:
			success, err := cp.resolveType(result.SuccessType)
			if err != nil {
				return ResolvedTypeRef{}, err
			}
			failure, err := cp.resolveType(result.FailureType)
			if err != nil {
				return ResolvedTypeRef{}, err
			}
			resolved := resolvedResult(success, failure)
			if !isResolvedBoundedValueResult(cp.modules.LanguageContract(), resolved) {
				return ResolvedTypeRef{}, oneDiagnostic(cp.sources, CodeInvalidType, CategorySemantic, result.Span, fmt.Sprintf("err admits only Result<List<R>,string> or v0.25.0 Result<string,string>, got %s", resolved))
			}
			failureValue, err := cp.inferExprType(result.Error, env)
			if err != nil {
				return ResolvedTypeRef{}, err
			}
			if !failureValue.Equal(failure) {
				return ResolvedTypeRef{}, oneDiagnostic(cp.sources, CodeInvalidType, CategorySemantic, result.Error.SourceSpan(), fmt.Sprintf("err payload requires %s, got %s", failure, failureValue))
			}
			return resolved, nil
		case *ResultIsOKExpr:
			value, err := cp.inferExprType(result.Value, env)
			if err != nil {
				return ResolvedTypeRef{}, err
			}
			if !isResolvedBoundedValueResult(cp.modules.LanguageContract(), value) {
				return ResolvedTypeRef{}, oneDiagnostic(cp.sources, CodeInvalidType, CategorySemantic, result.Value.SourceSpan(), fmt.Sprintf("is_ok requires a bounded snapshot/text Result, got %s", value))
			}
			return resolvedPrimitive(TypeBool), nil
		case *ResultSuccessOrExpr:
			value, err := cp.inferExprType(result.Value, env)
			if err != nil {
				return ResolvedTypeRef{}, err
			}
			if !isResolvedBoundedValueResult(cp.modules.LanguageContract(), value) {
				return ResolvedTypeRef{}, oneDiagnostic(cp.sources, CodeInvalidType, CategorySemantic, result.Value.SourceSpan(), fmt.Sprintf("success_or requires a bounded snapshot/text Result, got %s", value))
			}
			fallback, err := cp.inferExprType(result.Fallback, env)
			if err != nil {
				return ResolvedTypeRef{}, err
			}
			if !fallback.Equal(value.Arguments[0]) {
				return ResolvedTypeRef{}, oneDiagnostic(cp.sources, CodeInvalidType, CategorySemantic, result.Fallback.SourceSpan(), fmt.Sprintf("success_or fallback requires %s, got %s", value.Arguments[0], fallback))
			}
			return fallback, nil
		case *ResultFailureOrExpr:
			value, err := cp.inferExprType(result.Value, env)
			if err != nil {
				return ResolvedTypeRef{}, err
			}
			if !isResolvedBoundedValueResult(cp.modules.LanguageContract(), value) {
				return ResolvedTypeRef{}, oneDiagnostic(cp.sources, CodeInvalidType, CategorySemantic, result.Value.SourceSpan(), fmt.Sprintf("failure_or requires a bounded snapshot/text Result, got %s", value))
			}
			fallback, err := cp.inferExprType(result.Fallback, env)
			if err != nil {
				return ResolvedTypeRef{}, err
			}
			if !fallback.Equal(value.Arguments[1]) {
				return ResolvedTypeRef{}, oneDiagnostic(cp.sources, CodeInvalidType, CategorySemantic, result.Fallback.SourceSpan(), fmt.Sprintf("failure_or fallback requires %s, got %s", value.Arguments[1], fallback))
			}
			return fallback, nil
		}
	}
	switch list := expr.(type) {
	case *ListSortByOrdinalDirectionsExpr:
		if cp == nil || cp.modules == nil || ((cp.modules.LanguageContract() != PipeLangLanguageContractV870 && cp.modules.LanguageContract() != PipeLangLanguageContractV860) && cp.modules.LanguageContract() != PipeLangLanguageContractV850 && cp.modules.LanguageContract() != PipeLangLanguageContractV840 && cp.modules.LanguageContract() != PipeLangLanguageContractV830 && cp.modules.LanguageContract() != PipeLangLanguageContractV820 && cp.modules.LanguageContract() != PipeLangLanguageContractV810 && cp.modules.LanguageContract() != PipeLangLanguageContractV800 && cp.modules.LanguageContract() != PipeLangLanguageContractV790 && cp.modules.LanguageContract() != PipeLangLanguageContractV780 && cp.modules.LanguageContract() != PipeLangLanguageContractV770 && cp.modules.LanguageContract() != PipeLangLanguageContractV760 && cp.modules.LanguageContract() != PipeLangLanguageContractV750 && cp.modules.LanguageContract() != PipeLangLanguageContractV740 && (cp.modules.LanguageContract() != PipeLangLanguageContractV730 && cp.modules.LanguageContract() != PipeLangLanguageContractV720 && cp.modules.LanguageContract() != PipeLangLanguageContractV710 && cp.modules.LanguageContract() != PipeLangLanguageContractV700 && cp.modules.LanguageContract() != PipeLangLanguageContractV690 && cp.modules.LanguageContract() != PipeLangLanguageContractV680 && cp.modules.LanguageContract() != PipeLangLanguageContractV670 && cp.modules.LanguageContract() != PipeLangLanguageContractV660 && cp.modules.LanguageContract() != PipeLangLanguageContractV650 && cp.modules.LanguageContract() != PipeLangLanguageContractV640 && cp.modules.LanguageContract() != PipeLangLanguageContractV630 && cp.modules.LanguageContract() != PipeLangLanguageContractV620 && cp.modules.LanguageContract() != PipeLangLanguageContractV610 && cp.modules.LanguageContract() != PipeLangLanguageContractV600 && cp.modules.LanguageContract() != PipeLangLanguageContractV590 && cp.modules.LanguageContract() != PipeLangLanguageContractV580) && cp.modules.LanguageContract() != PipeLangLanguageContractV570 && cp.modules.LanguageContract() != PipeLangLanguageContractV560 && cp.modules.LanguageContract() != PipeLangLanguageContractV550 && cp.modules.LanguageContract() != PipeLangLanguageContractV540 && cp.modules.LanguageContract() != PipeLangLanguageContractV530 && cp.modules.LanguageContract() != PipeLangLanguageContractV520 && cp.modules.LanguageContract() != PipeLangLanguageContractV510 && cp.modules.LanguageContract() != PipeLangLanguageContractV500 && cp.modules.LanguageContract() != PipeLangLanguageContractV490 && cp.modules.LanguageContract() != PipeLangLanguageContractV320 && cp.modules.LanguageContract() != PipeLangLanguageContractV330 && cp.modules.LanguageContract() != PipeLangLanguageContractV340 && cp.modules.LanguageContract() != PipeLangLanguageContractV350 && cp.modules.LanguageContract() != PipeLangLanguageContractV360 && cp.modules.LanguageContract() != PipeLangLanguageContractV370 && cp.modules.LanguageContract() != PipeLangLanguageContractV380 && cp.modules.LanguageContract() != PipeLangLanguageContractV390 && cp.modules.LanguageContract() != PipeLangLanguageContractV400 && cp.modules.LanguageContract() != PipeLangLanguageContractV410 && cp.modules.LanguageContract() != PipeLangLanguageContractV420 && cp.modules.LanguageContract() != PipeLangLanguageContractV430 && cp.modules.LanguageContract() != PipeLangLanguageContractV440 && cp.modules.LanguageContract() != PipeLangLanguageContractV450 && cp.modules.LanguageContract() != PipeLangLanguageContractV460 && cp.modules.LanguageContract() != PipeLangLanguageContractV470 && cp.modules.LanguageContract() != PipeLangLanguageContractV480) {
			return ResolvedTypeRef{}, oneDiagnostic(cp.sources, CodeExpressionType, CategorySemantic, list.Span, "directional record-list ordinal sorting requires language contract v0.32.0")
		}
		values, err := cp.inferExprType(list.Values, env)
		if err != nil {
			return ResolvedTypeRef{}, err
		}
		if !isResolvedRecordList(values) || !cp.isResolvedRecordType(values.Arguments[0]) {
			return ResolvedTypeRef{}, oneDiagnostic(cp.sources, CodeInvalidType, CategorySemantic, list.Values.SourceSpan(), fmt.Sprintf("sort_by_ordinal requires one existing primitive-record List value first, got %s", values))
		}
		plain := &ListSortByOrdinalsExpr{Values: list.Values, Span: list.Span}
		for _, selector := range list.Selectors {
			plain.Selectors = append(plain.Selectors, selector.ListTextFieldSelector)
		}
		if len(plain.Selectors) == 1 {
			single := &ListSortByOrdinalExpr{Values: list.Values, RecordType: plain.Selectors[0].RecordType, Field: plain.Selectors[0].Field, FieldSpan: plain.Selectors[0].FieldSpan, Span: list.Span}
			if _, _, err := cp.resolveListSortByOrdinalSelector(single, values); err != nil {
				return ResolvedTypeRef{}, err
			}
		} else if _, _, err := cp.resolveListSortByOrdinalsSelectors(plain, values); err != nil {
			return ResolvedTypeRef{}, err
		}
		return values, nil
	case *ListSortByOrdinalExpr:
		if cp == nil || cp.modules == nil || !hasPrimitiveRecordListSortByOrdinalSourceContract(cp.modules.LanguageContract()) {
			return ResolvedTypeRef{}, oneDiagnostic(cp.sources, CodeExpressionType, CategorySemantic, list.Span, "record-list ordinal sorting requires language contract v0.28.0")
		}
		values, err := cp.inferExprType(list.Values, env)
		if err != nil {
			return ResolvedTypeRef{}, err
		}
		if !isResolvedRecordList(values) || !cp.isResolvedRecordType(values.Arguments[0]) {
			return ResolvedTypeRef{}, oneDiagnostic(cp.sources, CodeInvalidType, CategorySemantic, list.Values.SourceSpan(), fmt.Sprintf("sort_by_ordinal requires one existing primitive-record List value first, got %s", values))
		}
		if _, _, err := cp.resolveListSortByOrdinalSelector(list, values); err != nil {
			return ResolvedTypeRef{}, err
		}
		return values, nil
	case *ListSortByOrdinalsExpr:
		if cp == nil || cp.modules == nil || !hasPrimitiveRecordListSortByOrdinalsSourceContract(cp.modules.LanguageContract()) {
			return ResolvedTypeRef{}, oneDiagnostic(cp.sources, CodeExpressionType, CategorySemantic, list.Span, "multi-key record-list ordinal sorting requires language contract v0.30.0")
		}
		values, err := cp.inferExprType(list.Values, env)
		if err != nil {
			return ResolvedTypeRef{}, err
		}
		if !isResolvedRecordList(values) || !cp.isResolvedRecordType(values.Arguments[0]) {
			return ResolvedTypeRef{}, oneDiagnostic(cp.sources, CodeInvalidType, CategorySemantic, list.Values.SourceSpan(), fmt.Sprintf("sort_by_ordinal requires one existing primitive-record List value first, got %s", values))
		}
		if _, _, err := cp.resolveListSortByOrdinalsSelectors(list, values); err != nil {
			return ResolvedTypeRef{}, err
		}
		return values, nil
	case *ListFilterJoinedContainsCaseFoldedExpr:
		if cp == nil || cp.modules == nil || !hasPrimitiveRecordListFilterJoinedContainsCaseFoldedSourceContract(cp.modules.LanguageContract()) {
			return ResolvedTypeRef{}, oneDiagnostic(cp.sources, CodeExpressionType, CategorySemantic, list.Span, "record-list joined-field case-folded filtering requires language contract v0.27.0")
		}
		values, err := cp.inferExprType(list.Values, env)
		if err != nil {
			return ResolvedTypeRef{}, err
		}
		if !isResolvedRecordList(values) || !cp.isResolvedRecordType(values.Arguments[0]) {
			return ResolvedTypeRef{}, oneDiagnostic(cp.sources, CodeInvalidType, CategorySemantic, list.Values.SourceSpan(), fmt.Sprintf("filter_joined_contains_casefolded requires one existing primitive-record List value first, got %s", values))
		}
		if _, _, err := cp.resolveListFilterJoinedContainsCaseFoldedSelectors(list, values); err != nil {
			return ResolvedTypeRef{}, err
		}
		query, err := cp.inferExprType(list.Query, env)
		if err != nil {
			return ResolvedTypeRef{}, err
		}
		if !query.Equal(resolvedPrimitive(TypeString)) {
			return ResolvedTypeRef{}, oneDiagnostic(cp.sources, CodeInvalidType, CategorySemantic, list.Query.SourceSpan(), fmt.Sprintf("filter_joined_contains_casefolded requires a string query last, got %s", query))
		}
		return values, nil
	case *ListFilterContainsCaseFoldedExpr:
		if cp == nil || cp.modules == nil || !hasPrimitiveRecordListFilterContainsCaseFoldedSourceContract(cp.modules.LanguageContract()) {
			return ResolvedTypeRef{}, oneDiagnostic(cp.sources, CodeExpressionType, CategorySemantic, list.Span, "record-list selected-field case-folded filtering requires language contract v0.24.0")
		}
		values, err := cp.inferExprType(list.Values, env)
		if err != nil {
			return ResolvedTypeRef{}, err
		}
		if !isResolvedRecordList(values) || !cp.isResolvedRecordType(values.Arguments[0]) {
			return ResolvedTypeRef{}, oneDiagnostic(cp.sources, CodeInvalidType, CategorySemantic, list.Values.SourceSpan(), fmt.Sprintf("filter_contains_casefolded requires one existing primitive-record List value first, got %s", values))
		}
		if _, _, err := cp.resolveListFilterContainsCaseFoldedSelector(list, values); err != nil {
			return ResolvedTypeRef{}, err
		}
		query, err := cp.inferExprType(list.Query, env)
		if err != nil {
			return ResolvedTypeRef{}, err
		}
		if !query.Equal(resolvedPrimitive(TypeString)) {
			return ResolvedTypeRef{}, oneDiagnostic(cp.sources, CodeInvalidType, CategorySemantic, list.Query.SourceSpan(), fmt.Sprintf("filter_contains_casefolded requires a string query third, got %s", query))
		}
		return values, nil
	case *ListFilterByTextExpr:
		if cp == nil || cp.modules == nil || !hasPrimitiveRecordListFilterByTextSourceContract(cp.modules.LanguageContract()) {
			return ResolvedTypeRef{}, oneDiagnostic(cp.sources, CodeExpressionType, CategorySemantic, list.Span, "record-list selected-field filtering requires language contract v0.22.0")
		}
		values, err := cp.inferExprType(list.Values, env)
		if err != nil {
			return ResolvedTypeRef{}, err
		}
		if !isResolvedRecordList(values) || !cp.isResolvedRecordType(values.Arguments[0]) {
			return ResolvedTypeRef{}, oneDiagnostic(cp.sources, CodeInvalidType, CategorySemantic, list.Values.SourceSpan(), fmt.Sprintf("filter_by requires one existing primitive-record List value first, got %s", values))
		}
		if _, _, err := cp.resolveListFilterByTextSelector(list, values); err != nil {
			return ResolvedTypeRef{}, err
		}
		key, err := cp.inferExprType(list.Key, env)
		if err != nil {
			return ResolvedTypeRef{}, err
		}
		if !key.Equal(resolvedPrimitive(TypeString)) {
			return ResolvedTypeRef{}, oneDiagnostic(cp.sources, CodeInvalidType, CategorySemantic, list.Key.SourceSpan(), fmt.Sprintf("filter_by requires a string key third, got %s", key))
		}
		return values, nil
	case *ListFindByTextExpr:
		if cp == nil || cp.modules == nil || !hasPrimitiveRecordListFindByTextSourceContract(cp.modules.LanguageContract()) {
			return ResolvedTypeRef{}, oneDiagnostic(cp.sources, CodeExpressionType, CategorySemantic, list.Span, "record-list stable-key lookup requires language contract v0.21.0")
		}
		values, err := cp.inferExprType(list.Values, env)
		if err != nil {
			return ResolvedTypeRef{}, err
		}
		if !isResolvedRecordList(values) || !cp.isResolvedRecordType(values.Arguments[0]) {
			return ResolvedTypeRef{}, oneDiagnostic(cp.sources, CodeInvalidType, CategorySemantic, list.Values.SourceSpan(), fmt.Sprintf("find_by requires one existing primitive-record List value first, got %s", values))
		}
		if _, _, err := cp.resolveListFindByTextSelector(list, values); err != nil {
			return ResolvedTypeRef{}, err
		}
		key, err := cp.inferExprType(list.Key, env)
		if err != nil {
			return ResolvedTypeRef{}, err
		}
		if !key.Equal(resolvedPrimitive(TypeString)) {
			return ResolvedTypeRef{}, oneDiagnostic(cp.sources, CodeInvalidType, CategorySemantic, list.Key.SourceSpan(), fmt.Sprintf("find_by requires a string key third, got %s", key))
		}
		return resolvedOptional(values.Arguments[0]), nil
	case *ListAtExpr:
		if cp == nil || cp.modules == nil || !hasPrimitiveRecordListAtSourceContract(cp.modules.LanguageContract()) {
			return ResolvedTypeRef{}, oneDiagnostic(cp.sources, CodeExpressionType, CategorySemantic, list.Span, "record-list indexing requires language contract v0.20.0")
		}
		if list.Postfix && (cp.modules.LanguageContract() != PipeLangLanguageContractV870 && cp.modules.LanguageContract() != PipeLangLanguageContractV860) && cp.modules.LanguageContract() != PipeLangLanguageContractV850 && cp.modules.LanguageContract() != PipeLangLanguageContractV840 && cp.modules.LanguageContract() != PipeLangLanguageContractV830 && cp.modules.LanguageContract() != PipeLangLanguageContractV820 && cp.modules.LanguageContract() != PipeLangLanguageContractV810 && cp.modules.LanguageContract() != PipeLangLanguageContractV800 && cp.modules.LanguageContract() != PipeLangLanguageContractV790 && cp.modules.LanguageContract() != PipeLangLanguageContractV780 && cp.modules.LanguageContract() != PipeLangLanguageContractV770 && cp.modules.LanguageContract() != PipeLangLanguageContractV760 && cp.modules.LanguageContract() != PipeLangLanguageContractV750 && cp.modules.LanguageContract() != PipeLangLanguageContractV740 && (cp.modules.LanguageContract() != PipeLangLanguageContractV730 && cp.modules.LanguageContract() != PipeLangLanguageContractV720 && cp.modules.LanguageContract() != PipeLangLanguageContractV710 && cp.modules.LanguageContract() != PipeLangLanguageContractV700 && cp.modules.LanguageContract() != PipeLangLanguageContractV690 && cp.modules.LanguageContract() != PipeLangLanguageContractV680 && cp.modules.LanguageContract() != PipeLangLanguageContractV670 && cp.modules.LanguageContract() != PipeLangLanguageContractV660 && cp.modules.LanguageContract() != PipeLangLanguageContractV650 && cp.modules.LanguageContract() != PipeLangLanguageContractV640 && cp.modules.LanguageContract() != PipeLangLanguageContractV630 && cp.modules.LanguageContract() != PipeLangLanguageContractV620 && cp.modules.LanguageContract() != PipeLangLanguageContractV610 && cp.modules.LanguageContract() != PipeLangLanguageContractV600 && cp.modules.LanguageContract() != PipeLangLanguageContractV590 && cp.modules.LanguageContract() != PipeLangLanguageContractV580) && cp.modules.LanguageContract() != PipeLangLanguageContractV570 && cp.modules.LanguageContract() != PipeLangLanguageContractV560 && cp.modules.LanguageContract() != PipeLangLanguageContractV550 && cp.modules.LanguageContract() != PipeLangLanguageContractV540 && cp.modules.LanguageContract() != PipeLangLanguageContractV530 && cp.modules.LanguageContract() != PipeLangLanguageContractV520 && cp.modules.LanguageContract() != PipeLangLanguageContractV510 && cp.modules.LanguageContract() != PipeLangLanguageContractV500 && cp.modules.LanguageContract() != PipeLangLanguageContractV490 && cp.modules.LanguageContract() != PipeLangLanguageContractV330 && cp.modules.LanguageContract() != PipeLangLanguageContractV340 && cp.modules.LanguageContract() != PipeLangLanguageContractV350 && cp.modules.LanguageContract() != PipeLangLanguageContractV360 && cp.modules.LanguageContract() != PipeLangLanguageContractV370 && cp.modules.LanguageContract() != PipeLangLanguageContractV380 && cp.modules.LanguageContract() != PipeLangLanguageContractV390 && cp.modules.LanguageContract() != PipeLangLanguageContractV400 && cp.modules.LanguageContract() != PipeLangLanguageContractV410 && cp.modules.LanguageContract() != PipeLangLanguageContractV420 && cp.modules.LanguageContract() != PipeLangLanguageContractV430 && cp.modules.LanguageContract() != PipeLangLanguageContractV440 && cp.modules.LanguageContract() != PipeLangLanguageContractV450 && cp.modules.LanguageContract() != PipeLangLanguageContractV460 && cp.modules.LanguageContract() != PipeLangLanguageContractV470 && cp.modules.LanguageContract() != PipeLangLanguageContractV480 {
			return ResolvedTypeRef{}, oneDiagnostic(cp.sources, CodeExpressionType, CategorySemantic, list.Span, "postfix safe indexing requires language contract v0.33.0")
		}
		values, err := cp.inferExprType(list.Values, env)
		if err != nil {
			return ResolvedTypeRef{}, err
		}
		if !isResolvedRecordList(values) || !cp.isResolvedRecordType(values.Arguments[0]) {
			return ResolvedTypeRef{}, oneDiagnostic(cp.sources, CodeInvalidType, CategorySemantic, list.Values.SourceSpan(), fmt.Sprintf("record-list indexing requires a primitive-record List receiver, got %s", values))
		}
		index, err := cp.inferExprType(list.Index, env)
		if err != nil {
			return ResolvedTypeRef{}, err
		}
		if !index.Equal(resolvedPrimitive(TypeInt)) {
			return ResolvedTypeRef{}, oneDiagnostic(cp.sources, CodeInvalidType, CategorySemantic, list.Index.SourceSpan(), fmt.Sprintf("record-list indexing requires a signed 64-bit int index, got %s", index))
		}
		return resolvedOptional(values.Arguments[0]), nil
	case *ListAppendExpr:
		if cp == nil || cp.modules == nil || !hasPrimitiveRecordListAppendSourceContract(cp.modules.LanguageContract()) {
			return ResolvedTypeRef{}, oneDiagnostic(cp.sources, CodeExpressionType, CategorySemantic, list.Span, "record-list append requires language contract v0.17.0")
		}
		values, err := cp.inferExprType(list.Values, env)
		if err != nil {
			return ResolvedTypeRef{}, err
		}
		if !isResolvedRecordList(values) {
			return ResolvedTypeRef{}, oneDiagnostic(cp.sources, CodeInvalidType, CategorySemantic, list.Values.SourceSpan(), fmt.Sprintf("append requires one existing primitive-record List value first, got %s", values))
		}
		value, err := cp.inferExprType(list.Value, env)
		if err != nil {
			return ResolvedTypeRef{}, err
		}
		if !value.Equal(values.Arguments[0]) {
			return ResolvedTypeRef{}, oneDiagnostic(cp.sources, CodeInvalidType, CategorySemantic, list.Value.SourceSpan(), fmt.Sprintf("append requires a matching %s record value second, got %s", values.Arguments[0], value))
		}
		return values, nil
	case *ListCountExpr:
		if cp == nil || cp.modules == nil || !hasPrimitiveRecordListCountSourceContract(cp.modules.LanguageContract()) {
			return ResolvedTypeRef{}, oneDiagnostic(cp.sources, CodeExpressionType, CategorySemantic, list.Span, "record-list count requires language contract v0.16.0")
		}
		value, err := cp.inferExprType(list.Value, env)
		if err != nil {
			return ResolvedTypeRef{}, err
		}
		if !isResolvedRecordList(value) {
			return ResolvedTypeRef{}, oneDiagnostic(cp.sources, CodeInvalidType, CategorySemantic, list.Value.SourceSpan(), fmt.Sprintf("count requires one existing primitive-record List value, got %s", value))
		}
		return resolvedPrimitive(TypeInt), nil
	case *ListEmptyExpr:
		if cp == nil || cp.modules == nil || !hasPrimitiveRecordListSourceContract(cp.modules.LanguageContract()) {
			return ResolvedTypeRef{}, oneDiagnostic(cp.sources, CodeExpressionType, CategorySemantic, list.Span, "record-list construction requires language contract v0.15.0")
		}
		element, err := cp.resolveType(list.ElementType)
		if err != nil {
			return ResolvedTypeRef{}, err
		}
		if !cp.isResolvedRecordType(element) {
			return ResolvedTypeRef{}, oneDiagnostic(cp.sources, CodeInvalidType, CategorySemantic, list.ElementType.Span, fmt.Sprintf("empty_list requires an existing public primitive record type, got %s", element))
		}
		return resolvedRecordList(element), nil
	case *ListSingletonExpr:
		if cp == nil || cp.modules == nil || !hasPrimitiveRecordListSourceContract(cp.modules.LanguageContract()) {
			return ResolvedTypeRef{}, oneDiagnostic(cp.sources, CodeExpressionType, CategorySemantic, list.Span, "record-list construction requires language contract v0.15.0")
		}
		element, err := cp.inferExprType(list.Value, env)
		if err != nil {
			return ResolvedTypeRef{}, err
		}
		if !cp.isResolvedRecordType(element) {
			return ResolvedTypeRef{}, oneDiagnostic(cp.sources, CodeInvalidType, CategorySemantic, list.Value.SourceSpan(), fmt.Sprintf("list requires one existing public primitive record value, got %s", element))
		}
		return resolvedRecordList(element), nil
	}
	switch optional := expr.(type) {
	case *OptionalSomeExpr:
		if cp == nil || cp.modules == nil || !hasPrimitiveOptionalSourceContract(cp.modules.LanguageContract()) {
			return ResolvedTypeRef{}, oneDiagnostic(cp.sources, CodeExpressionType, CategorySemantic, optional.Span, "primitive Optional construction requires language contract v0.13.0")
		}
		value, err := cp.inferExprType(optional.Value, env)
		if err != nil {
			return ResolvedTypeRef{}, err
		}
		if value.Kind != TypeRefPrimitive && !(hasPrimitiveRecordOptionalSourceContract(cp.modules.LanguageContract()) && cp.isResolvedRecordType(value)) {
			return ResolvedTypeRef{}, oneDiagnostic(cp.sources, CodeInvalidType, CategorySemantic, optional.Value.SourceSpan(), fmt.Sprintf("some requires a primitive or existing public primitive-record value, got %s", value))
		}
		return resolvedOptional(value), nil
	case *OptionalNoneExpr:
		if cp == nil || cp.modules == nil || !hasPrimitiveOptionalSourceContract(cp.modules.LanguageContract()) {
			return ResolvedTypeRef{}, oneDiagnostic(cp.sources, CodeExpressionType, CategorySemantic, optional.Span, "primitive Optional construction requires language contract v0.13.0")
		}
		value, err := cp.resolveType(optional.ValueType)
		if err != nil {
			return ResolvedTypeRef{}, err
		}
		if value.Kind != TypeRefPrimitive && !(hasPrimitiveRecordOptionalSourceContract(cp.modules.LanguageContract()) && cp.isResolvedRecordType(value)) {
			return ResolvedTypeRef{}, oneDiagnostic(cp.sources, CodeInvalidType, CategorySemantic, optional.ValueType.Span, fmt.Sprintf("none requires a primitive or existing public primitive-record type, got %s", value))
		}
		return resolvedOptional(value), nil
	case *OptionalHasValueExpr:
		if cp == nil || cp.modules == nil || !hasPrimitiveOptionalSourceContract(cp.modules.LanguageContract()) {
			return ResolvedTypeRef{}, oneDiagnostic(cp.sources, CodeExpressionType, CategorySemantic, optional.Span, "primitive Optional inspection requires language contract v0.13.0")
		}
		value, err := cp.inferExprType(optional.Value, env)
		if err != nil {
			return ResolvedTypeRef{}, err
		}
		if !cp.isResolvedOptionalValue(cp.modules.LanguageContract(), value) {
			return ResolvedTypeRef{}, oneDiagnostic(cp.sources, CodeInvalidType, CategorySemantic, optional.Value.SourceSpan(), fmt.Sprintf("has_value requires an admitted Optional, got %s", value))
		}
		return resolvedPrimitive(TypeBool), nil
	case *OptionalValueOrExpr:
		if cp == nil || cp.modules == nil || !hasPrimitiveOptionalDefaultSourceContract(cp.modules.LanguageContract()) {
			return ResolvedTypeRef{}, oneDiagnostic(cp.sources, CodeExpressionType, CategorySemantic, optional.Span, "primitive Optional defaulting requires language contract v0.14.0")
		}
		value, err := cp.inferExprType(optional.Value, env)
		if err != nil {
			return ResolvedTypeRef{}, err
		}
		if !cp.isResolvedOptionalValue(cp.modules.LanguageContract(), value) {
			return ResolvedTypeRef{}, oneDiagnostic(cp.sources, CodeInvalidType, CategorySemantic, optional.Value.SourceSpan(), fmt.Sprintf("value_or requires an admitted Optional first operand, got %s", value))
		}
		fallback, err := cp.inferExprType(optional.Fallback, env)
		if err != nil {
			return ResolvedTypeRef{}, err
		}
		if !fallback.Equal(value.Arguments[0]) {
			return ResolvedTypeRef{}, oneDiagnostic(cp.sources, CodeInvalidType, CategorySemantic, optional.Fallback.SourceSpan(), fmt.Sprintf("value_or fallback requires %s, got %s", value.Arguments[0], fallback))
		}
		return fallback, nil
	}
	if construction, ok := expr.(*RecordConstructExpr); ok {
		if cp == nil || cp.modules == nil || !hasPrimitiveRecordConstructionSourceContract(cp.modules.LanguageContract()) {
			return ResolvedTypeRef{}, oneDiagnostic(cp.sources, CodeExpressionType, CategorySemantic, construction.Span, "primitive record construction requires language contract v0.11.0")
		}
		resolved, err := cp.resolveType(construction.Type)
		if err != nil {
			return ResolvedTypeRef{}, err
		}
		if !cp.isResolvedRecordType(resolved) {
			return ResolvedTypeRef{}, oneDiagnostic(cp.sources, CodeInvalidType, CategorySemantic, construction.Type.Span, fmt.Sprintf("record construction requires an existing public primitive record, got %s", resolved))
		}
		entry, found := cp.symbols.lookupIDEntry(resolved.Symbol)
		if !found || entry.recordDecl == nil {
			return ResolvedTypeRef{}, oneDiagnostic(cp.sources, CodeInvalidType, CategorySemantic, construction.Type.Span, "record construction has no resolved record declaration")
		}
		if len(construction.Fields) != len(entry.recordDecl.Fields) {
			return ResolvedTypeRef{}, oneDiagnostic(cp.sources, CodeInvalidType, CategorySemantic, construction.Span, fmt.Sprintf("record construction requires exactly %d declaration-ordered fields, got %d", len(entry.recordDecl.Fields), len(construction.Fields)))
		}
		seen := make(map[string]Span, len(construction.Fields))
		for position, initialized := range construction.Fields {
			if previous, duplicate := seen[initialized.Name]; duplicate {
				return ResolvedTypeRef{}, oneDiagnostic(cp.sources, CodeDuplicateMember, CategorySemantic, initialized.NameSpan, fmt.Sprintf("record construction repeats field %q", initialized.Name), RelatedSpan{Span: previous, Message: "first field initializer"})
			}
			seen[initialized.Name] = initialized.NameSpan
			declaredField := entry.recordDecl.Fields[position]
			if initialized.Name != declaredField.Name {
				return ResolvedTypeRef{}, oneDiagnostic(cp.sources, CodeInvalidType, CategorySemantic, initialized.NameSpan, fmt.Sprintf("record construction field %q is out of declaration order; expected %q", initialized.Name, declaredField.Name), RelatedSpan{Span: declaredField.Span, Message: "declared field position"})
			}
			actual, inferErr := cp.inferExprType(initialized.Value, env)
			if inferErr != nil {
				return ResolvedTypeRef{}, inferErr
			}
			expected, resolveErr := cp.resolveType(declaredField.Type)
			if resolveErr != nil {
				return ResolvedTypeRef{}, resolveErr
			}
			if !actual.Equal(expected) {
				return ResolvedTypeRef{}, oneDiagnostic(cp.sources, CodeInvalidType, CategorySemantic, initialized.Value.SourceSpan(), fmt.Sprintf("record field %s requires %s, got %s", initialized.Name, expected, actual), RelatedSpan{Span: declaredField.Span, Message: "record field declaration"})
			}
		}
		return resolved, nil
	}
	if field, ok := expr.(*FieldExpr); ok {
		if cp == nil || cp.modules == nil || !hasRecordFieldProjectionSourceContract(cp.modules.LanguageContract()) {
			return ResolvedTypeRef{}, oneDiagnostic(cp.sources, CodeInvalidMember, CategorySemantic, field.NameSpan, "record field projection requires language contract v0.10.0")
		}
		receiver, err := cp.inferExprType(field.Receiver, env)
		if err != nil {
			return ResolvedTypeRef{}, err
		}
		resolved, _, _, err := cp.resolveRecordField(receiver, field.Name, field.NameSpan)
		return resolved, err
	}
	strictNumeric := cp != nil && cp.modules != nil && isPipeLangSemanticContract(cp.modules.LanguageContract())
	if unary, ok := expr.(*UnaryExpr); ok && containsCallExpression(unary) {
		resolved, err := cp.inferExprType(unary.Expr, env)
		if err != nil {
			return ResolvedTypeRef{}, err
		}
		switch unary.Op {
		case "!":
			if !resolved.Equal(resolvedPrimitive(TypeBool)) {
				return ResolvedTypeRef{}, oneDiagnostic(cp.sources, CodeExpressionType, CategorySemantic, unary.Span, fmt.Sprintf("operator ! expects bool, got %s", resolved))
			}
			return resolvedPrimitive(TypeBool), nil
		case "-":
			if !isResolvedNumeric(resolved) {
				return ResolvedTypeRef{}, oneDiagnostic(cp.sources, CodeExpressionType, CategorySemantic, unary.Span, fmt.Sprintf("operator - expects int or float, got %s", resolved))
			}
			if strictNumeric {
				return ResolvedTypeRef{}, oneDiagnostic(cp.sources, CodeNumericSemantics, CategorySemantic, unary.Span, "numeric negation requires an explicitly declared checked Result return")
			}
			return resolved, nil
		default:
			return ResolvedTypeRef{}, oneDiagnostic(cp.sources, CodeExpressionType, CategorySemantic, unary.Span, fmt.Sprintf("unsupported unary operator %q", unary.Op))
		}
	}
	if binary, ok := expr.(*BinaryExpr); ok && containsCallExpression(binary) {
		left, err := cp.inferExprType(binary.Left, env)
		if err != nil {
			return ResolvedTypeRef{}, err
		}
		right, err := cp.inferExprType(binary.Right, env)
		if err != nil {
			return ResolvedTypeRef{}, err
		}
		if isOrdinalTextOrderingOperator(binary.Op) && left.Equal(resolvedPrimitive(TypeString)) && right.Equal(resolvedPrimitive(TypeString)) && cp.modules != nil && hasOrdinalTextOrderingSourceContract(cp.modules.LanguageContract()) {
			return resolvedPrimitive(TypeBool), nil
		}
		return inferBinaryTypeWithPolicy(cp.sources, binary.Span, binary.Op, left, right, strictNumeric)
	}
	return inferExprTypeWithPolicy(cp.sources, expr, env, strictNumeric)
}

func (cp *checkedProgram) resolveRecordField(record ResolvedTypeRef, name string, span Span) (ResolvedTypeRef, FieldDecl, int, error) {
	if !cp.isResolvedRecordType(record) {
		return ResolvedTypeRef{}, FieldDecl{}, 0, oneDiagnostic(cp.sources, CodeInvalidMember, CategorySemantic, span, fmt.Sprintf("cannot project field %q from non-record type %s", name, record))
	}
	entry, ok := cp.symbols.lookupIDEntry(record.Symbol)
	if !ok || entry.recordDecl == nil {
		return ResolvedTypeRef{}, FieldDecl{}, 0, oneDiagnostic(cp.sources, CodeInvalidMember, CategorySemantic, span, fmt.Sprintf("record field %q is inaccessible", name))
	}
	for position, field := range entry.recordDecl.Fields {
		if field.Name != name {
			continue
		}
		resolved, err := cp.resolveType(field.Type, RelatedSpan{Span: field.Span, Message: "record field declaration"})
		if err != nil {
			return ResolvedTypeRef{}, FieldDecl{}, 0, err
		}
		return resolved, field, position, nil
	}
	return ResolvedTypeRef{}, FieldDecl{}, 0, oneDiagnostic(cp.sources, CodeInvalidMember, CategorySemantic, span, fmt.Sprintf("record type %s has no field %q", record, name), RelatedSpan{Span: entry.recordDecl.Span, Message: "record declaration"})
}

func (cp *checkedProgram) inferMethodBodyType(method MethodDecl, env map[string]ResolvedTypeRef, declared ResolvedTypeRef) (ResolvedTypeRef, error) {
	expr := method.Body
	if _, ok := expr.(*ImmutableLocalExpr); ok {
		var contract LanguageContract
		if cp != nil && cp.modules != nil {
			contract = inheritedLanguageContract(cp.modules.LanguageContract())
		}
		if cp != nil && cp.modules != nil && ((contract == PipeLangLanguageContractV730 || contract == PipeLangLanguageContractV720 || contract == PipeLangLanguageContractV710 || contract == PipeLangLanguageContractV700 || contract == PipeLangLanguageContractV690 || contract == PipeLangLanguageContractV680 || contract == PipeLangLanguageContractV670 || contract == PipeLangLanguageContractV660 || contract == PipeLangLanguageContractV650 || contract == PipeLangLanguageContractV640 || contract == PipeLangLanguageContractV630 || contract == PipeLangLanguageContractV620 || contract == PipeLangLanguageContractV610 || contract == PipeLangLanguageContractV600 || contract == PipeLangLanguageContractV590 || contract == PipeLangLanguageContractV580) || contract == PipeLangLanguageContractV570 || contract == PipeLangLanguageContractV560 || contract == PipeLangLanguageContractV550 || contract == PipeLangLanguageContractV540) && isResolvedSourceArithmeticResult(contract, declared) && containsPropagationExpression(expr) {
			return cp.inferCheckedArithmeticPropagationBlock(expr, env, declared)
		}
		return cp.inferExprType(expr, env)
	}
	if cp.modules != nil && hasConditionalSourceContract(cp.modules.LanguageContract()) && containsConditionalExpression(expr) {
		return cp.inferExprType(expr, env)
	}
	if cp.modules != nil && hasPureCallSourceContract(cp.modules.LanguageContract()) && containsCallExpression(expr) {
		return cp.inferExprType(expr, env)
	}
	if cp.modules != nil && hasPropagationSourceContract(cp.modules.LanguageContract()) {
		var propagated *PropagateExpr
		if some, ok := expr.(*OptionalSomeExpr); ok {
			propagated, _ = some.Value.(*PropagateExpr)
		}
		if okExpr, ok := expr.(*ResultOKExpr); ok {
			propagated, _ = okExpr.Value.(*PropagateExpr)
		}
		if propagated != nil {
			identifier, direct := propagated.Value.(*IdentExpr)
			if !direct {
				return ResolvedTypeRef{}, oneDiagnostic(cp.sources, CodePropagation, CategorySemantic, propagated.Value.SourceSpan(), "propagate requires one direct carrier parameter")
			}
			carrier, err := cp.inferExprType(propagated.Value, env)
			if err != nil {
				return ResolvedTypeRef{}, err
			}
			if _, exists := env[identifier.Name]; exists && (containsResolvedOptional(declared) || containsResolvedResult(declared)) && carrier.Equal(declared) {
				return declared, nil
			}
			return ResolvedTypeRef{}, oneDiagnostic(cp.sources, CodePropagation, CategorySemantic, propagated.Span, "propagate operand and enclosing Optional or bounded Result carrier must be identical")
		}
	}
	if inferred, handled, err := cp.inferBoundedResultMethodBodyType(method, env, declared); handled {
		return inferred, err
	}
	if inferred, handled, err := cp.inferRecordListMethodBodyType(method, env, declared); handled {
		return inferred, err
	}
	if inferred, handled, err := cp.inferOptionalMethodBodyType(method, env, declared); handled {
		return inferred, err
	}
	if cp == nil || cp.modules == nil || !hasArithmeticResultSourceContract(cp.modules.LanguageContract()) || !isResolvedSourceArithmeticResult(cp.modules.LanguageContract(), declared) {
		return cp.inferNonResultMethodBodyType(method, env, declared)
	}
	contract := inheritedLanguageContract(cp.modules.LanguageContract())
	if hasResultTransportSourceContract(contract) {
		hasTransportInput := false
		for _, resolved := range env {
			hasTransportInput = hasTransportInput || resolved.Equal(declared)
		}
		if hasTransportInput {
			if identifier, ok := expr.(*IdentExpr); ok {
				if resolved, found := env[identifier.Name]; found && resolved.Equal(declared) {
					return declared, nil
				}
			}
			return ResolvedTypeRef{}, oneDiagnostic(cp.sources, CodeExpressionType, CategorySemantic, expr.SourceSpan(), fmt.Sprintf("%s Result transport requires the sole Result parameter as the complete method body", contract))
		}
	}
	if isResolvedFloatArithmeticResult(declared) {
		binary, ok := expr.(*BinaryExpr)
		if !ok || binary.Op != "/" {
			return ResolvedTypeRef{}, oneDiagnostic(cp.sources, CodeNumericSemantics, CategorySemantic, expr.SourceSpan(), fmt.Sprintf("%s admits only one direct checked binary64 division as the complete Result<float,ArithmeticError> method body", contract))
		}
		left, err := inferExprTypeWithPolicy(cp.sources, binary.Left, env, true)
		if err != nil {
			return ResolvedTypeRef{}, err
		}
		right, err := inferExprTypeWithPolicy(cp.sources, binary.Right, env, true)
		if err != nil {
			return ResolvedTypeRef{}, err
		}
		binary64 := resolvedPrimitive(TypeFloat)
		if !left.Equal(binary64) || !right.Equal(binary64) {
			return ResolvedTypeRef{}, oneDiagnostic(cp.sources, CodeNumericSemantics, CategorySemantic, binary.Span, fmt.Sprintf("checked binary64 division requires float and float, got %s and %s", left, right))
		}
		return resolvedArithmeticResult(binary64), nil
	}
	if unary, unaryOK := expr.(*UnaryExpr); unaryOK && ((contract == PipeLangLanguageContractV730 || contract == PipeLangLanguageContractV720 || contract == PipeLangLanguageContractV710 || contract == PipeLangLanguageContractV700 || contract == PipeLangLanguageContractV690 || contract == PipeLangLanguageContractV680 || contract == PipeLangLanguageContractV670 || contract == PipeLangLanguageContractV660 || contract == PipeLangLanguageContractV650 || contract == PipeLangLanguageContractV640 || contract == PipeLangLanguageContractV630 || contract == PipeLangLanguageContractV620 || contract == PipeLangLanguageContractV610 || contract == PipeLangLanguageContractV600 || contract == PipeLangLanguageContractV590 || contract == PipeLangLanguageContractV580) || contract == PipeLangLanguageContractV570 || contract == PipeLangLanguageContractV560 || contract == PipeLangLanguageContractV550 || contract == PipeLangLanguageContractV540 || contract == PipeLangLanguageContractV530 || contract == PipeLangLanguageContractV520 || contract == PipeLangLanguageContractV510 || contract == PipeLangLanguageContractV500 || contract == PipeLangLanguageContractV490 || contract == PipeLangLanguageContractV310 || contract == PipeLangLanguageContractV320 || contract == PipeLangLanguageContractV330 || contract == PipeLangLanguageContractV340 || contract == PipeLangLanguageContractV350 || contract == PipeLangLanguageContractV360 || contract == PipeLangLanguageContractV370 || contract == PipeLangLanguageContractV380 || contract == PipeLangLanguageContractV390 || contract == PipeLangLanguageContractV400 || contract == PipeLangLanguageContractV410 || contract == PipeLangLanguageContractV420 || contract == PipeLangLanguageContractV430 || contract == PipeLangLanguageContractV440 || contract == PipeLangLanguageContractV450 || contract == PipeLangLanguageContractV460 || contract == PipeLangLanguageContractV470 || contract == PipeLangLanguageContractV480) && unary.Op == "-" {
		operand, err := inferExprTypeWithPolicy(cp.sources, unary.Expr, env, true)
		if err != nil {
			return ResolvedTypeRef{}, err
		}
		integer := resolvedPrimitive(TypeInt)
		if !operand.Equal(integer) {
			return ResolvedTypeRef{}, oneDiagnostic(cp.sources, CodeNumericSemantics, CategorySemantic, unary.Span, fmt.Sprintf("checked integer negation requires int, got %s", operand))
		}
		return resolvedArithmeticResult(integer), nil
	}
	if unary, unaryOK := expr.(*UnaryExpr); unaryOK && (contract == PipeLangLanguageContractV050 || contract == PipeLangLanguageContractV060 || contract == PipeLangLanguageContractV070 || contract == PipeLangLanguageContractV080 || contract == PipeLangLanguageContractV090 || contract == PipeLangLanguageContractV100 || contract == PipeLangLanguageContractV110 || contract == PipeLangLanguageContractV120 || contract == PipeLangLanguageContractV130 || contract == PipeLangLanguageContractV140 || contract == PipeLangLanguageContractV150 || contract == PipeLangLanguageContractV160 || contract == PipeLangLanguageContractV170 || contract == PipeLangLanguageContractV180 || contract == PipeLangLanguageContractV190 || contract == PipeLangLanguageContractV200 || contract == PipeLangLanguageContractV210 || contract == PipeLangLanguageContractV220 || contract == PipeLangLanguageContractV230 || contract == PipeLangLanguageContractV240 || contract == PipeLangLanguageContractV260 || contract == PipeLangLanguageContractV250 || contract == PipeLangLanguageContractV270 || contract == PipeLangLanguageContractV280 || contract == PipeLangLanguageContractV290 || contract == PipeLangLanguageContractV300) && unary.Op == "-" {
		operand, err := inferExprTypeWithPolicy(cp.sources, unary.Expr, env, true)
		if err != nil {
			return ResolvedTypeRef{}, err
		}
		integer := resolvedPrimitive(TypeInt)
		if !operand.Equal(integer) {
			return ResolvedTypeRef{}, oneDiagnostic(cp.sources, CodeNumericSemantics, CategorySemantic, unary.Span, fmt.Sprintf("checked integer negation requires int, got %s", operand))
		}
		return resolvedArithmeticResult(integer), nil
	}
	binary, ok := expr.(*BinaryExpr)
	operatorAccepted := ok && binary.Op == "+"
	if (contract == PipeLangLanguageContractV730 || contract == PipeLangLanguageContractV720 || contract == PipeLangLanguageContractV710 || contract == PipeLangLanguageContractV700 || contract == PipeLangLanguageContractV690 || contract == PipeLangLanguageContractV680 || contract == PipeLangLanguageContractV670 || contract == PipeLangLanguageContractV660 || contract == PipeLangLanguageContractV650 || contract == PipeLangLanguageContractV640 || contract == PipeLangLanguageContractV630 || contract == PipeLangLanguageContractV620 || contract == PipeLangLanguageContractV610 || contract == PipeLangLanguageContractV600 || contract == PipeLangLanguageContractV590 || contract == PipeLangLanguageContractV580) || contract == PipeLangLanguageContractV570 || contract == PipeLangLanguageContractV560 || contract == PipeLangLanguageContractV550 || contract == PipeLangLanguageContractV540 || contract == PipeLangLanguageContractV530 || contract == PipeLangLanguageContractV520 || contract == PipeLangLanguageContractV510 || contract == PipeLangLanguageContractV500 || contract == PipeLangLanguageContractV490 || contract == PipeLangLanguageContractV310 || contract == PipeLangLanguageContractV320 || contract == PipeLangLanguageContractV330 || contract == PipeLangLanguageContractV340 || contract == PipeLangLanguageContractV350 || contract == PipeLangLanguageContractV360 || contract == PipeLangLanguageContractV370 || contract == PipeLangLanguageContractV380 || contract == PipeLangLanguageContractV390 || contract == PipeLangLanguageContractV400 || contract == PipeLangLanguageContractV410 || contract == PipeLangLanguageContractV420 || contract == PipeLangLanguageContractV430 || contract == PipeLangLanguageContractV440 || contract == PipeLangLanguageContractV450 || contract == PipeLangLanguageContractV460 || contract == PipeLangLanguageContractV470 || contract == PipeLangLanguageContractV480 {
		operatorAccepted = ok && (binary.Op == "+" || binary.Op == "-" || binary.Op == "*")
	}
	if contract == PipeLangLanguageContractV040 || contract == PipeLangLanguageContractV050 || contract == PipeLangLanguageContractV060 || contract == PipeLangLanguageContractV070 || contract == PipeLangLanguageContractV080 || contract == PipeLangLanguageContractV090 || contract == PipeLangLanguageContractV100 || contract == PipeLangLanguageContractV110 || contract == PipeLangLanguageContractV120 || contract == PipeLangLanguageContractV130 || contract == PipeLangLanguageContractV140 || contract == PipeLangLanguageContractV150 || contract == PipeLangLanguageContractV160 || contract == PipeLangLanguageContractV170 || contract == PipeLangLanguageContractV180 || contract == PipeLangLanguageContractV190 || contract == PipeLangLanguageContractV200 || contract == PipeLangLanguageContractV210 || contract == PipeLangLanguageContractV220 || contract == PipeLangLanguageContractV230 || contract == PipeLangLanguageContractV240 || contract == PipeLangLanguageContractV260 || contract == PipeLangLanguageContractV250 || contract == PipeLangLanguageContractV270 || contract == PipeLangLanguageContractV280 || contract == PipeLangLanguageContractV290 || contract == PipeLangLanguageContractV300 {
		operatorAccepted = ok && (binary.Op == "+" || binary.Op == "-" || binary.Op == "*")
	} else if contract == PipeLangLanguageContractV030 {
		operatorAccepted = ok && (binary.Op == "+" || binary.Op == "-")
	}
	if !operatorAccepted {
		span := expr.SourceSpan()
		return ResolvedTypeRef{}, oneDiagnostic(cp.sources, CodeNumericSemantics, CategorySemantic, span, fmt.Sprintf("%s admits only one direct checked integer %s as the complete Result<int,ArithmeticError> method body", contract, arithmeticSourceOperators(contract)))
	}
	left, err := inferExprTypeWithPolicy(cp.sources, binary.Left, env, true)
	if err != nil {
		return ResolvedTypeRef{}, err
	}
	right, err := inferExprTypeWithPolicy(cp.sources, binary.Right, env, true)
	if err != nil {
		return ResolvedTypeRef{}, err
	}
	integer := resolvedPrimitive(TypeInt)
	if !left.Equal(integer) || !right.Equal(integer) {
		return ResolvedTypeRef{}, oneDiagnostic(cp.sources, CodeNumericSemantics, CategorySemantic, binary.Span, fmt.Sprintf("checked integer operator %q requires int and int, got %s and %s", binary.Op, left, right))
	}
	return resolvedArithmeticResult(integer), nil
}

func (cp *checkedProgram) inferCheckedArithmeticPropagationBlock(expr Expr, env map[string]ResolvedTypeRef, declared ResolvedTypeRef) (ResolvedTypeRef, error) {
	local, ok := expr.(*ImmutableLocalExpr)
	if !ok {
		continuationEnv := make(map[string]ResolvedTypeRef, len(env))
		for name, resolved := range env {
			if !resolved.Equal(declared) {
				continuationEnv[name] = resolved
			}
		}
		return cp.inferMethodBodyType(MethodDecl{Body: expr}, continuationEnv, declared)
	}
	if _, exists := env[local.Name]; exists {
		return ResolvedTypeRef{}, oneDiagnostic(cp.sources, CodeExpressionType, CategorySemantic, local.NameSpan, fmt.Sprintf("immutable local %q shadows an existing binding", local.Name))
	}
	localType, err := cp.resolveType(local.Type, RelatedSpan{Span: local.Span, Message: "immutable local declaration"})
	if err != nil {
		return ResolvedTypeRef{}, err
	}
	var initialized ResolvedTypeRef
	contract := inheritedLanguageContract(cp.modules.LanguageContract())
	if isResolvedSourceArithmeticResult(contract, localType) {
		initializerEnv := env
		if (contract == PipeLangLanguageContractV730 || contract == PipeLangLanguageContractV720 || contract == PipeLangLanguageContractV710 || contract == PipeLangLanguageContractV700 || contract == PipeLangLanguageContractV690 || contract == PipeLangLanguageContractV680 || contract == PipeLangLanguageContractV670 || contract == PipeLangLanguageContractV660 || contract == PipeLangLanguageContractV650 || contract == PipeLangLanguageContractV640 || contract == PipeLangLanguageContractV630 || contract == PipeLangLanguageContractV620 || contract == PipeLangLanguageContractV610 || contract == PipeLangLanguageContractV600 || contract == PipeLangLanguageContractV590 || contract == PipeLangLanguageContractV580) || contract == PipeLangLanguageContractV570 {
			initializerEnv = make(map[string]ResolvedTypeRef, len(env))
			for name, resolved := range env {
				if !resolved.Equal(declared) {
					initializerEnv[name] = resolved
				}
			}
		}
		initialized, err = cp.inferMethodBodyType(MethodDecl{Body: local.Initializer, ReturnType: local.Type}, initializerEnv, localType)
	} else {
		initialized, err = cp.inferExprType(local.Initializer, env)
	}
	if err != nil {
		return ResolvedTypeRef{}, err
	}
	if !initialized.Equal(localType) {
		return ResolvedTypeRef{}, oneDiagnostic(cp.sources, CodeExpressionType, CategorySemantic, local.Initializer.SourceSpan(), fmt.Sprintf("immutable local %s initializer has type %s, declared %s", local.Name, initialized, localType), RelatedSpan{Span: local.Type.Span, Message: "declared local type"})
	}
	scoped := make(map[string]ResolvedTypeRef, len(env)+1)
	for name, resolved := range env {
		scoped[name] = resolved
	}
	scoped[local.Name] = localType
	return cp.inferCheckedArithmeticPropagationBlock(local.Return, scoped, declared)
}

func (cp *checkedProgram) inferBoundedResultMethodBodyType(method MethodDecl, env map[string]ResolvedTypeRef, declared ResolvedTypeRef) (ResolvedTypeRef, bool, error) {
	parameterTypes := make([]ResolvedTypeRef, 0, len(method.Params))
	contract := cp.modules.LanguageContract()
	hasBoundedResult := isResolvedBoundedValueResult(contract, declared) || containsResultExpression(method.Body)
	for _, parameter := range method.Params {
		resolved, err := cp.resolveType(parameter.Type)
		if err != nil {
			return ResolvedTypeRef{}, true, err
		}
		parameterTypes = append(parameterTypes, resolved)
		hasBoundedResult = hasBoundedResult || isResolvedBoundedValueResult(contract, resolved)
	}
	if !hasBoundedResult {
		return ResolvedTypeRef{}, false, nil
	}
	_, matchBody := method.Body.(*MatchExpr)
	matchSignature := matchBody && hasMatchSourceContract(contract) && len(parameterTypes) == 1 && isResolvedBoundedValueResult(contract, parameterTypes[0])
	dependentSignature := cp.dependentCarrierHelperSignature(method, declared, parameterTypes)
	if !hasSnapshotResultSourceContract(contract) || (!cp.boundedResultSignatureMatches(declared, parameterTypes) && !matchSignature && !dependentSignature) {
		return ResolvedTypeRef{}, true, oneDiagnostic(cp.sources, CodeInvalidType, CategorySemantic, method.Span, fmt.Sprintf("%s bounded Result requires one exact snapshot or text signature", contract))
	}
	if dependentSignature {
		actual, err := cp.inferExprType(method.Body, env)
		if err != nil {
			return ResolvedTypeRef{}, true, err
		}
		if actual.Equal(declared) {
			return declared, true, nil
		}
	}
	directParameter := func(expression Expr, position int) bool {
		identifier, ok := expression.(*IdentExpr)
		return ok && position < len(method.Params) && identifier.Name == method.Params[position].Name
	}
	switch body := method.Body.(type) {
	case *MatchExpr:
		if matchSignature && directParameter(body.Value, 0) {
			actual, err := cp.inferExprType(body, env)
			if err != nil {
				return ResolvedTypeRef{}, true, err
			}
			if actual.Equal(declared) {
				return declared, true, nil
			}
		}
	case *ResultOKExpr:
		inferred, err := cp.inferExprType(body, env)
		if err == nil && inferred.Equal(declared) && len(parameterTypes) == 1 && parameterTypes[0].Equal(declared.Arguments[0]) && directParameter(body.Value, 0) {
			return declared, true, nil
		}
	case *ResultErrExpr:
		inferred, err := cp.inferExprType(body, env)
		if err == nil && inferred.Equal(declared) && len(parameterTypes) == 1 && parameterTypes[0].Equal(declared.Arguments[1]) && directParameter(body.Error, 0) {
			return declared, true, nil
		}
	case *IdentExpr:
		if isResolvedBoundedValueResult(contract, declared) && len(parameterTypes) == 1 && parameterTypes[0].Equal(declared) && body.Name == method.Params[0].Name {
			return declared, true, nil
		}
	case *ResultIsOKExpr:
		if declared.Equal(resolvedPrimitive(TypeBool)) && len(parameterTypes) == 1 && isResolvedBoundedValueResult(contract, parameterTypes[0]) && directParameter(body.Value, 0) {
			return declared, true, nil
		}
	case *ResultSuccessOrExpr:
		if len(parameterTypes) == 2 && isResolvedBoundedValueResult(contract, parameterTypes[0]) && parameterTypes[0].Arguments[0].Equal(declared) && parameterTypes[1].Equal(declared) && directParameter(body.Value, 0) && directParameter(body.Fallback, 1) {
			return declared, true, nil
		}
	case *ResultFailureOrExpr:
		if declared.Equal(resolvedPrimitive(TypeString)) && len(parameterTypes) == 2 && isResolvedBoundedValueResult(contract, parameterTypes[0]) && parameterTypes[0].Arguments[1].Equal(declared) && parameterTypes[1].Equal(declared) && directParameter(body.Value, 0) && directParameter(body.Fallback, 1) {
			return declared, true, nil
		}
	}
	return ResolvedTypeRef{}, true, oneDiagnostic(cp.sources, CodeExpressionType, CategorySemantic, method.Body.SourceSpan(), fmt.Sprintf("%s bounded Result requires one complete direct ok, err, identity, is_ok, success_or, or failure_or method body", contract))
}

func (cp *checkedProgram) inferRecordListMethodBodyType(method MethodDecl, env map[string]ResolvedTypeRef, declared ResolvedTypeRef) (ResolvedTypeRef, bool, error) {
	parameterTypes := make([]ResolvedTypeRef, 0, len(method.Params))
	hasList := isResolvedRecordList(declared) || containsListExpression(method.Body)
	for _, parameter := range method.Params {
		resolved, err := cp.resolveType(parameter.Type)
		if err != nil {
			return ResolvedTypeRef{}, true, err
		}
		parameterTypes = append(parameterTypes, resolved)
		hasList = hasList || containsResolvedRecordList(resolved)
	}
	if !hasList {
		return ResolvedTypeRef{}, false, nil
	}
	contract := inheritedLanguageContract(cp.modules.LanguageContract())
	if !hasPrimitiveRecordListSourceContract(contract) {
		return ResolvedTypeRef{}, true, oneDiagnostic(cp.sources, CodeInvalidType, CategorySemantic, method.Span, fmt.Sprintf("record-list methods require language contract %q", PipeLangLanguageContractV150))
	}
	directParameter := func(expression Expr, position int) bool {
		identifier, ok := expression.(*IdentExpr)
		return ok && position < len(method.Params) && identifier.Name == method.Params[position].Name
	}
	if isResolvedRecordList(declared) {
		switch body := method.Body.(type) {
		case *ListEmptyExpr:
			element, err := cp.resolveType(body.ElementType)
			if err != nil {
				return ResolvedTypeRef{}, true, err
			}
			if len(parameterTypes) == 0 && element.Equal(declared.Arguments[0]) {
				return declared, true, nil
			}
		case *ListSingletonExpr:
			if len(parameterTypes) == 1 && parameterTypes[0].Equal(declared.Arguments[0]) && directParameter(body.Value, 0) {
				return declared, true, nil
			}
		case *IdentExpr:
			if len(parameterTypes) == 1 && parameterTypes[0].Equal(declared) && body.Name == method.Params[0].Name {
				return declared, true, nil
			}
		case *ListAppendExpr:
			if hasPrimitiveRecordListAppendSourceContract(contract) && len(parameterTypes) == 2 && parameterTypes[0].Equal(declared) && parameterTypes[1].Equal(declared.Arguments[0]) && directParameter(body.Values, 0) && directParameter(body.Value, 1) {
				return declared, true, nil
			}
		}
	}
	if hasPrimitiveRecordListCountSourceContract(contract) && declared.Equal(resolvedPrimitive(TypeInt)) {
		if body, ok := method.Body.(*ListCountExpr); ok && len(parameterTypes) == 1 && isResolvedRecordList(parameterTypes[0]) && directParameter(body.Value, 0) {
			return declared, true, nil
		}
	}
	if hasPrimitiveRecordListAtSourceContract(contract) && isResolvedRecordOptional(declared) && cp.isResolvedRecordType(declared.Arguments[0]) {
		if body, ok := method.Body.(*ListAtExpr); ok && len(parameterTypes) == 2 && isResolvedRecordList(parameterTypes[0]) && parameterTypes[0].Arguments[0].Equal(declared.Arguments[0]) && parameterTypes[1].Equal(resolvedPrimitive(TypeInt)) && directParameter(body.Values, 0) && directParameter(body.Index, 1) {
			return declared, true, nil
		}
	}
	if hasPrimitiveRecordListFindByTextSourceContract(contract) && isResolvedRecordOptional(declared) && cp.isResolvedRecordType(declared.Arguments[0]) {
		if body, ok := method.Body.(*ListFindByTextExpr); ok && len(parameterTypes) == 2 && isResolvedRecordList(parameterTypes[0]) && parameterTypes[0].Arguments[0].Equal(declared.Arguments[0]) && parameterTypes[1].Equal(resolvedPrimitive(TypeString)) {
			if _, _, err := cp.resolveListFindByTextSelector(body, parameterTypes[0]); err != nil {
				return ResolvedTypeRef{}, true, err
			}
			if directParameter(body.Values, 0) && directParameter(body.Key, 1) {
				return declared, true, nil
			}
		}
	}
	if hasPrimitiveRecordListFilterByTextSourceContract(contract) && isResolvedRecordList(declared) && cp.isResolvedRecordType(declared.Arguments[0]) {
		if body, ok := method.Body.(*ListFilterByTextExpr); ok && len(parameterTypes) == 2 && parameterTypes[0].Equal(declared) && parameterTypes[1].Equal(resolvedPrimitive(TypeString)) {
			if _, _, err := cp.resolveListFilterByTextSelector(body, parameterTypes[0]); err != nil {
				return ResolvedTypeRef{}, true, err
			}
			if directParameter(body.Values, 0) && directParameter(body.Key, 1) {
				return declared, true, nil
			}
		}
	}
	if hasNamedRecordPredicateSourceContract(contract) && isResolvedRecordList(declared) && cp.isResolvedRecordType(declared.Arguments[0]) {
		if body, ok := method.Body.(*ListFilterPredicateExpr); ok && len(parameterTypes) >= 1 && parameterTypes[0].Equal(declared) && len(body.Arguments) == len(parameterTypes)-1 {
			predicate, err := cp.resolveNamedRecordPredicate(method, body.Predicate, body.PredicateSpan)
			if err != nil {
				return ResolvedTypeRef{}, true, err
			}
			valid := normalizeVisibility(predicate.Visibility) == VisibilityPublic && len(predicate.Params) == len(parameterTypes) && directParameter(body.Values, 0)
			predicateReturn, err := cp.resolveType(predicate.ReturnType)
			if err != nil {
				return ResolvedTypeRef{}, true, err
			}
			valid = valid && predicateReturn.Equal(resolvedPrimitive(TypeBool))
			for position, parameter := range predicate.Params {
				resolved, err := cp.resolveType(parameter.Type)
				if err != nil {
					return ResolvedTypeRef{}, true, err
				}
				expected := declared.Arguments[0]
				if position > 0 {
					expected = parameterTypes[position]
				}
				valid = valid && resolved.Equal(expected)
				if position > 0 {
					valid = valid && directParameter(body.Arguments[position-1], position)
				}
			}
			if valid {
				return declared, true, nil
			}
			return ResolvedTypeRef{}, true, oneDiagnostic(cp.sources, CodeExpressionType, CategorySemantic, body.Span, "filter requires a same-class public bool predicate whose record and primitive parameters exactly match the direct filter operands")
		}
	}
	if hasPrimitiveRecordListFilterContainsCaseFoldedSourceContract(contract) && isResolvedRecordList(declared) && cp.isResolvedRecordType(declared.Arguments[0]) {
		if body, ok := method.Body.(*ListFilterContainsCaseFoldedExpr); ok && len(parameterTypes) == 2 && parameterTypes[0].Equal(declared) && parameterTypes[1].Equal(resolvedPrimitive(TypeString)) {
			if _, _, err := cp.resolveListFilterContainsCaseFoldedSelector(body, parameterTypes[0]); err != nil {
				return ResolvedTypeRef{}, true, err
			}
			if directParameter(body.Values, 0) && directParameter(body.Query, 1) {
				return declared, true, nil
			}
		}
	}
	if hasPrimitiveRecordListFilterJoinedContainsCaseFoldedSourceContract(contract) && isResolvedRecordList(declared) && cp.isResolvedRecordType(declared.Arguments[0]) {
		if body, ok := method.Body.(*ListFilterJoinedContainsCaseFoldedExpr); ok && len(parameterTypes) == 2 && parameterTypes[0].Equal(declared) && parameterTypes[1].Equal(resolvedPrimitive(TypeString)) {
			if _, _, err := cp.resolveListFilterJoinedContainsCaseFoldedSelectors(body, parameterTypes[0]); err != nil {
				return ResolvedTypeRef{}, true, err
			}
			if directParameter(body.Values, 0) && directParameter(body.Query, 1) {
				return declared, true, nil
			}
		}
	}
	if hasPrimitiveRecordListSortByOrdinalSourceContract(contract) && isResolvedRecordList(declared) && cp.isResolvedRecordType(declared.Arguments[0]) {
		if body, ok := method.Body.(*ListSortByOrdinalExpr); ok && len(parameterTypes) == 1 && parameterTypes[0].Equal(declared) {
			if _, _, err := cp.resolveListSortByOrdinalSelector(body, parameterTypes[0]); err != nil {
				return ResolvedTypeRef{}, true, err
			}
			if directParameter(body.Values, 0) {
				return declared, true, nil
			}
		}
		if body, ok := method.Body.(*ListSortByOrdinalsExpr); ok && len(parameterTypes) == 1 && parameterTypes[0].Equal(declared) {
			if _, _, err := cp.resolveListSortByOrdinalsSelectors(body, parameterTypes[0]); err != nil {
				return ResolvedTypeRef{}, true, err
			}
			if directParameter(body.Values, 0) {
				return declared, true, nil
			}
		}
		if body, ok := method.Body.(*ListSortByOrdinalDirectionsExpr); ok && ((contract == PipeLangLanguageContractV730 || contract == PipeLangLanguageContractV720 || contract == PipeLangLanguageContractV710 || contract == PipeLangLanguageContractV700 || contract == PipeLangLanguageContractV690 || contract == PipeLangLanguageContractV680 || contract == PipeLangLanguageContractV670 || contract == PipeLangLanguageContractV660 || contract == PipeLangLanguageContractV650 || contract == PipeLangLanguageContractV640 || contract == PipeLangLanguageContractV630 || contract == PipeLangLanguageContractV620 || contract == PipeLangLanguageContractV610 || contract == PipeLangLanguageContractV600 || contract == PipeLangLanguageContractV590 || contract == PipeLangLanguageContractV580) || contract == PipeLangLanguageContractV570 || contract == PipeLangLanguageContractV560 || contract == PipeLangLanguageContractV550 || contract == PipeLangLanguageContractV540 || contract == PipeLangLanguageContractV530 || contract == PipeLangLanguageContractV520 || contract == PipeLangLanguageContractV510 || contract == PipeLangLanguageContractV500 || contract == PipeLangLanguageContractV490 || contract == PipeLangLanguageContractV320 || contract == PipeLangLanguageContractV330 || contract == PipeLangLanguageContractV340 || contract == PipeLangLanguageContractV350 || contract == PipeLangLanguageContractV360 || contract == PipeLangLanguageContractV370 || contract == PipeLangLanguageContractV380 || contract == PipeLangLanguageContractV390 || contract == PipeLangLanguageContractV400 || contract == PipeLangLanguageContractV410 || contract == PipeLangLanguageContractV420 || contract == PipeLangLanguageContractV430 || contract == PipeLangLanguageContractV440 || contract == PipeLangLanguageContractV450 || contract == PipeLangLanguageContractV460 || contract == PipeLangLanguageContractV470 || contract == PipeLangLanguageContractV480) && len(parameterTypes) == 1 && parameterTypes[0].Equal(declared) {
			if directParameter(body.Values, 0) {
				return declared, true, nil
			}
		}
	}
	forms := "empty_list, singleton list, or identity-transport"
	if hasPrimitiveRecordListCountSourceContract(contract) {
		forms += ", or count"
	}
	if hasPrimitiveRecordListAppendSourceContract(contract) {
		forms += ", or append"
	}
	if hasPrimitiveRecordListAtSourceContract(contract) {
		forms += ", or at"
	}
	if hasPrimitiveRecordListFindByTextSourceContract(contract) {
		forms += ", or find_by"
	}
	if hasPrimitiveRecordListFilterByTextSourceContract(contract) {
		forms += ", or filter_by"
	}
	if hasNamedRecordPredicateSourceContract(contract) {
		forms += ", or filter"
	}
	if hasPrimitiveRecordListFilterContainsCaseFoldedSourceContract(contract) {
		forms += ", or filter_contains_casefolded"
	}
	if hasPrimitiveRecordListFilterJoinedContainsCaseFoldedSourceContract(contract) {
		forms += ", or filter_joined_contains_casefolded"
	}
	if hasPrimitiveRecordListSortByOrdinalSourceContract(contract) {
		forms += ", or sort_by_ordinal"
	}
	return ResolvedTypeRef{}, true, oneDiagnostic(cp.sources, CodeExpressionType, CategorySemantic, method.Body.SourceSpan(), fmt.Sprintf("%s record-list methods require exact %s bodies", contract, forms))
}

func (cp *checkedProgram) resolveNamedRecordPredicate(caller MethodDecl, name string, span Span) (MethodDecl, error) {
	for _, class := range cp.program.Classes {
		ownsCaller := false
		for _, method := range class.Methods {
			if method.Name == caller.Name && method.Span == caller.Span {
				ownsCaller = true
				break
			}
		}
		if !ownsCaller {
			continue
		}
		for _, method := range class.Methods {
			if method.Name == name {
				return method, nil
			}
		}
		return MethodDecl{}, oneDiagnostic(cp.sources, CodeInvalidMember, CategorySemantic, span, fmt.Sprintf("class %s has no predicate method %q", class.Name, name), RelatedSpan{Span: class.Span, Message: "owning class declaration"})
	}
	return MethodDecl{}, oneDiagnostic(cp.sources, CodeInvalidMember, CategorySemantic, span, fmt.Sprintf("predicate %q has no owning class", name))
}

func (cp *checkedProgram) resolveListFindByTextSelector(expression *ListFindByTextExpr, values ResolvedTypeRef) (FieldDecl, int, error) {
	record, err := cp.resolveType(expression.RecordType)
	if err != nil {
		return FieldDecl{}, 0, err
	}
	if !isResolvedRecordList(values) || !record.Equal(values.Arguments[0]) {
		return FieldDecl{}, 0, oneDiagnostic(cp.sources, CodeInvalidType, CategorySemantic, expression.RecordType.Span, fmt.Sprintf("find_by selector type %s must match list element type %s", record, values.Arguments[0]))
	}
	fieldType, field, position, err := cp.resolveRecordField(record, expression.Field, expression.FieldSpan)
	if err != nil {
		return FieldDecl{}, 0, err
	}
	if normalizeVisibility(field.Visibility) != VisibilityPublic || !fieldType.Equal(resolvedPrimitive(TypeString)) {
		return FieldDecl{}, 0, oneDiagnostic(cp.sources, CodeInvalidType, CategorySemantic, expression.FieldSpan, fmt.Sprintf("find_by selector %s.%s must identify one public string field", expression.RecordType.Name, expression.Field), RelatedSpan{Span: field.Span, Message: "record field declaration"})
	}
	return field, position, nil
}

func (cp *checkedProgram) resolveListFilterByTextSelector(expression *ListFilterByTextExpr, values ResolvedTypeRef) (FieldDecl, int, error) {
	record, err := cp.resolveType(expression.RecordType)
	if err != nil {
		return FieldDecl{}, 0, err
	}
	if !isResolvedRecordList(values) || !record.Equal(values.Arguments[0]) {
		return FieldDecl{}, 0, oneDiagnostic(cp.sources, CodeInvalidType, CategorySemantic, expression.RecordType.Span, fmt.Sprintf("filter_by selector type %s must match list element type %s", record, values.Arguments[0]))
	}
	fieldType, field, position, err := cp.resolveRecordField(record, expression.Field, expression.FieldSpan)
	if err != nil {
		return FieldDecl{}, 0, err
	}
	if normalizeVisibility(field.Visibility) != VisibilityPublic || !fieldType.Equal(resolvedPrimitive(TypeString)) {
		return FieldDecl{}, 0, oneDiagnostic(cp.sources, CodeInvalidType, CategorySemantic, expression.FieldSpan, fmt.Sprintf("filter_by selector %s.%s must identify one public string field", expression.RecordType.Name, expression.Field), RelatedSpan{Span: field.Span, Message: "record field declaration"})
	}
	return field, position, nil
}

func (cp *checkedProgram) resolveListFilterContainsCaseFoldedSelector(expression *ListFilterContainsCaseFoldedExpr, values ResolvedTypeRef) (FieldDecl, int, error) {
	record, err := cp.resolveType(expression.RecordType)
	if err != nil {
		return FieldDecl{}, 0, err
	}
	if !isResolvedRecordList(values) || !record.Equal(values.Arguments[0]) {
		return FieldDecl{}, 0, oneDiagnostic(cp.sources, CodeInvalidType, CategorySemantic, expression.RecordType.Span, fmt.Sprintf("filter_contains_casefolded selector type %s must match list element type %s", record, values.Arguments[0]))
	}
	fieldType, field, position, err := cp.resolveRecordField(record, expression.Field, expression.FieldSpan)
	if err != nil {
		return FieldDecl{}, 0, err
	}
	if normalizeVisibility(field.Visibility) != VisibilityPublic || !fieldType.Equal(resolvedPrimitive(TypeString)) {
		return FieldDecl{}, 0, oneDiagnostic(cp.sources, CodeInvalidType, CategorySemantic, expression.FieldSpan, fmt.Sprintf("filter_contains_casefolded selector %s.%s must identify one public string field", expression.RecordType.Name, expression.Field), RelatedSpan{Span: field.Span, Message: "record field declaration"})
	}
	return field, position, nil
}

func (cp *checkedProgram) resolveListFilterJoinedContainsCaseFoldedSelectors(expression *ListFilterJoinedContainsCaseFoldedExpr, values ResolvedTypeRef) ([]FieldDecl, []int, error) {
	if (cp.modules.LanguageContract() == PipeLangLanguageContractV290 || cp.modules.LanguageContract() == PipeLangLanguageContractV300 || cp.modules.LanguageContract() == PipeLangLanguageContractV310) && len(expression.Selectors) < 2 {
		return nil, nil, oneDiagnostic(cp.sources, CodeInvalidType, CategorySemantic, expression.Span, fmt.Sprintf("filter_joined_contains_casefolded requires at least two distinct public string field selectors, got %d", len(expression.Selectors)))
	}
	if cp.modules.LanguageContract() != PipeLangLanguageContractV290 && cp.modules.LanguageContract() != PipeLangLanguageContractV300 && cp.modules.LanguageContract() != PipeLangLanguageContractV310 && len(expression.Selectors) != 5 {
		return nil, nil, oneDiagnostic(cp.sources, CodeInvalidType, CategorySemantic, expression.Span, fmt.Sprintf("filter_joined_contains_casefolded requires exactly five public string field selectors, got %d", len(expression.Selectors)))
	}
	fields := make([]FieldDecl, 0, len(expression.Selectors))
	positions := make([]int, 0, len(expression.Selectors))
	selected := make(map[int]struct{}, len(expression.Selectors))
	for _, selector := range expression.Selectors {
		record, err := cp.resolveType(selector.RecordType)
		if err != nil {
			return nil, nil, err
		}
		if !isResolvedRecordList(values) || !record.Equal(values.Arguments[0]) {
			return nil, nil, oneDiagnostic(cp.sources, CodeInvalidType, CategorySemantic, selector.RecordType.Span, fmt.Sprintf("filter_joined_contains_casefolded selector type %s must match list element type %s", record, values.Arguments[0]))
		}
		fieldType, field, position, err := cp.resolveRecordField(record, selector.Field, selector.FieldSpan)
		if err != nil {
			return nil, nil, err
		}
		if normalizeVisibility(field.Visibility) != VisibilityPublic || !fieldType.Equal(resolvedPrimitive(TypeString)) {
			return nil, nil, oneDiagnostic(cp.sources, CodeInvalidType, CategorySemantic, selector.FieldSpan, fmt.Sprintf("filter_joined_contains_casefolded selector %s.%s must identify one public string field", selector.RecordType.Name, selector.Field), RelatedSpan{Span: field.Span, Message: "record field declaration"})
		}
		if _, duplicate := selected[position]; duplicate {
			return nil, nil, oneDiagnostic(cp.sources, CodeInvalidType, CategorySemantic, selector.FieldSpan, fmt.Sprintf("filter_joined_contains_casefolded selector %s.%s is duplicated", selector.RecordType.Name, selector.Field), RelatedSpan{Span: field.Span, Message: "record field declaration"})
		}
		selected[position] = struct{}{}
		fields = append(fields, field)
		positions = append(positions, position)
	}
	return fields, positions, nil
}

func (cp *checkedProgram) resolveListSortByOrdinalSelector(expression *ListSortByOrdinalExpr, values ResolvedTypeRef) (FieldDecl, int, error) {
	record, err := cp.resolveType(expression.RecordType)
	if err != nil {
		return FieldDecl{}, 0, err
	}
	if !isResolvedRecordList(values) || !record.Equal(values.Arguments[0]) {
		return FieldDecl{}, 0, oneDiagnostic(cp.sources, CodeInvalidType, CategorySemantic, expression.RecordType.Span, fmt.Sprintf("sort_by_ordinal selector type %s must match list element type %s", record, values.Arguments[0]))
	}
	fieldType, field, position, err := cp.resolveRecordField(record, expression.Field, expression.FieldSpan)
	if err != nil {
		return FieldDecl{}, 0, err
	}
	if normalizeVisibility(field.Visibility) != VisibilityPublic || !fieldType.Equal(resolvedPrimitive(TypeString)) {
		return FieldDecl{}, 0, oneDiagnostic(cp.sources, CodeInvalidType, CategorySemantic, expression.FieldSpan, fmt.Sprintf("sort_by_ordinal selector %s.%s must identify one public string field", expression.RecordType.Name, expression.Field), RelatedSpan{Span: field.Span, Message: "record field declaration"})
	}
	return field, position, nil
}

func (cp *checkedProgram) resolveListSortByOrdinalsSelectors(expression *ListSortByOrdinalsExpr, values ResolvedTypeRef) ([]FieldDecl, []int, error) {
	if len(expression.Selectors) < 2 {
		return nil, nil, oneDiagnostic(cp.sources, CodeInvalidType, CategorySemantic, expression.Span, fmt.Sprintf("sort_by_ordinal requires at least two distinct public string field selectors in its multi-key form, got %d", len(expression.Selectors)))
	}
	fields := make([]FieldDecl, 0, len(expression.Selectors))
	positions := make([]int, 0, len(expression.Selectors))
	selected := make(map[int]struct{}, len(expression.Selectors))
	for _, selector := range expression.Selectors {
		record, err := cp.resolveType(selector.RecordType)
		if err != nil {
			return nil, nil, err
		}
		if !isResolvedRecordList(values) || !record.Equal(values.Arguments[0]) {
			return nil, nil, oneDiagnostic(cp.sources, CodeInvalidType, CategorySemantic, selector.RecordType.Span, fmt.Sprintf("sort_by_ordinal selector type %s must match list element type %s", record, values.Arguments[0]))
		}
		fieldType, field, position, err := cp.resolveRecordField(record, selector.Field, selector.FieldSpan)
		if err != nil {
			return nil, nil, err
		}
		if normalizeVisibility(field.Visibility) != VisibilityPublic || !fieldType.Equal(resolvedPrimitive(TypeString)) {
			return nil, nil, oneDiagnostic(cp.sources, CodeInvalidType, CategorySemantic, selector.FieldSpan, fmt.Sprintf("sort_by_ordinal selector %s.%s must identify one public string field", selector.RecordType.Name, selector.Field), RelatedSpan{Span: field.Span, Message: "record field declaration"})
		}
		if _, duplicate := selected[position]; duplicate {
			return nil, nil, oneDiagnostic(cp.sources, CodeInvalidType, CategorySemantic, selector.FieldSpan, fmt.Sprintf("sort_by_ordinal selector %s.%s is duplicated", selector.RecordType.Name, selector.Field), RelatedSpan{Span: field.Span, Message: "record field declaration"})
		}
		selected[position] = struct{}{}
		fields = append(fields, field)
		positions = append(positions, position)
	}
	return fields, positions, nil
}

func (cp *checkedProgram) inferOptionalMethodBodyType(method MethodDecl, env map[string]ResolvedTypeRef, declared ResolvedTypeRef) (ResolvedTypeRef, bool, error) {
	parameterTypes := make([]ResolvedTypeRef, 0, len(method.Params))
	hasOptional := containsResolvedOptional(declared) || containsOptionalExpression(method.Body)
	for _, parameter := range method.Params {
		resolved, err := cp.resolveType(parameter.Type)
		if err != nil {
			return ResolvedTypeRef{}, true, err
		}
		parameterTypes = append(parameterTypes, resolved)
		hasOptional = hasOptional || containsResolvedOptional(resolved)
	}
	if !hasOptional {
		return ResolvedTypeRef{}, false, nil
	}
	contract := cp.modules.LanguageContract()
	if !hasPrimitiveOptionalSourceContract(contract) {
		return ResolvedTypeRef{}, true, oneDiagnostic(cp.sources, CodeInvalidType, CategorySemantic, method.Span, fmt.Sprintf("primitive Optional methods require language contract %q", PipeLangLanguageContractV130))
	}
	directParameter := func(expression Expr, position int) bool {
		identifier, ok := expression.(*IdentExpr)
		return ok && position < len(method.Params) && identifier.Name == method.Params[position].Name
	}
	switch body := method.Body.(type) {
	case *MatchExpr:
		if hasMatchSourceContract(contract) && len(parameterTypes) == 1 && cp.isResolvedOptionalValue(contract, parameterTypes[0]) && directParameter(body.Value, 0) {
			actual, err := cp.inferExprType(body, env)
			if err != nil {
				return ResolvedTypeRef{}, true, err
			}
			if actual.Equal(declared) {
				return declared, true, nil
			}
		}
	case *OptionalSomeExpr:
		valid := cp.isResolvedOptionalValue(contract, declared) && len(parameterTypes) == 1 && parameterTypes[0].Equal(declared.Arguments[0]) && directParameter(body.Value, 0)
		if valid {
			return declared, true, nil
		}
	case *OptionalNoneExpr:
		valueType, err := cp.resolveType(body.ValueType)
		if err != nil {
			return ResolvedTypeRef{}, true, err
		}
		if cp.isResolvedOptionalValue(contract, declared) && len(parameterTypes) == 0 && valueType.Equal(declared.Arguments[0]) {
			return declared, true, nil
		}
	case *IdentExpr:
		if cp.isResolvedOptionalValue(contract, declared) && len(parameterTypes) == 1 && parameterTypes[0].Equal(declared) && body.Name == method.Params[0].Name {
			return declared, true, nil
		}
	case *OptionalHasValueExpr:
		if declared.Equal(resolvedPrimitive(TypeBool)) && len(parameterTypes) == 1 && cp.isResolvedOptionalValue(contract, parameterTypes[0]) && directParameter(body.Value, 0) {
			return declared, true, nil
		}
	case *OptionalValueOrExpr:
		if hasPrimitiveOptionalDefaultSourceContract(contract) && len(parameterTypes) == 2 && cp.isResolvedOptionalValue(contract, parameterTypes[0]) && parameterTypes[0].Arguments[0].Equal(declared) && parameterTypes[1].Equal(declared) && directParameter(body.Value, 0) && directParameter(body.Fallback, 1) {
			return declared, true, nil
		}
	}
	forms := "some, none, identity transport, or has_value"
	if hasPrimitiveOptionalDefaultSourceContract(contract) {
		forms = "some, none, identity transport, has_value, or value_or"
	}
	return ResolvedTypeRef{}, true, oneDiagnostic(cp.sources, CodeExpressionType, CategorySemantic, method.Body.SourceSpan(), fmt.Sprintf("%s primitive Optional requires one complete direct %s method body", contract, forms))
}

func (cp *checkedProgram) inferNonResultMethodBodyType(method MethodDecl, env map[string]ResolvedTypeRef, declared ResolvedTypeRef) (ResolvedTypeRef, error) {
	if cp != nil && cp.modules != nil && hasNamedRecordPredicateSourceContract(cp.modules.LanguageContract()) && declared.Equal(resolvedPrimitive(TypeBool)) && len(method.Params) >= 2 {
		rowType, err := cp.resolveType(method.Params[0].Type)
		if err != nil {
			return ResolvedTypeRef{}, err
		}
		if cp.isResolvedRecordType(rowType) {
			owner := cp.ownerClassForMethod(method)
			validSignature := owner != nil && normalizeVisibility(owner.Visibility) == VisibilityPublic && normalizeVisibility(method.Visibility) == VisibilityPublic
			allowed := make(map[string]struct{}, len(method.Params)-1)
			for _, parameter := range method.Params[1:] {
				resolved, err := cp.resolveType(parameter.Type)
				if err != nil {
					return ResolvedTypeRef{}, err
				}
				validSignature = validSignature && resolved.Kind == TypeRefPrimitive
				allowed[parameter.Name] = struct{}{}
			}
			if !validSignature || !isNamedPredicateExpression(method.Body, method.Params[0].Name, allowed) {
				return ResolvedTypeRef{}, oneDiagnostic(cp.sources, CodeExpressionType, CategorySemantic, method.Body.SourceSpan(), fmt.Sprintf("%s named record predicates require one public same-class bool method over a primitive record followed by primitive parameters and a bounded pure predicate body", cp.modules.LanguageContract()))
			}
			inferred, err := cp.inferNamedPredicateExprType(method.Body, env)
			if err != nil {
				return ResolvedTypeRef{}, err
			}
			if !inferred.Equal(declared) {
				return ResolvedTypeRef{}, oneDiagnostic(cp.sources, CodeExpressionType, CategorySemantic, method.Body.SourceSpan(), fmt.Sprintf("named record predicate returns %s, expected bool", inferred))
			}
			return inferred, nil
		}
	}
	if cp != nil && cp.modules != nil && hasTextTrimSourceContract(cp.modules.LanguageContract()) {
		if body, ok := method.Body.(*TextTrimExpr); ok {
			text := resolvedPrimitive(TypeString)
			valid := declared.Equal(text) && len(method.Params) == 1
			if valid {
				parameter, err := cp.resolveType(method.Params[0].Type)
				if err != nil {
					return ResolvedTypeRef{}, err
				}
				valid = parameter.Equal(text)
			}
			value, valueOK := body.Value.(*IdentExpr)
			valid = valid && valueOK && value.Name == method.Params[0].Name
			if !valid {
				return ResolvedTypeRef{}, oneDiagnostic(cp.sources, CodeExpressionType, CategorySemantic, method.Body.SourceSpan(), fmt.Sprintf("%s trim requires exactly one direct string parameter as the complete string method body", cp.modules.LanguageContract()))
			}
			return text, nil
		}
		if containsTextTrimExpression(method.Body) {
			return ResolvedTypeRef{}, oneDiagnostic(cp.sources, CodeExpressionType, CategorySemantic, method.Body.SourceSpan(), fmt.Sprintf("%s trim requires exactly one direct string parameter as the complete string method body", cp.modules.LanguageContract()))
		}
	}
	if cp != nil && cp.modules != nil && hasCaseFoldedTextContainmentSourceContract(cp.modules.LanguageContract()) {
		if body, ok := method.Body.(*TextContainsCaseFoldedExpr); ok {
			text := resolvedPrimitive(TypeString)
			boolean := resolvedPrimitive(TypeBool)
			valid := declared.Equal(boolean) && len(method.Params) == 2
			for _, parameter := range method.Params {
				resolved, err := cp.resolveType(parameter.Type)
				if err != nil {
					return ResolvedTypeRef{}, err
				}
				valid = valid && resolved.Equal(text)
			}
			value, valueOK := body.Value.(*IdentExpr)
			query, queryOK := body.Query.(*IdentExpr)
			valid = valid && valueOK && queryOK && value.Name == method.Params[0].Name && query.Name == method.Params[1].Name
			if !valid {
				return ResolvedTypeRef{}, oneDiagnostic(cp.sources, CodeExpressionType, CategorySemantic, method.Body.SourceSpan(), fmt.Sprintf("%s contains_casefolded requires exactly two string parameters in declared order as the complete bool method body", cp.modules.LanguageContract()))
			}
			return boolean, nil
		}
		if containsCaseFoldedTextExpression(method.Body) {
			return ResolvedTypeRef{}, oneDiagnostic(cp.sources, CodeExpressionType, CategorySemantic, method.Body.SourceSpan(), fmt.Sprintf("%s contains_casefolded requires exactly two string parameters in declared order as the complete bool method body", cp.modules.LanguageContract()))
		}
	}
	if cp.isResolvedRecordType(declared) {
		if construction, ok := method.Body.(*RecordConstructExpr); ok && cp.modules != nil && hasPrimitiveRecordConstructionSourceContract(cp.modules.LanguageContract()) {
			return cp.validateRecordConstruction(method, construction, env, declared)
		}
		if len(method.Params) == 1 {
			if identifier, ok := method.Body.(*IdentExpr); ok && identifier.Name == method.Params[0].Name {
				resolved, err := cp.resolveType(method.Params[0].Type)
				if err != nil {
					return ResolvedTypeRef{}, err
				}
				if resolved.Equal(declared) {
					return declared, nil
				}
			}
		}
		return ResolvedTypeRef{}, oneDiagnostic(cp.sources, CodeExpressionType, CategorySemantic, method.Body.SourceSpan(), fmt.Sprintf("%s primitive record transport requires its sole record parameter as the complete method body", cp.modules.LanguageContract()))
	}
	if containsRecordConstruction(method.Body) {
		return ResolvedTypeRef{}, oneDiagnostic(cp.sources, CodeExpressionType, CategorySemantic, method.Body.SourceSpan(), fmt.Sprintf("%s primitive record construction must be the complete body of a method returning that record", cp.modules.LanguageContract()))
	}
	if cp.modules != nil && hasPrimitiveRecordEqualitySourceContract(cp.modules.LanguageContract()) {
		parameters := make([]ResolvedTypeRef, 0, len(method.Params))
		for _, parameter := range method.Params {
			resolved, err := cp.resolveType(parameter.Type)
			if err != nil {
				return ResolvedTypeRef{}, err
			}
			parameters = append(parameters, resolved)
		}
		if cp.recordEqualitySignatureMatches(declared, parameters) {
			binary, directBinary := method.Body.(*BinaryExpr)
			if !directBinary || (binary.Op != "==" && binary.Op != "!=") {
				return ResolvedTypeRef{}, oneDiagnostic(cp.sources, CodeExpressionType, CategorySemantic, method.Body.SourceSpan(), fmt.Sprintf("%s primitive record equality requires exactly two identical record parameters compared in declared order with == or != as the complete bool method body", cp.modules.LanguageContract()))
			}
			leftIdentifier, leftOK := binary.Left.(*IdentExpr)
			rightIdentifier, rightOK := binary.Right.(*IdentExpr)
			if !leftOK || !rightOK || leftIdentifier.Name != method.Params[0].Name || rightIdentifier.Name != method.Params[1].Name {
				return ResolvedTypeRef{}, oneDiagnostic(cp.sources, CodeExpressionType, CategorySemantic, method.Body.SourceSpan(), fmt.Sprintf("%s primitive record equality requires exactly two identical record parameters compared in declared order with == or != as the complete bool method body", cp.modules.LanguageContract()))
			}
			return resolvedPrimitive(TypeBool), nil
		}
	}
	if cp.modules != nil && hasRecordFieldProjectionSourceContract(cp.modules.LanguageContract()) {
		hasRecordParameter := false
		for _, parameter := range method.Params {
			resolved, err := cp.resolveType(parameter.Type)
			if err != nil {
				return ResolvedTypeRef{}, err
			}
			hasRecordParameter = hasRecordParameter || cp.isResolvedRecordType(resolved)
		}
		if hasRecordParameter {
			field, directField := method.Body.(*FieldExpr)
			if !directField {
				return ResolvedTypeRef{}, oneDiagnostic(cp.sources, CodeExpressionType, CategorySemantic, method.Body.SourceSpan(), fmt.Sprintf("%s primitive record field projection requires parameter.Field as the complete method body", cp.modules.LanguageContract()))
			}
			inferred, err := cp.inferExprType(field, env)
			if err != nil {
				return ResolvedTypeRef{}, err
			}
			receiver, directReceiver := field.Receiver.(*IdentExpr)
			if len(method.Params) != 1 || !directReceiver || receiver.Name != method.Params[0].Name || !inferred.Equal(declared) {
				return ResolvedTypeRef{}, oneDiagnostic(cp.sources, CodeExpressionType, CategorySemantic, method.Body.SourceSpan(), fmt.Sprintf("%s primitive record field projection requires the sole record parameter, one declared field, and that field's exact primitive return type", cp.modules.LanguageContract()))
			}
			return inferred, nil
		}
	}
	binary, directBinary := method.Body.(*BinaryExpr)
	if cp != nil && cp.modules != nil && hasOrdinalTextOrderingSourceContract(cp.modules.LanguageContract()) && directBinary && isOrdinalTextOrderingOperator(binary.Op) {
		left, err := inferExprTypeWithPolicy(cp.sources, binary.Left, env, true)
		if err != nil {
			return ResolvedTypeRef{}, err
		}
		right, err := inferExprTypeWithPolicy(cp.sources, binary.Right, env, true)
		if err != nil {
			return ResolvedTypeRef{}, err
		}
		text := resolvedPrimitive(TypeString)
		if left.Equal(text) || right.Equal(text) {
			boolean := resolvedPrimitive(TypeBool)
			valid := declared.Equal(boolean) && left.Equal(text) && right.Equal(text) && len(method.Params) == 2
			leftIdentifier, leftOK := binary.Left.(*IdentExpr)
			rightIdentifier, rightOK := binary.Right.(*IdentExpr)
			valid = valid && leftOK && rightOK && leftIdentifier.Name == method.Params[0].Name && rightIdentifier.Name == method.Params[1].Name
			for _, parameter := range method.Params {
				resolved, resolveErr := cp.resolveType(parameter.Type)
				if resolveErr != nil {
					return ResolvedTypeRef{}, resolveErr
				}
				valid = valid && resolved.Equal(text)
			}
			if !valid {
				return ResolvedTypeRef{}, oneDiagnostic(cp.sources, CodeExpressionType, CategorySemantic, method.Body.SourceSpan(), fmt.Sprintf("%s ordinal text ordering requires exactly two string parameters compared in declared order as the complete bool method body", cp.modules.LanguageContract()))
			}
			return boolean, nil
		}
	}
	return cp.inferExprType(method.Body, env)
}

func (cp *checkedProgram) inferNamedPredicateExprType(expression Expr, env map[string]ResolvedTypeRef) (ResolvedTypeRef, error) {
	switch node := expression.(type) {
	case *LiteralExpr, *IdentExpr, *FieldExpr:
		return cp.inferExprType(expression, env)
	case *TextTrimExpr:
		value, err := cp.inferNamedPredicateExprType(node.Value, env)
		if err != nil {
			return ResolvedTypeRef{}, err
		}
		if !value.Equal(resolvedPrimitive(TypeString)) {
			return ResolvedTypeRef{}, oneDiagnostic(cp.sources, CodeInvalidType, CategorySemantic, node.Span, fmt.Sprintf("trim requires string, got %s", value))
		}
		return resolvedPrimitive(TypeString), nil
	case *TextContainsCaseFoldedExpr:
		value, err := cp.inferNamedPredicateExprType(node.Value, env)
		if err != nil {
			return ResolvedTypeRef{}, err
		}
		query, err := cp.inferNamedPredicateExprType(node.Query, env)
		if err != nil {
			return ResolvedTypeRef{}, err
		}
		if !value.Equal(resolvedPrimitive(TypeString)) || !query.Equal(resolvedPrimitive(TypeString)) {
			return ResolvedTypeRef{}, oneDiagnostic(cp.sources, CodeInvalidType, CategorySemantic, node.Span, "contains_casefolded requires two strings")
		}
		return resolvedPrimitive(TypeBool), nil
	case *UnaryExpr:
		operand, err := cp.inferNamedPredicateExprType(node.Expr, env)
		if err != nil {
			return ResolvedTypeRef{}, err
		}
		if node.Op != "!" || !operand.Equal(resolvedPrimitive(TypeBool)) {
			return ResolvedTypeRef{}, oneDiagnostic(cp.sources, CodeExpressionType, CategorySemantic, node.Span, "named predicate unary expression requires bool logical not")
		}
		return resolvedPrimitive(TypeBool), nil
	case *BinaryExpr:
		left, err := cp.inferNamedPredicateExprType(node.Left, env)
		if err != nil {
			return ResolvedTypeRef{}, err
		}
		right, err := cp.inferNamedPredicateExprType(node.Right, env)
		if err != nil {
			return ResolvedTypeRef{}, err
		}
		if left.Equal(resolvedPrimitive(TypeString)) && right.Equal(left) && isOrdinalTextOrderingOperator(node.Op) {
			return resolvedPrimitive(TypeBool), nil
		}
		return inferBinaryTypeWithPolicy(cp.sources, node.Span, node.Op, left, right, true)
	}
	return ResolvedTypeRef{}, oneDiagnostic(cp.sources, CodeExpressionType, CategorySemantic, expression.SourceSpan(), "unsupported named predicate expression")
}

func (cp *checkedProgram) ownerClassForMethod(target MethodDecl) *ClassDecl {
	if cp == nil || cp.program == nil {
		return nil
	}
	for _, class := range cp.program.Classes {
		for _, method := range class.Methods {
			if method.Name == target.Name && method.Span == target.Span {
				return class
			}
		}
	}
	return nil
}

func (cp *checkedProgram) bindPureCalls(class *ClassDecl) error {
	if class == nil {
		return nil
	}
	methods := make(map[string]*MethodDecl, len(class.Methods))
	bySpan := make(map[Span]*MethodDecl, len(class.Methods))
	for index := range class.Methods {
		method := &class.Methods[index]
		methods[method.Name] = method
		bySpan[method.Span] = method
	}
	edges := make(map[Span][]*CallExpr, len(class.Methods))
	participants := make(map[Span]struct{}, len(class.Methods))
	var bind func(MethodDecl, Expr) error
	bind = func(caller MethodDecl, expression Expr) error {
		if call, ok := expression.(*CallExpr); ok {
			if normalizeVisibility(caller.Visibility) != VisibilityPublic {
				return oneDiagnostic(cp.sources, CodeInvalidMember, CategorySemantic, call.NameSpan, fmt.Sprintf("%s same-class calls require a public calling method", cp.modules.LanguageContract()), RelatedSpan{Span: caller.Span, Message: "calling method declaration"})
			}
			target := methods[call.Name]
			if target == nil {
				return oneDiagnostic(cp.sources, CodeInvalidMember, CategorySemantic, call.NameSpan, fmt.Sprintf("class %s has no method %q", class.Name, call.Name), RelatedSpan{Span: class.Span, Message: "owning class declaration"})
			}
			if normalizeVisibility(target.Visibility) != VisibilityPublic {
				return oneDiagnostic(cp.sources, CodeInvalidMember, CategorySemantic, call.NameSpan, fmt.Sprintf("same-class call target %s.%s must be public", class.Name, call.Name), RelatedSpan{Span: target.Span, Message: "called method declaration"})
			}
			call.TargetSpan = target.Span
			edges[caller.Span] = append(edges[caller.Span], call)
			participants[caller.Span] = struct{}{}
			participants[target.Span] = struct{}{}
		}
		for _, child := range expressionChildren(expression) {
			if err := bind(caller, child); err != nil {
				return err
			}
		}
		return nil
	}
	for _, method := range class.Methods {
		if err := bind(method, method.Body); err != nil {
			return err
		}
	}
	fields := make(map[string]struct{}, len(class.Fields))
	for _, field := range class.Fields {
		fields[field.Name] = struct{}{}
	}
	for index := range class.Methods {
		method := &class.Methods[index]
		if _, participates := participants[method.Span]; !participates {
			continue
		}
		bound := make(map[string]struct{}, len(method.Params))
		for _, parameter := range method.Params {
			bound[parameter.Name] = struct{}{}
		}
		if expressionReferencesClassState(method.Body, fields, bound) {
			return oneDiagnostic(cp.sources, CodeExpressionType, CategorySemantic, method.Body.SourceSpan(), fmt.Sprintf("%s pure call participant %s.%s may reference only parameters and arm-local bindings", cp.modules.LanguageContract(), class.Name, method.Name), RelatedSpan{Span: method.Span, Message: "call participant declaration"})
		}
	}
	state := make(map[Span]uint8, len(class.Methods))
	var visit func(*MethodDecl) error
	visit = func(method *MethodDecl) error {
		state[method.Span] = 1
		for _, call := range edges[method.Span] {
			target := bySpan[call.TargetSpan]
			if target == nil {
				return oneDiagnostic(cp.sources, CodeInvalidMember, CategorySemantic, call.NameSpan, fmt.Sprintf("same-class call target %q is unavailable", call.Name))
			}
			if state[target.Span] == 1 {
				return oneDiagnostic(cp.sources, CodePureCallCycle, CategorySemantic, call.NameSpan, fmt.Sprintf("pure call cycle reaches %s.%s", class.Name, target.Name), RelatedSpan{Span: target.Span, Message: "cycle target declaration"})
			}
			if state[target.Span] == 0 {
				if err := visit(target); err != nil {
					return err
				}
			}
		}
		state[method.Span] = 2
		return nil
	}
	for index := range class.Methods {
		if state[class.Methods[index].Span] == 0 {
			if err := visit(&class.Methods[index]); err != nil {
				return err
			}
		}
	}
	return nil
}

func expressionReferencesClassState(expression Expr, fields, bound map[string]struct{}) bool {
	switch node := expression.(type) {
	case *IdentExpr:
		_, isField := fields[node.Name]
		_, isBound := bound[node.Name]
		return isField && !isBound
	case *MatchExpr:
		if expressionReferencesClassState(node.Value, fields, bound) {
			return true
		}
		for _, arm := range node.Arms {
			armBound := make(map[string]struct{}, len(bound)+1)
			for name := range bound {
				armBound[name] = struct{}{}
			}
			if arm.Binding != "" {
				armBound[arm.Binding] = struct{}{}
			}
			if expressionReferencesClassState(arm.Body, fields, armBound) {
				return true
			}
		}
		return false
	default:
		for _, child := range expressionChildren(expression) {
			if expressionReferencesClassState(child, fields, bound) {
				return true
			}
		}
		return false
	}
}

func isNamedPredicateExpression(expression Expr, row string, allowed map[string]struct{}) bool {
	switch node := expression.(type) {
	case *LiteralExpr:
		return true
	case *IdentExpr:
		_, ok := allowed[node.Name]
		return ok
	case *FieldExpr:
		receiver, ok := node.Receiver.(*IdentExpr)
		return ok && receiver.Name == row
	case *UnaryExpr:
		return node.Op == "!" && isNamedPredicateExpression(node.Expr, row, allowed)
	case *BinaryExpr:
		switch node.Op {
		case "&&", "||", "==", "!=", "<", "<=", ">", ">=":
			return isNamedPredicateExpression(node.Left, row, allowed) && isNamedPredicateExpression(node.Right, row, allowed)
		}
	case *TextTrimExpr:
		return isNamedPredicateExpression(node.Value, row, allowed)
	case *TextContainsCaseFoldedExpr:
		return isNamedPredicateExpression(node.Value, row, allowed) && isNamedPredicateExpression(node.Query, row, allowed)
	}
	return false
}

func (cp *checkedProgram) validateRecordConstruction(method MethodDecl, construction *RecordConstructExpr, env map[string]ResolvedTypeRef, declared ResolvedTypeRef) (ResolvedTypeRef, error) {
	constructed, err := cp.resolveType(construction.Type)
	if err != nil {
		return ResolvedTypeRef{}, err
	}
	if !constructed.Equal(declared) {
		return ResolvedTypeRef{}, oneDiagnostic(cp.sources, CodeInvalidType, CategorySemantic, construction.Type.Span, fmt.Sprintf("record construction type %s does not match declared return type %s", constructed, declared), RelatedSpan{Span: method.ReturnType.Span, Message: "declared return type"})
	}
	entry, ok := cp.symbols.lookupIDEntry(declared.Symbol)
	if !ok || entry.recordDecl == nil {
		return ResolvedTypeRef{}, oneDiagnostic(cp.sources, CodeInvalidType, CategorySemantic, construction.Type.Span, "record construction has no resolved record declaration")
	}
	declaredFields := entry.recordDecl.Fields
	if len(construction.Fields) != len(declaredFields) {
		return ResolvedTypeRef{}, oneDiagnostic(cp.sources, CodeInvalidType, CategorySemantic, construction.Span, fmt.Sprintf("record construction requires exactly %d declaration-ordered fields, got %d", len(declaredFields), len(construction.Fields)), RelatedSpan{Span: entry.recordDecl.Span, Message: "record declaration"})
	}
	seen := map[string]Span{}
	for index, initialized := range construction.Fields {
		if previous, duplicate := seen[initialized.Name]; duplicate {
			return ResolvedTypeRef{}, oneDiagnostic(cp.sources, CodeDuplicateMember, CategorySemantic, initialized.NameSpan, fmt.Sprintf("record construction repeats field %q", initialized.Name), RelatedSpan{Span: previous, Message: "first field initializer"})
		}
		seen[initialized.Name] = initialized.NameSpan
		declaredField := declaredFields[index]
		if initialized.Name != declaredField.Name {
			known := false
			for _, candidate := range declaredFields {
				known = known || candidate.Name == initialized.Name
			}
			if !known {
				return ResolvedTypeRef{}, oneDiagnostic(cp.sources, CodeInvalidMember, CategorySemantic, initialized.NameSpan, fmt.Sprintf("record type %s has no field %q", declared, initialized.Name), RelatedSpan{Span: entry.recordDecl.Span, Message: "record declaration"})
			}
			return ResolvedTypeRef{}, oneDiagnostic(cp.sources, CodeInvalidType, CategorySemantic, initialized.NameSpan, fmt.Sprintf("record construction field %q is out of declaration order; expected %q", initialized.Name, declaredField.Name), RelatedSpan{Span: declaredField.Span, Message: "declared field position"})
		}
		identifier, direct := initialized.Value.(*IdentExpr)
		if index >= len(method.Params) {
			return ResolvedTypeRef{}, oneDiagnostic(cp.sources, CodeInvalidType, CategorySemantic, initialized.Value.SourceSpan(), fmt.Sprintf("record field %s has no corresponding parameter", initialized.Name), RelatedSpan{Span: method.Span, Message: "record construction method"})
		}
		if !direct || identifier.Name != method.Params[index].Name {
			return ResolvedTypeRef{}, oneDiagnostic(cp.sources, CodeExpressionType, CategorySemantic, initialized.Value.SourceSpan(), fmt.Sprintf("record field %s requires its corresponding direct parameter %s", initialized.Name, method.Params[index].Name), RelatedSpan{Span: method.Params[index].Span, Message: "corresponding parameter"})
		}
		fieldType, err := cp.resolveType(declaredField.Type)
		if err != nil {
			return ResolvedTypeRef{}, err
		}
		parameterType, found := env[identifier.Name]
		if !found || !parameterType.Equal(fieldType) {
			return ResolvedTypeRef{}, oneDiagnostic(cp.sources, CodeInvalidType, CategorySemantic, identifier.Span, fmt.Sprintf("record field %s requires %s, got %s", initialized.Name, fieldType, parameterType), RelatedSpan{Span: declaredField.Type.Span, Message: "declared field type"})
		}
	}
	return declared, nil
}

func containsRecordConstruction(expression Expr) bool {
	switch node := expression.(type) {
	case *RecordConstructExpr:
		return true
	case *UnaryExpr:
		return containsRecordConstruction(node.Expr)
	case *BinaryExpr:
		return containsRecordConstruction(node.Left) || containsRecordConstruction(node.Right)
	case *TextContainsCaseFoldedExpr:
		return containsRecordConstruction(node.Value) || containsRecordConstruction(node.Query)
	case *TextTrimExpr:
		return containsRecordConstruction(node.Value)
	case *FieldExpr:
		return containsRecordConstruction(node.Receiver)
	case *ListSingletonExpr:
		return containsRecordConstruction(node.Value)
	case *ListCountExpr:
		return containsRecordConstruction(node.Value)
	case *ListAppendExpr:
		return containsRecordConstruction(node.Values) || containsRecordConstruction(node.Value)
	case *ListAtExpr:
		return containsRecordConstruction(node.Values) || containsRecordConstruction(node.Index)
	case *ListFindByTextExpr:
		return containsRecordConstruction(node.Values) || containsRecordConstruction(node.Key)
	case *ListFilterByTextExpr:
		return containsRecordConstruction(node.Values) || containsRecordConstruction(node.Key)
	case *ListFilterContainsCaseFoldedExpr:
		return containsRecordConstruction(node.Values) || containsRecordConstruction(node.Query)
	case *ListFilterJoinedContainsCaseFoldedExpr:
		return containsRecordConstruction(node.Values) || containsRecordConstruction(node.Query)
	case *ListSortByOrdinalExpr:
		return containsRecordConstruction(node.Values)
	case *ListSortByOrdinalsExpr:
		return containsRecordConstruction(node.Values)
	case *ResultOKExpr:
		return containsRecordConstruction(node.Value)
	case *ResultErrExpr:
		return containsRecordConstruction(node.Error)
	case *ResultIsOKExpr:
		return containsRecordConstruction(node.Value)
	case *ResultSuccessOrExpr:
		return containsRecordConstruction(node.Value) || containsRecordConstruction(node.Fallback)
	case *ResultFailureOrExpr:
		return containsRecordConstruction(node.Value) || containsRecordConstruction(node.Fallback)
	default:
		return false
	}
}

func containsOptionalExpression(expression Expr) bool {
	switch node := expression.(type) {
	case *OptionalSomeExpr, *OptionalNoneExpr, *OptionalHasValueExpr, *OptionalValueOrExpr:
		return true
	case *UnaryExpr:
		return containsOptionalExpression(node.Expr)
	case *BinaryExpr:
		return containsOptionalExpression(node.Left) || containsOptionalExpression(node.Right)
	case *TextContainsCaseFoldedExpr:
		return containsOptionalExpression(node.Value) || containsOptionalExpression(node.Query)
	case *TextTrimExpr:
		return containsOptionalExpression(node.Value)
	case *FieldExpr:
		return containsOptionalExpression(node.Receiver)
	case *RecordConstructExpr:
		for _, field := range node.Fields {
			if containsOptionalExpression(field.Value) {
				return true
			}
		}
	case *ListSingletonExpr:
		return containsOptionalExpression(node.Value)
	case *ListCountExpr:
		return containsOptionalExpression(node.Value)
	case *ListAppendExpr:
		return containsOptionalExpression(node.Values) || containsOptionalExpression(node.Value)
	case *ListAtExpr:
		return containsOptionalExpression(node.Values) || containsOptionalExpression(node.Index)
	case *ListFindByTextExpr:
		return containsOptionalExpression(node.Values) || containsOptionalExpression(node.Key)
	case *ListFilterByTextExpr:
		return containsOptionalExpression(node.Values) || containsOptionalExpression(node.Key)
	case *ListFilterContainsCaseFoldedExpr:
		return containsOptionalExpression(node.Values) || containsOptionalExpression(node.Query)
	case *ListFilterJoinedContainsCaseFoldedExpr:
		return containsOptionalExpression(node.Values) || containsOptionalExpression(node.Query)
	case *ListSortByOrdinalExpr:
		return containsOptionalExpression(node.Values)
	case *ListSortByOrdinalsExpr:
		return containsOptionalExpression(node.Values)
	case *ResultOKExpr:
		return containsOptionalExpression(node.Value)
	case *ResultErrExpr:
		return containsOptionalExpression(node.Error)
	case *ResultIsOKExpr:
		return containsOptionalExpression(node.Value)
	case *ResultSuccessOrExpr:
		return containsOptionalExpression(node.Value) || containsOptionalExpression(node.Fallback)
	case *ResultFailureOrExpr:
		return containsOptionalExpression(node.Value) || containsOptionalExpression(node.Fallback)
	}
	return false
}

func containsCaseFoldedTextExpression(expression Expr) bool {
	switch node := expression.(type) {
	case *TextContainsCaseFoldedExpr:
		return true
	case *TextTrimExpr:
		return containsCaseFoldedTextExpression(node.Value)
	case *UnaryExpr:
		return containsCaseFoldedTextExpression(node.Expr)
	case *BinaryExpr:
		return containsCaseFoldedTextExpression(node.Left) || containsCaseFoldedTextExpression(node.Right)
	case *FieldExpr:
		return containsCaseFoldedTextExpression(node.Receiver)
	case *RecordConstructExpr:
		for _, field := range node.Fields {
			if containsCaseFoldedTextExpression(field.Value) {
				return true
			}
		}
	case *OptionalSomeExpr:
		return containsCaseFoldedTextExpression(node.Value)
	case *OptionalHasValueExpr:
		return containsCaseFoldedTextExpression(node.Value)
	case *OptionalValueOrExpr:
		return containsCaseFoldedTextExpression(node.Value) || containsCaseFoldedTextExpression(node.Fallback)
	case *ListSingletonExpr:
		return containsCaseFoldedTextExpression(node.Value)
	case *ListCountExpr:
		return containsCaseFoldedTextExpression(node.Value)
	case *ListAppendExpr:
		return containsCaseFoldedTextExpression(node.Values) || containsCaseFoldedTextExpression(node.Value)
	case *ListAtExpr:
		return containsCaseFoldedTextExpression(node.Values) || containsCaseFoldedTextExpression(node.Index)
	case *ListFindByTextExpr:
		return containsCaseFoldedTextExpression(node.Values) || containsCaseFoldedTextExpression(node.Key)
	case *ListFilterByTextExpr:
		return containsCaseFoldedTextExpression(node.Values) || containsCaseFoldedTextExpression(node.Key)
	case *ListFilterPredicateExpr:
		if containsCaseFoldedTextExpression(node.Values) {
			return true
		}
		for _, argument := range node.Arguments {
			if containsCaseFoldedTextExpression(argument) {
				return true
			}
		}
	case *ListFilterContainsCaseFoldedExpr:
		return containsCaseFoldedTextExpression(node.Values) || containsCaseFoldedTextExpression(node.Query)
	case *ListFilterJoinedContainsCaseFoldedExpr:
		return true
	case *ListSortByOrdinalExpr:
		return containsCaseFoldedTextExpression(node.Values)
	case *ListSortByOrdinalsExpr:
		return containsCaseFoldedTextExpression(node.Values)
	case *ResultOKExpr:
		return containsCaseFoldedTextExpression(node.Value)
	case *ResultErrExpr:
		return containsCaseFoldedTextExpression(node.Error)
	case *ResultIsOKExpr:
		return containsCaseFoldedTextExpression(node.Value)
	case *ResultSuccessOrExpr:
		return containsCaseFoldedTextExpression(node.Value) || containsCaseFoldedTextExpression(node.Fallback)
	case *ResultFailureOrExpr:
		return containsCaseFoldedTextExpression(node.Value) || containsCaseFoldedTextExpression(node.Fallback)
	}
	return false
}

func containsTextTrimExpression(expression Expr) bool {
	switch node := expression.(type) {
	case *TextTrimExpr:
		return true
	case *UnaryExpr:
		return containsTextTrimExpression(node.Expr)
	case *BinaryExpr:
		return containsTextTrimExpression(node.Left) || containsTextTrimExpression(node.Right)
	case *TextContainsCaseFoldedExpr:
		return containsTextTrimExpression(node.Value) || containsTextTrimExpression(node.Query)
	case *FieldExpr:
		return containsTextTrimExpression(node.Receiver)
	case *RecordConstructExpr:
		for _, field := range node.Fields {
			if containsTextTrimExpression(field.Value) {
				return true
			}
		}
	case *OptionalSomeExpr:
		return containsTextTrimExpression(node.Value)
	case *OptionalHasValueExpr:
		return containsTextTrimExpression(node.Value)
	case *OptionalValueOrExpr:
		return containsTextTrimExpression(node.Value) || containsTextTrimExpression(node.Fallback)
	case *ListSingletonExpr:
		return containsTextTrimExpression(node.Value)
	case *ListCountExpr:
		return containsTextTrimExpression(node.Value)
	case *ListAppendExpr:
		return containsTextTrimExpression(node.Values) || containsTextTrimExpression(node.Value)
	case *ListAtExpr:
		return containsTextTrimExpression(node.Values) || containsTextTrimExpression(node.Index)
	case *ListFindByTextExpr:
		return containsTextTrimExpression(node.Values) || containsTextTrimExpression(node.Key)
	case *ListFilterByTextExpr:
		return containsTextTrimExpression(node.Values) || containsTextTrimExpression(node.Key)
	case *ListFilterPredicateExpr:
		if containsTextTrimExpression(node.Values) {
			return true
		}
		for _, argument := range node.Arguments {
			if containsTextTrimExpression(argument) {
				return true
			}
		}
	case *ListFilterContainsCaseFoldedExpr:
		return containsTextTrimExpression(node.Values) || containsTextTrimExpression(node.Query)
	case *ListFilterJoinedContainsCaseFoldedExpr:
		return containsTextTrimExpression(node.Values) || containsTextTrimExpression(node.Query)
	case *ListSortByOrdinalExpr:
		return containsTextTrimExpression(node.Values)
	case *ListSortByOrdinalsExpr:
		return containsTextTrimExpression(node.Values)
	case *ResultOKExpr:
		return containsTextTrimExpression(node.Value)
	case *ResultErrExpr:
		return containsTextTrimExpression(node.Error)
	case *ResultIsOKExpr:
		return containsTextTrimExpression(node.Value)
	case *ResultSuccessOrExpr:
		return containsTextTrimExpression(node.Value) || containsTextTrimExpression(node.Fallback)
	case *ResultFailureOrExpr:
		return containsTextTrimExpression(node.Value) || containsTextTrimExpression(node.Fallback)
	}
	return false
}

func containsListExpression(expression Expr) bool {
	switch node := expression.(type) {
	case *ListEmptyExpr, *ListSingletonExpr, *ListCountExpr, *ListAppendExpr, *ListAtExpr, *ListFindByTextExpr, *ListFilterByTextExpr, *ListFilterPredicateExpr, *ListFilterContainsCaseFoldedExpr, *ListFilterJoinedContainsCaseFoldedExpr, *ListSortByOrdinalExpr, *ListSortByOrdinalsExpr:
		return true
	case *UnaryExpr:
		return containsListExpression(node.Expr)
	case *BinaryExpr:
		return containsListExpression(node.Left) || containsListExpression(node.Right)
	case *TextContainsCaseFoldedExpr:
		return containsListExpression(node.Value) || containsListExpression(node.Query)
	case *TextTrimExpr:
		return containsListExpression(node.Value)
	case *FieldExpr:
		return containsListExpression(node.Receiver)
	case *RecordConstructExpr:
		for _, field := range node.Fields {
			if containsListExpression(field.Value) {
				return true
			}
		}
	case *OptionalSomeExpr:
		return containsListExpression(node.Value)
	case *OptionalHasValueExpr:
		return containsListExpression(node.Value)
	case *OptionalValueOrExpr:
		return containsListExpression(node.Value) || containsListExpression(node.Fallback)
	case *ResultOKExpr:
		return containsListExpression(node.Value)
	case *ResultErrExpr:
		return containsListExpression(node.Error)
	case *ResultIsOKExpr:
		return containsListExpression(node.Value)
	case *ResultSuccessOrExpr:
		return containsListExpression(node.Value) || containsListExpression(node.Fallback)
	case *ResultFailureOrExpr:
		return containsListExpression(node.Value) || containsListExpression(node.Fallback)
	}
	return false
}

func containsResultExpression(expression Expr) bool {
	switch node := expression.(type) {
	case *ResultOKExpr, *ResultErrExpr, *ResultIsOKExpr, *ResultSuccessOrExpr, *ResultFailureOrExpr:
		return true
	case *UnaryExpr:
		return containsResultExpression(node.Expr)
	case *BinaryExpr:
		return containsResultExpression(node.Left) || containsResultExpression(node.Right)
	case *TextContainsCaseFoldedExpr:
		return containsResultExpression(node.Value) || containsResultExpression(node.Query)
	case *TextTrimExpr:
		return containsResultExpression(node.Value)
	case *FieldExpr:
		return containsResultExpression(node.Receiver)
	case *RecordConstructExpr:
		for _, field := range node.Fields {
			if containsResultExpression(field.Value) {
				return true
			}
		}
	case *OptionalSomeExpr:
		return containsResultExpression(node.Value)
	case *OptionalHasValueExpr:
		return containsResultExpression(node.Value)
	case *OptionalValueOrExpr:
		return containsResultExpression(node.Value) || containsResultExpression(node.Fallback)
	case *ListSingletonExpr:
		return containsResultExpression(node.Value)
	case *ListCountExpr:
		return containsResultExpression(node.Value)
	case *ListAppendExpr:
		return containsResultExpression(node.Values) || containsResultExpression(node.Value)
	case *ListAtExpr:
		return containsResultExpression(node.Values) || containsResultExpression(node.Index)
	case *ListFindByTextExpr:
		return containsResultExpression(node.Values) || containsResultExpression(node.Key)
	case *ListFilterByTextExpr:
		return containsResultExpression(node.Values) || containsResultExpression(node.Key)
	case *ListFilterContainsCaseFoldedExpr:
		return containsResultExpression(node.Values) || containsResultExpression(node.Query)
	case *ListFilterJoinedContainsCaseFoldedExpr:
		return containsResultExpression(node.Values) || containsResultExpression(node.Query)
	case *ListSortByOrdinalExpr:
		return containsResultExpression(node.Values)
	case *ListSortByOrdinalsExpr:
		return containsResultExpression(node.Values)
	}
	return false
}

func isOrdinalTextOrderingOperator(operator string) bool {
	switch operator {
	case "<", "<=", ">", ">=":
		return true
	default:
		return false
	}
}

func arithmeticSourceOperators(contract LanguageContract) string {
	if (contract == PipeLangLanguageContractV730 || contract == PipeLangLanguageContractV720 || contract == PipeLangLanguageContractV710 || contract == PipeLangLanguageContractV700 || contract == PipeLangLanguageContractV690 || contract == PipeLangLanguageContractV680 || contract == PipeLangLanguageContractV670 || contract == PipeLangLanguageContractV660 || contract == PipeLangLanguageContractV650 || contract == PipeLangLanguageContractV640 || contract == PipeLangLanguageContractV630 || contract == PipeLangLanguageContractV620 || contract == PipeLangLanguageContractV610 || contract == PipeLangLanguageContractV600 || contract == PipeLangLanguageContractV590 || contract == PipeLangLanguageContractV580) || contract == PipeLangLanguageContractV570 || contract == PipeLangLanguageContractV560 || contract == PipeLangLanguageContractV550 || contract == PipeLangLanguageContractV540 || contract == PipeLangLanguageContractV530 || contract == PipeLangLanguageContractV520 || contract == PipeLangLanguageContractV510 || contract == PipeLangLanguageContractV500 || contract == PipeLangLanguageContractV490 || contract == PipeLangLanguageContractV310 || contract == PipeLangLanguageContractV320 || contract == PipeLangLanguageContractV330 || contract == PipeLangLanguageContractV340 || contract == PipeLangLanguageContractV350 || contract == PipeLangLanguageContractV360 || contract == PipeLangLanguageContractV370 || contract == PipeLangLanguageContractV380 || contract == PipeLangLanguageContractV390 || contract == PipeLangLanguageContractV400 || contract == PipeLangLanguageContractV410 || contract == PipeLangLanguageContractV420 || contract == PipeLangLanguageContractV430 || contract == PipeLangLanguageContractV440 || contract == PipeLangLanguageContractV450 || contract == PipeLangLanguageContractV460 || contract == PipeLangLanguageContractV470 || contract == PipeLangLanguageContractV480 {
		return "addition, subtraction, multiplication, or negation"
	}
	if contract == PipeLangLanguageContractV050 || contract == PipeLangLanguageContractV060 || contract == PipeLangLanguageContractV070 || contract == PipeLangLanguageContractV080 || contract == PipeLangLanguageContractV090 || contract == PipeLangLanguageContractV100 || contract == PipeLangLanguageContractV110 || contract == PipeLangLanguageContractV120 || contract == PipeLangLanguageContractV130 || contract == PipeLangLanguageContractV140 || contract == PipeLangLanguageContractV150 || contract == PipeLangLanguageContractV160 || contract == PipeLangLanguageContractV170 || contract == PipeLangLanguageContractV180 || contract == PipeLangLanguageContractV190 || contract == PipeLangLanguageContractV200 || contract == PipeLangLanguageContractV210 || contract == PipeLangLanguageContractV220 || contract == PipeLangLanguageContractV230 || contract == PipeLangLanguageContractV240 || contract == PipeLangLanguageContractV260 || contract == PipeLangLanguageContractV250 || contract == PipeLangLanguageContractV270 || contract == PipeLangLanguageContractV280 || contract == PipeLangLanguageContractV290 || contract == PipeLangLanguageContractV300 {
		return "addition, subtraction, multiplication, or negation"
	}
	if contract == PipeLangLanguageContractV040 {
		return "addition, subtraction, or multiplication"
	}
	if contract == PipeLangLanguageContractV030 {
		return "addition or subtraction"
	}
	return "addition"
}

func inferExprTypeWithPolicy(sources *SourceSet, expr Expr, env map[string]ResolvedTypeRef, strictNumeric bool) (ResolvedTypeRef, error) {
	switch node := expr.(type) {
	case *LiteralExpr:
		return resolvedPrimitive(node.Value.Type), nil
	case *IdentExpr:
		resolved, ok := env[node.Name]
		if !ok {
			return ResolvedTypeRef{}, oneDiagnostic(sources, CodeExpressionType, CategorySemantic, node.Span, fmt.Sprintf("unknown identifier %q", node.Name))
		}
		return resolved, nil
	case *UnaryExpr:
		resolved, err := inferExprTypeWithPolicy(sources, node.Expr, env, strictNumeric)
		if err != nil {
			return ResolvedTypeRef{}, err
		}
		switch node.Op {
		case "!":
			if !resolved.Equal(resolvedPrimitive(TypeBool)) {
				return ResolvedTypeRef{}, oneDiagnostic(sources, CodeExpressionType, CategorySemantic, node.Span, fmt.Sprintf("operator ! expects bool, got %s", resolved))
			}
			return resolvedPrimitive(TypeBool), nil
		case "-":
			if !isResolvedNumeric(resolved) {
				return ResolvedTypeRef{}, oneDiagnostic(sources, CodeExpressionType, CategorySemantic, node.Span, fmt.Sprintf("operator - expects int or float, got %s", resolved))
			}
			if strictNumeric {
				return ResolvedTypeRef{}, oneDiagnostic(sources, CodeNumericSemantics, CategorySemantic, node.Span, "numeric negation requires an explicitly declared checked Result return")
			}
			return resolved, nil
		default:
			return ResolvedTypeRef{}, oneDiagnostic(sources, CodeExpressionType, CategorySemantic, node.Span, fmt.Sprintf("unsupported unary operator %q", node.Op))
		}
	case *BinaryExpr:
		left, err := inferExprTypeWithPolicy(sources, node.Left, env, strictNumeric)
		if err != nil {
			return ResolvedTypeRef{}, err
		}
		right, err := inferExprTypeWithPolicy(sources, node.Right, env, strictNumeric)
		if err != nil {
			return ResolvedTypeRef{}, err
		}
		return inferBinaryTypeWithPolicy(sources, node.Span, node.Op, left, right, strictNumeric)
	default:
		span := Span{}
		if expr != nil {
			span = expr.SourceSpan()
		}
		return ResolvedTypeRef{}, oneDiagnostic(sources, CodeExpressionType, CategorySemantic, span, "unsupported expression")
	}
}

func inferBinaryType(sources *SourceSet, span Span, op string, left, right ResolvedTypeRef) (ResolvedTypeRef, error) {
	return inferBinaryTypeWithPolicy(sources, span, op, left, right, false)
}

func inferBinaryTypeWithPolicy(sources *SourceSet, span Span, op string, left, right ResolvedTypeRef, strictNumeric bool) (ResolvedTypeRef, error) {
	if strictNumeric && isResolvedNumeric(left) && isResolvedNumeric(right) {
		switch op {
		case "+", "-", "*", "/":
			return ResolvedTypeRef{}, oneDiagnostic(sources, CodeNumericSemantics, CategorySemantic, span, fmt.Sprintf("numeric operator %q requires an explicitly declared checked Result return", op))
		case "<", "<=", ">", ">=", "==", "!=":
			if !left.Equal(right) {
				return ResolvedTypeRef{}, oneDiagnostic(sources, CodeNumericSemantics, CategorySemantic, span, fmt.Sprintf("numeric operator %q does not implicitly convert %s and %s", op, left, right))
			}
		}
	}
	switch op {
	case "+":
		if left.Equal(resolvedPrimitive(TypeString)) && right.Equal(resolvedPrimitive(TypeString)) {
			return resolvedPrimitive(TypeString), nil
		}
		if isResolvedNumeric(left) && isResolvedNumeric(right) {
			if left.Primitive == TypeFloat || right.Primitive == TypeFloat {
				return resolvedPrimitive(TypeFloat), nil
			}
			return resolvedPrimitive(TypeInt), nil
		}
	case "-", "*":
		if isResolvedNumeric(left) && isResolvedNumeric(right) {
			if left.Primitive == TypeFloat || right.Primitive == TypeFloat {
				return resolvedPrimitive(TypeFloat), nil
			}
			return resolvedPrimitive(TypeInt), nil
		}
	case "/":
		if isResolvedNumeric(left) && isResolvedNumeric(right) {
			return resolvedPrimitive(TypeFloat), nil
		}
	case "<", "<=", ">", ">=":
		if isResolvedNumeric(left) && isResolvedNumeric(right) {
			return resolvedPrimitive(TypeBool), nil
		}
		if left.Equal(right) && !left.IsPrimitive() {
			return ResolvedTypeRef{}, oneDiagnostic(sources, CodeExpressionType, CategorySemantic, span, fmt.Sprintf("IComparable is not implemented yet for type %s", left))
		}
	case "==", "!=":
		if left.Equal(right) {
			return resolvedPrimitive(TypeBool), nil
		}
	case "&&", "||":
		if left.Equal(resolvedPrimitive(TypeBool)) && right.Equal(resolvedPrimitive(TypeBool)) {
			return resolvedPrimitive(TypeBool), nil
		}
	}
	return ResolvedTypeRef{}, oneDiagnostic(sources, CodeExpressionType, CategorySemantic, span, fmt.Sprintf("invalid operand types for %q: %s and %s", op, left, right))
}

func isResolvedNumeric(ref ResolvedTypeRef) bool {
	return ref.Kind == TypeRefPrimitive && (ref.Primitive == TypeInt || ref.Primitive == TypeFloat)
}

func pickEntryClass(cp *checkedProgram, name string) (*ClassDecl, error) {
	if cp == nil {
		return nil, oneDiagnostic(nil, CodeInvalidProgram, CategorySemantic, Span{}, "checked program is nil")
	}
	if strings.TrimSpace(name) != "" {
		entry, ok := cp.symbols.lookupEntry(name)
		if !ok || entry.symbol.Kind != SymbolClass {
			return nil, oneDiagnostic(cp.sources, CodeEntrySelection, CategorySemantic, cp.program.Span, fmt.Sprintf("class %q not found", name))
		}
		class := entry.classDecl
		if normalizeVisibility(class.Visibility) != VisibilityPublic {
			return nil, oneDiagnostic(cp.sources, CodeEntrySelection, CategorySemantic, class.Span, fmt.Sprintf("class %q is private and cannot be referenced from CLI", name))
		}
		return class, nil
	}
	if len(cp.program.Classes) == 0 {
		return nil, oneDiagnostic(cp.sources, CodeEntrySelection, CategorySemantic, cp.program.Span, "no class declarations found")
	}
	for _, class := range cp.program.Classes {
		if normalizeVisibility(class.Visibility) == VisibilityPublic {
			return class, nil
		}
	}
	return nil, oneDiagnostic(cp.sources, CodeEntrySelection, CategorySemantic, cp.program.Span, "no public class declarations found")
}
