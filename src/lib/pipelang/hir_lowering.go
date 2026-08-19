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
		return hir.Program{}, hirLoweringError(analysis, analysis.Program.Span, identity, fmt.Sprintf("typed HIR lowering requires a supported post-legacy language contract through %q", PipeLangLanguageContractV170))
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
	return hir.Program{LanguageContract: string(analysis.Modules.LanguageContract()), CompilerContract: coreir.CompilerContractV1, Functions: []hir.Function{function}}, nil
}

func lowerMethodBodyToHIR(analysis *Analysis, function SemanticIdentity, expression Expr, bindings map[string]hir.Binding, typeEnvironment map[string]ResolvedTypeRef, returnType ResolvedTypeRef) (hir.Expr, error) {
	contract := analysis.Modules.LanguageContract()
	if analysis.checked.isResolvedRecordType(returnType) {
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
	if unary, ok := expression.(*UnaryExpr); ok && (contract == PipeLangLanguageContractV050 || contract == PipeLangLanguageContractV060 || contract == PipeLangLanguageContractV070 || contract == PipeLangLanguageContractV080 || contract == PipeLangLanguageContractV090 || contract == PipeLangLanguageContractV100 || contract == PipeLangLanguageContractV110 || contract == PipeLangLanguageContractV120 || contract == PipeLangLanguageContractV130 || contract == PipeLangLanguageContractV140 || contract == PipeLangLanguageContractV150 || contract == PipeLangLanguageContractV160 || contract == PipeLangLanguageContractV170) && unary.Op == "-" {
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
	switch binary.Op {
	case "+":
		return hir.OperatorAdd, true
	case "-":
		return hir.OperatorSubtract, contract == PipeLangLanguageContractV030 || contract == PipeLangLanguageContractV040 || contract == PipeLangLanguageContractV050 || contract == PipeLangLanguageContractV060 || contract == PipeLangLanguageContractV070 || contract == PipeLangLanguageContractV080 || contract == PipeLangLanguageContractV090 || contract == PipeLangLanguageContractV100 || contract == PipeLangLanguageContractV110 || contract == PipeLangLanguageContractV120 || contract == PipeLangLanguageContractV130 || contract == PipeLangLanguageContractV140 || contract == PipeLangLanguageContractV150 || contract == PipeLangLanguageContractV160 || contract == PipeLangLanguageContractV170
	case "*":
		return hir.OperatorMultiply, contract == PipeLangLanguageContractV040 || contract == PipeLangLanguageContractV050 || contract == PipeLangLanguageContractV060 || contract == PipeLangLanguageContractV070 || contract == PipeLangLanguageContractV080 || contract == PipeLangLanguageContractV090 || contract == PipeLangLanguageContractV100 || contract == PipeLangLanguageContractV110 || contract == PipeLangLanguageContractV120 || contract == PipeLangLanguageContractV130 || contract == PipeLangLanguageContractV140 || contract == PipeLangLanguageContractV150 || contract == PipeLangLanguageContractV160 || contract == PipeLangLanguageContractV170
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
	resolved, err := analysis.checked.inferExprType(expression, typeEnvironment)
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
		if program.LanguageContract != coreir.LanguageContractV130 && program.LanguageContract != coreir.LanguageContractV140 && program.LanguageContract != coreir.LanguageContractV150 && program.LanguageContract != coreir.LanguageContractV160 && program.LanguageContract != coreir.LanguageContractV170 && hirFunctionContainsOptional(function) {
			return coreir.Program{}, coreLoweringError(function.Span, fmt.Sprintf("primitive Optional HIR requires language contract %q", coreir.LanguageContractV130))
		}
		if program.LanguageContract != coreir.LanguageContractV140 && program.LanguageContract != coreir.LanguageContractV150 && program.LanguageContract != coreir.LanguageContractV160 && program.LanguageContract != coreir.LanguageContractV170 && hirExprContainsOptionalDefault(function.Body) {
			return coreir.Program{}, coreLoweringError(function.Span, fmt.Sprintf("primitive Optional defaulting HIR requires language contract %q", coreir.LanguageContractV140))
		}
		if program.LanguageContract != coreir.LanguageContractV150 && program.LanguageContract != coreir.LanguageContractV160 && program.LanguageContract != coreir.LanguageContractV170 && hirFunctionContainsList(function) {
			return coreir.Program{}, coreLoweringError(function.Span, fmt.Sprintf("record-list HIR requires language contract %q", coreir.LanguageContractV150))
		}
		if program.LanguageContract != coreir.LanguageContractV160 && program.LanguageContract != coreir.LanguageContractV170 && hirExprContainsListCount(function.Body) {
			return coreir.Program{}, coreLoweringError(function.Span, fmt.Sprintf("record-list count HIR requires language contract %q", coreir.LanguageContractV160))
		}
		if program.LanguageContract != coreir.LanguageContractV170 && hirExprContainsListAppend(function.Body) {
			return coreir.Program{}, coreLoweringError(function.Span, fmt.Sprintf("record-list append HIR requires language contract %q", coreir.LanguageContractV170))
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

func hirExprContainsList(expression hir.Expr) bool {
	switch expression.Kind {
	case hir.ExprListEmpty, hir.ExprListSingleton, hir.ExprListCount, hir.ExprListAppend:
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
	if isResolvedPrimitiveOptional(resolved) {
		return hir.Type{Kind: hir.TypeOptional, Optional: &hir.OptionalType{Value: toHIRType(analysis, resolved.Arguments[0])}}
	}
	if isResolvedArithmeticResult(resolved) {
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
