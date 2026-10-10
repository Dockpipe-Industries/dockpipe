package coreir

import (
	"fmt"
	"strings"
)

func enumRepresentationEqual(a, b *EnumType) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	if len(a.Members) != len(b.Members) {
		return false
	}
	for i, x := range a.Members {
		y := b.Members[i]
		if x.Name != y.Name || x.Tag != y.Tag || x.Identity.PackageID != y.Identity.PackageID || x.Identity.Path != y.Identity.Path || x.Identity.Callable != nil || y.Identity.Callable != nil {
			return false
		}
	}
	return true
}
func validateEnumType(t Type) error {
	if t.Enum == nil || len(t.Enum.Members) == 0 || t.Identity == nil || t.Identity.PackageID == "" || t.Identity.Path == "" || t.Identity.Callable != nil || !enumIdentifier(t.Name) {
		return fmt.Errorf("enum requires a nominal identity and closed member schema")
	}
	if t.Primitive != "" || t.Numeric != nil || t.Record != nil || t.Optional != nil || t.Result != nil || t.List != nil || len(t.Arguments) != 0 {
		return fmt.Errorf("enum type carries a non-enum representation")
	}
	names, identities := map[string]bool{}, map[string]bool{}
	for i, m := range t.Enum.Members {
		if !enumIdentifier(m.Name) || m.Tag == "" || ValidateText(m.Tag) != nil || m.Identity.PackageID != t.Identity.PackageID || (!strings.HasPrefix(m.Identity.Path, t.Identity.Path+".") || strings.Contains(strings.TrimPrefix(m.Identity.Path, t.Identity.Path+"."), ".") || m.Identity.Path == t.Identity.Path+".") || m.Identity.Callable != nil {
			return fmt.Errorf("enum member %d has invalid identity/name/tag", i)
		}
		if i > 0 && t.Enum.Members[i-1].Tag >= m.Tag {
			return fmt.Errorf("enum members require unique tags in canonical tag order")
		}
		if names[m.Name] || identities[m.Identity.Path] {
			return fmt.Errorf("enum repeats a member name or identity")
		}
		names[m.Name] = true
		identities[m.Identity.Path] = true
	}
	return nil
}

// ValidateEnumTag is the same closed-value boundary for the evaluator and hosts.
func ValidateEnumTag(t Type, tag string) error {
	if t.Kind != TypeEnum {
		return fmt.Errorf("enum value requires enum type")
	}
	if err := validateEnumType(t); err != nil {
		return err
	}
	for _, m := range t.Enum.Members {
		if m.Tag == tag {
			return nil
		}
	}
	return fmt.Errorf("unknown tag %q for enum %s", tag, t.Name)
}
func FunctionHasEnum(f Function) bool {
	if f.ReturnType.Kind == TypeEnum {
		return true
	}
	for _, p := range f.Parameters {
		if p.Type.Kind == TypeEnum {
			return true
		}
	}
	var walk func(Expr) bool
	walk = func(e Expr) bool {
		if e.Type.Kind == TypeEnum {
			return true
		}
		for _, c := range expressionChildren(e) {
			if c != nil && walk(*c) {
				return true
			}
		}
		return false
	}
	return walk(f.Body)
}
func validateEnumDeclarations(p Program) error {
	seen := map[string]Type{}
	typ := func(t Type) error {
		if t.Kind == TypeEnum {
			if err := validateEnumType(t); err != nil {
				return err
			}
			key := t.Identity.PackageID + "\x00" + t.Identity.Path
			if old, ok := seen[key]; ok && !TypeEqual(old, t) {
				return fmt.Errorf("enum identity has conflicting declarations")
			}
			seen[key] = t
		}
		return nil
	}
	var walk func(Expr) error
	walk = func(e Expr) error {
		if err := typ(e.Type); err != nil {
			return err
		}
		for _, c := range expressionChildren(e) {
			if c != nil {
				if err := walk(*c); err != nil {
					return err
				}
			}
		}
		return nil
	}
	for _, f := range p.Functions {
		for _, local := range BlockLocalTypes(f.Body.Block) {
			if err := typ(local); err != nil {
				return err
			}
		}
		if err := typ(f.ReturnType); err != nil {
			return err
		}
		for _, a := range f.Parameters {
			if err := typ(a.Type); err != nil {
				return err
			}
		}
		if err := walk(f.Body); err != nil {
			return err
		}
	}
	return nil
}

// Enum composition adds closed matching to pure value expressions. The selected
// slice does not admit mutation, carriers, arbitrary control flow or deeper
// conditional trees through an otherwise unused enum parameter.
func validateEnumComposition(f Function) error {
	valueType := func(t Type) bool { return t.Kind == TypeEnum || t.Kind == TypePrimitive || t.Kind == TypeNumeric }
	if !valueType(f.ReturnType) {
		return fmt.Errorf("enum composition result requires a primitive or enum value")
	}
	for _, p := range f.Parameters {
		if !valueType(p.Type) {
			return fmt.Errorf("enum composition parameters require primitive or enum values")
		}
	}
	var walk func(Expr, int, int, bool) error
	walk = func(e Expr, depth, matchDepth int, allowLocal bool) error {
		if !valueType(e.Type) {
			return fmt.Errorf("enum composition excludes carrier and object values")
		}
		switch e.Kind {
		case ExprLiteral, ExprReference:
			return nil
		case ExprImmutableLocal:
			if !allowLocal || e.ImmutableLocal == nil || e.ImmutableLocal.Initializer == nil || e.ImmutableLocal.Return == nil {
				return fmt.Errorf("enum local has invalid placement")
			}
			if err := walk(*e.ImmutableLocal.Initializer, depth, matchDepth, false); err != nil {
				return err
			}
			return walk(*e.ImmutableLocal.Return, depth, matchDepth, true)
		case ExprConditional:
			if e.Conditional == nil || e.Conditional.Condition == nil || e.Conditional.WhenTrue == nil || e.Conditional.WhenFalse == nil {
				return fmt.Errorf("enum conditional requires a condition and both branches")
			}
			depth++
			if depth > 3 {
				return fmt.Errorf("enum composition conditional depth exceeds three")
			}
			if e.Conditional != nil && e.Conditional.TerminalStatement {
				if !allowLocal {
					return fmt.Errorf("enum terminal statement has invalid expression placement")
				}
				if err := walk(*e.Conditional.Condition, depth, matchDepth, false); err != nil {
					return err
				}
				if err := walk(*e.Conditional.WhenTrue, depth, matchDepth, true); err != nil {
					return err
				}
				return walk(*e.Conditional.WhenFalse, depth, matchDepth, true)
			}
		case ExprMatch:
			matchDepth++
			if matchDepth > 3 || e.Match == nil || e.Match.Value == nil || e.Match.Value.Type.Kind != TypeEnum {
				return fmt.Errorf("enum composition requires an enum match through depth three")
			}
		case ExprCall, ExprBinary, ExprUnary, ExprTextTrim, ExprTextContainsCaseFolded:
		default:
			return fmt.Errorf("unsupported enum composition expression %q", e.Kind)
		}
		for _, c := range expressionChildren(e) {
			if c != nil {
				if err := walk(*c, depth, matchDepth, false); err != nil {
					return err
				}
			}
		}
		return nil
	}
	return walk(f.Body, 0, 0, true)
}

func enumIdentifier(name string) bool {
	if name == "" {
		return false
	}
	for i, r := range name {
		if r != '_' && !(r >= 'A' && r <= 'Z') && !(r >= 'a' && r <= 'z') && !(i > 0 && r >= '0' && r <= '9') {
			return false
		}
	}
	return true
}
