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
		if containsCallExpression(method.Body) && !validPureCallPlacement(method.Body) {
			return oneDiagnostic(cp.sources, CodeExpressionType, CategorySemantic, method.Body.SourceSpan(), "v0.36.0 same-class calls must be the complete method body or a directly nested call argument")
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
	contract := cp.modules.LanguageContract()
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
	if !hasPrimitiveRecordSourceContract(contract) || (!identityTransport && !fieldProjection && !recordConstruction && !recordEquality && !recordList && !recordListCount && !recordListAppend && !recordListAt && !recordListFindByText && !recordListFilterByText && !recordListFilterContainsCaseFolded && !namedPredicate && !recordListFilterPredicate && !recordOptional && !snapshotResult) {
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
	if _, ok := method.Body.(*MatchExpr); ok && hasMatchSourceContract(contract) && len(resolvedParameters) == 1 && (isResolvedBoundedValueResult(contract, resolvedParameters[0]) || isResolvedSourceArithmeticResult(contract, resolvedParameters[0])) {
		return true, nil
	}
	if hasSnapshotResultSourceContract(contract) && cp.boundedResultSignatureMatches(result, resolvedParameters) {
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
	if call, ok := expr.(*CallExpr); ok {
		if cp == nil || cp.modules == nil || !hasPureCallSourceContract(cp.modules.LanguageContract()) {
			return ResolvedTypeRef{}, oneDiagnostic(cp.sources, CodeExpressionType, CategorySemantic, call.Span, "same-class pure calls require language contract v0.36.0")
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
		carrier, err := cp.inferExprType(match.Value, env)
		if err != nil {
			return ResolvedTypeRef{}, err
		}
		if !direct || identifier.Name == "" || (!cp.isResolvedOptionalValue(cp.modules.LanguageContract(), carrier) && !isResolvedBoundedValueResult(cp.modules.LanguageContract(), carrier) && !isResolvedSourceArithmeticResult(cp.modules.LanguageContract(), carrier)) {
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
		if cp == nil || cp.modules == nil || (cp.modules.LanguageContract() != PipeLangLanguageContractV320 && cp.modules.LanguageContract() != PipeLangLanguageContractV330 && cp.modules.LanguageContract() != PipeLangLanguageContractV340 && cp.modules.LanguageContract() != PipeLangLanguageContractV350 && cp.modules.LanguageContract() != PipeLangLanguageContractV360) {
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
		if list.Postfix && cp.modules.LanguageContract() != PipeLangLanguageContractV330 && cp.modules.LanguageContract() != PipeLangLanguageContractV340 && cp.modules.LanguageContract() != PipeLangLanguageContractV350 && cp.modules.LanguageContract() != PipeLangLanguageContractV360 {
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
	contract := cp.modules.LanguageContract()
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
	if unary, unaryOK := expr.(*UnaryExpr); unaryOK && (contract == PipeLangLanguageContractV310 || contract == PipeLangLanguageContractV320 || contract == PipeLangLanguageContractV330 || contract == PipeLangLanguageContractV340 || contract == PipeLangLanguageContractV350 || contract == PipeLangLanguageContractV360) && unary.Op == "-" {
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
	if contract == PipeLangLanguageContractV310 || contract == PipeLangLanguageContractV320 || contract == PipeLangLanguageContractV330 || contract == PipeLangLanguageContractV340 || contract == PipeLangLanguageContractV350 || contract == PipeLangLanguageContractV360 {
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
	if !hasSnapshotResultSourceContract(contract) || (!cp.boundedResultSignatureMatches(declared, parameterTypes) && !matchSignature) {
		return ResolvedTypeRef{}, true, oneDiagnostic(cp.sources, CodeInvalidType, CategorySemantic, method.Span, fmt.Sprintf("%s bounded Result requires one exact snapshot or text signature", contract))
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
	contract := cp.modules.LanguageContract()
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
		if body, ok := method.Body.(*ListSortByOrdinalDirectionsExpr); ok && (contract == PipeLangLanguageContractV320 || contract == PipeLangLanguageContractV330 || contract == PipeLangLanguageContractV340 || contract == PipeLangLanguageContractV350 || contract == PipeLangLanguageContractV360) && len(parameterTypes) == 1 && parameterTypes[0].Equal(declared) {
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
				return oneDiagnostic(cp.sources, CodeInvalidMember, CategorySemantic, call.NameSpan, "v0.36.0 same-class calls require a public calling method", RelatedSpan{Span: caller.Span, Message: "calling method declaration"})
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
			return oneDiagnostic(cp.sources, CodeExpressionType, CategorySemantic, method.Body.SourceSpan(), fmt.Sprintf("v0.36.0 pure call participant %s.%s may reference only parameters and arm-local bindings", class.Name, method.Name), RelatedSpan{Span: method.Span, Message: "call participant declaration"})
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
	if contract == PipeLangLanguageContractV310 || contract == PipeLangLanguageContractV320 || contract == PipeLangLanguageContractV330 || contract == PipeLangLanguageContractV340 || contract == PipeLangLanguageContractV350 || contract == PipeLangLanguageContractV360 {
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
