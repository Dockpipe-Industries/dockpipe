package coreir

import "fmt"

// This ordered allowlist admits exact identities, not arbitrary semver strings.
// Feature ordinals below refer to these accepted language contracts only.
var supportedLanguageContracts = [...]string{
	LanguageContractV010, LanguageContractV020, LanguageContractV030, LanguageContractV040, LanguageContractV050, LanguageContractV060, LanguageContractV070, LanguageContractV080,
	LanguageContractV090, LanguageContractV100, LanguageContractV110, LanguageContractV120, LanguageContractV130, LanguageContractV140, LanguageContractV150, LanguageContractV160,
	LanguageContractV170, LanguageContractV180, LanguageContractV190, LanguageContractV200, LanguageContractV210, LanguageContractV220, LanguageContractV230, LanguageContractV240,
	LanguageContractV250, LanguageContractV260, LanguageContractV270, LanguageContractV280, LanguageContractV290, LanguageContractV300, LanguageContractV310, LanguageContractV320,
	LanguageContractV330, LanguageContractV340, LanguageContractV350, LanguageContractV360, LanguageContractV370, LanguageContractV380, LanguageContractV390, LanguageContractV400,
	LanguageContractV410, LanguageContractV420, LanguageContractV430, LanguageContractV440, LanguageContractV450, LanguageContractV460, LanguageContractV470, LanguageContractV480,
	LanguageContractV490, LanguageContractV500, LanguageContractV510, LanguageContractV520, LanguageContractV530, LanguageContractV540, LanguageContractV550, LanguageContractV560,
	LanguageContractV570, LanguageContractV580, LanguageContractV590, LanguageContractV600, LanguageContractV610, LanguageContractV620, LanguageContractV630, LanguageContractV640,
	LanguageContractV650, LanguageContractV660, LanguageContractV670, LanguageContractV680, LanguageContractV690, LanguageContractV700, LanguageContractV710, LanguageContractV720,
	LanguageContractV730, LanguageContractV740, LanguageContractV750, LanguageContractV760, LanguageContractV770, LanguageContractV780, LanguageContractV790, LanguageContractV800, LanguageContractV810, LanguageContractV820, LanguageContractV830, LanguageContractV840, LanguageContractV850, LanguageContractV860, LanguageContractV870, LanguageContractV880,
}

func validateProgramIdentity(program Program) (int, error) {
	if program.CompilerContract != CompilerContractV1 {
		return 0, fmt.Errorf("unsupported Core IR compiler contract %q", program.CompilerContract)
	}
	for index, contract := range supportedLanguageContracts {
		if program.LanguageContract == contract {
			return index + 1, nil
		}
	}
	return 0, fmt.Errorf("unsupported Core IR language contract %q", program.LanguageContract)
}

// validateFeatureContract owns target-neutral feature admission. Structural
// validation remains in ValidateFunction; later composition/topology gates remain
// in ValidateProgram. Check signatures and every child, even unused/unselected ones.
// Checked arithmetic and arithmetic Result representation are already internal Core
// capabilities in v0.1.0; their later source-syntax gates must not be imposed here.
func validateFeatureContract(version int, function Function) error {
	require := func(minimum int, feature string) error {
		if version < minimum {
			return fmt.Errorf("function %s %s requires language contract %q or later", function.Name, feature, supportedLanguageContracts[minimum-1])
		}
		return nil
	}
	var checkType func(Type) error
	checkType = func(typ Type) error {
		minimum := 1
		switch typ.Kind {
		case TypeRecord:
			minimum = 9
		case TypeList:
			minimum = 15
		case TypeOptional:
			minimum = 13
			if typ.Optional != nil && typ.Optional.Value.Kind == TypeRecord {
				minimum = 18
			}
		case TypeResult:
			if typ.Result != nil {
				switch typ.Result.Success.Kind {
				case TypeList:
					minimum = 19
				case TypePrimitive:
					if typ.Result.Success.Primitive == PrimitiveString {
						minimum = 25
					}
				}
			}
		}
		if err := require(minimum, string(typ.Kind)+" type"); err != nil {
			return err
		}
		var children []Type
		if typ.Record != nil {
			for _, field := range typ.Record.Fields {
				children = append(children, field.Type)
			}
		}
		if typ.List != nil {
			children = append(children, typ.List.Element)
		}
		if typ.Optional != nil {
			children = append(children, typ.Optional.Value)
		}
		if typ.Result != nil {
			children = append(children, typ.Result.Success, typ.Result.Failure)
		}
		children = append(children, typ.Arguments...)
		for _, child := range children {
			if err := checkType(child); err != nil {
				return err
			}
		}
		return nil
	}
	for _, parameter := range function.Parameters {
		if err := checkType(parameter.Type); err != nil {
			return err
		}
	}
	if err := checkType(function.ReturnType); err != nil {
		return err
	}
	var walk func(Expr) error
	walk = func(expression Expr) error {
		if err := checkType(expression.Type); err != nil {
			return err
		}
		minimum := 1
		switch expression.Kind {
		case ExprBinary:
			if binary := expression.Binary; binary != nil {
				switch binary.Operator {
				case OperatorLessThan, OperatorLessOrEqual, OperatorGreaterThan, OperatorGreaterOrEqual:
					if binary.Left != nil && binary.Left.Type.Primitive == PrimitiveString {
						minimum = 8
					}
				case OperatorEqual, OperatorNotEqual:
					if binary.Left != nil && binary.Left.Type.Kind == TypeRecord {
						minimum = 12
					}
				}
			}
		case ExprFieldProjection:
			minimum = 10
		case ExprRecordConstruct:
			minimum = 11
		case ExprOptionalSome, ExprOptionalNone, ExprOptionalHasValue:
			minimum = 13
		case ExprOptionalValueOr:
			minimum = 14
		case ExprListEmpty, ExprListSingleton:
			minimum = 15
		case ExprListCount:
			minimum = 16
		case ExprListAppend:
			minimum = 17
		case ExprResultOK, ExprResultErr, ExprResultIsOK, ExprResultSuccessOr, ExprResultFailureOr:
			minimum = 19
		case ExprListAt:
			minimum = 20 // v0.33 postfix indexing reuses this existing Core operation.
		case ExprListFindByText:
			minimum = 21
		case ExprListFilterByText:
			minimum = 22
		case ExprTextContainsCaseFolded:
			minimum = 23
		case ExprListFilterContainsCaseFolded:
			minimum = 24
		case ExprTextTrim:
			minimum = 26
		case ExprListFilterJoinedContainsCaseFolded:
			minimum = 27
			if expression.ListFilterJoinedContainsCaseFolded != nil && len(expression.ListFilterJoinedContainsCaseFolded.Selectors) != 5 {
				minimum = 29
			}
		case ExprListSortByOrdinalText:
			minimum = 28
		case ExprListSortByOrdinalTexts:
			minimum = 30
		case ExprListFilterPredicate:
			minimum = 31
		case ExprListSortByOrdinalDirections:
			minimum = 32
		case ExprPropagate:
			minimum = 34
		case ExprMatch:
			minimum = 35
		default:
			// Calls, conditionals, locals and their later expansions have contextual gates.
		}
		if err := require(minimum, string(expression.Kind)); err != nil {
			return err
		}
		if expression.ImmutableLocal != nil {
			if err := checkType(expression.ImmutableLocal.Type); err != nil {
				return err
			}
		}
		if expression.Propagate != nil {
			if err := checkType(expression.Propagate.Carrier); err != nil {
				return err
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
