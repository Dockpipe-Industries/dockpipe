package coreir

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

const (
	maxInt64 = int64(9223372036854775807)
	minInt64 = -maxInt64 - 1
)

func SignedInteger(bits int) Type {
	return Type{Kind: TypeNumeric, Numeric: &NumericType{Representation: NumericInteger, Bits: bits, Signed: true}}
}

func BinaryFloat(bits int) Type {
	return Type{Kind: TypeNumeric, Numeric: &NumericType{Representation: NumericBinaryFloat, Bits: bits}}
}

func ArithmeticErrorType() Type {
	return Type{Kind: TypeArithmeticError}
}

func ArithmeticResult(success Type) Type {
	return Type{Kind: TypeResult, Result: &ResultType{Success: success, Failure: ArithmeticErrorType()}}
}

// ArithmeticResultType is the single target-independent signature contract
// for bounded checked arithmetic. The v0.1.0 source lane remains closed; the
// v0.2.0 maps only direct integer addition here; v0.3.0 additionally maps
// direct integer subtraction; v0.4.0 additionally maps direct integer
// multiplication; v0.5.0 additionally maps direct integer negation; v0.6.0
// additionally maps direct binary64 division. Other operations remain
// compiler-internal.
func ArithmeticResultType(operator Operator, left Type, right *Type) (Type, error) {
	integer64 := SignedInteger(64)
	binary64 := BinaryFloat(64)
	switch operator {
	case OperatorNegate:
		if right != nil || !TypeEqual(left, integer64) {
			return Type{}, fmt.Errorf("operator %q requires one signed 64-bit integer operand", operator)
		}
		return ArithmeticResult(integer64), nil
	case OperatorAdd, OperatorSubtract, OperatorMultiply:
		if right == nil || !TypeEqual(left, integer64) || !TypeEqual(*right, integer64) {
			return Type{}, fmt.Errorf("operator %q requires two signed 64-bit integer operands", operator)
		}
		return ArithmeticResult(integer64), nil
	case OperatorDivide:
		if right == nil || !TypeEqual(left, binary64) || !TypeEqual(*right, binary64) {
			return Type{}, fmt.Errorf("operator %q requires two binary64 operands", operator)
		}
		return ArithmeticResult(binary64), nil
	default:
		return Type{}, fmt.Errorf("operator %q is not in the checked arithmetic contract", operator)
	}
}

func TypeEqual(left, right Type) bool {
	if left.Kind != right.Kind || left.Primitive != right.Primitive || left.Name != right.Name || (left.Numeric == nil) != (right.Numeric == nil) || (left.Result == nil) != (right.Result == nil) || (left.Optional == nil) != (right.Optional == nil) || (left.List == nil) != (right.List == nil) || (left.Record == nil) != (right.Record == nil) || (left.Identity == nil) != (right.Identity == nil) || len(left.Arguments) != len(right.Arguments) {
		return false
	}
	if left.Numeric != nil && *left.Numeric != *right.Numeric {
		return false
	}
	if left.Result != nil && (!TypeEqual(left.Result.Success, right.Result.Success) || !TypeEqual(left.Result.Failure, right.Result.Failure)) {
		return false
	}
	if left.Optional != nil && !TypeEqual(left.Optional.Value, right.Optional.Value) {
		return false
	}
	if left.List != nil && !TypeEqual(left.List.Element, right.List.Element) {
		return false
	}
	if left.Record != nil {
		if len(left.Record.Fields) != len(right.Record.Fields) {
			return false
		}
		for index := range left.Record.Fields {
			leftField, rightField := left.Record.Fields[index], right.Record.Fields[index]
			if leftField.Name != rightField.Name || leftField.Identity.PackageID != rightField.Identity.PackageID || leftField.Identity.Path != rightField.Identity.Path || !TypeEqual(leftField.Type, rightField.Type) {
				return false
			}
		}
	}
	if left.Identity != nil && (left.Identity.PackageID != right.Identity.PackageID || left.Identity.Path != right.Identity.Path) {
		return false
	}
	for index := range left.Arguments {
		if !TypeEqual(left.Arguments[index], right.Arguments[index]) {
			return false
		}
	}
	return true
}

// ValidateText enforces PipeLang's target-independent string invariant at
// Core boundaries. PipeLang text is always a valid UTF-8 encoding of a
// preserved Unicode scalar sequence.
func ValidateText(value string) error {
	if !utf8.ValidString(value) {
		return fmt.Errorf("string value is not valid UTF-8")
	}
	return nil
}

// CompareOrdinalText compares preserved Unicode scalar sequences without
// normalization, case folding, locale, or target collation.
func CompareOrdinalText(left, right string) (int, error) {
	if err := ValidateText(left); err != nil {
		return 0, fmt.Errorf("left %w", err)
	}
	if err := ValidateText(right); err != nil {
		return 0, fmt.Errorf("right %w", err)
	}
	for len(left) > 0 && len(right) > 0 {
		leftScalar, leftWidth := utf8.DecodeRuneInString(left)
		rightScalar, rightWidth := utf8.DecodeRuneInString(right)
		if leftScalar < rightScalar {
			return -1, nil
		}
		if leftScalar > rightScalar {
			return 1, nil
		}
		left = left[leftWidth:]
		right = right[rightWidth:]
	}
	if len(left) < len(right) {
		return -1, nil
	}
	if len(left) > len(right) {
		return 1, nil
	}
	return 0, nil
}

// ValidateFunction checks Core types and operator contracts before either a
// semantic evaluator or a target backend consumes the function.
func ValidateFunction(function Function) error {
	namedPredicate := isNamedPredicateFunction(function)
	composed := exprContainsCall(function.Body) || exprContainsConditional(function.Body) || function.Body.Kind == ExprImmutableLocal
	for position, parameter := range function.Parameters {
		if parameter.Position != position {
			return fmt.Errorf("parameter %d is not in normalized position order", position)
		}
		if err := validateType(parameter.Type); err != nil {
			return fmt.Errorf("parameter %d type: %w", position, err)
		}
	}
	if err := validateType(function.ReturnType); err != nil {
		return fmt.Errorf("return type: %w", err)
	}
	if !composed && exprContainsRecordConstruction(function.Body) && function.Body.Kind != ExprRecordConstruct {
		return fmt.Errorf("record construction must be the complete function body")
	}
	if !composed && exprContainsTextCaseFolded(function.Body) && function.Body.Kind != ExprTextContainsCaseFolded && !namedPredicate {
		return fmt.Errorf("contains_casefolded must be the complete function body")
	}
	if !composed && exprContainsTextTrim(function.Body) && function.Body.Kind != ExprTextTrim && !namedPredicate {
		return fmt.Errorf("trim must be the complete function body")
	}
	if !composed && exprContainsListFilterContainsCaseFolded(function.Body) && function.Body.Kind != ExprListFilterContainsCaseFolded {
		return fmt.Errorf("filter_contains_casefolded must be the complete function body")
	}
	if !composed && exprContainsListFilterJoinedContainsCaseFolded(function.Body) && function.Body.Kind != ExprListFilterJoinedContainsCaseFolded {
		return fmt.Errorf("filter_joined_contains_casefolded must be the complete function body")
	}
	if !composed && exprContainsListSortByOrdinalText(function.Body) && function.Body.Kind != ExprListSortByOrdinalText && function.Body.Kind != ExprListSortByOrdinalTexts && function.Body.Kind != ExprListSortByOrdinalDirections {
		return fmt.Errorf("sort_by_ordinal must be the complete function body")
	}
	if err := validateExpr(function.Body, function.Parameters); err != nil {
		return err
	}
	if !composed && exprContainsRecordEquality(function.Body) {
		if err := validateDirectRecordEquality(function); err != nil {
			return err
		}
	}
	textContainsCaseFolded := !composed && function.Body.Kind == ExprTextContainsCaseFolded
	if textContainsCaseFolded {
		if err := validateDirectTextContainsCaseFoldedFunction(function); err != nil {
			return err
		}
	}
	textTrim := !composed && function.Body.Kind == ExprTextTrim
	if textTrim {
		if err := validateDirectTextTrimFunction(function); err != nil {
			return err
		}
	}
	boundedResult := !composed && functionContainsBoundedValueResult(function)
	if boundedResult {
		if err := validateDirectBoundedValueResultFunction(function); err != nil {
			return err
		}
	}
	listAt := !composed && function.Body.Kind == ExprListAt
	if listAt {
		if err := validateDirectListAtFunction(function); err != nil {
			return err
		}
	}
	listFindByText := !composed && function.Body.Kind == ExprListFindByText
	if listFindByText {
		if err := validateDirectListFindByTextFunction(function); err != nil {
			return err
		}
	}
	listFilterByText := !composed && function.Body.Kind == ExprListFilterByText
	if listFilterByText {
		if err := validateDirectListFilterByTextFunction(function); err != nil {
			return err
		}
	}
	listFilterPredicate := !composed && function.Body.Kind == ExprListFilterPredicate
	if listFilterPredicate {
		if err := validateDirectListFilterPredicateFunction(function); err != nil {
			return err
		}
	}
	listFilterContainsCaseFolded := !composed && function.Body.Kind == ExprListFilterContainsCaseFolded
	if listFilterContainsCaseFolded {
		if err := validateDirectListFilterContainsCaseFoldedFunction(function); err != nil {
			return err
		}
	}
	listFilterJoinedContainsCaseFolded := !composed && function.Body.Kind == ExprListFilterJoinedContainsCaseFolded
	if listFilterJoinedContainsCaseFolded {
		if err := validateDirectListFilterJoinedContainsCaseFoldedFunction(function); err != nil {
			return err
		}
	}
	listSortByOrdinalText := !composed && function.Body.Kind == ExprListSortByOrdinalText
	if listSortByOrdinalText {
		if err := validateDirectListSortByOrdinalTextFunction(function); err != nil {
			return err
		}
	}
	listSortByOrdinalTexts := !composed && function.Body.Kind == ExprListSortByOrdinalTexts
	if listSortByOrdinalTexts {
		if err := validateDirectListSortByOrdinalTextsFunction(function); err != nil {
			return err
		}
	}
	listSortByOrdinalDirections := !composed && function.Body.Kind == ExprListSortByOrdinalDirections
	if listSortByOrdinalDirections {
		if err := validateDirectListSortByOrdinalDirectionsFunction(function); err != nil {
			return err
		}
	}
	if !composed && functionContainsOptional(function) && !boundedResult && !listAt && !listFindByText && !listSortByOrdinalText && !listSortByOrdinalTexts {
		if err := validateDirectOptionalFunction(function); err != nil {
			return err
		}
	}
	if !composed && functionContainsList(function) && !boundedResult && !listAt && !listFindByText && !listFilterByText && !listFilterPredicate && !listFilterContainsCaseFolded && !listFilterJoinedContainsCaseFolded && !listSortByOrdinalText && !listSortByOrdinalTexts && !listSortByOrdinalDirections {
		if err := validateDirectListFunction(function); err != nil {
			return err
		}
	}
	if !TypeEqual(function.ReturnType, function.Body.Type) {
		return fmt.Errorf("function return type does not match its body type")
	}
	return nil
}

// ValidateProgram admits exact compiler/language identities and validates all
// functions against their versioned Core feature and composition contracts.
func ValidateProgram(program Program) error {
	version, err := validateProgramIdentity(program)
	if err != nil {
		return err
	}
	inheritedContract := program.LanguageContract
	if ((((inheritedContract == LanguageContractV890 || (inheritedContract == LanguageContractV900 || (inheritedContract == LanguageContractV910 || (inheritedContract == LanguageContractV920 || (inheritedContract == LanguageContractV930 || (inheritedContract == LanguageContractV940 || (inheritedContract == LanguageContractV950 || (inheritedContract == LanguageContractV960 || (inheritedContract == LanguageContractV970 || (inheritedContract == LanguageContractV980 || (inheritedContract == LanguageContractV990 || (inheritedContract == LanguageContractV1000 || (inheritedContract == LanguageContractV1010 || (inheritedContract == LanguageContractV1020 || (inheritedContract == LanguageContractV1030 || (inheritedContract == LanguageContractV1040 || inheritedContract == LanguageContractV1050)))))))))))))))) || inheritedContract == LanguageContractV880) || inheritedContract == LanguageContractV870) || inheritedContract == LanguageContractV860) || inheritedContract == LanguageContractV850 || inheritedContract == LanguageContractV840 || inheritedContract == LanguageContractV830 || inheritedContract == LanguageContractV820 || inheritedContract == LanguageContractV810 || inheritedContract == LanguageContractV800 || inheritedContract == LanguageContractV790 || inheritedContract == LanguageContractV780 || inheritedContract == LanguageContractV770 || inheritedContract == LanguageContractV760 || inheritedContract == LanguageContractV750 || inheritedContract == LanguageContractV740 {
		inheritedContract = LanguageContractV730
	}
	functions := make(map[string]Function, len(program.Functions))
	for _, function := range program.Functions {
		if err := validateImmutableLocalContract(program.LanguageContract, function); err != nil {
			return err
		}
		if err := ValidateFunction(function); err != nil {
			return fmt.Errorf("function %s: %w", function.Name, err)
		}
		if err := validateFeatureContract(version, function); err != nil {
			return err
		}
		key := function.Identity.PackageID + "\x00" + function.Identity.Path
		if _, exists := functions[key]; exists {
			return fmt.Errorf("duplicate function semantic identity")
		}
		functions[key] = function
	}
	for _, function := range program.Functions {
		if err := validateConditionalContract(program.LanguageContract, function); err != nil {
			return err
		}
		if err := validatePureCallPlacement(inheritedContract, function); err != nil {
			return err
		}
		if err := validatePureCalls(inheritedContract, function, functions); err != nil {
			return err
		}
		if err := validateHelperResultMatchContract(inheritedContract, function, functions); err != nil {
			return err
		}
		filter := function.Body.ListFilterPredicate
		if function.Body.Kind != ExprListFilterPredicate || filter == nil {
			continue
		}
		if !isV310OrLaterContract(inheritedContract) {
			return fmt.Errorf("function %s named predicate filtering requires language contract %q", function.Name, LanguageContractV310)
		}
		target, ok := functions[filter.Predicate.PackageID+"\x00"+filter.Predicate.Path]
		if !ok {
			return fmt.Errorf("function %s named predicate target was not found", function.Name)
		}
		if !isNamedPredicateFunction(target) || target.Name != filter.PredicateName || semanticOwnerPath(target.Identity.Path) != semanticOwnerPath(function.Identity.Path) || !callableIdentityEqual(filter.Predicate.Callable, target.Identity.Callable) {
			return fmt.Errorf("function %s named predicate target is invalid", function.Name)
		}
		if len(target.Parameters) != len(filter.Arguments)+1 || function.Body.Type.List == nil || !TypeEqual(target.Parameters[0].Type, function.Body.Type.List.Element) {
			return fmt.Errorf("function %s named predicate signature does not match its filter", function.Name)
		}
		for position, argument := range filter.Arguments {
			if argument == nil || !TypeEqual(argument.Type, target.Parameters[position+1].Type) {
				return fmt.Errorf("function %s named predicate argument %d type mismatch", function.Name, position+1)
			}
		}
	}
	state := make(map[string]uint8, len(functions))
	var visit func(Function) error
	visit = func(function Function) error {
		key := function.Identity.PackageID + "\x00" + function.Identity.Path
		state[key] = 1
		var walk func(Expr) error
		walk = func(expression Expr) error {
			if expression.Kind == ExprCall && expression.Call != nil {
				targetKey := expression.Call.Target.PackageID + "\x00" + expression.Call.Target.Path
				if state[targetKey] == 1 {
					return fmt.Errorf("function %s pure call graph contains a cycle", function.Name)
				}
				if state[targetKey] == 0 {
					if err := visit(functions[targetKey]); err != nil {
						return err
					}
				}
			}
			for _, child := range expressionChildren(expression) {
				if child != nil {
					if err := walk(*child); err != nil {
						return err
					}
				}
			}
			return nil
		}
		if err := walk(function.Body); err != nil {
			return err
		}
		state[key] = 2
		return nil
	}
	for _, function := range program.Functions {
		key := function.Identity.PackageID + "\x00" + function.Identity.Path
		if state[key] == 0 {
			if err := visit(function); err != nil {
				return err
			}
		}
	}
	return nil
}

func validatePureCalls(contract string, function Function, functions map[string]Function) error {
	var walk func(Expr) error
	walk = func(expression Expr) error {
		if expression.Kind == ExprCall {
			if (contract != LanguageContractV730 && contract != LanguageContractV720 && contract != LanguageContractV710 && contract != LanguageContractV700 && contract != LanguageContractV690 && contract != LanguageContractV680 && contract != LanguageContractV670 && contract != LanguageContractV660 && contract != LanguageContractV650 && contract != LanguageContractV640 && contract != LanguageContractV630 && contract != LanguageContractV620 && contract != LanguageContractV610 && contract != LanguageContractV600 && contract != LanguageContractV590 && contract != LanguageContractV580) && contract != LanguageContractV570 && contract != LanguageContractV560 && contract != LanguageContractV550 && contract != LanguageContractV540 && contract != LanguageContractV530 && contract != LanguageContractV520 && contract != LanguageContractV510 && contract != LanguageContractV500 && contract != LanguageContractV490 && contract != LanguageContractV360 && contract != LanguageContractV370 && contract != LanguageContractV380 && contract != LanguageContractV390 && contract != LanguageContractV400 && contract != LanguageContractV410 && contract != LanguageContractV420 && contract != LanguageContractV430 && contract != LanguageContractV440 && contract != LanguageContractV450 && contract != LanguageContractV460 && contract != LanguageContractV470 && contract != LanguageContractV480 {
				return fmt.Errorf("function %s pure calls require language contract %q or later", function.Name, LanguageContractV360)
			}
			call := expression.Call
			if call == nil || call.TargetName == "" || call.Target.PackageID == "" || call.Target.Path == "" || call.Target.Callable == nil {
				return fmt.Errorf("function %s pure call is incomplete", function.Name)
			}
			target, ok := functions[call.Target.PackageID+"\x00"+call.Target.Path]
			if !ok {
				return fmt.Errorf("function %s pure call target was not found", function.Name)
			}
			if target.Name != call.TargetName || target.Identity.PackageID != function.Identity.PackageID || semanticOwnerPath(target.Identity.Path) != semanticOwnerPath(function.Identity.Path) || !callableIdentityEqual(call.Target.Callable, target.Identity.Callable) {
				return fmt.Errorf("function %s pure call target is invalid", function.Name)
			}
			if len(call.Arguments) != len(target.Parameters) {
				return fmt.Errorf("function %s pure call argument count mismatch", function.Name)
			}
			for position, argument := range call.Arguments {
				if argument == nil || !TypeEqual(argument.Type, target.Parameters[position].Type) {
					return fmt.Errorf("function %s pure call argument %d type mismatch", function.Name, position+1)
				}
			}
			if !TypeEqual(expression.Type, target.ReturnType) {
				return fmt.Errorf("function %s pure call result type mismatch", function.Name)
			}
		}
		for _, child := range expressionChildren(expression) {
			if child != nil {
				if err := walk(*child); err != nil {
					return err
				}
			}
		}
		return nil
	}
	return walk(function.Body)
}

func exprContainsCall(expression Expr) bool {
	if expression.Kind == ExprCall {
		return true
	}
	for _, child := range expressionChildren(expression) {
		if child != nil && exprContainsCall(*child) {
			return true
		}
	}
	return false
}

func exprContainsConditional(expression Expr) bool {
	if expression.Kind == ExprConditional {
		return true
	}
	for _, child := range expressionChildren(expression) {
		if child != nil && exprContainsConditional(*child) {
			return true
		}
	}
	return false
}

func countConditionalExpressions(expression Expr) int {
	count := 0
	if expression.Kind == ExprConditional {
		count++
	}
	for _, child := range expressionChildren(expression) {
		if child != nil {
			count += countConditionalExpressions(*child)
		}
	}
	return count
}

func countTerminalIfStatements(expression Expr) int {
	count := 0
	if expression.Kind == ExprConditional && expression.Conditional != nil && expression.Conditional.TerminalStatement {
		count++
	}
	for _, child := range expressionChildren(expression) {
		if child != nil {
			count += countTerminalIfStatements(*child)
		}
	}
	return count
}

func validConditionalOperand(expression Expr) bool {
	if expression.Kind == ExprConditional || expression.Kind == ExprMatch || expression.Kind == ExprPropagate {
		return false
	}
	for _, child := range expressionChildren(expression) {
		if child == nil || !validConditionalOperand(*child) {
			return false
		}
	}
	return true
}

func countImmutableLocalExpressions(expression Expr) int {
	count := 0
	if expression.Kind == ExprImmutableLocal {
		count++
	}
	for _, child := range expressionChildren(expression) {
		if child != nil {
			count += countImmutableLocalExpressions(*child)
		}
	}
	return count
}

func terminalBranchLocalShape(expression Expr, limit int) (int, bool) {
	count := 0
	current := expression
	for current.Kind == ExprImmutableLocal {
		local := current.ImmutableLocal
		count++
		if (limit >= 0 && count > limit) || local == nil || local.Initializer == nil || local.Return == nil {
			return count, false
		}
		current = *local.Return
	}
	return count, limit < 0 || count <= limit
}

func terminalBranchTail(expression Expr) (int, Expr, bool) {
	count := 0
	current := expression
	for current.Kind == ExprImmutableLocal {
		local := current.ImmutableLocal
		count++
		if local == nil || local.Initializer == nil || local.Return == nil || !validConditionalOperand(*local.Initializer) {
			return count, Expr{}, false
		}
		current = *local.Return
	}
	return count, current, true
}

func validV740NestedTerminalIf(expression Expr) bool {
	outer := expression.Conditional
	if expression.Kind != ExprConditional || outer == nil || !outer.TerminalStatement || outer.Condition == nil || outer.WhenTrue == nil || outer.WhenFalse == nil || !validConditionalOperand(*outer.Condition) {
		return false
	}
	validNested := func(branch Expr) bool {
		_, tail, valid := terminalBranchTail(branch)
		nested := tail.Conditional
		if !valid || tail.Kind != ExprConditional || nested == nil || !nested.TerminalStatement || nested.Condition == nil || nested.WhenTrue == nil || nested.WhenFalse == nil {
			return false
		}
		if nested.WhenTrue.Kind == ExprImmutableLocal || nested.WhenFalse.Kind == ExprImmutableLocal {
			return false
		}
		return validConditionalOperand(*nested.Condition) && validConditionalOperand(*nested.WhenTrue) && validConditionalOperand(*nested.WhenFalse)
	}
	validOrdinary := func(branch Expr) bool {
		_, tail, valid := terminalBranchTail(branch)
		return valid && tail.Kind != ExprConditional && validConditionalOperand(tail)
	}
	trueNested, falseNested := validNested(*outer.WhenTrue), validNested(*outer.WhenFalse)
	return countConditionalExpressions(expression) == 2 && countTerminalIfStatements(expression) == 2 &&
		((trueNested && validOrdinary(*outer.WhenFalse)) || (falseNested && validOrdinary(*outer.WhenTrue)))
}

func validV750NestedTerminalIf(expression Expr) bool {
	outer := expression.Conditional
	if expression.Kind != ExprConditional || outer == nil || !outer.TerminalStatement || outer.Condition == nil || outer.WhenTrue == nil || outer.WhenFalse == nil || !validConditionalOperand(*outer.Condition) {
		return false
	}
	validInnerLeaf := func(branch Expr) bool {
		_, tail, valid := terminalBranchTail(branch)
		if !valid || tail.Kind == ExprConditional {
			return false
		}
		return validConditionalOperand(tail)
	}
	validNested := func(branch Expr) bool {
		_, tail, valid := terminalBranchTail(branch)
		nested := tail.Conditional
		if !valid || tail.Kind != ExprConditional || nested == nil || !nested.TerminalStatement || nested.Condition == nil || nested.WhenTrue == nil || nested.WhenFalse == nil {
			return false
		}
		return validConditionalOperand(*nested.Condition) && validInnerLeaf(*nested.WhenTrue) && validInnerLeaf(*nested.WhenFalse)
	}
	validOrdinary := func(branch Expr) bool {
		_, tail, valid := terminalBranchTail(branch)
		return valid && tail.Kind != ExprConditional && validConditionalOperand(tail)
	}
	trueNested, falseNested := validNested(*outer.WhenTrue), validNested(*outer.WhenFalse)
	return countConditionalExpressions(expression) == 2 && countTerminalIfStatements(expression) == 2 &&
		((trueNested && validOrdinary(*outer.WhenFalse)) || (falseNested && validOrdinary(*outer.WhenTrue)))
}

func validV760RootLocalNestedTerminalIf(expression Expr) bool {
	locals, tail, valid := terminalBranchTail(expression)
	return valid && locals > 0 && validV750NestedTerminalIf(tail)
}

func validV770SymmetricRootLocalNestedTerminalIf(expression Expr) bool {
	return validSymmetricNestedTerminalIf(expression, true)
}

func validSymmetricNestedTerminalIf(expression Expr, requireRoot bool) bool {
	locals, tail, valid := terminalBranchTail(expression)
	outer := tail.Conditional
	if !valid || (requireRoot && locals == 0) || tail.Kind != ExprConditional || outer == nil || !outer.TerminalStatement || outer.Condition == nil || outer.WhenTrue == nil || outer.WhenFalse == nil || !validConditionalOperand(*outer.Condition) {
		return false
	}
	validInnerLeaf := func(branch Expr) bool {
		_, leaf, valid := terminalBranchTail(branch)
		return valid && leaf.Kind != ExprConditional && validConditionalOperand(leaf)
	}
	validNested := func(branch Expr) bool {
		_, nestedTail, valid := terminalBranchTail(branch)
		nested := nestedTail.Conditional
		if !valid || nestedTail.Kind != ExprConditional || nested == nil || !nested.TerminalStatement || nested.Condition == nil || nested.WhenTrue == nil || nested.WhenFalse == nil {
			return false
		}
		return validConditionalOperand(*nested.Condition) && validInnerLeaf(*nested.WhenTrue) && validInnerLeaf(*nested.WhenFalse)
	}
	return countConditionalExpressions(expression) == 3 && countTerminalIfStatements(expression) == 3 && validNested(*outer.WhenTrue) && validNested(*outer.WhenFalse)
}

func validV790BoundedDepthThreeTerminalIf(expression Expr) bool {
	return validExpandedDepthThreeTerminalIf(expression, 1)
}

func validV800TwoExpandedTerminalIf(expression Expr) bool {
	return validExpandedDepthThreeTerminalIf(expression, 2)
}

// Validate topology independently of source analysis. ValidateFunction separately
// enforces types, binding positions, and lexical references throughout the tree.
func validV810TerminalTree(expression Expr, remaining int) bool {
	_, tail, valid := terminalBranchTail(expression)
	if !valid {
		return false
	}
	if tail.Kind != ExprConditional {
		return validConditionalOperand(tail)
	}
	conditional := tail.Conditional
	return remaining > 0 && conditional != nil && conditional.TerminalStatement &&
		conditional.Condition != nil && conditional.WhenTrue != nil && conditional.WhenFalse != nil &&
		validConditionalOperand(*conditional.Condition) &&
		validV810TerminalTree(*conditional.WhenTrue, remaining-1) &&
		validV810TerminalTree(*conditional.WhenFalse, remaining-1)
}

func validExpandedDepthThreeTerminalIf(expression Expr, expandedLeaves int) bool {
	_, tail, valid := terminalBranchTail(expression)
	outer := tail.Conditional
	if !valid || tail.Kind != ExprConditional || outer == nil || !outer.TerminalStatement || outer.Condition == nil || outer.WhenTrue == nil || outer.WhenFalse == nil || !validConditionalOperand(*outer.Condition) {
		return false
	}
	validLeaf := func(branch Expr) bool {
		_, leaf, valid := terminalBranchTail(branch)
		return valid && leaf.Kind != ExprConditional && validConditionalOperand(leaf)
	}
	classifyDepthTwoLeaf := func(branch Expr) (int, bool) {
		_, leaf, valid := terminalBranchTail(branch)
		if !valid {
			return 0, false
		}
		if leaf.Kind != ExprConditional {
			return 0, validConditionalOperand(leaf)
		}
		third := leaf.Conditional
		if third == nil || !third.TerminalStatement || third.Condition == nil || third.WhenTrue == nil || third.WhenFalse == nil || !validConditionalOperand(*third.Condition) {
			return 0, false
		}
		return 1, validLeaf(*third.WhenTrue) && validLeaf(*third.WhenFalse)
	}
	classifyInner := func(branch Expr) (int, bool) {
		_, nestedTail, valid := terminalBranchTail(branch)
		inner := nestedTail.Conditional
		if !valid || nestedTail.Kind != ExprConditional || inner == nil || !inner.TerminalStatement || inner.Condition == nil || inner.WhenTrue == nil || inner.WhenFalse == nil || !validConditionalOperand(*inner.Condition) {
			return 0, false
		}
		trueThird, validTrue := classifyDepthTwoLeaf(*inner.WhenTrue)
		falseThird, validFalse := classifyDepthTwoLeaf(*inner.WhenFalse)
		return trueThird + falseThird, validTrue && validFalse
	}
	trueThird, validTrue := classifyInner(*outer.WhenTrue)
	falseThird, validFalse := classifyInner(*outer.WhenFalse)
	return validTrue && validFalse && trueThird+falseThird == expandedLeaves && countConditionalExpressions(expression) == 3+expandedLeaves && countTerminalIfStatements(expression) == 3+expandedLeaves
}

func validateImmutableLocalContract(contract string, function Function) error {
	if contract == LanguageContractV1050 && validDepthThreeTerminalConditionalTests(function.Body) {
		return nil
	}
	if (contract == LanguageContractV1040 || contract == LanguageContractV1050) && validNestedTerminalConditionalTests(function.Body) {
		return nil
	}
	if (contract == LanguageContractV1030 || (contract == LanguageContractV1040 || contract == LanguageContractV1050)) && validTerminalConditionalTests(function.Body) {
		return nil
	}
	if (contract == LanguageContractV1020 || (contract == LanguageContractV1030 || (contract == LanguageContractV1040 || contract == LanguageContractV1050))) && validTerminalBooleanSelectorInitializers(function.Body) {
		return nil
	}
	if (contract == LanguageContractV1010 || (contract == LanguageContractV1020 || (contract == LanguageContractV1030 || (contract == LanguageContractV1040 || contract == LanguageContractV1050)))) && validStraightLineBooleanSelectorInitializers(function.Body) {
		return nil
	}
	if (contract == LanguageContractV980 || (contract == LanguageContractV990 || (contract == LanguageContractV1000 || (contract == LanguageContractV1010 || (contract == LanguageContractV1020 || (contract == LanguageContractV1030 || (contract == LanguageContractV1040 || contract == LanguageContractV1050))))))) && validConditionalBooleanSelector(function.Body) || (contract == LanguageContractV990 || (contract == LanguageContractV1000 || (contract == LanguageContractV1010 || (contract == LanguageContractV1020 || (contract == LanguageContractV1030 || (contract == LanguageContractV1040 || contract == LanguageContractV1050)))))) && validTerminalLeafBooleanSelectors(function.Body) {
		return nil
	}
	if (contract == LanguageContractV940 || (contract == LanguageContractV950 || (contract == LanguageContractV960 || (contract == LanguageContractV970 || (contract == LanguageContractV980 || (contract == LanguageContractV990 || (contract == LanguageContractV1000 || (contract == LanguageContractV1010 || (contract == LanguageContractV1020 || (contract == LanguageContractV1030 || (contract == LanguageContractV1040 || contract == LanguageContractV1050))))))))))) && validDepthThreeTerminalLeafReturns(function.Body) {
		return nil
	}
	if (contract == LanguageContractV970 || (contract == LanguageContractV980 || (contract == LanguageContractV990 || (contract == LanguageContractV1000 || (contract == LanguageContractV1010 || (contract == LanguageContractV1020 || (contract == LanguageContractV1030 || (contract == LanguageContractV1040 || contract == LanguageContractV1050)))))))) && validDepthThreeTerminalInitializers(function.Body) {
		return nil
	}
	if (contract == LanguageContractV960 || (contract == LanguageContractV970 || (contract == LanguageContractV980 || (contract == LanguageContractV990 || (contract == LanguageContractV1000 || (contract == LanguageContractV1010 || (contract == LanguageContractV1020 || (contract == LanguageContractV1030 || (contract == LanguageContractV1040 || contract == LanguageContractV1050))))))))) && validDepthThreeStraightLineInitializers(function.Body) {
		return nil
	}
	if (contract == LanguageContractV930 || (contract == LanguageContractV940 || (contract == LanguageContractV950 || (contract == LanguageContractV960 || (contract == LanguageContractV970 || (contract == LanguageContractV980 || (contract == LanguageContractV990 || (contract == LanguageContractV1000 || (contract == LanguageContractV1010 || (contract == LanguageContractV1020 || (contract == LanguageContractV1030 || (contract == LanguageContractV1040 || contract == LanguageContractV1050)))))))))))) && validDepthThreeStraightLineReturns(function.Body) {
		return nil
	}
	if (contract == LanguageContractV910 || (contract == LanguageContractV920 || (contract == LanguageContractV930 || (contract == LanguageContractV940 || (contract == LanguageContractV950 || (contract == LanguageContractV960 || (contract == LanguageContractV970 || (contract == LanguageContractV980 || (contract == LanguageContractV990 || (contract == LanguageContractV1000 || (contract == LanguageContractV1010 || (contract == LanguageContractV1020 || (contract == LanguageContractV1030 || (contract == LanguageContractV1040 || contract == LanguageContractV1050)))))))))))))) && validNestedTerminalInitializers(function.Body) {
		return nil
	}
	if (contract == LanguageContractV900 || (contract == LanguageContractV910 || (contract == LanguageContractV920 || (contract == LanguageContractV930 || (contract == LanguageContractV940 || (contract == LanguageContractV950 || (contract == LanguageContractV960 || (contract == LanguageContractV970 || (contract == LanguageContractV980 || (contract == LanguageContractV990 || (contract == LanguageContractV1000 || (contract == LanguageContractV1010 || (contract == LanguageContractV1020 || (contract == LanguageContractV1030 || (contract == LanguageContractV1040 || contract == LanguageContractV1050))))))))))))))) && validNestedStraightLineInitializers(function.Body) {
		return nil
	}
	if (contract == LanguageContractV890 || (contract == LanguageContractV900 || (contract == LanguageContractV910 || (contract == LanguageContractV920 || (contract == LanguageContractV930 || (contract == LanguageContractV940 || (contract == LanguageContractV950 || (contract == LanguageContractV960 || (contract == LanguageContractV970 || (contract == LanguageContractV980 || (contract == LanguageContractV990 || (contract == LanguageContractV1000 || (contract == LanguageContractV1010 || (contract == LanguageContractV1020 || (contract == LanguageContractV1030 || (contract == LanguageContractV1040 || contract == LanguageContractV1050)))))))))))))))) && validNestedTerminalLeafReturns(function.Body) {
		return nil
	}
	if ((contract == LanguageContractV890 || (contract == LanguageContractV900 || (contract == LanguageContractV910 || (contract == LanguageContractV920 || (contract == LanguageContractV930 || (contract == LanguageContractV940 || (contract == LanguageContractV950 || (contract == LanguageContractV960 || (contract == LanguageContractV970 || (contract == LanguageContractV980 || (contract == LanguageContractV990 || (contract == LanguageContractV1000 || (contract == LanguageContractV1010 || (contract == LanguageContractV1020 || (contract == LanguageContractV1030 || (contract == LanguageContractV1040 || contract == LanguageContractV1050)))))))))))))))) || contract == LanguageContractV880) && validNestedStraightLineReturns(function.Body) {
		return nil
	}
	if ((contract == LanguageContractV890 || (contract == LanguageContractV900 || (contract == LanguageContractV910 || (contract == LanguageContractV920 || (contract == LanguageContractV930 || (contract == LanguageContractV940 || (contract == LanguageContractV950 || (contract == LanguageContractV960 || (contract == LanguageContractV970 || (contract == LanguageContractV980 || (contract == LanguageContractV990 || (contract == LanguageContractV1000 || (contract == LanguageContractV1010 || (contract == LanguageContractV1020 || (contract == LanguageContractV1030 || (contract == LanguageContractV1040 || contract == LanguageContractV1050)))))))))))))))) || contract == LanguageContractV880) || contract == LanguageContractV870 {
		if validTerminalLeafConditionalReturns(function.Body) {
			return nil
		}
		return validateImmutableLocalContract(LanguageContractV860, function)
	}
	if contract == LanguageContractV860 {
		if validConditionalReturnComposition(function.Body) {
			return nil
		}
		return validateImmutableLocalContract(LanguageContractV850, function)
	}
	if contract == LanguageContractV850 {
		if validStraightLineConditionalLocals(function.Body) {
			return nil
		}
		return validateImmutableLocalContract(LanguageContractV840, function)
	}
	if contract == LanguageContractV840 {
		if validConditionalLocalTree(function.Body, 0) {
			return nil
		}
		return validateImmutableLocalContract(LanguageContractV830, function)
	}
	if contract == LanguageContractV830 {
		if validConditionalLocalTree(function.Body, 2) {
			return nil
		}
		return validateImmutableLocalContract(LanguageContractV820, function)
	}
	if contract == LanguageContractV820 {
		if validV820ConditionalLocalTree(function.Body) {
			return nil
		}
		return validateImmutableLocalContract(LanguageContractV810, function)
	}
	if contract == LanguageContractV810 {
		if countTerminalIfStatements(function.Body) > 0 && validV810TerminalTree(function.Body, 3) {
			return nil
		}
		return validateImmutableLocalContract(LanguageContractV800, function)
	}
	if contract == LanguageContractV800 {
		if validV800TwoExpandedTerminalIf(function.Body) {
			return nil
		}
		if countTerminalIfStatements(function.Body) >= 5 {
			return fmt.Errorf("function %s v0.80.0 permits exactly two expanded leaves on a symmetric depth-two terminal base; additional expansion or depth, asymmetric bases, conditional expressions, propagation, match, and fallthrough are excluded", function.Name)
		}
		return validateImmutableLocalContract(LanguageContractV790, function)
	}
	if contract == LanguageContractV790 {
		if validV790BoundedDepthThreeTerminalIf(function.Body) {
			return nil
		}
		if countTerminalIfStatements(function.Body) >= 4 {
			return fmt.Errorf("function %s v0.79.0 permits an inherited rootful or rootless symmetric depth-two terminal if/else with exactly one of its four leaves expanded into one third-level terminal if/else", function.Name)
		}
		return validateImmutableLocalContract(LanguageContractV780, function)
	}
	if contract == LanguageContractV780 {
		if validSymmetricNestedTerminalIf(function.Body, false) {
			return nil
		}
		return validateImmutableLocalContract(LanguageContractV770, function)
	}
	count := countImmutableLocalExpressions(function.Body)
	if count == 0 {
		return nil
	}
	if contract == LanguageContractV770 {
		if countTerminalIfStatements(function.Body) > 1 {
			if validV750NestedTerminalIf(function.Body) || validV760RootLocalNestedTerminalIf(function.Body) || validV770SymmetricRootLocalNestedTerminalIf(function.Body) {
				return nil
			}
			return fmt.Errorf("function %s v0.77.0 permits one or more top-level typed immutable locals before an outer terminal if/else whose two branches each end in one inner terminal if/else; every inner leaf may contain finite local sequences", function.Name)
		}
		return validateImmutableLocalContract(LanguageContractV730, function)
	}
	if contract == LanguageContractV760 {
		if countTerminalIfStatements(function.Body) > 1 {
			if validV750NestedTerminalIf(function.Body) || validV760RootLocalNestedTerminalIf(function.Body) {
				return nil
			}
			return fmt.Errorf("function %s v0.76.0 permits one or more top-level typed immutable locals before the exact v0.75.0 one-branch nested terminal if/else; both nested leaves may contain finite local sequences", function.Name)
		}
		return validateImmutableLocalContract(LanguageContractV730, function)
	}
	if contract == LanguageContractV750 {
		if countTerminalIfStatements(function.Body) > 1 {
			if validV750NestedTerminalIf(function.Body) {
				return nil
			}
			return fmt.Errorf("function %s v0.75.0 permits one complete outer terminal if/else with exactly one branch ending in one nested terminal if/else; inner leaves may contain finite typed immutable-local sequences", function.Name)
		}
		return validateImmutableLocalContract(LanguageContractV730, function)
	}
	if contract == LanguageContractV740 {
		if validV740NestedTerminalIf(function.Body) {
			return nil
		}
		return validateImmutableLocalContract(LanguageContractV730, function)
	}
	if contract != LanguageContractV730 && contract != LanguageContractV720 && contract != LanguageContractV710 && contract != LanguageContractV700 && contract != LanguageContractV690 && contract != LanguageContractV680 && contract != LanguageContractV670 && contract != LanguageContractV660 && contract != LanguageContractV650 && contract != LanguageContractV640 && contract != LanguageContractV630 && contract != LanguageContractV620 && contract != LanguageContractV610 && contract != LanguageContractV600 && contract != LanguageContractV590 && contract != LanguageContractV580 && contract != LanguageContractV570 && contract != LanguageContractV560 && contract != LanguageContractV550 && contract != LanguageContractV540 && contract != LanguageContractV530 && contract != LanguageContractV520 && contract != LanguageContractV510 && contract != LanguageContractV500 && contract != LanguageContractV490 && contract != LanguageContractV390 && contract != LanguageContractV400 && contract != LanguageContractV410 && contract != LanguageContractV420 && contract != LanguageContractV430 && contract != LanguageContractV440 && contract != LanguageContractV450 && contract != LanguageContractV460 && contract != LanguageContractV470 && contract != LanguageContractV480 {
		return fmt.Errorf("function %s immutable local requires language contract %q", function.Name, LanguageContractV390)
	}
	if contract == LanguageContractV390 && (count != 1 || function.Body.Kind != ExprImmutableLocal) {
		return fmt.Errorf("function %s admits exactly one top-level immutable local", function.Name)
	}
	if contract == LanguageContractV730 || contract == LanguageContractV720 || contract == LanguageContractV710 || contract == LanguageContractV700 || contract == LanguageContractV690 || contract == LanguageContractV680 || contract == LanguageContractV670 || contract == LanguageContractV660 || contract == LanguageContractV650 || contract == LanguageContractV640 || contract == LanguageContractV630 || contract == LanguageContractV620 || contract == LanguageContractV610 || contract == LanguageContractV600 || contract == LanguageContractV590 || contract == LanguageContractV580 || contract == LanguageContractV570 || contract == LanguageContractV560 || contract == LanguageContractV550 || contract == LanguageContractV540 || contract == LanguageContractV530 || contract == LanguageContractV520 || contract == LanguageContractV510 || contract == LanguageContractV500 || contract == LanguageContractV490 || contract == LanguageContractV400 || contract == LanguageContractV410 || contract == LanguageContractV420 || contract == LanguageContractV430 || contract == LanguageContractV440 || contract == LanguageContractV450 || contract == LanguageContractV460 || contract == LanguageContractV470 || contract == LanguageContractV480 {
		sequenceCount := 0
		body := function.Body
		for body.Kind == ExprImmutableLocal && body.ImmutableLocal != nil && body.ImmutableLocal.Return != nil {
			sequenceCount++
			body = *body.ImmutableLocal.Return
		}
		branchLocals := 0
		validBranches := true
		if (contract == LanguageContractV730 || contract == LanguageContractV720 || contract == LanguageContractV710 || contract == LanguageContractV700) && body.Kind == ExprConditional && body.Conditional != nil && body.Conditional.TerminalStatement && body.Conditional.WhenTrue != nil && body.Conditional.WhenFalse != nil {
			limit := 1
			if contract == LanguageContractV710 {
				limit = 2
			}
			if contract == LanguageContractV730 || contract == LanguageContractV720 {
				limit = -1
			}
			trueLocals, validTrue := terminalBranchLocalShape(*body.Conditional.WhenTrue, limit)
			falseLocals, validFalse := terminalBranchLocalShape(*body.Conditional.WhenFalse, limit)
			branchLocals = trueLocals + falseLocals
			validBranches = validTrue && validFalse
		}
		if (sequenceCount == 0 && contract != LanguageContractV730) || sequenceCount+branchLocals != count || !validBranches {
			if contract == LanguageContractV730 && body.Kind == ExprConditional && body.Conditional != nil && body.Conditional.TerminalStatement {
				return fmt.Errorf("function %s v0.73.0 terminal if/else may be the complete method body and permits any finite sequence of explicitly typed ordered immutable locals per branch", function.Name)
			}
			if contract == LanguageContractV720 && body.Kind == ExprConditional && body.Conditional != nil && body.Conditional.TerminalStatement {
				return fmt.Errorf("function %s v0.72.0 terminal if/else permits any finite sequence of explicitly typed ordered immutable locals per branch", function.Name)
			}
			if contract == LanguageContractV710 && body.Kind == ExprConditional && body.Conditional != nil && body.Conditional.TerminalStatement {
				return fmt.Errorf("function %s v0.71.0 terminal if/else permits at most two explicitly typed ordered immutable locals per branch", function.Name)
			}
			if contract == LanguageContractV700 && body.Kind == ExprConditional && body.Conditional != nil && body.Conditional.TerminalStatement {
				return fmt.Errorf("function %s v0.70.0 terminal if/else permits at most one explicitly typed immutable local per branch", function.Name)
			}
			return fmt.Errorf("function %s admits only one top-level ordered immutable-local sequence", function.Name)
		}
	}
	if exprContainsPropagation(function.Body) && contract != LanguageContractV730 && contract != LanguageContractV720 && contract != LanguageContractV710 && contract != LanguageContractV700 && contract != LanguageContractV690 && contract != LanguageContractV680 && contract != LanguageContractV670 && contract != LanguageContractV660 && contract != LanguageContractV650 && contract != LanguageContractV640 && contract != LanguageContractV630 && contract != LanguageContractV620 && contract != LanguageContractV610 && contract != LanguageContractV600 && contract != LanguageContractV590 && contract != LanguageContractV580 && contract != LanguageContractV570 && contract != LanguageContractV560 && contract != LanguageContractV550 && contract != LanguageContractV540 && contract != LanguageContractV530 && contract != LanguageContractV520 && contract != LanguageContractV510 && contract != LanguageContractV500 && contract != LanguageContractV490 && contract != LanguageContractV410 && contract != LanguageContractV420 && contract != LanguageContractV430 && contract != LanguageContractV440 && contract != LanguageContractV450 && contract != LanguageContractV460 && contract != LanguageContractV470 && contract != LanguageContractV480 {
		return fmt.Errorf("function %s immutable local initializer and return exclude propagation", function.Name)
	}
	if exprContainsPropagation(function.Body) {
		directFirst := function.Body.Kind == ExprImmutableLocal && function.Body.ImmutableLocal != nil && function.Body.ImmutableLocal.Initializer != nil && function.Body.ImmutableLocal.Initializer.Kind == ExprPropagate
		oneStageContextual := ((contract == LanguageContractV730 || contract == LanguageContractV720 || contract == LanguageContractV710 || contract == LanguageContractV700 || contract == LanguageContractV690 || contract == LanguageContractV680) && len(function.Parameters) >= 2) || ((contract == LanguageContractV670 || contract == LanguageContractV660 || contract == LanguageContractV650 || contract == LanguageContractV640 || contract == LanguageContractV630) && len(function.Parameters) == 2)
		if oneStageContextual && directFirst && isBoundedValueResultType(function.Parameters[0].Type) && isBoundedValueResultType(function.ReturnType) && countPropagationExpressions(function.Body) == 1 {
			return validateSingleStageContextualCrossPayloadResultPropagationContract(contract, function)
		}
		if (contract == LanguageContractV730 || contract == LanguageContractV720 || contract == LanguageContractV710 || contract == LanguageContractV700 || contract == LanguageContractV690 || contract == LanguageContractV680 || contract == LanguageContractV670 || contract == LanguageContractV660 || contract == LanguageContractV650 || contract == LanguageContractV640 || contract == LanguageContractV630 || contract == LanguageContractV620) && directFirst && len(function.Parameters) >= 2 && isBoundedValueResultType(function.Parameters[0].Type) && isBoundedValueResultType(function.ReturnType) && countPropagationExpressions(function.Body) >= 2 {
			return validateContextualCrossPayloadResultPropagationContract(contract, function)
		}
		if (contract == LanguageContractV730 || contract == LanguageContractV720 || contract == LanguageContractV710 || contract == LanguageContractV700 || contract == LanguageContractV690 || contract == LanguageContractV680 || contract == LanguageContractV670 || contract == LanguageContractV660 || contract == LanguageContractV650 || contract == LanguageContractV640 || contract == LanguageContractV630 || contract == LanguageContractV620 || contract == LanguageContractV610) && directFirst && len(function.Parameters) > 0 && isBoundedValueResultType(function.Parameters[0].Type) && isBoundedValueResultType(function.ReturnType) && countPropagationExpressions(function.Body) >= 2 {
			return validateGeneralizedCrossPayloadResultPropagationContract(function)
		}
		if contract == LanguageContractV600 && directFirst && len(function.Parameters) > 0 && isBoundedValueResultType(function.Parameters[0].Type) && isBoundedValueResultType(function.ReturnType) && countPropagationExpressions(function.Body) == 2 {
			return validateTwoStageCrossPayloadResultPropagationContract(function)
		}
		if (contract == LanguageContractV730 || contract == LanguageContractV720 || contract == LanguageContractV710 || contract == LanguageContractV700 || contract == LanguageContractV690 || contract == LanguageContractV680 || contract == LanguageContractV670 || contract == LanguageContractV660 || contract == LanguageContractV650 || contract == LanguageContractV640 || contract == LanguageContractV630 || contract == LanguageContractV620 || contract == LanguageContractV610 || contract == LanguageContractV600 || contract == LanguageContractV590) && directFirst && len(function.Parameters) > 0 && isBoundedValueResultType(function.Parameters[0].Type) && isBoundedValueResultType(function.ReturnType) && !TypeEqual(function.Parameters[0].Type.Result.Success, function.ReturnType.Result.Success) {
			return validateCrossPayloadResultPropagationContract(contract, function)
		}
		if (contract == LanguageContractV730 || contract == LanguageContractV720 || contract == LanguageContractV710 || contract == LanguageContractV700 || contract == LanguageContractV690 || contract == LanguageContractV680 || contract == LanguageContractV670 || contract == LanguageContractV660 || contract == LanguageContractV650 || contract == LanguageContractV640 || contract == LanguageContractV630 || contract == LanguageContractV620 || contract == LanguageContractV610 || contract == LanguageContractV600 || contract == LanguageContractV590 || contract == LanguageContractV580) && directFirst && (len(function.Parameters) >= 3 || countPropagationExpressions(function.Body) != 1) {
			return validateCheckedPropagationChainContract(function)
		}
		if contract == LanguageContractV570 && directFirst && (len(function.Parameters) == 3 || countPropagationExpressions(function.Body) != 1) {
			return validateTwoStageCheckedPropagationContract(function)
		}
		if ((contract == LanguageContractV730 || contract == LanguageContractV720 || contract == LanguageContractV710 || contract == LanguageContractV700 || contract == LanguageContractV690 || contract == LanguageContractV680 || contract == LanguageContractV670 || contract == LanguageContractV660 || contract == LanguageContractV650 || contract == LanguageContractV640 || contract == LanguageContractV630 || contract == LanguageContractV620 || contract == LanguageContractV610 || contract == LanguageContractV600 || contract == LanguageContractV590 || contract == LanguageContractV580) || contract == LanguageContractV570 || contract == LanguageContractV560 || contract == LanguageContractV550 || contract == LanguageContractV540 || contract == LanguageContractV530 || contract == LanguageContractV520 || contract == LanguageContractV510 || contract == LanguageContractV500 || contract == LanguageContractV490 || contract == LanguageContractV420 || contract == LanguageContractV430 || contract == LanguageContractV440 || contract == LanguageContractV450 || contract == LanguageContractV460 || contract == LanguageContractV470 || contract == LanguageContractV480) && function.Body.Kind == ExprImmutableLocal && function.Body.ImmutableLocal != nil && function.Body.ImmutableLocal.Initializer != nil && function.Body.ImmutableLocal.Initializer.Kind != ExprPropagate {
			return validatePriorLocalBlockPropagationContract(contract, function)
		}
		return validateBlockPropagationContract(contract, function)
	}
	return nil
}

func validateGeneralizedCrossPayloadResultPropagationContract(function Function) error {
	return validateCrossPayloadResultPropagationChainContract("", function, false)
}

func validateContextualCrossPayloadResultPropagationContract(contract string, function Function) error {
	return validateCrossPayloadResultPropagationChainContract(contract, function, true)
}

func validateCrossPayloadResultPropagationChainContract(contract string, function Function, contextual bool) error {
	feature := "v0.61.0 generalized cross-payload Result propagation"
	expectedParameters := 1
	multiContext := contextual && (contract == LanguageContractV730 || contract == LanguageContractV720 || contract == LanguageContractV710 || contract == LanguageContractV700 || contract == LanguageContractV690 || contract == LanguageContractV680 || contract == LanguageContractV670)
	if contextual {
		feature = "v0.62.0 contextual cross-payload Result propagation"
		expectedParameters = 2
	}
	propagationCount := countPropagationExpressions(function.Body)
	samePayloadAdmitted := contextual && (contract == LanguageContractV730 || contract == LanguageContractV720 || contract == LanguageContractV710 || contract == LanguageContractV700 || contract == LanguageContractV690 || contract == LanguageContractV680 || contract == LanguageContractV670 || contract == LanguageContractV660 || (contract == LanguageContractV650 && propagationCount == 2))
	if multiContext {
		feature = "v0.67.0 generalized shared-context Result propagation"
		expectedParameters = len(function.Parameters)
	} else if contextual && contract == LanguageContractV660 {
		feature = "v0.66.0 generalized contextual bounded Result propagation"
	} else if samePayloadAdmitted {
		feature = "v0.65.0 exact two-stage contextual bounded Result propagation"
	}
	fail := func(detail string) error {
		return fmt.Errorf("function %s %s %s", function.Name, feature, detail)
	}
	first := function.Body.ImmutableLocal
	if len(function.Parameters) != expectedParameters || (multiContext && len(function.Parameters) < 2) || first == nil || first.Initializer == nil || first.Initializer.Kind != ExprPropagate || first.Initializer.Propagate == nil || propagationCount < 2 {
		if contextual {
			if multiContext {
				return fail("requires one direct carrier, one or more direct string context parameters, and at least two canonical propagations")
			}
			return fail("requires one direct carrier, one direct string context parameter, and at least two canonical propagations")
		}
		return fail("requires one direct carrier parameter and at least two canonical propagations")
	}
	source := function.Parameters[0].Type
	target := function.ReturnType
	if !isBoundedValueResultType(source) || !isBoundedValueResultType(target) || source.Result.Failure.Kind != TypePrimitive || source.Result.Failure.Primitive != PrimitiveString || !TypeEqual(source.Result.Failure, target.Result.Failure) {
		return fail("requires bounded source and target Results with one shared string failure type")
	}
	if contextual {
		for position := 1; position < len(function.Parameters); position++ {
			if !TypeEqual(function.Parameters[position].Type, Type{Kind: TypePrimitive, Primitive: PrimitiveString}) {
				return fail(fmt.Sprintf("requires direct context parameter %d to be exactly string", position))
			}
		}
	}
	if first.Initializer.Propagate.Value == nil || !directReference(first.Initializer.Propagate.Value) || *first.Initializer.Propagate.Value.Parameter != 0 || !TypeEqual(first.Initializer.Propagate.Carrier, source) || first.Position != expectedParameters || !TypeEqual(first.Type, source.Result.Success) || !TypeEqual(first.Initializer.Type, source.Result.Success) {
		return fail("requires the direct incoming carrier and exact first payload local")
	}
	validHelperArguments := func(call *Call, payloadPosition int, payloadType Type) bool {
		expectedArguments := 1
		if contextual {
			expectedArguments = len(function.Parameters)
		}
		if call == nil || len(call.Arguments) != expectedArguments {
			return false
		}
		payload := call.Arguments[0]
		if payload == nil || !directReference(payload) || *payload.Parameter != payloadPosition || !TypeEqual(payload.Type, payloadType) {
			return false
		}
		if !contextual {
			return true
		}
		for position := 1; position < len(function.Parameters); position++ {
			context := call.Arguments[position]
			if context == nil || !directReference(context) || *context.Parameter != position || !TypeEqual(context.Type, function.Parameters[position].Type) {
				return false
			}
		}
		return true
	}

	payloadLocal := first
	payloadType := source.Result.Success
	observedPropagations := 1
	stage := 1
	for {
		if payloadLocal.Return == nil {
			return fail(fmt.Sprintf("stage %d has no helper continuation", stage))
		}
		if payloadLocal.Return.Kind == ExprCall {
			call := payloadLocal.Return.Call
			if stage < 2 || observedPropagations != propagationCount || call == nil || !TypeEqual(payloadLocal.Return.Type, target) || (!samePayloadAdmitted && TypeEqual(payloadType, target.Result.Success)) {
				return fail("requires a terminal helper call after at least two contiguous stages; equal payloads require v0.66.0 or the exact v0.65.0 two-stage contextual form")
			}
			if !validHelperArguments(call, payloadLocal.Position, payloadType) {
				return fail(fmt.Sprintf("stage %d terminal helper requires the direct preceding payload local followed by the canonical context argument", stage))
			}
			return nil
		}
		if payloadLocal.Return.Kind != ExprImmutableLocal || payloadLocal.Return.ImmutableLocal == nil {
			return fail(fmt.Sprintf("stage %d requires an explicit helper Result carrier local", stage))
		}
		carrierLocal := payloadLocal.Return.ImmutableLocal
		expectedCarrierPosition := payloadLocal.Position + 1
		if carrierLocal.Position != expectedCarrierPosition || !isBoundedValueResultType(carrierLocal.Type) || carrierLocal.Initializer == nil || carrierLocal.Initializer.Kind != ExprCall || carrierLocal.Initializer.Call == nil || !TypeEqual(carrierLocal.Initializer.Type, carrierLocal.Type) || !TypeEqual(carrierLocal.Type.Result.Failure, source.Result.Failure) || (!samePayloadAdmitted && TypeEqual(payloadType, carrierLocal.Type.Result.Success)) {
			return fail(fmt.Sprintf("stage %d requires one canonical helper Result carrier local; equal payloads require v0.66.0 or the exact v0.65.0 two-stage contextual form", stage))
		}
		if !validHelperArguments(carrierLocal.Initializer.Call, payloadLocal.Position, payloadType) {
			return fail(fmt.Sprintf("stage %d helper requires the direct preceding payload local followed by the canonical context argument", stage))
		}
		if carrierLocal.Return == nil || carrierLocal.Return.Kind != ExprImmutableLocal || carrierLocal.Return.ImmutableLocal == nil {
			return fail(fmt.Sprintf("stage %d carrier must be propagated by the immediately following payload local", stage))
		}
		nextPayload := carrierLocal.Return.ImmutableLocal
		if nextPayload.Position != carrierLocal.Position+1 || !TypeEqual(nextPayload.Type, carrierLocal.Type.Result.Success) || nextPayload.Initializer == nil || nextPayload.Initializer.Kind != ExprPropagate || nextPayload.Initializer.Propagate == nil || !TypeEqual(nextPayload.Initializer.Type, nextPayload.Type) || !TypeEqual(nextPayload.Initializer.Propagate.Carrier, carrierLocal.Type) || nextPayload.Initializer.Propagate.Value == nil || !directReference(nextPayload.Initializer.Propagate.Value) || *nextPayload.Initializer.Propagate.Value.Parameter != carrierLocal.Position {
			return fail(fmt.Sprintf("propagation after stage %d must directly consume its explicit carrier local", stage))
		}
		payloadLocal = nextPayload
		payloadType = carrierLocal.Type.Result.Success
		observedPropagations++
		stage++
	}
}

func validateTwoStageCrossPayloadResultPropagationContract(function Function) error {
	first := function.Body.ImmutableLocal
	if first == nil || first.Initializer == nil || first.Initializer.Kind != ExprPropagate || first.Initializer.Propagate == nil || countPropagationExpressions(function.Body) != 2 {
		return fmt.Errorf("function %s v0.60.0 two-stage cross-payload Result propagation requires exactly two canonical propagations", function.Name)
	}
	if len(function.Parameters) != 1 || !isBoundedValueResultType(function.Parameters[0].Type) || !isBoundedValueResultType(function.ReturnType) {
		return fmt.Errorf("function %s v0.60.0 two-stage cross-payload Result propagation requires one bounded Result parameter and bounded Result return", function.Name)
	}
	source := function.Parameters[0].Type
	target := function.ReturnType
	if source.Result.Failure.Kind != TypePrimitive || source.Result.Failure.Primitive != PrimitiveString || !TypeEqual(source.Result.Failure, target.Result.Failure) {
		return fmt.Errorf("function %s v0.60.0 two-stage cross-payload Result propagation requires one shared string failure type", function.Name)
	}
	if first.Initializer.Propagate.Value == nil || !directReference(first.Initializer.Propagate.Value) || *first.Initializer.Propagate.Value.Parameter != 0 || !TypeEqual(first.Initializer.Propagate.Carrier, source) || first.Position != 1 || !TypeEqual(first.Type, source.Result.Success) || !TypeEqual(first.Initializer.Type, source.Result.Success) {
		return fmt.Errorf("function %s v0.60.0 two-stage cross-payload Result propagation requires the direct incoming carrier and exact first payload local", function.Name)
	}
	if first.Return == nil || first.Return.Kind != ExprImmutableLocal || first.Return.ImmutableLocal == nil {
		return fmt.Errorf("function %s v0.60.0 two-stage cross-payload Result propagation requires an explicit intermediate carrier local", function.Name)
	}
	intermediate := first.Return.ImmutableLocal
	if intermediate.Position != 2 || !isBoundedValueResultType(intermediate.Type) || intermediate.Initializer == nil || intermediate.Initializer.Kind != ExprCall || intermediate.Initializer.Call == nil || !TypeEqual(intermediate.Initializer.Type, intermediate.Type) || len(intermediate.Initializer.Call.Arguments) != 1 {
		return fmt.Errorf("function %s v0.60.0 two-stage cross-payload Result propagation requires a canonical first-helper carrier local", function.Name)
	}
	firstArgument := intermediate.Initializer.Call.Arguments[0]
	if firstArgument == nil || !directReference(firstArgument) || *firstArgument.Parameter != first.Position || !TypeEqual(firstArgument.Type, source.Result.Success) || TypeEqual(source.Result.Success, intermediate.Type.Result.Success) || TypeEqual(intermediate.Type.Result.Success, target.Result.Success) || !TypeEqual(intermediate.Type.Result.Failure, source.Result.Failure) {
		return fmt.Errorf("function %s v0.60.0 two-stage cross-payload Result propagation requires adjacent distinct payloads and the first propagated local as the sole first-helper argument", function.Name)
	}
	if intermediate.Return == nil || intermediate.Return.Kind != ExprImmutableLocal || intermediate.Return.ImmutableLocal == nil {
		return fmt.Errorf("function %s v0.60.0 two-stage cross-payload Result propagation requires a second propagated payload local", function.Name)
	}
	second := intermediate.Return.ImmutableLocal
	if second.Position != 3 || !TypeEqual(second.Type, intermediate.Type.Result.Success) || second.Initializer == nil || second.Initializer.Kind != ExprPropagate || second.Initializer.Propagate == nil || !TypeEqual(second.Initializer.Type, second.Type) || !TypeEqual(second.Initializer.Propagate.Carrier, intermediate.Type) || second.Initializer.Propagate.Value == nil || !directReference(second.Initializer.Propagate.Value) || *second.Initializer.Propagate.Value.Parameter != intermediate.Position {
		return fmt.Errorf("function %s v0.60.0 two-stage cross-payload Result propagation requires direct propagation of the explicit intermediate carrier", function.Name)
	}
	if second.Return == nil || second.Return.Kind != ExprCall || second.Return.Call == nil || !TypeEqual(second.Return.Type, target) || len(second.Return.Call.Arguments) != 1 {
		return fmt.Errorf("function %s v0.60.0 two-stage cross-payload Result propagation requires one terminal second-helper call", function.Name)
	}
	secondArgument := second.Return.Call.Arguments[0]
	if secondArgument == nil || !directReference(secondArgument) || *secondArgument.Parameter != second.Position || !TypeEqual(secondArgument.Type, intermediate.Type.Result.Success) {
		return fmt.Errorf("function %s v0.60.0 two-stage cross-payload Result propagation requires the second propagated local as the sole second-helper argument", function.Name)
	}
	return nil
}

func validateCrossPayloadResultPropagationContract(contract string, function Function) error {
	return validateCrossPayloadResultPropagationFormContract(contract, function, false)
}

func validateSingleStageContextualCrossPayloadResultPropagationContract(contract string, function Function) error {
	return validateCrossPayloadResultPropagationFormContract(contract, function, true)
}

func validateCrossPayloadResultPropagationFormContract(contract string, function Function, contextual bool) error {
	feature := "v0.59.0 cross-payload Result propagation"
	expectedParameters := 1
	multiContext := contextual && (contract == LanguageContractV730 || contract == LanguageContractV720 || contract == LanguageContractV710 || contract == LanguageContractV700 || contract == LanguageContractV690 || contract == LanguageContractV680)
	if contextual {
		feature = "v0.63.0 one-stage contextual cross-payload Result propagation"
		if multiContext {
			feature = "v0.68.0 generalized one-stage shared-context Result propagation"
			expectedParameters = len(function.Parameters)
		} else if contract == LanguageContractV670 || contract == LanguageContractV660 || contract == LanguageContractV650 || contract == LanguageContractV640 {
			feature = "v0.64.0 one-stage contextual bounded Result propagation"
		}
		if !multiContext {
			expectedParameters = 2
		}
	}
	fail := func(detail string) error {
		return fmt.Errorf("function %s %s %s", function.Name, feature, detail)
	}
	first := function.Body.ImmutableLocal
	if first == nil || first.Initializer == nil || first.Initializer.Kind != ExprPropagate || first.Initializer.Propagate == nil || countPropagationExpressions(function.Body) != 1 {
		return fail("requires exactly one first-local propagation")
	}
	if len(function.Parameters) != expectedParameters || (multiContext && len(function.Parameters) < 2) || !isBoundedValueResultType(function.Parameters[0].Type) || !isBoundedValueResultType(function.ReturnType) {
		return fail("requires the exact direct parameter shape and bounded Result return")
	}
	carrier := function.Parameters[0].Type
	target := function.ReturnType
	samePayloadAdmitted := contextual && (contract == LanguageContractV730 || contract == LanguageContractV720 || contract == LanguageContractV710 || contract == LanguageContractV700 || contract == LanguageContractV690 || contract == LanguageContractV680 || contract == LanguageContractV670 || contract == LanguageContractV660 || contract == LanguageContractV650 || contract == LanguageContractV640)
	if (!samePayloadAdmitted && TypeEqual(carrier.Result.Success, target.Result.Success)) || !TypeEqual(carrier.Result.Failure, target.Result.Failure) || carrier.Result.Failure.Kind != TypePrimitive || carrier.Result.Failure.Primitive != PrimitiveString {
		return fail("requires admitted payloads and the same string failure type; equal payloads require v0.64.0 contextual propagation")
	}
	if contextual {
		for position := 1; position < len(function.Parameters); position++ {
			if !TypeEqual(function.Parameters[position].Type, Type{Kind: TypePrimitive, Primitive: PrimitiveString}) {
				return fail(fmt.Sprintf("requires direct context parameter %d to be exactly string", position))
			}
		}
	}
	propagated := first.Initializer.Propagate
	if propagated.Value == nil || !directReference(propagated.Value) || *propagated.Value.Parameter != 0 || !TypeEqual(propagated.Value.Type, carrier) || !TypeEqual(propagated.Carrier, carrier) {
		return fail("requires the direct carrier parameter")
	}
	if first.Position != expectedParameters || !TypeEqual(first.Type, carrier.Result.Success) || !TypeEqual(first.Initializer.Type, carrier.Result.Success) {
		return fail("local must exactly match the source success payload")
	}
	if first.Return == nil || first.Return.Kind != ExprCall || first.Return.Call == nil || !TypeEqual(first.Return.Type, target) || len(first.Return.Call.Arguments) != expectedParameters {
		return fail("requires one terminal helper call returning the target Result")
	}
	argument := first.Return.Call.Arguments[0]
	if argument == nil || !directReference(argument) || *argument.Parameter != first.Position || !TypeEqual(argument.Type, carrier.Result.Success) {
		return fail("requires the direct propagated local as the first helper argument")
	}
	if contextual {
		for position := 1; position < len(function.Parameters); position++ {
			context := first.Return.Call.Arguments[position]
			if context == nil || !directReference(context) || *context.Parameter != position || !TypeEqual(context.Type, function.Parameters[position].Type) {
				return fail("requires every direct string context parameter exactly once in declaration order after the payload")
			}
		}
	}
	return nil
}

func validateCheckedPropagationChainContract(function Function) error {
	stageCount := len(function.Parameters) - 1
	if stageCount < 2 {
		return fmt.Errorf("function %s v0.58.0 checked propagation chain requires a carrier and at least two payload operands", function.Name)
	}
	if countPropagationExpressions(function.Body) != stageCount {
		return fmt.Errorf("function %s v0.58.0 checked propagation chain requires one propagation for the incoming carrier and every non-terminal checked stage", function.Name)
	}
	first := function.Body.ImmutableLocal
	if first == nil || first.Initializer == nil || first.Initializer.Kind != ExprPropagate || first.Initializer.Propagate == nil {
		return fmt.Errorf("function %s v0.58.0 checked propagation chain requires the incoming carrier propagation first", function.Name)
	}
	carrier := function.ReturnType
	firstPropagation := first.Initializer.Propagate
	if !isArithmeticResultType(carrier) || !TypeEqual(function.Parameters[0].Type, carrier) || !TypeEqual(firstPropagation.Carrier, carrier) || firstPropagation.Value == nil || !directReference(firstPropagation.Value) || *firstPropagation.Value.Parameter != 0 {
		return fmt.Errorf("function %s v0.58.0 checked propagation chain requires the first direct arithmetic Result carrier", function.Name)
	}
	payload := carrier.Result.Success
	if first.Position != len(function.Parameters) || !TypeEqual(first.Type, payload) || !TypeEqual(first.Initializer.Type, payload) {
		return fmt.Errorf("function %s v0.58.0 first propagated local must match the arithmetic payload", function.Name)
	}
	for position := 1; position < len(function.Parameters); position++ {
		if !TypeEqual(function.Parameters[position].Type, payload) {
			return fmt.Errorf("function %s v0.58.0 checked propagation chain operand types must match the propagated payload", function.Name)
		}
	}

	payloadLocal := first
	for stage := 1; stage < stageCount; stage++ {
		if payloadLocal.Return == nil || payloadLocal.Return.Kind != ExprImmutableLocal || payloadLocal.Return.ImmutableLocal == nil {
			return fmt.Errorf("function %s v0.58.0 checked stage %d requires an explicit arithmetic Result carrier local", function.Name, stage)
		}
		checkedCarrier := payloadLocal.Return.ImmutableLocal
		if checkedCarrier.Position != payloadLocal.Position+1 || !TypeEqual(checkedCarrier.Type, carrier) || checkedCarrier.Initializer == nil || !TypeEqual(checkedCarrier.Initializer.Type, carrier) || !isAdmittedMultiParameterCheckedArithmeticContinuation(*checkedCarrier.Initializer, payload, payloadLocal.Position, stage) {
			return fmt.Errorf("function %s v0.58.0 checked stage %d requires local %d as left operand and parameter %d as right operand", function.Name, stage, payloadLocal.Position, stage)
		}
		if checkedCarrier.Return == nil || checkedCarrier.Return.Kind != ExprImmutableLocal || checkedCarrier.Return.ImmutableLocal == nil {
			return fmt.Errorf("function %s v0.58.0 checked stage %d carrier must be propagated by the immediately following payload local", function.Name, stage)
		}
		nextPayload := checkedCarrier.Return.ImmutableLocal
		if nextPayload.Position != checkedCarrier.Position+1 || !TypeEqual(nextPayload.Type, payload) || nextPayload.Initializer == nil || nextPayload.Initializer.Kind != ExprPropagate || nextPayload.Initializer.Propagate == nil {
			return fmt.Errorf("function %s v0.58.0 checked stage %d carrier must initialize the next payload through propagation", function.Name, stage)
		}
		nextPropagation := nextPayload.Initializer.Propagate
		if !TypeEqual(nextPropagation.Carrier, carrier) || nextPropagation.Value == nil || !directReference(nextPropagation.Value) || *nextPropagation.Value.Parameter != checkedCarrier.Position {
			return fmt.Errorf("function %s v0.58.0 propagation after checked stage %d must directly consume its explicit carrier local", function.Name, stage)
		}
		payloadLocal = nextPayload
	}
	if payloadLocal.Return == nil || !isAdmittedMultiParameterCheckedArithmeticContinuation(*payloadLocal.Return, payload, payloadLocal.Position, stageCount) {
		return fmt.Errorf("function %s v0.58.0 terminal checked stage requires local %d as left operand and parameter %d as right operand", function.Name, payloadLocal.Position, stageCount)
	}
	return nil
}

func countPropagationExpressions(expression Expr) int {
	count := 0
	if expression.Kind == ExprPropagate {
		count++
	}
	for _, child := range expressionChildren(expression) {
		if child != nil {
			count += countPropagationExpressions(*child)
		}
	}
	return count
}

func validateBlockPropagationContract(contract string, function Function) error {
	first := function.Body.ImmutableLocal
	if function.Body.Kind != ExprImmutableLocal || first == nil || first.Initializer == nil || first.Initializer.Kind != ExprPropagate || first.Initializer.Propagate == nil || countPropagationExpressions(function.Body) != 1 {
		return fmt.Errorf("function %s admits exactly one propagation as the first immutable-local initializer", function.Name)
	}
	propagated := first.Initializer.Propagate
	if len(function.Parameters) == 0 || !directReference(propagated.Value) || *propagated.Value.Parameter != 0 {
		return fmt.Errorf("function %s block propagation requires its sole direct carrier parameter", function.Name)
	}
	carrier := function.ReturnType
	if !TypeEqual(function.Parameters[0].Type, carrier) || !TypeEqual(propagated.Carrier, carrier) || !TypeEqual(propagated.Value.Type, carrier) {
		return fmt.Errorf("function %s propagated carrier parameter and return type must be identical", function.Name)
	}
	var payload Type
	switch {
	case carrier.Kind == TypeOptional && carrier.Optional != nil && isOptionalValueType(carrier.Optional.Value):
		payload = carrier.Optional.Value
	case isBoundedValueResultType(carrier):
		payload = carrier.Result.Success
	case ((contract == LanguageContractV730 || contract == LanguageContractV720 || contract == LanguageContractV710 || contract == LanguageContractV700 || contract == LanguageContractV690 || contract == LanguageContractV680 || contract == LanguageContractV670 || contract == LanguageContractV660 || contract == LanguageContractV650 || contract == LanguageContractV640 || contract == LanguageContractV630 || contract == LanguageContractV620 || contract == LanguageContractV610 || contract == LanguageContractV600 || contract == LanguageContractV590 || contract == LanguageContractV580) || contract == LanguageContractV570 || contract == LanguageContractV560 || contract == LanguageContractV550) && isArithmeticResultType(carrier):
		payload = carrier.Result.Success
	default:
		if (contract == LanguageContractV730 || contract == LanguageContractV720 || contract == LanguageContractV710 || contract == LanguageContractV700 || contract == LanguageContractV690 || contract == LanguageContractV680 || contract == LanguageContractV670 || contract == LanguageContractV660 || contract == LanguageContractV650 || contract == LanguageContractV640 || contract == LanguageContractV630 || contract == LanguageContractV620 || contract == LanguageContractV610 || contract == LanguageContractV600 || contract == LanguageContractV590 || contract == LanguageContractV580) || contract == LanguageContractV570 || contract == LanguageContractV560 || contract == LanguageContractV550 {
			return fmt.Errorf("function %s block propagation requires an admitted Optional, bounded Result, or checked-arithmetic Result", function.Name)
		}
		return fmt.Errorf("function %s block propagation requires an admitted Optional or bounded Result", function.Name)
	}
	if !TypeEqual(first.Type, payload) || !TypeEqual(first.Initializer.Type, payload) {
		return fmt.Errorf("function %s first immutable local type must match the propagated success payload", function.Name)
	}
	multiParameterArithmetic := ((contract == LanguageContractV730 || contract == LanguageContractV720 || contract == LanguageContractV710 || contract == LanguageContractV700 || contract == LanguageContractV690 || contract == LanguageContractV680 || contract == LanguageContractV670 || contract == LanguageContractV660 || contract == LanguageContractV650 || contract == LanguageContractV640 || contract == LanguageContractV630 || contract == LanguageContractV620 || contract == LanguageContractV610 || contract == LanguageContractV600 || contract == LanguageContractV590 || contract == LanguageContractV580) || contract == LanguageContractV570 || contract == LanguageContractV560) && isArithmeticResultType(carrier) && len(function.Parameters) == 2
	if len(function.Parameters) != 1 && !multiParameterArithmetic {
		if ((contract == LanguageContractV730 || contract == LanguageContractV720 || contract == LanguageContractV710 || contract == LanguageContractV700 || contract == LanguageContractV690 || contract == LanguageContractV680 || contract == LanguageContractV670 || contract == LanguageContractV660 || contract == LanguageContractV650 || contract == LanguageContractV640 || contract == LanguageContractV630 || contract == LanguageContractV620 || contract == LanguageContractV610 || contract == LanguageContractV600 || contract == LanguageContractV590 || contract == LanguageContractV580) || contract == LanguageContractV570 || contract == LanguageContractV560) && isArithmeticResultType(carrier) {
			return fmt.Errorf("function %s multi-parameter direct checked-arithmetic propagation requires exactly carrier and payload operand parameters", function.Name)
		}
		return fmt.Errorf("function %s block propagation requires its sole direct carrier parameter", function.Name)
	}
	if multiParameterArithmetic {
		if !TypeEqual(function.Parameters[1].Type, payload) {
			return fmt.Errorf("function %s direct checked-arithmetic operand type must match the propagated payload", function.Name)
		}
		if !isAdmittedMultiParameterCheckedArithmeticContinuation(*first.Return, payload, first.Position, 1) {
			return fmt.Errorf("function %s multi-parameter direct checked-arithmetic propagation requires the propagated local as left operand and parameter 1 as right operand", function.Name)
		}
	} else if ((contract == LanguageContractV730 || contract == LanguageContractV720 || contract == LanguageContractV710 || contract == LanguageContractV700 || contract == LanguageContractV690 || contract == LanguageContractV680 || contract == LanguageContractV670 || contract == LanguageContractV660 || contract == LanguageContractV650 || contract == LanguageContractV640 || contract == LanguageContractV630 || contract == LanguageContractV620 || contract == LanguageContractV610 || contract == LanguageContractV600 || contract == LanguageContractV590 || contract == LanguageContractV580) || contract == LanguageContractV570 || contract == LanguageContractV560 || contract == LanguageContractV550) && isArithmeticResultType(carrier) && !isAdmittedCheckedArithmeticContinuation(*first.Return, payload) {
		return fmt.Errorf("function %s direct checked-arithmetic propagation requires one admitted checked arithmetic continuation", function.Name)
	}
	return nil
}

func isAdmittedMultiParameterCheckedArithmeticContinuation(expression Expr, payload Type, localPosition, operandPosition int) bool {
	if !isArithmeticResultType(expression.Type) || expression.Type.Result == nil || !TypeEqual(expression.Type.Result.Success, payload) || expression.Kind != ExprBinary || expression.Binary == nil {
		return false
	}
	left := expression.Binary.Left
	right := expression.Binary.Right
	if !directReference(left) || *left.Parameter != localPosition || !directReference(right) || *right.Parameter != operandPosition {
		return false
	}
	if TypeEqual(payload, BinaryFloat(64)) {
		return expression.Binary.Operator == OperatorDivide
	}
	return TypeEqual(payload, SignedInteger(64)) && (expression.Binary.Operator == OperatorAdd || expression.Binary.Operator == OperatorSubtract || expression.Binary.Operator == OperatorMultiply)
}

func validateTwoStageCheckedPropagationContract(function Function) error {
	if countPropagationExpressions(function.Body) != 2 {
		return fmt.Errorf("function %s v0.57.0 two-stage checked propagation requires exactly two propagations", function.Name)
	}
	if len(function.Parameters) != 3 {
		return fmt.Errorf("function %s v0.57.0 two-stage checked propagation requires exactly carrier and two payload operands", function.Name)
	}
	first := function.Body.ImmutableLocal
	if first == nil || first.Initializer == nil || first.Initializer.Kind != ExprPropagate || first.Initializer.Propagate == nil {
		return fmt.Errorf("function %s v0.57.0 two-stage checked propagation requires the incoming carrier propagation first", function.Name)
	}
	carrier := function.ReturnType
	firstPropagation := first.Initializer.Propagate
	if !isArithmeticResultType(carrier) || !TypeEqual(function.Parameters[0].Type, carrier) || !TypeEqual(firstPropagation.Carrier, carrier) || firstPropagation.Value == nil || !directReference(firstPropagation.Value) || *firstPropagation.Value.Parameter != 0 {
		return fmt.Errorf("function %s v0.57.0 two-stage checked propagation requires the first direct arithmetic Result carrier", function.Name)
	}
	payload := carrier.Result.Success
	if first.Position != len(function.Parameters) || !TypeEqual(first.Type, payload) || !TypeEqual(first.Initializer.Type, payload) {
		return fmt.Errorf("function %s v0.57.0 first propagated local must match the arithmetic payload", function.Name)
	}
	if !TypeEqual(function.Parameters[1].Type, payload) || !TypeEqual(function.Parameters[2].Type, payload) {
		return fmt.Errorf("function %s v0.57.0 two-stage checked propagation operand types must match the propagated payload", function.Name)
	}
	if first.Return == nil || first.Return.Kind != ExprImmutableLocal || first.Return.ImmutableLocal == nil {
		return fmt.Errorf("function %s v0.57.0 two-stage checked propagation requires an explicit checked carrier local", function.Name)
	}
	checkedCarrier := first.Return.ImmutableLocal
	if checkedCarrier.Position != first.Position+1 || !TypeEqual(checkedCarrier.Type, carrier) || checkedCarrier.Initializer == nil || !TypeEqual(checkedCarrier.Initializer.Type, carrier) || !isAdmittedMultiParameterCheckedArithmeticContinuation(*checkedCarrier.Initializer, payload, first.Position, 1) {
		return fmt.Errorf("function %s v0.57.0 first checked stage requires local %d as left operand and parameter 1 as right operand", function.Name, first.Position)
	}
	if checkedCarrier.Return == nil || checkedCarrier.Return.Kind != ExprImmutableLocal || checkedCarrier.Return.ImmutableLocal == nil {
		return fmt.Errorf("function %s v0.57.0 two-stage checked propagation requires the intermediate carrier to be propagated by the next local", function.Name)
	}
	second := checkedCarrier.Return.ImmutableLocal
	if second.Position != checkedCarrier.Position+1 || !TypeEqual(second.Type, payload) || second.Initializer == nil || second.Initializer.Kind != ExprPropagate || second.Initializer.Propagate == nil {
		return fmt.Errorf("function %s v0.57.0 second propagation must initialize the next payload local", function.Name)
	}
	secondPropagation := second.Initializer.Propagate
	if !TypeEqual(secondPropagation.Carrier, carrier) || secondPropagation.Value == nil || !directReference(secondPropagation.Value) || *secondPropagation.Value.Parameter != checkedCarrier.Position {
		return fmt.Errorf("function %s v0.57.0 second propagation must directly consume the explicit checked carrier local", function.Name)
	}
	if second.Return == nil || !isAdmittedMultiParameterCheckedArithmeticContinuation(*second.Return, payload, second.Position, 2) {
		return fmt.Errorf("function %s v0.57.0 terminal checked stage requires local %d as left operand and parameter 2 as right operand", function.Name, second.Position)
	}
	return nil
}

func validatePriorLocalBlockPropagationContract(contract string, function Function) error {
	first := function.Body.ImmutableLocal
	if function.Body.Kind != ExprImmutableLocal || first == nil || first.Initializer == nil || first.Initializer.Kind != ExprCall || first.Initializer.Call == nil || first.Return == nil || first.Return.Kind != ExprImmutableLocal || first.Return.ImmutableLocal == nil || countPropagationExpressions(function.Body) != 1 {
		return fmt.Errorf("function %s prior-local propagation requires one helper-call carrier local followed immediately by one propagation local", function.Name)
	}
	second := first.Return.ImmutableLocal
	if second.Initializer == nil || second.Initializer.Kind != ExprPropagate || second.Initializer.Propagate == nil {
		return fmt.Errorf("function %s prior-local propagation requires propagation as the second immutable-local initializer", function.Name)
	}
	called := first.Initializer.Call
	if (contract == LanguageContractV730 || contract == LanguageContractV720 || contract == LanguageContractV710 || contract == LanguageContractV700 || contract == LanguageContractV690 || contract == LanguageContractV680 || contract == LanguageContractV670 || contract == LanguageContractV660 || contract == LanguageContractV650 || contract == LanguageContractV640 || contract == LanguageContractV630 || contract == LanguageContractV620 || contract == LanguageContractV610 || contract == LanguageContractV600 || contract == LanguageContractV590 || contract == LanguageContractV580) || contract == LanguageContractV570 || contract == LanguageContractV560 || contract == LanguageContractV550 || contract == LanguageContractV540 || contract == LanguageContractV530 {
		if len(function.Parameters) == 0 || !directCallerArgumentReferences(called.Arguments, function.Parameters) {
			return fmt.Errorf("function %s %s multi-parameter helper propagation requires every caller parameter directly once in declaration order", function.Name, contract)
		}
	} else if len(function.Parameters) != 1 || len(called.Arguments) != 1 || !directReference(called.Arguments[0]) || *called.Arguments[0].Parameter != 0 {
		return fmt.Errorf("function %s prior-local propagation requires one helper call over its sole direct parameter", function.Name)
	}
	propagated := second.Initializer.Propagate
	if !directReference(propagated.Value) || *propagated.Value.Parameter != first.Position {
		return fmt.Errorf("function %s prior-local propagation requires a direct reference to the immediately preceding helper-call carrier local", function.Name)
	}
	carrier := function.ReturnType
	if first.Position != len(function.Parameters) || second.Position != first.Position+1 || !TypeEqual(first.Type, carrier) || !TypeEqual(first.Initializer.Type, carrier) || !TypeEqual(propagated.Carrier, carrier) || !TypeEqual(propagated.Value.Type, carrier) {
		return fmt.Errorf("function %s helper-call carrier local and return type must be identical", function.Name)
	}
	var payload Type
	switch {
	case carrier.Kind == TypeOptional && carrier.Optional != nil && isOptionalValueType(carrier.Optional.Value):
		payload = carrier.Optional.Value
	case isBoundedValueResultType(carrier):
		payload = carrier.Result.Success
	case ((contract == LanguageContractV730 || contract == LanguageContractV720 || contract == LanguageContractV710 || contract == LanguageContractV700 || contract == LanguageContractV690 || contract == LanguageContractV680 || contract == LanguageContractV670 || contract == LanguageContractV660 || contract == LanguageContractV650 || contract == LanguageContractV640 || contract == LanguageContractV630 || contract == LanguageContractV620 || contract == LanguageContractV610 || contract == LanguageContractV600 || contract == LanguageContractV590 || contract == LanguageContractV580) || contract == LanguageContractV570 || contract == LanguageContractV560 || contract == LanguageContractV550 || contract == LanguageContractV540) && isArithmeticResultType(carrier):
		payload = carrier.Result.Success
	default:
		if (contract == LanguageContractV730 || contract == LanguageContractV720 || contract == LanguageContractV710 || contract == LanguageContractV700 || contract == LanguageContractV690 || contract == LanguageContractV680 || contract == LanguageContractV670 || contract == LanguageContractV660 || contract == LanguageContractV650 || contract == LanguageContractV640 || contract == LanguageContractV630 || contract == LanguageContractV620 || contract == LanguageContractV610 || contract == LanguageContractV600 || contract == LanguageContractV590 || contract == LanguageContractV580) || contract == LanguageContractV570 || contract == LanguageContractV560 || contract == LanguageContractV550 || contract == LanguageContractV540 {
			return fmt.Errorf("function %s prior-local propagation requires an admitted Optional, bounded Result, or checked-arithmetic Result", function.Name)
		}
		return fmt.Errorf("function %s prior-local propagation requires an admitted Optional or bounded Result", function.Name)
	}
	if !TypeEqual(second.Type, payload) || !TypeEqual(second.Initializer.Type, payload) {
		return fmt.Errorf("function %s second immutable local type must match the propagated success payload", function.Name)
	}
	if ((contract == LanguageContractV730 || contract == LanguageContractV720 || contract == LanguageContractV710 || contract == LanguageContractV700 || contract == LanguageContractV690 || contract == LanguageContractV680 || contract == LanguageContractV670 || contract == LanguageContractV660 || contract == LanguageContractV650 || contract == LanguageContractV640 || contract == LanguageContractV630 || contract == LanguageContractV620 || contract == LanguageContractV610 || contract == LanguageContractV600 || contract == LanguageContractV590 || contract == LanguageContractV580) || contract == LanguageContractV570 || contract == LanguageContractV560 || contract == LanguageContractV550 || contract == LanguageContractV540) && isArithmeticResultType(carrier) && !isAdmittedCheckedArithmeticContinuation(*second.Return, payload) {
		return fmt.Errorf("function %s checked-arithmetic helper propagation requires one admitted checked arithmetic continuation", function.Name)
	}
	return nil
}

func isAdmittedCheckedArithmeticContinuation(expression Expr, payload Type) bool {
	if !isArithmeticResultType(expression.Type) || expression.Type.Result == nil || !TypeEqual(expression.Type.Result.Success, payload) {
		return false
	}
	if TypeEqual(payload, BinaryFloat(64)) {
		return expression.Kind == ExprBinary && expression.Binary != nil && expression.Binary.Operator == OperatorDivide
	}
	if !TypeEqual(payload, SignedInteger(64)) {
		return false
	}
	if expression.Kind == ExprUnary && expression.Unary != nil {
		return expression.Unary.Operator == OperatorNegate
	}
	return expression.Kind == ExprBinary && expression.Binary != nil && (expression.Binary.Operator == OperatorAdd || expression.Binary.Operator == OperatorSubtract || expression.Binary.Operator == OperatorMultiply)
}

func directCallerArgumentReferences(arguments []*Expr, parameters []Parameter) bool {
	if len(arguments) != len(parameters) {
		return false
	}
	for position := range parameters {
		if !directReference(arguments[position]) || *arguments[position].Parameter != position {
			return false
		}
	}
	return true
}

func exprContainsPropagation(expression Expr) bool {
	if expression.Kind == ExprPropagate {
		return true
	}
	for _, child := range expressionChildren(expression) {
		if child != nil && exprContainsPropagation(*child) {
			return true
		}
	}
	return false
}

func validateConditionalContract(contract string, function Function) error {
	if contract == LanguageContractV1050 && validDepthThreeTerminalConditionalTests(function.Body) {
		return nil
	}
	if (contract == LanguageContractV1040 || contract == LanguageContractV1050) && validNestedTerminalConditionalTests(function.Body) {
		return nil
	}
	if (contract == LanguageContractV1030 || (contract == LanguageContractV1040 || contract == LanguageContractV1050)) && validTerminalConditionalTests(function.Body) {
		return nil
	}
	if (contract == LanguageContractV1020 || (contract == LanguageContractV1030 || (contract == LanguageContractV1040 || contract == LanguageContractV1050))) && validTerminalBooleanSelectorInitializers(function.Body) {
		return nil
	}
	if (contract == LanguageContractV1010 || (contract == LanguageContractV1020 || (contract == LanguageContractV1030 || (contract == LanguageContractV1040 || contract == LanguageContractV1050)))) && validStraightLineBooleanSelectorInitializers(function.Body) {
		return nil
	}
	if (contract == LanguageContractV980 || (contract == LanguageContractV990 || (contract == LanguageContractV1000 || (contract == LanguageContractV1010 || (contract == LanguageContractV1020 || (contract == LanguageContractV1030 || (contract == LanguageContractV1040 || contract == LanguageContractV1050))))))) && validConditionalBooleanSelector(function.Body) || (contract == LanguageContractV990 || (contract == LanguageContractV1000 || (contract == LanguageContractV1010 || (contract == LanguageContractV1020 || (contract == LanguageContractV1030 || (contract == LanguageContractV1040 || contract == LanguageContractV1050)))))) && validTerminalLeafBooleanSelectors(function.Body) {
		return nil
	}
	if (contract == LanguageContractV940 || (contract == LanguageContractV950 || (contract == LanguageContractV960 || (contract == LanguageContractV970 || (contract == LanguageContractV980 || (contract == LanguageContractV990 || (contract == LanguageContractV1000 || (contract == LanguageContractV1010 || (contract == LanguageContractV1020 || (contract == LanguageContractV1030 || (contract == LanguageContractV1040 || contract == LanguageContractV1050))))))))))) && validDepthThreeTerminalLeafReturns(function.Body) {
		return nil
	}
	if (contract == LanguageContractV970 || (contract == LanguageContractV980 || (contract == LanguageContractV990 || (contract == LanguageContractV1000 || (contract == LanguageContractV1010 || (contract == LanguageContractV1020 || (contract == LanguageContractV1030 || (contract == LanguageContractV1040 || contract == LanguageContractV1050)))))))) && validDepthThreeTerminalInitializers(function.Body) {
		return nil
	}
	if (contract == LanguageContractV960 || (contract == LanguageContractV970 || (contract == LanguageContractV980 || (contract == LanguageContractV990 || (contract == LanguageContractV1000 || (contract == LanguageContractV1010 || (contract == LanguageContractV1020 || (contract == LanguageContractV1030 || (contract == LanguageContractV1040 || contract == LanguageContractV1050))))))))) && validDepthThreeStraightLineInitializers(function.Body) {
		return nil
	}
	if (contract == LanguageContractV930 || (contract == LanguageContractV940 || (contract == LanguageContractV950 || (contract == LanguageContractV960 || (contract == LanguageContractV970 || (contract == LanguageContractV980 || (contract == LanguageContractV990 || (contract == LanguageContractV1000 || (contract == LanguageContractV1010 || (contract == LanguageContractV1020 || (contract == LanguageContractV1030 || (contract == LanguageContractV1040 || contract == LanguageContractV1050)))))))))))) && validDepthThreeStraightLineReturns(function.Body) {
		return nil
	}
	if (contract == LanguageContractV910 || (contract == LanguageContractV920 || (contract == LanguageContractV930 || (contract == LanguageContractV940 || (contract == LanguageContractV950 || (contract == LanguageContractV960 || (contract == LanguageContractV970 || (contract == LanguageContractV980 || (contract == LanguageContractV990 || (contract == LanguageContractV1000 || (contract == LanguageContractV1010 || (contract == LanguageContractV1020 || (contract == LanguageContractV1030 || (contract == LanguageContractV1040 || contract == LanguageContractV1050)))))))))))))) && validNestedTerminalInitializers(function.Body) {
		return nil
	}
	if (contract == LanguageContractV900 || (contract == LanguageContractV910 || (contract == LanguageContractV920 || (contract == LanguageContractV930 || (contract == LanguageContractV940 || (contract == LanguageContractV950 || (contract == LanguageContractV960 || (contract == LanguageContractV970 || (contract == LanguageContractV980 || (contract == LanguageContractV990 || (contract == LanguageContractV1000 || (contract == LanguageContractV1010 || (contract == LanguageContractV1020 || (contract == LanguageContractV1030 || (contract == LanguageContractV1040 || contract == LanguageContractV1050))))))))))))))) && validNestedStraightLineInitializers(function.Body) {
		return nil
	}
	if (contract == LanguageContractV890 || (contract == LanguageContractV900 || (contract == LanguageContractV910 || (contract == LanguageContractV920 || (contract == LanguageContractV930 || (contract == LanguageContractV940 || (contract == LanguageContractV950 || (contract == LanguageContractV960 || (contract == LanguageContractV970 || (contract == LanguageContractV980 || (contract == LanguageContractV990 || (contract == LanguageContractV1000 || (contract == LanguageContractV1010 || (contract == LanguageContractV1020 || (contract == LanguageContractV1030 || (contract == LanguageContractV1040 || contract == LanguageContractV1050)))))))))))))))) && validNestedTerminalLeafReturns(function.Body) {
		return nil
	}
	if ((contract == LanguageContractV890 || (contract == LanguageContractV900 || (contract == LanguageContractV910 || (contract == LanguageContractV920 || (contract == LanguageContractV930 || (contract == LanguageContractV940 || (contract == LanguageContractV950 || (contract == LanguageContractV960 || (contract == LanguageContractV970 || (contract == LanguageContractV980 || (contract == LanguageContractV990 || (contract == LanguageContractV1000 || (contract == LanguageContractV1010 || (contract == LanguageContractV1020 || (contract == LanguageContractV1030 || (contract == LanguageContractV1040 || contract == LanguageContractV1050)))))))))))))))) || contract == LanguageContractV880) && validNestedStraightLineReturns(function.Body) {
		return nil
	}
	if ((contract == LanguageContractV890 || (contract == LanguageContractV900 || (contract == LanguageContractV910 || (contract == LanguageContractV920 || (contract == LanguageContractV930 || (contract == LanguageContractV940 || (contract == LanguageContractV950 || (contract == LanguageContractV960 || (contract == LanguageContractV970 || (contract == LanguageContractV980 || (contract == LanguageContractV990 || (contract == LanguageContractV1000 || (contract == LanguageContractV1010 || (contract == LanguageContractV1020 || (contract == LanguageContractV1030 || (contract == LanguageContractV1040 || contract == LanguageContractV1050)))))))))))))))) || contract == LanguageContractV880) || contract == LanguageContractV870 {
		if validTerminalLeafConditionalReturns(function.Body) {
			return nil
		}
		return validateConditionalContract(LanguageContractV860, function)
	}
	if contract == LanguageContractV860 {
		if validConditionalReturnComposition(function.Body) {
			return nil
		}
		return validateConditionalContract(LanguageContractV850, function)
	}
	if contract == LanguageContractV850 {
		if validStraightLineConditionalLocals(function.Body) {
			return nil
		}
		return validateConditionalContract(LanguageContractV840, function)
	}
	if contract == LanguageContractV840 {
		if validConditionalLocalTree(function.Body, 0) {
			return nil
		}
		return validateConditionalContract(LanguageContractV830, function)
	}
	if contract == LanguageContractV830 {
		if validConditionalLocalTree(function.Body, 2) {
			return nil
		}
		return validateConditionalContract(LanguageContractV820, function)
	}
	if contract == LanguageContractV820 {
		if validV820ConditionalLocalTree(function.Body) {
			return nil
		}
		return validateConditionalContract(LanguageContractV810, function)
	}
	if contract == LanguageContractV810 {
		if countTerminalIfStatements(function.Body) > 0 && validV810TerminalTree(function.Body, 3) {
			return nil
		}
		return validateConditionalContract(LanguageContractV800, function)
	}
	if contract == LanguageContractV800 {
		if validV800TwoExpandedTerminalIf(function.Body) {
			return nil
		}
		if countTerminalIfStatements(function.Body) >= 5 {
			return fmt.Errorf("function %s v0.80.0 permits exactly two expanded leaves on a symmetric depth-two terminal base; additional expansion or depth, asymmetric bases, conditional expressions, propagation, match, and fallthrough are excluded", function.Name)
		}
		return validateConditionalContract(LanguageContractV790, function)
	}
	if contract == LanguageContractV790 {
		if validV790BoundedDepthThreeTerminalIf(function.Body) {
			return nil
		}
		if countTerminalIfStatements(function.Body) >= 4 {
			return fmt.Errorf("function %s v0.79.0 permits an inherited rootful or rootless symmetric depth-two terminal if/else with exactly one expanded leaf; additional expansion or depth, conditional expressions within the topology, propagation, match, and fallthrough are excluded", function.Name)
		}
		return validateConditionalContract(LanguageContractV780, function)
	}
	if contract == LanguageContractV780 {
		if validSymmetricNestedTerminalIf(function.Body, false) {
			return nil
		}
		return validateConditionalContract(LanguageContractV770, function)
	}
	if !exprContainsConditional(function.Body) {
		return nil
	}
	if contract == LanguageContractV770 {
		if countTerminalIfStatements(function.Body) > 1 {
			if (!validV750NestedTerminalIf(function.Body) && !validV760RootLocalNestedTerminalIf(function.Body) && !validV770SymmetricRootLocalNestedTerminalIf(function.Body)) || exprContainsPropagation(function.Body) || countMatchExpressions(function.Body) != 0 {
				return fmt.Errorf("function %s v0.77.0 permits one or more top-level typed immutable locals before an outer terminal if/else whose two branches each end in one inner terminal if/else; every inner leaf may contain finite local sequences, while additional depth, conditional expressions within the topology, propagation, match, and fallthrough are excluded", function.Name)
			}
			return nil
		}
		contract = LanguageContractV730
	}
	if contract == LanguageContractV760 {
		if countTerminalIfStatements(function.Body) > 1 {
			if (!validV750NestedTerminalIf(function.Body) && !validV760RootLocalNestedTerminalIf(function.Body)) || exprContainsPropagation(function.Body) || countMatchExpressions(function.Body) != 0 {
				return fmt.Errorf("function %s v0.76.0 permits one or more top-level typed immutable locals before the exact v0.75.0 one-branch nested terminal if/else; both nested leaves may contain finite local sequences, while nesting in both outer branches, additional depth, propagation, match, conditional expressions within the topology, and fallthrough are excluded", function.Name)
			}
			return nil
		}
		contract = LanguageContractV730
	}
	if contract == LanguageContractV750 {
		if countTerminalIfStatements(function.Body) > 1 {
			if !validV750NestedTerminalIf(function.Body) || exprContainsPropagation(function.Body) || countMatchExpressions(function.Body) != 0 {
				return fmt.Errorf("function %s v0.75.0 permits one complete outer terminal if/else with exactly one branch ending in one nested terminal if/else; inner leaves may contain finite typed immutable-local sequences, while propagation, match, additional nesting, and fallthrough are excluded", function.Name)
			}
			return nil
		}
		contract = LanguageContractV730
	}
	if contract == LanguageContractV740 {
		if countTerminalIfStatements(function.Body) > 1 {
			if !validV740NestedTerminalIf(function.Body) || exprContainsPropagation(function.Body) || countMatchExpressions(function.Body) != 0 {
				return fmt.Errorf("function %s v0.74.0 permits one complete outer terminal if/else with exactly one branch ending in one nested terminal if/else after any finite immutable-local sequence; nested leaves return directly, and propagation, match, additional nesting, and fallthrough are excluded", function.Name)
			}
			return nil
		}
		contract = LanguageContractV730
	}
	if (contract != LanguageContractV730 && contract != LanguageContractV720 && contract != LanguageContractV710 && contract != LanguageContractV700 && contract != LanguageContractV690 && contract != LanguageContractV680 && contract != LanguageContractV670 && contract != LanguageContractV660 && contract != LanguageContractV650 && contract != LanguageContractV640 && contract != LanguageContractV630 && contract != LanguageContractV620 && contract != LanguageContractV610 && contract != LanguageContractV600 && contract != LanguageContractV590 && contract != LanguageContractV580) && contract != LanguageContractV570 && contract != LanguageContractV560 && contract != LanguageContractV550 && contract != LanguageContractV540 && contract != LanguageContractV530 && contract != LanguageContractV520 && contract != LanguageContractV510 && contract != LanguageContractV500 && contract != LanguageContractV490 && contract != LanguageContractV380 && contract != LanguageContractV390 && contract != LanguageContractV400 && contract != LanguageContractV410 && contract != LanguageContractV420 && contract != LanguageContractV430 && contract != LanguageContractV440 && contract != LanguageContractV450 && contract != LanguageContractV460 && contract != LanguageContractV470 && contract != LanguageContractV480 {
		return fmt.Errorf("function %s conditional expressions require language contract %q", function.Name, LanguageContractV380)
	}
	conditionalCount := countConditionalExpressions(function.Body)
	if conditionalCount != 1 && contract != LanguageContractV730 && contract != LanguageContractV720 && contract != LanguageContractV710 && contract != LanguageContractV700 && contract != LanguageContractV690 {
		return fmt.Errorf("function %s admits exactly one conditional expression", function.Name)
	}
	var terminalConditional *Conditional
	terminalCount := 0
	valid := true
	var walk func(Expr)
	walk = func(expression Expr) {
		if expression.Kind == ExprConditional {
			conditional := expression.Conditional
			if conditional == nil || conditional.Condition == nil || conditional.WhenTrue == nil || conditional.WhenFalse == nil || !validConditionalOperand(*conditional.Condition) || !validConditionalOperand(*conditional.WhenTrue) || !validConditionalOperand(*conditional.WhenFalse) {
				valid = false
				return
			}
			if conditional.TerminalStatement {
				terminalCount++
				terminalConditional = conditional
			}
		}
		for _, child := range expressionChildren(expression) {
			if child != nil {
				walk(*child)
			}
		}
	}
	walk(function.Body)
	if !valid {
		return fmt.Errorf("function %s conditional operands must be existing eager pure expressions without nested conditionals, match, or propagate", function.Name)
	}
	if terminalCount != 0 {
		terminal := function.Body
		locals := 0
		for terminal.Kind == ExprImmutableLocal && terminal.ImmutableLocal != nil && terminal.ImmutableLocal.Return != nil {
			locals++
			terminal = *terminal.ImmutableLocal.Return
		}
		branchLocalLimit := 0
		if contract == LanguageContractV700 {
			branchLocalLimit = 1
		}
		if contract == LanguageContractV710 {
			branchLocalLimit = 2
		}
		if contract == LanguageContractV730 || contract == LanguageContractV720 {
			branchLocalLimit = -1
		}
		validBranches := terminal.Conditional != nil && terminal.Conditional.WhenTrue != nil && terminal.Conditional.WhenFalse != nil
		if validBranches {
			_, validTrue := terminalBranchLocalShape(*terminal.Conditional.WhenTrue, branchLocalLimit)
			_, validFalse := terminalBranchLocalShape(*terminal.Conditional.WhenFalse, branchLocalLimit)
			validBranches = validTrue && validFalse
		}
		minimumTopLevelLocals := 1
		if contract == LanguageContractV730 {
			minimumTopLevelLocals = 0
		}
		if (contract != LanguageContractV730 && contract != LanguageContractV720 && contract != LanguageContractV710 && contract != LanguageContractV700 && contract != LanguageContractV690) || terminalCount != 1 || conditionalCount > 2 || locals < minimumTopLevelLocals || terminal.Kind != ExprConditional || terminal.Conditional != terminalConditional || !validBranches || exprContainsPropagation(function.Body) || countMatchExpressions(function.Body) != 0 {
			if contract == LanguageContractV730 {
				return fmt.Errorf("function %s v0.73.0 terminal if/else may be the complete method body and permits any finite sequence of explicitly typed ordered immutable locals per branch; branch locals are lexical, and propagation, match, nested branches, and fallthrough are excluded", function.Name)
			}
			if contract == LanguageContractV720 {
				return fmt.Errorf("function %s v0.72.0 terminal if/else permits any finite sequence of explicitly typed ordered immutable locals per branch; branch locals are lexical, and propagation, match, nested branches, and fallthrough are excluded", function.Name)
			}
			if contract == LanguageContractV710 {
				return fmt.Errorf("function %s v0.71.0 terminal if/else permits at most two explicitly typed ordered immutable locals per branch; branch locals are lexical, and propagation, match, nested branches, three branch locals, and fallthrough are excluded", function.Name)
			}
			if contract == LanguageContractV700 {
				return fmt.Errorf("function %s v0.70.0 terminal if/else permits at most one explicitly typed immutable local per branch; branch locals are lexical, and propagation, match, nested branches, multiple branch locals, and fallthrough are excluded", function.Name)
			}
			return fmt.Errorf("function %s v0.69.0 terminal if/else requires one or more ordered immutable locals followed by one terminal bool branch over existing eager pure expressions; propagation, match, nested branches, branch locals, and fallthrough are excluded", function.Name)
		}
	} else if conditionalCount != 1 {
		return fmt.Errorf("function %s admits exactly one conditional expression", function.Name)
	}
	return nil
}

func validCallPlacement(expression Expr) bool {
	if !exprContainsCall(expression) {
		return true
	}
	if expression.Kind != ExprCall || expression.Call == nil {
		return false
	}
	for _, argument := range expression.Call.Arguments {
		if argument == nil || !validCallPlacement(*argument) {
			return false
		}
	}
	return true
}

func validGeneralCallPlacement(expression Expr) bool {
	switch expression.Kind {
	case ExprPropagate:
		return expression.Propagate != nil && directReference(expression.Propagate.Value)
	case ExprMatch:
		if expression.Match == nil || !directReference(expression.Match.Value) {
			return false
		}
		for _, arm := range expression.Match.Arms {
			if arm.Body == nil || !validGeneralCallPlacement(*arm.Body) {
				return false
			}
		}
		return true
	default:
		for _, child := range expressionChildren(expression) {
			if child == nil || !validGeneralCallPlacement(*child) {
				return false
			}
		}
		return true
	}
}

func validHelperCarrierMatchCallPlacement(expression Expr) bool {
	if expression.Kind != ExprMatch || expression.Match == nil || expression.Match.Value == nil || expression.Match.Value.Kind != ExprCall || expression.Match.Value.Call == nil {
		return validGeneralCallPlacement(expression)
	}
	call := expression.Match.Value.Call
	if len(call.Arguments) == 0 {
		return false
	}
	for _, argument := range call.Arguments {
		if !directReference(argument) {
			return false
		}
	}
	for _, arm := range expression.Match.Arms {
		if arm.Body == nil || !validGeneralCallPlacement(*arm.Body) {
			return false
		}
	}
	return true
}

func validHelperCarrierMatchLocalCallPlacement(expression Expr) bool {
	if expression.Kind != ExprImmutableLocal || expression.ImmutableLocal == nil || expression.ImmutableLocal.Initializer == nil || expression.ImmutableLocal.Return == nil {
		return validHelperCarrierMatchCallPlacement(expression)
	}
	local := expression.ImmutableLocal
	if local.Initializer.Kind == ExprMatch && local.Initializer.Match != nil && local.Initializer.Match.Value != nil && local.Initializer.Match.Value.Kind == ExprCall {
		return validHelperCarrierMatchCallPlacement(*local.Initializer) && validGeneralCallPlacement(*local.Return)
	}
	return validHelperCarrierMatchCallPlacement(expression)
}

func validHelperCarrierMatchLaterLocalCallPlacement(expression Expr) bool {
	if expression.Kind != ExprImmutableLocal || expression.ImmutableLocal == nil || expression.ImmutableLocal.Initializer == nil || expression.ImmutableLocal.Return == nil {
		return validHelperCarrierMatchCallPlacement(expression)
	}
	return validHelperCarrierMatchLocalSequence(*expression.ImmutableLocal)
}

func validHelperCarrierMatchLocalSequence(local ImmutableLocal) bool {
	initializer := local.Initializer
	if initializer.Kind == ExprMatch && initializer.Match != nil && initializer.Match.Value != nil && initializer.Match.Value.Kind == ExprCall {
		if !validHelperCarrierMatchCallPlacement(*initializer) {
			return false
		}
	} else if !validGeneralCallPlacement(*initializer) {
		return false
	}
	if local.Return.Kind == ExprImmutableLocal && local.Return.ImmutableLocal != nil && local.Return.ImmutableLocal.Initializer != nil && local.Return.ImmutableLocal.Return != nil {
		return validHelperCarrierMatchLocalSequence(*local.Return.ImmutableLocal)
	}
	return validGeneralCallPlacement(*local.Return)
}

func directReference(expression *Expr) bool {
	return expression != nil && expression.Kind == ExprReference && expression.Parameter != nil
}

func validatePureCallPlacement(contract string, function Function) error {
	if !exprContainsCall(function.Body) {
		return nil
	}
	switch contract {
	case LanguageContractV360:
		if !validCallPlacement(function.Body) {
			return fmt.Errorf("function %s pure calls must be the complete body or directly nested call arguments under %s", function.Name, LanguageContractV360)
		}
	case LanguageContractV370, LanguageContractV380, LanguageContractV390, LanguageContractV400, LanguageContractV410, LanguageContractV420:
		if !validGeneralCallPlacement(function.Body) {
			return fmt.Errorf("function %s composed pure calls retain direct match and propagate carriers", function.Name)
		}
	case LanguageContractV430, LanguageContractV440, LanguageContractV450, LanguageContractV460, LanguageContractV470, LanguageContractV480, LanguageContractV490, LanguageContractV500, LanguageContractV510, LanguageContractV520, LanguageContractV530, LanguageContractV540, LanguageContractV550, LanguageContractV560, LanguageContractV570, LanguageContractV580, LanguageContractV590, LanguageContractV600, LanguageContractV610, LanguageContractV620, LanguageContractV630, LanguageContractV640, LanguageContractV650, LanguageContractV660, LanguageContractV670, LanguageContractV680, LanguageContractV690, LanguageContractV700, LanguageContractV710, LanguageContractV720, LanguageContractV730:
		valid := validHelperCarrierMatchCallPlacement(function.Body)
		if contract == LanguageContractV450 || contract == LanguageContractV460 {
			valid = validHelperCarrierMatchLocalCallPlacement(function.Body)
		} else if contract == LanguageContractV470 || contract == LanguageContractV480 || contract == LanguageContractV490 || contract == LanguageContractV500 || contract == LanguageContractV510 || (contract == LanguageContractV730 || contract == LanguageContractV720 || contract == LanguageContractV710 || contract == LanguageContractV700 || contract == LanguageContractV690 || contract == LanguageContractV680 || contract == LanguageContractV670 || contract == LanguageContractV660 || contract == LanguageContractV650 || contract == LanguageContractV640 || contract == LanguageContractV630 || contract == LanguageContractV620 || contract == LanguageContractV610 || contract == LanguageContractV600 || contract == LanguageContractV590 || contract == LanguageContractV580) || contract == LanguageContractV570 || contract == LanguageContractV560 || contract == LanguageContractV550 || contract == LanguageContractV540 || contract == LanguageContractV530 || contract == LanguageContractV520 {
			valid = validHelperCarrierMatchLaterLocalCallPlacement(function.Body)
		}
		if !valid {
			if contract == LanguageContractV430 {
				return fmt.Errorf("function %s v0.43.0 admits helper calls only as one complete Result<string, string> match carrier over the direct caller parameter", function.Name)
			}
			if contract == LanguageContractV450 || contract == LanguageContractV460 {
				return fmt.Errorf("function %s %s admits one helper-carrier match only as the first immutable-local initializer or inherited complete body", function.Name, contract)
			}
			if contract == LanguageContractV470 {
				return fmt.Errorf("function %s v0.47.0 admits one helper-carrier match only as an immutable-local initializer or inherited complete body", function.Name)
			}
			if contract == LanguageContractV480 {
				return fmt.Errorf("function %s v0.48.0 admits one helper-carrier match only as an inherited placement or an adjacent helper-call carrier local and match local", function.Name)
			}
			if contract == LanguageContractV490 {
				return fmt.Errorf("function %s v0.49.0 admits either one inherited bounded match or exactly two adjacent helper-call carrier and match-local pairs", function.Name)
			}
			if contract == LanguageContractV500 {
				return fmt.Errorf("function %s v0.50.0 admits inherited bounded matches or one contiguous dependent second carrier stage", function.Name)
			}
			if contract == LanguageContractV510 {
				return fmt.Errorf("function %s v0.51.0 admits inherited bounded matches or one contiguous dependent carrier chain", function.Name)
			}
			if (contract == LanguageContractV730 || contract == LanguageContractV720 || contract == LanguageContractV710 || contract == LanguageContractV700 || contract == LanguageContractV690 || contract == LanguageContractV680 || contract == LanguageContractV670 || contract == LanguageContractV660 || contract == LanguageContractV650 || contract == LanguageContractV640 || contract == LanguageContractV630 || contract == LanguageContractV620 || contract == LanguageContractV610 || contract == LanguageContractV600 || contract == LanguageContractV590 || contract == LanguageContractV580) || contract == LanguageContractV570 || contract == LanguageContractV560 || contract == LanguageContractV550 || contract == LanguageContractV540 || contract == LanguageContractV530 || contract == LanguageContractV520 {
				return fmt.Errorf("function %s v0.52.0 admits inherited bounded matches, one v0.51 immediate-only chain, or one cumulative fan-in chain", function.Name)
			}
			return fmt.Errorf("function %s %s admits helper calls only as one complete match carrier over direct caller parameters", function.Name, contract)
		}
	default:
		return fmt.Errorf("function %s pure calls require language contract %q or later", function.Name, LanguageContractV360)
	}
	return nil
}

func countMatchExpressions(expression Expr) int {
	count := 0
	if expression.Kind == ExprMatch {
		count++
	}
	for _, child := range expressionChildren(expression) {
		if child != nil {
			count += countMatchExpressions(*child)
		}
	}
	return count
}

func validateHelperResultMatchContract(contract string, function Function, functions map[string]Function) error {
	if (contract == LanguageContractV730 || contract == LanguageContractV720 || contract == LanguageContractV710 || contract == LanguageContractV700 || contract == LanguageContractV690 || contract == LanguageContractV680 || contract == LanguageContractV670 || contract == LanguageContractV660 || contract == LanguageContractV650 || contract == LanguageContractV640 || contract == LanguageContractV630 || contract == LanguageContractV620 || contract == LanguageContractV610 || contract == LanguageContractV600 || contract == LanguageContractV590 || contract == LanguageContractV580) || contract == LanguageContractV570 || contract == LanguageContractV560 || contract == LanguageContractV550 || contract == LanguageContractV540 || contract == LanguageContractV530 || contract == LanguageContractV520 {
		matchCount := countMatchExpressions(function.Body)
		if matchCount == 0 {
			return nil
		}
		if matchCount <= 2 {
			return validateHelperResultMatchContract(LanguageContractV500, function, functions)
		}
		if err := validateHelperResultMatchContract(LanguageContractV510, function, functions); err == nil {
			return nil
		}
		pairs := priorLocalCarrierMatches(function.Body)
		if len(pairs) != matchCount {
			return fmt.Errorf("function %s v0.52.0 cumulative fan-in chain requires every match in an adjacent helper-call carrier and match-local pair", function.Name)
		}
		for position := 1; position < len(pairs); position++ {
			if pairs[position-1].matched.Return == nil || pairs[position-1].matched.Return.Kind != ExprImmutableLocal || pairs[position-1].matched.Return.ImmutableLocal != pairs[position].carrier {
				return fmt.Errorf("function %s v0.52.0 cumulative fan-in chain requires one contiguous carrier and match-local sequence", function.Name)
			}
		}
		first := *pairs[0].match
		first.Value = pairs[0].carrier.Initializer
		if err := validateGeneralHelperCarrierMatchCount(function, &first, functions, "v0.52.0 first helper-carrier match pair", true, matchCount); err != nil {
			return err
		}
		for position := 1; position < len(pairs); position++ {
			matched := *pairs[position].match
			matched.Value = pairs[position].carrier.Initializer
			selected := make([]*ImmutableLocal, position)
			for prior := 0; prior < position; prior++ {
				selected[prior] = pairs[prior].matched
			}
			if err := validateSelectedHelperCarrierMatch(function, selected, &matched, functions, fmt.Sprintf("v0.52.0 cumulative fan-in chain stage %d", position+1), "every prior selected local in chain order", matchCount); err != nil {
				return err
			}
		}
		return nil
	}
	if contract == LanguageContractV510 {
		matchCount := countMatchExpressions(function.Body)
		if matchCount == 0 {
			return nil
		}
		if matchCount <= 2 {
			return validateHelperResultMatchContract(LanguageContractV500, function, functions)
		}
		pairs := priorLocalCarrierMatches(function.Body)
		if len(pairs) != matchCount {
			return fmt.Errorf("function %s v0.51.0 dependent carrier chain requires every match in an adjacent helper-call carrier and match-local pair", function.Name)
		}
		for position := 1; position < len(pairs); position++ {
			if pairs[position-1].matched.Return == nil || pairs[position-1].matched.Return.Kind != ExprImmutableLocal || pairs[position-1].matched.Return.ImmutableLocal != pairs[position].carrier {
				return fmt.Errorf("function %s v0.51.0 dependent carrier chain requires one contiguous carrier and match-local sequence", function.Name)
			}
		}
		first := *pairs[0].match
		first.Value = pairs[0].carrier.Initializer
		if err := validateGeneralHelperCarrierMatchCount(function, &first, functions, "v0.51.0 first helper-carrier match pair", true, matchCount); err != nil {
			return err
		}
		for position := 1; position < len(pairs); position++ {
			matched := *pairs[position].match
			matched.Value = pairs[position].carrier.Initializer
			if err := validateDependentHelperCarrierMatch(function, pairs[position-1].matched, &matched, functions, fmt.Sprintf("v0.51.0 dependent carrier chain stage %d", position+1), "immediately preceding selected local", matchCount); err != nil {
				return err
			}
		}
		return nil
	}
	if contract == LanguageContractV500 {
		matchCount := countMatchExpressions(function.Body)
		if matchCount == 0 {
			return nil
		}
		if matchCount == 1 {
			return validateHelperResultMatchContract(LanguageContractV480, function, functions)
		}
		if matchCount != 2 {
			return fmt.Errorf("function %s v0.50.0 admits inherited bounded matches or exactly two matches in one contiguous dependent carrier stage", function.Name)
		}
		pairs := priorLocalCarrierMatches(function.Body)
		if len(pairs) != 2 {
			return fmt.Errorf("function %s v0.50.0 dependent composition requires exactly two adjacent carrier and match-local pairs", function.Name)
		}
		if err := validateHelperResultMatchContract(LanguageContractV490, function, functions); err == nil {
			return nil
		}
		if pairs[0].matched.Return == nil || pairs[0].matched.Return.Kind != ExprImmutableLocal || pairs[0].matched.Return.ImmutableLocal != pairs[1].carrier {
			return fmt.Errorf("function %s v0.50.0 dependent composition requires four contiguous carrier, match, carrier, and match locals", function.Name)
		}
		first := *pairs[0].match
		first.Value = pairs[0].carrier.Initializer
		if err := validateGeneralHelperCarrierMatchCount(function, &first, functions, "v0.50.0 first helper-carrier match pair", true, 2); err != nil {
			return err
		}
		second := *pairs[1].match
		second.Value = pairs[1].carrier.Initializer
		return validateDependentSecondHelperCarrierMatch(function, pairs[0].matched, &second, functions)
	}
	if contract == LanguageContractV490 {
		matchCount := countMatchExpressions(function.Body)
		if matchCount == 0 {
			return nil
		}
		if matchCount == 1 {
			return validateHelperResultMatchContract(LanguageContractV480, function, functions)
		}
		if matchCount != 2 {
			return fmt.Errorf("function %s v0.49.0 admits either one inherited bounded match or exactly two matches in adjacent helper-call carrier and match-local pairs", function.Name)
		}
		pairs := priorLocalCarrierMatches(function.Body)
		if len(pairs) != 2 {
			return fmt.Errorf("function %s v0.49.0 two-match composition requires exactly two non-overlapping adjacent helper-call carrier and match-local pairs", function.Name)
		}
		for position, pair := range pairs {
			synthetic := *pair.match
			synthetic.Value = pair.carrier.Initializer
			if err := validateGeneralHelperCarrierMatchCount(function, &synthetic, functions, fmt.Sprintf("v0.49.0 helper-carrier match pair %d", position+1), true, 2); err != nil {
				return err
			}
		}
		return nil
	}
	if contract == LanguageContractV480 {
		if carrier, match, ok := priorLocalCarrierMatch(function.Body); ok {
			if countMatchExpressions(function.Body) != 1 {
				return fmt.Errorf("function %s v0.48.0 prior-local carrier matching requires exactly one match in the immutable-local sequence", function.Name)
			}
			synthetic := *match
			synthetic.Value = carrier.Initializer
			return validateGeneralHelperCarrierMatch(function, &synthetic, functions, "v0.48.0 prior-local carrier match", true)
		}
		if match, ok := helperCarrierMatchLocal(function.Body); ok {
			if countMatchExpressions(function.Body) != 1 {
				return fmt.Errorf("function %s v0.48.0 inherited helper-carrier match requires exactly one match in the immutable-local sequence", function.Name)
			}
			return validateGeneralHelperCarrierMatch(function, match, functions, "v0.47.0 later-local helper-carrier match", true)
		}
		if function.Body.Kind == ExprMatch && function.Body.Match != nil {
			if function.Body.Match.Value != nil && function.Body.Match.Value.Kind == ExprCall {
				return validateGeneralHelperCarrierMatch(function, function.Body.Match, functions, "v0.44.0 helper-carrier match", false)
			}
			if countMatchExpressions(function.Body) == 1 && directFunctionParameter(function.Body.Match.Value, len(function.Parameters)) {
				return nil
			}
		}
		if countMatchExpressions(function.Body) != 0 {
			return fmt.Errorf("function %s v0.48.0 match must be an inherited complete-body/helper-local form or an adjacent helper-call carrier local and match local", function.Name)
		}
		return nil
	}
	if _, _, ok := priorLocalCarrierMatch(function.Body); ok {
		return fmt.Errorf("function %s prior-local carrier matching requires language contract %q", function.Name, LanguageContractV480)
	}
	if contract == LanguageContractV450 || contract == LanguageContractV460 || contract == LanguageContractV470 {
		if contract == LanguageContractV470 {
			if match, ok := helperCarrierMatchLocal(function.Body); ok {
				if countMatchExpressions(function.Body) != 1 {
					return fmt.Errorf("function %s v0.47.0 later-local helper-carrier match requires exactly one match in the immutable-local sequence", function.Name)
				}
				return validateGeneralHelperCarrierMatch(function, match, functions, "v0.47.0 later-local helper-carrier match", true)
			}
		}
		if function.Body.Kind == ExprImmutableLocal && function.Body.ImmutableLocal != nil && function.Body.ImmutableLocal.Initializer != nil && function.Body.ImmutableLocal.Initializer.Kind == ExprMatch && function.Body.ImmutableLocal.Initializer.Match != nil && function.Body.ImmutableLocal.Initializer.Match.Value != nil && function.Body.ImmutableLocal.Initializer.Match.Value.Kind == ExprCall {
			if countMatchExpressions(function.Body) != 1 {
				return fmt.Errorf("function %s %s helper-carrier match local requires exactly one match as the first immutable-local initializer", function.Name, contract)
			}
			allowArithmetic := contract == LanguageContractV460
			return validateGeneralHelperCarrierMatch(function, function.Body.ImmutableLocal.Initializer.Match, functions, contract+" helper-carrier match local", allowArithmetic)
		}
		if function.Body.Kind == ExprMatch && function.Body.Match != nil && function.Body.Match.Value != nil && function.Body.Match.Value.Kind == ExprCall {
			return validateGeneralHelperCarrierMatch(function, function.Body.Match, functions, "v0.44.0 helper-carrier match", false)
		}
		return nil
	}
	if function.Body.Kind != ExprMatch || function.Body.Match == nil || function.Body.Match.Value == nil || function.Body.Match.Value.Kind != ExprCall {
		return nil
	}
	if contract == LanguageContractV440 {
		return validateGeneralHelperCarrierMatch(function, function.Body.Match, functions, "v0.44.0 helper-carrier match", false)
	}
	if contract != LanguageContractV430 {
		return fmt.Errorf("function %s helper-result match requires language contract %q", function.Name, LanguageContractV430)
	}
	match := function.Body.Match
	call := match.Value.Call
	text := Type{Kind: TypePrimitive, Primitive: PrimitiveString}
	if call == nil || countMatchExpressions(function.Body) != 1 || len(function.Parameters) != 1 || !TypeEqual(function.Parameters[0].Type, text) || !TypeEqual(function.ReturnType, text) {
		return fmt.Errorf("function %s helper-result match requires one string parameter, string return, and one complete match", function.Name)
	}
	if len(call.Arguments) != 1 || !directReference(call.Arguments[0]) || *call.Arguments[0].Parameter != 0 {
		return fmt.Errorf("function %s helper-result match requires its sole direct string parameter as the helper argument", function.Name)
	}
	if !isTextResultType(match.Value.Type) {
		return fmt.Errorf("function %s helper-result match carrier must be Result<string, string>", function.Name)
	}
	target, ok := functions[call.Target.PackageID+"\x00"+call.Target.Path]
	if !ok || semanticOwnerPath(target.Identity.Path) != semanticOwnerPath(function.Identity.Path) || len(target.Parameters) != 1 || !TypeEqual(target.Parameters[0].Type, text) || !isTextResultType(target.ReturnType) {
		return fmt.Errorf("function %s helper-result match target must be one same-owner string -> Result<string, string> function", function.Name)
	}
	if len(match.Arms) != 2 || match.Arms[0].Tag != "ok" || match.Arms[0].Binding == nil || match.Arms[1].Tag != "err" || match.Arms[1].Binding == nil {
		return fmt.Errorf("function %s helper-result match requires source-ordered ok(binding) and err(binding) arms", function.Name)
	}
	return nil
}

func priorLocalCarrierMatch(expression Expr) (*ImmutableLocal, *Match, bool) {
	for expression.Kind == ExprImmutableLocal && expression.ImmutableLocal != nil && expression.ImmutableLocal.Initializer != nil && expression.ImmutableLocal.Return != nil {
		carrier := expression.ImmutableLocal
		next := carrier.Return
		if carrier.Initializer.Kind == ExprCall && next.Kind == ExprImmutableLocal && next.ImmutableLocal != nil && next.ImmutableLocal.Initializer != nil && next.ImmutableLocal.Return != nil {
			matched := next.ImmutableLocal.Initializer
			if matched.Kind == ExprMatch && matched.Match != nil && matched.Match.Value != nil && matched.Match.Value.Kind == ExprReference && matched.Match.Value.Parameter != nil && *matched.Match.Value.Parameter == carrier.Position && TypeEqual(matched.Match.Value.Type, carrier.Type) {
				return carrier, matched.Match, true
			}
		}
		expression = *carrier.Return
	}
	return nil, nil, false
}

type priorLocalCarrierMatchPair struct {
	carrier *ImmutableLocal
	matched *ImmutableLocal
	match   *Match
}

func priorLocalCarrierMatches(expression Expr) []priorLocalCarrierMatchPair {
	pairs := make([]priorLocalCarrierMatchPair, 0, 2)
	for expression.Kind == ExprImmutableLocal && expression.ImmutableLocal != nil && expression.ImmutableLocal.Initializer != nil && expression.ImmutableLocal.Return != nil {
		carrier := expression.ImmutableLocal
		next := carrier.Return
		if carrier.Initializer.Kind == ExprCall && next.Kind == ExprImmutableLocal && next.ImmutableLocal != nil && next.ImmutableLocal.Initializer != nil && next.ImmutableLocal.Return != nil {
			matched := next.ImmutableLocal.Initializer
			if matched.Kind == ExprMatch && matched.Match != nil && matched.Match.Value != nil && matched.Match.Value.Kind == ExprReference && matched.Match.Value.Parameter != nil && *matched.Match.Value.Parameter == carrier.Position && TypeEqual(matched.Match.Value.Type, carrier.Type) {
				pairs = append(pairs, priorLocalCarrierMatchPair{carrier: carrier, matched: next.ImmutableLocal, match: matched.Match})
				expression = *next.ImmutableLocal.Return
				continue
			}
		}
		expression = *carrier.Return
	}
	return pairs
}

func directFunctionParameter(expression *Expr, parameterCount int) bool {
	return expression != nil && expression.Kind == ExprReference && expression.Parameter != nil && *expression.Parameter >= 0 && *expression.Parameter < parameterCount
}

func helperCarrierMatchLocal(expression Expr) (*Match, bool) {
	for expression.Kind == ExprImmutableLocal && expression.ImmutableLocal != nil && expression.ImmutableLocal.Initializer != nil && expression.ImmutableLocal.Return != nil {
		initializer := expression.ImmutableLocal.Initializer
		if initializer.Kind == ExprMatch && initializer.Match != nil && initializer.Match.Value != nil && initializer.Match.Value.Kind == ExprCall {
			return initializer.Match, true
		}
		expression = *expression.ImmutableLocal.Return
	}
	return nil, false
}

func validateGeneralHelperCarrierMatch(function Function, match *Match, functions map[string]Function, contractName string, allowArithmetic bool) error {
	return validateGeneralHelperCarrierMatchCount(function, match, functions, contractName, allowArithmetic, 1)
}

func validateGeneralHelperCarrierMatchCount(function Function, match *Match, functions map[string]Function, contractName string, allowArithmetic bool, expectedMatches int) error {
	call := match.Value.Call
	if call == nil || countMatchExpressions(function.Body) != expectedMatches || len(function.Parameters) == 0 {
		if expectedMatches == 1 {
			return fmt.Errorf("function %s %s requires one or more parameters and exactly one match", function.Name, contractName)
		}
		return fmt.Errorf("function %s %s requires one or more parameters and exactly %d match expressions", function.Name, contractName, expectedMatches)
	}
	if len(call.Arguments) != len(function.Parameters) {
		return fmt.Errorf("function %s %s requires every caller parameter exactly once in declaration order", function.Name, contractName)
	}
	for position, argument := range call.Arguments {
		if !directReference(argument) || *argument.Parameter != position {
			return fmt.Errorf("function %s %s argument %d is not the corresponding direct caller parameter", function.Name, contractName, position+1)
		}
	}
	if !isGeneralHelperCarrierType(match.Value.Type) && !(allowArithmetic && isArithmeticResultType(match.Value.Type)) {
		if allowArithmetic {
			return fmt.Errorf("function %s %s requires an admitted Optional, bounded Result, or checked-arithmetic Result carrier", function.Name, contractName)
		}
		return fmt.Errorf("function %s %s requires an admitted Optional or bounded Result carrier", function.Name, contractName)
	}
	target, ok := functions[call.Target.PackageID+"\x00"+call.Target.Path]
	if !ok || semanticOwnerPath(target.Identity.Path) != semanticOwnerPath(function.Identity.Path) || len(target.Parameters) != len(function.Parameters) || !TypeEqual(target.ReturnType, match.Value.Type) {
		return fmt.Errorf("function %s %s target has an invalid owner or signature", function.Name, contractName)
	}
	for position := range function.Parameters {
		if !TypeEqual(target.Parameters[position].Type, function.Parameters[position].Type) {
			return fmt.Errorf("function %s %s target parameter %d does not match the caller", function.Name, contractName, position+1)
		}
	}
	if match.Value.Type.Kind == TypeOptional {
		if len(match.Arms) != 2 || match.Arms[0].Tag != "some" || match.Arms[0].Binding == nil || match.Arms[1].Tag != "none" || match.Arms[1].Binding != nil {
			return fmt.Errorf("function %s %s requires source-ordered some(binding) and none arms", function.Name, contractName)
		}
		return nil
	}
	if len(match.Arms) != 2 || match.Arms[0].Tag != "ok" || match.Arms[0].Binding == nil || match.Arms[1].Tag != "err" || match.Arms[1].Binding == nil {
		return fmt.Errorf("function %s %s requires source-ordered ok(binding) and err(binding) arms", function.Name, contractName)
	}
	return nil
}

func validateDependentSecondHelperCarrierMatch(function Function, firstSelected *ImmutableLocal, match *Match, functions map[string]Function) error {
	return validateDependentHelperCarrierMatch(function, firstSelected, match, functions, "v0.50.0 dependent second helper-carrier match", "first selected local", 2)
}

func validateDependentHelperCarrierMatch(function Function, previousSelected *ImmutableLocal, match *Match, functions map[string]Function, contractName string, selectedDescription string, expectedMatches int) error {
	return validateSelectedHelperCarrierMatch(function, []*ImmutableLocal{previousSelected}, match, functions, contractName, selectedDescription, expectedMatches)
}

func validateSelectedHelperCarrierMatch(function Function, selectedLocals []*ImmutableLocal, match *Match, functions map[string]Function, contractName string, selectedDescription string, expectedMatches int) error {
	if len(selectedLocals) == 0 || match == nil || match.Value == nil || match.Value.Call == nil || len(function.Parameters) == 0 || countMatchExpressions(function.Body) != expectedMatches {
		return fmt.Errorf("function %s %s requires one or more parameters and exactly %d match expressions", function.Name, contractName, expectedMatches)
	}
	for _, selected := range selectedLocals {
		if selected == nil {
			return fmt.Errorf("function %s %s requires a complete selected-local dependency sequence", function.Name, contractName)
		}
	}
	call := match.Value.Call
	if len(call.Arguments) != len(function.Parameters)+len(selectedLocals) {
		return fmt.Errorf("function %s %s requires the %s followed by every caller parameter exactly once in declaration order", function.Name, contractName, selectedDescription)
	}
	for position, selected := range selectedLocals {
		if !directReference(call.Arguments[position]) || *call.Arguments[position].Parameter != selected.Position {
			return fmt.Errorf("function %s %s argument %d is not the %s", function.Name, contractName, position+1, selectedDescription)
		}
	}
	for position, argument := range call.Arguments[len(selectedLocals):] {
		if !directReference(argument) || *argument.Parameter != position {
			return fmt.Errorf("function %s %s argument %d is not the corresponding direct caller parameter", function.Name, contractName, position+len(selectedLocals)+1)
		}
	}
	if !isGeneralHelperCarrierType(match.Value.Type) && !isArithmeticResultType(match.Value.Type) {
		return fmt.Errorf("function %s %s requires an admitted Optional, bounded Result, or checked-arithmetic Result carrier", function.Name, contractName)
	}
	target, ok := functions[call.Target.PackageID+"\x00"+call.Target.Path]
	if !ok || semanticOwnerPath(target.Identity.Path) != semanticOwnerPath(function.Identity.Path) || len(target.Parameters) != len(function.Parameters)+len(selectedLocals) || !TypeEqual(target.ReturnType, match.Value.Type) {
		return fmt.Errorf("function %s %s target has an invalid owner or signature", function.Name, contractName)
	}
	for position, selected := range selectedLocals {
		if !TypeEqual(target.Parameters[position].Type, selected.Type) {
			return fmt.Errorf("function %s %s target parameter %d does not match the %s", function.Name, contractName, position+1, selectedDescription)
		}
	}
	for position := range function.Parameters {
		offset := position + len(selectedLocals)
		if !TypeEqual(target.Parameters[offset].Type, function.Parameters[position].Type) {
			return fmt.Errorf("function %s %s target parameter %d does not match the caller", function.Name, contractName, offset+1)
		}
	}
	if match.Value.Type.Kind == TypeOptional {
		if len(match.Arms) != 2 || match.Arms[0].Tag != "some" || match.Arms[0].Binding == nil || match.Arms[1].Tag != "none" || match.Arms[1].Binding != nil {
			return fmt.Errorf("function %s %s requires source-ordered some(binding) and none arms", function.Name, contractName)
		}
		return nil
	}
	if len(match.Arms) != 2 || match.Arms[0].Tag != "ok" || match.Arms[0].Binding == nil || match.Arms[1].Tag != "err" || match.Arms[1].Binding == nil {
		return fmt.Errorf("function %s %s requires source-ordered ok(binding) and err(binding) arms", function.Name, contractName)
	}
	return nil
}

func expressionChildren(expression Expr) []*Expr {
	children := []*Expr{}
	switch expression.Kind {
	case ExprUnary:
		if expression.Unary != nil {
			children = append(children, expression.Unary.Operand)
		}
	case ExprBinary:
		if expression.Binary != nil {
			children = append(children, expression.Binary.Left, expression.Binary.Right)
		}
	case ExprConditional:
		if expression.Conditional != nil {
			children = append(children, expression.Conditional.Condition, expression.Conditional.WhenTrue, expression.Conditional.WhenFalse)
		}
	case ExprImmutableLocal:
		if expression.ImmutableLocal != nil {
			children = append(children, expression.ImmutableLocal.Initializer, expression.ImmutableLocal.Return)
		}
	case ExprCall:
		if expression.Call != nil {
			children = append(children, expression.Call.Arguments...)
		}
	case ExprTextContainsCaseFolded:
		if expression.TextContains != nil {
			children = append(children, expression.TextContains.Value, expression.TextContains.Query)
		}
	case ExprTextTrim:
		if expression.TextTrim != nil {
			children = append(children, expression.TextTrim.Value)
		}
	case ExprFieldProjection:
		if expression.Field != nil {
			children = append(children, expression.Field.Receiver)
		}
	case ExprRecordConstruct:
		if expression.Record != nil {
			for _, field := range expression.Record.Fields {
				children = append(children, field.Value)
			}
		}
	case ExprOptionalSome:
		if expression.Some != nil {
			children = append(children, expression.Some.Value)
		}
	case ExprOptionalHasValue:
		if expression.HasValue != nil {
			children = append(children, expression.HasValue.Value)
		}
	case ExprOptionalValueOr:
		if expression.ValueOr != nil {
			children = append(children, expression.ValueOr.Value, expression.ValueOr.Fallback)
		}
	case ExprPropagate:
		if expression.Propagate != nil {
			children = append(children, expression.Propagate.Value)
		}
	case ExprMatch:
		if expression.Match != nil {
			children = append(children, expression.Match.Value)
			for _, arm := range expression.Match.Arms {
				children = append(children, arm.Body)
			}
		}
	case ExprListSingleton:
		if expression.ListOne != nil {
			children = append(children, expression.ListOne.Value)
		}
	case ExprListCount:
		if expression.ListCount != nil {
			children = append(children, expression.ListCount.Value)
		}
	case ExprListAppend:
		if expression.ListAppend != nil {
			children = append(children, expression.ListAppend.Values, expression.ListAppend.Value)
		}
	case ExprListAt:
		if expression.ListAt != nil {
			children = append(children, expression.ListAt.Values, expression.ListAt.Index)
		}
	case ExprListFindByText:
		if expression.ListFind != nil {
			children = append(children, expression.ListFind.Values, expression.ListFind.Key)
		}
	case ExprListFilterByText:
		if expression.ListFilter != nil {
			children = append(children, expression.ListFilter.Values, expression.ListFilter.Key)
		}
	case ExprListFilterPredicate:
		if expression.ListFilterPredicate != nil {
			children = append(children, expression.ListFilterPredicate.Values)
			children = append(children, expression.ListFilterPredicate.Arguments...)
		}
	case ExprListFilterContainsCaseFolded:
		if expression.ListFilterContainsCaseFolded != nil {
			children = append(children, expression.ListFilterContainsCaseFolded.Values, expression.ListFilterContainsCaseFolded.Query)
		}
	case ExprListFilterJoinedContainsCaseFolded:
		if expression.ListFilterJoinedContainsCaseFolded != nil {
			children = append(children, expression.ListFilterJoinedContainsCaseFolded.Values, expression.ListFilterJoinedContainsCaseFolded.Query)
		}
	case ExprListSortByOrdinalText:
		if expression.ListSortByOrdinalText != nil {
			children = append(children, expression.ListSortByOrdinalText.Values)
		}
	case ExprListSortByOrdinalTexts:
		if expression.ListSortByOrdinalTexts != nil {
			children = append(children, expression.ListSortByOrdinalTexts.Values)
		}
	case ExprListSortByOrdinalDirections:
		if expression.ListSortByOrdinalDirections != nil {
			children = append(children, expression.ListSortByOrdinalDirections.Values)
		}
	case ExprResultOK:
		if expression.ResultOK != nil {
			children = append(children, expression.ResultOK.Value)
		}
	case ExprResultErr:
		if expression.ResultErr != nil {
			children = append(children, expression.ResultErr.Error)
		}
	case ExprResultIsOK:
		if expression.ResultIsOK != nil {
			children = append(children, expression.ResultIsOK.Value)
		}
	case ExprResultSuccessOr:
		if expression.SuccessOr != nil {
			children = append(children, expression.SuccessOr.Value, expression.SuccessOr.Fallback)
		}
	case ExprResultFailureOr:
		if expression.FailureOr != nil {
			children = append(children, expression.FailureOr.Value, expression.FailureOr.Fallback)
		}
	}
	return children
}

// WalkExpression visits a Core expression and its operands in deterministic
// pre-order. Returning false from visit skips that expression's descendants.
func WalkExpression(expression Expr, visit func(Expr) bool) {
	if visit == nil || !visit(expression) {
		return
	}
	for _, child := range expressionChildren(expression) {
		if child != nil {
			WalkExpression(*child, visit)
		}
	}
}

func semanticOwnerPath(path string) string {
	if position := strings.LastIndexByte(path, '.'); position >= 0 {
		return path[:position]
	}
	return ""
}

func callableIdentityEqual(left, right *CallableIdentity) bool {
	if left == nil || right == nil {
		return left == right
	}
	if len(left.Parameters) != len(right.Parameters) || !semanticTypeEqual(left.Returns, right.Returns) {
		return false
	}
	for index := range left.Parameters {
		if !semanticTypeEqual(left.Parameters[index], right.Parameters[index]) {
			return false
		}
	}
	return true
}

func isV310OrLaterContract(contract string) bool {
	switch contract {
	case LanguageContractV310, LanguageContractV320, LanguageContractV330, LanguageContractV340, LanguageContractV350, LanguageContractV360, LanguageContractV370, LanguageContractV380, LanguageContractV390, LanguageContractV400, LanguageContractV410, LanguageContractV420, LanguageContractV430, LanguageContractV440, LanguageContractV450, LanguageContractV460, LanguageContractV470, LanguageContractV480, LanguageContractV490, LanguageContractV500, LanguageContractV510, LanguageContractV520, LanguageContractV530, LanguageContractV540, LanguageContractV550, LanguageContractV560, LanguageContractV570, LanguageContractV580, LanguageContractV590, LanguageContractV600, LanguageContractV610, LanguageContractV620, LanguageContractV630, LanguageContractV640, LanguageContractV650, LanguageContractV660, LanguageContractV670, LanguageContractV680, LanguageContractV690, LanguageContractV700, LanguageContractV710, LanguageContractV720, LanguageContractV730:
		return true
	default:
		return false
	}
}

func semanticTypeEqual(left, right SemanticType) bool {
	if left.Kind != right.Kind || left.Primitive != right.Primitive || left.PackageID != right.PackageID || left.Path != right.Path || left.Name != right.Name || len(left.Arguments) != len(right.Arguments) {
		return false
	}
	for index := range left.Arguments {
		if !semanticTypeEqual(left.Arguments[index], right.Arguments[index]) {
			return false
		}
	}
	return true
}

func isNamedPredicateFunction(function Function) bool {
	if !TypeEqual(function.ReturnType, Type{Kind: TypePrimitive, Primitive: PrimitiveBool}) || len(function.Parameters) < 2 || function.Parameters[0].Type.Kind != TypeRecord {
		return false
	}
	for _, parameter := range function.Parameters[1:] {
		if parameter.Type.Kind != TypePrimitive {
			return false
		}
	}
	return validateNamedPredicateExpr(function.Body, 0) == nil
}

func validateNamedPredicateExpr(expression Expr, rowPosition int) error {
	switch expression.Kind {
	case ExprLiteral:
		return nil
	case ExprReference:
		if expression.Parameter == nil || *expression.Parameter == rowPosition {
			return fmt.Errorf("predicate record parameter requires one-hop field projection")
		}
		return nil
	case ExprFieldProjection:
		if expression.Field == nil || expression.Field.Receiver == nil || expression.Field.Receiver.Kind != ExprReference || expression.Field.Receiver.Parameter == nil || *expression.Field.Receiver.Parameter != rowPosition || expression.Type.Kind != TypePrimitive {
			return fmt.Errorf("predicate field projection must be one-hop primitive record data")
		}
		return nil
	case ExprUnary:
		if expression.Unary == nil || expression.Unary.Operator != OperatorNot || expression.Unary.Operand == nil {
			return fmt.Errorf("predicate unary expression must be logical not")
		}
		return validateNamedPredicateExpr(*expression.Unary.Operand, rowPosition)
	case ExprBinary:
		if expression.Binary == nil || expression.Binary.Left == nil || expression.Binary.Right == nil {
			return fmt.Errorf("predicate binary expression is incomplete")
		}
		switch expression.Binary.Operator {
		case OperatorAnd, OperatorOr, OperatorEqual, OperatorNotEqual, OperatorLessThan, OperatorLessOrEqual, OperatorGreaterThan, OperatorGreaterOrEqual:
		default:
			return fmt.Errorf("predicate binary operator is not admitted")
		}
		if err := validateNamedPredicateExpr(*expression.Binary.Left, rowPosition); err != nil {
			return err
		}
		return validateNamedPredicateExpr(*expression.Binary.Right, rowPosition)
	case ExprTextContainsCaseFolded:
		if expression.TextContains == nil || expression.TextContains.Value == nil || expression.TextContains.Query == nil {
			return fmt.Errorf("predicate contains_casefolded is incomplete")
		}
		if err := validateNamedPredicateExpr(*expression.TextContains.Value, rowPosition); err != nil {
			return err
		}
		return validateNamedPredicateExpr(*expression.TextContains.Query, rowPosition)
	case ExprTextTrim:
		if expression.TextTrim == nil || expression.TextTrim.Value == nil {
			return fmt.Errorf("predicate trim is incomplete")
		}
		return validateNamedPredicateExpr(*expression.TextTrim.Value, rowPosition)
	default:
		return fmt.Errorf("predicate expression kind %q is not admitted", expression.Kind)
	}
}

func validateDirectListFilterPredicateFunction(function Function) error {
	filter := function.Body.ListFilterPredicate
	if filter == nil || filter.Values == nil || function.ReturnType.Kind != TypeList || function.ReturnType.List == nil || len(function.Parameters) < 2 || !TypeEqual(function.Parameters[0].Type, function.ReturnType) || filter.Predicate.PackageID == "" || filter.Predicate.Path == "" || filter.Predicate.Callable == nil || filter.PredicateName == "" || len(filter.Arguments) != len(function.Parameters)-1 {
		return fmt.Errorf("named predicate filter requires one direct record List followed by direct primitive arguments")
	}
	if filter.Values.Kind != ExprReference || filter.Values.Parameter == nil || *filter.Values.Parameter != 0 {
		return fmt.Errorf("named predicate filter values must be direct parameter 0")
	}
	for position, argument := range filter.Arguments {
		if argument == nil || argument.Kind != ExprReference || argument.Parameter == nil || *argument.Parameter != position+1 || !TypeEqual(argument.Type, function.Parameters[position+1].Type) || argument.Type.Kind != TypePrimitive {
			return fmt.Errorf("named predicate filter argument %d must be a direct matching primitive parameter", position+1)
		}
	}
	return nil
}

func validateType(value Type) error {
	// Executable Core types are normalized. SemanticType separately retains
	// source-level primitive int/float and named/applied identities.
	switch value.Kind {
	case TypePrimitive, TypeNumeric, TypeArithmeticError, TypeResult, TypeOptional, TypeList, TypeRecord:
	default:
		return fmt.Errorf("unsupported Core type kind %q", value.Kind)
	}
	if (value.Kind != TypePrimitive && value.Primitive != "") ||
		(value.Kind != TypeNumeric && value.Numeric != nil) ||
		(value.Kind != TypeRecord && value.Record != nil) ||
		(value.Kind != TypeRecord && value.Kind != TypeList && (value.Identity != nil || value.Name != "")) ||
		len(value.Arguments) != 0 {
		return fmt.Errorf("%s type carries a non-%s representation", value.Kind, value.Kind)
	}
	if value.Kind != TypeResult && value.Result != nil {
		return fmt.Errorf("non-result type carries a result representation")
	}
	if value.Kind == TypeResult {
		if value.Result == nil {
			return fmt.Errorf("result type has no success/failure shape")
		}
		if !isArithmeticResultType(value) && !isBoundedValueResultType(value) {
			return fmt.Errorf("result type is outside the checked-arithmetic and bounded value envelopes")
		}
		if err := validateType(value.Result.Success); err != nil {
			return fmt.Errorf("result success type: %w", err)
		}
		if err := validateType(value.Result.Failure); err != nil {
			return fmt.Errorf("result failure type: %w", err)
		}
		if value.Primitive != "" || value.Numeric != nil || value.Optional != nil || value.List != nil || value.Record != nil || value.Identity != nil || value.Name != "" || len(value.Arguments) != 0 {
			return fmt.Errorf("result type carries a non-result representation")
		}
		return nil
	}
	if value.Kind != TypeList && value.List != nil {
		return fmt.Errorf("non-list type carries a list representation")
	}
	if value.Kind == TypeList {
		if value.List == nil || value.List.Element.Kind != TypeRecord || value.Identity == nil || value.Identity.PackageID != BuiltinPackageID || value.Identity.Path != ListSemanticPath || value.Identity.Callable != nil || value.Name != "List" {
			return fmt.Errorf("list type requires one identified primitive-record element type")
		}
		if err := validateType(value.List.Element); err != nil {
			return fmt.Errorf("list element type: %w", err)
		}
		if value.Primitive != "" || value.Numeric != nil || value.Result != nil || value.Optional != nil || value.Record != nil || len(value.Arguments) != 0 {
			return fmt.Errorf("list type carries a non-list representation")
		}
		return nil
	}
	if value.Kind != TypeOptional && value.Optional != nil {
		return fmt.Errorf("non-optional type carries an optional representation")
	}
	if value.Kind == TypeOptional {
		if value.Optional == nil || !isOptionalValueType(value.Optional.Value) {
			return fmt.Errorf("optional type requires one primitive or primitive-record value type")
		}
		if err := validateType(value.Optional.Value); err != nil {
			return fmt.Errorf("optional value type: %w", err)
		}
		if value.Primitive != "" || value.Numeric != nil || value.Result != nil || value.List != nil || value.Record != nil || value.Identity != nil || value.Name != "" || len(value.Arguments) != 0 {
			return fmt.Errorf("optional type carries a non-optional representation")
		}
		return nil
	}
	switch value.Kind {
	case TypePrimitive:
		if value.Primitive != PrimitiveString && value.Primitive != PrimitiveBool {
			return fmt.Errorf("unsupported Core primitive %q", value.Primitive)
		}
		return nil
	case TypeNumeric:
		if value.Numeric == nil {
			return fmt.Errorf("numeric type has no representation")
		}
		numeric := value.Numeric
		if numeric.Bits != 64 || !((numeric.Representation == NumericInteger && numeric.Signed) ||
			(numeric.Representation == NumericBinaryFloat && !numeric.Signed)) {
			return fmt.Errorf("unsupported Core numeric representation %q/%d signed=%t", numeric.Representation, numeric.Bits, numeric.Signed)
		}
		return nil
	case TypeArithmeticError:
		return nil
	}
	if value.Record == nil || value.Identity == nil || value.Identity.PackageID == "" || value.Identity.Path == "" || value.Identity.Callable != nil || value.Name == "" || len(value.Record.Fields) == 0 {
		return fmt.Errorf("record type has an invalid identity or field schema")
	}
	if value.Primitive != "" || value.Numeric != nil || value.Result != nil || value.Optional != nil || value.List != nil || len(value.Arguments) != 0 {
		return fmt.Errorf("record type carries a non-record representation")
	}
	seenNames := map[string]struct{}{}
	seenIdentities := map[string]struct{}{}
	for index, field := range value.Record.Fields {
		if field.Name == "" || field.Identity.PackageID == "" || field.Identity.Path == "" || field.Identity.Callable != nil {
			return fmt.Errorf("record field %d has an invalid identity", index)
		}
		if field.Identity.PackageID != value.Identity.PackageID || !strings.HasPrefix(field.Identity.Path, value.Identity.Path+".") {
			return fmt.Errorf("record field %d identity is outside its record identity", index)
		}
		if _, duplicate := seenNames[field.Name]; duplicate {
			return fmt.Errorf("record field %d repeats name %q", index, field.Name)
		}
		seenNames[field.Name] = struct{}{}
		identity := field.Identity.PackageID + "\x00" + field.Identity.Path
		if _, duplicate := seenIdentities[identity]; duplicate {
			return fmt.Errorf("record field %d repeats semantic identity", index)
		}
		seenIdentities[identity] = struct{}{}
		if !isPrimitiveRecordFieldType(field.Type) {
			return fmt.Errorf("record field %d has non-primitive type %q", index, field.Type.Kind)
		}
		if err := validateType(field.Type); err != nil {
			return fmt.Errorf("record field %d type: %w", index, err)
		}
	}
	return nil
}

func isArithmeticResultType(value Type) bool {
	return value.Kind == TypeResult && value.Result != nil && value.Result.Failure.Kind == TypeArithmeticError && (TypeEqual(value.Result.Success, SignedInteger(64)) || TypeEqual(value.Result.Success, BinaryFloat(64)))
}

func isSnapshotResultType(value Type) bool {
	return value.Kind == TypeResult && value.Result != nil && value.Result.Success.Kind == TypeList && value.Result.Success.List != nil && value.Result.Success.List.Element.Kind == TypeRecord && value.Result.Failure.Kind == TypePrimitive && value.Result.Failure.Primitive == PrimitiveString
}

func isTextResultType(value Type) bool {
	text := Type{Kind: TypePrimitive, Primitive: PrimitiveString}
	return value.Kind == TypeResult && value.Result != nil && TypeEqual(value.Result.Success, text) && TypeEqual(value.Result.Failure, text)
}

func isBoundedValueResultType(value Type) bool {
	return isSnapshotResultType(value) || isTextResultType(value)
}

func isGeneralHelperCarrierType(value Type) bool {
	if value.Kind == TypeOptional && value.Optional != nil {
		return isOptionalValueType(value.Optional.Value)
	}
	return isBoundedValueResultType(value)
}

func isPrimitiveOptionalValueType(value Type) bool {
	if value.Kind == TypePrimitive {
		return (value.Primitive == PrimitiveString || value.Primitive == PrimitiveBool) && value.Numeric == nil && value.Result == nil && value.Optional == nil && value.Record == nil && value.Identity == nil && value.Name == "" && len(value.Arguments) == 0
	}
	if value.Kind != TypeNumeric || value.Numeric == nil || value.Primitive != "" || value.Result != nil || value.Optional != nil || value.Record != nil || value.Identity != nil || value.Name != "" || len(value.Arguments) != 0 {
		return false
	}
	return (value.Numeric.Representation == NumericInteger && value.Numeric.Bits == 64 && value.Numeric.Signed) ||
		(value.Numeric.Representation == NumericBinaryFloat && value.Numeric.Bits == 64 && !value.Numeric.Signed)
}

func isOptionalValueType(value Type) bool {
	if isPrimitiveOptionalValueType(value) {
		return true
	}
	return value.Kind == TypeRecord && validateType(value) == nil
}

func isPrimitiveRecordFieldType(value Type) bool {
	if value.Kind == TypePrimitive {
		return value.Primitive == PrimitiveString || value.Primitive == PrimitiveBool
	}
	if value.Kind != TypeNumeric || value.Numeric == nil {
		return false
	}
	return (value.Numeric.Representation == NumericInteger && value.Numeric.Bits == 64 && value.Numeric.Signed) ||
		(value.Numeric.Representation == NumericBinaryFloat && value.Numeric.Bits == 64 && !value.Numeric.Signed)
}

func validateExpr(expression Expr, parameters []Parameter) error {
	if err := validateType(expression.Type); err != nil {
		return fmt.Errorf("%s expression type: %w", expression.Kind, err)
	}
	switch expression.Kind {
	case ExprLiteral:
		if expression.Literal == nil || !isLiteralType(expression.Type) {
			return fmt.Errorf("literal has an invalid type")
		}
		if expression.Type.Kind == TypePrimitive && expression.Type.Primitive == PrimitiveString {
			if err := ValidateText(expression.Literal.String); err != nil {
				return fmt.Errorf("literal %w", err)
			}
		}
	case ExprReference:
		if expression.Parameter == nil || *expression.Parameter < 0 || *expression.Parameter >= len(parameters) {
			return fmt.Errorf("reference has an invalid parameter position")
		}
		if !TypeEqual(expression.Type, parameters[*expression.Parameter].Type) {
			return fmt.Errorf("reference type does not match parameter %d", *expression.Parameter)
		}
	case ExprUnary:
		if expression.Unary == nil || expression.Unary.Operand == nil {
			return fmt.Errorf("unary expression is incomplete")
		}
		if err := validateExpr(*expression.Unary.Operand, parameters); err != nil {
			return err
		}
		switch expression.Unary.Operator {
		case OperatorNot:
			boolean := Type{Kind: TypePrimitive, Primitive: PrimitiveBool}
			if !TypeEqual(expression.Unary.Operand.Type, boolean) || !TypeEqual(expression.Type, boolean) {
				return fmt.Errorf("operator %q requires and returns bool", expression.Unary.Operator)
			}
		case OperatorNegate:
			expected, err := ArithmeticResultType(expression.Unary.Operator, expression.Unary.Operand.Type, nil)
			if err != nil {
				return err
			}
			if !TypeEqual(expression.Type, expected) {
				return fmt.Errorf("operator %q has an invalid Result type", expression.Unary.Operator)
			}
		default:
			return fmt.Errorf("unsupported unary operator %q", expression.Unary.Operator)
		}
	case ExprBinary:
		if expression.Binary == nil || expression.Binary.Left == nil || expression.Binary.Right == nil {
			return fmt.Errorf("binary expression is incomplete")
		}
		if err := validateExpr(*expression.Binary.Left, parameters); err != nil {
			return err
		}
		if err := validateExpr(*expression.Binary.Right, parameters); err != nil {
			return err
		}
		operator := expression.Binary.Operator
		switch operator {
		case OperatorAdd:
			text := Type{Kind: TypePrimitive, Primitive: PrimitiveString}
			if TypeEqual(expression.Binary.Left.Type, text) && TypeEqual(expression.Binary.Right.Type, text) && TypeEqual(expression.Type, text) {
				break
			}
			expected, err := ArithmeticResultType(operator, expression.Binary.Left.Type, &expression.Binary.Right.Type)
			if err != nil {
				return err
			}
			if !TypeEqual(expression.Type, expected) {
				return fmt.Errorf("operator %q has an invalid Result type", operator)
			}
		case OperatorSubtract, OperatorMultiply, OperatorDivide:
			expected, err := ArithmeticResultType(operator, expression.Binary.Left.Type, &expression.Binary.Right.Type)
			if err != nil {
				return err
			}
			if !TypeEqual(expression.Type, expected) {
				return fmt.Errorf("operator %q has an invalid Result type", operator)
			}
		case OperatorEqual, OperatorNotEqual, OperatorLessThan, OperatorLessOrEqual, OperatorGreaterThan, OperatorGreaterOrEqual:
			boolean := Type{Kind: TypePrimitive, Primitive: PrimitiveBool}
			if !TypeEqual(expression.Binary.Left.Type, expression.Binary.Right.Type) || !TypeEqual(expression.Type, boolean) {
				return fmt.Errorf("operator %q has mismatched operand or result types", operator)
			}
			if expression.Binary.Left.Type.Kind == TypeRecord && operator != OperatorEqual && operator != OperatorNotEqual {
				return fmt.Errorf("record values support structural equality only, not operator %q", operator)
			}
		case OperatorAnd, OperatorOr:
			boolean := Type{Kind: TypePrimitive, Primitive: PrimitiveBool}
			if !TypeEqual(expression.Binary.Left.Type, boolean) || !TypeEqual(expression.Binary.Right.Type, boolean) || !TypeEqual(expression.Type, boolean) {
				return fmt.Errorf("operator %q requires and returns bool", operator)
			}
		default:
			return fmt.Errorf("unsupported binary operator %q", operator)
		}
	case ExprConditional:
		conditional := expression.Conditional
		boolean := Type{Kind: TypePrimitive, Primitive: PrimitiveBool}
		if conditional == nil || conditional.Condition == nil || conditional.WhenTrue == nil || conditional.WhenFalse == nil {
			return fmt.Errorf("conditional expression is incomplete")
		}
		if err := validateExpr(*conditional.Condition, parameters); err != nil {
			return fmt.Errorf("conditional condition: %w", err)
		}
		if err := validateExpr(*conditional.WhenTrue, parameters); err != nil {
			return fmt.Errorf("conditional true branch: %w", err)
		}
		if err := validateExpr(*conditional.WhenFalse, parameters); err != nil {
			return fmt.Errorf("conditional false branch: %w", err)
		}
		if !TypeEqual(conditional.Condition.Type, boolean) {
			return fmt.Errorf("conditional condition is not bool")
		}
		if !TypeEqual(conditional.WhenTrue.Type, conditional.WhenFalse.Type) || !TypeEqual(expression.Type, conditional.WhenTrue.Type) {
			return fmt.Errorf("conditional branches and result must have exactly the same type")
		}
	case ExprImmutableLocal:
		local := expression.ImmutableLocal
		if local == nil || local.Name == "" || local.Initializer == nil || local.Return == nil || local.Position != len(parameters) {
			return fmt.Errorf("immutable local is incomplete or not canonically positioned")
		}
		for _, binding := range parameters {
			if binding.Name == local.Name {
				return fmt.Errorf("immutable local %q shadows an existing binding", local.Name)
			}
		}
		if err := validateType(local.Type); err != nil {
			return fmt.Errorf("immutable local type: %w", err)
		}
		if err := validateExpr(*local.Initializer, parameters); err != nil {
			return fmt.Errorf("immutable local initializer: %w", err)
		}
		if !TypeEqual(local.Initializer.Type, local.Type) {
			return fmt.Errorf("immutable local initializer type does not match its declaration")
		}
		scoped := append(append([]Parameter{}, parameters...), Parameter{Position: local.Position, Name: local.Name, Type: local.Type})
		if err := validateExpr(*local.Return, scoped); err != nil {
			return fmt.Errorf("immutable local return: %w", err)
		}
		if !TypeEqual(local.Return.Type, expression.Type) {
			return fmt.Errorf("immutable local return type does not match its expression")
		}
	case ExprCall:
		if expression.Call == nil || expression.Call.TargetName == "" || expression.Call.Target.PackageID == "" || expression.Call.Target.Path == "" || expression.Call.Target.Callable == nil {
			return fmt.Errorf("pure call is incomplete")
		}
		for position, argument := range expression.Call.Arguments {
			if argument == nil {
				return fmt.Errorf("pure call argument %d is missing", position+1)
			}
			if err := validateExpr(*argument, parameters); err != nil {
				return fmt.Errorf("pure call argument %d: %w", position+1, err)
			}
		}
	case ExprTextContainsCaseFolded:
		text := Type{Kind: TypePrimitive, Primitive: PrimitiveString}
		boolean := Type{Kind: TypePrimitive, Primitive: PrimitiveBool}
		if expression.TextContains == nil || expression.TextContains.Value == nil || expression.TextContains.Query == nil || !TypeEqual(expression.Type, boolean) {
			return fmt.Errorf("contains_casefolded expression is incomplete or does not return bool")
		}
		if err := validateExpr(*expression.TextContains.Value, parameters); err != nil {
			return fmt.Errorf("contains_casefolded value: %w", err)
		}
		if err := validateExpr(*expression.TextContains.Query, parameters); err != nil {
			return fmt.Errorf("contains_casefolded query: %w", err)
		}
		if !TypeEqual(expression.TextContains.Value.Type, text) || !TypeEqual(expression.TextContains.Query.Type, text) {
			return fmt.Errorf("contains_casefolded operands are not string")
		}
	case ExprTextTrim:
		text := Type{Kind: TypePrimitive, Primitive: PrimitiveString}
		if expression.TextTrim == nil || expression.TextTrim.Value == nil || !TypeEqual(expression.Type, text) {
			return fmt.Errorf("trim expression is incomplete or does not return string")
		}
		if err := validateExpr(*expression.TextTrim.Value, parameters); err != nil {
			return fmt.Errorf("trim value: %w", err)
		}
		if !TypeEqual(expression.TextTrim.Value.Type, text) {
			return fmt.Errorf("trim operand is not string")
		}
	case ExprFieldProjection:
		if expression.Field == nil || expression.Field.Receiver == nil {
			return fmt.Errorf("field projection is incomplete")
		}
		if err := validateExpr(*expression.Field.Receiver, parameters); err != nil {
			return err
		}
		receiver := expression.Field.Receiver.Type
		if receiver.Kind != TypeRecord || receiver.Record == nil {
			return fmt.Errorf("field projection receiver is not a record")
		}
		if err := validateType(receiver); err != nil {
			return fmt.Errorf("field projection receiver type: %w", err)
		}
		position := expression.Field.Position
		if position < 0 || position >= len(receiver.Record.Fields) {
			return fmt.Errorf("field projection has an invalid declared position")
		}
		field := receiver.Record.Fields[position]
		if expression.Field.Name != field.Name || expression.Field.Identity.PackageID != field.Identity.PackageID || expression.Field.Identity.Path != field.Identity.Path || expression.Field.Identity.Callable != nil {
			return fmt.Errorf("field projection does not match the record field identity at its declared position")
		}
		if !TypeEqual(expression.Type, field.Type) {
			return fmt.Errorf("field projection result type does not match the declared field type")
		}
	case ExprRecordConstruct:
		if expression.Record == nil {
			return fmt.Errorf("record construction is incomplete")
		}
		if expression.Type.Kind != TypeRecord || expression.Type.Record == nil || expression.Type.Identity == nil {
			return fmt.Errorf("record construction result is not an identified record")
		}
		if err := validateType(expression.Type); err != nil {
			return fmt.Errorf("record construction result type: %w", err)
		}
		if expression.Record.Identity.PackageID != expression.Type.Identity.PackageID || expression.Record.Identity.Path != expression.Type.Identity.Path || expression.Record.Identity.Callable != nil {
			return fmt.Errorf("record construction identity does not match its result type")
		}
		if len(expression.Record.Fields) != len(expression.Type.Record.Fields) {
			return fmt.Errorf("record construction field count does not match its schema")
		}
		if len(parameters) != len(expression.Record.Fields) {
			return fmt.Errorf("record construction requires one corresponding parameter per field")
		}
		for position, initialized := range expression.Record.Fields {
			if initialized.Value == nil {
				return fmt.Errorf("record construction field %d has no value", position)
			}
			if initialized.Position != position {
				return fmt.Errorf("record construction field %d is not in declaration order", position)
			}
			declared := expression.Type.Record.Fields[position]
			if initialized.Name != declared.Name || initialized.Identity.PackageID != declared.Identity.PackageID || initialized.Identity.Path != declared.Identity.Path || initialized.Identity.Callable != nil {
				return fmt.Errorf("record construction field %d does not match its declared identity", position)
			}
			if err := validateExpr(*initialized.Value, parameters); err != nil {
				return fmt.Errorf("record construction field %d: %w", position, err)
			}
			if initialized.Value.Kind != ExprReference || initialized.Value.Parameter == nil || *initialized.Value.Parameter != position {
				return fmt.Errorf("record construction field %d value is not its corresponding direct parameter", position)
			}
			if !TypeEqual(initialized.Value.Type, declared.Type) {
				return fmt.Errorf("record construction field %d value type does not match its declaration", position)
			}
		}
	case ExprOptionalSome:
		if expression.Some == nil || expression.Some.Value == nil || expression.Type.Kind != TypeOptional || expression.Type.Optional == nil {
			return fmt.Errorf("optional some expression is incomplete")
		}
		if err := validateType(expression.Type); err != nil {
			return fmt.Errorf("optional some result type: %w", err)
		}
		if err := validateExpr(*expression.Some.Value, parameters); err != nil {
			return fmt.Errorf("optional some value: %w", err)
		}
		if !TypeEqual(expression.Some.Value.Type, expression.Type.Optional.Value) {
			return fmt.Errorf("optional some value type does not match its result type")
		}
	case ExprOptionalNone:
		if expression.None == nil || expression.Type.Kind != TypeOptional || expression.Type.Optional == nil {
			return fmt.Errorf("optional none expression is incomplete")
		}
		if err := validateType(expression.Type); err != nil {
			return fmt.Errorf("optional none result type: %w", err)
		}
	case ExprOptionalHasValue:
		boolean := Type{Kind: TypePrimitive, Primitive: PrimitiveBool}
		if expression.HasValue == nil || expression.HasValue.Value == nil || !TypeEqual(expression.Type, boolean) {
			return fmt.Errorf("optional has_value expression is incomplete or does not return bool")
		}
		if err := validateExpr(*expression.HasValue.Value, parameters); err != nil {
			return fmt.Errorf("optional has_value operand: %w", err)
		}
		if expression.HasValue.Value.Type.Kind != TypeOptional {
			return fmt.Errorf("optional has_value operand is not Optional")
		}
	case ExprOptionalValueOr:
		if expression.ValueOr == nil || expression.ValueOr.Value == nil || expression.ValueOr.Fallback == nil {
			return fmt.Errorf("optional value_or expression is incomplete")
		}
		if err := validateExpr(*expression.ValueOr.Value, parameters); err != nil {
			return fmt.Errorf("optional value_or operand: %w", err)
		}
		if err := validateExpr(*expression.ValueOr.Fallback, parameters); err != nil {
			return fmt.Errorf("optional value_or fallback: %w", err)
		}
		if expression.ValueOr.Value.Type.Kind != TypeOptional || expression.ValueOr.Value.Type.Optional == nil {
			return fmt.Errorf("optional value_or first operand is not Optional")
		}
		if !TypeEqual(expression.ValueOr.Value.Type.Optional.Value, expression.ValueOr.Fallback.Type) || !TypeEqual(expression.Type, expression.ValueOr.Fallback.Type) {
			return fmt.Errorf("optional value_or payload, fallback, and result types do not match")
		}
	case ExprListEmpty:
		if expression.ListEmpty == nil || expression.Type.Kind != TypeList || expression.Type.List == nil {
			return fmt.Errorf("list empty expression is incomplete")
		}
		if err := validateType(expression.Type); err != nil {
			return fmt.Errorf("list empty result type: %w", err)
		}
	case ExprListSingleton:
		if expression.ListOne == nil || expression.ListOne.Value == nil || expression.Type.Kind != TypeList || expression.Type.List == nil {
			return fmt.Errorf("list singleton expression is incomplete")
		}
		if err := validateType(expression.Type); err != nil {
			return fmt.Errorf("list singleton result type: %w", err)
		}
		if err := validateExpr(*expression.ListOne.Value, parameters); err != nil {
			return fmt.Errorf("list singleton value: %w", err)
		}
		if !TypeEqual(expression.ListOne.Value.Type, expression.Type.List.Element) {
			return fmt.Errorf("list singleton value type does not match its element type")
		}
	case ExprListCount:
		if expression.ListCount == nil || expression.ListCount.Value == nil || !TypeEqual(expression.Type, SignedInteger(64)) {
			return fmt.Errorf("list count expression is incomplete or does not return signed 64-bit int")
		}
		if err := validateExpr(*expression.ListCount.Value, parameters); err != nil {
			return fmt.Errorf("list count operand: %w", err)
		}
		if expression.ListCount.Value.Type.Kind != TypeList {
			return fmt.Errorf("list count operand is not a List")
		}
	case ExprListAppend:
		if expression.ListAppend == nil || expression.ListAppend.Values == nil || expression.ListAppend.Value == nil || expression.Type.Kind != TypeList || expression.Type.List == nil {
			return fmt.Errorf("list append expression is incomplete")
		}
		if err := validateType(expression.Type); err != nil {
			return fmt.Errorf("list append result type: %w", err)
		}
		if err := validateExpr(*expression.ListAppend.Values, parameters); err != nil {
			return fmt.Errorf("list append values: %w", err)
		}
		if err := validateExpr(*expression.ListAppend.Value, parameters); err != nil {
			return fmt.Errorf("list append value: %w", err)
		}
		if !TypeEqual(expression.ListAppend.Values.Type, expression.Type) {
			return fmt.Errorf("list append values type does not match its result type")
		}
		if !TypeEqual(expression.ListAppend.Value.Type, expression.Type.List.Element) {
			return fmt.Errorf("list append value type does not match its element type")
		}
	case ExprListAt:
		if expression.ListAt == nil || expression.ListAt.Values == nil || expression.ListAt.Index == nil || expression.Type.Kind != TypeOptional || expression.Type.Optional == nil {
			return fmt.Errorf("list at expression is incomplete")
		}
		if err := validateType(expression.Type); err != nil {
			return fmt.Errorf("list at result type: %w", err)
		}
		if err := validateExpr(*expression.ListAt.Values, parameters); err != nil {
			return fmt.Errorf("list at values: %w", err)
		}
		if err := validateExpr(*expression.ListAt.Index, parameters); err != nil {
			return fmt.Errorf("list at index: %w", err)
		}
		if expression.ListAt.Values.Type.Kind != TypeList || expression.ListAt.Values.Type.List == nil || !TypeEqual(expression.ListAt.Values.Type.List.Element, expression.Type.Optional.Value) {
			return fmt.Errorf("list at values element type does not match its Optional result")
		}
		if !TypeEqual(expression.ListAt.Index.Type, SignedInteger(64)) {
			return fmt.Errorf("list at index is not signed 64-bit int")
		}
	case ExprListFindByText:
		if expression.ListFind == nil || expression.ListFind.Values == nil || expression.ListFind.Key == nil || expression.Type.Kind != TypeOptional || expression.Type.Optional == nil {
			return fmt.Errorf("list find_by expression is incomplete")
		}
		if err := validateType(expression.Type); err != nil {
			return fmt.Errorf("list find_by result type: %w", err)
		}
		if err := validateExpr(*expression.ListFind.Values, parameters); err != nil {
			return fmt.Errorf("list find_by values: %w", err)
		}
		if err := validateExpr(*expression.ListFind.Key, parameters); err != nil {
			return fmt.Errorf("list find_by key: %w", err)
		}
		valuesType := expression.ListFind.Values.Type
		if valuesType.Kind != TypeList || valuesType.List == nil || valuesType.List.Element.Kind != TypeRecord || valuesType.List.Element.Record == nil || !TypeEqual(valuesType.List.Element, expression.Type.Optional.Value) {
			return fmt.Errorf("list find_by values element type does not match its Optional record result")
		}
		if !TypeEqual(expression.ListFind.Key.Type, Type{Kind: TypePrimitive, Primitive: PrimitiveString}) {
			return fmt.Errorf("list find_by key is not string")
		}
		position := expression.ListFind.Position
		if position < 0 || position >= len(valuesType.List.Element.Record.Fields) {
			return fmt.Errorf("list find_by field position is outside the record schema")
		}
		field := valuesType.List.Element.Record.Fields[position]
		if field.Name != expression.ListFind.Name || field.Identity.PackageID != expression.ListFind.Field.PackageID || field.Identity.Path != expression.ListFind.Field.Path || expression.ListFind.Field.Callable != nil {
			return fmt.Errorf("list find_by field identity does not match the record schema")
		}
		if !TypeEqual(field.Type, Type{Kind: TypePrimitive, Primitive: PrimitiveString}) {
			return fmt.Errorf("list find_by field is not string")
		}
	case ExprListFilterByText:
		if expression.ListFilter == nil || expression.ListFilter.Values == nil || expression.ListFilter.Key == nil || expression.Type.Kind != TypeList || expression.Type.List == nil {
			return fmt.Errorf("list filter_by expression is incomplete")
		}
		if err := validateType(expression.Type); err != nil {
			return fmt.Errorf("list filter_by result type: %w", err)
		}
		if err := validateExpr(*expression.ListFilter.Values, parameters); err != nil {
			return fmt.Errorf("list filter_by values: %w", err)
		}
		if err := validateExpr(*expression.ListFilter.Key, parameters); err != nil {
			return fmt.Errorf("list filter_by key: %w", err)
		}
		valuesType := expression.ListFilter.Values.Type
		if valuesType.Kind != TypeList || valuesType.List == nil || valuesType.List.Element.Kind != TypeRecord || valuesType.List.Element.Record == nil || !TypeEqual(valuesType, expression.Type) {
			return fmt.Errorf("list filter_by values type does not match its record-list result")
		}
		if !TypeEqual(expression.ListFilter.Key.Type, Type{Kind: TypePrimitive, Primitive: PrimitiveString}) {
			return fmt.Errorf("list filter_by key is not string")
		}
		position := expression.ListFilter.Position
		if position < 0 || position >= len(valuesType.List.Element.Record.Fields) {
			return fmt.Errorf("list filter_by field position is outside the record schema")
		}
		field := valuesType.List.Element.Record.Fields[position]
		if field.Name != expression.ListFilter.Name || field.Identity.PackageID != expression.ListFilter.Field.PackageID || field.Identity.Path != expression.ListFilter.Field.Path || expression.ListFilter.Field.Callable != nil {
			return fmt.Errorf("list filter_by field identity does not match the record schema")
		}
		if !TypeEqual(field.Type, Type{Kind: TypePrimitive, Primitive: PrimitiveString}) {
			return fmt.Errorf("list filter_by field is not string")
		}
	case ExprListFilterPredicate:
		filter := expression.ListFilterPredicate
		if filter == nil || filter.Values == nil || expression.Type.Kind != TypeList || expression.Type.List == nil || filter.Predicate.PackageID == "" || filter.Predicate.Path == "" || filter.Predicate.Callable == nil || filter.PredicateName == "" {
			return fmt.Errorf("named predicate list filter expression is incomplete")
		}
		if err := validateExpr(*filter.Values, parameters); err != nil {
			return fmt.Errorf("named predicate list filter values: %w", err)
		}
		if !TypeEqual(filter.Values.Type, expression.Type) {
			return fmt.Errorf("named predicate list filter values type does not match result")
		}
		for position, argument := range filter.Arguments {
			if argument == nil {
				return fmt.Errorf("named predicate list filter argument %d is nil", position+1)
			}
			if err := validateExpr(*argument, parameters); err != nil {
				return fmt.Errorf("named predicate list filter argument %d: %w", position+1, err)
			}
		}
	case ExprListFilterContainsCaseFolded:
		filter := expression.ListFilterContainsCaseFolded
		if filter == nil || filter.Values == nil || filter.Query == nil || expression.Type.Kind != TypeList || expression.Type.List == nil {
			return fmt.Errorf("list filter_contains_casefolded expression is incomplete")
		}
		if err := validateType(expression.Type); err != nil {
			return fmt.Errorf("list filter_contains_casefolded result type: %w", err)
		}
		if err := validateExpr(*filter.Values, parameters); err != nil {
			return fmt.Errorf("list filter_contains_casefolded values: %w", err)
		}
		if err := validateExpr(*filter.Query, parameters); err != nil {
			return fmt.Errorf("list filter_contains_casefolded query: %w", err)
		}
		valuesType := filter.Values.Type
		if valuesType.Kind != TypeList || valuesType.List == nil || valuesType.List.Element.Kind != TypeRecord || valuesType.List.Element.Record == nil || !TypeEqual(valuesType, expression.Type) {
			return fmt.Errorf("list filter_contains_casefolded values type does not match its record-list result")
		}
		if !TypeEqual(filter.Query.Type, Type{Kind: TypePrimitive, Primitive: PrimitiveString}) {
			return fmt.Errorf("list filter_contains_casefolded query is not string")
		}
		position := filter.Position
		if position < 0 || position >= len(valuesType.List.Element.Record.Fields) {
			return fmt.Errorf("list filter_contains_casefolded field position is outside the record schema")
		}
		field := valuesType.List.Element.Record.Fields[position]
		if field.Name != filter.Name || field.Identity.PackageID != filter.Field.PackageID || field.Identity.Path != filter.Field.Path || filter.Field.Callable != nil {
			return fmt.Errorf("list filter_contains_casefolded field identity does not match the record schema")
		}
		if !TypeEqual(field.Type, Type{Kind: TypePrimitive, Primitive: PrimitiveString}) {
			return fmt.Errorf("list filter_contains_casefolded field is not string")
		}
	case ExprListFilterJoinedContainsCaseFolded:
		filter := expression.ListFilterJoinedContainsCaseFolded
		if filter == nil || filter.Values == nil || filter.Query == nil || len(filter.Selectors) < 2 || expression.Type.Kind != TypeList || expression.Type.List == nil {
			return fmt.Errorf("list filter_joined_contains_casefolded expression is incomplete")
		}
		if err := validateType(expression.Type); err != nil {
			return fmt.Errorf("list filter_joined_contains_casefolded result type: %w", err)
		}
		if err := validateExpr(*filter.Values, parameters); err != nil {
			return fmt.Errorf("list filter_joined_contains_casefolded values: %w", err)
		}
		if err := validateExpr(*filter.Query, parameters); err != nil {
			return fmt.Errorf("list filter_joined_contains_casefolded query: %w", err)
		}
		valuesType := filter.Values.Type
		if valuesType.Kind != TypeList || valuesType.List == nil || valuesType.List.Element.Kind != TypeRecord || valuesType.List.Element.Record == nil || !TypeEqual(valuesType, expression.Type) {
			return fmt.Errorf("list filter_joined_contains_casefolded values type does not match its record-list result")
		}
		if !TypeEqual(filter.Query.Type, Type{Kind: TypePrimitive, Primitive: PrimitiveString}) {
			return fmt.Errorf("list filter_joined_contains_casefolded query is not string")
		}
		seen := map[int]struct{}{}
		for _, selector := range filter.Selectors {
			if selector.Position < 0 || selector.Position >= len(valuesType.List.Element.Record.Fields) {
				return fmt.Errorf("list filter_joined_contains_casefolded field position is outside the record schema")
			}
			if _, exists := seen[selector.Position]; exists {
				return fmt.Errorf("list filter_joined_contains_casefolded field selectors must be distinct")
			}
			seen[selector.Position] = struct{}{}
			field := valuesType.List.Element.Record.Fields[selector.Position]
			if field.Name != selector.Name || field.Identity.PackageID != selector.Field.PackageID || field.Identity.Path != selector.Field.Path || selector.Field.Callable != nil {
				return fmt.Errorf("list filter_joined_contains_casefolded field identity does not match the record schema")
			}
			if !TypeEqual(field.Type, Type{Kind: TypePrimitive, Primitive: PrimitiveString}) {
				return fmt.Errorf("list filter_joined_contains_casefolded field is not string")
			}
		}
	case ExprListSortByOrdinalText:
		sorted := expression.ListSortByOrdinalText
		if sorted == nil || sorted.Values == nil || expression.Type.Kind != TypeList || expression.Type.List == nil {
			return fmt.Errorf("list sort_by_ordinal expression is incomplete")
		}
		if err := validateType(expression.Type); err != nil {
			return fmt.Errorf("list sort_by_ordinal result type: %w", err)
		}
		if err := validateExpr(*sorted.Values, parameters); err != nil {
			return fmt.Errorf("list sort_by_ordinal values: %w", err)
		}
		valuesType := sorted.Values.Type
		if valuesType.Kind != TypeList || valuesType.List == nil || valuesType.List.Element.Kind != TypeRecord || valuesType.List.Element.Record == nil || !TypeEqual(valuesType, expression.Type) {
			return fmt.Errorf("list sort_by_ordinal values type does not match its record-list result")
		}
		if sorted.Position < 0 || sorted.Position >= len(valuesType.List.Element.Record.Fields) {
			return fmt.Errorf("list sort_by_ordinal field position is outside the record schema")
		}
		field := valuesType.List.Element.Record.Fields[sorted.Position]
		if field.Name != sorted.Name || field.Identity.PackageID != sorted.Field.PackageID || field.Identity.Path != sorted.Field.Path || sorted.Field.Callable != nil {
			return fmt.Errorf("list sort_by_ordinal field identity does not match the record schema")
		}
		if !TypeEqual(field.Type, Type{Kind: TypePrimitive, Primitive: PrimitiveString}) {
			return fmt.Errorf("list sort_by_ordinal field is not string")
		}
	case ExprListSortByOrdinalTexts:
		sorted := expression.ListSortByOrdinalTexts
		if sorted == nil || sorted.Values == nil || len(sorted.Selectors) < 2 || expression.Type.Kind != TypeList || expression.Type.List == nil {
			return fmt.Errorf("multi-key list sort_by_ordinal expression is incomplete")
		}
		if err := validateType(expression.Type); err != nil {
			return fmt.Errorf("multi-key list sort_by_ordinal result type: %w", err)
		}
		if err := validateExpr(*sorted.Values, parameters); err != nil {
			return fmt.Errorf("multi-key list sort_by_ordinal values: %w", err)
		}
		valuesType := sorted.Values.Type
		if valuesType.Kind != TypeList || valuesType.List == nil || valuesType.List.Element.Kind != TypeRecord || valuesType.List.Element.Record == nil || !TypeEqual(valuesType, expression.Type) {
			return fmt.Errorf("multi-key list sort_by_ordinal values type does not match its record-list result")
		}
		seen := map[int]struct{}{}
		for _, selector := range sorted.Selectors {
			if selector.Position < 0 || selector.Position >= len(valuesType.List.Element.Record.Fields) {
				return fmt.Errorf("multi-key list sort_by_ordinal field position is outside the record schema")
			}
			if _, exists := seen[selector.Position]; exists {
				return fmt.Errorf("multi-key list sort_by_ordinal field selectors must be distinct")
			}
			seen[selector.Position] = struct{}{}
			field := valuesType.List.Element.Record.Fields[selector.Position]
			if field.Name != selector.Name || field.Identity.PackageID != selector.Field.PackageID || field.Identity.Path != selector.Field.Path || selector.Field.Callable != nil {
				return fmt.Errorf("multi-key list sort_by_ordinal field identity does not match the record schema")
			}
			if !TypeEqual(field.Type, Type{Kind: TypePrimitive, Primitive: PrimitiveString}) {
				return fmt.Errorf("multi-key list sort_by_ordinal field is not string")
			}
		}
	case ExprListSortByOrdinalDirections:
		sorted := expression.ListSortByOrdinalDirections
		if sorted == nil || sorted.Values == nil || len(sorted.Selectors) < 1 {
			return fmt.Errorf("directional list sort_by_ordinal expression is incomplete")
		}
		plain := make([]ListTextFieldSelector, 0, len(sorted.Selectors))
		for _, s := range sorted.Selectors {
			if s.Direction != "ascending" && s.Direction != "descending" {
				return fmt.Errorf("directional list sort_by_ordinal direction is invalid")
			}
			plain = append(plain, s.ListTextFieldSelector)
		}
		legacy := expression
		legacy.Kind = ExprListSortByOrdinalTexts
		legacy.ListSortByOrdinalDirections = nil
		legacy.ListSortByOrdinalTexts = &ListSortByOrdinalTexts{Values: sorted.Values, Selectors: plain}
		if len(plain) == 1 {
			legacy.Kind = ExprListSortByOrdinalText
			legacy.ListSortByOrdinalTexts = nil
			legacy.ListSortByOrdinalText = &ListSortByOrdinalText{Values: sorted.Values, Field: plain[0].Field, Name: plain[0].Name, Position: plain[0].Position}
		}
		return validateExpr(legacy, parameters)
	case ExprResultOK:
		if expression.ResultOK == nil || expression.ResultOK.Value == nil || !isBoundedValueResultType(expression.Type) {
			return fmt.Errorf("result ok expression is incomplete or has an invalid result type")
		}
		if err := validateExpr(*expression.ResultOK.Value, parameters); err != nil {
			return fmt.Errorf("result ok value: %w", err)
		}
		if !TypeEqual(expression.ResultOK.Value.Type, expression.Type.Result.Success) {
			return fmt.Errorf("result ok value type does not match its success type")
		}
	case ExprPropagate:
		if expression.Propagate == nil || expression.Propagate.Value == nil {
			return fmt.Errorf("propagate expression is incomplete")
		}
		if err := validateExpr(*expression.Propagate.Value, parameters); err != nil {
			return fmt.Errorf("propagate value: %w", err)
		}
		carrier := expression.Propagate.Carrier
		if err := validateType(carrier); err != nil {
			return fmt.Errorf("propagate carrier type: %w", err)
		}
		if !TypeEqual(carrier, expression.Propagate.Value.Type) {
			return fmt.Errorf("propagate carrier does not match operand")
		}
		if carrier.Kind == TypeOptional && carrier.Optional != nil && TypeEqual(expression.Type, carrier.Optional.Value) {
			break
		}
		if isBoundedValueResultType(carrier) && TypeEqual(expression.Type, carrier.Result.Success) {
			break
		}
		if isArithmeticResultType(carrier) && TypeEqual(expression.Type, carrier.Result.Success) {
			break
		}
		return fmt.Errorf("propagate requires a matching Optional, bounded Result, or checked-arithmetic Result carrier")
	case ExprResultErr:
		if expression.ResultErr == nil || expression.ResultErr.Error == nil || !isBoundedValueResultType(expression.Type) {
			return fmt.Errorf("result err expression is incomplete or has an invalid result type")
		}
		if err := validateExpr(*expression.ResultErr.Error, parameters); err != nil {
			return fmt.Errorf("result err value: %w", err)
		}
		if !TypeEqual(expression.ResultErr.Error.Type, expression.Type.Result.Failure) {
			return fmt.Errorf("result err value type does not match its failure type")
		}
	case ExprResultIsOK:
		boolean := Type{Kind: TypePrimitive, Primitive: PrimitiveBool}
		if expression.ResultIsOK == nil || expression.ResultIsOK.Value == nil || !TypeEqual(expression.Type, boolean) {
			return fmt.Errorf("result is_ok expression is incomplete or does not return bool")
		}
		if err := validateExpr(*expression.ResultIsOK.Value, parameters); err != nil {
			return fmt.Errorf("result is_ok value: %w", err)
		}
		if !isBoundedValueResultType(expression.ResultIsOK.Value.Type) {
			return fmt.Errorf("result is_ok operand is not a bounded value Result")
		}
	case ExprResultSuccessOr:
		if expression.SuccessOr == nil || expression.SuccessOr.Value == nil || expression.SuccessOr.Fallback == nil {
			return fmt.Errorf("result success_or expression is incomplete")
		}
		if err := validateExpr(*expression.SuccessOr.Value, parameters); err != nil {
			return fmt.Errorf("result success_or value: %w", err)
		}
		if err := validateExpr(*expression.SuccessOr.Fallback, parameters); err != nil {
			return fmt.Errorf("result success_or fallback: %w", err)
		}
		resultType := expression.SuccessOr.Value.Type
		if !isBoundedValueResultType(resultType) || !TypeEqual(expression.SuccessOr.Fallback.Type, resultType.Result.Success) || !TypeEqual(expression.Type, resultType.Result.Success) {
			return fmt.Errorf("result success_or operand, fallback, and return types do not match")
		}
	case ExprResultFailureOr:
		if expression.FailureOr == nil || expression.FailureOr.Value == nil || expression.FailureOr.Fallback == nil {
			return fmt.Errorf("result failure_or expression is incomplete")
		}
		if err := validateExpr(*expression.FailureOr.Value, parameters); err != nil {
			return fmt.Errorf("result failure_or value: %w", err)
		}
		if err := validateExpr(*expression.FailureOr.Fallback, parameters); err != nil {
			return fmt.Errorf("result failure_or fallback: %w", err)
		}
		resultType := expression.FailureOr.Value.Type
		if !isBoundedValueResultType(resultType) || !TypeEqual(expression.FailureOr.Fallback.Type, resultType.Result.Failure) || !TypeEqual(expression.Type, resultType.Result.Failure) {
			return fmt.Errorf("result failure_or operand, fallback, and return types do not match")
		}
	case ExprMatch:
		if expression.Match == nil || expression.Match.Value == nil || len(expression.Match.Arms) == 0 {
			return fmt.Errorf("match expression is incomplete")
		}
		if err := validateExpr(*expression.Match.Value, parameters); err != nil {
			return fmt.Errorf("match value: %w", err)
		}
		carrier := expression.Match.Value.Type
		if carrier.Kind != TypeOptional && carrier.Kind != TypeResult {
			return fmt.Errorf("match value is not tagged")
		}
		tags := []string{"some", "none"}
		payloadTags := map[string]bool{"some": true}
		if carrier.Kind == TypeResult {
			tags = []string{"ok", "err"}
			payloadTags = map[string]bool{"ok": true, "err": true}
		}
		seen := map[string]bool{}
		wildcard := false
		for _, arm := range expression.Match.Arms {
			if wildcard {
				return fmt.Errorf("match arm is unreachable after wildcard")
			}
			if arm.Tag == "_" {
				wildcard = true
				if arm.Binding != nil {
					return fmt.Errorf("match wildcard cannot bind a payload")
				}
			} else {
				valid := false
				for _, tag := range tags {
					if arm.Tag == tag {
						valid = true
						break
					}
				}
				if !valid {
					return fmt.Errorf("match arm tag %q is invalid for its carrier", arm.Tag)
				}
				if seen[arm.Tag] {
					return fmt.Errorf("duplicate match arm tag %q", arm.Tag)
				}
				seen[arm.Tag] = true
				if payloadTags[arm.Tag] != (arm.Binding != nil) {
					return fmt.Errorf("match arm tag %q has an invalid payload binding", arm.Tag)
				}
			}
			if arm.Body == nil {
				return fmt.Errorf("match arm has no body")
			}
			scoped := parameters
			if arm.Binding != nil {
				var payload Type
				if carrier.Kind == TypeOptional && arm.Tag == "some" {
					payload = carrier.Optional.Value
				} else if carrier.Kind == TypeResult && arm.Tag == "ok" {
					payload = carrier.Result.Success
				} else if carrier.Kind == TypeResult && arm.Tag == "err" {
					payload = carrier.Result.Failure
				} else {
					return fmt.Errorf("match arm binding has no payload")
				}
				if *arm.Binding != len(parameters) {
					return fmt.Errorf("match arm binding position is not canonical")
				}
				scoped = append(append([]Parameter{}, parameters...), Parameter{Position: len(parameters), Name: "match", Type: payload})
			}
			if err := validateExpr(*arm.Body, scoped); err != nil {
				return fmt.Errorf("match arm: %w", err)
			}
			if !TypeEqual(arm.Body.Type, expression.Type) {
				return fmt.Errorf("match arm type does not match expression")
			}
		}
		if !wildcard {
			for _, tag := range tags {
				if !seen[tag] {
					return fmt.Errorf("match is not exhaustive; missing %s arm", tag)
				}
			}
		}
	default:
		return fmt.Errorf("unsupported expression kind %q", expression.Kind)
	}
	return nil
}

func functionContainsBoundedValueResult(function Function) bool {
	if isBoundedValueResultType(function.ReturnType) || exprContainsBoundedValueResult(function.Body) {
		return true
	}
	for _, parameter := range function.Parameters {
		if isBoundedValueResultType(parameter.Type) {
			return true
		}
	}
	return false
}

func exprContainsBoundedValueResult(expression Expr) bool {
	if isBoundedValueResultType(expression.Type) {
		return true
	}
	switch expression.Kind {
	case ExprResultOK, ExprResultErr, ExprResultIsOK, ExprResultSuccessOr, ExprResultFailureOr:
		return true
	case ExprUnary:
		return expression.Unary != nil && expression.Unary.Operand != nil && exprContainsBoundedValueResult(*expression.Unary.Operand)
	case ExprBinary:
		return expression.Binary != nil && expression.Binary.Left != nil && expression.Binary.Right != nil && (exprContainsBoundedValueResult(*expression.Binary.Left) || exprContainsBoundedValueResult(*expression.Binary.Right))
	case ExprFieldProjection:
		return expression.Field != nil && expression.Field.Receiver != nil && exprContainsBoundedValueResult(*expression.Field.Receiver)
	case ExprListAt:
		return expression.ListAt != nil && expression.ListAt.Values != nil && expression.ListAt.Index != nil && (exprContainsBoundedValueResult(*expression.ListAt.Values) || exprContainsBoundedValueResult(*expression.ListAt.Index))
	case ExprListFindByText:
		return expression.ListFind != nil && expression.ListFind.Values != nil && expression.ListFind.Key != nil && (exprContainsBoundedValueResult(*expression.ListFind.Values) || exprContainsBoundedValueResult(*expression.ListFind.Key))
	case ExprListFilterByText:
		return expression.ListFilter != nil && expression.ListFilter.Values != nil && expression.ListFilter.Key != nil && (exprContainsBoundedValueResult(*expression.ListFilter.Values) || exprContainsBoundedValueResult(*expression.ListFilter.Key))
	case ExprListFilterContainsCaseFolded:
		return expression.ListFilterContainsCaseFolded != nil && expression.ListFilterContainsCaseFolded.Values != nil && expression.ListFilterContainsCaseFolded.Query != nil && (exprContainsBoundedValueResult(*expression.ListFilterContainsCaseFolded.Values) || exprContainsBoundedValueResult(*expression.ListFilterContainsCaseFolded.Query))
	case ExprListFilterJoinedContainsCaseFolded:
		return expression.ListFilterJoinedContainsCaseFolded != nil && expression.ListFilterJoinedContainsCaseFolded.Values != nil && expression.ListFilterJoinedContainsCaseFolded.Query != nil && (exprContainsBoundedValueResult(*expression.ListFilterJoinedContainsCaseFolded.Values) || exprContainsBoundedValueResult(*expression.ListFilterJoinedContainsCaseFolded.Query))
	case ExprListSortByOrdinalText:
		return expression.ListSortByOrdinalText != nil && expression.ListSortByOrdinalText.Values != nil && exprContainsBoundedValueResult(*expression.ListSortByOrdinalText.Values)
	case ExprListSortByOrdinalTexts:
		return expression.ListSortByOrdinalTexts != nil && expression.ListSortByOrdinalTexts.Values != nil && exprContainsBoundedValueResult(*expression.ListSortByOrdinalTexts.Values)
	}
	return false
}

func validateDirectBoundedValueResultFunction(function Function) error {
	directParameter := func(expression *Expr, position int) bool {
		return expression != nil && expression.Kind == ExprReference && expression.Parameter != nil && *expression.Parameter == position && position < len(function.Parameters) && TypeEqual(expression.Type, function.Parameters[position].Type)
	}
	switch function.Body.Kind {
	case ExprResultOK:
		propagated := len(function.Parameters) == 1 && function.Body.ResultOK != nil && function.Body.ResultOK.Value != nil && function.Body.ResultOK.Value.Kind == ExprPropagate && function.Body.ResultOK.Value.Propagate != nil && directParameter(function.Body.ResultOK.Value.Propagate.Value, 0) && TypeEqual(function.Parameters[0].Type, function.ReturnType)
		legacy := len(function.Parameters) == 1 && function.Body.ResultOK != nil && TypeEqual(function.Parameters[0].Type, function.ReturnType.Result.Success) && directParameter(function.Body.ResultOK.Value, 0)
		if !isBoundedValueResultType(function.ReturnType) || len(function.Parameters) != 1 || (!legacy && !propagated) {
			return fmt.Errorf("bounded Result ok requires one direct matching success parameter")
		}
	case ExprResultErr:
		if !isBoundedValueResultType(function.ReturnType) || function.Body.ResultErr == nil || len(function.Parameters) != 1 || !TypeEqual(function.Parameters[0].Type, function.ReturnType.Result.Failure) || !directParameter(function.Body.ResultErr.Error, 0) {
			return fmt.Errorf("bounded Result err requires one direct matching failure parameter")
		}
	case ExprReference:
		if !isBoundedValueResultType(function.ReturnType) || len(function.Parameters) != 1 || !TypeEqual(function.Parameters[0].Type, function.ReturnType) || !directParameter(&function.Body, 0) {
			return fmt.Errorf("bounded Result identity requires one identical direct parameter and return")
		}
	case ExprResultIsOK:
		boolean := Type{Kind: TypePrimitive, Primitive: PrimitiveBool}
		if function.Body.ResultIsOK == nil || len(function.Parameters) != 1 || !isBoundedValueResultType(function.Parameters[0].Type) || !TypeEqual(function.ReturnType, boolean) || !directParameter(function.Body.ResultIsOK.Value, 0) {
			return fmt.Errorf("bounded Result is_ok requires one direct Result parameter and bool return")
		}
	case ExprResultSuccessOr:
		if function.Body.SuccessOr == nil || len(function.Parameters) != 2 || !isBoundedValueResultType(function.Parameters[0].Type) || !TypeEqual(function.Parameters[0].Type.Result.Success, function.Parameters[1].Type) || !TypeEqual(function.ReturnType, function.Parameters[1].Type) || !directParameter(function.Body.SuccessOr.Value, 0) || !directParameter(function.Body.SuccessOr.Fallback, 1) {
			return fmt.Errorf("bounded Result success_or requires direct Result and matching success fallback parameters")
		}
	case ExprResultFailureOr:
		if function.Body.FailureOr == nil || len(function.Parameters) != 2 || !isBoundedValueResultType(function.Parameters[0].Type) || !TypeEqual(function.Parameters[0].Type.Result.Failure, function.Parameters[1].Type) || !TypeEqual(function.ReturnType, function.Parameters[1].Type) || !directParameter(function.Body.FailureOr.Value, 0) || !directParameter(function.Body.FailureOr.Fallback, 1) {
			return fmt.Errorf("bounded Result failure_or requires direct Result and matching failure fallback parameters")
		}
	case ExprMatch:
		if function.Body.Match == nil || len(function.Parameters) != 1 || !isBoundedValueResultType(function.Parameters[0].Type) || !directParameter(function.Body.Match.Value, 0) {
			return fmt.Errorf("bounded Result match requires one direct Result parameter")
		}
	default:
		return fmt.Errorf("bounded Result types are admitted only in direct ok, err, identity, is_ok, success_or, or failure_or functions")
	}
	return nil
}

func functionContainsList(function Function) bool {
	if function.ReturnType.Kind == TypeList || exprContainsList(function.Body) {
		return true
	}
	for _, parameter := range function.Parameters {
		if parameter.Type.Kind == TypeList {
			return true
		}
	}
	return false
}

func exprContainsList(expression Expr) bool {
	switch expression.Kind {
	case ExprListEmpty, ExprListSingleton, ExprListCount, ExprListAppend, ExprListAt, ExprListFindByText, ExprListFilterByText, ExprListFilterPredicate, ExprListFilterContainsCaseFolded, ExprListFilterJoinedContainsCaseFolded, ExprListSortByOrdinalText, ExprListSortByOrdinalTexts:
		return true
	case ExprUnary:
		return expression.Unary != nil && expression.Unary.Operand != nil && exprContainsList(*expression.Unary.Operand)
	case ExprBinary:
		return expression.Binary != nil && expression.Binary.Left != nil && expression.Binary.Right != nil && (exprContainsList(*expression.Binary.Left) || exprContainsList(*expression.Binary.Right))
	case ExprFieldProjection:
		return expression.Field != nil && expression.Field.Receiver != nil && exprContainsList(*expression.Field.Receiver)
	case ExprRecordConstruct:
		if expression.Record != nil {
			for _, field := range expression.Record.Fields {
				if field.Value != nil && exprContainsList(*field.Value) {
					return true
				}
			}
		}
	}
	return false
}

func validateDirectListFunction(function Function) error {
	if function.Body.Kind == ExprListCount {
		if function.Body.ListCount == nil || function.Body.ListCount.Value == nil || len(function.Parameters) != 1 || function.Parameters[0].Type.Kind != TypeList || !TypeEqual(function.ReturnType, SignedInteger(64)) {
			return fmt.Errorf("count requires one record-list parameter and a signed 64-bit int return")
		}
		value := function.Body.ListCount.Value
		if value.Kind != ExprReference || value.Parameter == nil || *value.Parameter != 0 || !TypeEqual(value.Type, function.Parameters[0].Type) {
			return fmt.Errorf("count operand must be its sole direct record-list parameter")
		}
		return nil
	}
	if function.ReturnType.Kind != TypeList || function.ReturnType.List == nil {
		return fmt.Errorf("record-list values are admitted only as direct list-returning functions")
	}
	switch function.Body.Kind {
	case ExprListEmpty:
		if function.Body.ListEmpty == nil || len(function.Parameters) != 0 {
			return fmt.Errorf("empty_list requires no parameters and a record-list return")
		}
	case ExprListSingleton:
		if function.Body.ListOne == nil || function.Body.ListOne.Value == nil || len(function.Parameters) != 1 {
			return fmt.Errorf("list singleton requires one record parameter and a matching record-list return")
		}
		value := function.Body.ListOne.Value
		if value.Kind != ExprReference || value.Parameter == nil || *value.Parameter != 0 || !TypeEqual(function.Parameters[0].Type, function.ReturnType.List.Element) || !TypeEqual(value.Type, function.Parameters[0].Type) {
			return fmt.Errorf("list singleton value must be its sole corresponding direct record parameter")
		}
	case ExprReference:
		if len(function.Parameters) != 1 || function.Body.Parameter == nil || *function.Body.Parameter != 0 || !TypeEqual(function.Parameters[0].Type, function.ReturnType) {
			return fmt.Errorf("record-list identity transport requires one identical direct parameter and return")
		}
	case ExprListAppend:
		if function.Body.ListAppend == nil || function.Body.ListAppend.Values == nil || function.Body.ListAppend.Value == nil || len(function.Parameters) != 2 {
			return fmt.Errorf("append requires one record-list parameter, one matching record parameter, and a matching record-list return")
		}
		values := function.Body.ListAppend.Values
		value := function.Body.ListAppend.Value
		if values.Kind != ExprReference || values.Parameter == nil || *values.Parameter != 0 || !TypeEqual(values.Type, function.Parameters[0].Type) || !TypeEqual(function.Parameters[0].Type, function.ReturnType) {
			return fmt.Errorf("append values must be its first direct record-list parameter")
		}
		if value.Kind != ExprReference || value.Parameter == nil || *value.Parameter != 1 || !TypeEqual(value.Type, function.Parameters[1].Type) || !TypeEqual(function.Parameters[1].Type, function.ReturnType.List.Element) {
			return fmt.Errorf("append value must be its second direct matching record parameter")
		}
	default:
		return fmt.Errorf("record-list values are admitted only in direct empty_list, singleton list, identity-transport, or append functions")
	}
	return nil
}

func validateDirectTextContainsCaseFoldedFunction(function Function) error {
	text := Type{Kind: TypePrimitive, Primitive: PrimitiveString}
	boolean := Type{Kind: TypePrimitive, Primitive: PrimitiveBool}
	if function.Body.TextContains == nil || function.Body.TextContains.Value == nil || function.Body.TextContains.Query == nil || len(function.Parameters) != 2 || !TypeEqual(function.Parameters[0].Type, text) || !TypeEqual(function.Parameters[1].Type, text) || !TypeEqual(function.ReturnType, boolean) {
		return fmt.Errorf("contains_casefolded requires two direct string parameters and a bool return")
	}
	value := function.Body.TextContains.Value
	query := function.Body.TextContains.Query
	if value.Kind != ExprReference || value.Parameter == nil || *value.Parameter != 0 || !TypeEqual(value.Type, function.Parameters[0].Type) {
		return fmt.Errorf("contains_casefolded value must be its first direct string parameter")
	}
	if query.Kind != ExprReference || query.Parameter == nil || *query.Parameter != 1 || !TypeEqual(query.Type, function.Parameters[1].Type) {
		return fmt.Errorf("contains_casefolded query must be its second direct string parameter")
	}
	return nil
}

func validateDirectTextTrimFunction(function Function) error {
	text := Type{Kind: TypePrimitive, Primitive: PrimitiveString}
	if function.Body.TextTrim == nil || function.Body.TextTrim.Value == nil || len(function.Parameters) != 1 || !TypeEqual(function.Parameters[0].Type, text) || !TypeEqual(function.ReturnType, text) {
		return fmt.Errorf("trim requires one direct string parameter and a string return")
	}
	value := function.Body.TextTrim.Value
	if value.Kind != ExprReference || value.Parameter == nil || *value.Parameter != 0 || !TypeEqual(value.Type, function.Parameters[0].Type) {
		return fmt.Errorf("trim value must be its sole direct string parameter")
	}
	return nil
}

func exprContainsTextCaseFolded(expression Expr) bool {
	if expression.Kind == ExprTextContainsCaseFolded {
		return true
	}
	children := []*Expr{}
	switch expression.Kind {
	case ExprUnary:
		if expression.Unary != nil {
			children = append(children, expression.Unary.Operand)
		}
	case ExprBinary:
		if expression.Binary != nil {
			children = append(children, expression.Binary.Left, expression.Binary.Right)
		}
	case ExprTextTrim:
		if expression.TextTrim != nil {
			children = append(children, expression.TextTrim.Value)
		}
	case ExprFieldProjection:
		if expression.Field != nil {
			children = append(children, expression.Field.Receiver)
		}
	case ExprRecordConstruct:
		if expression.Record != nil {
			for _, field := range expression.Record.Fields {
				children = append(children, field.Value)
			}
		}
	case ExprOptionalSome:
		if expression.Some != nil {
			children = append(children, expression.Some.Value)
		}
	case ExprOptionalHasValue:
		if expression.HasValue != nil {
			children = append(children, expression.HasValue.Value)
		}
	case ExprOptionalValueOr:
		if expression.ValueOr != nil {
			children = append(children, expression.ValueOr.Value, expression.ValueOr.Fallback)
		}
	case ExprListSingleton:
		if expression.ListOne != nil {
			children = append(children, expression.ListOne.Value)
		}
	case ExprListCount:
		if expression.ListCount != nil {
			children = append(children, expression.ListCount.Value)
		}
	case ExprListAppend:
		if expression.ListAppend != nil {
			children = append(children, expression.ListAppend.Values, expression.ListAppend.Value)
		}
	case ExprListAt:
		if expression.ListAt != nil {
			children = append(children, expression.ListAt.Values, expression.ListAt.Index)
		}
	case ExprListFindByText:
		if expression.ListFind != nil {
			children = append(children, expression.ListFind.Values, expression.ListFind.Key)
		}
	case ExprListFilterByText:
		if expression.ListFilter != nil {
			children = append(children, expression.ListFilter.Values, expression.ListFilter.Key)
		}
	case ExprListFilterContainsCaseFolded:
		if expression.ListFilterContainsCaseFolded != nil {
			children = append(children, expression.ListFilterContainsCaseFolded.Values, expression.ListFilterContainsCaseFolded.Query)
		}
	case ExprListFilterJoinedContainsCaseFolded:
		if expression.ListFilterJoinedContainsCaseFolded != nil {
			children = append(children, expression.ListFilterJoinedContainsCaseFolded.Values, expression.ListFilterJoinedContainsCaseFolded.Query)
		}
	case ExprResultOK:
		if expression.ResultOK != nil {
			children = append(children, expression.ResultOK.Value)
		}
	case ExprResultErr:
		if expression.ResultErr != nil {
			children = append(children, expression.ResultErr.Error)
		}
	case ExprResultIsOK:
		if expression.ResultIsOK != nil {
			children = append(children, expression.ResultIsOK.Value)
		}
	case ExprResultSuccessOr:
		if expression.SuccessOr != nil {
			children = append(children, expression.SuccessOr.Value, expression.SuccessOr.Fallback)
		}
	case ExprResultFailureOr:
		if expression.FailureOr != nil {
			children = append(children, expression.FailureOr.Value, expression.FailureOr.Fallback)
		}
	}
	for _, child := range children {
		if child != nil && exprContainsTextCaseFolded(*child) {
			return true
		}
	}
	return false
}

func exprContainsTextTrim(expression Expr) bool {
	if expression.Kind == ExprTextTrim {
		return true
	}
	children := []*Expr{}
	switch expression.Kind {
	case ExprUnary:
		if expression.Unary != nil {
			children = append(children, expression.Unary.Operand)
		}
	case ExprBinary:
		if expression.Binary != nil {
			children = append(children, expression.Binary.Left, expression.Binary.Right)
		}
	case ExprTextContainsCaseFolded:
		if expression.TextContains != nil {
			children = append(children, expression.TextContains.Value, expression.TextContains.Query)
		}
	case ExprFieldProjection:
		if expression.Field != nil {
			children = append(children, expression.Field.Receiver)
		}
	case ExprRecordConstruct:
		if expression.Record != nil {
			for _, field := range expression.Record.Fields {
				children = append(children, field.Value)
			}
		}
	case ExprOptionalSome:
		if expression.Some != nil {
			children = append(children, expression.Some.Value)
		}
	case ExprOptionalHasValue:
		if expression.HasValue != nil {
			children = append(children, expression.HasValue.Value)
		}
	case ExprOptionalValueOr:
		if expression.ValueOr != nil {
			children = append(children, expression.ValueOr.Value, expression.ValueOr.Fallback)
		}
	case ExprListSingleton:
		if expression.ListOne != nil {
			children = append(children, expression.ListOne.Value)
		}
	case ExprListCount:
		if expression.ListCount != nil {
			children = append(children, expression.ListCount.Value)
		}
	case ExprListAppend:
		if expression.ListAppend != nil {
			children = append(children, expression.ListAppend.Values, expression.ListAppend.Value)
		}
	case ExprListAt:
		if expression.ListAt != nil {
			children = append(children, expression.ListAt.Values, expression.ListAt.Index)
		}
	case ExprListFindByText:
		if expression.ListFind != nil {
			children = append(children, expression.ListFind.Values, expression.ListFind.Key)
		}
	case ExprListFilterByText:
		if expression.ListFilter != nil {
			children = append(children, expression.ListFilter.Values, expression.ListFilter.Key)
		}
	case ExprListFilterContainsCaseFolded:
		if expression.ListFilterContainsCaseFolded != nil {
			children = append(children, expression.ListFilterContainsCaseFolded.Values, expression.ListFilterContainsCaseFolded.Query)
		}
	case ExprListFilterJoinedContainsCaseFolded:
		if expression.ListFilterJoinedContainsCaseFolded != nil {
			children = append(children, expression.ListFilterJoinedContainsCaseFolded.Values, expression.ListFilterJoinedContainsCaseFolded.Query)
		}
	case ExprResultOK:
		if expression.ResultOK != nil {
			children = append(children, expression.ResultOK.Value)
		}
	case ExprResultErr:
		if expression.ResultErr != nil {
			children = append(children, expression.ResultErr.Error)
		}
	case ExprResultIsOK:
		if expression.ResultIsOK != nil {
			children = append(children, expression.ResultIsOK.Value)
		}
	case ExprResultSuccessOr:
		if expression.SuccessOr != nil {
			children = append(children, expression.SuccessOr.Value, expression.SuccessOr.Fallback)
		}
	case ExprResultFailureOr:
		if expression.FailureOr != nil {
			children = append(children, expression.FailureOr.Value, expression.FailureOr.Fallback)
		}
	}
	for _, child := range children {
		if child != nil && exprContainsTextTrim(*child) {
			return true
		}
	}
	return false
}

func exprContainsListFilterContainsCaseFolded(expression Expr) bool {
	if expression.Kind == ExprListFilterContainsCaseFolded {
		return true
	}
	children := []*Expr{}
	switch expression.Kind {
	case ExprUnary:
		if expression.Unary != nil {
			children = append(children, expression.Unary.Operand)
		}
	case ExprBinary:
		if expression.Binary != nil {
			children = append(children, expression.Binary.Left, expression.Binary.Right)
		}
	case ExprFieldProjection:
		if expression.Field != nil {
			children = append(children, expression.Field.Receiver)
		}
	case ExprRecordConstruct:
		if expression.Record != nil {
			for _, field := range expression.Record.Fields {
				children = append(children, field.Value)
			}
		}
	case ExprOptionalSome:
		if expression.Some != nil {
			children = append(children, expression.Some.Value)
		}
	case ExprOptionalHasValue:
		if expression.HasValue != nil {
			children = append(children, expression.HasValue.Value)
		}
	case ExprOptionalValueOr:
		if expression.ValueOr != nil {
			children = append(children, expression.ValueOr.Value, expression.ValueOr.Fallback)
		}
	case ExprListSingleton:
		if expression.ListOne != nil {
			children = append(children, expression.ListOne.Value)
		}
	case ExprListCount:
		if expression.ListCount != nil {
			children = append(children, expression.ListCount.Value)
		}
	case ExprListAppend:
		if expression.ListAppend != nil {
			children = append(children, expression.ListAppend.Values, expression.ListAppend.Value)
		}
	case ExprListAt:
		if expression.ListAt != nil {
			children = append(children, expression.ListAt.Values, expression.ListAt.Index)
		}
	case ExprListFindByText:
		if expression.ListFind != nil {
			children = append(children, expression.ListFind.Values, expression.ListFind.Key)
		}
	case ExprListFilterByText:
		if expression.ListFilter != nil {
			children = append(children, expression.ListFilter.Values, expression.ListFilter.Key)
		}
	case ExprResultOK:
		if expression.ResultOK != nil {
			children = append(children, expression.ResultOK.Value)
		}
	case ExprResultErr:
		if expression.ResultErr != nil {
			children = append(children, expression.ResultErr.Error)
		}
	case ExprResultIsOK:
		if expression.ResultIsOK != nil {
			children = append(children, expression.ResultIsOK.Value)
		}
	case ExprResultSuccessOr:
		if expression.SuccessOr != nil {
			children = append(children, expression.SuccessOr.Value, expression.SuccessOr.Fallback)
		}
	case ExprResultFailureOr:
		if expression.FailureOr != nil {
			children = append(children, expression.FailureOr.Value, expression.FailureOr.Fallback)
		}
	}
	for _, child := range children {
		if child != nil && exprContainsListFilterContainsCaseFolded(*child) {
			return true
		}
	}
	return false
}

func exprContainsListFilterJoinedContainsCaseFolded(expression Expr) bool {
	if expression.Kind == ExprListFilterJoinedContainsCaseFolded {
		return true
	}
	children := []*Expr{}
	switch expression.Kind {
	case ExprUnary:
		if expression.Unary != nil {
			children = append(children, expression.Unary.Operand)
		}
	case ExprBinary:
		if expression.Binary != nil {
			children = append(children, expression.Binary.Left, expression.Binary.Right)
		}
	case ExprTextContainsCaseFolded:
		if expression.TextContains != nil {
			children = append(children, expression.TextContains.Value, expression.TextContains.Query)
		}
	case ExprTextTrim:
		if expression.TextTrim != nil {
			children = append(children, expression.TextTrim.Value)
		}
	case ExprFieldProjection:
		if expression.Field != nil {
			children = append(children, expression.Field.Receiver)
		}
	case ExprRecordConstruct:
		if expression.Record != nil {
			for _, field := range expression.Record.Fields {
				children = append(children, field.Value)
			}
		}
	case ExprOptionalSome:
		if expression.Some != nil {
			children = append(children, expression.Some.Value)
		}
	case ExprOptionalHasValue:
		if expression.HasValue != nil {
			children = append(children, expression.HasValue.Value)
		}
	case ExprOptionalValueOr:
		if expression.ValueOr != nil {
			children = append(children, expression.ValueOr.Value, expression.ValueOr.Fallback)
		}
	case ExprListSingleton:
		if expression.ListOne != nil {
			children = append(children, expression.ListOne.Value)
		}
	case ExprListCount:
		if expression.ListCount != nil {
			children = append(children, expression.ListCount.Value)
		}
	case ExprListAppend:
		if expression.ListAppend != nil {
			children = append(children, expression.ListAppend.Values, expression.ListAppend.Value)
		}
	case ExprListAt:
		if expression.ListAt != nil {
			children = append(children, expression.ListAt.Values, expression.ListAt.Index)
		}
	case ExprListFindByText:
		if expression.ListFind != nil {
			children = append(children, expression.ListFind.Values, expression.ListFind.Key)
		}
	case ExprListFilterByText:
		if expression.ListFilter != nil {
			children = append(children, expression.ListFilter.Values, expression.ListFilter.Key)
		}
	case ExprListFilterContainsCaseFolded:
		if expression.ListFilterContainsCaseFolded != nil {
			children = append(children, expression.ListFilterContainsCaseFolded.Values, expression.ListFilterContainsCaseFolded.Query)
		}
	case ExprResultOK:
		if expression.ResultOK != nil {
			children = append(children, expression.ResultOK.Value)
		}
	case ExprResultErr:
		if expression.ResultErr != nil {
			children = append(children, expression.ResultErr.Error)
		}
	case ExprResultIsOK:
		if expression.ResultIsOK != nil {
			children = append(children, expression.ResultIsOK.Value)
		}
	case ExprResultSuccessOr:
		if expression.SuccessOr != nil {
			children = append(children, expression.SuccessOr.Value, expression.SuccessOr.Fallback)
		}
	case ExprResultFailureOr:
		if expression.FailureOr != nil {
			children = append(children, expression.FailureOr.Value, expression.FailureOr.Fallback)
		}
	}
	for _, child := range children {
		if child != nil && exprContainsListFilterJoinedContainsCaseFolded(*child) {
			return true
		}
	}
	return false
}

func exprContainsListSortByOrdinalText(expression Expr) bool {
	if expression.Kind == ExprListSortByOrdinalText || expression.Kind == ExprListSortByOrdinalTexts {
		return true
	}
	children := []*Expr{}
	switch expression.Kind {
	case ExprUnary:
		if expression.Unary != nil {
			children = append(children, expression.Unary.Operand)
		}
	case ExprBinary:
		if expression.Binary != nil {
			children = append(children, expression.Binary.Left, expression.Binary.Right)
		}
	case ExprTextContainsCaseFolded:
		if expression.TextContains != nil {
			children = append(children, expression.TextContains.Value, expression.TextContains.Query)
		}
	case ExprTextTrim:
		if expression.TextTrim != nil {
			children = append(children, expression.TextTrim.Value)
		}
	case ExprFieldProjection:
		if expression.Field != nil {
			children = append(children, expression.Field.Receiver)
		}
	case ExprRecordConstruct:
		if expression.Record != nil {
			for _, field := range expression.Record.Fields {
				children = append(children, field.Value)
			}
		}
	case ExprOptionalSome:
		if expression.Some != nil {
			children = append(children, expression.Some.Value)
		}
	case ExprOptionalHasValue:
		if expression.HasValue != nil {
			children = append(children, expression.HasValue.Value)
		}
	case ExprOptionalValueOr:
		if expression.ValueOr != nil {
			children = append(children, expression.ValueOr.Value, expression.ValueOr.Fallback)
		}
	case ExprListSingleton:
		if expression.ListOne != nil {
			children = append(children, expression.ListOne.Value)
		}
	case ExprListCount:
		if expression.ListCount != nil {
			children = append(children, expression.ListCount.Value)
		}
	case ExprListAppend:
		if expression.ListAppend != nil {
			children = append(children, expression.ListAppend.Values, expression.ListAppend.Value)
		}
	case ExprListAt:
		if expression.ListAt != nil {
			children = append(children, expression.ListAt.Values, expression.ListAt.Index)
		}
	case ExprListFindByText:
		if expression.ListFind != nil {
			children = append(children, expression.ListFind.Values, expression.ListFind.Key)
		}
	case ExprListFilterByText:
		if expression.ListFilter != nil {
			children = append(children, expression.ListFilter.Values, expression.ListFilter.Key)
		}
	case ExprListFilterContainsCaseFolded:
		if expression.ListFilterContainsCaseFolded != nil {
			children = append(children, expression.ListFilterContainsCaseFolded.Values, expression.ListFilterContainsCaseFolded.Query)
		}
	case ExprListFilterJoinedContainsCaseFolded:
		if expression.ListFilterJoinedContainsCaseFolded != nil {
			children = append(children, expression.ListFilterJoinedContainsCaseFolded.Values, expression.ListFilterJoinedContainsCaseFolded.Query)
		}
	case ExprResultOK:
		if expression.ResultOK != nil {
			children = append(children, expression.ResultOK.Value)
		}
	case ExprResultErr:
		if expression.ResultErr != nil {
			children = append(children, expression.ResultErr.Error)
		}
	case ExprResultIsOK:
		if expression.ResultIsOK != nil {
			children = append(children, expression.ResultIsOK.Value)
		}
	case ExprResultSuccessOr:
		if expression.SuccessOr != nil {
			children = append(children, expression.SuccessOr.Value, expression.SuccessOr.Fallback)
		}
	case ExprResultFailureOr:
		if expression.FailureOr != nil {
			children = append(children, expression.FailureOr.Value, expression.FailureOr.Fallback)
		}
	}
	for _, child := range children {
		if child != nil && exprContainsListSortByOrdinalText(*child) {
			return true
		}
	}
	return false
}

func validateDirectListAtFunction(function Function) error {
	if function.Body.ListAt == nil || function.Body.ListAt.Values == nil || function.Body.ListAt.Index == nil || len(function.Parameters) != 2 || function.Parameters[0].Type.Kind != TypeList || function.Parameters[0].Type.List == nil || !TypeEqual(function.Parameters[1].Type, SignedInteger(64)) || function.ReturnType.Kind != TypeOptional || function.ReturnType.Optional == nil || !TypeEqual(function.ReturnType.Optional.Value, function.Parameters[0].Type.List.Element) {
		return fmt.Errorf("at requires direct List<R> and signed 64-bit int parameters with Optional<R> return")
	}
	values := function.Body.ListAt.Values
	index := function.Body.ListAt.Index
	if values.Kind != ExprReference || values.Parameter == nil || *values.Parameter != 0 || !TypeEqual(values.Type, function.Parameters[0].Type) {
		return fmt.Errorf("at values must be its first direct record-list parameter")
	}
	if index.Kind != ExprReference || index.Parameter == nil || *index.Parameter != 1 || !TypeEqual(index.Type, function.Parameters[1].Type) {
		return fmt.Errorf("at index must be its second direct signed 64-bit parameter")
	}
	return nil
}

func validateDirectListFindByTextFunction(function Function) error {
	if function.Body.ListFind == nil || function.Body.ListFind.Values == nil || function.Body.ListFind.Key == nil || len(function.Parameters) != 2 || function.Parameters[0].Type.Kind != TypeList || function.Parameters[0].Type.List == nil || function.Parameters[0].Type.List.Element.Kind != TypeRecord || !TypeEqual(function.Parameters[1].Type, Type{Kind: TypePrimitive, Primitive: PrimitiveString}) || function.ReturnType.Kind != TypeOptional || function.ReturnType.Optional == nil || !TypeEqual(function.ReturnType.Optional.Value, function.Parameters[0].Type.List.Element) {
		return fmt.Errorf("find_by requires direct List<R> and string parameters with Optional<R> return")
	}
	values := function.Body.ListFind.Values
	key := function.Body.ListFind.Key
	if values.Kind != ExprReference || values.Parameter == nil || *values.Parameter != 0 || !TypeEqual(values.Type, function.Parameters[0].Type) {
		return fmt.Errorf("find_by values must be its first direct record-list parameter")
	}
	if key.Kind != ExprReference || key.Parameter == nil || *key.Parameter != 1 || !TypeEqual(key.Type, function.Parameters[1].Type) {
		return fmt.Errorf("find_by key must be its second direct string parameter")
	}
	return nil
}

func validateDirectListFilterByTextFunction(function Function) error {
	if function.Body.ListFilter == nil || function.Body.ListFilter.Values == nil || function.Body.ListFilter.Key == nil || len(function.Parameters) != 2 || function.Parameters[0].Type.Kind != TypeList || function.Parameters[0].Type.List == nil || function.Parameters[0].Type.List.Element.Kind != TypeRecord || !TypeEqual(function.Parameters[1].Type, Type{Kind: TypePrimitive, Primitive: PrimitiveString}) || !TypeEqual(function.ReturnType, function.Parameters[0].Type) {
		return fmt.Errorf("filter_by requires direct List<R> and string parameters with matching List<R> return")
	}
	values := function.Body.ListFilter.Values
	key := function.Body.ListFilter.Key
	if values.Kind != ExprReference || values.Parameter == nil || *values.Parameter != 0 || !TypeEqual(values.Type, function.Parameters[0].Type) {
		return fmt.Errorf("filter_by values must be its first direct record-list parameter")
	}
	if key.Kind != ExprReference || key.Parameter == nil || *key.Parameter != 1 || !TypeEqual(key.Type, function.Parameters[1].Type) {
		return fmt.Errorf("filter_by key must be its second direct string parameter")
	}
	return nil
}

func validateDirectListFilterContainsCaseFoldedFunction(function Function) error {
	filter := function.Body.ListFilterContainsCaseFolded
	if filter == nil || filter.Values == nil || filter.Query == nil || len(function.Parameters) != 2 || function.Parameters[0].Type.Kind != TypeList || function.Parameters[0].Type.List == nil || function.Parameters[0].Type.List.Element.Kind != TypeRecord || !TypeEqual(function.Parameters[1].Type, Type{Kind: TypePrimitive, Primitive: PrimitiveString}) || !TypeEqual(function.ReturnType, function.Parameters[0].Type) {
		return fmt.Errorf("filter_contains_casefolded requires direct List<R> and string parameters with matching List<R> return")
	}
	values := filter.Values
	query := filter.Query
	if values.Kind != ExprReference || values.Parameter == nil || *values.Parameter != 0 || !TypeEqual(values.Type, function.Parameters[0].Type) {
		return fmt.Errorf("filter_contains_casefolded values must be its first direct record-list parameter")
	}
	if query.Kind != ExprReference || query.Parameter == nil || *query.Parameter != 1 || !TypeEqual(query.Type, function.Parameters[1].Type) {
		return fmt.Errorf("filter_contains_casefolded query must be its second direct string parameter")
	}
	return nil
}

func validateDirectListFilterJoinedContainsCaseFoldedFunction(function Function) error {
	filter := function.Body.ListFilterJoinedContainsCaseFolded
	stringType := Type{Kind: TypePrimitive, Primitive: PrimitiveString}
	if filter == nil || filter.Values == nil || filter.Query == nil || len(filter.Selectors) < 2 || len(function.Parameters) != 2 || function.Parameters[0].Type.Kind != TypeList || function.Parameters[0].Type.List == nil || function.Parameters[0].Type.List.Element.Kind != TypeRecord || !TypeEqual(function.Parameters[1].Type, stringType) || !TypeEqual(function.ReturnType, function.Parameters[0].Type) {
		return fmt.Errorf("filter_joined_contains_casefolded requires direct List<R> and string parameters, at least two selectors, and matching List<R> return")
	}
	if filter.Values.Kind != ExprReference || filter.Values.Parameter == nil || *filter.Values.Parameter != 0 || !TypeEqual(filter.Values.Type, function.Parameters[0].Type) {
		return fmt.Errorf("filter_joined_contains_casefolded values must be its first direct record-list parameter")
	}
	if filter.Query.Kind != ExprReference || filter.Query.Parameter == nil || *filter.Query.Parameter != 1 || !TypeEqual(filter.Query.Type, function.Parameters[1].Type) {
		return fmt.Errorf("filter_joined_contains_casefolded query must be its second direct string parameter")
	}
	return nil
}

func validateDirectListSortByOrdinalTextFunction(function Function) error {
	sorted := function.Body.ListSortByOrdinalText
	if sorted == nil || sorted.Values == nil || len(function.Parameters) != 1 || function.Parameters[0].Type.Kind != TypeList || function.Parameters[0].Type.List == nil || function.Parameters[0].Type.List.Element.Kind != TypeRecord || !TypeEqual(function.ReturnType, function.Parameters[0].Type) {
		return fmt.Errorf("sort_by_ordinal requires one direct List<R> parameter with matching List<R> return")
	}
	if sorted.Values.Kind != ExprReference || sorted.Values.Parameter == nil || *sorted.Values.Parameter != 0 || !TypeEqual(sorted.Values.Type, function.Parameters[0].Type) {
		return fmt.Errorf("sort_by_ordinal values must be its sole direct record-list parameter")
	}
	return nil
}

func validateDirectListSortByOrdinalTextsFunction(function Function) error {
	sorted := function.Body.ListSortByOrdinalTexts
	if sorted == nil || sorted.Values == nil || len(sorted.Selectors) < 2 || len(function.Parameters) != 1 || function.Parameters[0].Type.Kind != TypeList || function.Parameters[0].Type.List == nil || function.Parameters[0].Type.List.Element.Kind != TypeRecord || !TypeEqual(function.ReturnType, function.Parameters[0].Type) {
		return fmt.Errorf("multi-key sort_by_ordinal requires one direct List<R> parameter, at least two selectors, and matching List<R> return")
	}
	if sorted.Values.Kind != ExprReference || sorted.Values.Parameter == nil || *sorted.Values.Parameter != 0 || !TypeEqual(sorted.Values.Type, function.Parameters[0].Type) {
		return fmt.Errorf("multi-key sort_by_ordinal values must be its sole direct record-list parameter")
	}
	return nil
}

func validateDirectListSortByOrdinalDirectionsFunction(function Function) error {
	sorted := function.Body.ListSortByOrdinalDirections
	if sorted == nil || sorted.Values == nil || len(sorted.Selectors) < 1 || len(function.Parameters) != 1 || function.Parameters[0].Type.Kind != TypeList || !TypeEqual(function.ReturnType, function.Parameters[0].Type) {
		return fmt.Errorf("directional sort_by_ordinal requires one direct List<R> parameter, selector/direction pairs, and matching List<R> return")
	}
	if sorted.Values.Kind != ExprReference || sorted.Values.Parameter == nil || *sorted.Values.Parameter != 0 {
		return fmt.Errorf("directional sort_by_ordinal values must be its sole direct record-list parameter")
	}
	return nil
}

func functionContainsOptional(function Function) bool {
	if function.ReturnType.Kind == TypeOptional || exprContainsOptional(function.Body) {
		return true
	}
	for _, parameter := range function.Parameters {
		if parameter.Type.Kind == TypeOptional {
			return true
		}
	}
	return false
}

func exprContainsOptional(expression Expr) bool {
	switch expression.Kind {
	case ExprOptionalSome, ExprOptionalNone, ExprOptionalHasValue, ExprOptionalValueOr:
		return true
	case ExprUnary:
		return expression.Unary != nil && expression.Unary.Operand != nil && exprContainsOptional(*expression.Unary.Operand)
	case ExprBinary:
		return expression.Binary != nil && expression.Binary.Left != nil && expression.Binary.Right != nil && (exprContainsOptional(*expression.Binary.Left) || exprContainsOptional(*expression.Binary.Right))
	case ExprFieldProjection:
		return expression.Field != nil && expression.Field.Receiver != nil && exprContainsOptional(*expression.Field.Receiver)
	case ExprRecordConstruct:
		if expression.Record != nil {
			for _, field := range expression.Record.Fields {
				if field.Value != nil && exprContainsOptional(*field.Value) {
					return true
				}
			}
		}
	case ExprListSingleton:
		return expression.ListOne != nil && expression.ListOne.Value != nil && exprContainsOptional(*expression.ListOne.Value)
	case ExprListCount:
		return expression.ListCount != nil && expression.ListCount.Value != nil && exprContainsOptional(*expression.ListCount.Value)
	case ExprListAppend:
		return expression.ListAppend != nil && expression.ListAppend.Values != nil && expression.ListAppend.Value != nil && (exprContainsOptional(*expression.ListAppend.Values) || exprContainsOptional(*expression.ListAppend.Value))
	case ExprListAt:
		return true
	case ExprListFindByText:
		return true
	case ExprListFilterByText:
		return expression.ListFilter != nil && expression.ListFilter.Values != nil && expression.ListFilter.Key != nil && (exprContainsOptional(*expression.ListFilter.Values) || exprContainsOptional(*expression.ListFilter.Key))
	case ExprListFilterContainsCaseFolded:
		return expression.ListFilterContainsCaseFolded != nil && expression.ListFilterContainsCaseFolded.Values != nil && expression.ListFilterContainsCaseFolded.Query != nil && (exprContainsOptional(*expression.ListFilterContainsCaseFolded.Values) || exprContainsOptional(*expression.ListFilterContainsCaseFolded.Query))
	case ExprListFilterJoinedContainsCaseFolded:
		return expression.ListFilterJoinedContainsCaseFolded != nil && expression.ListFilterJoinedContainsCaseFolded.Values != nil && expression.ListFilterJoinedContainsCaseFolded.Query != nil && (exprContainsOptional(*expression.ListFilterJoinedContainsCaseFolded.Values) || exprContainsOptional(*expression.ListFilterJoinedContainsCaseFolded.Query))
	case ExprListSortByOrdinalText:
		return expression.ListSortByOrdinalText != nil && expression.ListSortByOrdinalText.Values != nil && exprContainsOptional(*expression.ListSortByOrdinalText.Values)
	case ExprListSortByOrdinalTexts:
		return expression.ListSortByOrdinalTexts != nil && expression.ListSortByOrdinalTexts.Values != nil && exprContainsOptional(*expression.ListSortByOrdinalTexts.Values)
	}
	return false
}

func validateDirectOptionalFunction(function Function) error {
	boolean := Type{Kind: TypePrimitive, Primitive: PrimitiveBool}
	switch function.Body.Kind {
	case ExprOptionalSome:
		if function.Body.Some == nil || function.Body.Some.Value == nil || function.ReturnType.Kind != TypeOptional || function.ReturnType.Optional == nil || len(function.Parameters) != 1 {
			return fmt.Errorf("optional some requires one matching value parameter and an Optional return")
		}
		value := function.Body.Some.Value
		propagated := value.Kind == ExprPropagate && value.Propagate != nil && value.Propagate.Value != nil && value.Propagate.Value.Kind == ExprReference && value.Propagate.Value.Parameter != nil && *value.Propagate.Value.Parameter == 0 && TypeEqual(function.Parameters[0].Type, function.ReturnType)
		legacy := value.Kind == ExprReference && value.Parameter != nil && *value.Parameter == 0 && TypeEqual(function.Parameters[0].Type, function.ReturnType.Optional.Value)
		if !legacy && !propagated {
			return fmt.Errorf("optional some value must be its sole corresponding direct parameter")
		}
	case ExprOptionalNone:
		if function.Body.None == nil || function.ReturnType.Kind != TypeOptional || len(function.Parameters) != 0 {
			return fmt.Errorf("optional none requires no parameters and an Optional return")
		}
	case ExprReference:
		if function.ReturnType.Kind != TypeOptional || len(function.Parameters) != 1 || function.Body.Parameter == nil || *function.Body.Parameter != 0 || !TypeEqual(function.Parameters[0].Type, function.ReturnType) {
			return fmt.Errorf("optional identity transport requires one identical direct parameter and return")
		}
	case ExprOptionalHasValue:
		if function.Body.HasValue == nil || function.Body.HasValue.Value == nil || len(function.Parameters) != 1 || function.Parameters[0].Type.Kind != TypeOptional || !TypeEqual(function.ReturnType, boolean) {
			return fmt.Errorf("optional has_value requires one Optional parameter and a bool return")
		}
		value := function.Body.HasValue.Value
		if value.Kind != ExprReference || value.Parameter == nil || *value.Parameter != 0 || !TypeEqual(value.Type, function.Parameters[0].Type) {
			return fmt.Errorf("optional has_value operand must be its sole direct parameter")
		}
	case ExprOptionalValueOr:
		if function.Body.ValueOr == nil || function.Body.ValueOr.Value == nil || function.Body.ValueOr.Fallback == nil || len(function.Parameters) != 2 || function.Parameters[0].Type.Kind != TypeOptional || function.Parameters[0].Type.Optional == nil {
			return fmt.Errorf("optional value_or requires one Optional parameter, one matching fallback parameter, and a matching return")
		}
		value := function.Body.ValueOr.Value
		fallback := function.Body.ValueOr.Fallback
		if value.Kind != ExprReference || value.Parameter == nil || *value.Parameter != 0 || !TypeEqual(value.Type, function.Parameters[0].Type) {
			return fmt.Errorf("optional value_or operand must be its first direct parameter")
		}
		if fallback.Kind != ExprReference || fallback.Parameter == nil || *fallback.Parameter != 1 || !TypeEqual(fallback.Type, function.Parameters[1].Type) {
			return fmt.Errorf("optional value_or fallback must be its second direct parameter")
		}
		if !TypeEqual(function.Parameters[0].Type.Optional.Value, function.Parameters[1].Type) || !TypeEqual(function.ReturnType, function.Parameters[1].Type) {
			return fmt.Errorf("optional value_or payload, fallback, and return types must match")
		}
	case ExprMatch:
		if function.Body.Match == nil || function.Body.Match.Value == nil || len(function.Parameters) != 1 || function.Body.Match.Value.Kind != ExprReference || function.Body.Match.Value.Parameter == nil || *function.Body.Match.Value.Parameter != 0 {
			return fmt.Errorf("match requires one direct tagged parameter")
		}
	default:
		return fmt.Errorf("Optional types are admitted only in direct some, none, identity transport, has_value, or value_or functions")
	}
	return nil
}

func exprContainsRecordConstruction(expression Expr) bool {
	switch expression.Kind {
	case ExprRecordConstruct:
		return true
	case ExprUnary:
		return expression.Unary != nil && expression.Unary.Operand != nil && exprContainsRecordConstruction(*expression.Unary.Operand)
	case ExprBinary:
		return expression.Binary != nil && expression.Binary.Left != nil && expression.Binary.Right != nil && (exprContainsRecordConstruction(*expression.Binary.Left) || exprContainsRecordConstruction(*expression.Binary.Right))
	case ExprFieldProjection:
		return expression.Field != nil && expression.Field.Receiver != nil && exprContainsRecordConstruction(*expression.Field.Receiver)
	case ExprListSingleton:
		return expression.ListOne != nil && expression.ListOne.Value != nil && exprContainsRecordConstruction(*expression.ListOne.Value)
	case ExprListCount:
		return expression.ListCount != nil && expression.ListCount.Value != nil && exprContainsRecordConstruction(*expression.ListCount.Value)
	case ExprListAppend:
		return expression.ListAppend != nil && expression.ListAppend.Values != nil && expression.ListAppend.Value != nil && (exprContainsRecordConstruction(*expression.ListAppend.Values) || exprContainsRecordConstruction(*expression.ListAppend.Value))
	case ExprListAt:
		return expression.ListAt != nil && expression.ListAt.Values != nil && expression.ListAt.Index != nil && (exprContainsRecordConstruction(*expression.ListAt.Values) || exprContainsRecordConstruction(*expression.ListAt.Index))
	case ExprListFindByText:
		return expression.ListFind != nil && expression.ListFind.Values != nil && expression.ListFind.Key != nil && (exprContainsRecordConstruction(*expression.ListFind.Values) || exprContainsRecordConstruction(*expression.ListFind.Key))
	case ExprListFilterByText:
		return expression.ListFilter != nil && expression.ListFilter.Values != nil && expression.ListFilter.Key != nil && (exprContainsRecordConstruction(*expression.ListFilter.Values) || exprContainsRecordConstruction(*expression.ListFilter.Key))
	case ExprListFilterContainsCaseFolded:
		return expression.ListFilterContainsCaseFolded != nil && expression.ListFilterContainsCaseFolded.Values != nil && expression.ListFilterContainsCaseFolded.Query != nil && (exprContainsRecordConstruction(*expression.ListFilterContainsCaseFolded.Values) || exprContainsRecordConstruction(*expression.ListFilterContainsCaseFolded.Query))
	case ExprListFilterJoinedContainsCaseFolded:
		return expression.ListFilterJoinedContainsCaseFolded != nil && expression.ListFilterJoinedContainsCaseFolded.Values != nil && expression.ListFilterJoinedContainsCaseFolded.Query != nil && (exprContainsRecordConstruction(*expression.ListFilterJoinedContainsCaseFolded.Values) || exprContainsRecordConstruction(*expression.ListFilterJoinedContainsCaseFolded.Query))
	case ExprListSortByOrdinalText:
		return expression.ListSortByOrdinalText != nil && expression.ListSortByOrdinalText.Values != nil && exprContainsRecordConstruction(*expression.ListSortByOrdinalText.Values)
	case ExprListSortByOrdinalTexts:
		return expression.ListSortByOrdinalTexts != nil && expression.ListSortByOrdinalTexts.Values != nil && exprContainsRecordConstruction(*expression.ListSortByOrdinalTexts.Values)
	default:
		return false
	}
}

func exprContainsRecordEquality(expression Expr) bool {
	switch expression.Kind {
	case ExprBinary:
		if expression.Binary == nil || expression.Binary.Left == nil || expression.Binary.Right == nil {
			return false
		}
		if expression.Binary.Left.Type.Kind == TypeRecord || expression.Binary.Right.Type.Kind == TypeRecord {
			return true
		}
		return exprContainsRecordEquality(*expression.Binary.Left) || exprContainsRecordEquality(*expression.Binary.Right)
	case ExprUnary:
		return expression.Unary != nil && expression.Unary.Operand != nil && exprContainsRecordEquality(*expression.Unary.Operand)
	case ExprFieldProjection:
		return expression.Field != nil && expression.Field.Receiver != nil && exprContainsRecordEquality(*expression.Field.Receiver)
	case ExprListCount:
		return expression.ListCount != nil && expression.ListCount.Value != nil && exprContainsRecordEquality(*expression.ListCount.Value)
	case ExprListAppend:
		return expression.ListAppend != nil && expression.ListAppend.Values != nil && expression.ListAppend.Value != nil && (exprContainsRecordEquality(*expression.ListAppend.Values) || exprContainsRecordEquality(*expression.ListAppend.Value))
	case ExprListAt:
		return expression.ListAt != nil && expression.ListAt.Values != nil && expression.ListAt.Index != nil && (exprContainsRecordEquality(*expression.ListAt.Values) || exprContainsRecordEquality(*expression.ListAt.Index))
	case ExprListFindByText:
		return expression.ListFind != nil && expression.ListFind.Values != nil && expression.ListFind.Key != nil && (exprContainsRecordEquality(*expression.ListFind.Values) || exprContainsRecordEquality(*expression.ListFind.Key))
	case ExprListFilterByText:
		return expression.ListFilter != nil && expression.ListFilter.Values != nil && expression.ListFilter.Key != nil && (exprContainsRecordEquality(*expression.ListFilter.Values) || exprContainsRecordEquality(*expression.ListFilter.Key))
	case ExprListFilterContainsCaseFolded:
		return expression.ListFilterContainsCaseFolded != nil && expression.ListFilterContainsCaseFolded.Values != nil && expression.ListFilterContainsCaseFolded.Query != nil && (exprContainsRecordEquality(*expression.ListFilterContainsCaseFolded.Values) || exprContainsRecordEquality(*expression.ListFilterContainsCaseFolded.Query))
	case ExprListFilterJoinedContainsCaseFolded:
		return expression.ListFilterJoinedContainsCaseFolded != nil && expression.ListFilterJoinedContainsCaseFolded.Values != nil && expression.ListFilterJoinedContainsCaseFolded.Query != nil && (exprContainsRecordEquality(*expression.ListFilterJoinedContainsCaseFolded.Values) || exprContainsRecordEquality(*expression.ListFilterJoinedContainsCaseFolded.Query))
	case ExprListSortByOrdinalText:
		return expression.ListSortByOrdinalText != nil && expression.ListSortByOrdinalText.Values != nil && exprContainsRecordEquality(*expression.ListSortByOrdinalText.Values)
	case ExprListSortByOrdinalTexts:
		return expression.ListSortByOrdinalTexts != nil && expression.ListSortByOrdinalTexts.Values != nil && exprContainsRecordEquality(*expression.ListSortByOrdinalTexts.Values)
	case ExprRecordConstruct:
		if expression.Record == nil {
			return false
		}
		for _, field := range expression.Record.Fields {
			if field.Value != nil && exprContainsRecordEquality(*field.Value) {
				return true
			}
		}
	}
	return false
}

func validateDirectRecordEquality(function Function) error {
	boolean := Type{Kind: TypePrimitive, Primitive: PrimitiveBool}
	if len(function.Parameters) != 2 || function.Body.Kind != ExprBinary || function.Body.Binary == nil || function.Body.Binary.Left == nil || function.Body.Binary.Right == nil {
		return fmt.Errorf("record equality requires exactly two parameters and one direct binary body")
	}
	binary := function.Body.Binary
	if binary.Operator != OperatorEqual && binary.Operator != OperatorNotEqual {
		return fmt.Errorf("record equality requires operator %q or %q", OperatorEqual, OperatorNotEqual)
	}
	if function.Parameters[0].Type.Kind != TypeRecord || !TypeEqual(function.Parameters[0].Type, function.Parameters[1].Type) || !TypeEqual(function.ReturnType, boolean) || !TypeEqual(function.Body.Type, boolean) {
		return fmt.Errorf("record equality requires two identical record parameters and a bool result")
	}
	if binary.Left.Kind != ExprReference || binary.Left.Parameter == nil || *binary.Left.Parameter != 0 || binary.Right.Kind != ExprReference || binary.Right.Parameter == nil || *binary.Right.Parameter != 1 {
		return fmt.Errorf("record equality operands must reference the two parameters in declared order")
	}
	return nil
}

func isLiteralType(value Type) bool {
	if value.Kind == TypeNumeric && value.Numeric != nil {
		return TypeEqual(value, SignedInteger(64)) || TypeEqual(value, BinaryFloat(64))
	}
	return value.Kind == TypePrimitive && (value.Primitive == PrimitiveString || value.Primitive == PrimitiveBool)
}

func CheckedInt64(operator Operator, left, right int64) (int64, ArithmeticError) {
	switch operator {
	case OperatorAdd:
		if (right > 0 && left > maxInt64-right) || (right < 0 && left < minInt64-right) {
			return 0, ArithmeticOverflow
		}
		return left + right, ""
	case OperatorSubtract:
		if (right < 0 && left > maxInt64+right) || (right > 0 && left < minInt64+right) {
			return 0, ArithmeticOverflow
		}
		return left - right, ""
	case OperatorMultiply:
		if left == 0 || right == 0 {
			return 0, ""
		}
		if (left == minInt64 && right == -1) || (right == minInt64 && left == -1) {
			return 0, ArithmeticOverflow
		}
		value := left * right
		if value/right != left {
			return 0, ArithmeticOverflow
		}
		return value, ""
	default:
		panic(fmt.Sprintf("unsupported checked int64 operator %q", operator))
	}
}

func CheckedNegateInt64(value int64) (int64, ArithmeticError) {
	if value == minInt64 {
		return 0, ArithmeticOverflow
	}
	return -value, ""
}

func CheckedDivideBinary64(left, right float64) (float64, ArithmeticError) {
	if right == 0 {
		return 0, ArithmeticDivisionByZero
	}
	return left / right, ""
}

// This is independent of source admission. Structural validation still checks all
// operand types, lexical references, and binding positions during program admission.
func validV820ConditionalLocalTree(expression Expr) bool {
	return validConditionalLocalTree(expression, 1)
}

func validConditionalLocalTree(expression Expr, maxChoices int) bool {
	// A zero bound admits finite sequences; historical callers retain their bounds.
	choices := 0
	hasChoices := false
	var walk func(Expr, int) bool
	walk = func(current Expr, remaining int) bool {
		for current.Kind == ExprImmutableLocal {
			local := current.ImmutableLocal
			if local == nil || local.Initializer == nil || local.Return == nil {
				return false
			}
			initializer := *local.Initializer
			if initializer.Kind == ExprConditional {
				choice := initializer.Conditional
				hasChoices = true
				if maxChoices > 0 {
					choices++
				}
				if (maxChoices > 0 && choices > maxChoices) || choice == nil || choice.TerminalStatement || choice.Condition == nil || choice.WhenTrue == nil || choice.WhenFalse == nil ||
					!validConditionalOperand(*choice.Condition) || !validConditionalOperand(*choice.WhenTrue) || !validConditionalOperand(*choice.WhenFalse) {
					return false
				}
			} else if !validConditionalOperand(initializer) {
				return false
			}
			current = *local.Return
		}
		if current.Kind != ExprConditional {
			return validConditionalOperand(current)
		}
		branch := current.Conditional
		return remaining > 0 && branch != nil && branch.TerminalStatement && branch.Condition != nil && branch.WhenTrue != nil && branch.WhenFalse != nil &&
			validConditionalOperand(*branch.Condition) && walk(*branch.WhenTrue, remaining-1) && walk(*branch.WhenFalse, remaining-1)
	}
	return countTerminalIfStatements(expression) > 0 && walk(expression, 3) && hasChoices
}

// Public v0.85 placement is validated independently of the source AST. Structural
// function validation still owns exact types, lexical references and positions.
func validStraightLineConditionalLocals(expr Expr) bool {
	hasChoice := false
	for expr.Kind == ExprImmutableLocal {
		local := expr.ImmutableLocal
		if local == nil || local.Initializer == nil || local.Return == nil {
			return false
		}
		init := *local.Initializer
		if init.Kind == ExprConditional {
			choice := init.Conditional
			if choice == nil || choice.TerminalStatement || choice.Condition == nil || choice.WhenTrue == nil || choice.WhenFalse == nil || !validConditionalOperand(*choice.Condition) || !validConditionalOperand(*choice.WhenTrue) || !validConditionalOperand(*choice.WhenFalse) {
				return false
			}
			hasChoice = true
		} else if !validConditionalOperand(init) {
			return false
		}
		expr = *local.Return
	}
	return hasChoice && validConditionalOperand(expr)
}

// Core checks public v0.86 placement independently of the source validator.
// Structural validation separately owns types, lexical references and positions.
func validConditionalReturnComposition(expr Expr) bool {
	hasLocal := false
	for expr.Kind == ExprImmutableLocal {
		hasLocal = true
		local := expr.ImmutableLocal
		if local == nil || local.Initializer == nil || local.Return == nil {
			return false
		}
		init := *local.Initializer
		if init.Kind == ExprConditional {
			if !validReturnCompositionChoice(init.Conditional) {
				return false
			}
		} else if !validConditionalOperand(init) {
			return false
		}
		expr = *local.Return
	}
	return hasLocal && expr.Kind == ExprConditional && validReturnCompositionChoice(expr.Conditional)
}

func validReturnCompositionChoice(choice *Conditional) bool {
	return choice != nil && !choice.TerminalStatement && choice.Condition != nil && choice.WhenTrue != nil && choice.WhenFalse != nil &&
		validConditionalOperand(*choice.Condition) && validConditionalOperand(*choice.WhenTrue) && validConditionalOperand(*choice.WhenFalse)
}

// Public v0.88 placement is checked independently of source admission. Core
// normalizes block/arrow spelling; structural validation owns types and bindings.
func validNestedStraightLineReturns(expr Expr) bool {
	for expr.Kind == ExprImmutableLocal {
		local := expr.ImmutableLocal
		if local == nil || local.Initializer == nil || local.Return == nil {
			return false
		}
		init := *local.Initializer
		if countImmutableLocalExpressions(init) != 0 {
			return false
		}
		if init.Kind == ExprConditional {
			if !validReturnCompositionChoice(init.Conditional) {
				return false
			}
		} else if !validConditionalOperand(init) {
			return false
		}
		expr = *local.Return
	}
	if expr.Kind != ExprConditional || countImmutableLocalExpressions(expr) != 0 {
		return false
	}
	var walk func(*Expr, int) bool
	walk = func(current *Expr, remaining int) bool {
		if current == nil {
			return false
		}
		if current.Kind != ExprConditional {
			return validConditionalOperand(*current)
		}
		choice := current.Conditional
		return remaining > 0 && choice != nil && !choice.TerminalStatement &&
			choice.Condition != nil && validConditionalOperand(*choice.Condition) &&
			walk(choice.WhenTrue, remaining-1) && walk(choice.WhenFalse, remaining-1)
	}
	return walk(&expr, 2)
}

// Public v0.90 placement is validated independently of the source AST. Generic
// internal Core local expressions remain supported by ValidateFunction.
func validNestedStraightLineInitializers(expr Expr) bool {
	hasLocal := false
	for expr.Kind == ExprImmutableLocal {
		hasLocal = true
		local := expr.ImmutableLocal
		if local == nil || local.Initializer == nil || local.Return == nil || countImmutableLocalExpressions(*local.Initializer) != 0 {
			return false
		}
		if !validNestedStraightLineReturns(*local.Initializer) && !validConditionalOperand(*local.Initializer) {
			return false
		}
		expr = *local.Return
	}
	return hasLocal && countImmutableLocalExpressions(expr) == 0 &&
		(validNestedStraightLineReturns(expr) || validConditionalOperand(expr))
}

// Independent Core placement admission; structural validation owns exact types,
// lexical references and canonical binding positions.
func validTerminalLeafConditionalReturns(expr Expr) bool {
	return validTerminalLeafReturnsDepth(expr, 1, 1)
}

func validNestedTerminalLeafReturns(expr Expr) bool {
	return validTerminalLeafReturnsDepth(expr, 2, 1)
}

// v0.91 counts statement depth separately from complete initializer/return choices.
func validNestedTerminalInitializers(expr Expr) bool {
	return validTerminalLeafReturnsDepth(expr, 2, 2)
}

func validTerminalLeafReturnsDepth(expr Expr, returnDepth, initializerDepth int) bool {
	return validTerminalLeafReturnsWithSelectors(expr, returnDepth, initializerDepth, false)
}

// v0.99 adds the flat selector only at complete returns in inherited terminal trees.
func validTerminalLeafBooleanSelectors(expr Expr) bool {
	return validTerminalLeafReturnsWithSelectors(expr, 3, 3, true)
}

func validTerminalLeafReturnsWithSelectors(expr Expr, returnDepth, initializerDepth int, allowSelectors bool) bool {
	hasReturnChoice := false
	var walk func(Expr, int) bool
	walk = func(current Expr, remaining int) bool {
		for current.Kind == ExprImmutableLocal {
			local := current.ImmutableLocal
			if local == nil || local.Initializer == nil || local.Return == nil {
				return false
			}
			initializer := *local.Initializer
			if returnDepth >= 2 && countImmutableLocalExpressions(initializer) != 0 {
				return false
			}
			if initializer.Kind == ExprConditional {
				if (initializerDepth == 3 && !validDepthThreeStraightLineReturns(*local.Initializer)) || (initializerDepth == 2 && !validNestedStraightLineReturns(initializer)) || (initializerDepth == 1 && !validReturnCompositionChoice(initializer.Conditional)) {
					return false
				}
			} else if !validConditionalOperand(initializer) {
				return false
			}
			current = *local.Return
		}
		if current.Kind != ExprConditional {
			return validConditionalOperand(current) && (returnDepth == 1 || countImmutableLocalExpressions(current) == 0)
		}
		choice := current.Conditional
		if choice == nil {
			return false
		}
		if !choice.TerminalStatement {
			hasReturnChoice = true
			if allowSelectors && validConditionalBooleanSelector(current) {
				return true
			}
			if returnDepth == 3 {
				return validDepthThreeStraightLineReturns(current)
			}
			if returnDepth == 2 {
				return validNestedStraightLineReturns(current)
			}
			return validReturnCompositionChoice(choice)
		}
		return remaining > 0 && choice.Condition != nil && choice.WhenTrue != nil && choice.WhenFalse != nil &&
			validConditionalOperand(*choice.Condition) && (returnDepth == 1 || countImmutableLocalExpressions(*choice.Condition) == 0) && walk(*choice.WhenTrue, remaining-1) && walk(*choice.WhenFalse, remaining-1)
	}
	return countTerminalIfStatements(expr) > 0 && walk(expr, 3) && (hasReturnChoice || initializerDepth >= 2)
}

// v0.93 widens only the straight-line return tree. Core erases block/arrow spelling;
// the parser owns that source distinction. This validator never widens initializers.
func validDepthThreeStraightLineReturns(expr Expr) bool {
	for expr.Kind == ExprImmutableLocal {
		local := expr.ImmutableLocal
		if local == nil || local.Initializer == nil || local.Return == nil || countImmutableLocalExpressions(*local.Initializer) != 0 ||
			(!validConditionalOperand(*local.Initializer) && !validNestedStraightLineReturns(*local.Initializer)) {
			return false
		}
		expr = *local.Return
	}
	if expr.Kind != ExprConditional || countImmutableLocalExpressions(expr) != 0 {
		return false
	}
	var walk func(*Expr, int) bool
	walk = func(current *Expr, remaining int) bool {
		if current == nil {
			return false
		}
		if current.Kind != ExprConditional {
			return validConditionalOperand(*current)
		}
		choice := current.Conditional
		return remaining > 0 && choice != nil && !choice.TerminalStatement && choice.Condition != nil &&
			validConditionalOperand(*choice.Condition) && walk(choice.WhenTrue, remaining-1) && walk(choice.WhenFalse, remaining-1)
	}
	return walk(&expr, 3)
}

// v0.94 widens complete terminal-leaf returns only; initializer and statement
// depths remain independently bounded. Hidden local operands remain forbidden.
func validDepthThreeTerminalLeafReturns(expr Expr) bool {
	return validTerminalLeafReturnsDepth(expr, 3, 2)
}

// Independent public v0.96 placement validation; generic internal Core locals retain their contract.
func validDepthThreeStraightLineInitializers(expr Expr) bool {
	hasLocal := false
	for expr.Kind == ExprImmutableLocal {
		hasLocal = true
		local := expr.ImmutableLocal
		if local == nil || local.Initializer == nil || local.Return == nil || countImmutableLocalExpressions(*local.Initializer) != 0 ||
			(!validConditionalOperand(*local.Initializer) && !validDepthThreeStraightLineReturns(*local.Initializer)) {
			return false
		}
		expr = *local.Return
	}
	return hasLocal && countImmutableLocalExpressions(expr) == 0 &&
		(validConditionalOperand(expr) || validDepthThreeStraightLineReturns(expr))
}

// v0.97 widens initializer placement within inherited depth-three terminal trees.
// Conditions and operands remain local-free; return and statement limits do not grow.
func validDepthThreeTerminalInitializers(expr Expr) bool {
	return validTerminalLeafReturnsDepth(expr, 3, 3)
}

// Independent public placement gate. Type validation still requires bool selectors
// and exact arms. Core erases block/arrow spelling; the parser checks that boundary.
func validConditionalBooleanSelector(expr Expr) bool {
	for expr.Kind == ExprImmutableLocal {
		local := expr.ImmutableLocal
		if local == nil || local.Initializer == nil || local.Return == nil || countImmutableLocalExpressions(*local.Initializer) != 0 ||
			(!validConditionalOperand(*local.Initializer) && !validDepthThreeStraightLineReturns(*local.Initializer)) {
			return false
		}
		expr = *local.Return
	}
	if expr.Kind != ExprConditional || expr.Conditional == nil || expr.Conditional.TerminalStatement || countImmutableLocalExpressions(expr) != 0 {
		return false
	}
	outer := expr.Conditional
	if outer.Condition == nil || outer.Condition.Kind != ExprConditional || outer.Condition.Conditional == nil {
		return false
	}
	selector := outer.Condition.Conditional
	if selector.TerminalStatement {
		return false
	}
	for _, operand := range []*Expr{selector.Condition, selector.WhenTrue, selector.WhenFalse, outer.WhenTrue, outer.WhenFalse} {
		if operand == nil || !validConditionalOperand(*operand) {
			return false
		}
	}
	return true
}

// Independent public placement gate; generic internal Core locals remain valid.
func validStraightLineBooleanSelectorInitializers(expr Expr) bool {
	hasSelector := false
	for expr.Kind == ExprImmutableLocal {
		local := expr.ImmutableLocal
		if local == nil || local.Initializer == nil || local.Return == nil || countImmutableLocalExpressions(*local.Initializer) != 0 {
			return false
		}
		selector := validConditionalBooleanSelector(*local.Initializer)
		if !selector && !validConditionalOperand(*local.Initializer) && !validDepthThreeStraightLineReturns(*local.Initializer) {
			return false
		}
		hasSelector = hasSelector || selector
		expr = *local.Return
	}
	return hasSelector && countImmutableLocalExpressions(expr) == 0 &&
		(validConditionalOperand(expr) || validDepthThreeStraightLineReturns(expr) || validConditionalBooleanSelector(expr))
}

// Independent public placement validation; generic Core locals are unchanged.
func validTerminalBooleanSelectorInitializers(expr Expr) bool {
	hasSelector, hasStatement := false, false
	var validScope func(Expr, int) bool
	validScope = func(expr Expr, depth int) bool {
		for expr.Kind == ExprImmutableLocal {
			local := expr.ImmutableLocal
			if local == nil || local.Initializer == nil || local.Return == nil || countImmutableLocalExpressions(*local.Initializer) != 0 {
				return false
			}
			selector := validConditionalBooleanSelector(*local.Initializer)
			if !selector && !validConditionalOperand(*local.Initializer) && !validDepthThreeStraightLineReturns(*local.Initializer) {
				return false
			}
			hasSelector = hasSelector || selector
			expr = *local.Return
		}
		if expr.Kind == ExprConditional && expr.Conditional != nil && expr.Conditional.TerminalStatement {
			branch := expr.Conditional
			hasStatement = true
			return depth < 3 && branch.Condition != nil && countImmutableLocalExpressions(*branch.Condition) == 0 && validConditionalOperand(*branch.Condition) &&
				branch.WhenTrue != nil && branch.WhenFalse != nil &&
				validScope(*branch.WhenTrue, depth+1) && validScope(*branch.WhenFalse, depth+1)
		}
		return countImmutableLocalExpressions(expr) == 0 &&
			(validConditionalOperand(expr) || validDepthThreeStraightLineReturns(expr) || validConditionalBooleanSelector(expr))
	}
	return validScope(expr, 0) && hasSelector && hasStatement
}

// v0.103 permits flat conditional tests at existing terminal statement positions.
// Local sequences and inherited value expressions do not increase statement depth.
func validTerminalConditionalTests(expr Expr) bool {
	hasConditionalTest := false
	var validScope func(Expr, int) bool
	validScope = func(expr Expr, depth int) bool {
		for expr.Kind == ExprImmutableLocal {
			local := expr.ImmutableLocal
			if local == nil || local.Initializer == nil || local.Return == nil || countImmutableLocalExpressions(*local.Initializer) != 0 {
				return false
			}
			selector := validConditionalBooleanSelector(*local.Initializer)
			if !selector && !validConditionalOperand(*local.Initializer) && !validDepthThreeStraightLineReturns(*local.Initializer) {
				return false
			}
			expr = *local.Return
		}
		if expr.Kind == ExprConditional && expr.Conditional != nil && expr.Conditional.TerminalStatement {
			branch := expr.Conditional
			conditional := branch.Condition != nil && validFlatConditionalTest(*branch.Condition)
			hasConditionalTest = hasConditionalTest || conditional
			return depth < 3 && branch.Condition != nil && countImmutableLocalExpressions(*branch.Condition) == 0 && (validConditionalOperand(*branch.Condition) || conditional) &&
				branch.WhenTrue != nil && branch.WhenFalse != nil &&
				validScope(*branch.WhenTrue, depth+1) && validScope(*branch.WhenFalse, depth+1)
		}
		return countImmutableLocalExpressions(expr) == 0 &&
			(validConditionalOperand(expr) || validDepthThreeStraightLineReturns(expr) || validConditionalBooleanSelector(expr))
	}
	return validScope(expr, 0) && hasConditionalTest
}

func validFlatConditionalTest(expr Expr) bool {
	if expr.Kind != ExprConditional || expr.Conditional == nil || expr.Conditional.TerminalStatement || countImmutableLocalExpressions(expr) != 0 {
		return false
	}
	c := expr.Conditional
	for _, operand := range []*Expr{c.Condition, c.WhenTrue, c.WhenFalse} {
		if operand == nil || !validConditionalOperand(*operand) {
			return false
		}
	}
	return true
}

// v0.104 permits depth-two value arms in terminal tests; selector nesting stays excluded.
func validNestedTerminalConditionalTests(expr Expr) bool {
	hasConditionalTest := false
	var validScope func(Expr, int) bool
	validScope = func(expr Expr, depth int) bool {
		for expr.Kind == ExprImmutableLocal {
			local := expr.ImmutableLocal
			if local == nil || local.Initializer == nil || local.Return == nil || countImmutableLocalExpressions(*local.Initializer) != 0 {
				return false
			}
			selector := validConditionalBooleanSelector(*local.Initializer)
			if !selector && !validConditionalOperand(*local.Initializer) && !validDepthThreeStraightLineReturns(*local.Initializer) {
				return false
			}
			expr = *local.Return
		}
		if expr.Kind == ExprConditional && expr.Conditional != nil && expr.Conditional.TerminalStatement {
			branch := expr.Conditional
			conditional := branch.Condition != nil && validNestedConditionalTest(*branch.Condition)
			hasConditionalTest = hasConditionalTest || conditional
			return depth < 3 && branch.Condition != nil && countImmutableLocalExpressions(*branch.Condition) == 0 && (validConditionalOperand(*branch.Condition) || conditional) &&
				branch.WhenTrue != nil && branch.WhenFalse != nil &&
				validScope(*branch.WhenTrue, depth+1) && validScope(*branch.WhenFalse, depth+1)
		}
		return countImmutableLocalExpressions(expr) == 0 &&
			(validConditionalOperand(expr) || validDepthThreeStraightLineReturns(expr) || validConditionalBooleanSelector(expr))
	}
	return validScope(expr, 0) && hasConditionalTest
}

func validNestedConditionalTest(expr Expr) bool {
	if expr.Kind != ExprConditional || expr.Conditional == nil || expr.Conditional.TerminalStatement || countImmutableLocalExpressions(expr) != 0 {
		return false
	}
	c := expr.Conditional
	if c.Condition == nil || !validConditionalOperand(*c.Condition) {
		return false
	}
	for _, arm := range []*Expr{c.WhenTrue, c.WhenFalse} {
		if arm == nil || (!validConditionalOperand(*arm) && !validFlatConditionalTest(*arm)) {
			return false
		}
	}
	return true
}

// v0.105 permits depth-three value arms in terminal tests; selector nesting stays excluded.
func validDepthThreeTerminalConditionalTests(expr Expr) bool {
	hasConditionalTest := false
	var validScope func(Expr, int) bool
	validScope = func(expr Expr, depth int) bool {
		for expr.Kind == ExprImmutableLocal {
			local := expr.ImmutableLocal
			if local == nil || local.Initializer == nil || local.Return == nil || countImmutableLocalExpressions(*local.Initializer) != 0 {
				return false
			}
			selector := validConditionalBooleanSelector(*local.Initializer)
			if !selector && !validConditionalOperand(*local.Initializer) && !validDepthThreeStraightLineReturns(*local.Initializer) {
				return false
			}
			expr = *local.Return
		}
		if expr.Kind == ExprConditional && expr.Conditional != nil && expr.Conditional.TerminalStatement {
			branch := expr.Conditional
			conditional := branch.Condition != nil && validDepthThreeConditionalTest(*branch.Condition)
			hasConditionalTest = hasConditionalTest || conditional
			return depth < 3 && branch.Condition != nil && countImmutableLocalExpressions(*branch.Condition) == 0 && (validConditionalOperand(*branch.Condition) || conditional) &&
				branch.WhenTrue != nil && branch.WhenFalse != nil &&
				validScope(*branch.WhenTrue, depth+1) && validScope(*branch.WhenFalse, depth+1)
		}
		return countImmutableLocalExpressions(expr) == 0 &&
			(validConditionalOperand(expr) || validDepthThreeStraightLineReturns(expr) || validConditionalBooleanSelector(expr))
	}
	return validScope(expr, 0) && hasConditionalTest
}

func validDepthThreeConditionalTest(expr Expr) bool {
	if expr.Kind != ExprConditional || expr.Conditional == nil || expr.Conditional.TerminalStatement || countImmutableLocalExpressions(expr) != 0 {
		return false
	}
	c := expr.Conditional
	if c.Condition == nil || !validConditionalOperand(*c.Condition) {
		return false
	}
	for _, arm := range []*Expr{c.WhenTrue, c.WhenFalse} {
		if arm == nil || (!validConditionalOperand(*arm) && !validNestedConditionalTest(*arm)) {
			return false
		}
	}
	return true
}
