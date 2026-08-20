package pipelang

import (
	"fmt"

	"dockpipe/src/lib/pipelang/coreir"
	"dockpipe/src/lib/pipelang/hir"
)

// LowerSemanticMethodToHIR selects one checked post-legacy semantic method and
// lowers its existing expression syntax into typed, target-independent HIR.
func LowerSemanticMethodToHIR(analysis *Analysis, identity SemanticIdentity) (hir.Program, error) {
	if analysis == nil {
		return hir.Program{}, hirLoweringError(analysis, Span{}, identity, "typed HIR lowering requires a successful semantic module analysis")
	}
	if err := analysis.Error(); err != nil {
		return hir.Program{}, err
	}
	if analysis.Program == nil || analysis.checked == nil || analysis.Modules == nil || analysis.SemanticIDs == nil {
		return hir.Program{}, hirLoweringError(analysis, Span{}, identity, "typed HIR lowering requires a successful semantic module analysis")
	}
	if !isPipeLangSemanticContract(analysis.Modules.LanguageContract()) {
		return hir.Program{}, hirLoweringError(analysis, analysis.Program.Span, identity, fmt.Sprintf("typed HIR lowering requires a supported post-legacy language contract through %q", PipeLangLanguageContractV310))
	}
	semantic, ok := analysis.SemanticIDs.LookupIdentity(identity)
	if !ok || semantic.Kind != SemanticMethod {
		return hir.Program{}, hirLoweringError(analysis, analysis.Program.Span, identity, fmt.Sprintf("semantic method %q was not found", identity.String()))
	}

	class, method := methodBySpan(analysis.Program, semantic.DeclarationSpan)
	if class == nil || method == nil {
		return hir.Program{}, hirLoweringError(analysis, semantic.DeclarationSpan, identity, "semantic method has no checked syntax declaration")
	}
	ownerSymbol, ok := symbolBySpan(analysis.Symbols, class.Span)
	if !ok || ownerSymbol.Owner.Kind != SymbolOwnerModule {
		return hir.Program{}, hirLoweringError(analysis, class.Span, identity, "semantic method owner has no bound module symbol")
	}
	ownerIdentity, ok := analysis.SemanticIDs.IdentityForSpan(class.Span)
	if !ok || semanticIdentityKey(ownerIdentity) != semanticIdentityKey(semantic.Parent) {
		return hir.Program{}, hirLoweringError(analysis, class.Span, identity, "semantic method owner identity is inconsistent")
	}

	functionIdentity := toHIRSemanticIdentity(identity)
	parameters := make([]hir.Parameter, 0, len(method.Params))
	bindings := make(map[string]hir.Binding, len(method.Params))
	typeEnvironment := map[string]ResolvedTypeRef{}
	for _, field := range class.Fields {
		resolved, err := analysis.checked.resolveType(field.Type)
		if err != nil {
			return hir.Program{}, err
		}
		typeEnvironment[field.Name] = resolved
	}
	for position, parameter := range method.Params {
		resolved, err := analysis.checked.resolveType(parameter.Type)
		if err != nil {
			return hir.Program{}, err
		}
		binding := hir.Binding{Kind: hir.BindingParameter, Function: functionIdentity, Position: position, Name: parameter.Name}
		bindings[parameter.Name] = binding
		typeEnvironment[parameter.Name] = resolved
		parameters = append(parameters, hir.Parameter{
			Binding: binding, Type: toHIRType(analysis, resolved), TypeSpan: toHIRSpan(parameter.Type.Span), Span: toHIRSpan(parameter.Span),
		})
	}
	returnType, err := analysis.checked.resolveType(method.ReturnType)
	if err != nil {
		return hir.Program{}, err
	}
	body, err := lowerMethodBodyToHIR(analysis, identity, method.Body, bindings, typeEnvironment, returnType)
	if err != nil {
		return hir.Program{}, err
	}
	function := hir.Function{
		Identity: functionIdentity,
		Owner: hir.Owner{
			Module: ownerSymbol.Owner.ID, SymbolID: uint32(ownerSymbol.ID), Identity: toHIRSemanticIdentity(ownerIdentity), SourceSpan: toHIRSpan(ownerSymbol.DeclarationSpan),
		},
		Name: method.Name, Parameters: parameters, ReturnType: toHIRType(analysis, returnType), ReturnTypeSpan: toHIRSpan(method.ReturnType.Span), Body: body, Span: toHIRSpan(method.Span),
	}
	program := hir.Program{LanguageContract: string(analysis.Modules.LanguageContract()), CompilerContract: coreir.CompilerContractV1, Functions: []hir.Function{function}}
	if filter, ok := method.Body.(*ListFilterPredicateExpr); ok {
		predicate, err := analysis.checked.resolveNamedRecordPredicate(*method, filter.Predicate, filter.PredicateSpan)
		if err != nil {
			return hir.Program{}, err
		}
		predicateIdentity, ok := analysis.SemanticIDs.IdentityForSpan(predicate.Span)
		if !ok {
			return hir.Program{}, hirLoweringError(analysis, filter.PredicateSpan, identity, fmt.Sprintf("predicate method %q has no semantic identity", filter.Predicate))
		}
		predicateProgram, err := LowerSemanticMethodToHIR(analysis, predicateIdentity)
		if err != nil {
			return hir.Program{}, err
		}
		program.Functions = append(predicateProgram.Functions, program.Functions...)
	}
	return program, nil
}

func methodByIdentity(analysis *Analysis, identity SemanticIdentity) *MethodDecl {
	if analysis == nil || analysis.SemanticIDs == nil || analysis.Program == nil {
		return nil
	}
	semantic, ok := analysis.SemanticIDs.LookupIdentity(identity)
	if !ok {
		return nil
	}
	_, method := methodBySpan(analysis.Program, semantic.DeclarationSpan)
	return method
}

func lowerMethodBodyToHIR(analysis *Analysis, function SemanticIdentity, expression Expr, bindings map[string]hir.Binding, typeEnvironment map[string]ResolvedTypeRef, returnType ResolvedTypeRef) (hir.Expr, error) {
	contract := analysis.Modules.LanguageContract()
	if hasSnapshotResultSourceContract(contract) && (isResolvedBoundedValueResult(contract, returnType) || containsResultExpression(expression)) {
		return lowerExprToHIR(analysis, function, expression, bindings, typeEnvironment)
	}
	if analysis.checked.isResolvedRecordType(returnType) {
		if _, ok := expression.(*OptionalValueOrExpr); ok && hasPrimitiveRecordOptionalSourceContract(contract) {
			return lowerExprToHIR(analysis, function, expression, bindings, typeEnvironment)
		}
		if _, ok := expression.(*RecordConstructExpr); ok && hasPrimitiveRecordConstructionSourceContract(contract) {
			return lowerExprToHIR(analysis, function, expression, bindings, typeEnvironment)
		}
		identifier, ok := expression.(*IdentExpr)
		if !ok {
			return hir.Expr{}, hirLoweringError(analysis, expression.SourceSpan(), function, fmt.Sprintf("%s primitive record transport body is not the direct parameter reference proven by semantic analysis", contract))
		}
		resolved, found := typeEnvironment[identifier.Name]
		if !found || !resolved.Equal(returnType) {
			return hir.Expr{}, hirLoweringError(analysis, expression.SourceSpan(), function, fmt.Sprintf("%s primitive record transport body does not reference its record parameter", contract))
		}
		return lowerExprToHIR(analysis, function, expression, bindings, typeEnvironment)
	}
	if !hasArithmeticResultSourceContract(contract) || !isResolvedSourceArithmeticResult(contract, returnType) {
		if hasOrdinalTextOrderingSourceContract(contract) {
			if binary, ok := directOrdinalTextOrderingHIRShape(expression, bindings, typeEnvironment, returnType); ok {
				left, err := lowerExprToHIR(analysis, function, binary.Left, bindings, typeEnvironment)
				if err != nil {
					return hir.Expr{}, err
				}
				right, err := lowerExprToHIR(analysis, function, binary.Right, bindings, typeEnvironment)
				if err != nil {
					return hir.Expr{}, err
				}
				operator, _ := hirBinaryOperator(binary.Op)
				return hir.Expr{
					Kind: hir.ExprBinary, Type: toHIRType(analysis, returnType), Span: toHIRSpan(binary.Span),
					Binary: &hir.Binary{Operator: operator, Left: &left, Right: &right},
				}, nil
			}
		}
		return lowerExprToHIR(analysis, function, expression, bindings, typeEnvironment)
	}
	if hasResultTransportSourceContract(contract) {
		if identifier, ok := expression.(*IdentExpr); ok {
			if resolved, found := typeEnvironment[identifier.Name]; found && resolved.Equal(returnType) {
				return lowerExprToHIR(analysis, function, expression, bindings, typeEnvironment)
			}
		}
	}
	if isResolvedFloatArithmeticResult(returnType) {
		binary, ok := expression.(*BinaryExpr)
		if !ok || binary.Op != "/" {
			return hir.Expr{}, hirLoweringError(analysis, expression.SourceSpan(), function, fmt.Sprintf("%s Result method body is not the direct checked binary64 division shape proven by semantic analysis", contract))
		}
		left, err := lowerExprToHIR(analysis, function, binary.Left, bindings, typeEnvironment)
		if err != nil {
			return hir.Expr{}, err
		}
		right, err := lowerExprToHIR(analysis, function, binary.Right, bindings, typeEnvironment)
		if err != nil {
			return hir.Expr{}, err
		}
		return hir.Expr{
			Kind: hir.ExprBinary, Type: toHIRType(analysis, returnType), Span: toHIRSpan(binary.Span),
			Binary: &hir.Binary{Operator: hir.OperatorDivide, Left: &left, Right: &right},
		}, nil
	}
	if unary, ok := expression.(*UnaryExpr); ok && contract == PipeLangLanguageContractV310 && unary.Op == "-" {
		operand, err := lowerExprToHIR(analysis, function, unary.Expr, bindings, typeEnvironment)
		if err != nil {
			return hir.Expr{}, err
		}
		return hir.Expr{Kind: hir.ExprUnary, Type: toHIRType(analysis, returnType), Span: toHIRSpan(unary.Span), Unary: &hir.Unary{Operator: hir.OperatorNegate, Operand: &operand}}, nil
	}
	if unary, ok := expression.(*UnaryExpr); ok && (contract == PipeLangLanguageContractV050 || contract == PipeLangLanguageContractV060 || contract == PipeLangLanguageContractV070 || contract == PipeLangLanguageContractV080 || contract == PipeLangLanguageContractV090 || contract == PipeLangLanguageContractV100 || contract == PipeLangLanguageContractV110 || contract == PipeLangLanguageContractV120 || contract == PipeLangLanguageContractV130 || contract == PipeLangLanguageContractV140 || contract == PipeLangLanguageContractV150 || contract == PipeLangLanguageContractV160 || contract == PipeLangLanguageContractV170 || contract == PipeLangLanguageContractV180 || contract == PipeLangLanguageContractV190 || contract == PipeLangLanguageContractV200 || contract == PipeLangLanguageContractV210 || contract == PipeLangLanguageContractV220 || contract == PipeLangLanguageContractV230 || contract == PipeLangLanguageContractV240 || contract == PipeLangLanguageContractV260 || contract == PipeLangLanguageContractV250 || contract == PipeLangLanguageContractV270 || contract == PipeLangLanguageContractV280 || contract == PipeLangLanguageContractV290 || contract == PipeLangLanguageContractV300) && unary.Op == "-" {
		operand, err := lowerExprToHIR(analysis, function, unary.Expr, bindings, typeEnvironment)
		if err != nil {
			return hir.Expr{}, err
		}
		return hir.Expr{
			Kind: hir.ExprUnary, Type: toHIRType(analysis, returnType), Span: toHIRSpan(unary.Span),
			Unary: &hir.Unary{Operator: hir.OperatorNegate, Operand: &operand},
		}, nil
	}
	binary, ok := expression.(*BinaryExpr)
	operator, accepted := checkedArithmeticHIROperator(contract, binary)
	if !ok || !accepted {
		return hir.Expr{}, hirLoweringError(analysis, expression.SourceSpan(), function, fmt.Sprintf("%s Result method body is not a direct checked integer %s shape proven by semantic analysis", contract, arithmeticSourceOperators(contract)))
	}
	left, err := lowerExprToHIR(analysis, function, binary.Left, bindings, typeEnvironment)
	if err != nil {
		return hir.Expr{}, err
	}
	right, err := lowerExprToHIR(analysis, function, binary.Right, bindings, typeEnvironment)
	if err != nil {
		return hir.Expr{}, err
	}
	return hir.Expr{
		Kind: hir.ExprBinary, Type: toHIRType(analysis, returnType), Span: toHIRSpan(binary.Span),
		Binary: &hir.Binary{Operator: operator, Left: &left, Right: &right},
	}, nil
}

func checkedArithmeticHIROperator(contract LanguageContract, binary *BinaryExpr) (hir.Operator, bool) {
	if binary == nil {
		return "", false
	}
	if contract == PipeLangLanguageContractV310 {
		switch binary.Op {
		case "+":
			return hir.OperatorAdd, true
		case "-":
			return hir.OperatorSubtract, true
		case "*":
			return hir.OperatorMultiply, true
		}
	}
	switch binary.Op {
	case "+":
		return hir.OperatorAdd, true
	case "-":
		return hir.OperatorSubtract, contract == PipeLangLanguageContractV030 || contract == PipeLangLanguageContractV040 || contract == PipeLangLanguageContractV050 || contract == PipeLangLanguageContractV060 || contract == PipeLangLanguageContractV070 || contract == PipeLangLanguageContractV080 || contract == PipeLangLanguageContractV090 || contract == PipeLangLanguageContractV100 || contract == PipeLangLanguageContractV110 || contract == PipeLangLanguageContractV120 || contract == PipeLangLanguageContractV130 || contract == PipeLangLanguageContractV140 || contract == PipeLangLanguageContractV150 || contract == PipeLangLanguageContractV160 || contract == PipeLangLanguageContractV170 || contract == PipeLangLanguageContractV180 || contract == PipeLangLanguageContractV190 || contract == PipeLangLanguageContractV200 || contract == PipeLangLanguageContractV210 || contract == PipeLangLanguageContractV220 || contract == PipeLangLanguageContractV230 || contract == PipeLangLanguageContractV240 || contract == PipeLangLanguageContractV260 || contract == PipeLangLanguageContractV250 || contract == PipeLangLanguageContractV270 || contract == PipeLangLanguageContractV280 || contract == PipeLangLanguageContractV290 || contract == PipeLangLanguageContractV300
	case "*":
		return hir.OperatorMultiply, contract == PipeLangLanguageContractV040 || contract == PipeLangLanguageContractV050 || contract == PipeLangLanguageContractV060 || contract == PipeLangLanguageContractV070 || contract == PipeLangLanguageContractV080 || contract == PipeLangLanguageContractV090 || contract == PipeLangLanguageContractV100 || contract == PipeLangLanguageContractV110 || contract == PipeLangLanguageContractV120 || contract == PipeLangLanguageContractV130 || contract == PipeLangLanguageContractV140 || contract == PipeLangLanguageContractV150 || contract == PipeLangLanguageContractV160 || contract == PipeLangLanguageContractV170 || contract == PipeLangLanguageContractV180 || contract == PipeLangLanguageContractV190 || contract == PipeLangLanguageContractV200 || contract == PipeLangLanguageContractV210 || contract == PipeLangLanguageContractV220 || contract == PipeLangLanguageContractV230 || contract == PipeLangLanguageContractV240 || contract == PipeLangLanguageContractV260 || contract == PipeLangLanguageContractV250 || contract == PipeLangLanguageContractV270 || contract == PipeLangLanguageContractV280 || contract == PipeLangLanguageContractV290 || contract == PipeLangLanguageContractV300
	default:
		return "", false
	}
}

func directOrdinalTextOrderingHIRShape(expression Expr, bindings map[string]hir.Binding, environment map[string]ResolvedTypeRef, returnType ResolvedTypeRef) (*BinaryExpr, bool) {
	binary, ok := expression.(*BinaryExpr)
	if !ok || !isOrdinalTextOrderingOperator(binary.Op) || !returnType.Equal(resolvedPrimitive(TypeBool)) || len(bindings) != 2 {
		return nil, false
	}
	left, leftOK := binary.Left.(*IdentExpr)
	right, rightOK := binary.Right.(*IdentExpr)
	if !leftOK || !rightOK {
		return nil, false
	}
	leftBinding, leftBound := bindings[left.Name]
	rightBinding, rightBound := bindings[right.Name]
	text := resolvedPrimitive(TypeString)
	return binary, leftBound && rightBound && leftBinding.Position == 0 && rightBinding.Position == 1 && environment[left.Name].Equal(text) && environment[right.Name].Equal(text)
}

func lowerExprToHIR(analysis *Analysis, function SemanticIdentity, expression Expr, bindings map[string]hir.Binding, typeEnvironment map[string]ResolvedTypeRef) (hir.Expr, error) {
	var resolved ResolvedTypeRef
	var err error
	if filter, ok := expression.(*ListFilterPredicateExpr); ok {
		resolved, err = analysis.checked.inferExprType(filter.Values, typeEnvironment)
	} else if caller := methodByIdentity(analysis, function); caller != nil && hasNamedRecordPredicateSourceContract(analysis.Modules.LanguageContract()) && len(caller.Params) > 0 {
		returnType, returnErr := analysis.checked.resolveType(caller.ReturnType)
		rowType, rowErr := analysis.checked.resolveType(caller.Params[0].Type)
		if returnErr == nil && rowErr == nil && returnType.Equal(resolvedPrimitive(TypeBool)) && analysis.checked.isResolvedRecordType(rowType) {
			resolved, err = analysis.checked.inferNamedPredicateExprType(expression, typeEnvironment)
		} else {
			resolved, err = analysis.checked.inferExprType(expression, typeEnvironment)
		}
	} else {
		resolved, err = analysis.checked.inferExprType(expression, typeEnvironment)
	}
	if err != nil {
		return hir.Expr{}, err
	}
	result := hir.Expr{Type: toHIRType(analysis, resolved), Span: toHIRSpan(expression.SourceSpan())}
	switch node := expression.(type) {
	case *LiteralExpr:
		result.Kind = hir.ExprLiteral
		result.Literal = &hir.Literal{String: node.Value.String, Int: node.Value.Int, Float: node.Value.Float, Bool: node.Value.Bool}
	case *IdentExpr:
		binding, ok := bindings[node.Name]
		if !ok {
			return hir.Expr{}, hirLoweringError(analysis, node.Span, function, fmt.Sprintf("step-6 pure function references only parameters; %q is owned state", node.Name))
		}
		result.Kind = hir.ExprReference
		bindingCopy := binding
		result.Reference = &bindingCopy
	case *UnaryExpr:
		operand, err := lowerExprToHIR(analysis, function, node.Expr, bindings, typeEnvironment)
		if err != nil {
			return hir.Expr{}, err
		}
		operator, ok := hirUnaryOperator(node.Op)
		if !ok {
			return hir.Expr{}, hirLoweringError(analysis, node.Span, function, fmt.Sprintf("unsupported existing unary operator %q", node.Op))
		}
		result.Kind = hir.ExprUnary
		result.Unary = &hir.Unary{Operator: operator, Operand: &operand}
	case *BinaryExpr:
		left, err := lowerExprToHIR(analysis, function, node.Left, bindings, typeEnvironment)
		if err != nil {
			return hir.Expr{}, err
		}
		right, err := lowerExprToHIR(analysis, function, node.Right, bindings, typeEnvironment)
		if err != nil {
			return hir.Expr{}, err
		}
		operator, ok := hirBinaryOperator(node.Op)
		if !ok {
			return hir.Expr{}, hirLoweringError(analysis, node.Span, function, fmt.Sprintf("unsupported existing binary operator %q", node.Op))
		}
		result.Kind = hir.ExprBinary
		result.Binary = &hir.Binary{Operator: operator, Left: &left, Right: &right}
	case *TextContainsCaseFoldedExpr:
		value, err := lowerExprToHIR(analysis, function, node.Value, bindings, typeEnvironment)
		if err != nil {
			return hir.Expr{}, err
		}
		query, err := lowerExprToHIR(analysis, function, node.Query, bindings, typeEnvironment)
		if err != nil {
			return hir.Expr{}, err
		}
		result.Kind = hir.ExprTextContainsCaseFolded
		result.TextContains = &hir.TextContainsCaseFolded{Value: &value, Query: &query}
	case *TextTrimExpr:
		value, err := lowerExprToHIR(analysis, function, node.Value, bindings, typeEnvironment)
		if err != nil {
			return hir.Expr{}, err
		}
		result.Kind = hir.ExprTextTrim
		result.TextTrim = &hir.TextTrim{Value: &value}
	case *FieldExpr:
		receiverType, err := analysis.checked.inferExprType(node.Receiver, typeEnvironment)
		if err != nil {
			return hir.Expr{}, err
		}
		_, field, position, err := analysis.checked.resolveRecordField(receiverType, node.Name, node.NameSpan)
		if err != nil {
			return hir.Expr{}, err
		}
		receiver, err := lowerExprToHIR(analysis, function, node.Receiver, bindings, typeEnvironment)
		if err != nil {
			return hir.Expr{}, err
		}
		identity, ok := analysis.SemanticIDs.IdentityForSpan(field.Span)
		if !ok {
			return hir.Expr{}, hirLoweringError(analysis, node.NameSpan, function, fmt.Sprintf("record field %q has no semantic identity", node.Name))
		}
		result.Kind = hir.ExprFieldProjection
		result.Field = &hir.FieldProjection{Receiver: &receiver, Identity: toHIRSemanticIdentity(identity), Name: node.Name, Position: position}
	case *RecordConstructExpr:
		constructed, err := analysis.checked.resolveType(node.Type)
		if err != nil {
			return hir.Expr{}, err
		}
		entry, ok := analysis.Symbols.lookupIDEntry(constructed.Symbol)
		if !ok || entry.recordDecl == nil {
			return hir.Expr{}, hirLoweringError(analysis, node.Type.Span, function, "record construction has no resolved record declaration")
		}
		recordIdentity, ok := analysis.SemanticIDs.IdentityForSpan(entry.recordDecl.Span)
		if !ok {
			return hir.Expr{}, hirLoweringError(analysis, node.Type.Span, function, "record construction has no semantic record identity")
		}
		construction := &hir.RecordConstruct{Identity: toHIRSemanticIdentity(recordIdentity), Fields: make([]hir.RecordConstructField, 0, len(node.Fields))}
		for position, initialized := range node.Fields {
			if position >= len(entry.recordDecl.Fields) {
				return hir.Expr{}, hirLoweringError(analysis, initialized.Span, function, "record construction has more fields than its declaration")
			}
			field := entry.recordDecl.Fields[position]
			fieldIdentity, ok := analysis.SemanticIDs.IdentityForSpan(field.Span)
			if !ok {
				return hir.Expr{}, hirLoweringError(analysis, initialized.NameSpan, function, fmt.Sprintf("record field %q has no semantic identity", initialized.Name))
			}
			value, err := lowerExprToHIR(analysis, function, initialized.Value, bindings, typeEnvironment)
			if err != nil {
				return hir.Expr{}, err
			}
			construction.Fields = append(construction.Fields, hir.RecordConstructField{Identity: toHIRSemanticIdentity(fieldIdentity), Name: initialized.Name, Position: position, Value: &value})
		}
		result.Kind = hir.ExprRecordConstruct
		result.Record = construction
	case *OptionalSomeExpr:
		value, err := lowerExprToHIR(analysis, function, node.Value, bindings, typeEnvironment)
		if err != nil {
			return hir.Expr{}, err
		}
		result.Kind = hir.ExprOptionalSome
		result.Some = &hir.OptionalSome{Value: &value}
	case *OptionalNoneExpr:
		result.Kind = hir.ExprOptionalNone
		result.None = &hir.OptionalNone{}
	case *OptionalHasValueExpr:
		value, err := lowerExprToHIR(analysis, function, node.Value, bindings, typeEnvironment)
		if err != nil {
			return hir.Expr{}, err
		}
		result.Kind = hir.ExprOptionalHasValue
		result.HasValue = &hir.OptionalHasValue{Value: &value}
	case *OptionalValueOrExpr:
		value, err := lowerExprToHIR(analysis, function, node.Value, bindings, typeEnvironment)
		if err != nil {
			return hir.Expr{}, err
		}
		fallback, err := lowerExprToHIR(analysis, function, node.Fallback, bindings, typeEnvironment)
		if err != nil {
			return hir.Expr{}, err
		}
		result.Kind = hir.ExprOptionalValueOr
		result.ValueOr = &hir.OptionalValueOr{Value: &value, Fallback: &fallback}
	case *ListEmptyExpr:
		result.Kind = hir.ExprListEmpty
		result.ListEmpty = &hir.ListEmpty{}
	case *ListSingletonExpr:
		value, err := lowerExprToHIR(analysis, function, node.Value, bindings, typeEnvironment)
		if err != nil {
			return hir.Expr{}, err
		}
		result.Kind = hir.ExprListSingleton
		result.ListOne = &hir.ListSingleton{Value: &value}
	case *ListCountExpr:
		value, err := lowerExprToHIR(analysis, function, node.Value, bindings, typeEnvironment)
		if err != nil {
			return hir.Expr{}, err
		}
		result.Kind = hir.ExprListCount
		result.ListCount = &hir.ListCount{Value: &value}
	case *ListAppendExpr:
		values, err := lowerExprToHIR(analysis, function, node.Values, bindings, typeEnvironment)
		if err != nil {
			return hir.Expr{}, err
		}
		value, err := lowerExprToHIR(analysis, function, node.Value, bindings, typeEnvironment)
		if err != nil {
			return hir.Expr{}, err
		}
		result.Kind = hir.ExprListAppend
		result.ListAppend = &hir.ListAppend{Values: &values, Value: &value}
	case *ListAtExpr:
		values, err := lowerExprToHIR(analysis, function, node.Values, bindings, typeEnvironment)
		if err != nil {
			return hir.Expr{}, err
		}
		index, err := lowerExprToHIR(analysis, function, node.Index, bindings, typeEnvironment)
		if err != nil {
			return hir.Expr{}, err
		}
		result.Kind = hir.ExprListAt
		result.ListAt = &hir.ListAt{Values: &values, Index: &index}
	case *ListFindByTextExpr:
		valuesType, err := analysis.checked.inferExprType(node.Values, typeEnvironment)
		if err != nil {
			return hir.Expr{}, err
		}
		field, position, err := analysis.checked.resolveListFindByTextSelector(node, valuesType)
		if err != nil {
			return hir.Expr{}, err
		}
		fieldIdentity, ok := analysis.SemanticIDs.IdentityForSpan(field.Span)
		if !ok {
			return hir.Expr{}, hirLoweringError(analysis, node.FieldSpan, function, fmt.Sprintf("record field %q has no semantic identity", node.Field))
		}
		values, err := lowerExprToHIR(analysis, function, node.Values, bindings, typeEnvironment)
		if err != nil {
			return hir.Expr{}, err
		}
		key, err := lowerExprToHIR(analysis, function, node.Key, bindings, typeEnvironment)
		if err != nil {
			return hir.Expr{}, err
		}
		result.Kind = hir.ExprListFindByText
		result.ListFind = &hir.ListFindByText{Values: &values, Field: toHIRSemanticIdentity(fieldIdentity), Name: node.Field, Position: position, Key: &key}
	case *ListFilterByTextExpr:
		valuesType, err := analysis.checked.inferExprType(node.Values, typeEnvironment)
		if err != nil {
			return hir.Expr{}, err
		}
		field, position, err := analysis.checked.resolveListFilterByTextSelector(node, valuesType)
		if err != nil {
			return hir.Expr{}, err
		}
		fieldIdentity, ok := analysis.SemanticIDs.IdentityForSpan(field.Span)
		if !ok {
			return hir.Expr{}, hirLoweringError(analysis, node.FieldSpan, function, fmt.Sprintf("record field %q has no semantic identity", node.Field))
		}
		values, err := lowerExprToHIR(analysis, function, node.Values, bindings, typeEnvironment)
		if err != nil {
			return hir.Expr{}, err
		}
		key, err := lowerExprToHIR(analysis, function, node.Key, bindings, typeEnvironment)
		if err != nil {
			return hir.Expr{}, err
		}
		result.Kind = hir.ExprListFilterByText
		result.ListFilter = &hir.ListFilterByText{Values: &values, Field: toHIRSemanticIdentity(fieldIdentity), Name: node.Field, Position: position, Key: &key}
	case *ListFilterPredicateExpr:
		caller := methodByIdentity(analysis, function)
		if caller == nil {
			return hir.Expr{}, hirLoweringError(analysis, node.Span, function, "named predicate filter has no owning method")
		}
		predicate, err := analysis.checked.resolveNamedRecordPredicate(*caller, node.Predicate, node.PredicateSpan)
		if err != nil {
			return hir.Expr{}, err
		}
		predicateIdentity, ok := analysis.SemanticIDs.IdentityForSpan(predicate.Span)
		if !ok {
			return hir.Expr{}, hirLoweringError(analysis, node.PredicateSpan, function, fmt.Sprintf("predicate method %q has no semantic identity", node.Predicate))
		}
		values, err := lowerExprToHIR(analysis, function, node.Values, bindings, typeEnvironment)
		if err != nil {
			return hir.Expr{}, err
		}
		arguments := make([]*hir.Expr, 0, len(node.Arguments))
		for _, argument := range node.Arguments {
			lowered, err := lowerExprToHIR(analysis, function, argument, bindings, typeEnvironment)
			if err != nil {
				return hir.Expr{}, err
			}
			arguments = append(arguments, &lowered)
		}
		result.Kind = hir.ExprListFilterPredicate
		result.ListFilterPredicate = &hir.ListFilterPredicate{Values: &values, Predicate: toHIRSemanticIdentity(predicateIdentity), PredicateName: predicate.Name, Arguments: arguments}
	case *ListFilterContainsCaseFoldedExpr:
		valuesType, err := analysis.checked.inferExprType(node.Values, typeEnvironment)
		if err != nil {
			return hir.Expr{}, err
		}
		field, position, err := analysis.checked.resolveListFilterContainsCaseFoldedSelector(node, valuesType)
		if err != nil {
			return hir.Expr{}, err
		}
		fieldIdentity, ok := analysis.SemanticIDs.IdentityForSpan(field.Span)
		if !ok {
			return hir.Expr{}, hirLoweringError(analysis, node.FieldSpan, function, fmt.Sprintf("record field %q has no semantic identity", node.Field))
		}
		values, err := lowerExprToHIR(analysis, function, node.Values, bindings, typeEnvironment)
		if err != nil {
			return hir.Expr{}, err
		}
		query, err := lowerExprToHIR(analysis, function, node.Query, bindings, typeEnvironment)
		if err != nil {
			return hir.Expr{}, err
		}
		result.Kind = hir.ExprListFilterContainsCaseFolded
		result.ListFilterContainsCaseFolded = &hir.ListFilterContainsCaseFolded{Values: &values, Field: toHIRSemanticIdentity(fieldIdentity), Name: node.Field, Position: position, Query: &query}
	case *ListFilterJoinedContainsCaseFoldedExpr:
		valuesType, err := analysis.checked.inferExprType(node.Values, typeEnvironment)
		if err != nil {
			return hir.Expr{}, err
		}
		fields, positions, err := analysis.checked.resolveListFilterJoinedContainsCaseFoldedSelectors(node, valuesType)
		if err != nil {
			return hir.Expr{}, err
		}
		selectors := make([]hir.ListTextFieldSelector, 0, len(fields))
		for index, field := range fields {
			fieldIdentity, ok := analysis.SemanticIDs.IdentityForSpan(field.Span)
			if !ok {
				return hir.Expr{}, hirLoweringError(analysis, node.Selectors[index].FieldSpan, function, fmt.Sprintf("record field %q has no semantic identity", node.Selectors[index].Field))
			}
			selectors = append(selectors, hir.ListTextFieldSelector{Field: toHIRSemanticIdentity(fieldIdentity), Name: node.Selectors[index].Field, Position: positions[index]})
		}
		values, err := lowerExprToHIR(analysis, function, node.Values, bindings, typeEnvironment)
		if err != nil {
			return hir.Expr{}, err
		}
		query, err := lowerExprToHIR(analysis, function, node.Query, bindings, typeEnvironment)
		if err != nil {
			return hir.Expr{}, err
		}
		result.Kind = hir.ExprListFilterJoinedContainsCaseFolded
		result.ListFilterJoinedContainsCaseFolded = &hir.ListFilterJoinedContainsCaseFolded{Values: &values, Selectors: selectors, Query: &query}
	case *ListSortByOrdinalExpr:
		valuesType, err := analysis.checked.inferExprType(node.Values, typeEnvironment)
		if err != nil {
			return hir.Expr{}, err
		}
		field, position, err := analysis.checked.resolveListSortByOrdinalSelector(node, valuesType)
		if err != nil {
			return hir.Expr{}, err
		}
		fieldIdentity, ok := analysis.SemanticIDs.IdentityForSpan(field.Span)
		if !ok {
			return hir.Expr{}, hirLoweringError(analysis, node.FieldSpan, function, fmt.Sprintf("record field %q has no semantic identity", node.Field))
		}
		values, err := lowerExprToHIR(analysis, function, node.Values, bindings, typeEnvironment)
		if err != nil {
			return hir.Expr{}, err
		}
		result.Kind = hir.ExprListSortByOrdinalText
		result.ListSortByOrdinalText = &hir.ListSortByOrdinalText{Values: &values, Field: toHIRSemanticIdentity(fieldIdentity), Name: node.Field, Position: position}
	case *ListSortByOrdinalsExpr:
		valuesType, err := analysis.checked.inferExprType(node.Values, typeEnvironment)
		if err != nil {
			return hir.Expr{}, err
		}
		fields, positions, err := analysis.checked.resolveListSortByOrdinalsSelectors(node, valuesType)
		if err != nil {
			return hir.Expr{}, err
		}
		selectors := make([]hir.ListTextFieldSelector, 0, len(fields))
		for index, field := range fields {
			fieldIdentity, ok := analysis.SemanticIDs.IdentityForSpan(field.Span)
			if !ok {
				return hir.Expr{}, hirLoweringError(analysis, node.Selectors[index].FieldSpan, function, fmt.Sprintf("record field %q has no semantic identity", node.Selectors[index].Field))
			}
			selectors = append(selectors, hir.ListTextFieldSelector{Field: toHIRSemanticIdentity(fieldIdentity), Name: node.Selectors[index].Field, Position: positions[index]})
		}
		values, err := lowerExprToHIR(analysis, function, node.Values, bindings, typeEnvironment)
		if err != nil {
			return hir.Expr{}, err
		}
		result.Kind = hir.ExprListSortByOrdinalTexts
		result.ListSortByOrdinalTexts = &hir.ListSortByOrdinalTexts{Values: &values, Selectors: selectors}
	case *ResultOKExpr:
		value, err := lowerExprToHIR(analysis, function, node.Value, bindings, typeEnvironment)
		if err != nil {
			return hir.Expr{}, err
		}
		result.Kind = hir.ExprResultOK
		result.ResultOK = &hir.ResultOK{Value: &value}
	case *ResultErrExpr:
		failure, err := lowerExprToHIR(analysis, function, node.Error, bindings, typeEnvironment)
		if err != nil {
			return hir.Expr{}, err
		}
		result.Kind = hir.ExprResultErr
		result.ResultErr = &hir.ResultErr{Error: &failure}
	case *ResultIsOKExpr:
		value, err := lowerExprToHIR(analysis, function, node.Value, bindings, typeEnvironment)
		if err != nil {
			return hir.Expr{}, err
		}
		result.Kind = hir.ExprResultIsOK
		result.ResultIsOK = &hir.ResultIsOK{Value: &value}
	case *ResultSuccessOrExpr:
		value, err := lowerExprToHIR(analysis, function, node.Value, bindings, typeEnvironment)
		if err != nil {
			return hir.Expr{}, err
		}
		fallback, err := lowerExprToHIR(analysis, function, node.Fallback, bindings, typeEnvironment)
		if err != nil {
			return hir.Expr{}, err
		}
		result.Kind = hir.ExprResultSuccessOr
		result.SuccessOr = &hir.ResultSuccessOr{Value: &value, Fallback: &fallback}
	case *ResultFailureOrExpr:
		value, err := lowerExprToHIR(analysis, function, node.Value, bindings, typeEnvironment)
		if err != nil {
			return hir.Expr{}, err
		}
		fallback, err := lowerExprToHIR(analysis, function, node.Fallback, bindings, typeEnvironment)
		if err != nil {
			return hir.Expr{}, err
		}
		result.Kind = hir.ExprResultFailureOr
		result.FailureOr = &hir.ResultFailureOr{Value: &value, Fallback: &fallback}
	default:
		return hir.Expr{}, hirLoweringError(analysis, expression.SourceSpan(), function, "unsupported checked expression")
	}
	return result, nil
}

// LowerHIRToCore removes analysis-local ownership and source information while
// retaining semantic function identity, normalized types, parameters, values,
// references, and operators in backend-neutral Core IR.
func LowerHIRToCore(program hir.Program) (coreir.Program, error) {
	core := coreir.Program{LanguageContract: program.LanguageContract, CompilerContract: program.CompilerContract, Functions: make([]coreir.Function, 0, len(program.Functions))}
	for _, function := range program.Functions {
		if program.LanguageContract != coreir.LanguageContractV270 && program.LanguageContract != coreir.LanguageContractV280 && program.LanguageContract != coreir.LanguageContractV290 && program.LanguageContract != coreir.LanguageContractV300 && program.LanguageContract != coreir.LanguageContractV310 {
			if program.LanguageContract != coreir.LanguageContractV130 && program.LanguageContract != coreir.LanguageContractV140 && program.LanguageContract != coreir.LanguageContractV150 && program.LanguageContract != coreir.LanguageContractV160 && program.LanguageContract != coreir.LanguageContractV170 && program.LanguageContract != coreir.LanguageContractV180 && program.LanguageContract != coreir.LanguageContractV190 && program.LanguageContract != coreir.LanguageContractV200 && program.LanguageContract != coreir.LanguageContractV210 && program.LanguageContract != coreir.LanguageContractV220 && program.LanguageContract != coreir.LanguageContractV230 && program.LanguageContract != coreir.LanguageContractV240 && program.LanguageContract != coreir.LanguageContractV250 && program.LanguageContract != coreir.LanguageContractV260 && hirFunctionContainsOptional(function) {
				return coreir.Program{}, coreLoweringError(function.Span, fmt.Sprintf("primitive Optional HIR requires language contract %q", coreir.LanguageContractV130))
			}
			if program.LanguageContract != coreir.LanguageContractV140 && program.LanguageContract != coreir.LanguageContractV150 && program.LanguageContract != coreir.LanguageContractV160 && program.LanguageContract != coreir.LanguageContractV170 && program.LanguageContract != coreir.LanguageContractV180 && program.LanguageContract != coreir.LanguageContractV190 && program.LanguageContract != coreir.LanguageContractV200 && program.LanguageContract != coreir.LanguageContractV210 && program.LanguageContract != coreir.LanguageContractV220 && program.LanguageContract != coreir.LanguageContractV230 && program.LanguageContract != coreir.LanguageContractV240 && program.LanguageContract != coreir.LanguageContractV250 && program.LanguageContract != coreir.LanguageContractV260 && hirExprContainsOptionalDefault(function.Body) {
				return coreir.Program{}, coreLoweringError(function.Span, fmt.Sprintf("primitive Optional defaulting HIR requires language contract %q", coreir.LanguageContractV140))
			}
			if program.LanguageContract != coreir.LanguageContractV150 && program.LanguageContract != coreir.LanguageContractV160 && program.LanguageContract != coreir.LanguageContractV170 && program.LanguageContract != coreir.LanguageContractV180 && program.LanguageContract != coreir.LanguageContractV190 && program.LanguageContract != coreir.LanguageContractV200 && program.LanguageContract != coreir.LanguageContractV210 && program.LanguageContract != coreir.LanguageContractV220 && program.LanguageContract != coreir.LanguageContractV230 && program.LanguageContract != coreir.LanguageContractV240 && program.LanguageContract != coreir.LanguageContractV250 && program.LanguageContract != coreir.LanguageContractV260 && hirFunctionContainsList(function) {
				return coreir.Program{}, coreLoweringError(function.Span, fmt.Sprintf("record-list HIR requires language contract %q", coreir.LanguageContractV150))
			}
			if program.LanguageContract != coreir.LanguageContractV160 && program.LanguageContract != coreir.LanguageContractV170 && program.LanguageContract != coreir.LanguageContractV180 && program.LanguageContract != coreir.LanguageContractV190 && program.LanguageContract != coreir.LanguageContractV200 && program.LanguageContract != coreir.LanguageContractV210 && program.LanguageContract != coreir.LanguageContractV220 && program.LanguageContract != coreir.LanguageContractV230 && program.LanguageContract != coreir.LanguageContractV240 && program.LanguageContract != coreir.LanguageContractV250 && program.LanguageContract != coreir.LanguageContractV260 && hirExprContainsListCount(function.Body) {
				return coreir.Program{}, coreLoweringError(function.Span, fmt.Sprintf("record-list count HIR requires language contract %q", coreir.LanguageContractV160))
			}
			if program.LanguageContract != coreir.LanguageContractV170 && program.LanguageContract != coreir.LanguageContractV180 && program.LanguageContract != coreir.LanguageContractV190 && program.LanguageContract != coreir.LanguageContractV200 && program.LanguageContract != coreir.LanguageContractV210 && program.LanguageContract != coreir.LanguageContractV220 && program.LanguageContract != coreir.LanguageContractV230 && program.LanguageContract != coreir.LanguageContractV240 && program.LanguageContract != coreir.LanguageContractV250 && program.LanguageContract != coreir.LanguageContractV260 && hirExprContainsListAppend(function.Body) {
				return coreir.Program{}, coreLoweringError(function.Span, fmt.Sprintf("record-list append HIR requires language contract %q", coreir.LanguageContractV170))
			}
			if program.LanguageContract != coreir.LanguageContractV180 && program.LanguageContract != coreir.LanguageContractV190 && program.LanguageContract != coreir.LanguageContractV200 && program.LanguageContract != coreir.LanguageContractV210 && program.LanguageContract != coreir.LanguageContractV220 && program.LanguageContract != coreir.LanguageContractV230 && program.LanguageContract != coreir.LanguageContractV240 && program.LanguageContract != coreir.LanguageContractV250 && program.LanguageContract != coreir.LanguageContractV260 && hirFunctionContainsRecordOptional(function) {
				return coreir.Program{}, coreLoweringError(function.Span, fmt.Sprintf("primitive-record Optional HIR requires language contract %q", coreir.LanguageContractV180))
			}
			if program.LanguageContract != coreir.LanguageContractV190 && program.LanguageContract != coreir.LanguageContractV200 && program.LanguageContract != coreir.LanguageContractV210 && program.LanguageContract != coreir.LanguageContractV220 && program.LanguageContract != coreir.LanguageContractV230 && program.LanguageContract != coreir.LanguageContractV240 && program.LanguageContract != coreir.LanguageContractV250 && program.LanguageContract != coreir.LanguageContractV260 && hirFunctionContainsBoundedValueResult(function) {
				return coreir.Program{}, coreLoweringError(function.Span, fmt.Sprintf("bounded Result HIR requires language contract %q", coreir.LanguageContractV190))
			}
			if program.LanguageContract != coreir.LanguageContractV250 && program.LanguageContract != coreir.LanguageContractV260 && hirFunctionContainsTextResult(function) {
				return coreir.Program{}, coreLoweringError(function.Span, fmt.Sprintf("text Result HIR requires language contract %q", coreir.LanguageContractV250))
			}
			if program.LanguageContract != coreir.LanguageContractV260 && hirExprContainsTextTrim(function.Body) {
				return coreir.Program{}, coreLoweringError(function.Span, fmt.Sprintf("trim HIR requires language contract %q", coreir.LanguageContractV260))
			}
			if program.LanguageContract != coreir.LanguageContractV200 && program.LanguageContract != coreir.LanguageContractV210 && program.LanguageContract != coreir.LanguageContractV220 && program.LanguageContract != coreir.LanguageContractV230 && program.LanguageContract != coreir.LanguageContractV240 && program.LanguageContract != coreir.LanguageContractV250 && program.LanguageContract != coreir.LanguageContractV260 && hirExprContainsListAt(function.Body) {
				return coreir.Program{}, coreLoweringError(function.Span, fmt.Sprintf("record-list at HIR requires language contract %q", coreir.LanguageContractV200))
			}
			if program.LanguageContract != coreir.LanguageContractV210 && program.LanguageContract != coreir.LanguageContractV220 && program.LanguageContract != coreir.LanguageContractV230 && program.LanguageContract != coreir.LanguageContractV240 && program.LanguageContract != coreir.LanguageContractV250 && program.LanguageContract != coreir.LanguageContractV260 && hirExprContainsListFindByText(function.Body) {
				return coreir.Program{}, coreLoweringError(function.Span, fmt.Sprintf("record-list find_by HIR requires language contract %q", coreir.LanguageContractV210))
			}
			if program.LanguageContract != coreir.LanguageContractV220 && program.LanguageContract != coreir.LanguageContractV230 && program.LanguageContract != coreir.LanguageContractV240 && program.LanguageContract != coreir.LanguageContractV250 && program.LanguageContract != coreir.LanguageContractV260 && hirExprContainsListFilterByText(function.Body) {
				return coreir.Program{}, coreLoweringError(function.Span, fmt.Sprintf("record-list filter_by HIR requires language contract %q", coreir.LanguageContractV220))
			}
			if program.LanguageContract != coreir.LanguageContractV230 && program.LanguageContract != coreir.LanguageContractV240 && program.LanguageContract != coreir.LanguageContractV250 && program.LanguageContract != coreir.LanguageContractV260 && hirExprContainsTextCaseFold(function.Body) {
				return coreir.Program{}, coreLoweringError(function.Span, fmt.Sprintf("contains_casefolded HIR requires language contract %q", coreir.LanguageContractV230))
			}
			if program.LanguageContract != coreir.LanguageContractV240 && program.LanguageContract != coreir.LanguageContractV250 && program.LanguageContract != coreir.LanguageContractV260 && hirExprContainsListFilterContainsCaseFolded(function.Body) {
				return coreir.Program{}, coreLoweringError(function.Span, fmt.Sprintf("record-list filter_contains_casefolded HIR requires language contract %q", coreir.LanguageContractV240))
			}
		}
		if program.LanguageContract != coreir.LanguageContractV270 && program.LanguageContract != coreir.LanguageContractV280 && program.LanguageContract != coreir.LanguageContractV290 && program.LanguageContract != coreir.LanguageContractV300 && program.LanguageContract != coreir.LanguageContractV310 && hirExprContainsListFilterJoinedContainsCaseFolded(function.Body) {
			return coreir.Program{}, coreLoweringError(function.Span, fmt.Sprintf("record-list filter_joined_contains_casefolded HIR requires language contract %q", coreir.LanguageContractV270))
		}
		if program.LanguageContract != coreir.LanguageContractV290 && program.LanguageContract != coreir.LanguageContractV300 && program.LanguageContract != coreir.LanguageContractV310 && function.Body.Kind == hir.ExprListFilterJoinedContainsCaseFolded && (function.Body.ListFilterJoinedContainsCaseFolded == nil || len(function.Body.ListFilterJoinedContainsCaseFolded.Selectors) != 5) {
			return coreir.Program{}, coreLoweringError(function.Span, "record-list filter_joined_contains_casefolded HIR requires exactly five selectors before language contract v0.29.0")
		}
		if program.LanguageContract != coreir.LanguageContractV280 && program.LanguageContract != coreir.LanguageContractV290 && program.LanguageContract != coreir.LanguageContractV300 && program.LanguageContract != coreir.LanguageContractV310 && hirExprContainsListSortByOrdinalText(function.Body) {
			return coreir.Program{}, coreLoweringError(function.Span, fmt.Sprintf("record-list sort_by_ordinal HIR requires language contract %q", coreir.LanguageContractV280))
		}
		if program.LanguageContract != coreir.LanguageContractV300 && program.LanguageContract != coreir.LanguageContractV310 && hirExprContainsListSortByOrdinalTexts(function.Body) {
			return coreir.Program{}, coreLoweringError(function.Span, fmt.Sprintf("multi-key record-list sort_by_ordinal HIR requires language contract %q", coreir.LanguageContractV300))
		}
		if program.LanguageContract != coreir.LanguageContractV310 && function.Body.Kind == hir.ExprListFilterPredicate {
			return coreir.Program{}, coreLoweringError(function.Span, fmt.Sprintf("named record predicate filter HIR requires language contract %q", coreir.LanguageContractV310))
		}
		if function.Identity.PackageID == "" || function.Identity.Path == "" || function.Owner.SymbolID == 0 || function.Owner.Module == "" {
			return coreir.Program{}, coreLoweringError(function.Span, "typed HIR function is missing bound semantic ownership")
		}
		parameters := make([]coreir.Parameter, 0, len(function.Parameters))
		for position, parameter := range function.Parameters {
			if parameter.Binding.Kind != hir.BindingParameter || parameter.Binding.Position != position || parameter.Binding.Function.PackageID != function.Identity.PackageID || parameter.Binding.Function.Path != function.Identity.Path {
				return coreir.Program{}, coreLoweringError(parameter.Span, "typed HIR parameter binding is not normalized")
			}
			parameters = append(parameters, coreir.Parameter{Position: position, Name: parameter.Binding.Name, Type: hirTypeToCore(parameter.Type)})
		}
		body, err := hirExprToCore(function.Body, function.Parameters)
		if err != nil {
			return coreir.Program{}, err
		}
		lowered := coreir.Function{Identity: hirIdentityToCore(function.Identity), Name: function.Name, Parameters: parameters, ReturnType: hirTypeToCore(function.ReturnType), Body: body}
		if err := coreir.ValidateFunction(lowered); err != nil {
			return coreir.Program{}, coreLoweringError(function.Span, err.Error())
		}
		core.Functions = append(core.Functions, lowered)
	}
	if err := coreir.ValidateProgram(core); err != nil {
		return coreir.Program{}, coreLoweringError(hir.SourceSpan{}, err.Error())
	}
	return core, nil
}

func hirFunctionContainsList(function hir.Function) bool {
	if function.ReturnType.Kind == hir.TypeList || hirExprContainsList(function.Body) {
		return true
	}
	for _, parameter := range function.Parameters {
		if parameter.Type.Kind == hir.TypeList {
			return true
		}
	}
	return false
}

func hirExprContainsTextCaseFold(expression hir.Expr) bool {
	switch expression.Kind {
	case hir.ExprTextContainsCaseFolded:
		return true
	case hir.ExprUnary:
		return expression.Unary != nil && expression.Unary.Operand != nil && hirExprContainsTextCaseFold(*expression.Unary.Operand)
	case hir.ExprBinary:
		return expression.Binary != nil && expression.Binary.Left != nil && expression.Binary.Right != nil && (hirExprContainsTextCaseFold(*expression.Binary.Left) || hirExprContainsTextCaseFold(*expression.Binary.Right))
	case hir.ExprFieldProjection:
		return expression.Field != nil && expression.Field.Receiver != nil && hirExprContainsTextCaseFold(*expression.Field.Receiver)
	case hir.ExprRecordConstruct:
		if expression.Record != nil {
			for _, field := range expression.Record.Fields {
				if field.Value != nil && hirExprContainsTextCaseFold(*field.Value) {
					return true
				}
			}
		}
	}
	return false
}

func hirExprContainsTextTrim(expression hir.Expr) bool {
	if expression.Kind == hir.ExprTextTrim {
		return true
	}
	children := []*hir.Expr{}
	switch expression.Kind {
	case hir.ExprUnary:
		if expression.Unary != nil {
			children = append(children, expression.Unary.Operand)
		}
	case hir.ExprBinary:
		if expression.Binary != nil {
			children = append(children, expression.Binary.Left, expression.Binary.Right)
		}
	case hir.ExprTextContainsCaseFolded:
		if expression.TextContains != nil {
			children = append(children, expression.TextContains.Value, expression.TextContains.Query)
		}
	case hir.ExprFieldProjection:
		if expression.Field != nil {
			children = append(children, expression.Field.Receiver)
		}
	case hir.ExprRecordConstruct:
		if expression.Record != nil {
			for _, field := range expression.Record.Fields {
				children = append(children, field.Value)
			}
		}
	case hir.ExprOptionalSome:
		if expression.Some != nil {
			children = append(children, expression.Some.Value)
		}
	case hir.ExprOptionalHasValue:
		if expression.HasValue != nil {
			children = append(children, expression.HasValue.Value)
		}
	case hir.ExprOptionalValueOr:
		if expression.ValueOr != nil {
			children = append(children, expression.ValueOr.Value, expression.ValueOr.Fallback)
		}
	case hir.ExprListSingleton:
		if expression.ListOne != nil {
			children = append(children, expression.ListOne.Value)
		}
	case hir.ExprListCount:
		if expression.ListCount != nil {
			children = append(children, expression.ListCount.Value)
		}
	case hir.ExprListAppend:
		if expression.ListAppend != nil {
			children = append(children, expression.ListAppend.Values, expression.ListAppend.Value)
		}
	case hir.ExprListAt:
		if expression.ListAt != nil {
			children = append(children, expression.ListAt.Values, expression.ListAt.Index)
		}
	case hir.ExprListFindByText:
		if expression.ListFind != nil {
			children = append(children, expression.ListFind.Values, expression.ListFind.Key)
		}
	case hir.ExprListFilterByText:
		if expression.ListFilter != nil {
			children = append(children, expression.ListFilter.Values, expression.ListFilter.Key)
		}
	case hir.ExprListFilterPredicate:
		if expression.ListFilterPredicate != nil {
			children = append(children, expression.ListFilterPredicate.Values)
			children = append(children, expression.ListFilterPredicate.Arguments...)
		}
	case hir.ExprListFilterContainsCaseFolded:
		if expression.ListFilterContainsCaseFolded != nil {
			children = append(children, expression.ListFilterContainsCaseFolded.Values, expression.ListFilterContainsCaseFolded.Query)
		}
	case hir.ExprResultOK:
		if expression.ResultOK != nil {
			children = append(children, expression.ResultOK.Value)
		}
	case hir.ExprResultErr:
		if expression.ResultErr != nil {
			children = append(children, expression.ResultErr.Error)
		}
	case hir.ExprResultIsOK:
		if expression.ResultIsOK != nil {
			children = append(children, expression.ResultIsOK.Value)
		}
	case hir.ExprResultSuccessOr:
		if expression.SuccessOr != nil {
			children = append(children, expression.SuccessOr.Value, expression.SuccessOr.Fallback)
		}
	case hir.ExprResultFailureOr:
		if expression.FailureOr != nil {
			children = append(children, expression.FailureOr.Value, expression.FailureOr.Fallback)
		}
	}
	for _, child := range children {
		if child != nil && hirExprContainsTextTrim(*child) {
			return true
		}
	}
	return false
}

func hirFunctionContainsBoundedValueResult(function hir.Function) bool {
	if hirTypeContainsBoundedValueResult(function.ReturnType) || hirExprContainsBoundedValueResult(function.Body) {
		return true
	}
	for _, parameter := range function.Parameters {
		if hirTypeContainsBoundedValueResult(parameter.Type) {
			return true
		}
	}
	return false
}

func hirFunctionContainsTextResult(function hir.Function) bool {
	if hirTypeContainsTextResult(function.ReturnType) || hirExprContainsTextResult(function.Body) {
		return true
	}
	for _, parameter := range function.Parameters {
		if hirTypeContainsTextResult(parameter.Type) {
			return true
		}
	}
	return false
}

func hirTypeContainsTextResult(value hir.Type) bool {
	return value.Kind == hir.TypeResult && value.Result != nil && value.Result.Success.Kind == hir.TypePrimitive && value.Result.Success.Primitive == hir.PrimitiveString && value.Result.Failure.Kind == hir.TypePrimitive && value.Result.Failure.Primitive == hir.PrimitiveString
}

func hirExprContainsTextResult(expression hir.Expr) bool {
	if hirTypeContainsTextResult(expression.Type) {
		return true
	}
	switch expression.Kind {
	case hir.ExprResultOK, hir.ExprResultErr:
		return hirTypeContainsTextResult(expression.Type)
	case hir.ExprResultIsOK:
		return expression.ResultIsOK != nil && expression.ResultIsOK.Value != nil && hirTypeContainsTextResult(expression.ResultIsOK.Value.Type)
	case hir.ExprResultSuccessOr:
		return expression.SuccessOr != nil && expression.SuccessOr.Value != nil && hirTypeContainsTextResult(expression.SuccessOr.Value.Type)
	case hir.ExprResultFailureOr:
		return expression.FailureOr != nil && expression.FailureOr.Value != nil && hirTypeContainsTextResult(expression.FailureOr.Value.Type)
	case hir.ExprUnary:
		return expression.Unary != nil && expression.Unary.Operand != nil && hirExprContainsTextResult(*expression.Unary.Operand)
	case hir.ExprBinary:
		return expression.Binary != nil && expression.Binary.Left != nil && expression.Binary.Right != nil && (hirExprContainsTextResult(*expression.Binary.Left) || hirExprContainsTextResult(*expression.Binary.Right))
	case hir.ExprFieldProjection:
		return expression.Field != nil && expression.Field.Receiver != nil && hirExprContainsTextResult(*expression.Field.Receiver)
	}
	return false
}

func hirTypeContainsBoundedValueResult(value hir.Type) bool {
	if value.Kind != hir.TypeResult || value.Result == nil || value.Result.Failure.Kind != hir.TypePrimitive || value.Result.Failure.Primitive != hir.PrimitiveString {
		return false
	}
	return value.Result.Success.Kind == hir.TypeList || (value.Result.Success.Kind == hir.TypePrimitive && value.Result.Success.Primitive == hir.PrimitiveString)
}

func hirExprContainsBoundedValueResult(expression hir.Expr) bool {
	if hirTypeContainsBoundedValueResult(expression.Type) {
		return true
	}
	switch expression.Kind {
	case hir.ExprResultOK, hir.ExprResultErr, hir.ExprResultIsOK, hir.ExprResultSuccessOr, hir.ExprResultFailureOr:
		return true
	case hir.ExprUnary:
		return expression.Unary != nil && expression.Unary.Operand != nil && hirExprContainsBoundedValueResult(*expression.Unary.Operand)
	case hir.ExprBinary:
		return expression.Binary != nil && expression.Binary.Left != nil && expression.Binary.Right != nil && (hirExprContainsBoundedValueResult(*expression.Binary.Left) || hirExprContainsBoundedValueResult(*expression.Binary.Right))
	case hir.ExprFieldProjection:
		return expression.Field != nil && expression.Field.Receiver != nil && hirExprContainsBoundedValueResult(*expression.Field.Receiver)
	}
	return false
}

func hirExprContainsList(expression hir.Expr) bool {
	switch expression.Kind {
	case hir.ExprListEmpty, hir.ExprListSingleton, hir.ExprListCount, hir.ExprListAppend, hir.ExprListAt, hir.ExprListFindByText, hir.ExprListFilterByText, hir.ExprListFilterPredicate, hir.ExprListFilterContainsCaseFolded, hir.ExprListFilterJoinedContainsCaseFolded, hir.ExprListSortByOrdinalText, hir.ExprListSortByOrdinalTexts:
		return true
	case hir.ExprUnary:
		return expression.Unary != nil && expression.Unary.Operand != nil && hirExprContainsList(*expression.Unary.Operand)
	case hir.ExprBinary:
		return expression.Binary != nil && expression.Binary.Left != nil && expression.Binary.Right != nil && (hirExprContainsList(*expression.Binary.Left) || hirExprContainsList(*expression.Binary.Right))
	case hir.ExprFieldProjection:
		return expression.Field != nil && expression.Field.Receiver != nil && hirExprContainsList(*expression.Field.Receiver)
	case hir.ExprRecordConstruct:
		if expression.Record != nil {
			for _, field := range expression.Record.Fields {
				if field.Value != nil && hirExprContainsList(*field.Value) {
					return true
				}
			}
		}
	}
	return false
}

func hirExprContainsListCount(expression hir.Expr) bool {
	switch expression.Kind {
	case hir.ExprListCount:
		return true
	case hir.ExprUnary:
		return expression.Unary != nil && expression.Unary.Operand != nil && hirExprContainsListCount(*expression.Unary.Operand)
	case hir.ExprBinary:
		return expression.Binary != nil && expression.Binary.Left != nil && expression.Binary.Right != nil && (hirExprContainsListCount(*expression.Binary.Left) || hirExprContainsListCount(*expression.Binary.Right))
	case hir.ExprFieldProjection:
		return expression.Field != nil && expression.Field.Receiver != nil && hirExprContainsListCount(*expression.Field.Receiver)
	case hir.ExprRecordConstruct:
		if expression.Record != nil {
			for _, field := range expression.Record.Fields {
				if field.Value != nil && hirExprContainsListCount(*field.Value) {
					return true
				}
			}
		}
	}
	return false
}

func hirExprContainsListAppend(expression hir.Expr) bool {
	switch expression.Kind {
	case hir.ExprListAppend:
		return true
	case hir.ExprUnary:
		return expression.Unary != nil && expression.Unary.Operand != nil && hirExprContainsListAppend(*expression.Unary.Operand)
	case hir.ExprBinary:
		return expression.Binary != nil && expression.Binary.Left != nil && expression.Binary.Right != nil && (hirExprContainsListAppend(*expression.Binary.Left) || hirExprContainsListAppend(*expression.Binary.Right))
	case hir.ExprFieldProjection:
		return expression.Field != nil && expression.Field.Receiver != nil && hirExprContainsListAppend(*expression.Field.Receiver)
	case hir.ExprRecordConstruct:
		if expression.Record != nil {
			for _, field := range expression.Record.Fields {
				if field.Value != nil && hirExprContainsListAppend(*field.Value) {
					return true
				}
			}
		}
	}
	return false
}

func hirExprContainsListAt(expression hir.Expr) bool {
	switch expression.Kind {
	case hir.ExprListAt:
		return true
	case hir.ExprUnary:
		return expression.Unary != nil && expression.Unary.Operand != nil && hirExprContainsListAt(*expression.Unary.Operand)
	case hir.ExprBinary:
		return expression.Binary != nil && expression.Binary.Left != nil && expression.Binary.Right != nil && (hirExprContainsListAt(*expression.Binary.Left) || hirExprContainsListAt(*expression.Binary.Right))
	case hir.ExprFieldProjection:
		return expression.Field != nil && expression.Field.Receiver != nil && hirExprContainsListAt(*expression.Field.Receiver)
	case hir.ExprRecordConstruct:
		if expression.Record != nil {
			for _, field := range expression.Record.Fields {
				if field.Value != nil && hirExprContainsListAt(*field.Value) {
					return true
				}
			}
		}
	}
	return false
}

func hirExprContainsListFindByText(expression hir.Expr) bool {
	switch expression.Kind {
	case hir.ExprListFindByText:
		return true
	case hir.ExprUnary:
		return expression.Unary != nil && expression.Unary.Operand != nil && hirExprContainsListFindByText(*expression.Unary.Operand)
	case hir.ExprBinary:
		return expression.Binary != nil && expression.Binary.Left != nil && expression.Binary.Right != nil && (hirExprContainsListFindByText(*expression.Binary.Left) || hirExprContainsListFindByText(*expression.Binary.Right))
	case hir.ExprFieldProjection:
		return expression.Field != nil && expression.Field.Receiver != nil && hirExprContainsListFindByText(*expression.Field.Receiver)
	case hir.ExprRecordConstruct:
		if expression.Record != nil {
			for _, field := range expression.Record.Fields {
				if field.Value != nil && hirExprContainsListFindByText(*field.Value) {
					return true
				}
			}
		}
	}
	return false
}

func hirExprContainsListFilterByText(expression hir.Expr) bool {
	switch expression.Kind {
	case hir.ExprListFilterByText:
		return true
	case hir.ExprUnary:
		return expression.Unary != nil && expression.Unary.Operand != nil && hirExprContainsListFilterByText(*expression.Unary.Operand)
	case hir.ExprBinary:
		return expression.Binary != nil && expression.Binary.Left != nil && expression.Binary.Right != nil && (hirExprContainsListFilterByText(*expression.Binary.Left) || hirExprContainsListFilterByText(*expression.Binary.Right))
	case hir.ExprFieldProjection:
		return expression.Field != nil && expression.Field.Receiver != nil && hirExprContainsListFilterByText(*expression.Field.Receiver)
	case hir.ExprRecordConstruct:
		if expression.Record != nil {
			for _, field := range expression.Record.Fields {
				if field.Value != nil && hirExprContainsListFilterByText(*field.Value) {
					return true
				}
			}
		}
	}
	return false
}

func hirExprContainsListFilterContainsCaseFolded(expression hir.Expr) bool {
	switch expression.Kind {
	case hir.ExprListFilterContainsCaseFolded:
		return true
	case hir.ExprUnary:
		return expression.Unary != nil && expression.Unary.Operand != nil && hirExprContainsListFilterContainsCaseFolded(*expression.Unary.Operand)
	case hir.ExprBinary:
		return expression.Binary != nil && expression.Binary.Left != nil && expression.Binary.Right != nil && (hirExprContainsListFilterContainsCaseFolded(*expression.Binary.Left) || hirExprContainsListFilterContainsCaseFolded(*expression.Binary.Right))
	case hir.ExprFieldProjection:
		return expression.Field != nil && expression.Field.Receiver != nil && hirExprContainsListFilterContainsCaseFolded(*expression.Field.Receiver)
	case hir.ExprRecordConstruct:
		if expression.Record != nil {
			for _, field := range expression.Record.Fields {
				if field.Value != nil && hirExprContainsListFilterContainsCaseFolded(*field.Value) {
					return true
				}
			}
		}
	}
	return false
}

func hirExprContainsListFilterJoinedContainsCaseFolded(expression hir.Expr) bool {
	if expression.Kind == hir.ExprListFilterJoinedContainsCaseFolded {
		return true
	}
	children := []*hir.Expr{}
	switch expression.Kind {
	case hir.ExprUnary:
		if expression.Unary != nil {
			children = append(children, expression.Unary.Operand)
		}
	case hir.ExprBinary:
		if expression.Binary != nil {
			children = append(children, expression.Binary.Left, expression.Binary.Right)
		}
	case hir.ExprTextContainsCaseFolded:
		if expression.TextContains != nil {
			children = append(children, expression.TextContains.Value, expression.TextContains.Query)
		}
	case hir.ExprTextTrim:
		if expression.TextTrim != nil {
			children = append(children, expression.TextTrim.Value)
		}
	case hir.ExprFieldProjection:
		if expression.Field != nil {
			children = append(children, expression.Field.Receiver)
		}
	case hir.ExprRecordConstruct:
		if expression.Record != nil {
			for _, field := range expression.Record.Fields {
				children = append(children, field.Value)
			}
		}
	case hir.ExprOptionalSome:
		if expression.Some != nil {
			children = append(children, expression.Some.Value)
		}
	case hir.ExprOptionalHasValue:
		if expression.HasValue != nil {
			children = append(children, expression.HasValue.Value)
		}
	case hir.ExprOptionalValueOr:
		if expression.ValueOr != nil {
			children = append(children, expression.ValueOr.Value, expression.ValueOr.Fallback)
		}
	case hir.ExprListSingleton:
		if expression.ListOne != nil {
			children = append(children, expression.ListOne.Value)
		}
	case hir.ExprListCount:
		if expression.ListCount != nil {
			children = append(children, expression.ListCount.Value)
		}
	case hir.ExprListAppend:
		if expression.ListAppend != nil {
			children = append(children, expression.ListAppend.Values, expression.ListAppend.Value)
		}
	case hir.ExprListAt:
		if expression.ListAt != nil {
			children = append(children, expression.ListAt.Values, expression.ListAt.Index)
		}
	case hir.ExprListFindByText:
		if expression.ListFind != nil {
			children = append(children, expression.ListFind.Values, expression.ListFind.Key)
		}
	case hir.ExprListFilterByText:
		if expression.ListFilter != nil {
			children = append(children, expression.ListFilter.Values, expression.ListFilter.Key)
		}
	case hir.ExprListFilterPredicate:
		if expression.ListFilterPredicate != nil {
			children = append(children, expression.ListFilterPredicate.Values)
			children = append(children, expression.ListFilterPredicate.Arguments...)
		}
	case hir.ExprListFilterContainsCaseFolded:
		if expression.ListFilterContainsCaseFolded != nil {
			children = append(children, expression.ListFilterContainsCaseFolded.Values, expression.ListFilterContainsCaseFolded.Query)
		}
	case hir.ExprResultOK:
		if expression.ResultOK != nil {
			children = append(children, expression.ResultOK.Value)
		}
	case hir.ExprResultErr:
		if expression.ResultErr != nil {
			children = append(children, expression.ResultErr.Error)
		}
	case hir.ExprResultIsOK:
		if expression.ResultIsOK != nil {
			children = append(children, expression.ResultIsOK.Value)
		}
	case hir.ExprResultSuccessOr:
		if expression.SuccessOr != nil {
			children = append(children, expression.SuccessOr.Value, expression.SuccessOr.Fallback)
		}
	case hir.ExprResultFailureOr:
		if expression.FailureOr != nil {
			children = append(children, expression.FailureOr.Value, expression.FailureOr.Fallback)
		}
	}
	for _, child := range children {
		if child != nil && hirExprContainsListFilterJoinedContainsCaseFolded(*child) {
			return true
		}
	}
	return false
}

func hirExprContainsListSortByOrdinalText(expression hir.Expr) bool {
	return expression.Kind == hir.ExprListSortByOrdinalText
}

func hirExprContainsListSortByOrdinalTexts(expression hir.Expr) bool {
	return expression.Kind == hir.ExprListSortByOrdinalTexts
}

func hirFunctionContainsOptional(function hir.Function) bool {
	if function.ReturnType.Kind == hir.TypeOptional || hirExprContainsOptional(function.Body) {
		return true
	}
	for _, parameter := range function.Parameters {
		if parameter.Type.Kind == hir.TypeOptional {
			return true
		}
	}
	return false
}

func hirFunctionContainsRecordOptional(function hir.Function) bool {
	if hirTypeContainsRecordOptional(function.ReturnType) || hirExprContainsRecordOptional(function.Body) {
		return true
	}
	for _, parameter := range function.Parameters {
		if hirTypeContainsRecordOptional(parameter.Type) {
			return true
		}
	}
	return false
}

func hirTypeContainsRecordOptional(value hir.Type) bool {
	return value.Kind == hir.TypeOptional && value.Optional != nil && value.Optional.Value.Kind == hir.TypeRecord
}

func hirExprContainsRecordOptional(expression hir.Expr) bool {
	if hirTypeContainsRecordOptional(expression.Type) {
		return true
	}
	switch expression.Kind {
	case hir.ExprUnary:
		return expression.Unary != nil && expression.Unary.Operand != nil && hirExprContainsRecordOptional(*expression.Unary.Operand)
	case hir.ExprBinary:
		return expression.Binary != nil && expression.Binary.Left != nil && expression.Binary.Right != nil && (hirExprContainsRecordOptional(*expression.Binary.Left) || hirExprContainsRecordOptional(*expression.Binary.Right))
	case hir.ExprFieldProjection:
		return expression.Field != nil && expression.Field.Receiver != nil && hirExprContainsRecordOptional(*expression.Field.Receiver)
	case hir.ExprOptionalSome:
		return expression.Some != nil && expression.Some.Value != nil && hirExprContainsRecordOptional(*expression.Some.Value)
	case hir.ExprOptionalHasValue:
		return expression.HasValue != nil && expression.HasValue.Value != nil && hirExprContainsRecordOptional(*expression.HasValue.Value)
	case hir.ExprOptionalValueOr:
		return expression.ValueOr != nil && expression.ValueOr.Value != nil && expression.ValueOr.Fallback != nil && (hirExprContainsRecordOptional(*expression.ValueOr.Value) || hirExprContainsRecordOptional(*expression.ValueOr.Fallback))
	}
	return false
}

func hirExprContainsOptional(expression hir.Expr) bool {
	switch expression.Kind {
	case hir.ExprOptionalSome, hir.ExprOptionalNone, hir.ExprOptionalHasValue, hir.ExprOptionalValueOr:
		return true
	case hir.ExprUnary:
		return expression.Unary != nil && expression.Unary.Operand != nil && hirExprContainsOptional(*expression.Unary.Operand)
	case hir.ExprBinary:
		return expression.Binary != nil && expression.Binary.Left != nil && expression.Binary.Right != nil && (hirExprContainsOptional(*expression.Binary.Left) || hirExprContainsOptional(*expression.Binary.Right))
	case hir.ExprFieldProjection:
		return expression.Field != nil && expression.Field.Receiver != nil && hirExprContainsOptional(*expression.Field.Receiver)
	case hir.ExprRecordConstruct:
		if expression.Record != nil {
			for _, field := range expression.Record.Fields {
				if field.Value != nil && hirExprContainsOptional(*field.Value) {
					return true
				}
			}
		}
	}
	return false
}

func hirExprContainsOptionalDefault(expression hir.Expr) bool {
	switch expression.Kind {
	case hir.ExprOptionalValueOr:
		return true
	case hir.ExprUnary:
		return expression.Unary != nil && expression.Unary.Operand != nil && hirExprContainsOptionalDefault(*expression.Unary.Operand)
	case hir.ExprBinary:
		return expression.Binary != nil && expression.Binary.Left != nil && expression.Binary.Right != nil && (hirExprContainsOptionalDefault(*expression.Binary.Left) || hirExprContainsOptionalDefault(*expression.Binary.Right))
	case hir.ExprFieldProjection:
		return expression.Field != nil && expression.Field.Receiver != nil && hirExprContainsOptionalDefault(*expression.Field.Receiver)
	case hir.ExprRecordConstruct:
		if expression.Record != nil {
			for _, field := range expression.Record.Fields {
				if field.Value != nil && hirExprContainsOptionalDefault(*field.Value) {
					return true
				}
			}
		}
	}
	return false
}

func hirExprToCore(expression hir.Expr, parameters []hir.Parameter) (coreir.Expr, error) {
	result := coreir.Expr{Type: hirTypeToCore(expression.Type)}
	switch expression.Kind {
	case hir.ExprLiteral:
		if expression.Literal == nil {
			return coreir.Expr{}, coreLoweringError(expression.Span, "typed HIR literal has no value")
		}
		result.Kind = coreir.ExprLiteral
		result.Literal = &coreir.Literal{String: expression.Literal.String, Int: expression.Literal.Int, Float: expression.Literal.Float, Bool: expression.Literal.Bool}
	case hir.ExprReference:
		if expression.Reference == nil || expression.Reference.Kind != hir.BindingParameter || expression.Reference.Position < 0 || expression.Reference.Position >= len(parameters) {
			return coreir.Expr{}, coreLoweringError(expression.Span, "typed HIR reference has no bound parameter")
		}
		position := expression.Reference.Position
		result.Kind = coreir.ExprReference
		result.Parameter = &position
	case hir.ExprUnary:
		if expression.Unary == nil || expression.Unary.Operand == nil {
			return coreir.Expr{}, coreLoweringError(expression.Span, "typed HIR unary expression is incomplete")
		}
		operand, err := hirExprToCore(*expression.Unary.Operand, parameters)
		if err != nil {
			return coreir.Expr{}, err
		}
		result.Kind = coreir.ExprUnary
		result.Unary = &coreir.Unary{Operator: coreir.Operator(expression.Unary.Operator), Operand: &operand}
	case hir.ExprBinary:
		if expression.Binary == nil || expression.Binary.Left == nil || expression.Binary.Right == nil {
			return coreir.Expr{}, coreLoweringError(expression.Span, "typed HIR binary expression is incomplete")
		}
		left, err := hirExprToCore(*expression.Binary.Left, parameters)
		if err != nil {
			return coreir.Expr{}, err
		}
		right, err := hirExprToCore(*expression.Binary.Right, parameters)
		if err != nil {
			return coreir.Expr{}, err
		}
		result.Kind = coreir.ExprBinary
		result.Binary = &coreir.Binary{Operator: coreir.Operator(expression.Binary.Operator), Left: &left, Right: &right}
	case hir.ExprTextContainsCaseFolded:
		if expression.TextContains == nil || expression.TextContains.Value == nil || expression.TextContains.Query == nil {
			return coreir.Expr{}, coreLoweringError(expression.Span, "typed HIR contains_casefolded expression is incomplete")
		}
		value, err := hirExprToCore(*expression.TextContains.Value, parameters)
		if err != nil {
			return coreir.Expr{}, err
		}
		query, err := hirExprToCore(*expression.TextContains.Query, parameters)
		if err != nil {
			return coreir.Expr{}, err
		}
		result.Kind = coreir.ExprTextContainsCaseFolded
		result.TextContains = &coreir.TextContainsCaseFolded{Value: &value, Query: &query}
	case hir.ExprTextTrim:
		if expression.TextTrim == nil || expression.TextTrim.Value == nil {
			return coreir.Expr{}, coreLoweringError(expression.Span, "typed HIR trim expression is incomplete")
		}
		value, err := hirExprToCore(*expression.TextTrim.Value, parameters)
		if err != nil {
			return coreir.Expr{}, err
		}
		result.Kind = coreir.ExprTextTrim
		result.TextTrim = &coreir.TextTrim{Value: &value}
	case hir.ExprFieldProjection:
		if expression.Field == nil || expression.Field.Receiver == nil {
			return coreir.Expr{}, coreLoweringError(expression.Span, "typed HIR field projection is incomplete")
		}
		receiver, err := hirExprToCore(*expression.Field.Receiver, parameters)
		if err != nil {
			return coreir.Expr{}, err
		}
		result.Kind = coreir.ExprFieldProjection
		result.Field = &coreir.FieldProjection{Receiver: &receiver, Identity: hirIdentityToCore(expression.Field.Identity), Name: expression.Field.Name, Position: expression.Field.Position}
	case hir.ExprRecordConstruct:
		if expression.Record == nil {
			return coreir.Expr{}, coreLoweringError(expression.Span, "typed HIR record construction is incomplete")
		}
		construction := &coreir.RecordConstruct{Identity: hirIdentityToCore(expression.Record.Identity), Fields: make([]coreir.RecordConstructField, 0, len(expression.Record.Fields))}
		for _, field := range expression.Record.Fields {
			if field.Value == nil {
				return coreir.Expr{}, coreLoweringError(expression.Span, "typed HIR record construction field has no value")
			}
			value, err := hirExprToCore(*field.Value, parameters)
			if err != nil {
				return coreir.Expr{}, err
			}
			construction.Fields = append(construction.Fields, coreir.RecordConstructField{Identity: hirIdentityToCore(field.Identity), Name: field.Name, Position: field.Position, Value: &value})
		}
		result.Kind = coreir.ExprRecordConstruct
		result.Record = construction
	case hir.ExprOptionalSome:
		if expression.Some == nil || expression.Some.Value == nil {
			return coreir.Expr{}, coreLoweringError(expression.Span, "typed HIR optional some expression is incomplete")
		}
		value, err := hirExprToCore(*expression.Some.Value, parameters)
		if err != nil {
			return coreir.Expr{}, err
		}
		result.Kind = coreir.ExprOptionalSome
		result.Some = &coreir.OptionalSome{Value: &value}
	case hir.ExprOptionalNone:
		if expression.None == nil {
			return coreir.Expr{}, coreLoweringError(expression.Span, "typed HIR optional none expression is incomplete")
		}
		result.Kind = coreir.ExprOptionalNone
		result.None = &coreir.OptionalNone{}
	case hir.ExprOptionalHasValue:
		if expression.HasValue == nil || expression.HasValue.Value == nil {
			return coreir.Expr{}, coreLoweringError(expression.Span, "typed HIR optional has_value expression is incomplete")
		}
		value, err := hirExprToCore(*expression.HasValue.Value, parameters)
		if err != nil {
			return coreir.Expr{}, err
		}
		result.Kind = coreir.ExprOptionalHasValue
		result.HasValue = &coreir.OptionalHasValue{Value: &value}
	case hir.ExprOptionalValueOr:
		if expression.ValueOr == nil || expression.ValueOr.Value == nil || expression.ValueOr.Fallback == nil {
			return coreir.Expr{}, coreLoweringError(expression.Span, "typed HIR optional value_or expression is incomplete")
		}
		value, err := hirExprToCore(*expression.ValueOr.Value, parameters)
		if err != nil {
			return coreir.Expr{}, err
		}
		fallback, err := hirExprToCore(*expression.ValueOr.Fallback, parameters)
		if err != nil {
			return coreir.Expr{}, err
		}
		result.Kind = coreir.ExprOptionalValueOr
		result.ValueOr = &coreir.OptionalValueOr{Value: &value, Fallback: &fallback}
	case hir.ExprListEmpty:
		if expression.ListEmpty == nil {
			return coreir.Expr{}, coreLoweringError(expression.Span, "typed HIR list empty expression is incomplete")
		}
		result.Kind = coreir.ExprListEmpty
		result.ListEmpty = &coreir.ListEmpty{}
	case hir.ExprListSingleton:
		if expression.ListOne == nil || expression.ListOne.Value == nil {
			return coreir.Expr{}, coreLoweringError(expression.Span, "typed HIR list singleton expression is incomplete")
		}
		value, err := hirExprToCore(*expression.ListOne.Value, parameters)
		if err != nil {
			return coreir.Expr{}, err
		}
		result.Kind = coreir.ExprListSingleton
		result.ListOne = &coreir.ListSingleton{Value: &value}
	case hir.ExprListCount:
		if expression.ListCount == nil || expression.ListCount.Value == nil {
			return coreir.Expr{}, coreLoweringError(expression.Span, "typed HIR list count expression is incomplete")
		}
		value, err := hirExprToCore(*expression.ListCount.Value, parameters)
		if err != nil {
			return coreir.Expr{}, err
		}
		result.Kind = coreir.ExprListCount
		result.ListCount = &coreir.ListCount{Value: &value}
	case hir.ExprListAppend:
		if expression.ListAppend == nil || expression.ListAppend.Values == nil || expression.ListAppend.Value == nil {
			return coreir.Expr{}, coreLoweringError(expression.Span, "typed HIR list append expression is incomplete")
		}
		values, err := hirExprToCore(*expression.ListAppend.Values, parameters)
		if err != nil {
			return coreir.Expr{}, err
		}
		value, err := hirExprToCore(*expression.ListAppend.Value, parameters)
		if err != nil {
			return coreir.Expr{}, err
		}
		result.Kind = coreir.ExprListAppend
		result.ListAppend = &coreir.ListAppend{Values: &values, Value: &value}
	case hir.ExprListAt:
		if expression.ListAt == nil || expression.ListAt.Values == nil || expression.ListAt.Index == nil {
			return coreir.Expr{}, coreLoweringError(expression.Span, "typed HIR record-list at expression is incomplete")
		}
		values, err := hirExprToCore(*expression.ListAt.Values, parameters)
		if err != nil {
			return coreir.Expr{}, err
		}
		index, err := hirExprToCore(*expression.ListAt.Index, parameters)
		if err != nil {
			return coreir.Expr{}, err
		}
		result.Kind = coreir.ExprListAt
		result.ListAt = &coreir.ListAt{Values: &values, Index: &index}
	case hir.ExprListFindByText:
		if expression.ListFind == nil || expression.ListFind.Values == nil || expression.ListFind.Key == nil {
			return coreir.Expr{}, coreLoweringError(expression.Span, "typed HIR record-list find_by expression is incomplete")
		}
		values, err := hirExprToCore(*expression.ListFind.Values, parameters)
		if err != nil {
			return coreir.Expr{}, err
		}
		key, err := hirExprToCore(*expression.ListFind.Key, parameters)
		if err != nil {
			return coreir.Expr{}, err
		}
		result.Kind = coreir.ExprListFindByText
		result.ListFind = &coreir.ListFindByText{Values: &values, Field: hirIdentityToCore(expression.ListFind.Field), Name: expression.ListFind.Name, Position: expression.ListFind.Position, Key: &key}
	case hir.ExprListFilterByText:
		if expression.ListFilter == nil || expression.ListFilter.Values == nil || expression.ListFilter.Key == nil {
			return coreir.Expr{}, coreLoweringError(expression.Span, "typed HIR record-list filter_by expression is incomplete")
		}
		values, err := hirExprToCore(*expression.ListFilter.Values, parameters)
		if err != nil {
			return coreir.Expr{}, err
		}
		key, err := hirExprToCore(*expression.ListFilter.Key, parameters)
		if err != nil {
			return coreir.Expr{}, err
		}
		result.Kind = coreir.ExprListFilterByText
		result.ListFilter = &coreir.ListFilterByText{Values: &values, Field: hirIdentityToCore(expression.ListFilter.Field), Name: expression.ListFilter.Name, Position: expression.ListFilter.Position, Key: &key}
	case hir.ExprListFilterPredicate:
		filter := expression.ListFilterPredicate
		if filter == nil || filter.Values == nil || filter.Predicate.PackageID == "" || filter.Predicate.Path == "" {
			return coreir.Expr{}, coreLoweringError(expression.Span, "typed HIR named record predicate filter expression is incomplete")
		}
		values, err := hirExprToCore(*filter.Values, parameters)
		if err != nil {
			return coreir.Expr{}, err
		}
		arguments := make([]*coreir.Expr, 0, len(filter.Arguments))
		for _, argument := range filter.Arguments {
			if argument == nil {
				return coreir.Expr{}, coreLoweringError(expression.Span, "typed HIR named record predicate filter argument is nil")
			}
			lowered, err := hirExprToCore(*argument, parameters)
			if err != nil {
				return coreir.Expr{}, err
			}
			arguments = append(arguments, &lowered)
		}
		result.Kind = coreir.ExprListFilterPredicate
		result.ListFilterPredicate = &coreir.ListFilterPredicate{Values: &values, Predicate: hirIdentityToCore(filter.Predicate), PredicateName: filter.PredicateName, Arguments: arguments}
	case hir.ExprListFilterContainsCaseFolded:
		if expression.ListFilterContainsCaseFolded == nil || expression.ListFilterContainsCaseFolded.Values == nil || expression.ListFilterContainsCaseFolded.Query == nil {
			return coreir.Expr{}, coreLoweringError(expression.Span, "typed HIR record-list filter_contains_casefolded expression is incomplete")
		}
		values, err := hirExprToCore(*expression.ListFilterContainsCaseFolded.Values, parameters)
		if err != nil {
			return coreir.Expr{}, err
		}
		query, err := hirExprToCore(*expression.ListFilterContainsCaseFolded.Query, parameters)
		if err != nil {
			return coreir.Expr{}, err
		}
		result.Kind = coreir.ExprListFilterContainsCaseFolded
		result.ListFilterContainsCaseFolded = &coreir.ListFilterContainsCaseFolded{Values: &values, Field: hirIdentityToCore(expression.ListFilterContainsCaseFolded.Field), Name: expression.ListFilterContainsCaseFolded.Name, Position: expression.ListFilterContainsCaseFolded.Position, Query: &query}
	case hir.ExprListFilterJoinedContainsCaseFolded:
		joined := expression.ListFilterJoinedContainsCaseFolded
		if joined == nil || joined.Values == nil || joined.Query == nil || len(joined.Selectors) < 2 {
			return coreir.Expr{}, coreLoweringError(expression.Span, "typed HIR record-list filter_joined_contains_casefolded expression is incomplete")
		}
		values, err := hirExprToCore(*joined.Values, parameters)
		if err != nil {
			return coreir.Expr{}, err
		}
		query, err := hirExprToCore(*joined.Query, parameters)
		if err != nil {
			return coreir.Expr{}, err
		}
		selectors := make([]coreir.ListTextFieldSelector, 0, len(joined.Selectors))
		for _, selector := range joined.Selectors {
			selectors = append(selectors, coreir.ListTextFieldSelector{Field: hirIdentityToCore(selector.Field), Name: selector.Name, Position: selector.Position})
		}
		result.Kind = coreir.ExprListFilterJoinedContainsCaseFolded
		result.ListFilterJoinedContainsCaseFolded = &coreir.ListFilterJoinedContainsCaseFolded{Values: &values, Selectors: selectors, Query: &query}
	case hir.ExprListSortByOrdinalText:
		sorted := expression.ListSortByOrdinalText
		if sorted == nil || sorted.Values == nil {
			return coreir.Expr{}, coreLoweringError(expression.Span, "typed HIR record-list sort_by_ordinal expression is incomplete")
		}
		values, err := hirExprToCore(*sorted.Values, parameters)
		if err != nil {
			return coreir.Expr{}, err
		}
		result.Kind = coreir.ExprListSortByOrdinalText
		result.ListSortByOrdinalText = &coreir.ListSortByOrdinalText{Values: &values, Field: hirIdentityToCore(sorted.Field), Name: sorted.Name, Position: sorted.Position}
	case hir.ExprListSortByOrdinalTexts:
		sorted := expression.ListSortByOrdinalTexts
		if sorted == nil || sorted.Values == nil || len(sorted.Selectors) < 2 {
			return coreir.Expr{}, coreLoweringError(expression.Span, "typed HIR multi-key record-list sort_by_ordinal expression is incomplete")
		}
		values, err := hirExprToCore(*sorted.Values, parameters)
		if err != nil {
			return coreir.Expr{}, err
		}
		selectors := make([]coreir.ListTextFieldSelector, 0, len(sorted.Selectors))
		for _, selector := range sorted.Selectors {
			selectors = append(selectors, coreir.ListTextFieldSelector{Field: hirIdentityToCore(selector.Field), Name: selector.Name, Position: selector.Position})
		}
		result.Kind = coreir.ExprListSortByOrdinalTexts
		result.ListSortByOrdinalTexts = &coreir.ListSortByOrdinalTexts{Values: &values, Selectors: selectors}
	case hir.ExprResultOK:
		if expression.ResultOK == nil || expression.ResultOK.Value == nil {
			return coreir.Expr{}, coreLoweringError(expression.Span, "typed HIR result ok expression is incomplete")
		}
		value, err := hirExprToCore(*expression.ResultOK.Value, parameters)
		if err != nil {
			return coreir.Expr{}, err
		}
		result.Kind = coreir.ExprResultOK
		result.ResultOK = &coreir.ResultOK{Value: &value}
	case hir.ExprResultErr:
		if expression.ResultErr == nil || expression.ResultErr.Error == nil {
			return coreir.Expr{}, coreLoweringError(expression.Span, "typed HIR result err expression is incomplete")
		}
		failure, err := hirExprToCore(*expression.ResultErr.Error, parameters)
		if err != nil {
			return coreir.Expr{}, err
		}
		result.Kind = coreir.ExprResultErr
		result.ResultErr = &coreir.ResultErr{Error: &failure}
	case hir.ExprResultIsOK:
		if expression.ResultIsOK == nil || expression.ResultIsOK.Value == nil {
			return coreir.Expr{}, coreLoweringError(expression.Span, "typed HIR result is_ok expression is incomplete")
		}
		value, err := hirExprToCore(*expression.ResultIsOK.Value, parameters)
		if err != nil {
			return coreir.Expr{}, err
		}
		result.Kind = coreir.ExprResultIsOK
		result.ResultIsOK = &coreir.ResultIsOK{Value: &value}
	case hir.ExprResultSuccessOr:
		if expression.SuccessOr == nil || expression.SuccessOr.Value == nil || expression.SuccessOr.Fallback == nil {
			return coreir.Expr{}, coreLoweringError(expression.Span, "typed HIR result success_or expression is incomplete")
		}
		value, err := hirExprToCore(*expression.SuccessOr.Value, parameters)
		if err != nil {
			return coreir.Expr{}, err
		}
		fallback, err := hirExprToCore(*expression.SuccessOr.Fallback, parameters)
		if err != nil {
			return coreir.Expr{}, err
		}
		result.Kind = coreir.ExprResultSuccessOr
		result.SuccessOr = &coreir.ResultSuccessOr{Value: &value, Fallback: &fallback}
	case hir.ExprResultFailureOr:
		if expression.FailureOr == nil || expression.FailureOr.Value == nil || expression.FailureOr.Fallback == nil {
			return coreir.Expr{}, coreLoweringError(expression.Span, "typed HIR result failure_or expression is incomplete")
		}
		value, err := hirExprToCore(*expression.FailureOr.Value, parameters)
		if err != nil {
			return coreir.Expr{}, err
		}
		fallback, err := hirExprToCore(*expression.FailureOr.Fallback, parameters)
		if err != nil {
			return coreir.Expr{}, err
		}
		result.Kind = coreir.ExprResultFailureOr
		result.FailureOr = &coreir.ResultFailureOr{Value: &value, Fallback: &fallback}
	default:
		return coreir.Expr{}, coreLoweringError(expression.Span, fmt.Sprintf("unsupported typed HIR expression kind %q", expression.Kind))
	}
	return result, nil
}

func methodBySpan(program *Program, span Span) (*ClassDecl, *MethodDecl) {
	if program == nil {
		return nil, nil
	}
	for _, class := range program.Classes {
		for index := range class.Methods {
			if class.Methods[index].Span == span {
				return class, &class.Methods[index]
			}
		}
	}
	return nil, nil
}

func symbolBySpan(table *SymbolTable, span Span) (Symbol, bool) {
	if table == nil {
		return Symbol{}, false
	}
	for _, symbol := range table.Symbols() {
		if symbol.DeclarationSpan == span {
			return symbol, true
		}
	}
	return Symbol{}, false
}

func toHIRType(analysis *Analysis, resolved ResolvedTypeRef) hir.Type {
	if isResolvedRecordList(resolved) {
		identity := hir.SemanticIdentity{PackageID: string(PipeLangBuiltinPackageID), Path: string(PipeLangListSemanticPath)}
		return hir.Type{Kind: hir.TypeList, List: &hir.ListType{Element: toHIRType(analysis, resolved.Arguments[0])}, Identity: &identity, Name: "List"}
	}
	if isResolvedOptional(resolved) {
		return hir.Type{Kind: hir.TypeOptional, Optional: &hir.OptionalType{Value: toHIRType(analysis, resolved.Arguments[0])}}
	}
	if isResolvedResult(resolved) {
		return hir.Type{
			Kind:   hir.TypeResult,
			Result: &hir.ResultType{Success: toHIRType(analysis, resolved.Arguments[0]), Failure: toHIRType(analysis, resolved.Arguments[1])},
		}
	}
	if isResolvedArithmeticError(resolved) {
		return hir.Type{Kind: hir.TypeArithmeticError}
	}
	if analysis != nil && analysis.checked != nil && analysis.checked.isResolvedRecordType(resolved) {
		result := hir.Type{Kind: hir.TypeRecord, SymbolID: uint32(resolved.Symbol), Name: resolved.Name, Record: &hir.RecordType{Fields: []hir.RecordField{}}}
		if symbol, ok := analysis.Symbols.LookupID(resolved.Symbol); ok {
			if identity, ok := analysis.SemanticIDs.IdentityForSpan(symbol.DeclarationSpan); ok {
				converted := toHIRSemanticIdentity(identity)
				result.Identity = &converted
			}
		}
		if entry, ok := analysis.Symbols.lookupIDEntry(resolved.Symbol); ok && entry.recordDecl != nil {
			for _, field := range entry.recordDecl.Fields {
				fieldType, err := analysis.checked.resolveType(field.Type)
				if err != nil {
					continue
				}
				identity, _ := analysis.SemanticIDs.IdentityForSpan(field.Span)
				result.Record.Fields = append(result.Record.Fields, hir.RecordField{Identity: toHIRSemanticIdentity(identity), Name: field.Name, Type: toHIRType(analysis, fieldType)})
			}
		}
		return result
	}
	result := hir.Type{Kind: hir.TypeKind(resolved.Kind), Primitive: hir.PrimitiveType(resolved.Primitive), SymbolID: uint32(resolved.Symbol), Name: resolved.Name}
	if resolved.Kind == TypeRefPrimitive {
		switch resolved.Primitive {
		case TypeInt:
			result.Kind = hir.TypeNumeric
			result.Primitive = ""
			result.Numeric = &hir.NumericType{Representation: hir.NumericInteger, Bits: 64, Signed: true}
		case TypeFloat:
			result.Kind = hir.TypeNumeric
			result.Primitive = ""
			result.Numeric = &hir.NumericType{Representation: hir.NumericBinaryFloat, Bits: 64}
		}
	}
	if resolved.Kind == TypeRefNamed && analysis != nil && analysis.Symbols != nil && analysis.SemanticIDs != nil {
		if symbol, ok := analysis.Symbols.LookupID(resolved.Symbol); ok {
			if identity, ok := analysis.SemanticIDs.IdentityForSpan(symbol.DeclarationSpan); ok {
				converted := toHIRSemanticIdentity(identity)
				result.Identity = &converted
			}
		}
	}
	for _, argument := range resolved.Arguments {
		result.Arguments = append(result.Arguments, toHIRType(analysis, argument))
	}
	return result
}

func toHIRSemanticIdentity(identity SemanticIdentity) hir.SemanticIdentity {
	result := hir.SemanticIdentity{PackageID: string(identity.PackageID), Path: string(identity.Path)}
	if identity.Callable != nil {
		callable := hir.CallableIdentity{Returns: toHIRSemanticType(identity.Callable.Returns), Parameters: make([]hir.SemanticType, 0, len(identity.Callable.Parameters))}
		for _, parameter := range identity.Callable.Parameters {
			callable.Parameters = append(callable.Parameters, toHIRSemanticType(parameter))
		}
		result.Callable = &callable
	}
	return result
}

func toHIRSemanticType(identity SemanticTypeIdentity) hir.SemanticType {
	result := hir.SemanticType{Kind: hir.TypeKind(identity.Kind), Primitive: hir.PrimitiveType(identity.Primitive), PackageID: string(identity.PackageID), Path: string(identity.Path), Name: identity.Name}
	for _, argument := range identity.Arguments {
		result.Arguments = append(result.Arguments, toHIRSemanticType(argument))
	}
	return result
}

func hirTypeToCore(value hir.Type) coreir.Type {
	result := coreir.Type{Kind: coreir.TypeKind(value.Kind), Primitive: coreir.PrimitiveType(value.Primitive), Name: value.Name}
	if value.Numeric != nil {
		result.Numeric = &coreir.NumericType{Representation: coreir.NumericRepresentation(value.Numeric.Representation), Bits: value.Numeric.Bits, Signed: value.Numeric.Signed}
	}
	if value.Result != nil {
		result.Result = &coreir.ResultType{Success: hirTypeToCore(value.Result.Success), Failure: hirTypeToCore(value.Result.Failure)}
	}
	if value.Optional != nil {
		result.Optional = &coreir.OptionalType{Value: hirTypeToCore(value.Optional.Value)}
	}
	if value.List != nil {
		result.List = &coreir.ListType{Element: hirTypeToCore(value.List.Element)}
	}
	if value.Record != nil {
		result.Record = &coreir.RecordType{Fields: make([]coreir.RecordField, 0, len(value.Record.Fields))}
		for _, field := range value.Record.Fields {
			result.Record.Fields = append(result.Record.Fields, coreir.RecordField{Identity: hirIdentityToCore(field.Identity), Name: field.Name, Type: hirTypeToCore(field.Type)})
		}
	}
	if value.Identity != nil {
		identity := hirIdentityToCore(*value.Identity)
		result.Identity = &identity
	}
	for _, argument := range value.Arguments {
		result.Arguments = append(result.Arguments, hirTypeToCore(argument))
	}
	return result
}

func hirIdentityToCore(identity hir.SemanticIdentity) coreir.SemanticIdentity {
	result := coreir.SemanticIdentity{PackageID: identity.PackageID, Path: identity.Path}
	if identity.Callable != nil {
		callable := coreir.CallableIdentity{Returns: hirSemanticTypeToCore(identity.Callable.Returns), Parameters: make([]coreir.SemanticType, 0, len(identity.Callable.Parameters))}
		for _, parameter := range identity.Callable.Parameters {
			callable.Parameters = append(callable.Parameters, hirSemanticTypeToCore(parameter))
		}
		result.Callable = &callable
	}
	return result
}

func hirSemanticTypeToCore(value hir.SemanticType) coreir.SemanticType {
	result := coreir.SemanticType{Kind: coreir.TypeKind(value.Kind), Primitive: coreir.PrimitiveType(value.Primitive), PackageID: value.PackageID, Path: value.Path, Name: value.Name}
	for _, argument := range value.Arguments {
		result.Arguments = append(result.Arguments, hirSemanticTypeToCore(argument))
	}
	return result
}

func toHIRSpan(span Span) hir.SourceSpan {
	return hir.SourceSpan{File: string(span.File), Start: span.Start, End: span.End}
}

func hirUnaryOperator(operator string) (hir.Operator, bool) {
	switch operator {
	case "!":
		return hir.OperatorNot, true
	case "-":
		return hir.OperatorNegate, true
	default:
		return "", false
	}
}

func hirBinaryOperator(operator string) (hir.Operator, bool) {
	switch operator {
	case "+":
		return hir.OperatorAdd, true
	case "-":
		return hir.OperatorSubtract, true
	case "*":
		return hir.OperatorMultiply, true
	case "/":
		return hir.OperatorDivide, true
	case "==":
		return hir.OperatorEqual, true
	case "!=":
		return hir.OperatorNotEqual, true
	case "<":
		return hir.OperatorLessThan, true
	case "<=":
		return hir.OperatorLessOrEqual, true
	case ">":
		return hir.OperatorGreaterThan, true
	case ">=":
		return hir.OperatorGreaterOrEqual, true
	case "&&":
		return hir.OperatorAnd, true
	case "||":
		return hir.OperatorOr, true
	default:
		return "", false
	}
}

func hirLoweringError(analysis *Analysis, span Span, identity SemanticIdentity, message string) error {
	diagnostic := Diagnostic{Code: CodeHIRLowering, Category: CategorySemantic, Severity: SeverityError, Message: message, Primary: span}
	if identity.IsValid() {
		diagnostic.SemanticIDs = []SemanticID{identity.Path}
		diagnostic.SemanticIdentities = []SemanticIdentity{identity}
	}
	var sources *SourceSet
	if analysis != nil {
		sources = analysis.Sources
	}
	return diagnosticError(sources, Diagnostics{diagnostic})
}

func coreLoweringError(span hir.SourceSpan, message string) error {
	return oneDiagnostic(nil, CodeCoreLowering, CategorySemantic, Span{File: FileID(span.File), Start: span.Start, End: span.End}, message)
}
