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
		if modules != nil && hasArithmeticResultSourceContract(modules.LanguageContract()) && (entry.symbol.Name == "Result" || entry.symbol.Name == "ArithmeticError") {
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
		if containsResolvedArithmeticContractType(resolved) {
			return oneDiagnostic(cp.sources, CodeInvalidType, CategorySemantic, field.Type.Span, fmt.Sprintf("the %s arithmetic Result is admitted only as a class method return type", cp.modules.LanguageContract()))
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
		if containsResolvedArithmeticContractType(resolved) {
			return oneDiagnostic(cp.sources, CodeInvalidType, CategorySemantic, method.ReturnType.Span, fmt.Sprintf("the %s arithmetic Result requires one checked class method body", cp.modules.LanguageContract()))
		}
		if previous, ok := seen[method.Name]; ok {
			return oneDiagnostic(cp.sources, CodeDuplicateMember, CategorySemantic, method.Span, fmt.Sprintf("interface %s has duplicate member %q", decl.Name, method.Name), RelatedSpan{Span: previous, Message: "first member"})
		}
		seen[method.Name] = method.Span
		if err := cp.validateParams(decl.Name, method.Name, method.Params, false); err != nil {
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
		if containsResolvedArithmeticContractType(fieldType) {
			return oneDiagnostic(cp.sources, CodeInvalidType, CategorySemantic, field.Type.Span, fmt.Sprintf("the %s arithmetic Result is admitted only as a class method return type", cp.modules.LanguageContract()))
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
		if containsResolvedArithmeticContractType(resolved) && !isResolvedSourceArithmeticResult(cp.modules.LanguageContract(), resolved) {
			return oneDiagnostic(cp.sources, CodeInvalidType, CategorySemantic, method.ReturnType.Span, fmt.Sprintf("the %s arithmetic slice admits only its exact checked-arithmetic Result shapes as class method return types", cp.modules.LanguageContract()))
		}
		if previous, ok := seen[method.Name]; ok {
			return oneDiagnostic(cp.sources, CodeDuplicateMember, CategorySemantic, method.Span, fmt.Sprintf("class %s has duplicate member %q", decl.Name, method.Name), RelatedSpan{Span: previous, Message: "first member"})
		}
		seen[method.Name] = method.Span
		allowResultParameter, err := cp.validateResultTransportSignature(method, resolved)
		if err != nil {
			return err
		}
		if err := cp.validateParams(decl.Name, method.Name, method.Params, allowResultParameter); err != nil {
			return err
		}
	}
	for _, field := range decl.Fields {
		if field.Default == nil {
			continue
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
		inferred, err := cp.inferMethodBodyType(method.Body, env, declared)
		if err != nil {
			return prefixDiagnostic(err, fmt.Sprintf("class %s method %s: ", decl.Name, method.Name))
		}
		if !inferred.Equal(declared) {
			return oneDiagnostic(cp.sources, CodeExpressionType, CategorySemantic, method.Body.SourceSpan(), fmt.Sprintf("class %s method %s returns %s but declared %s", decl.Name, method.Name, inferred, declared), RelatedSpan{Span: method.ReturnType.Span, Message: "declared return type"})
		}
	}
	return nil
}

func (cp *checkedProgram) validateResultTransportSignature(method MethodDecl, result ResolvedTypeRef) (bool, error) {
	hasResultParameter := false
	resolvedParameters := make([]ResolvedTypeRef, 0, len(method.Params))
	for _, param := range method.Params {
		resolved, err := cp.resolveType(param.Type, RelatedSpan{Span: param.Span, Message: "parameter declaration"})
		if err != nil {
			return false, prefixDiagnostic(err, fmt.Sprintf("class method %s parameter %s: ", method.Name, param.Name))
		}
		resolvedParameters = append(resolvedParameters, resolved)
		hasResultParameter = hasResultParameter || containsResolvedArithmeticContractType(resolved)
	}
	if !hasResultParameter {
		return false, nil
	}
	contract := cp.modules.LanguageContract()
	if contract != PipeLangLanguageContractV070 || len(resolvedParameters) != 1 || !isResolvedSourceArithmeticResult(contract, result) || !resolvedParameters[0].Equal(result) {
		return false, oneDiagnostic(cp.sources, CodeInvalidType, CategorySemantic, method.Span, fmt.Sprintf("the %s arithmetic Result parameter is admitted only as one parameter identical to the method return type", contract))
	}
	return true, nil
}

func (cp *checkedProgram) validateParams(owner, method string, params []Param, allowArithmeticResult bool) error {
	seen := map[string]Span{}
	for _, param := range params {
		resolved, err := cp.resolveType(param.Type, RelatedSpan{Span: param.Span, Message: "parameter declaration"})
		if err != nil {
			return prefixDiagnostic(err, fmt.Sprintf("%s method %s parameter %s: ", owner, method, param.Name))
		}
		if containsResolvedArithmeticContractType(resolved) && !allowArithmeticResult {
			return oneDiagnostic(cp.sources, CodeInvalidType, CategorySemantic, param.Type.Span, fmt.Sprintf("the %s arithmetic Result is not admitted as a parameter type", cp.modules.LanguageContract()))
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
	strictNumeric := cp != nil && cp.modules != nil && isPipeLangSemanticContract(cp.modules.LanguageContract())
	return inferExprTypeWithPolicy(cp.sources, expr, env, strictNumeric)
}

func (cp *checkedProgram) inferMethodBodyType(expr Expr, env map[string]ResolvedTypeRef, declared ResolvedTypeRef) (ResolvedTypeRef, error) {
	if cp == nil || cp.modules == nil || !hasArithmeticResultSourceContract(cp.modules.LanguageContract()) || !isResolvedSourceArithmeticResult(cp.modules.LanguageContract(), declared) {
		return cp.inferExprType(expr, env)
	}
	contract := cp.modules.LanguageContract()
	if contract == PipeLangLanguageContractV070 {
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
			return ResolvedTypeRef{}, oneDiagnostic(cp.sources, CodeExpressionType, CategorySemantic, expr.SourceSpan(), "v0.7.0 Result transport requires the sole Result parameter as the complete method body")
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
	if unary, unaryOK := expr.(*UnaryExpr); unaryOK && (contract == PipeLangLanguageContractV050 || contract == PipeLangLanguageContractV060 || contract == PipeLangLanguageContractV070) && unary.Op == "-" {
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
	if contract == PipeLangLanguageContractV040 || contract == PipeLangLanguageContractV050 || contract == PipeLangLanguageContractV060 || contract == PipeLangLanguageContractV070 {
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

func arithmeticSourceOperators(contract LanguageContract) string {
	if contract == PipeLangLanguageContractV050 || contract == PipeLangLanguageContractV060 || contract == PipeLangLanguageContractV070 {
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
